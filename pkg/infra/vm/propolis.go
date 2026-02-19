// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/stacklok/propolis"
	"github.com/stacklok/propolis/hooks"
	"github.com/stacklok/propolis/hypervisor/libkrun"
	propolisssh "github.com/stacklok/propolis/ssh"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

// vmEntry holds the runtime state for a single VM.
type vmEntry struct {
	vm         *propolis.VM
	sshKeyPath string
}

// PropolisProvider implements Provider using propolis microVMs.
type PropolisProvider struct {
	mu sync.RWMutex
	// vms maps environment ID to the running VM entry.
	vms map[string]*vmEntry
}

// NewPropolisProvider creates a new PropolisProvider.
func NewPropolisProvider() *PropolisProvider {
	return &PropolisProvider{
		vms: make(map[string]*vmEntry),
	}
}

// CreateVM provisions a new microVM for the given environment.
func (p *PropolisProvider) CreateVM(ctx context.Context, env *environment.Environment, opts CreateVMOpts) (*Handle, error) {
	// Create per-environment data directory.
	envDataDir := filepath.Join(opts.DataDir, "envs", env.ID)
	if err := os.MkdirAll(envDataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create env data dir: %w", err)
	}

	// Generate SSH keys for this environment.
	privateKeyPath, publicKeyPath, err := propolisssh.GenerateKeyPair(envDataDir)
	if err != nil {
		return nil, fmt.Errorf("generate SSH keys: %w", err)
	}

	// Read the public key content for injection into rootfs.
	pubKeyContent, err := propolisssh.GetPublicKeyContent(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read SSH public key: %w", err)
	}

	slog.Info("creating microVM",
		"env_id", env.ID,
		"runtime", env.Runtime,
		"image", opts.ImageRef,
		"ssh_port", env.SSHPort,
	)

	// Build propolis options.
	propolisOpts := []propolis.Option{
		propolis.WithName("waggle-" + env.ID),
		propolis.WithCPUs(opts.CPUs),
		propolis.WithMemory(opts.MemoryMB),
		propolis.WithPorts(propolis.PortForward{
			Host:  env.SSHPort,
			Guest: 22,
		}),
		propolis.WithDataDir(envDataDir),

		// Inject SSH authorized_keys into the rootfs before boot.
		propolis.WithRootFSHook(hooks.InjectAuthorizedKeys(pubKeyContent, hooks.WithKeyUser("/root", 0, 0))),

		// Wait for SSH to become ready after boot.
		propolis.WithPostBoot(sshReadyWaiter(env.SSHPort, privateKeyPath)),
	}

	if opts.InitPath != "" {
		hook, err := initInjector(opts.InitPath)
		if err != nil {
			return nil, err
		}
		propolisOpts = append(propolisOpts,
			propolis.WithRootFSHook(hook),
			propolis.WithInitOverride("/waggle-init"),
		)
	}

	var backendOpts []libkrun.Option
	if opts.RunnerPath != "" {
		backendOpts = append(backendOpts, libkrun.WithRunnerPath(opts.RunnerPath))
	}
	if opts.LibDir != "" {
		backendOpts = append(backendOpts, libkrun.WithLibDir(opts.LibDir))
	}
	propolisOpts = append(propolisOpts, propolis.WithBackend(libkrun.NewBackend(backendOpts...)))

	// Start the VM.
	vm, err := propolis.Run(ctx, opts.ImageRef, propolisOpts...)
	if err != nil {
		return nil, fmt.Errorf("propolis.Run: %w", err)
	}

	// Store the VM handle.
	p.mu.Lock()
	p.vms[env.ID] = &vmEntry{
		vm:         vm,
		sshKeyPath: privateKeyPath,
	}
	p.mu.Unlock()

	slog.Info("microVM created successfully",
		"env_id", env.ID,
		"id", vm.ID(),
	)

	return &Handle{
		EnvID:      env.ID,
		SSHKeyPath: privateKeyPath,
	}, nil
}

// DestroyVM tears down the VM for the given environment.
func (p *PropolisProvider) DestroyVM(ctx context.Context, envID string) error {
	p.mu.Lock()
	entry, ok := p.vms[envID]
	if ok {
		delete(p.vms, envID)
	}
	p.mu.Unlock()

	if !ok {
		slog.Debug("DestroyVM called but VM already gone", "env_id", envID)
		return nil
	}

	slog.Info("destroying microVM", "env_id", envID)

	if err := entry.vm.Remove(ctx); err != nil {
		return fmt.Errorf("remove VM: %w", err)
	}

	return nil
}

// IsRunning checks whether the VM for the given environment is alive.
func (p *PropolisProvider) IsRunning(_ context.Context, envID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.vms[envID]
	return ok
}

// SSHKeyPath returns the SSH private key path for the given environment.
func (p *PropolisProvider) SSHKeyPath(envID string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entry, ok := p.vms[envID]
	if !ok {
		return ""
	}
	return entry.sshKeyPath
}

// initInjector reads the waggle-init binary from disk and returns a
// RootFSHook that injects it into the guest rootfs at /waggle-init.
// The caller must ensure initPath is non-empty.
func initInjector(initPath string) (propolis.RootFSHook, error) {
	data, err := os.ReadFile(initPath) //nolint:gosec // initPath comes from server config, not user input
	if err != nil {
		return nil, fmt.Errorf("read init binary %s: %w", initPath, err)
	}
	return hooks.InjectBinary("/waggle-init", data), nil
}

// sshReadyWaiter returns a PostBootHook that waits for SSH to become
// available on the given port.
func sshReadyWaiter(port uint16, keyPath string) propolis.PostBootHook {
	return func(ctx context.Context, _ *propolis.VM) error {
		client := propolisssh.NewClient("127.0.0.1", port, "root", keyPath)
		return client.WaitForReady(ctx)
	}
}
