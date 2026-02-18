// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"testing"

	"github.com/stacklok/waggle/pkg/domain/execution"
)

func TestNewPackageName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid names.
		{name: "simple name", input: "requests", wantErr: false},
		{name: "hyphenated name", input: "my-package", wantErr: false},
		{name: "underscored name", input: "my_package", wantErr: false},
		{name: "dotted name", input: "my.package", wantErr: false},
		{name: "version with ==", input: "requests==2.31.0", wantErr: false},
		{name: "version with >=", input: "flask>=2.0", wantErr: false},
		{name: "version with ~=", input: "numpy~=1.24", wantErr: false},
		{name: "scoped npm package", input: "express@4.18.2", wantErr: false},
		{name: "alphanumeric with digits", input: "package2", wantErr: false},

		// Invalid — injection characters.
		{name: "semicolon injection", input: "pkg; rm -rf /", wantErr: true},
		{name: "pipe injection", input: "pkg|cat /etc/passwd", wantErr: true},
		{name: "backtick injection", input: "pkg`id`", wantErr: true},
		{name: "dollar subshell injection", input: "pkg$(id)", wantErr: true},
		{name: "double ampersand injection", input: "pkg&&id", wantErr: true},
		{name: "newline injection", input: "pkg\nid", wantErr: true},
		{name: "space injection", input: "pkg name", wantErr: true},
		{name: "empty name", input: "", wantErr: true},
		{name: "leading dash", input: "-e /etc/passwd", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg, err := execution.NewPackageName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewPackageName(%q) expected error, got nil (value=%q)", tt.input, pkg.String())
				}
				return
			}
			if err != nil {
				t.Errorf("NewPackageName(%q) unexpected error: %v", tt.input, err)
				return
			}
			if pkg.String() != tt.input {
				t.Errorf("PackageName.String() = %q, want %q", pkg.String(), tt.input)
			}
		})
	}
}
