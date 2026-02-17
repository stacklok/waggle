// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

func newTestEnv(id, name string) *environment.Environment {
	return environment.New(id, name, environment.RuntimePython, 10000, 30*time.Minute)
}

func TestMemoryStoreSaveAndFindByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStore()

	env := newTestEnv("id-1", "test-env")
	if err := s.Save(ctx, env); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.FindByID(ctx, "id-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.ID != "id-1" {
		t.Errorf("ID = %q, want %q", got.ID, "id-1")
	}
	if got.Name != "test-env" {
		t.Errorf("Name = %q, want %q", got.Name, "test-env")
	}
}

func TestMemoryStoreFindByIDNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStore()

	_, err := s.FindByID(ctx, "nonexistent")
	if !errors.Is(err, environment.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreSaveUpdatesExisting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStore()

	env := newTestEnv("id-1", "original")
	if err := s.Save(ctx, env); err != nil {
		t.Fatalf("Save: %v", err)
	}

	env.Name = "updated"
	if err := s.Save(ctx, env); err != nil {
		t.Fatalf("Save update: %v", err)
	}

	got, err := s.FindByID(ctx, "id-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name != "updated" {
		t.Errorf("Name = %q, want %q", got.Name, "updated")
	}

	// Count should still be 1 (not 2).
	count, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestMemoryStoreFindAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStore()

	// Empty store returns empty slice.
	all, err := s.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll empty: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("FindAll empty = %d items, want 0", len(all))
	}

	// Add two environments.
	if err := s.Save(ctx, newTestEnv("id-1", "env-1")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save(ctx, newTestEnv("id-2", "env-2")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	all, err = s.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("FindAll = %d items, want 2", len(all))
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStore()

	env := newTestEnv("id-1", "test-env")
	if err := s.Save(ctx, env); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := s.Delete(ctx, "id-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.FindByID(ctx, "id-1")
	if !errors.Is(err, environment.ErrNotFound) {
		t.Errorf("after delete, FindByID err = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreDeleteNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStore()

	err := s.Delete(ctx, "nonexistent")
	if !errors.Is(err, environment.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreCount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStore()

	count, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("initial Count = %d, want 0", count)
	}

	if err := s.Save(ctx, newTestEnv("id-1", "env-1")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	count, err = s.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestMemoryStoreCopyIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStore()

	env := newTestEnv("id-1", "original")
	if err := s.Save(ctx, env); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Mutate the original after saving.
	env.Name = "mutated"

	// The stored copy should not be affected.
	got, err := s.FindByID(ctx, "id-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name != "original" {
		t.Errorf("Name = %q, want %q (store should hold a copy)", got.Name, "original")
	}

	// Mutate the returned copy.
	got.Name = "also-mutated"

	// The stored copy should still not be affected.
	got2, err := s.FindByID(ctx, "id-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got2.Name != "original" {
		t.Errorf("Name = %q, want %q (returned value should be a copy)", got2.Name, "original")
	}
}

func TestMemoryStoreConcurrentAccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStore()

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent writes.
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			env := newTestEnv(
				"id-"+strconv.Itoa(i),
				"env-"+strconv.Itoa(i),
			)
			_ = s.Save(ctx, env)
		}(i)
	}
	wg.Wait()

	// Verify count.
	count, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != goroutines {
		t.Errorf("Count = %d, want %d", count, goroutines)
	}

	// Concurrent reads.
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = s.FindByID(ctx, "id-"+strconv.Itoa(i))
		}(i)
	}
	wg.Wait()
}
