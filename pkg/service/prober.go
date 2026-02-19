// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

// EnvironmentProber inspects a running environment for available capabilities.
type EnvironmentProber interface {
	Probe(ctx context.Context, host string, port uint16, keyPath string) (environment.Capabilities, error)
}
