// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package initbin embeds the pre-compiled waggle-init binary.
// The binary is built by `task build-init` and placed at
// pkg/infra/vm/initbin/waggle-init before compiling waggle.
package initbin

import _ "embed"

// Binary contains the embedded waggle-init binary.
//
//go:embed waggle-init
var Binary []byte
