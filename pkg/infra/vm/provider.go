// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"context"

	"github.com/stacklok/go-microvm/extract"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

// CreateVMOpts holds options for creating a VM.
type CreateVMOpts struct {
	CPUs     uint32
	MemoryMB uint32
	ImageRef string
	DataDir  string

	// RunnerPath is an optional explicit path to go-microvm-runner.
	RunnerPath string

	// InitPath is an optional path to the waggle-init binary.
	// When set, the binary is injected into the rootfs and used as
	// the VM init process instead of the OCI image CMD/ENTRYPOINT.
	InitPath string

	// LibDir is an optional path to the libkrun library directory.
	LibDir string

	// RuntimeSource provides go-microvm-runner and libkrun via extraction.
	// Mutually exclusive with RunnerPath and LibDir.
	RuntimeSource extract.Source

	// FirmwareSource provides libkrunfw via extraction.
	FirmwareSource extract.Source

	// CacheDir is the directory for extract.Source cache. Required when
	// using bundle-based sources. Defaults to DataDir if empty.
	CacheDir string
}

// Handle holds the runtime state for a running VM, including
// the SSH key path needed to connect to it.
type Handle struct {
	EnvID      string
	SSHKeyPath string
}

// Provider abstracts VM lifecycle management.
type Provider interface {
	// CreateVM provisions a new microVM for the given environment.
	CreateVM(ctx context.Context, env *environment.Environment, opts CreateVMOpts) (*Handle, error)

	// DestroyVM tears down the VM associated with the given environment ID.
	DestroyVM(ctx context.Context, envID string) error

	// IsRunning checks whether the VM for the given environment ID is alive.
	IsRunning(ctx context.Context, envID string) bool

	// SSHKeyPath returns the path to the SSH private key for the given
	// environment, or an empty string if the environment is unknown.
	SSHKeyPath(envID string) string
}
