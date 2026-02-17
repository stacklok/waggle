// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	propolisssh "github.com/stacklok/propolis/ssh"

	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/execution"
	"github.com/stacklok/waggle/pkg/infra/vm"
)

// Executor implements execution.Executor using SSH connections to microVMs.
type Executor struct {
	repo     environment.Repository
	provider vm.VMProvider
}

// NewExecutor creates a new SSH-based Executor.
func NewExecutor(repo environment.Repository, provider vm.VMProvider) *Executor {
	return &Executor{
		repo:     repo,
		provider: provider,
	}
}

// Execute runs a command in the specified environment via SSH.
// It captures stdout and stderr separately and returns the result.
func (e *Executor) Execute(
	ctx context.Context, envID string, command string, timeout time.Duration,
) (*execution.ExecResult, error) {
	env, err := e.repo.FindByID(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("find environment: %w", err)
	}
	if !env.IsRunning() {
		return nil, environment.ErrNotRunning
	}

	keyPath := e.provider.SSHKeyPath(envID)
	if keyPath == "" {
		return nil, fmt.Errorf("SSH key not found for environment %q", envID)
	}

	client := propolisssh.NewClient("127.0.0.1", env.SSHPort, "root", keyPath)

	// Apply timeout via context.
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	start := time.Now()

	err = client.RunStream(execCtx, command, &stdout, &stderr)

	duration := time.Since(start)
	exitCode := 0

	if err != nil {
		exitCode = extractExitCode(err)
		// If the exit code is non-zero, it's a normal command failure,
		// not an SSH error. Clear the error so we return the result.
		if exitCode != 0 {
			err = nil
		}
	}

	result := &execution.ExecResult{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode,
		DurationMs: duration.Milliseconds(),
	}

	if err != nil {
		return result, fmt.Errorf("ssh execute: %w", err)
	}

	return result, nil
}

// extractExitCode attempts to extract a numeric exit code from an SSH
// error message. Returns 0 if no exit code is found (indicating an
// infrastructure error rather than a command failure).
func extractExitCode(err error) int {
	if err == nil {
		return 0
	}

	msg := err.Error()

	// golang.org/x/crypto/ssh returns errors like "Process exited with status N"
	// or "exit status N".
	for _, prefix := range []string{
		"Process exited with status ",
		"exit status ",
	} {
		if idx := strings.Index(msg, prefix); idx >= 0 {
			codeStr := strings.TrimSpace(msg[idx+len(prefix):])
			// Parse the first integer.
			var code int
			if _, err := fmt.Sscanf(codeStr, "%d", &code); err == nil {
				return code
			}
		}
	}

	return 0
}
