// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"sync"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

// MemoryStore is an in-memory implementation of environment.Repository.
// It is safe for concurrent use.
type MemoryStore struct {
	mu   sync.RWMutex
	envs map[string]*environment.Environment
}

// NewMemoryStore creates a new empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		envs: make(map[string]*environment.Environment),
	}
}

// Save persists an environment. If an environment with the same ID
// already exists, it is updated.
func (s *MemoryStore) Save(_ context.Context, env *environment.Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy to prevent external mutation.
	cp := *env
	s.envs[env.ID] = &cp
	return nil
}

// FindByID retrieves an environment by its unique identifier.
func (s *MemoryStore) FindByID(_ context.Context, id string) (*environment.Environment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	env, ok := s.envs[id]
	if !ok {
		return nil, environment.ErrNotFound
	}

	// Return a copy to prevent external mutation.
	cp := *env
	return &cp, nil
}

// FindAll returns all environments currently stored.
func (s *MemoryStore) FindAll(_ context.Context) ([]*environment.Environment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*environment.Environment, 0, len(s.envs))
	for _, env := range s.envs {
		cp := *env
		result = append(result, &cp)
	}
	return result, nil
}

// Delete removes an environment by its ID.
func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.envs[id]; !ok {
		return environment.ErrNotFound
	}

	delete(s.envs, id)
	return nil
}

// Count returns the number of environments currently stored.
func (s *MemoryStore) Count(_ context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.envs), nil
}
