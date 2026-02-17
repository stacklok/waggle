// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"testing"

	"github.com/stacklok/waggle/pkg/domain/filesystem"
)

func TestParseFindOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
		want   []filesystem.FileInfo
	}{
		{
			name:   "empty output",
			output: "",
			want:   nil,
		},
		{
			name:   "single file",
			output: "f|-rw-r--r--|1024|1700000000.000000|test.py\n",
			want: []filesystem.FileInfo{
				{Name: "test.py", Size: 1024, IsDir: false, Mode: "-rw-r--r--"},
			},
		},
		{
			name:   "directory",
			output: "d|drwxr-xr-x|4096|1700000000.000000|src\n",
			want: []filesystem.FileInfo{
				{Name: "src", Size: 4096, IsDir: true, Mode: "drwxr-xr-x"},
			},
		},
		{
			name: "multiple entries",
			output: "f|-rw-r--r--|100|1700000000.000000|a.txt\n" +
				"d|drwxr-xr-x|4096|1700000000.000000|subdir\n" +
				"f|-rwxr-xr-x|2048|1700000000.000000|script.sh\n",
			want: []filesystem.FileInfo{
				{Name: "a.txt", Size: 100, IsDir: false, Mode: "-rw-r--r--"},
				{Name: "subdir", Size: 4096, IsDir: true, Mode: "drwxr-xr-x"},
				{Name: "script.sh", Size: 2048, IsDir: false, Mode: "-rwxr-xr-x"},
			},
		},
		{
			name:   "malformed line is skipped",
			output: "not|enough|fields\nf|-rw-r--r--|512|1700000000.000000|ok.txt\n",
			want: []filesystem.FileInfo{
				{Name: "ok.txt", Size: 512, IsDir: false, Mode: "-rw-r--r--"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseFindOutput(tt.output)

			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}

			for i, f := range got {
				w := tt.want[i]
				if f.Name != w.Name {
					t.Errorf("[%d] Name = %q, want %q", i, f.Name, w.Name)
				}
				if f.Size != w.Size {
					t.Errorf("[%d] Size = %d, want %d", i, f.Size, w.Size)
				}
				if f.IsDir != w.IsDir {
					t.Errorf("[%d] IsDir = %v, want %v", i, f.IsDir, w.IsDir)
				}
				if f.Mode != w.Mode {
					t.Errorf("[%d] Mode = %q, want %q", i, f.Mode, w.Mode)
				}
			}
		})
	}
}

func TestParentDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{"/home/user/test.py", "/home/user"},
		{"/test.py", "/"},
		{"test.py", "/"},
		{"/a/b/c/d.txt", "/a/b/c"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := parentDir(tt.path)
			if got != tt.want {
				t.Errorf("parentDir(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
