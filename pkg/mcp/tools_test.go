// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package mcp

import "testing"

func TestToolsCount(t *testing.T) {
	t.Parallel()

	tools := Tools()
	if len(tools) != 8 {
		t.Errorf("len(Tools()) = %d, want 8", len(tools))
	}
}

func TestToolNames(t *testing.T) {
	t.Parallel()

	expected := map[string]bool{
		ToolCreateEnvironment:  false,
		ToolDestroyEnvironment: false,
		ToolListEnvironments:   false,
		ToolExecute:            false,
		ToolWriteFile:          false,
		ToolReadFile:           false,
		ToolListFiles:          false,
		ToolInstallPackages:    false,
	}

	for _, tool := range Tools() {
		if _, ok := expected[tool.Name]; !ok {
			t.Errorf("unexpected tool name: %q", tool.Name)
		}
		expected[tool.Name] = true
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing tool: %q", name)
		}
	}
}

func TestToolsHaveDescriptions(t *testing.T) {
	t.Parallel()

	for _, tool := range Tools() {
		if tool.Description == "" {
			t.Errorf("tool %q has no description", tool.Name)
		}
	}
}
