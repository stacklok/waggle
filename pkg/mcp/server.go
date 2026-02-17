// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/stacklok/waggle/pkg/service"
)

// NewServer creates a configured MCP server with all waggle tools registered.
func NewServer(
	version string,
	envSvc *service.EnvironmentService,
	execSvc *service.ExecutionService,
	fsSvc *service.FilesystemService,
) *server.MCPServer {
	s := server.NewMCPServer(
		"Waggle",
		version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
		server.WithLogging(),
	)

	handler := NewToolHandler(envSvc, execSvc, fsSvc)

	// Register all tools with their handlers.
	for _, tool := range Tools() {
		switch tool.Name {
		case ToolCreateEnvironment:
			s.AddTool(tool, handler.HandleCreateEnvironment)
		case ToolDestroyEnvironment:
			s.AddTool(tool, handler.HandleDestroyEnvironment)
		case ToolListEnvironments:
			s.AddTool(tool, handler.HandleListEnvironments)
		case ToolExecute:
			s.AddTool(tool, handler.HandleExecute)
		case ToolWriteFile:
			s.AddTool(tool, handler.HandleWriteFile)
		case ToolReadFile:
			s.AddTool(tool, handler.HandleReadFile)
		case ToolListFiles:
			s.AddTool(tool, handler.HandleListFiles)
		case ToolInstallPackages:
			s.AddTool(tool, handler.HandleInstallPackages)
		}
	}

	return s
}
