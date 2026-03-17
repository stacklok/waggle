// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	microvmssh "github.com/stacklok/go-microvm/ssh"

	"github.com/stacklok/waggle/pkg/domain/filesystem"
)

const listHeader = "WAGGLE_LIST_V1"

//go:embed list_files.sh
var listFilesScript string

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
	client := microvmssh.NewClient(conn.Host, conn.Port, "sandbox", conn.KeyPath)

	// Create parent directory if needed.
	dir := parentDir(path)
	if dir != "" && dir != "." && dir != "/" {
		mkdirCmd := fmt.Sprintf("mkdir -p %s", microvmssh.ShellEscape(dir))
		if _, mkdirErr := client.Run(ctx, mkdirCmd); mkdirErr != nil {
			return fmt.Errorf("mkdir for write: %w", mkdirErr)
		}
	}

	// Write content via stdin pipe to cat, then chmod.
	cmd := fmt.Sprintf("cat > %s && chmod %o %s",
		microvmssh.ShellEscape(path), mode, microvmssh.ShellEscape(path))

	var stdout, stderr bytes.Buffer
	stdinReader := bytes.NewReader(content)

	// We need to use a lower-level approach since CopyTo expects a local file.
	// Use Run with stdin piped content.
	// Actually, go-microvm ssh.Client doesn't expose stdin on Run, only on the
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
	client := microvmssh.NewClient(conn.Host, conn.Port, "sandbox", conn.KeyPath)

	cmd := fmt.Sprintf("cat %s", microvmssh.ShellEscape(path))
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
	client := microvmssh.NewClient(conn.Host, conn.Port, "sandbox", conn.KeyPath)
	cmd := listFilesCommand(path)

	output, runErr := client.Run(ctx, cmd)
	if runErr != nil {
		return nil, fmt.Errorf("list files %s: %w", path, runErr)
	}

	files, parseErr := parseFindOutput(output)
	if parseErr != nil {
		return nil, fmt.Errorf("parse list output: %w", parseErr)
	}

	return files, nil
}

// parseFindOutput parses the output produced by listFilesCommand.
func parseFindOutput(output string) ([]filesystem.FileInfo, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, fmt.Errorf("list output missing header")
	}
	lines := strings.Split(trimmed, "\n")
	header := strings.TrimSuffix(lines[0], "\r")
	if header != listHeader {
		return nil, fmt.Errorf("unexpected list output header: %q", header)
	}

	var files []filesystem.FileInfo
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\r")
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
	return files, nil
}

func listFilesCommand(path string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(listFilesScript))
	template := strings.Join([]string{
		"if command -v base64 >/dev/null 2>&1; then ",
		"printf '%%s' %s | base64 -d | sh -s -- %s %s; ",
		"else cat <<'WAGGLE_EOF' | sh -s -- %s %s\n",
		"%s\n",
		"WAGGLE_EOF\n",
		"fi",
	}, "")

	return fmt.Sprintf(
		template,
		microvmssh.ShellEscape(encoded),
		microvmssh.ShellEscape(path),
		microvmssh.ShellEscape(listHeader),
		microvmssh.ShellEscape(path),
		microvmssh.ShellEscape(listHeader),
		listFilesScript,
	)
}

// parentDir returns the parent directory of a path.
func parentDir(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}
