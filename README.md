# Waggle

An MCP server that provides AI agents with secure, isolated code execution environments powered by [propolis](https://github.com/stacklok/propolis) microVMs.

> The waggle dance is one of the most fascinating things in nature -- bees encode distance, direction, and quality of resources in a figure-eight dance pattern. Waggle encodes execution environments, runs code, and communicates structured results back to the AI.

**Ecosystem**: [ToolHive](https://github.com/stacklok/toolhive) (platform) -> [Propolis](https://github.com/stacklok/propolis) (VM substrate) -> **Waggle** (execution/communication layer)

## Overview

Waggle exposes 8 MCP tools over Streamable HTTP that let AI agents:

- **Create** isolated Python, Node.js, or shell environments (each is a real microVM)
- **Execute** multi-line code with separate stdout/stderr capture
- **Read/write files** inside environments
- **Install packages** (pip, npm, apk)
- **Manage** environment lifecycle with automatic timeout-based cleanup

Each environment runs in its own [libkrun](https://github.com/containers/libkrun) microVM, providing true VM-level isolation -- stronger than containers, with near-container startup times.

## Architecture

```
AI Agent (MCP Client)
        |
        | Streamable HTTP (/mcp)
        v
+------------------+     +------------------+     +------------------+
|   MCP Layer      | --> |  App Services    | --> |  Infrastructure  |
|   (mcp-go)       |     |  Env/Exec/FS     |     |  Propolis + SSH  |
+------------------+     +------------------+     +--------+---------+
                                                           |
                                                  SSH over localhost
                                                           |
                                              +------------v-----------+
                                              |  microVM (libkrun)     |
                                              |  Python / Node / Shell |
                                              |  SSH server            |
                                              +------------------------+
```

The project follows strict **Domain-Driven Design**:

| Layer | Package | Responsibility |
|-------|---------|----------------|
| Domain | `pkg/domain/` | Pure business types, interfaces, state machine -- no external dependencies |
| Service | `pkg/service/` | Application orchestration (environment lifecycle, code execution, file ops) |
| Infrastructure | `pkg/infra/` | Propolis adapter, SSH executor/filesystem, in-memory store, port allocator |
| Interface | `pkg/mcp/` | MCP tool definitions, handlers, server assembly |
| Support | `pkg/config/`, `pkg/cleanup/` | Configuration, background reaper |

## MCP Tools

| Tool | Description | Key Inputs |
|------|-------------|------------|
| `create_environment` | Spin up an isolated microVM | `runtime` (python/node/shell), `name`, `timeout_minutes` |
| `destroy_environment` | Tear down a microVM | `environment_id` |
| `list_environments` | List all active environments | -- |
| `execute` | Run code or a shell command | `environment_id`, `code`, `language`, `timeout_seconds` |
| `write_file` | Write content into the VM | `environment_id`, `path`, `content` |
| `read_file` | Read a file from the VM | `environment_id`, `path` |
| `list_files` | List directory contents | `environment_id`, `path` |
| `install_packages` | Install language packages | `environment_id`, `packages` |

## Prerequisites

- Go 1.25.6+
- [Task](https://taskfile.dev/) (build system)
- [propolis](https://github.com/stacklok/propolis) checked out as a sibling directory (used via `go.mod replace`)
- `propolis-runner` binary built and available in PATH (see propolis docs)
- libkrun installed (Linux: `libkrun-devel`, macOS: via Homebrew)
- KVM access on Linux (`/dev/kvm`) or Hypervisor.framework on macOS
- OCI images for your target runtimes (see [Configuration](#configuration))

## Quick Start

```bash
# Clone alongside propolis
cd ~/Development/stacklok
git clone https://github.com/stacklok/waggle.git
# propolis should already be at ../propolis

# Build
cd waggle
task build

# Configure runtime images and run
export WAGGLE_IMAGE_PYTHON=ghcr.io/stacklok/waggle-python:latest
export WAGGLE_IMAGE_NODE=ghcr.io/stacklok/waggle-node:latest
export WAGGLE_IMAGE_SHELL=alpine:latest
task run
```

The server starts on `127.0.0.1:8080` with the MCP endpoint at `/mcp`.

## Configuration

All configuration is via environment variables with the `WAGGLE_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `WAGGLE_LISTEN_ADDR` | `127.0.0.1:8080` | Server listen address |
| `WAGGLE_DEFAULT_CPUS` | `1` | vCPUs per environment |
| `WAGGLE_DEFAULT_MEMORY_MB` | `512` | RAM in MiB per environment |
| `WAGGLE_MAX_ENVIRONMENTS` | `10` | Maximum concurrent environments |
| `WAGGLE_DEFAULT_TIMEOUT_MIN` | `30` | Default inactivity timeout (minutes) |
| `WAGGLE_BOOT_TIMEOUT` | `2m` | Max time to wait for VM boot |
| `WAGGLE_DEFAULT_EXEC_TIMEOUT` | `30s` | Default code execution timeout |
| `WAGGLE_MAX_EXEC_TIMEOUT` | `5m` | Maximum allowed execution timeout |
| `WAGGLE_SSH_PORT_BASE` | `10000` | Start of SSH port range |
| `WAGGLE_SSH_PORT_MAX` | `11000` | End of SSH port range |
| `WAGGLE_DATA_DIR` | `~/.config/waggle` | State, SSH keys, cache directory |
| `WAGGLE_RUNNER_PATH` | _(auto-detect)_ | Explicit path to `propolis-runner` |
| `WAGGLE_LIB_DIR` | _(system)_ | Path to libkrun libraries |
| `WAGGLE_REAPER_INTERVAL` | `1m` | How often to check for expired environments |
| `WAGGLE_IMAGE_PYTHON` | _(required)_ | OCI image for Python environments |
| `WAGGLE_IMAGE_NODE` | _(required)_ | OCI image for Node.js environments |
| `WAGGLE_IMAGE_SHELL` | _(required)_ | OCI image for shell environments |

## Development

```bash
task build           # Build the binary
task test            # Run tests with race detector
task lint            # Run golangci-lint
task verify          # fmt + lint + test (CI gate)
task tidy            # go mod tidy
task gen             # Generate mocks
task clean           # Remove build artifacts
```

Run a single test:
```bash
go test -v -race -run TestEnvironmentServiceCreate ./pkg/service/
```

## Security

- **VM-level isolation**: Each environment is a separate libkrun microVM with its own kernel
- **Localhost binding**: Server binds to `127.0.0.1` by default
- **Per-environment SSH keys**: ECDSA P-256 keys generated per environment, stored with `0600` permissions
- **Shell escaping**: All user-provided strings are escaped via `propolis/ssh.ShellEscape()` before SSH execution
- **Resource limits**: Configurable max environments, CPU, memory, and execution timeouts
- **Automatic cleanup**: Background reaper destroys environments that exceed their inactivity timeout
- **No host filesystem exposure**: All file operations are scoped within the VM

## License

Apache-2.0 -- see [LICENSE](LICENSE).
