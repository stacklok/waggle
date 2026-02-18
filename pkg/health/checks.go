// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"fmt"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

// RepositoryChecker verifies that the environment repository is accessible.
type RepositoryChecker struct {
	repo environment.Repository
}

// NewRepositoryChecker creates a checker for the given repository.
func NewRepositoryChecker(repo environment.Repository) *RepositoryChecker {
	return &RepositoryChecker{repo: repo}
}

// Name returns the checker's identifier.
func (*RepositoryChecker) Name() string {
	return "repository"
}

// Check verifies the repository is accessible by calling Count.
func (c *RepositoryChecker) Check(ctx context.Context) error {
	if _, err := c.repo.Count(ctx); err != nil {
		return fmt.Errorf("repository check: %w", err)
	}
	return nil
}
