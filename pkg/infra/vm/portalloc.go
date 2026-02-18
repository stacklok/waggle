// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"fmt"
	"net"
	"sync"
)

// ErrPortsExhausted is returned when no ports are available for allocation.
var ErrPortsExhausted = fmt.Errorf("no ports available in configured range")

// PortAllocator manages dynamic port allocation for SSH connections
// to environment microVMs. It allocates ports from a configurable range
// and verifies availability using net.Listen.
type PortAllocator struct {
	mu        sync.Mutex
	basePort  uint16
	maxPort   uint16
	allocated map[uint16]bool

	// listenCheck is a function that verifies a port is available.
	// Defaults to tcpListenCheck. Injected for testability.
	listenCheck func(port uint16) error
}

// NewPortAllocator creates a PortAllocator for the given port range [base, maxPort).
func NewPortAllocator(base, maxPort uint16) *PortAllocator {
	return &PortAllocator{
		basePort:    base,
		maxPort:     maxPort,
		allocated:   make(map[uint16]bool),
		listenCheck: tcpListenCheck,
	}
}

// Allocate finds and reserves an available port. It scans the configured
// range and verifies the port is not in use by another process.
// Returns ErrPortsExhausted if no ports are available.
func (a *PortAllocator) Allocate() (uint16, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for port := a.basePort; port < a.maxPort; port++ {
		if a.allocated[port] {
			continue
		}

		if err := a.listenCheck(port); err != nil {
			continue
		}

		a.allocated[port] = true
		return port, nil
	}

	return 0, ErrPortsExhausted
}

// Release returns a previously allocated port to the pool.
func (a *PortAllocator) Release(port uint16) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.allocated, port)
}

// SetListenCheck replaces the port availability check function.
// This is intended for testing to avoid binding real ports.
func (a *PortAllocator) SetListenCheck(fn func(port uint16) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.listenCheck = fn
}

// AllocatedCount returns the number of currently allocated ports.
func (a *PortAllocator) AllocatedCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return len(a.allocated)
}

// tcpListenCheck verifies a port is available by attempting to listen on it.
func tcpListenCheck(port uint16) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	return ln.Close()
}
