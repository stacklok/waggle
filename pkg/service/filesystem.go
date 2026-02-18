// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"os"

	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/filesystem"
	"github.com/stacklok/waggle/pkg/infra/vm"
)

const (
	// MaxFileSizeBytes is the maximum allowed file size for reads and writes (10 MB).
	MaxFileSizeBytes = 10 * 1024 * 1024
)

// FilesystemService orchestrates file operations within environments.
type FilesystemService struct {
	repo        environment.Repository
	fs          filesystem.FileSystem
	provider    vm.Provider
	envSvc      *EnvironmentService
	maxFileSize int
}

// NewFilesystemService creates a new FilesystemService.
func NewFilesystemService(
	repo environment.Repository,
	fs filesystem.FileSystem,
	provider vm.Provider,
	envSvc *EnvironmentService,
) *FilesystemService {
	return &FilesystemService{
		repo:        repo,
		fs:          fs,
		provider:    provider,
		envSvc:      envSvc,
		maxFileSize: MaxFileSizeBytes,
	}
}

// WriteFile writes content to a file in the specified environment.
func (s *FilesystemService) WriteFile(
	ctx context.Context, envID, path, content string, mode os.FileMode,
) error {
	if len(content) > s.maxFileSize {
		return fmt.Errorf("file content exceeds maximum allowed size of %d bytes", s.maxFileSize)
	}

	conn, err := s.validateAndConn(ctx, envID)
	if err != nil {
		return err
	}

	_ = s.envSvc.Touch(ctx, envID)

	return s.fs.WriteFile(ctx, conn, path, []byte(content), mode)
}

// ReadFile reads a file from the specified environment.
func (s *FilesystemService) ReadFile(
	ctx context.Context, envID, path string,
) (string, error) {
	conn, err := s.validateAndConn(ctx, envID)
	if err != nil {
		return "", err
	}

	_ = s.envSvc.Touch(ctx, envID)

	data, readErr := s.fs.ReadFile(ctx, conn, path)
	if readErr != nil {
		return "", readErr
	}
	if len(data) > s.maxFileSize {
		return "", fmt.Errorf("file content exceeds maximum allowed size of %d bytes", s.maxFileSize)
	}
	return string(data), nil
}

// ListFiles returns directory contents from the specified environment.
func (s *FilesystemService) ListFiles(
	ctx context.Context, envID, path string,
) ([]filesystem.FileInfo, error) {
	conn, err := s.validateAndConn(ctx, envID)
	if err != nil {
		return nil, err
	}

	_ = s.envSvc.Touch(ctx, envID)

	return s.fs.ListFiles(ctx, conn, path)
}

// validateAndConn validates the environment state and returns pre-resolved connection info.
func (s *FilesystemService) validateAndConn(ctx context.Context, envID string) (filesystem.ConnInfo, error) {
	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return filesystem.ConnInfo{}, err
	}
	if !env.IsRunning() {
		return filesystem.ConnInfo{}, environment.ErrNotRunning
	}

	keyPath := s.provider.SSHKeyPath(envID)
	if keyPath == "" {
		return filesystem.ConnInfo{}, fmt.Errorf("SSH key not found for environment %q", envID)
	}

	return filesystem.ConnInfo{
		Host:    "127.0.0.1",
		Port:    env.SSHPort,
		KeyPath: keyPath,
	}, nil
}
