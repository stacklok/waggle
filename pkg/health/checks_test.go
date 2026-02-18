// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"testing"

	"github.com/stacklok/waggle/pkg/infra/store"
)

func TestRepositoryCheckerName(t *testing.T) {
	t.Parallel()

	c := NewRepositoryChecker(store.NewMemoryStore())
	if got := c.Name(); got != "repository" {
		t.Errorf("Name() = %q, want %q", got, "repository")
	}
}

func TestRepositoryCheckerHealthy(t *testing.T) {
	t.Parallel()

	c := NewRepositoryChecker(store.NewMemoryStore())
	if err := c.Check(context.Background()); err != nil {
		t.Errorf("Check() = %v, want nil", err)
	}
}
