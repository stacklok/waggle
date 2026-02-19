// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/stacklok/waggle/pkg/config"
	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/execution"
	"github.com/stacklok/waggle/pkg/infra/vm"
)

// maxCodeSize is the maximum allowed size for code payloads (500 KB).
const maxCodeSize = 512_000

const (
	defaultPythonCommand  = "python3"
	defaultPipInstall     = "pip install"
	defaultNodeCommand    = "node"
	defaultNpmInstall     = "npm install -g"
	defaultShellCommand   = "sh"
	defaultShellInstall   = "apk add --no-cache"
	defaultPythonVenv     = "/home/sandbox/venv"
	defaultPythonFallback = "/usr/bin/python3"
)

// ExecutionService orchestrates code execution within environments.
type ExecutionService struct {
	repo     environment.Repository
	executor execution.Executor
	provider vm.Provider
	envSvc   *EnvironmentService
	config   *config.Config
}

// NewExecutionService creates a new ExecutionService.
func NewExecutionService(
	repo environment.Repository,
	executor execution.Executor,
	provider vm.Provider,
	envSvc *EnvironmentService,
	cfg *config.Config,
) *ExecutionService {
	return &ExecutionService{
		repo:     repo,
		executor: executor,
		provider: provider,
		envSvc:   envSvc,
		config:   cfg,
	}
}

// Execute runs code in the specified environment. It validates the environment,
// resolves connection info, and delegates execution to the Executor.
func (s *ExecutionService) Execute(
	ctx context.Context, envID, code, language string, timeoutSec int,
) (*execution.ExecResult, error) {
	if len(code) > maxCodeSize {
		return nil, fmt.Errorf("code payload too large: %d bytes exceeds limit of %d bytes", len(code), maxCodeSize)
	}

	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return nil, err
	}
	if !env.IsRunning() {
		return nil, environment.ErrNotRunning
	}

	// Touch the environment to reset inactivity timer.
	if err := s.envSvc.Touch(ctx, envID); err != nil {
		slog.Warn("failed to touch environment", "env_id", envID, "error", err)
	}

	// Determine runtime for this execution.
	rt := env.Runtime
	if language != "" {
		parsed, parseErr := environment.ParseRuntime(language)
		if parseErr != nil {
			return nil, parseErr
		}
		rt = parsed
	}

	s.ensureCapabilities(ctx, env)

	// Resolve timeout.
	timeout := s.config.DefaultExecTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
		if timeout > s.config.MaxExecTimeout {
			timeout = s.config.MaxExecTimeout
		}
	}

	// Resolve pre-validated connection info.
	conn, connErr := s.connInfo(envID, env.SSHPort)
	if connErr != nil {
		return nil, connErr
	}

	req := &execution.CodeExecution{
		Language:      rt.String(),
		Code:          code,
		TimeoutMs:     timeout.Milliseconds(),
		ExecCommand:   s.resolveExecCommand(env, rt),
		FileExtension: rt.FileExtension(),
	}

	return s.executor.ExecuteCode(ctx, envID, conn, req)
}

// InstallPackages installs language packages in the specified environment.
// For Python, it retries inside a virtual environment if PEP 668 blocks installs.
func (s *ExecutionService) InstallPackages(
	ctx context.Context, envID string, packages []string,
) (*execution.ExecResult, error) {
	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return nil, err
	}
	if !env.IsRunning() {
		return nil, environment.ErrNotRunning
	}

	if err := s.envSvc.Touch(ctx, envID); err != nil {
		slog.Warn("failed to touch environment", "env_id", envID, "error", err)
	}

	if len(packages) == 0 {
		return &execution.ExecResult{ExitCode: 0}, nil
	}

	s.ensureCapabilities(ctx, env)

	validatedNames, err := validatePackages(packages)
	if err != nil {
		return nil, err
	}

	conn, connErr := s.connInfo(envID, env.SSHPort)
	if connErr != nil {
		return nil, connErr
	}

	req := &execution.PackageInstallation{
		Language:       env.Runtime.String(),
		Packages:       validatedNames,
		InstallCommand: s.resolveInstallCommand(env),
	}

	result, runErr := s.executor.InstallPackages(ctx, envID, conn, req)
	if runErr != nil || result == nil || result.Succeeded() {
		return result, runErr
	}
	if env.Runtime != environment.RuntimePython || !isPep668(result) {
		return result, runErr
	}

	return s.retryPythonInstallInVenv(ctx, env, conn, validatedNames)
}

func (s *ExecutionService) resolveExecCommand(env *environment.Environment, rt environment.Runtime) string {
	if s.config != nil {
		if cmd := s.config.RuntimeExecCommand(rt); cmd != "" {
			if isSafePathCommand(cmd) {
				return cmd
			}
			slog.Warn("ignoring unsafe runtime exec command override", "runtime", rt, "command", cmd)
		}
	}

	if env.CapabilitiesDetected {
		switch rt {
		case environment.RuntimePython:
			if env.Capabilities.PythonCommand != "" {
				if isSafePathCommand(env.Capabilities.PythonCommand) {
					return env.Capabilities.PythonCommand
				}
				slog.Warn("ignoring unsafe probed python command", "runtime", rt, "command", env.Capabilities.PythonCommand)
			}
		case environment.RuntimeNode:
			if env.Capabilities.NodeCommand != "" {
				if isSafePathCommand(env.Capabilities.NodeCommand) {
					return env.Capabilities.NodeCommand
				}
				slog.Warn("ignoring unsafe probed node command", "runtime", rt, "command", env.Capabilities.NodeCommand)
			}
		case environment.RuntimeShell:
		}
	}

	if cmd := fallbackExecCommand(rt); cmd != "" {
		return cmd
	}

	return rt.ExecCommand()
}

func (s *ExecutionService) resolveInstallCommand(env *environment.Environment) string {
	if s.config != nil {
		if cmd := s.config.RuntimeInstallCommand(env.Runtime); cmd != "" {
			if isSafeInstallCommand(cmd) {
				return cmd
			}
			slog.Warn("ignoring unsafe runtime install command override", "runtime", env.Runtime, "command", cmd)
		}
	}

	if cmd := installCommandFromCapabilities(env); cmd != "" {
		return cmd
	}

	if cmd := fallbackInstallCommand(env.Runtime); cmd != "" {
		return cmd
	}

	return env.Runtime.PackageInstallCommand()
}

func (s *ExecutionService) ensureCapabilities(ctx context.Context, env *environment.Environment) {
	if s.envSvc == nil {
		return
	}
	if err := s.envSvc.EnsureCapabilities(ctx, env); err != nil {
		slog.Warn("capability probe failed", "env_id", env.ID, "error", err)
	}
}

func (s *ExecutionService) retryPythonInstallInVenv(
	ctx context.Context,
	env *environment.Environment,
	conn execution.ConnInfo,
	packages []string,
) (*execution.ExecResult, error) {
	pythonCmd := s.resolveExecCommand(env, environment.RuntimePython)
	if pythonCmd == defaultPythonCommand {
		pythonCmd = defaultPythonFallback
	}
	venvResult, venvErr := s.createPythonVenv(ctx, env.ID, conn, pythonCmd, defaultPythonVenv)
	if venvErr != nil {
		return venvResult, venvErr
	}
	if venvResult != nil && !venvResult.Succeeded() {
		return venvResult, nil
	}

	venvInstall := &execution.PackageInstallation{
		Language:       env.Runtime.String(),
		Packages:       packages,
		InstallCommand: fmt.Sprintf("%s/bin/pip install", defaultPythonVenv),
	}

	return s.executor.InstallPackages(ctx, env.ID, conn, venvInstall)
}

func validatePackages(packages []string) ([]string, error) {
	validatedNames := make([]string, 0, len(packages))
	for _, pkg := range packages {
		pn, err := execution.NewPackageName(pkg)
		if err != nil {
			return nil, fmt.Errorf("invalid package name: %w", err)
		}
		validatedNames = append(validatedNames, pn.String())
	}
	return validatedNames, nil
}

func (s *ExecutionService) createPythonVenv(
	ctx context.Context,
	envID string,
	conn execution.ConnInfo,
	pythonCmd string,
	venvPath string,
) (*execution.ExecResult, error) {
	setup := fmt.Sprintf("%s -m venv %s", pythonCmd, venvPath)
	return s.executor.ExecuteCode(ctx, envID, conn, &execution.CodeExecution{
		Language:      environment.RuntimeShell.String(),
		Code:          setup,
		TimeoutMs:     s.config.DefaultExecTimeout.Milliseconds(),
		ExecCommand:   defaultShellCommand,
		FileExtension: environment.RuntimeShell.FileExtension(),
	})
}

func isPep668(result *execution.ExecResult) bool {
	if result == nil || result.ExitCode == 0 {
		return false
	}
	combined := strings.ToLower(result.Stdout + "\n" + result.Stderr)
	return strings.Contains(combined, "externally-managed-environment") ||
		strings.Contains(combined, "externally managed environment") ||
		strings.Contains(combined, "pep 668")
}

func installCommandFromCapabilities(env *environment.Environment) string {
	if !env.CapabilitiesDetected {
		return ""
	}

	switch env.Runtime {
	case environment.RuntimePython:
		return pythonInstallCommand(env)
	case environment.RuntimeNode:
		return nodeInstallCommand(env)
	case environment.RuntimeShell:
		return shellInstallCommand(env)
	default:
		return ""
	}
}

func pythonInstallCommand(env *environment.Environment) string {
	if env.Capabilities.PipCommand == "" {
		return ""
	}
	if !isSafePathCommand(env.Capabilities.PipCommand) {
		slog.Warn("ignoring unsafe probed pip command", "runtime", env.Runtime, "command", env.Capabilities.PipCommand)
		return ""
	}
	return fmt.Sprintf("%s install", env.Capabilities.PipCommand)
}

func nodeInstallCommand(env *environment.Environment) string {
	if env.Capabilities.NpmCommand == "" {
		return ""
	}
	if !isSafePathCommand(env.Capabilities.NpmCommand) {
		slog.Warn("ignoring unsafe probed npm command", "runtime", env.Runtime, "command", env.Capabilities.NpmCommand)
		return ""
	}
	return fmt.Sprintf("%s install -g", env.Capabilities.NpmCommand)
}

func shellInstallCommand(env *environment.Environment) string {
	if env.Capabilities.HasApk {
		return "apk add --no-cache"
	}
	if env.Capabilities.HasAptGet {
		return "apt-get update && apt-get install -y"
	}
	if env.Capabilities.HasDnf {
		return "dnf install -y"
	}
	if env.Capabilities.HasYum {
		return "yum install -y"
	}
	if env.Capabilities.HasZypper {
		return "zypper -n install"
	}
	return ""
}

func fallbackExecCommand(rt environment.Runtime) string {
	switch rt {
	case environment.RuntimePython:
		return defaultPythonCommand
	case environment.RuntimeNode:
		return defaultNodeCommand
	case environment.RuntimeShell:
		return defaultShellCommand
	default:
		return ""
	}
}

func fallbackInstallCommand(rt environment.Runtime) string {
	switch rt {
	case environment.RuntimePython:
		return defaultPipInstall
	case environment.RuntimeNode:
		return defaultNpmInstall
	case environment.RuntimeShell:
		return defaultShellInstall
	default:
		return ""
	}
}

func isSafePathCommand(cmd string) bool {
	if cmd == "" {
		return false
	}
	if !strings.HasPrefix(cmd, "/") {
		return false
	}
	for _, r := range cmd {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '/', r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func isSafeInstallCommand(cmd string) bool {
	if cmd == "" {
		return false
	}
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	if !isSafePathCommand(parts[0]) {
		return false
	}
	for _, part := range parts[1:] {
		for _, r := range part {
			switch {
			case r >= 'a' && r <= 'z':
			case r >= 'A' && r <= 'Z':
			case r >= '0' && r <= '9':
			case r == '/', r == '.', r == '-', r == '_', r == '=':
			default:
				return false
			}
		}
	}
	return true
}

// connInfo resolves pre-validated SSH connection info for an environment.
func (s *ExecutionService) connInfo(envID string, sshPort uint16) (execution.ConnInfo, error) {
	keyPath := s.provider.SSHKeyPath(envID)
	if keyPath == "" {
		return execution.ConnInfo{}, fmt.Errorf("SSH key not found for environment %q", envID)
	}
	return execution.ConnInfo{
		Host:    "127.0.0.1",
		Port:    sshPort,
		KeyPath: keyPath,
	}, nil
}
