// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	propolisssh "github.com/stacklok/propolis/ssh"

	"github.com/stacklok/waggle/pkg/domain/filesystem"
)

// FileSystem implements filesystem.FileSystem using SSH connections to microVMs.
type FileSystem struct{}

// NewFileSystem creates a new SSH-based FileSystem.
func NewFileSystem() *FileSystem {
	return &FileSystem{}
}

// WriteFile writes content to a file inside the environment via SSH.
func (*FileSystem) WriteFile(
	ctx context.Context, conn filesystem.ConnInfo, path string, content []byte, mode os.FileMode,
) error {
	client := propolisssh.NewClient(conn.Host, conn.Port, "root", conn.KeyPath)

	// Create parent directory if needed.
	dir := parentDir(path)
	if dir != "" && dir != "." && dir != "/" {
		mkdirCmd := fmt.Sprintf("mkdir -p %s", propolisssh.ShellEscape(dir))
		if _, mkdirErr := client.Run(ctx, mkdirCmd); mkdirErr != nil {
			return fmt.Errorf("mkdir for write: %w", mkdirErr)
		}
	}

	// Write content via stdin pipe to cat, then chmod.
	cmd := fmt.Sprintf("cat > %s && chmod %o %s",
		propolisssh.ShellEscape(path), mode, propolisssh.ShellEscape(path))

	var stdout, stderr bytes.Buffer
	stdinReader := bytes.NewReader(content)

	// We need to use a lower-level approach since CopyTo expects a local file.
	// Use Run with stdin piped content.
	// Actually, propolis ssh.Client doesn't expose stdin on Run, only on the
	// session directly. We'll write via a temp approach:
	// base64 encode on host, decode on guest.
	_ = stdinReader // unused, we'll use a different approach

	// Write the content using printf + base64 decode via SSH.
	// For small files, write directly via echo. For larger files or binary,
	// use base64. Let's use a simple cat heredoc approach via Run.
	_ = cmd    // unused
	_ = stdout // unused
	_ = stderr // unused

	// Simplest correct approach: write to a temp file locally, CopyTo.
	tmpFile, tmpErr := os.CreateTemp("", "waggle-upload-*")
	if tmpErr != nil {
		return fmt.Errorf("create temp file: %w", tmpErr)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, writeErr := tmpFile.Write(content); writeErr != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file: %w", writeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return fmt.Errorf("close temp file: %w", closeErr)
	}

	if copyErr := client.CopyTo(ctx, tmpPath, path, mode); copyErr != nil {
		return fmt.Errorf("copy to VM: %w", copyErr)
	}

	return nil
}

// ReadFile reads a file from the environment via SSH.
func (*FileSystem) ReadFile(ctx context.Context, conn filesystem.ConnInfo, path string) ([]byte, error) {
	client := propolisssh.NewClient(conn.Host, conn.Port, "root", conn.KeyPath)

	cmd := fmt.Sprintf("cat %s", propolisssh.ShellEscape(path))
	output, runErr := client.Run(ctx, cmd)
	if runErr != nil {
		return nil, fmt.Errorf("read file %s: %w", path, runErr)
	}

	return []byte(output), nil
}

// ListFiles returns directory contents from the environment via SSH.
func (*FileSystem) ListFiles(
	ctx context.Context, conn filesystem.ConnInfo, path string,
) ([]filesystem.FileInfo, error) {
	client := propolisssh.NewClient(conn.Host, conn.Port, "root", conn.KeyPath)

	// Use stat-style output for reliable parsing.
	// Format: type|perms|size|mtime_epoch|name
	cmd := fmt.Sprintf(
		`find %s -maxdepth 1 -mindepth 1 -printf '%%y|%%M|%%s|%%T@|%%f\n' 2>/dev/null || `+
			`ls -la --time-style=+%%s %s 2>/dev/null`,
		propolisssh.ShellEscape(path),
		propolisssh.ShellEscape(path),
	)

	output, runErr := client.Run(ctx, cmd)
	if runErr != nil {
		return nil, fmt.Errorf("list files %s: %w", path, runErr)
	}

	return parseFindOutput(output), nil
}

// parseFindOutput parses the output of find -printf '%y|%M|%s|%T@|%f\n'.
func parseFindOutput(output string) []filesystem.FileInfo {
	var files []filesystem.FileInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}

		fileType := parts[0]
		perms := parts[1]
		sizeStr := parts[2]
		mtimeStr := parts[3]
		name := parts[4]

		size, _ := strconv.ParseInt(sizeStr, 10, 64)

		// Parse epoch timestamp (may have fractional part).
		var modified time.Time
		if dotIdx := strings.Index(mtimeStr, "."); dotIdx > 0 {
			mtimeStr = mtimeStr[:dotIdx]
		}
		if epoch, parseErr := strconv.ParseInt(mtimeStr, 10, 64); parseErr == nil {
			modified = time.Unix(epoch, 0)
		}

		files = append(files, filesystem.FileInfo{
			Name:     name,
			Size:     size,
			IsDir:    fileType == "d",
			Mode:     perms,
			Modified: modified,
		})
	}
	return files
}

// parentDir returns the parent directory of a path.
func parentDir(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}
