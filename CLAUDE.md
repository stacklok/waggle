# waggle

MCP server for isolated code execution via propolis microVMs. Go + mcp-go, Streamable HTTP transport. Module: `github.com/stacklok/waggle`.

## Commands

```bash
task build           # Build self-contained binary with embedded propolis runtime
task test            # go test -v -race ./...
task lint            # golangci-lint run ./...
task verify          # fmt + lint + test (CI gate)
task tidy            # go mod tidy
task gen             # go generate ./... (mocks)
task run             # Build and run server
```

Run a single test: `go test -v -race -run TestName ./pkg/path/to/package`

## Architecture

- `pkg/domain/` — Pure types and interfaces, no external deps. `environment/` is the aggregate root with a state machine (Creating→Running→Destroying→Destroyed|Error)
- `pkg/service/` — Orchestration: `EnvironmentService`, `ExecutionService`, `FilesystemService`
- `pkg/infra/` — Adapters: `vm/` (propolis), `ssh/` (executor+filesystem), `store/` (in-memory repo)
- `pkg/mcp/` — 8 MCP tool definitions + handlers + server assembly
- `pkg/cleanup/` — Background reaper for expired environments
- Entry point: `cmd/waggle/main.go` wires everything with DI, handles signals

## Things That Will Bite You

- **propolis is a tagged dependency (v0.0.16)**: `task build` embeds propolis-runner, libkrun, and libkrunfw into the binary (downloaded via `task fetch-runtime`/`task fetch-firmware`, verified with sha256sums). Use `build-dev-system` for the system libkrun-devel path.
- **Image cache and log level**: `WAGGLE_IMAGE_CACHE_DIR` (default `~/.cache/waggle/images/`) enables shared OCI image cache with layer-level caching and COW rootfs cloning. `WAGGLE_IMAGE_CACHE_MAX_AGE` (default `168h`/7d) controls eviction. `WAGGLE_LOG_LEVEL` (0-5, default 0) sets libkrun verbosity; levels > 3 leak hypervisor internals.
- **Layered images**: Runtime images (python/node/shell) inherit from `images/base/` via `ARG BASE_IMAGE`. Build base first: `task build-image-base`. All runtime image tasks depend on it automatically.
- **MCP error handling has two paths**: Return `mcp.NewToolResultError("msg"), nil` for user-facing errors (bad input, not found). Return `nil, err` only for internal server failures. Mixing these up breaks the MCP protocol.
- **Code execution uses temp files, not `-c`**: Multi-line code is written to `/tmp/waggle_<uuid>.<ext>` in the VM via heredoc, executed, then cleaned up. Using `python3 -c` or `node -e` breaks on complex code.
- **Shell escaping is mandatory**: Always use `propolis/ssh.ShellEscape()` for any user-provided string passed to SSH commands. Missing this is a command injection vulnerability.
- **MemoryStore returns copies**: Both `Save()` and `FindByID()` copy the struct to prevent aliasing bugs. Mutating a returned `*Environment` does not affect the store — you must call `Save()` again.
- **Port allocator verifies with net.Listen**: It doesn't just track allocations — it probes each port. Tests must call `portAlloc.SetListenCheck(func(_ uint16) error { return nil })` to skip real binding.
- **SPDX headers on every file**: Every `.go` and `.yaml` file needs both `SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.` and `SPDX-License-Identifier: Apache-2.0`. Linting will fail without them.

## Conventions

- `log/slog` exclusively — use `fmt.Errorf("context: %w", err)` for error wrapping
- Table-driven tests with `t.Parallel()` at both test and subtest level
- Every package has a `doc.go` with SPDX header
- Domain interfaces defined in `pkg/domain/`, implemented in `pkg/infra/`
- IMPORTANT: Stage specific files only. Use `git add file1 file2`, never `git add -A`

## Verification

After any change:
```bash
task verify          # Format, lint, test — the full CI gate
```

When tests fail, fix the implementation, not the tests.
