// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package filesystem

import (
	"context"
	"os"
	"time"
)

// FileInfo describes a file or directory within an environment.
type FileInfo struct {
	// Name is the file or directory name (not the full path).
	Name string

	// Size is the file size in bytes.
	Size int64

	// IsDir indicates whether this entry is a directory.
	IsDir bool

	// Mode is the file permission string (e.g., "-rwxr-xr-x").
	Mode string

	// Modified is the last modification time.
	Modified time.Time
}

// FileSystem provides file operations within an environment identified by its ID.
type FileSystem interface {
	// WriteFile writes content to a file at the given path inside the environment.
	// If the file does not exist, it is created. If it exists, it is overwritten.
	WriteFile(ctx context.Context, envID string, path string, content []byte, mode os.FileMode) error

	// ReadFile reads the content of a file at the given path inside the environment.
	ReadFile(ctx context.Context, envID string, path string) ([]byte, error)

	// ListFiles returns the contents of a directory at the given path
	// inside the environment.
	ListFiles(ctx context.Context, envID string, path string) ([]FileInfo, error)
}
