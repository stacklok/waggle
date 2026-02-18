// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stacklok/waggle/pkg/config"
	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/infra/store"
	"github.com/stacklok/waggle/pkg/infra/vm"
)

// fakeProvider implements vm.Provider for testing.
type fakeProvider struct {
	createErr error
	vms       map[string]*vm.Handle
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{vms: make(map[string]*vm.Handle)}
}

func (f *fakeProvider) CreateVM(_ context.Context, env *environment.Environment, _ vm.CreateVMOpts) (*vm.Handle, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
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

func TestEnvironmentServiceCreate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil }) // skip real listen
	cfg := testConfig()

	svc := NewEnvironmentService(repo, provider, portAlloc, cfg)

	env, err := svc.Create(ctx, environment.RuntimePython, "test-env", 30)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if env.Name != "test-env" {
		t.Errorf("Name = %q, want %q", env.Name, "test-env")
	}
	if env.Runtime != environment.RuntimePython {
		t.Errorf("Runtime = %q, want %q", env.Runtime, environment.RuntimePython)
	}
	if env.Status != environment.StatusRunning {
		t.Errorf("Status = %q, want %q", env.Status, environment.StatusRunning)
	}
	if env.SSHPort < 20000 || env.SSHPort >= 20100 {
		t.Errorf("SSHPort = %d, outside expected range", env.SSHPort)
	}
	if env.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", env.Timeout)
	}
}

func TestEnvironmentServiceCreateAutoName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })

	svc := NewEnvironmentService(repo, provider, portAlloc, testConfig())

	env, err := svc.Create(ctx, environment.RuntimeNode, "", 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if env.Name == "" {
		t.Error("auto-generated name should not be empty")
	}
	if env.Timeout != time.Duration(testConfig().DefaultTimeoutMin)*time.Minute {
		t.Errorf("Timeout = %v, want default", env.Timeout)
	}
}

func TestEnvironmentServiceCreateInvalidRuntime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc := NewEnvironmentService(store.NewMemoryStore(), newFakeProvider(),
		vm.NewPortAllocator(20000, 20100), testConfig())

	_, err := svc.Create(ctx, "ruby", "test", 30)
	if !errors.Is(err, environment.ErrInvalidRuntime) {
		t.Errorf("err = %v, want ErrInvalidRuntime", err)
	}
}

func TestEnvironmentServiceCreateTimeoutTooLarge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })

	svc := NewEnvironmentService(store.NewMemoryStore(), newFakeProvider(), portAlloc, testConfig())

	_, err := svc.Create(ctx, environment.RuntimePython, "test", MaxTimeoutMinutes+1)
	if err == nil {
		t.Fatal("expected error for timeout exceeding max, got nil")
	}
}

func TestEnvironmentServiceCreateTimeoutAtMax(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })

	svc := NewEnvironmentService(store.NewMemoryStore(), newFakeProvider(), portAlloc, testConfig())

	env, err := svc.Create(ctx, environment.RuntimePython, "test", MaxTimeoutMinutes)
	if err != nil {
		t.Fatalf("expected no error at max timeout, got: %v", err)
	}
	if env.Timeout != time.Duration(MaxTimeoutMinutes)*time.Minute {
		t.Errorf("Timeout = %v, want %v", env.Timeout, time.Duration(MaxTimeoutMinutes)*time.Minute)
	}
}

func TestEnvironmentServiceCreateMaxReached(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := testConfig()
	cfg.MaxEnvironments = 1

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })

	svc := NewEnvironmentService(repo, provider, portAlloc, cfg)

	// Create first environment.
	_, err := svc.Create(ctx, environment.RuntimePython, "first", 30)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// Second should fail.
	_, err = svc.Create(ctx, environment.RuntimePython, "second", 30)
	if !errors.Is(err, environment.ErrMaxEnvironments) {
		t.Errorf("err = %v, want ErrMaxEnvironments", err)
	}
}

func TestEnvironmentServiceCreateVMFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	provider := newFakeProvider()
	provider.createErr = errors.New("boot failed")

	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })

	svc := NewEnvironmentService(store.NewMemoryStore(), provider, portAlloc, testConfig())

	_, err := svc.Create(ctx, environment.RuntimePython, "test", 30)
	if err == nil {
		t.Fatal("expected error on VM failure")
	}

	// Port should be released on failure.
	if portAlloc.AllocatedCount() != 0 {
		t.Errorf("port not released after failure: %d allocated", portAlloc.AllocatedCount())
	}
}

func TestEnvironmentServiceDestroy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })

	svc := NewEnvironmentService(repo, provider, portAlloc, testConfig())

	env, err := svc.Create(ctx, environment.RuntimeShell, "to-destroy", 30)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Destroy(ctx, env.ID); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	// Should be gone from store.
	_, err = repo.FindByID(ctx, env.ID)
	if !errors.Is(err, environment.ErrNotFound) {
		t.Errorf("after destroy, FindByID err = %v, want ErrNotFound", err)
	}

	// Port should be released.
	if portAlloc.AllocatedCount() != 0 {
		t.Errorf("port not released: %d allocated", portAlloc.AllocatedCount())
	}
}

func TestEnvironmentServiceList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })

	svc := NewEnvironmentService(repo, provider, portAlloc, testConfig())

	_, _ = svc.Create(ctx, environment.RuntimePython, "env-1", 30)
	_, _ = svc.Create(ctx, environment.RuntimeNode, "env-2", 30)

	envs, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(envs) != 2 {
		t.Errorf("List returned %d envs, want 2", len(envs))
	}
}

func TestEnvironmentServiceTouch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })

	svc := NewEnvironmentService(repo, provider, portAlloc, testConfig())

	env, _ := svc.Create(ctx, environment.RuntimePython, "touchable", 30)
	originalLastUsed := env.LastUsed

	time.Sleep(time.Millisecond)
	if err := svc.Touch(ctx, env.ID); err != nil {
		t.Fatalf("Touch: %v", err)
	}

	updated, _ := svc.Get(ctx, env.ID)
	if !updated.LastUsed.After(originalLastUsed) {
		t.Error("Touch should update LastUsed")
	}
}
