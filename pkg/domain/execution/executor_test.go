// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package execution

import "testing"

func TestExecResultSucceeded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exitCode int
		want     bool
	}{
		{name: "zero exit code", exitCode: 0, want: true},
		{name: "non-zero exit code", exitCode: 1, want: false},
		{name: "signal exit code", exitCode: 137, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &ExecResult{ExitCode: tt.exitCode}
			if got := r.Succeeded(); got != tt.want {
				t.Errorf("Succeeded() = %v, want %v", got, tt.want)
			}
		})
	}
}
