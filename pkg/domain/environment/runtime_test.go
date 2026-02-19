// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package environment

import (
	"errors"
	"testing"
)

func TestRuntimeValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime Runtime
		wantErr bool
	}{
		{name: "python is valid", runtime: RuntimePython, wantErr: false},
		{name: "node is valid", runtime: RuntimeNode, wantErr: false},
		{name: "shell is valid", runtime: RuntimeShell, wantErr: false},
		{name: "empty is invalid", runtime: "", wantErr: true},
		{name: "unknown is invalid", runtime: "ruby", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.runtime.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error for runtime %q, got nil", tt.runtime)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for runtime %q: %v", tt.runtime, err)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidRuntime) {
				t.Errorf("expected ErrInvalidRuntime, got: %v", err)
			}
		})
	}
}

func TestRuntimeFileExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		runtime Runtime
		want    string
	}{
		{RuntimePython, ".py"},
		{RuntimeNode, ".js"},
		{RuntimeShell, ".sh"},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			t.Parallel()
			if got := tt.runtime.FileExtension(); got != tt.want {
				t.Errorf("FileExtension() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimeExecCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		runtime Runtime
		want    string
	}{
		{RuntimePython, "/opt/waggle-venv/bin/python"},
		{RuntimeNode, "node"},
		{RuntimeShell, "sh"},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			t.Parallel()
			if got := tt.runtime.ExecCommand(); got != tt.want {
				t.Errorf("ExecCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimePackageInstallCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		runtime Runtime
		want    string
	}{
		{RuntimePython, "/opt/waggle-venv/bin/pip install"},
		{RuntimeNode, "npm install -g"},
		{RuntimeShell, "apk add --no-cache"},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			t.Parallel()
			if got := tt.runtime.PackageInstallCommand(); got != tt.want {
				t.Errorf("PackageInstallCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    Runtime
		wantErr bool
	}{
		{name: "python", input: "python", want: RuntimePython},
		{name: "node", input: "node", want: RuntimeNode},
		{name: "shell", input: "shell", want: RuntimeShell},
		{name: "invalid", input: "go", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRuntime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseRuntime(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
