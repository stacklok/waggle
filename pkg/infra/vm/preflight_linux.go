// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package vm

import "github.com/stacklok/propolis/preflight"

// extraPreflightChecks returns additional preflight checks for Linux hosts.
// Currently this adds a user namespace availability check, which catches
// kernel.unprivileged_userns_clone=0 before VM creation fails with EPERM.
func extraPreflightChecks() []preflight.Check {
	return []preflight.Check{preflight.UserNamespaceCheck()}
}
