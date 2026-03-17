// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	microvmssh "github.com/stacklok/go-microvm/ssh"

	"github.com/stacklok/waggle/pkg/domain/execution"
)

// Executor implements execution.Executor using SSH connections to microVMs.
type Executor struct{}

// NewExecutor creates a new SSH-based Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// ExecuteCode runs code in the specified environment via SSH.
// It writes the code to a temp file via base64 decoding (so raw user code never
// appears in the shell command string), executes it, captures output, and cleans up.
func (e *Executor) ExecuteCode(
	ctx context.Context, _ string, conn execution.ConnInfo, req *execution.CodeExecution,
) (*execution.ExecResult, error) {
	client := microvmssh.NewClient(conn.Host, conn.Port, "sandbox", conn.KeyPath)

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond

	// Base64-encode user code so it never appears raw in the shell command.
	encoded := base64.StdEncoding.EncodeToString([]byte(req.Code))

	// Build the temp file path and shell-escape it for safe interpolation.
	tempFile := fmt.Sprintf("/tmp/waggle_%s%s", uuid.New().String()[:12], req.FileExtension)
	escapedTempFile := microvmssh.ShellEscape(tempFile)

	// Write code via base64 decode into the temp file, then execute and clean up.
	// Using printf | base64 -d avoids heredoc quoting issues entirely.
	command := fmt.Sprintf(
		"printf '%%s' %s | base64 -d > %s; %s %s; __exit=$?; rm -f %s; exit $__exit",
		microvmssh.ShellEscape(encoded),
		escapedTempFile,
		req.ExecCommand,
		escapedTempFile,
		escapedTempFile,
	)

	return e.run(ctx, client, command, timeout)
}

// InstallPackages installs language packages in the specified environment via SSH.
// Each package name is shell-escaped before being included in the command.
func (e *Executor) InstallPackages(
	ctx context.Context, _ string, conn execution.ConnInfo, req *execution.PackageInstallation,
) (*execution.ExecResult, error) {
	client := microvmssh.NewClient(conn.Host, conn.Port, "sandbox", conn.KeyPath)
	command := buildInstallCommand(req.InstallCommand, req.Packages)
	return e.run(ctx, client, command, 0)
}

// buildInstallCommand constructs the shell command for installing packages.
// Each package name is shell-escaped to prevent command injection.
func buildInstallCommand(installCmd string, packages []string) string {
	escapedPkgs := make([]string, len(packages))
	for i, pkg := range packages {
		escapedPkgs[i] = microvmssh.ShellEscape(pkg)
	}
	return fmt.Sprintf("%s %s", installCmd, strings.Join(escapedPkgs, " "))
}

// run executes a command via SSH and returns the result.
// If timeout is 0, no timeout is applied beyond the context deadline.
func (*Executor) run(
	ctx context.Context, client *microvmssh.Client, command string, timeout time.Duration,
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
