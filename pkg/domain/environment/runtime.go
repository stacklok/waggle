// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package environment

import "fmt"

// Runtime represents the language runtime for an execution environment.
type Runtime string

const (
	// RuntimePython is a Python execution environment.
	RuntimePython Runtime = "python"

	// RuntimeNode is a Node.js execution environment.
	RuntimeNode Runtime = "node"

	// RuntimeShell is a generic shell execution environment.
	RuntimeShell Runtime = "shell"
)

// ValidRuntimes contains all supported runtime values.
var ValidRuntimes = []Runtime{RuntimePython, RuntimeNode, RuntimeShell}

// Validate checks whether the runtime is a supported value.
func (r Runtime) Validate() error {
	switch r {
	case RuntimePython, RuntimeNode, RuntimeShell:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidRuntime, r)
	}
}

// FileExtension returns the file extension used for code files in this runtime.
func (r Runtime) FileExtension() string {
	switch r {
	case RuntimePython:
		return ".py"
	case RuntimeNode:
		return ".js"
	case RuntimeShell:
		return ".sh"
	default:
		return ".sh"
	}
}

// ExecCommand returns the command prefix used to execute a code file
// in this runtime.
func (r Runtime) ExecCommand() string {
	switch r {
	case RuntimePython:
		return "/opt/waggle-venv/bin/python"
	case RuntimeNode:
		return "node"
	case RuntimeShell:
		return "sh"
	default:
		return "sh"
	}
}

// PackageInstallCommand returns the command used to install packages
// for this runtime. The packages argument is appended to the returned command.
func (r Runtime) PackageInstallCommand() string {
	switch r {
	case RuntimePython:
		return "/opt/waggle-venv/bin/pip install"
	case RuntimeNode:
		return "npm install -g"
	case RuntimeShell:
		return "apk add --no-cache"
	default:
		return "apk add --no-cache"
	}
}

// String returns the string representation of the runtime.
func (r Runtime) String() string {
	return string(r)
}

// ParseRuntime converts a string to a Runtime, returning an error if invalid.
func ParseRuntime(s string) (Runtime, error) {
	r := Runtime(s)
	if err := r.Validate(); err != nil {
		return "", err
	}
	return r, nil
}
