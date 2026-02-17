// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stacklok/waggle/pkg/config"
	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/execution"
)

// ExecutionService orchestrates code execution within environments.
type ExecutionService struct {
	repo     environment.Repository
	executor execution.Executor
	envSvc   *EnvironmentService
	config   *config.Config
}

// NewExecutionService creates a new ExecutionService.
func NewExecutionService(
	repo environment.Repository,
	executor execution.Executor,
	envSvc *EnvironmentService,
	cfg *config.Config,
) *ExecutionService {
	return &ExecutionService{
		repo:     repo,
		executor: executor,
		envSvc:   envSvc,
		config:   cfg,
	}
}

// Execute runs code in the specified environment. It writes the code to a
// temp file in the VM, executes it, captures output, and cleans up.
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

	// Build the execution command: write code to temp file, execute, clean up.
	tempFile := fmt.Sprintf("/tmp/waggle_%s%s", uuid.New().String()[:12], rt.FileExtension())
	execCmd := rt.ExecCommand()

	// Combined command: write code via heredoc, execute, clean up.
	// Using a unique delimiter to avoid conflicts with user code.
	delimiter := fmt.Sprintf("WAGGLE_EOF_%s", uuid.New().String()[:8])
	command := fmt.Sprintf(
		"cat > %s << '%s'\n%s\n%s\n%s %s; __exit=$?; rm -f %s; exit $__exit",
		tempFile, delimiter, code, delimiter, execCmd, tempFile, tempFile,
	)

	return s.executor.Execute(ctx, envID, command, timeout)
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

	command := fmt.Sprintf("%s %s", env.Runtime.PackageInstallCommand(), strings.Join(packages, " "))
	return s.executor.Execute(ctx, envID, command, s.config.MaxExecTimeout)
}
