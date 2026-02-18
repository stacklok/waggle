// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stacklok/waggle/pkg/config"
	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/infra/vm"
)

// EnvironmentService orchestrates environment lifecycle operations.
type EnvironmentService struct {
	repo      environment.Repository
	provider  vm.Provider
	portAlloc *vm.PortAllocator
	config    *config.Config
}

// NewEnvironmentService creates a new EnvironmentService.
func NewEnvironmentService(
	repo environment.Repository,
	provider vm.Provider,
	portAlloc *vm.PortAllocator,
	cfg *config.Config,
) *EnvironmentService {
	return &EnvironmentService{
		repo:      repo,
		provider:  provider,
		portAlloc: portAlloc,
		config:    cfg,
	}
}

// Create provisions a new isolated environment with the given runtime.
func (s *EnvironmentService) Create(
	ctx context.Context, runtime environment.Runtime, name string, timeoutMin int,
) (*environment.Environment, error) {
	// Validate runtime.
	if err := runtime.Validate(); err != nil {
		return nil, err
	}

	// Check capacity.
	count, err := s.repo.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count environments: %w", err)
	}
	if count >= s.config.MaxEnvironments {
		return nil, environment.ErrMaxEnvironments
	}

	// Resolve image.
	imageRef := s.config.ImageRef(runtime)
	if imageRef == "" {
		return nil, fmt.Errorf("no image configured for runtime %q", runtime)
	}

	// Generate name if not provided.
	if name == "" {
		name = fmt.Sprintf("%s-%s", runtime, uuid.New().String()[:8])
	}

	// Apply timeout default.
	if timeoutMin <= 0 {
		timeoutMin = s.config.DefaultTimeoutMin
	}
	timeout := time.Duration(timeoutMin) * time.Minute

	// Allocate SSH port.
	sshPort, err := s.portAlloc.Allocate()
	if err != nil {
		return nil, fmt.Errorf("allocate SSH port: %w", err)
	}

	// Create domain object.
	env := environment.New(uuid.New().String(), name, runtime, sshPort, timeout)

	// Persist in Creating state.
	if err := s.repo.Save(ctx, env); err != nil {
		s.portAlloc.Release(sshPort)
		return nil, fmt.Errorf("save environment: %w", err)
	}

	slog.Info("creating environment",
		"id", env.ID,
		"name", env.Name,
		"runtime", env.Runtime,
		"ssh_port", env.SSHPort,
	)

	// Create the VM with boot timeout.
	bootCtx, cancel := context.WithTimeout(ctx, s.config.BootTimeout)
	defer cancel()

	_, vmErr := s.provider.CreateVM(bootCtx, env, vm.CreateVMOpts{
		CPUs:       s.config.DefaultCPUs,
		MemoryMB:   s.config.DefaultMemoryMB,
		ImageRef:   imageRef,
		DataDir:    s.config.DataDir,
		RunnerPath: s.config.RunnerPath,
		LibDir:     s.config.LibDir,
	})
	if vmErr != nil {
		// Transition to error state.
		_ = env.TransitionTo(environment.StatusError)
		_ = s.repo.Save(ctx, env)
		s.portAlloc.Release(sshPort)
		return nil, fmt.Errorf("create VM: %w", vmErr)
	}

	// Transition to Running.
	if err := env.TransitionTo(environment.StatusRunning); err != nil {
		return nil, fmt.Errorf("transition to running: %w", err)
	}
	if err := s.repo.Save(ctx, env); err != nil {
		return nil, fmt.Errorf("save running state: %w", err)
	}

	slog.Info("environment ready",
		"id", env.ID,
		"name", env.Name,
	)

	return env, nil
}

// Destroy tears down an environment and releases its resources.
func (s *EnvironmentService) Destroy(ctx context.Context, envID string) error {
	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return err
	}

	slog.Info("destroying environment", "id", envID, "name", env.Name)

	// Transition to Destroying.
	if transErr := env.TransitionTo(environment.StatusDestroying); transErr != nil {
		// If we can't transition (e.g., already destroying), still try to clean up.
		slog.Warn("state transition failed, proceeding with cleanup",
			"id", envID, "error", transErr)
	}
	_ = s.repo.Save(ctx, env)

	// Destroy the VM.
	if vmErr := s.provider.DestroyVM(ctx, envID); vmErr != nil {
		slog.Error("VM destruction failed", "id", envID, "error", vmErr)
	}

	// Release the port.
	s.portAlloc.Release(env.SSHPort)

	// Remove from store.
	_ = s.repo.Delete(ctx, envID)

	return nil
}

// List returns all active environments.
func (s *EnvironmentService) List(ctx context.Context) ([]*environment.Environment, error) {
	return s.repo.FindAll(ctx)
}

// Get returns a single environment by ID.
func (s *EnvironmentService) Get(ctx context.Context, envID string) (*environment.Environment, error) {
	return s.repo.FindByID(ctx, envID)
}

// Touch updates the last-used timestamp of an environment.
func (s *EnvironmentService) Touch(ctx context.Context, envID string) error {
	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return err
	}
	env.Touch()
	return s.repo.Save(ctx, env)
}
