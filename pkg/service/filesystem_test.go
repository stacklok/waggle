// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/filesystem"
	"github.com/stacklok/waggle/pkg/infra/store"
	"github.com/stacklok/waggle/pkg/infra/vm"
)

// fakeFS implements filesystem.FileSystem for testing.
type fakeFS struct {
	files map[string][]byte
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: make(map[string][]byte)}
}

func (f *fakeFS) WriteFile(_ context.Context, _ string, path string, content []byte, _ os.FileMode) error {
	f.files[path] = content
	return nil
}

func (f *fakeFS) ReadFile(_ context.Context, _ string, path string) ([]byte, error) {
	data, ok := f.files[path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return data, nil
}

func (f *fakeFS) ListFiles(_ context.Context, _ string, _ string) ([]filesystem.FileInfo, error) {
	return []filesystem.FileInfo{
		{Name: "test.py", Size: 100, IsDir: false},
	}, nil
}

func setupFSTest(t *testing.T) (*FilesystemService, *fakeFS, string) {
	t.Helper()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })
	cfg := testConfig()

	envSvc := NewEnvironmentService(repo, provider, portAlloc, cfg)

	env, err := envSvc.Create(ctx, environment.RuntimePython, "fs-test", 30)
	if err != nil {
		t.Fatalf("Create env: %v", err)
	}

	fs := newFakeFS()
	fsSvc := NewFilesystemService(repo, fs, envSvc)

	return fsSvc, fs, env.ID
}

func TestFilesystemServiceWriteAndRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, _, envID := setupFSTest(t)

	err := svc.WriteFile(ctx, envID, "/home/user/test.py", "print('hi')", 0o644)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	content, err := svc.ReadFile(ctx, envID, "/home/user/test.py")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if content != "print('hi')" {
		t.Errorf("content = %q, want %q", content, "print('hi')")
	}
}

func TestFilesystemServiceListFiles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, _, envID := setupFSTest(t)

	files, err := svc.ListFiles(ctx, envID, "/home/user")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("len = %d, want 1", len(files))
	}
	if files[0].Name != "test.py" {
		t.Errorf("Name = %q, want %q", files[0].Name, "test.py")
	}
}

func TestFilesystemServiceNotRunning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	env := environment.New("stopped-id", "stopped", environment.RuntimePython, 10000, time.Hour)
	_ = repo.Save(ctx, env)

	svc := NewFilesystemService(repo, newFakeFS(), nil)

	err := svc.WriteFile(ctx, "stopped-id", "/test", "data", 0o644)
	if !errors.Is(err, environment.ErrNotRunning) {
		t.Errorf("WriteFile err = %v, want ErrNotRunning", err)
	}

	_, err = svc.ReadFile(ctx, "stopped-id", "/test")
	if !errors.Is(err, environment.ErrNotRunning) {
		t.Errorf("ReadFile err = %v, want ErrNotRunning", err)
	}

	_, err = svc.ListFiles(ctx, "stopped-id", "/")
	if !errors.Is(err, environment.ErrNotRunning) {
		t.Errorf("ListFiles err = %v, want ErrNotRunning", err)
	}
}

func TestFilesystemServiceNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc := NewFilesystemService(store.NewMemoryStore(), newFakeFS(), nil)

	_, err := svc.ReadFile(ctx, "nonexistent", "/test")
	if !errors.Is(err, environment.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
