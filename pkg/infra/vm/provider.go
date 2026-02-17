// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"context"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

// CreateVMOpts holds options for creating a VM.
type CreateVMOpts struct {
	CPUs     uint32
	MemoryMB uint32
	ImageRef string
	DataDir  string

	// RunnerPath is an optional explicit path to propolis-runner.
	RunnerPath string

	// LibDir is an optional path to the libkrun library directory.
	LibDir string
}

// VMHandle holds the runtime state for a running VM, including
// the SSH key path needed to connect to it.
type VMHandle struct {
	EnvID      string
	SSHKeyPath string
}

// VMProvider abstracts VM lifecycle management.
type VMProvider interface {
	// CreateVM provisions a new microVM for the given environment.
	CreateVM(ctx context.Context, env *environment.Environment, opts CreateVMOpts) (*VMHandle, error)

	// DestroyVM tears down the VM associated with the given environment ID.
	DestroyVM(ctx context.Context, envID string) error

	// IsRunning checks whether the VM for the given environment ID is alive.
	IsRunning(ctx context.Context, envID string) bool

	// SSHKeyPath returns the path to the SSH private key for the given
	// environment, or an empty string if the environment is unknown.
	SSHKeyPath(envID string) string
}
