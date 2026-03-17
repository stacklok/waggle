# Waggle

[![Build](https://github.com/stacklok/waggle/actions/workflows/build.yml/badge.svg)](https://github.com/stacklok/waggle/actions/workflows/build.yml)
[![Tests](https://github.com/stacklok/waggle/actions/workflows/test.yml/badge.svg)](https://github.com/stacklok/waggle/actions/workflows/test.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/stacklok/waggle)](go.mod)

> [!WARNING]
> **Experimental** — Waggle is under active development. APIs, configuration, and behavior may change without notice. Use at your own risk and expect breaking changes between releases.

An MCP server that gives AI agents their own isolated coding environments. Each environment is a real microVM — spin one up, write code, execute it, install packages, read results, tear it down. All through 8 simple MCP tools.

> The waggle dance is one of the most fascinating things in nature — bees encode distance, direction, and quality of resources in a figure-eight dance pattern. Waggle encodes execution environments, runs code, and communicates structured results back to the AI.

## Table of Contents

- [What It Looks Like](#what-it-looks-like)
- [MCP Tools](#mcp-tools)
- [Why MicroVMs](#why-microvms)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Security](#security)
- [Runtime Images](#runtime-images)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## What It Looks Like

When an AI agent connects to Waggle, it gets tools to create and use isolated environments. Here's a typical interaction:

```
Agent: I need to analyze this CSV data. Let me set up an environment.

→ create_environment(runtime: "python", name: "data-analysis")
← { environment_id: "a1b2c3", status: "running", runtime: "python" }

→ install_packages(environment_id: "a1b2c3", packages: "pandas matplotlib")
← { exit_code: 0 }

→ write_file(environment_id: "a1b2c3", path: "/home/sandbox/data.csv", content: "name,score\nAlice,95\nBob,87\n...")
← { status: "written", bytes_written: 1024 }

→ execute(environment_id: "a1b2c3", code: """
  import pandas as pd
  df = pd.read_csv('/home/sandbox/data.csv')
  print(f"Mean score: {df['score'].mean():.1f}")
  print(f"Top performer: {df.loc[df['score'].idxmax(), 'name']}")
  """)
← { stdout: "Mean score: 91.0\nTop performer: Alice", stderr: "", exit_code: 0, duration_ms: 245 }

→ destroy_environment(environment_id: "a1b2c3")
← { status: "destroyed" }
```

The agent gets a full Linux environment with real package managers, a real filesystem, and real process isolation — not a sandbox or a REPL, but an actual VM.

## MCP Tools

### Environment Lifecycle

**`create_environment`** — Spin up a fresh isolated environment.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `runtime` | yes | `python`, `node`, or `shell` |
| `name` | no | Human-readable name (auto-generated if omitted) |
| `timeout_minutes` | no | Auto-destroy after inactivity (default: 30) |

Returns the `environment_id` used for all subsequent operations. Each environment is a separate [libkrun](https://github.com/containers/libkrun) microVM with its own filesystem, network stack, and process space.

**`destroy_environment`** — Tear down an environment and release all resources (VM, ports, SSH keys).

**`list_environments`** — List all active environments with their status, runtime, and creation time.

### Code Execution

**`execute`** — Run code or a shell command. Multi-line code is fully supported.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `environment_id` | yes | Target environment |
| `code` | yes | Code or shell command to run |
| `language` | no | Override: `python`, `node`, or `shell` (defaults to env runtime) |
| `timeout_seconds` | no | Execution timeout (default: 30, max: 300) |

Returns `stdout`, `stderr`, `exit_code`, and `duration_ms` — separately captured, so the agent can distinguish output from errors.

**`install_packages`** — Install language packages. Automatically uses the right package manager: `pip` for Python, `npm` for Node.js, `apk` for shell environments.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `environment_id` | yes | Target environment |
| `packages` | yes | Space-separated package names |

### File Operations

**`write_file`** — Write content to a file inside the environment. Creates parent directories automatically.

**`read_file`** — Read a file's content from the environment.

**`list_files`** — List directory contents with name, size, type, permissions, and modification time.

All file operations take `environment_id` and `path`. Files persist within the environment until it's destroyed.

## Why MicroVMs

Most code execution MCP servers use containers or V8 isolates. Waggle uses microVMs ([go-microvm](https://github.com/stacklok/go-microvm) + [libkrun](https://github.com/containers/libkrun)):

| | Containers | V8 Isolates | Waggle (microVMs) |
|---|---|---|---|
| **Isolation** | Shared kernel | Single language | Separate kernel per env |
| **Languages** | Any | JavaScript only | Any |
| **Filesystem** | Shared layers | None | Independent per env |
| **Package managers** | Yes | No | Yes (pip, npm, apk) |
| **Startup** | ~1s | ~1ms | ~2-5s |
| **Security boundary** | Namespace/cgroup | V8 sandbox | Hardware virtualization |

The trade-off is startup time (seconds vs milliseconds), but you get a real Linux environment where `pip install numpy` and `apt-get install ffmpeg` just work.

**Ecosystem**: [ToolHive](https://github.com/stacklok/toolhive) (platform) → [go-microvm](https://github.com/stacklok/go-microvm) (VM substrate) → **Waggle** (MCP interface)

## Quick Start

### Prerequisites

- [Go](https://go.dev/dl/) 1.26+
- [Task](https://taskfile.dev/) (`go install github.com/go-task/task/v3/cmd/task@latest`)
- [GitHub CLI](https://cli.github.com/) (`gh`) — used to download the go-microvm runtime
- KVM access on Linux (`/dev/kvm`) or Hypervisor.framework on macOS

### Build and Run

```bash
git clone https://github.com/stacklok/waggle.git
cd waggle
task build    # Downloads go-microvm runtime + firmware, builds a self-contained binary
task run      # Starts the MCP server
```

The server starts on `127.0.0.1:8080` with the MCP endpoint at `/mcp` (Streamable HTTP transport).

Verify it's running:

```bash
curl -s http://127.0.0.1:8080/healthz
```

## Configuration

All settings via `WAGGLE_*` environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `WAGGLE_LISTEN_ADDR` | `127.0.0.1:8080` | Server listen address |
| `WAGGLE_MAX_ENVIRONMENTS` | `10` | Max concurrent environments |
| `WAGGLE_DEFAULT_CPUS` | `1` | vCPUs per environment |
| `WAGGLE_DEFAULT_MEMORY_MB` | `512` | RAM (MiB) per environment |
| `WAGGLE_DEFAULT_TIMEOUT_MIN` | `30` | Inactivity timeout (minutes) |
| `WAGGLE_DEFAULT_EXEC_TIMEOUT` | `30s` | Default execution timeout |
| `WAGGLE_MAX_EXEC_TIMEOUT` | `5m` | Max execution timeout |
| `WAGGLE_IMAGE_PYTHON` | `ghcr.io/stacklok/waggle/python:latest` | OCI image for Python environments |
| `WAGGLE_IMAGE_NODE` | `ghcr.io/stacklok/waggle/node:latest` | OCI image for Node.js environments |
| `WAGGLE_IMAGE_SHELL` | `ghcr.io/stacklok/waggle/shell:latest` | OCI image for shell environments |
| `WAGGLE_RUNTIME_PYTHON_EXEC_COMMAND` | *(unset)* | Override Python exec command (absolute path) |
| `WAGGLE_RUNTIME_PYTHON_INSTALL_COMMAND` | *(unset)* | Override Python install command (absolute path + args) |
| `WAGGLE_RUNTIME_NODE_EXEC_COMMAND` | *(unset)* | Override Node exec command (absolute path) |
| `WAGGLE_RUNTIME_NODE_INSTALL_COMMAND` | *(unset)* | Override Node install command (absolute path + args) |
| `WAGGLE_RUNTIME_SHELL_EXEC_COMMAND` | *(unset)* | Override shell exec command (absolute path) |
| `WAGGLE_RUNTIME_SHELL_INSTALL_COMMAND` | *(unset)* | Override shell install command (absolute path + args) |

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the full configuration reference and development guide.

## Security

Each environment is a separate microVM with its own kernel — not a container sharing the host kernel. Additionally:

- Server binds to `127.0.0.1` by default (localhost only)
- Per-environment ECDSA P-256 SSH keys with `0600` permissions
- All user-provided strings shell-escaped before SSH execution
- Configurable resource limits (CPU, memory, max environments, timeouts)
- Background reaper auto-destroys idle environments
- No host filesystem exposed to environments

## Runtime Images

Waggle provides reference OCI images for each supported runtime and uses them by default. These are minimal Alpine containers with just the language runtime and required utilities.

| Image | Contents |
|-------|----------|
| `ghcr.io/stacklok/waggle/python:latest` | Alpine 3.21 + Python 3, pip, coreutils, findutils |
| `ghcr.io/stacklok/waggle/node:latest` | Alpine 3.21 + Node.js, npm, coreutils, findutils |
| `ghcr.io/stacklok/waggle/shell:latest` | Alpine 3.21 + coreutils, findutils |

Build all images locally:

```bash
task build-images          # Build all three runtime images
task build-image-python    # Build only the Python image
```

### Custom Images

You can use your own OCI images as long as they include:

- The language runtime for the target environment (`python3`, `node`, etc.)
- GNU coreutils (waggle's SSH executor requires `base64 -d`)
- GNU findutils (waggle's file listing requires `find -printf`)

See [docs/CUSTOM_IMAGES.md](docs/CUSTOM_IMAGES.md) for a full checklist and troubleshooting tips.

Runtime command selection order:

1) Config overrides via `WAGGLE_RUNTIME_*` env vars
2) Probed commands inside the VM (if available)
3) Built-in fallbacks (`python3`, `pip install`, `node`, `npm install -g`, `sh`, `apk add --no-cache`)

No SSH server is needed — `waggle-init` is injected into the VM at boot time by go-microvm and handles all communication.

## Development

```bash
task build    # Build binary
task test     # Tests with race detector
task verify   # Full CI gate (fmt + lint + test)
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the DDD layer breakdown and [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the full dev guide.

## Contributing

Contributions are welcome! Whether it's bug reports, feature requests, or pull requests — all contributions help.

To get started with development:

```bash
task verify   # Full CI gate: format, lint, test
```

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the full development guide, including how to add new MCP tools and runtimes.

## License

This project is licensed under the Apache 2.0 License — see the [LICENSE](LICENSE) file for details.
