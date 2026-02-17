// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"
	"time"
)

// ExecResult contains the outcome of a command execution in an environment.
type ExecResult struct {
	// Stdout is the standard output captured from the command.
	Stdout string

	// Stderr is the standard error output captured from the command.
	Stderr string

	// ExitCode is the exit status of the command. Zero indicates success.
	ExitCode int

	// DurationMs is the wall-clock execution time in milliseconds.
	DurationMs int64
}

// Succeeded returns true if the command exited with status 0.
func (r *ExecResult) Succeeded() bool {
	return r.ExitCode == 0
}

// Executor runs commands in an environment identified by its ID.
type Executor interface {
	// Execute runs a command string in the specified environment.
	// The command is executed via SSH in the environment's microVM.
	// The timeout limits how long the command can run.
	Execute(ctx context.Context, envID string, command string, timeout time.Duration) (*ExecResult, error)
}
