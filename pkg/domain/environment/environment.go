// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package environment

import (
	"fmt"
	"time"
)

// Status represents the lifecycle state of an environment.
type Status string

const (
	// StatusCreating indicates the environment is being provisioned.
	StatusCreating Status = "creating"

	// StatusRunning indicates the environment is ready for use.
	StatusRunning Status = "running"

	// StatusDestroying indicates the environment is being torn down.
	StatusDestroying Status = "destroying"

	// StatusDestroyed indicates the environment has been fully cleaned up.
	StatusDestroyed Status = "destroyed"

	// StatusError indicates the environment encountered an unrecoverable error.
	StatusError Status = "error"
)

// validTransitions defines the allowed state transitions.
var validTransitions = map[Status][]Status{
	StatusCreating:   {StatusRunning, StatusError},
	StatusRunning:    {StatusDestroying, StatusError},
	StatusDestroying: {StatusDestroyed},
}

// Environment is the aggregate root for an isolated code execution context.
// Each environment corresponds to a single propolis microVM.
type Environment struct {
	ID        string
	Name      string
	Runtime   Runtime
	Status    Status
	SSHPort   uint16
	CreatedAt time.Time
	LastUsed  time.Time
	Timeout   time.Duration
}

// New creates a new Environment in the Creating state.
func New(id, name string, runtime Runtime, sshPort uint16, timeout time.Duration) *Environment {
	now := time.Now()
	return &Environment{
		ID:        id,
		Name:      name,
		Runtime:   runtime,
		Status:    StatusCreating,
		SSHPort:   sshPort,
		CreatedAt: now,
		LastUsed:  now,
		Timeout:   timeout,
	}
}

// TransitionTo attempts to move the environment to a new state.
// It returns ErrInvalidTransition if the transition is not allowed.
func (e *Environment) TransitionTo(target Status) error {
	allowed, ok := validTransitions[e.Status]
	if !ok {
		return fmt.Errorf("%w: no transitions from %q", ErrInvalidTransition, e.Status)
	}

	for _, s := range allowed {
		if s == target {
			e.Status = target
			return nil
		}
	}

	return fmt.Errorf("%w: %q -> %q", ErrInvalidTransition, e.Status, target)
}

// Touch updates the LastUsed timestamp to the current time.
func (e *Environment) Touch() {
	e.LastUsed = time.Now()
}

// IsExpired returns true if the environment has exceeded its inactivity timeout.
func (e *Environment) IsExpired() bool {
	if e.Timeout <= 0 {
		return false
	}
	return time.Since(e.LastUsed) > e.Timeout
}

// IsRunning returns true if the environment is in the Running state.
func (e *Environment) IsRunning() bool {
	return e.Status == StatusRunning
}
