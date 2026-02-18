// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/stacklok/waggle/pkg/config"
	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/execution"
	"github.com/stacklok/waggle/pkg/infra/vm"
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
	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return nil, err
	}
	if !env.IsRunning() {
		return nil, environment.ErrNotRunning
	}

	// Touch the environment to reset inactivity timer.
	_ = s.envSvc.Touch(ctx, envID)

	// Determine runtime for this execution.
	rt := env.Runtime
	if language != "" {
		parsed, parseErr := environment.ParseRuntime(language)
		if parseErr != nil {
			return nil, parseErr
		}
		rt = parsed
	}

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
		ExecCommand:   rt.ExecCommand(),
		FileExtension: rt.FileExtension(),
	}

	return s.executor.ExecuteCode(ctx, envID, conn, req)
}

// InstallPackages installs language packages in the specified environment.
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

	_ = s.envSvc.Touch(ctx, envID)

	if len(packages) == 0 {
		return &execution.ExecResult{ExitCode: 0}, nil
	}

	// Resolve pre-validated connection info.
	conn, connErr := s.connInfo(envID, env.SSHPort)
	if connErr != nil {
		return nil, connErr
	}

	req := &execution.PackageInstallation{
		Language:       env.Runtime.String(),
		Packages:       packages,
		InstallCommand: env.Runtime.PackageInstallCommand(),
	}

	return s.executor.InstallPackages(ctx, envID, conn, req)
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
