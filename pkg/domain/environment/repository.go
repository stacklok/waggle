// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package environment

import "context"

// Repository defines storage operations for Environment aggregates.
type Repository interface {
	// Save persists an environment. If an environment with the same ID
	// already exists, it is updated.
	Save(ctx context.Context, env *Environment) error

	// FindByID retrieves an environment by its unique identifier.
	// Returns ErrNotFound if the environment does not exist.
	FindByID(ctx context.Context, id string) (*Environment, error)

	// FindAll returns all environments currently stored.
	FindAll(ctx context.Context) ([]*Environment, error)

	// Delete removes an environment by its ID.
	// Returns ErrNotFound if the environment does not exist.
	Delete(ctx context.Context, id string) error

	// Count returns the number of environments currently stored.
	Count(ctx context.Context) (int, error)
}
