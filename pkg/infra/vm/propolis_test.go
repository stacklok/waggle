// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stacklok/propolis/extract"
	"github.com/stacklok/propolis/image"
)

func TestNewPropolisProvider(t *testing.T) {
	t.Parallel()

	t.Run("no options", func(t *testing.T) {
		t.Parallel()
		p := NewPropolisProvider()
		if p.vms == nil {
			t.Fatal("vms map should be initialized")
		}
		if p.runtimeSource != nil {
			t.Error("runtimeSource should be nil without option")
		}
		if p.firmwareSource != nil {
			t.Error("firmwareSource should be nil without option")
		}
	})

	t.Run("with runtime and firmware sources", func(t *testing.T) {
		t.Parallel()
		rtSrc := extract.Dir(t.TempDir())
		fwSrc := extract.Dir(t.TempDir())
		p := NewPropolisProvider(
			WithRuntimeSource(rtSrc),
			WithFirmwareSource(fwSrc),
		)
		if p.runtimeSource == nil {
			t.Error("runtimeSource should be set")
		}
		if p.firmwareSource == nil {
			t.Error("firmwareSource should be set")
		}
	})

	t.Run("with image cache", func(t *testing.T) {
		t.Parallel()
		cache := image.NewCache(t.TempDir())
		p := NewPropolisProvider(WithImageCache(cache))
		if p.imageCache == nil {
			t.Error("imageCache should be set")
		}
	})

	t.Run("with log level", func(t *testing.T) {
		t.Parallel()
		p := NewPropolisProvider(WithLogLevel(3))
		if p.logLevel != 3 {
			t.Errorf("logLevel = %d, want 3", p.logLevel)
		}
	})
}

// TestBuildBackendOpts verifies the merging logic in buildBackendOpts.
// Since libkrun.Option is an opaque func type, we assert on option count
// to verify which branches were taken. Each configuration produces a
// unique count.
func TestBuildBackendOpts(t *testing.T) {
	t.Parallel()

	rtSrc := extract.Dir(t.TempDir())
	fwSrc := extract.Dir(t.TempDir())
	altSrc := extract.Dir(t.TempDir())

	tests := []struct {
		name      string
		provider  *PropolisProvider
		opts      CreateVMOpts
		wantCount int
	}{
		{
			name:      "no sources no paths",
			provider:  NewPropolisProvider(),
			opts:      CreateVMOpts{DataDir: "/tmp"},
			wantCount: 0,
		},
		{
			name:     "runner path only",
			provider: NewPropolisProvider(),
			opts: CreateVMOpts{
				RunnerPath: "/usr/bin/propolis-runner",
				DataDir:    "/tmp",
			},
			wantCount: 1,
		},
		{
			name:     "runner path and lib dir",
			provider: NewPropolisProvider(),
			opts: CreateVMOpts{
				RunnerPath: "/usr/bin/propolis-runner",
				LibDir:     "/usr/lib",
				DataDir:    "/tmp",
			},
			wantCount: 2,
		},
		{
			name:     "provider runtime source only",
			provider: NewPropolisProvider(WithRuntimeSource(rtSrc)),
			opts:     CreateVMOpts{DataDir: "/tmp"},
			// WithRuntime + WithCacheDir = 2
			wantCount: 2,
		},
		{
			name:     "provider firmware source only",
			provider: NewPropolisProvider(WithFirmwareSource(fwSrc)),
			opts:     CreateVMOpts{DataDir: "/tmp"},
			// WithFirmware + WithCacheDir = 2
			wantCount: 2,
		},
		{
			name: "provider both sources",
			provider: NewPropolisProvider(
				WithRuntimeSource(rtSrc),
				WithFirmwareSource(fwSrc),
			),
			opts: CreateVMOpts{DataDir: "/tmp"},
			// WithRuntime + WithFirmware + WithCacheDir = 3
			wantCount: 3,
		},
		{
			name:     "runtime source suppresses runner path and lib dir",
			provider: NewPropolisProvider(WithRuntimeSource(rtSrc)),
			opts: CreateVMOpts{
				RunnerPath: "/usr/bin/propolis-runner",
				LibDir:     "/usr/lib",
				DataDir:    "/tmp",
			},
			// WithRuntime + WithCacheDir = 2 (RunnerPath/LibDir ignored)
			wantCount: 2,
		},
		{
			name:     "per-call runtime source overrides provider",
			provider: NewPropolisProvider(WithRuntimeSource(rtSrc)),
			opts: CreateVMOpts{
				RuntimeSource: altSrc,
				DataDir:       "/tmp",
			},
			// WithRuntime(altSrc) + WithCacheDir = 2
			wantCount: 2,
		},
		{
			name:     "per-call firmware source overrides provider",
			provider: NewPropolisProvider(WithFirmwareSource(fwSrc)),
			opts: CreateVMOpts{
				FirmwareSource: altSrc,
				DataDir:        "/tmp",
			},
			// WithFirmware(altSrc) + WithCacheDir = 2
			wantCount: 2,
		},
		{
			name: "explicit cache dir used over data dir",
			provider: NewPropolisProvider(
				WithRuntimeSource(rtSrc),
				WithFirmwareSource(fwSrc),
			),
			opts: CreateVMOpts{
				DataDir:  "/tmp/data",
				CacheDir: "/tmp/cache",
			},
			// WithRuntime + WithFirmware + WithCacheDir = 3
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.provider.buildBackendOpts(tt.opts)
			if len(got) != tt.wantCount {
				t.Errorf("buildBackendOpts returned %d options, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestInitInjector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T) string // returns initPath
		wantHook  bool
		wantErr   bool
		wantBytes []byte // expected content at /waggle-init in rootfs
	}{
		{
			name: "binary exists and is injected",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				path := filepath.Join(dir, "waggle-init")
				if err := os.WriteFile(path, []byte("fake-binary"), 0o755); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantHook:  true,
			wantBytes: []byte("fake-binary"),
		},
		{
			name: "binary does not exist returns error",
			setup: func(_ *testing.T) string {
				return "/nonexistent/waggle-init"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			initPath := tt.setup(t)
			hook, err := initInjector(initPath)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !tt.wantHook {
				if hook != nil {
					t.Fatal("expected nil hook for empty path")
				}
				return
			}

			if hook == nil {
				t.Fatal("expected non-nil hook")
			}

			// Exercise the hook against a temp rootfs directory.
			rootfs := t.TempDir()
			if err := hook(rootfs, nil); err != nil {
				t.Fatalf("hook execution failed: %v", err)
			}

			injected := filepath.Join(rootfs, "waggle-init")
			got, err := os.ReadFile(injected)
			if err != nil {
				t.Fatalf("failed to read injected binary: %v", err)
			}
			if string(got) != string(tt.wantBytes) {
				t.Errorf("injected content = %q, want %q", got, tt.wantBytes)
			}

			info, err := os.Stat(injected)
			if err != nil {
				t.Fatalf("failed to stat injected binary: %v", err)
			}
			// InjectBinary uses 0755 permissions.
			if perm := info.Mode().Perm(); perm != 0o755 {
				t.Errorf("injected binary permissions = %o, want 0755", perm)
			}
		})
	}
}
