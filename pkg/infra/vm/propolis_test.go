// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"os"
	"path/filepath"
	"testing"
)

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
