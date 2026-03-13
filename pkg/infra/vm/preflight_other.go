// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package vm

import "github.com/stacklok/propolis/preflight"

// extraPreflightChecks returns nil on non-Linux platforms where user
// namespace checks are not applicable.
func extraPreflightChecks() []preflight.Check {
	return nil
}
