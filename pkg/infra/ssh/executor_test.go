// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"errors"
	"strings"
	"testing"
)

func TestExtractExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error",
			err:  nil,
			want: 0,
		},
		{
			name: "process exited with status",
			err:  errors.New("ssh command \"false\": Process exited with status 1"),
			want: 1,
		},
		{
			name: "exit status",
			err:  errors.New("exit status 137"),
			want: 137,
		},
		{
			name: "exit status 2",
			err:  errors.New("ssh stream command \"bad\": exit status 2"),
			want: 2,
		},
		{
			name: "unrelated error",
			err:  errors.New("connection refused"),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractExitCode(tt.err)
			if got != tt.want {
				t.Errorf("extractExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// TestBuildInstallCommandInjection verifies that dangerous characters in package
// names are shell-escaped and cannot be interpreted as shell metacharacters.
func TestBuildInstallCommandInjection(t *testing.T) {
	t.Parallel()

	// Each input represents a package name containing a dangerous character.
	// After shell-escaping, the resulting command must not contain the raw
	// metacharacter outside of single quotes.
	tests := []struct {
		name      string
		pkg       string
		dangerous string // raw metacharacter that must not appear unquoted
	}{
		{name: "semicolon", pkg: "pkg; rm -rf /", dangerous: ";"},
		{name: "pipe", pkg: "pkg|cat /etc/passwd", dangerous: "|"},
		{name: "backtick", pkg: "pkg`id`", dangerous: "`"},
		{name: "dollar subshell", pkg: "pkg$(id)", dangerous: "$("},
		{name: "double ampersand", pkg: "pkg&&id", dangerous: "&&"},
		{name: "newline", pkg: "pkg\nid", dangerous: "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := buildInstallCommand("pip install", []string{tt.pkg})

			// The command must start with the install prefix.
			if !strings.HasPrefix(cmd, "pip install ") {
				t.Errorf("buildInstallCommand(%q) = %q: missing install prefix", tt.pkg, cmd)
			}

			// The dangerous metacharacter must not appear unquoted.
			// Shell escaping wraps the value in single quotes; any single quote
			// inside the value is itself escaped. So the raw metacharacter (if
			// present) will be inside single quotes and not interpreted by the shell.
			// We verify this by confirming the raw metacharacter does not appear
			// outside of single-quoted regions in the suffix.
			suffix := strings.TrimPrefix(cmd, "pip install ")
			if containsUnquoted(suffix, tt.dangerous) {
				t.Errorf(
					"buildInstallCommand(%q) = %q: dangerous string %q appears unquoted in command",
					tt.pkg, cmd, tt.dangerous,
				)
			}
		})
	}
}

// containsUnquoted reports whether s contains the substring sub outside
// single-quoted regions (i.e., where it could be interpreted by the shell).
func containsUnquoted(s, sub string) bool {
	inQuotes := false
	for i := 0; i < len(s); {
		if s[i] == '\'' {
			inQuotes = !inQuotes
			i++
			continue
		}
		if !inQuotes && strings.HasPrefix(s[i:], sub) {
			return true
		}
		i++
	}
	return false
}
