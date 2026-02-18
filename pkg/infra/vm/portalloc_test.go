// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func newTestAllocator(base, maxPort uint16) *PortAllocator { //nolint:unparam // base varies conceptually even if tests use 10000
	a := NewPortAllocator(base, maxPort)
	// Replace listen check with a no-op for unit tests.
	a.listenCheck = func(_ uint16) error { return nil }
	return a
}

func TestPortAllocatorAllocate(t *testing.T) {
	t.Parallel()

	a := newTestAllocator(10000, 10003)

	port1, err := a.Allocate()
	if err != nil {
		t.Fatalf("first Allocate: %v", err)
	}
	if port1 != 10000 {
		t.Errorf("first port = %d, want 10000", port1)
	}

	port2, err := a.Allocate()
	if err != nil {
		t.Fatalf("second Allocate: %v", err)
	}
	if port2 != 10001 {
		t.Errorf("second port = %d, want 10001", port2)
	}

	if a.AllocatedCount() != 2 {
		t.Errorf("AllocatedCount = %d, want 2", a.AllocatedCount())
	}
}

func TestPortAllocatorExhaustion(t *testing.T) {
	t.Parallel()

	a := newTestAllocator(10000, 10002) // Only 2 ports available.

	if _, err := a.Allocate(); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := a.Allocate(); err != nil {
		t.Fatalf("second: %v", err)
	}

	_, err := a.Allocate()
	if !errors.Is(err, ErrPortsExhausted) {
		t.Errorf("err = %v, want ErrPortsExhausted", err)
	}
}

func TestPortAllocatorRelease(t *testing.T) {
	t.Parallel()

	a := newTestAllocator(10000, 10001) // Single port.

	port, err := a.Allocate()
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	// Exhaust the pool.
	_, err = a.Allocate()
	if !errors.Is(err, ErrPortsExhausted) {
		t.Fatalf("expected exhaustion, got: %v", err)
	}

	// Release and re-allocate.
	a.Release(port)

	if a.AllocatedCount() != 0 {
		t.Errorf("AllocatedCount after release = %d, want 0", a.AllocatedCount())
	}

	port2, err := a.Allocate()
	if err != nil {
		t.Fatalf("re-Allocate: %v", err)
	}
	if port2 != port {
		t.Errorf("re-allocated port = %d, want %d", port2, port)
	}
}

func TestPortAllocatorSkipsUnavailable(t *testing.T) {
	t.Parallel()

	a := NewPortAllocator(10000, 10003)
	// Simulate port 10000 and 10001 being in use by another process.
	a.listenCheck = func(port uint16) error {
		if port == 10000 || port == 10001 {
			return fmt.Errorf("port %d in use", port)
		}
		return nil
	}

	port, err := a.Allocate()
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if port != 10002 {
		t.Errorf("port = %d, want 10002 (skipping unavailable)", port)
	}
}

func TestPortAllocatorConcurrent(t *testing.T) {
	t.Parallel()

	const poolSize = 100
	a := newTestAllocator(10000, 10000+poolSize)

	var wg sync.WaitGroup
	ports := make(chan uint16, poolSize)
	errs := make(chan error, poolSize)

	// Allocate all ports concurrently.
	for range poolSize {
		wg.Add(1)
		go func() {
			defer wg.Done()
			port, err := a.Allocate()
			if err != nil {
				errs <- err
				return
			}
			ports <- port
		}()
	}
	wg.Wait()
	close(ports)
	close(errs)

	for err := range errs {
		t.Errorf("concurrent allocation error: %v", err)
	}

	// Verify all ports are unique.
	seen := make(map[uint16]bool)
	for port := range ports {
		if seen[port] {
			t.Errorf("duplicate port allocated: %d", port)
		}
		seen[port] = true
	}

	if len(seen) != poolSize {
		t.Errorf("allocated %d unique ports, want %d", len(seen), poolSize)
	}

	if a.AllocatedCount() != poolSize {
		t.Errorf("AllocatedCount = %d, want %d", a.AllocatedCount(), poolSize)
	}
}

func TestPortAllocatorRealListenCheck(t *testing.T) {
	t.Parallel()

	// Use the real listen check - just verify it doesn't panic
	// and can allocate at least one port from a high range.
	a := NewPortAllocator(49152, 49252)
	port, err := a.Allocate()
	if err != nil {
		t.Skipf("could not allocate port (may be CI): %v", err)
	}
	if port < 49152 || port >= 49252 {
		t.Errorf("port %d outside range [49152, 49252)", port)
	}
	a.Release(port)
}
