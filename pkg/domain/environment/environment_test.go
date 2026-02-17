// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package environment

import (
	"errors"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Parallel()

	env := New("test-id", "test-env", RuntimePython, 10000, 30*time.Minute)

	if env.ID != "test-id" {
		t.Errorf("ID = %q, want %q", env.ID, "test-id")
	}
	if env.Name != "test-env" {
		t.Errorf("Name = %q, want %q", env.Name, "test-env")
	}
	if env.Runtime != RuntimePython {
		t.Errorf("Runtime = %q, want %q", env.Runtime, RuntimePython)
	}
	if env.Status != StatusCreating {
		t.Errorf("Status = %q, want %q", env.Status, StatusCreating)
	}
	if env.SSHPort != 10000 {
		t.Errorf("SSHPort = %d, want %d", env.SSHPort, 10000)
	}
	if env.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want %v", env.Timeout, 30*time.Minute)
	}
	if env.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if env.LastUsed.IsZero() {
		t.Error("LastUsed should not be zero")
	}
}

func TestTransitionTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    Status
		to      Status
		wantErr bool
	}{
		// Valid transitions
		{name: "creating to running", from: StatusCreating, to: StatusRunning},
		{name: "creating to error", from: StatusCreating, to: StatusError},
		{name: "running to destroying", from: StatusRunning, to: StatusDestroying},
		{name: "running to error", from: StatusRunning, to: StatusError},
		{name: "destroying to destroyed", from: StatusDestroying, to: StatusDestroyed},

		// Invalid transitions
		{name: "creating to destroying", from: StatusCreating, to: StatusDestroying, wantErr: true},
		{name: "creating to destroyed", from: StatusCreating, to: StatusDestroyed, wantErr: true},
		{name: "running to creating", from: StatusRunning, to: StatusCreating, wantErr: true},
		{name: "running to running", from: StatusRunning, to: StatusRunning, wantErr: true},
		{name: "destroying to running", from: StatusDestroying, to: StatusRunning, wantErr: true},
		{name: "destroyed to anything", from: StatusDestroyed, to: StatusRunning, wantErr: true},
		{name: "error to anything", from: StatusError, to: StatusRunning, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := &Environment{Status: tt.from}
			err := env.TransitionTo(tt.to)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q -> %q, got nil", tt.from, tt.to)
				}
				if !errors.Is(err, ErrInvalidTransition) {
					t.Errorf("expected ErrInvalidTransition, got: %v", err)
				}
				// Status should not change on error
				if env.Status != tt.from {
					t.Errorf("status changed on error: got %q, want %q", env.Status, tt.from)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if env.Status != tt.to {
				t.Errorf("status = %q, want %q", env.Status, tt.to)
			}
		})
	}
}

func TestTouch(t *testing.T) {
	t.Parallel()

	env := New("id", "name", RuntimeShell, 10000, time.Hour)
	original := env.LastUsed

	// Small sleep to ensure time difference
	time.Sleep(time.Millisecond)
	env.Touch()

	if !env.LastUsed.After(original) {
		t.Error("Touch() should update LastUsed to a later time")
	}
}

func TestIsExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		timeout  time.Duration
		lastUsed time.Time
		want     bool
	}{
		{
			name:     "not expired",
			timeout:  time.Hour,
			lastUsed: time.Now(),
			want:     false,
		},
		{
			name:     "expired",
			timeout:  time.Millisecond,
			lastUsed: time.Now().Add(-time.Second),
			want:     true,
		},
		{
			name:     "zero timeout never expires",
			timeout:  0,
			lastUsed: time.Now().Add(-24 * time.Hour),
			want:     false,
		},
		{
			name:     "negative timeout never expires",
			timeout:  -time.Hour,
			lastUsed: time.Now().Add(-24 * time.Hour),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := &Environment{
				Timeout:  tt.timeout,
				LastUsed: tt.lastUsed,
			}
			if got := env.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRunning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status Status
		want   bool
	}{
		{StatusCreating, false},
		{StatusRunning, true},
		{StatusDestroying, false},
		{StatusDestroyed, false},
		{StatusError, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			t.Parallel()
			env := &Environment{Status: tt.status}
			if got := env.IsRunning(); got != tt.want {
				t.Errorf("IsRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFullLifecycle(t *testing.T) {
	t.Parallel()

	env := New("lc-1", "lifecycle-test", RuntimeNode, 10001, 30*time.Minute)

	// Creating -> Running
	if err := env.TransitionTo(StatusRunning); err != nil {
		t.Fatalf("Creating -> Running: %v", err)
	}

	// Running -> Destroying
	if err := env.TransitionTo(StatusDestroying); err != nil {
		t.Fatalf("Running -> Destroying: %v", err)
	}

	// Destroying -> Destroyed
	if err := env.TransitionTo(StatusDestroyed); err != nil {
		t.Fatalf("Destroying -> Destroyed: %v", err)
	}

	// No transitions from Destroyed
	if err := env.TransitionTo(StatusRunning); err == nil {
		t.Error("expected error transitioning from Destroyed")
	}
}
