// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package execution

import "context"

// ConnInfo holds pre-validated SSH connection details for an environment.
// The service layer resolves these from the environment before calling adapters.
type ConnInfo struct {
	// Host is the SSH host address (typically "127.0.0.1" for microVMs).
	Host string

	// Port is the SSH port for the environment's microVM.
	Port uint16

	// KeyPath is the path to the SSH private key for this environment.
	KeyPath string
}

// CodeExecution represents a request to execute code in an environment.
type CodeExecution struct {
	// Language is the runtime language identifier (e.g., "python", "node").
	Language string

	// Code is the source code to execute.
	Code string

	// TimeoutMs is the execution timeout in milliseconds. Zero means no timeout.
	TimeoutMs int64

	// ExecCommand is the runtime command used to execute the temp file (e.g., "python3").
	// Resolved by the service layer from the environment's runtime.
	ExecCommand string

	// FileExtension is the file extension for the temp file (e.g., ".py").
	// Resolved by the service layer from the environment's runtime.
	FileExtension string
}

// PackageInstallation represents a request to install packages.
type PackageInstallation struct {
	// Language is the runtime language identifier (e.g., "python", "node").
	Language string

	// Packages is the list of package names to install.
	Packages []string

	// InstallCommand is the package manager install command (e.g., "pip install").
	// Resolved by the service layer from the environment's runtime.
	InstallCommand string
}

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
	// ExecuteCode runs code in the specified environment.
	// The code is written to a temp file in the VM, executed, and cleaned up.
	// conn contains pre-validated SSH connection details resolved by the service layer.
	ExecuteCode(ctx context.Context, envID string, conn ConnInfo, req *CodeExecution) (*ExecResult, error)

	// InstallPackages installs language packages in the specified environment.
	// conn contains pre-validated SSH connection details resolved by the service layer.
	InstallPackages(ctx context.Context, envID string, conn ConnInfo, req *PackageInstallation) (*ExecResult, error)
}
