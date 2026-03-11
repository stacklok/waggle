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

const (
	// MaxTimeoutMinutes is the maximum allowed inactivity timeout for an environment.
	MaxTimeoutMinutes = 60

	probeTimeout = 30 * time.Second

	capabilityMaxAge = 10 * time.Minute
	probeRetryCount  = 3
)

// EnvironmentService orchestrates environment lifecycle operations.
type EnvironmentService struct {
	repo      environment.Repository
	provider  vm.Provider
	portAlloc *vm.PortAllocator
	prober    EnvironmentProber
	config    *config.Config
}

// NewEnvironmentService creates a new EnvironmentService.
func NewEnvironmentService(
	repo environment.Repository,
	provider vm.Provider,
	portAlloc *vm.PortAllocator,
	prober EnvironmentProber,
	cfg *config.Config,
) *EnvironmentService {
	return &EnvironmentService{
		repo:      repo,
		provider:  provider,
		portAlloc: portAlloc,
		prober:    prober,
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
	if timeoutMin > MaxTimeoutMinutes {
		return nil, fmt.Errorf("timeout_minutes %d exceeds maximum allowed value of %d", timeoutMin, MaxTimeoutMinutes)
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
		InitPath:   s.config.InitPath,
		LibDir:     s.config.LibDir,
		CacheDir:   s.config.CacheDir,
	})
	if vmErr != nil {
		// Transition to error state.
		if transErr := env.TransitionTo(environment.StatusError); transErr != nil {
			slog.Error("failed to transition environment to error state",
				"id", env.ID, "error", transErr, "vm_error", vmErr)
		}
		if saveErr := s.repo.Save(ctx, env); saveErr != nil {
			slog.Error("failed to save environment error state",
				"id", env.ID, "error", saveErr, "vm_error", vmErr)
		}
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

	//nolint:gosec // G118: probe intentionally outlives request ctx.
	go s.probeCapabilitiesWithRetry(context.Background(), env.ID, env.SSHPort, probeRetryCount)

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
	if saveErr := s.repo.Save(ctx, env); saveErr != nil {
		slog.Error("failed to save destroying state", "id", envID, "error", saveErr)
	}

	// Destroy the VM.
	if vmErr := s.provider.DestroyVM(ctx, envID); vmErr != nil {
		slog.Error("VM destruction failed", "id", envID, "error", vmErr)
	}

	// Release the port.
	s.portAlloc.Release(env.SSHPort)

	// Remove from store.
	if err := s.repo.Delete(ctx, envID); err != nil {
		return fmt.Errorf("delete environment from store: %w", err)
	}

	return nil
}

// EnsureCapabilities probes the environment if capabilities are missing or stale.
func (s *EnvironmentService) EnsureCapabilities(ctx context.Context, env *environment.Environment) error {
	if s.prober == nil {
		return nil
	}
	if !capabilitiesStale(env) {
		return nil
	}
	keyPath := s.provider.SSHKeyPath(env.ID)
	if keyPath == "" {
		return fmt.Errorf("capability probe skipped: missing SSH key for %s", env.ID)
	}

	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	caps, err := s.prober.Probe(probeCtx, "127.0.0.1", env.SSHPort, keyPath)
	if err != nil {
		return err
	}

	env.Capabilities = caps
	env.CapabilitiesDetected = true
	return s.repo.Save(ctx, env)
}

func (s *EnvironmentService) probeCapabilitiesWithRetry(
	ctx context.Context, envID string, sshPort uint16, attempts int,
) {
	if s.prober == nil {
		return
	}
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return
		}
		caps, err := s.probeCapabilitiesOnce(ctx, envID, sshPort)
		if err == nil {
			if saveErr := s.storeCapabilities(ctx, envID, caps); saveErr == nil {
				return
			}
		}
		if i < attempts-1 {
			timer := time.NewTimer(500 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
	slog.Warn("capability probe failed after retries", "env_id", envID)
}

func (s *EnvironmentService) probeCapabilitiesOnce(
	ctx context.Context, envID string, sshPort uint16,
) (environment.Capabilities, error) {
	keyPath := s.provider.SSHKeyPath(envID)
	if keyPath == "" {
		return environment.Capabilities{}, fmt.Errorf("capability probe skipped: missing SSH key for %s", envID)
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	return s.prober.Probe(probeCtx, "127.0.0.1", sshPort, keyPath)
}

func (s *EnvironmentService) storeCapabilities(
	ctx context.Context, envID string, caps environment.Capabilities,
) error {
	env, err := s.repo.FindByID(ctx, envID)
	if err != nil {
		return err
	}
	env.Capabilities = caps
	env.CapabilitiesDetected = true
	return s.repo.Save(ctx, env)
}

func capabilitiesStale(env *environment.Environment) bool {
	if !env.CapabilitiesDetected {
		return true
	}
	if env.Capabilities.DetectedAt.IsZero() {
		return true
	}
	return time.Since(env.Capabilities.DetectedAt) > capabilityMaxAge
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
