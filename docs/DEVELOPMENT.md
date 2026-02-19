# Development Guide

## Prerequisites

| Requirement | Purpose |
|-------------|---------|
| Go 1.25.6+ | Language runtime |
| [Task](https://taskfile.dev/) | Build system (`go install github.com/go-task/task/v3/cmd/task@latest`) |
| [golangci-lint](https://golangci-lint.run/) | Linting |
| [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) | Import formatting |
| [propolis](https://github.com/stacklok/propolis) | VM substrate (checked out as sibling directory) |
| libkrun-devel | VM hypervisor (Linux: `dnf install libkrun-devel`, macOS: `brew install libkrun`) |
| KVM access | Linux: ensure `/dev/kvm` is accessible to your user |

## Repository Layout

```
waggle/
├── cmd/waggle/main.go          # Entry point, DI wiring, signal handling
├── pkg/
│   ├── domain/                 # Pure business types (no external deps)
│   │   ├── environment/        # Aggregate root, state machine, Runtime enum
│   │   ├── execution/          # Executor interface, ExecResult
│   │   └── filesystem/         # FileSystem interface, FileInfo
│   ├── service/                # Application services (orchestration)
│   │   ├── environment.go      # Create, Destroy, List, Get, Touch
│   │   ├── execution.go        # Execute code, InstallPackages
│   │   └── filesystem.go       # WriteFile, ReadFile, ListFiles
│   ├── infra/                  # Infrastructure adapters
│   │   ├── vm/                 # PropolisProvider, PortAllocator
│   │   ├── ssh/                # SSHExecutor, SSHFilesystem
│   │   └── store/              # InMemoryStore
│   ├── mcp/                    # MCP tool definitions + handlers
│   ├── config/                 # Configuration (env vars)
│   └── cleanup/                # Background reaper
├── docs/                       # Architecture and development docs
├── Taskfile.yaml               # Build system
├── CLAUDE.md                   # AI assistant instructions
└── go.mod                      # Go module (with propolis replace)
```

## Getting Started

```bash
# Ensure propolis is checked out alongside waggle
ls ../propolis/go.mod  # Should exist

# Build
task build

# Run tests
task test

# Full CI check
task verify
```

## Build Commands

| Command | Description |
|---------|-------------|
| `task build` | Build the `waggle` binary to `bin/` (CGO_ENABLED=0) |
| `task test` | Run all tests with `-race` detector |
| `task lint` | Run golangci-lint |
| `task lint-fix` | Auto-fix lint issues |
| `task fmt` | Run `go fmt` + `goimports` |
| `task verify` | Format + lint + test (the full CI gate) |
| `task tidy` | `go mod tidy` |
| `task gen` | `go generate ./...` (mock generation) |
| `task clean` | Remove `bin/` and coverage files |
| `task run` | Build and run the server |
| `task version` | Print version info |
| `task test-coverage` | Generate HTML coverage report |

## Running a Single Test

```bash
go test -v -race -run TestEnvironmentServiceCreate ./pkg/service/
go test -v -race -run TestPortAllocatorConcurrent ./pkg/infra/vm/
```

## Adding a New MCP Tool

1. Add the tool constant in `pkg/mcp/tools.go`
2. Add the tool definition function (using mcp-go's fluent builder)
3. Add it to the `Tools()` slice
4. Add the handler method on `ToolHandler` in `pkg/mcp/handlers.go`
5. Register it in the switch statement in `pkg/mcp/server.go`
6. Add tests in `tools_test.go` and `handlers_test.go`

## Adding a New Runtime

1. Add the constant to `Runtime` in `pkg/domain/environment/runtime.go`
2. Add it to `ValidRuntimes` slice
3. Add cases in `Validate()`, `FileExtension()`, `ExecCommand()`, `PackageInstallCommand()`
4. Add tests in `runtime_test.go`
5. Configure the OCI image via `WAGGLE_IMAGE_<RUNTIME>` (or add a default image in config)

## Testing Patterns

### Table-Driven Tests
All tests should be table-driven with parallel execution:

```go
func TestSomething(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name string
        // ...
    }{
        {name: "case 1", /* ... */},
        {name: "case 2", /* ... */},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // test body
        })
    }
}
```

### Faking Infrastructure

Service tests use fake implementations defined in the test files:

- `fakeProvider` — implements `vm.VMProvider` (in-memory VM tracking)
- `fakeExecutor` — implements `execution.Executor` (records last command)
- `fakeFS` — implements `filesystem.FileSystem` (in-memory file map)

The `PortAllocator` is real but with `SetListenCheck()` to skip port probing:
```go
portAlloc := vm.NewPortAllocator(20000, 20100)
portAlloc.SetListenCheck(func(_ uint16) error { return nil })
```

## Error Handling

### Domain Errors
Typed sentinel errors in `pkg/domain/environment/errors.go`:
- `ErrNotFound` — environment doesn't exist
- `ErrNotRunning` — operation requires running environment
- `ErrMaxEnvironments` — capacity exceeded
- `ErrInvalidRuntime` — unsupported runtime string
- `ErrInvalidTransition` — illegal state machine transition

### MCP Error Handling
Two distinct paths in tool handlers:
- **User-facing** (validation, not-found): `return mcp.NewToolResultError("message"), nil`
- **Internal** (server bug): `return nil, err`

This distinction matters for the MCP protocol — user-facing errors are tool results (shown to the user), while internal errors are protocol-level failures.

### Error Wrapping
Always wrap with context: `fmt.Errorf("create VM: %w", err)`

## Code Style

- SPDX headers on every `.go` and `.yaml` file
- `log/slog` for logging (never `fmt.Println` or `log.Printf`)
- Every package has a `doc.go`
- Domain interfaces in `pkg/domain/`, implementations in `pkg/infra/`
- Imports ordered: stdlib, external, internal (enforced by gci in `.golangci.yml`)

## Running the Server

```bash
# Optional: override runtime images
# export WAGGLE_IMAGE_PYTHON=ghcr.io/stacklok/waggle/python:latest
# export WAGGLE_IMAGE_NODE=ghcr.io/stacklok/waggle/node:latest
# export WAGGLE_IMAGE_SHELL=ghcr.io/stacklok/waggle/shell:latest

# Optional: customize settings
export WAGGLE_LISTEN_ADDR=127.0.0.1:9090
export WAGGLE_MAX_ENVIRONMENTS=5
export WAGGLE_DEFAULT_TIMEOUT_MIN=60

# Run
task run
```

The server binds to `127.0.0.1:8080` by default with the MCP endpoint at `/mcp`.

## Testing with curl

```bash
# Initialize MCP session
curl -s -X POST http://127.0.0.1:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-11-25",
      "capabilities": {},
      "clientInfo": {"name": "curl-test", "version": "0.1"}
    }
  }' | jq .

# List tools (after initialization)
curl -s -X POST http://127.0.0.1:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "MCP-Protocol-Version: 2025-11-25" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }' | jq .
```
