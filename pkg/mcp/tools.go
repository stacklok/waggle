// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package mcp

import "github.com/mark3labs/mcp-go/mcp"

// Tool name constants.
const (
	ToolCreateEnvironment  = "create_environment"
	ToolDestroyEnvironment = "destroy_environment"
	ToolListEnvironments   = "list_environments"
	ToolExecute            = "execute"
	ToolWriteFile          = "write_file"
	ToolReadFile           = "read_file"
	ToolListFiles          = "list_files"
	ToolInstallPackages    = "install_packages"
)

// Tools returns all waggle MCP tool definitions.
func Tools() []mcp.Tool {
	return []mcp.Tool{
		createEnvironmentTool(),
		destroyEnvironmentTool(),
		listEnvironmentsTool(),
		executeTool(),
		writeFileTool(),
		readFileTool(),
		listFilesTool(),
		installPackagesTool(),
	}
}

func createEnvironmentTool() mcp.Tool {
	return mcp.NewTool(ToolCreateEnvironment,
		mcp.WithDescription(
			"Create a fresh isolated execution environment with a specific runtime. "+
				"Returns the environment ID needed for all subsequent operations.",
		),
		mcp.WithString("runtime",
			mcp.Required(),
			mcp.Description("Runtime type for the environment"),
			mcp.Enum("python", "node", "shell"),
		),
		mcp.WithString("name",
			mcp.Description("Optional human-readable name for the environment"),
		),
		mcp.WithNumber("timeout_minutes",
			mcp.Description("Auto-destroy timeout in minutes after last activity (default: 30)"),
		),
	)
}

func destroyEnvironmentTool() mcp.Tool {
	return mcp.NewTool(ToolDestroyEnvironment,
		mcp.WithDescription("Destroy an execution environment and release all its resources."),
		mcp.WithString("environment_id",
			mcp.Required(),
			mcp.Description("ID of the environment to destroy"),
		),
	)
}

func listEnvironmentsTool() mcp.Tool {
	return mcp.NewTool(ToolListEnvironments,
		mcp.WithDescription("List all active execution environments with their status and metadata."),
	)
}

func executeTool() mcp.Tool {
	return mcp.NewTool(ToolExecute,
		mcp.WithDescription(
			"Execute code or a shell command in an environment. "+
				"Returns stdout, stderr, exit code, and duration. "+
				"Multi-line code is fully supported.",
		),
		mcp.WithString("environment_id",
			mcp.Required(),
			mcp.Description("ID of the target environment"),
		),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("Code or shell command to execute"),
		),
		mcp.WithString("language",
			mcp.Description("Override language for this execution: python, node, or shell. Defaults to the environment's runtime."),
			mcp.Enum("python", "node", "shell"),
		),
		mcp.WithNumber("timeout_seconds",
			mcp.Description("Execution timeout in seconds (default: 30, max: 300)"),
		),
	)
}

func writeFileTool() mcp.Tool {
	return mcp.NewTool(ToolWriteFile,
		mcp.WithDescription("Write content to a file inside an environment. Creates parent directories if needed."),
		mcp.WithString("environment_id",
			mcp.Required(),
			mcp.Description("ID of the target environment"),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute path for the file inside the environment"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Content to write to the file"),
		),
	)
}

func readFileTool() mcp.Tool {
	return mcp.NewTool(ToolReadFile,
		mcp.WithDescription("Read the content of a file from an environment."),
		mcp.WithString("environment_id",
			mcp.Required(),
			mcp.Description("ID of the target environment"),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute path of the file to read"),
		),
	)
}

func listFilesTool() mcp.Tool {
	return mcp.NewTool(ToolListFiles,
		mcp.WithDescription("List files and directories at a path inside an environment."),
		mcp.WithString("environment_id",
			mcp.Required(),
			mcp.Description("ID of the target environment"),
		),
		mcp.WithString("path",
			mcp.Description("Directory path to list (default: /root)"),
		),
	)
}

func installPackagesTool() mcp.Tool {
	return mcp.NewTool(ToolInstallPackages,
		mcp.WithDescription(
			"Install language packages in an environment. "+
				"Uses pip for Python, npm for Node.js, apk for shell environments.",
		),
		mcp.WithString("environment_id",
			mcp.Required(),
			mcp.Description("ID of the target environment"),
		),
		mcp.WithString("packages",
			mcp.Required(),
			mcp.Description("Space-separated list of package names to install"),
		),
	)
}
