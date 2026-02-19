// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"context"
	"testing"
	"time"

	"github.com/stacklok/waggle/pkg/config"
	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/infra/store"
	"github.com/stacklok/waggle/pkg/infra/vm"
	"github.com/stacklok/waggle/pkg/service"
)

// fakeProvider implements vm.Provider for testing.
type fakeProvider struct {
	vms map[string]*vm.Handle
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{vms: make(map[string]*vm.Handle)}
}

func (f *fakeProvider) CreateVM(_ context.Context, env *environment.Environment, _ vm.CreateVMOpts) (*vm.Handle, error) {
	handle := &vm.Handle{EnvID: env.ID, SSHKeyPath: "/tmp/fake_key"}
	f.vms[env.ID] = handle
	return handle, nil
}

func (f *fakeProvider) DestroyVM(_ context.Context, envID string) error {
	delete(f.vms, envID)
	return nil
}

func (f *fakeProvider) IsRunning(_ context.Context, envID string) bool {
	_, ok := f.vms[envID]
	return ok
}

func (f *fakeProvider) SSHKeyPath(envID string) string {
	h, ok := f.vms[envID]
	if !ok {
		return ""
	}
	return h.SSHKeyPath
}

func testConfig() *config.Config {
	cfg := config.Default()
	cfg.Images = map[string]string{
		"python": "test-python:latest",
		"node":   "test-node:latest",
		"shell":  "test-shell:latest",
	}
	cfg.DataDir = "/tmp/waggle-test"
	return cfg
}

func TestReaperDestroysExpiredEnvironments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(30000, 30100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })
	cfg := testConfig()

	envSvc := service.NewEnvironmentService(repo, provider, portAlloc, nil, cfg)

	// Create an environment with a very short timeout.
	env, err := envSvc.Create(ctx, environment.RuntimePython, "soon-expired", 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Manually set LastUsed to the past to simulate expiration.
	stored, _ := repo.FindByID(ctx, env.ID)
	stored.LastUsed = time.Now().Add(-2 * time.Minute)
	stored.Timeout = time.Millisecond // Expire immediately.
	_ = repo.Save(ctx, stored)

	// Run the reaper sweep.
	reaper := NewReaper(envSvc, repo, time.Second)
	reaper.sweep(ctx)

	// Environment should be gone.
	count, _ := repo.Count(ctx)
	if count != 0 {
		t.Errorf("expected 0 environments after reaper, got %d", count)
	}
}

func TestReaperLeavesActiveEnvironments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(30100, 30200)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })
	cfg := testConfig()

	envSvc := service.NewEnvironmentService(repo, provider, portAlloc, nil, cfg)

	// Create an environment with a long timeout.
	_, err := envSvc.Create(ctx, environment.RuntimePython, "still-active", 60)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Run the reaper sweep.
	reaper := NewReaper(envSvc, repo, time.Second)
	reaper.sweep(ctx)

	// Environment should still exist.
	count, _ := repo.Count(ctx)
	if count != 1 {
		t.Errorf("expected 1 environment after reaper (active), got %d", count)
	}
}

func TestReaperStartStop(t *testing.T) {
	t.Parallel()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(30200, 30300)
	cfg := testConfig()

	envSvc := service.NewEnvironmentService(repo, provider, portAlloc, nil, cfg)

	reaper := NewReaper(envSvc, repo, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Start should block until context is done.
	done := make(chan struct{})
	go func() {
		reaper.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Success - reaper stopped when context was cancelled.
	case <-time.After(time.Second):
		t.Fatal("reaper did not stop within timeout")
	}
}
