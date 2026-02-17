// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"os"

	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/filesystem"
)

// FilesystemService orchestrates file operations within environments.
type FilesystemService struct {
	repo   environment.Repository
	fs     filesystem.FileSystem
	envSvc *EnvironmentService
}

// NewFilesystemService creates a new FilesystemService.
func NewFilesystemService(
	repo environment.Repository,
	fs filesystem.FileSystem,
	envSvc *EnvironmentService,
) *FilesystemService {
	return &FilesystemService{
		repo:   repo,
		fs:     fs,
		envSvc: envSvc,
	}
}

// WriteFile writes content to a file in the specified environment.
func (s *FilesystemService) WriteFile(
	ctx context.Context, envID, path, content string, mode os.FileMode,
) error {
	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return err
	}
	if !env.IsRunning() {
		return environment.ErrNotRunning
	}

	_ = s.envSvc.Touch(ctx, envID)

	return s.fs.WriteFile(ctx, envID, path, []byte(content), mode)
}

// ReadFile reads a file from the specified environment.
func (s *FilesystemService) ReadFile(
	ctx context.Context, envID, path string,
) (string, error) {
	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return "", err
	}
	if !env.IsRunning() {
		return "", environment.ErrNotRunning
	}

	_ = s.envSvc.Touch(ctx, envID)

	data, readErr := s.fs.ReadFile(ctx, envID, path)
	if readErr != nil {
		return "", readErr
	}
	return string(data), nil
}

// ListFiles returns directory contents from the specified environment.
func (s *FilesystemService) ListFiles(
	ctx context.Context, envID, path string,
) ([]filesystem.FileInfo, error) {
	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return nil, err
	}
	if !env.IsRunning() {
		return nil, environment.ErrNotRunning
	}

	_ = s.envSvc.Touch(ctx, envID)

	return s.fs.ListFiles(ctx, envID, path)
}
