// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package environment

import "time"

// Capabilities captures runtime capabilities detected inside an environment.
type Capabilities struct {
	DetectedAt time.Time
	Path       string

	PythonCommand string
	PipCommand    string
	NodeCommand   string
	NpmCommand    string

	HasVenv   bool
	HasApk    bool
	HasAptGet bool
	HasDnf    bool
	HasYum    bool
	HasZypper bool
	IsRoot    bool
}
