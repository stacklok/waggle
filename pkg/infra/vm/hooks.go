// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/stacklok/propolis/image"

	"github.com/stacklok/waggle/pkg/infra/vm/initbin"
)

// InjectInitBinary returns a RootFS hook that writes the embedded waggle-init
// binary to /waggle-init in the guest rootfs.
func InjectInitBinary() func(string, *image.OCIConfig) error {
	return func(rootfsPath string, _ *image.OCIConfig) error {
		initPath := filepath.Join(rootfsPath, "waggle-init")
		if err := os.WriteFile(initPath, initbin.Binary, 0o755); err != nil { //nolint:gosec // needs to be executable in the guest
			return fmt.Errorf("writing init binary: %w", err)
		}
		return nil
	}
}
