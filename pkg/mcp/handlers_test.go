// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"strings"
	"testing"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
)

func TestIntArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       map[string]any
		key        string
		defaultVal int
		want       int
	}{
		{
			name:       "float64 value",
			args:       map[string]any{"timeout": float64(30)},
			key:        "timeout",
			defaultVal: 10,
			want:       30,
		},
		{
			name:       "int value",
			args:       map[string]any{"timeout": 45},
			key:        "timeout",
			defaultVal: 10,
			want:       45,
		},
		{
			name:       "missing key returns default",
			args:       map[string]any{},
			key:        "timeout",
			defaultVal: 10,
			want:       10,
		},
		{
			name:       "wrong type returns default",
			args:       map[string]any{"timeout": "thirty"},
			key:        "timeout",
			defaultVal: 10,
			want:       10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := intArg(tt.args, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("intArg() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJsonResult(t *testing.T) {
	t.Parallel()

	result, err := jsonResult(map[string]any{
		"status": "ok",
		"count":  42,
	})
	if err != nil {
		t.Fatalf("jsonResult: %v", err)
	}

	if result.IsError {
		t.Error("result should not be an error")
	}

	// Check that the result contains the expected JSON fields.
	textContent := result.Content[0].(mcptypes.TextContent)
	text := textContent.Text
	if !strings.Contains(text, `"status": "ok"`) {
		t.Errorf("result should contain status, got: %s", text)
	}
	if !strings.Contains(text, `"count": 42`) {
		t.Errorf("result should contain count, got: %s", text)
	}
}
