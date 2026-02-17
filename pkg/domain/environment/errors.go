// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package environment

import "errors"

var (
	// ErrNotFound is returned when an environment cannot be found by ID.
	ErrNotFound = errors.New("environment not found")

	// ErrNotRunning is returned when an operation requires a running environment
	// but the environment is in a different state.
	ErrNotRunning = errors.New("environment is not running")

	// ErrAlreadyExists is returned when attempting to create an environment
	// with an ID that already exists.
	ErrAlreadyExists = errors.New("environment already exists")

	// ErrInvalidTransition is returned when an invalid state transition
	// is attempted.
	ErrInvalidTransition = errors.New("invalid state transition")

	// ErrMaxEnvironments is returned when the maximum number of concurrent
	// environments has been reached.
	ErrMaxEnvironments = errors.New("maximum number of environments reached")

	// ErrInvalidRuntime is returned when an unsupported runtime is specified.
	ErrInvalidRuntime = errors.New("invalid runtime")
)
