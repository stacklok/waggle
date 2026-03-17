# Architecture

This document describes Waggle's internal architecture, design decisions, and extension points.

## System Overview

Waggle is an MCP (Model Context Protocol) server that provides AI agents with isolated code execution environments. Each environment is a lightweight microVM powered by [go-microvm](https://github.com/stacklok/go-microvm)/[libkrun](https://github.com/containers/libkrun), giving true VM-level isolation with near-container startup times.

```
                     AI Agent (Claude, etc.)
                            |
                            | Streamable HTTP POST/GET (/mcp)
                            | JSON-RPC 2.0 over SSE
                            v
                    +-------+-------+
                    |  MCP Server   |  pkg/mcp/
                    |  (mcp-go)     |  tools.go, handlers.go, server.go
                    +-------+-------+
                            |
                  +---------+---------+
                  |                   |
          +-------v------+   +-------v------+
          |  Environment |   |  Execution   |  pkg/service/
          |  Service     |   |  Service     |  environment.go, execution.go,
          |              |   |  Filesystem  |  filesystem.go
          +-------+------+   |  Service     |
                  |           +-------+------+
                  |                   |
          +-------v------+   +-------v------+
          |  VMProvider  |   |  SSH         |  pkg/infra/
          |  (go-microvm)|   |  Executor +  |  vm/microvm.go, ssh/executor.go,
          |  PortAlloc   |   |  FileSystem  |  ssh/filesystem.go
          +-------+------+   +-------+------+
                  |                   |
                  +--------+----------+
                           |
                    SSH over localhost
                    (port-forwarded)
                           |
                  +--------v---------+
                  |   microVM        |
                  |   (libkrun)      |
                  |                  |
                  |   Python/Node/   |
                  |   Shell + SSH    |
                  +------------------+
```

## Domain-Driven Design

The codebase follows strict DDD layering. Dependencies only flow inward -- domain knows nothing about infrastructure.

### Domain Layer (`pkg/domain/`)

Pure business types and interfaces with zero external dependencies.

**Environment** (`pkg/domain/environment/`) is the aggregate root:

```
Status State Machine:

  Creating ──> Running ──> Destroying ──> Destroyed
      |            |
      └──> Error <─┘
```

- `Environment` struct: ID, Name, Runtime, Status, SSHPort, CreatedAt, LastUsed, Timeout
- `Runtime` enum: python, node, shell -- with methods for file extensions, exec commands, package managers
- `Repository` interface: Save, FindByID, FindAll, Delete, Count
- Typed errors: ErrNotFound, ErrNotRunning, ErrAlreadyExists, ErrInvalidTransition, ErrMaxEnvironments, ErrInvalidRuntime

**Execution** (`pkg/domain/execution/`):
- `Executor` interface: Execute(ctx, envID, command, timeout) -> ExecResult
- `ExecResult`: Stdout, Stderr, ExitCode, DurationMs

**Filesystem** (`pkg/domain/filesystem/`):
- `FileSystem` interface: WriteFile, ReadFile, ListFiles
- `FileInfo`: Name, Size, IsDir, Mode, Modified

### Service Layer (`pkg/service/`)

Application services orchestrate domain objects and infrastructure adapters.

**EnvironmentService** -- Full lifecycle orchestration:
1. Validate runtime, check capacity (`MaxEnvironments`)
2. Allocate SSH port from `PortAllocator`
3. Create `Environment` in `Creating` state
4. Call `VMProvider.CreateVM()` (SSH key gen, microvm.Run, SSH readiness wait)
5. Transition to `Running` (or `Error` on failure, releasing the port)

**ExecutionService** -- Code execution:
1. Verify environment is Running, touch for inactivity tracking
2. Resolve language (explicit override or environment default)
3. Build heredoc command: write code to temp file, execute, clean up
4. Delegate to `Executor.Execute()` with timeout

**FilesystemService** -- File operations:
- WriteFile: local temp file -> `ssh.Client.CopyTo()`
- ReadFile: `ssh.Client.Run("cat <path>")`
- ListFiles: `find -printf` with structured output parsing

### Infrastructure Layer (`pkg/infra/`)

Adapters implementing domain interfaces using concrete technologies.

**MicroVMProvider** (`pkg/infra/vm/microvm.go`):
- Wraps `microvm.Run()` with SSH key injection and readiness wait
- Maintains in-memory map of `envID -> vmEntry{vm, sshKeyPath}`
- Uses `WithRootFSHook` to inject authorized_keys before boot
- Uses `WithPostBoot` to wait for SSH via `ssh.Client.WaitForReady()`

**PortAllocator** (`pkg/infra/vm/portalloc.go`):
- Range-based allocation with `sync.Mutex`
- Verifies port availability via `net.Listen` probe
- Testable via `SetListenCheck()` injection

**SSHExecutor** (`pkg/infra/ssh/executor.go`):
- Uses `ssh.Client.RunStream()` for separate stdout/stderr capture
- Extracts exit codes from SSH error messages
- Applies timeout via `context.WithTimeout`

**SSHFilesystem** (`pkg/infra/ssh/filesystem.go`):
- WriteFile via local temp + `CopyTo()`
- ReadFile via `Run("cat ...")`
- ListFiles via `find -printf` with structured parsing

**MemoryStore** (`pkg/infra/store/memory.go`):
- `sync.RWMutex`-protected `map[string]*Environment`
- Copies on Save and FindByID to prevent aliasing mutations
- Sufficient for Phase 1 -- environments are ephemeral (lost on restart)

### MCP Interface Layer (`pkg/mcp/`)

**tools.go**: 8 tool definitions using mcp-go's fluent builder API. Each tool has a clear description, required/optional parameters with enums.

**handlers.go**: `ToolHandler` struct routes tool calls to services. Two error paths:
- User-facing: `mcp.NewToolResultError("message"), nil`
- Internal: `nil, error`

**server.go**: `NewServer()` assembles the MCPServer with tool registration, panic recovery, and logging.

### Support (`pkg/config/`, `pkg/cleanup/`)

**Config**: Flat struct loaded from `WAGGLE_*` environment variables with sensible defaults. Validated at startup.

**Reaper**: Background goroutine on a configurable interval. Sweeps all environments, destroys any where `time.Since(LastUsed) > Timeout`.

## How Code Execution Works

This is the critical path -- how user code goes from MCP tool call to execution inside a VM:

```
1. MCP Client sends:  execute(environment_id="abc", code="print('hello')")
2. Handler extracts args, calls ExecutionService.Execute()
3. ExecutionService:
   a. Verifies environment is Running
   b. Touches LastUsed (resets inactivity timer)
   c. Resolves runtime (python → python3, .py)
   d. Generates temp path: /tmp/waggle_<uuid>.py
   e. Builds heredoc command:
      cat > /tmp/waggle_abc123.py << 'WAGGLE_EOF_12345678'
      print('hello')
      WAGGLE_EOF_12345678
      python3 /tmp/waggle_abc123.py; __exit=$?; rm -f /tmp/waggle_abc123.py; exit $__exit
   f. Delegates to Executor.Execute()
4. SSHExecutor:
   a. Looks up environment SSH port and key path
   b. Creates go-microvm SSH client
   c. Calls RunStream() with separate stdout/stderr buffers
   d. Extracts exit code from SSH error (if non-zero)
   e. Returns ExecResult{Stdout, Stderr, ExitCode, DurationMs}
5. Handler serializes result as JSON, returns as MCP tool result
```

## Extension Points

- **New runtimes**: Add to `Runtime` enum in `pkg/domain/environment/runtime.go`, add image config
- **Persistent storage**: Implement `environment.Repository` backed by SQLite/Postgres
- **Network policies**: Use go-microvm `WithEgressPolicy()` or `WithFirewallRules()` in MicroVMProvider
- **VirtioFS mounts**: Add shared host directories via `microvm.WithVirtioFS()`
- **Custom images**: Build OCI images with pre-installed tools, configure via `WAGGLE_IMAGE_*`

## Concurrency Model

- MCP server handles concurrent requests via Go's HTTP goroutine model
- `MemoryStore` uses `sync.RWMutex` for concurrent environment access
- `PortAllocator` uses `sync.Mutex` for allocation/release
- `MicroVMProvider` uses `sync.RWMutex` for VM handle map
- Each environment is an independent VM -- no shared state between environments
- Reaper runs in its own goroutine, accesses environments through the store
