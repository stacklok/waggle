// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"errors"
	"testing"
)

func TestExtractExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error",
			err:  nil,
			want: 0,
		},
		{
			name: "process exited with status",
			err:  errors.New("ssh command \"false\": Process exited with status 1"),
			want: 1,
		},
		{
			name: "exit status",
			err:  errors.New("exit status 137"),
			want: 137,
		},
		{
			name: "exit status 2",
			err:  errors.New("ssh stream command \"bad\": exit status 2"),
			want: 2,
		},
		{
			name: "unrelated error",
			err:  errors.New("connection refused"),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractExitCode(tt.err)
			if got != tt.want {
				t.Errorf("extractExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
