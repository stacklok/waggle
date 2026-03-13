// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package vm

import (
	"testing"
)

func TestExtraPreflightChecks(t *testing.T) {
	t.Parallel()

	checks := extraPreflightChecks()

	t.Run("returns exactly one check", func(t *testing.T) {
		t.Parallel()
		if got := len(checks); got != 1 {
			t.Fatalf("expected 1 check, got %d", got)
		}
	})

	t.Run("check is named userns", func(t *testing.T) {
		t.Parallel()
		if got := checks[0].Name; got != "userns" {
			t.Fatalf("expected check name %q, got %q", "userns", got)
		}
	})

	t.Run("check is required", func(t *testing.T) {
		t.Parallel()
		if !checks[0].Required {
			t.Fatal("expected userns check to be required")
		}
	})
}
