// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/service"
)

// ToolHandler routes MCP tool calls to application services.
type ToolHandler struct {
	envSvc  *service.EnvironmentService
	execSvc *service.ExecutionService
	fsSvc   *service.FilesystemService
}

// NewToolHandler creates a new ToolHandler.
func NewToolHandler(
	envSvc *service.EnvironmentService,
	execSvc *service.ExecutionService,
	fsSvc *service.FilesystemService,
) *ToolHandler {
	return &ToolHandler{
		envSvc:  envSvc,
		execSvc: execSvc,
		fsSvc:   fsSvc,
	}
}

// HandleCreateEnvironment handles the create_environment tool call.
func (h *ToolHandler) HandleCreateEnvironment(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	runtime, _ := args["runtime"].(string)
	if runtime == "" {
		return mcp.NewToolResultError("runtime is required (python, node, or shell)"), nil
	}

	name, _ := args["name"].(string)
	timeoutMin := intArg(args, "timeout_minutes", 0)

	env, err := h.envSvc.Create(ctx, environment.Runtime(runtime), name, timeoutMin)
	if err != nil {
		slog.Error("failed to create environment", "error", err)
		return mcp.NewToolResultError("failed to create environment"), nil
	}

	return jsonResult(map[string]any{
		"environment_id": env.ID,
		"name":           env.Name,
		"runtime":        string(env.Runtime),
		"status":         string(env.Status),
		"ssh_port":       env.SSHPort,
		"created_at":     env.CreatedAt.Format(time.RFC3339),
		"timeout":        env.Timeout.String(),
	})
}

// HandleDestroyEnvironment handles the destroy_environment tool call.
func (h *ToolHandler) HandleDestroyEnvironment(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	envID, _ := args["environment_id"].(string)
	if envID == "" {
		return mcp.NewToolResultError("environment_id is required"), nil
	}

	if err := h.envSvc.Destroy(ctx, envID); err != nil {
		slog.Error("failed to destroy environment", "env_id", envID, "error", err)
		return mcp.NewToolResultError("failed to destroy environment"), nil
	}

	return jsonResult(map[string]any{
		"status":  "destroyed",
		"message": fmt.Sprintf("Environment %s has been destroyed", envID),
	})
}

// HandleListEnvironments handles the list_environments tool call.
func (h *ToolHandler) HandleListEnvironments(
	ctx context.Context, _ mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	envs, err := h.envSvc.List(ctx)
	if err != nil {
		slog.Error("failed to list environments", "error", err)
		return mcp.NewToolResultError("failed to list environments"), nil
	}

	items := make([]map[string]any, 0, len(envs))
	for _, env := range envs {
		items = append(items, map[string]any{
			"environment_id": env.ID,
			"name":           env.Name,
			"runtime":        string(env.Runtime),
			"status":         string(env.Status),
			"created_at":     env.CreatedAt.Format(time.RFC3339),
		})
	}

	return jsonResult(map[string]any{
		"environments": items,
		"count":        len(items),
	})
}

// HandleExecute handles the execute tool call.
func (h *ToolHandler) HandleExecute(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	envID, _ := args["environment_id"].(string)
	if envID == "" {
		return mcp.NewToolResultError("environment_id is required"), nil
	}

	code, _ := args["code"].(string)
	if code == "" {
		return mcp.NewToolResultError("code is required"), nil
	}

	language, _ := args["language"].(string)
	timeoutSec := intArg(args, "timeout_seconds", 0)

	result, err := h.execSvc.Execute(ctx, envID, code, language, timeoutSec)
	if err != nil {
		slog.Error("failed to execute code", "env_id", envID, "error", err)
		return mcp.NewToolResultError("failed to execute code"), nil
	}

	return jsonResult(map[string]any{
		"stdout":      result.Stdout,
		"stderr":      result.Stderr,
		"exit_code":   result.ExitCode,
		"duration_ms": result.DurationMs,
	})
}

// HandleWriteFile handles the write_file tool call.
func (h *ToolHandler) HandleWriteFile(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	envID, _ := args["environment_id"].(string)
	if envID == "" {
		return mcp.NewToolResultError("environment_id is required"), nil
	}

	path, _ := args["path"].(string)
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	content, _ := args["content"].(string)

	if err := h.fsSvc.WriteFile(ctx, envID, path, content, 0o644); err != nil {
		slog.Error("failed to write file", "env_id", envID, "path", path, "error", err)
		return mcp.NewToolResultError("failed to write file"), nil
	}

	return jsonResult(map[string]any{
		"status":        "written",
		"path":          path,
		"bytes_written": len(content),
	})
}

// HandleReadFile handles the read_file tool call.
func (h *ToolHandler) HandleReadFile(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	envID, _ := args["environment_id"].(string)
	if envID == "" {
		return mcp.NewToolResultError("environment_id is required"), nil
	}

	path, _ := args["path"].(string)
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	content, err := h.fsSvc.ReadFile(ctx, envID, path)
	if err != nil {
		slog.Error("failed to read file", "env_id", envID, "path", path, "error", err)
		return mcp.NewToolResultError("failed to read file"), nil
	}

	return mcp.NewToolResultText(content), nil
}

// HandleListFiles handles the list_files tool call.
func (h *ToolHandler) HandleListFiles(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	envID, _ := args["environment_id"].(string)
	if envID == "" {
		return mcp.NewToolResultError("environment_id is required"), nil
	}

	path, _ := args["path"].(string)
	if path == "" {
		path = "/home/sandbox"
	}

	files, err := h.fsSvc.ListFiles(ctx, envID, path)
	if err != nil {
		slog.Error("failed to list files", "env_id", envID, "path", path, "error", err)
		return mcp.NewToolResultError("failed to list files"), nil
	}

	items := make([]map[string]any, 0, len(files))
	for _, f := range files {
		items = append(items, map[string]any{
			"name":     f.Name,
			"size":     f.Size,
			"is_dir":   f.IsDir,
			"mode":     f.Mode,
			"modified": f.Modified.Format(time.RFC3339),
		})
	}

	return jsonResult(map[string]any{
		"path":  path,
		"files": items,
		"count": len(items),
	})
}

// HandleInstallPackages handles the install_packages tool call.
func (h *ToolHandler) HandleInstallPackages(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	envID, _ := args["environment_id"].(string)
	if envID == "" {
		return mcp.NewToolResultError("environment_id is required"), nil
	}

	packagesStr, _ := args["packages"].(string)
	if packagesStr == "" {
		return mcp.NewToolResultError("packages is required"), nil
	}

	packages := strings.Fields(packagesStr)

	result, err := h.execSvc.InstallPackages(ctx, envID, packages)
	if err != nil {
		slog.Error("failed to install packages", "env_id", envID, "error", err)
		return mcp.NewToolResultError("failed to install packages"), nil
	}

	return jsonResult(map[string]any{
		"stdout":    result.Stdout,
		"stderr":    result.Stderr,
		"exit_code": result.ExitCode,
	})
}

// jsonResult serializes data as indented JSON and returns it as a tool result.
func jsonResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

// intArg extracts an integer from the arguments map, handling JSON number types.
func intArg(args map[string]any, key string, defaultVal int) int {
	v, ok := args[key]
	if !ok {
		return defaultVal
	}

	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return defaultVal
	}
}
