// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package filesystem

import (
	"testing"
	"time"
)

func TestFileInfoFields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	fi := FileInfo{
		Name:     "test.py",
		Size:     1024,
		IsDir:    false,
		Mode:     "-rw-r--r--",
		Modified: now,
	}

	if fi.Name != "test.py" {
		t.Errorf("Name = %q, want %q", fi.Name, "test.py")
	}
	if fi.Size != 1024 {
		t.Errorf("Size = %d, want %d", fi.Size, 1024)
	}
	if fi.IsDir {
		t.Error("IsDir = true, want false")
	}
	if fi.Mode != "-rw-r--r--" {
		t.Errorf("Mode = %q, want %q", fi.Mode, "-rw-r--r--")
	}
	if !fi.Modified.Equal(now) {
		t.Errorf("Modified = %v, want %v", fi.Modified, now)
	}
}

func TestFileInfoDirectory(t *testing.T) {
	t.Parallel()

	fi := FileInfo{
		Name:  "src",
		IsDir: true,
		Mode:  "drwxr-xr-x",
	}

	if !fi.IsDir {
		t.Error("IsDir = false, want true for directory")
	}
}
