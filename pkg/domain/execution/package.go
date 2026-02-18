// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"fmt"
	"regexp"
)

// packageNameRe is the allowlist for package names. It permits alphanumeric
// characters, hyphens, dots, underscores, and common version specifier
// characters (@, >=, <=, ==, !=, ~=, >, <).
// This prevents shell injection via package names containing ;, |, `, $(), &&, etc.
var packageNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-]*([><=!~]{1,2}[a-zA-Z0-9._\-,.*]+|@[a-zA-Z0-9._\-,.*]+)?$`)

// PackageName is a validated package name value object.
// It guarantees that the underlying string matches the allowlist regex and is
// safe to use in shell commands (after additional ShellEscape in the adapter).
type PackageName struct {
	value string
}

// NewPackageName constructs a PackageName after validating against the allowlist.
// Returns an error if the name contains characters outside the allowlist.
func NewPackageName(name string) (PackageName, error) {
	if name == "" {
		return PackageName{}, fmt.Errorf("package name must not be empty")
	}
	if !packageNameRe.MatchString(name) {
		return PackageName{}, fmt.Errorf("invalid package name %q: must match %s", name, packageNameRe)
	}
	return PackageName{value: name}, nil
}

// String returns the validated package name string.
func (p PackageName) String() string {
	return p.value
}
