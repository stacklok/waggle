// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	propolisssh "github.com/stacklok/propolis/ssh"

	"github.com/stacklok/waggle/pkg/domain/execution"
)

// Executor implements execution.Executor using SSH connections to microVMs.
type Executor struct{}

// NewExecutor creates a new SSH-based Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// ExecuteCode runs code in the specified environment via SSH.
// It writes the code to a temp file, executes it, captures output, and cleans up.
func (e *Executor) ExecuteCode(
	ctx context.Context, _ string, conn execution.ConnInfo, req *execution.CodeExecution,
) (*execution.ExecResult, error) {
	client := propolisssh.NewClient(conn.Host, conn.Port, "root", conn.KeyPath)

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond

	// Build execution command: write code to temp file, execute, clean up.
	tempFile := fmt.Sprintf("/tmp/waggle_%s%s", uuid.New().String()[:12], req.FileExtension)
	delimiter := fmt.Sprintf("WAGGLE_EOF_%s", uuid.New().String()[:8])
	command := fmt.Sprintf(
		"cat > %s << '%s'\n%s\n%s\n%s %s; __exit=$?; rm -f %s; exit $__exit",
		tempFile, delimiter, req.Code, delimiter, req.ExecCommand, tempFile, tempFile,
	)

	return e.run(ctx, client, command, timeout)
}

// InstallPackages installs language packages in the specified environment via SSH.
func (e *Executor) InstallPackages(
	ctx context.Context, _ string, conn execution.ConnInfo, req *execution.PackageInstallation,
) (*execution.ExecResult, error) {
	client := propolisssh.NewClient(conn.Host, conn.Port, "root", conn.KeyPath)

	command := fmt.Sprintf("%s %s", req.InstallCommand, strings.Join(req.Packages, " "))

	return e.run(ctx, client, command, 0)
}

// run executes a command via SSH and returns the result.
// If timeout is 0, no timeout is applied beyond the context deadline.
func (*Executor) run(
	ctx context.Context, client *propolisssh.Client, command string, timeout time.Duration,
) (*execution.ExecResult, error) {
	execCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var stdout, stderr bytes.Buffer
	start := time.Now()

	err := client.RunStream(execCtx, command, &stdout, &stderr)

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
