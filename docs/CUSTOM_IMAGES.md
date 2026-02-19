# Custom Images Checklist

Use this checklist when building or adopting your own OCI images for Waggle runtimes.

## Core Requirements (All Runtimes)
- A POSIX shell at `/bin/sh`.
- GNU coreutils with `base64` (Waggle uses `base64 -d` for SSH payloads).
- GNU findutils with `find -printf` (used by `list_files`).
- A writable home directory for the `sandbox` user.

## Runtime-Specific Requirements

### Python Runtime
- `python3` is present and runnable.
- `pip` is available for installs (prefer a venv or user install).
- If you ship a venv, ensure its interpreter and pip are on PATH or set overrides.

### Node Runtime
- `node` and `npm` are available.
- Global installs should target a writable prefix, or use overrides.

### Shell Runtime
- A package manager is available if you want `install_packages` to work:
  - `apk`, `apt-get`, `dnf`, `yum`, or `zypper`.
- The `sandbox` user must have permission to install packages (or expect install failures).

## Command Resolution Order
Waggle chooses commands in this order:

1) Config overrides via `WAGGLE_RUNTIME_*` env vars
2) Probed commands inside the VM (when available)
3) Built-in fallbacks (`python3`, `pip install`, `node`, `npm install -g`, `sh`, `apk add --no-cache`)

Overrides must be absolute paths and use simple, safe arguments. Examples:

```bash
export WAGGLE_RUNTIME_PYTHON_EXEC_COMMAND=/usr/bin/python3
export WAGGLE_RUNTIME_PYTHON_INSTALL_COMMAND="/usr/bin/pip3 install --no-cache-dir"
export WAGGLE_RUNTIME_NODE_EXEC_COMMAND=/usr/local/bin/node
export WAGGLE_RUNTIME_NODE_INSTALL_COMMAND="/usr/local/bin/npm install -g"
```

## Troubleshooting Tips
- If `install_packages` fails, verify the package manager path and permissions.
- If `execute` fails, verify the runtime binary exists at the resolved path.
- If `list_files` is empty or errors, confirm `find -printf` is available.
- For Python, `install_packages` will retry in a venv when PEP 668 blocks system installs.

## Capabilities Data
Waggle probes runtime capabilities (available binaries, package managers) to improve command resolution. This data is internal and not exposed over MCP by default. If we ever add an API for it, it will be redacted and opt-in to avoid leaking environment details.
