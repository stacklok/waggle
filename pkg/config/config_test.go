// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
	"time"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := Default()

	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, "127.0.0.1:8080")
	}
	if cfg.DefaultCPUs != 1 {
		t.Errorf("DefaultCPUs = %d, want %d", cfg.DefaultCPUs, 1)
	}
	if cfg.DefaultMemoryMB != 512 {
		t.Errorf("DefaultMemoryMB = %d, want %d", cfg.DefaultMemoryMB, 512)
	}
	if cfg.MaxEnvironments != 10 {
		t.Errorf("MaxEnvironments = %d, want %d", cfg.MaxEnvironments, 10)
	}
	if cfg.SSHPortBase != 10000 {
		t.Errorf("SSHPortBase = %d, want %d", cfg.SSHPortBase, 10000)
	}
	if cfg.SSHPortMax != 11000 {
		t.Errorf("SSHPortMax = %d, want %d", cfg.SSHPortMax, 11000)
	}
	if cfg.DataDir == "" {
		t.Error("DataDir should not be empty")
	}
	if cfg.ImageRef(environment.RuntimePython) != defaultImagePython {
		t.Errorf("ImageRef(python) = %q, want %q", cfg.ImageRef(environment.RuntimePython), defaultImagePython)
	}
	if cfg.ImageRef(environment.RuntimeNode) != defaultImageNode {
		t.Errorf("ImageRef(node) = %q, want %q", cfg.ImageRef(environment.RuntimeNode), defaultImageNode)
	}
	if cfg.ImageRef(environment.RuntimeShell) != defaultImageShell {
		t.Errorf("ImageRef(shell) = %q, want %q", cfg.ImageRef(environment.RuntimeShell), defaultImageShell)
	}
	if len(cfg.RuntimeCommands) != 0 {
		t.Errorf("RuntimeCommands = %v, want empty", cfg.RuntimeCommands)
	}
	if cfg.ImageCacheMaxAge != 7*24*time.Hour {
		t.Errorf("ImageCacheMaxAge = %v, want %v", cfg.ImageCacheMaxAge, 7*24*time.Hour)
	}
	if cfg.ImageCacheDir != "" {
		t.Errorf("ImageCacheDir = %q, want empty", cfg.ImageCacheDir)
	}
	if cfg.LogLevel != 0 {
		t.Errorf("LogLevel = %d, want 0", cfg.LogLevel)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:   "default config is valid",
			modify: func(_ *Config) {},
		},
		{
			name:    "empty listen addr",
			modify:  func(c *Config) { c.ListenAddr = "" },
			wantErr: true,
		},
		{
			name:    "zero CPUs",
			modify:  func(c *Config) { c.DefaultCPUs = 0 },
			wantErr: true,
		},
		{
			name:    "zero memory",
			modify:  func(c *Config) { c.DefaultMemoryMB = 0 },
			wantErr: true,
		},
		{
			name:    "zero max environments",
			modify:  func(c *Config) { c.MaxEnvironments = 0 },
			wantErr: true,
		},
		{
			name:    "port base >= max",
			modify:  func(c *Config) { c.SSHPortBase = 11000; c.SSHPortMax = 10000 },
			wantErr: true,
		},
		{
			name:    "port base == max",
			modify:  func(c *Config) { c.SSHPortBase = 10000; c.SSHPortMax = 10000 },
			wantErr: true,
		},
		{
			name:    "empty data dir",
			modify:  func(c *Config) { c.DataDir = "" },
			wantErr: true,
		},
		{
			name: "exec timeout exceeds max",
			modify: func(c *Config) {
				c.DefaultExecTimeout = c.MaxExecTimeout + 1
			},
			wantErr: true,
		},
		{
			name:    "init path does not exist",
			modify:  func(c *Config) { c.InitPath = "/nonexistent/waggle-init" },
			wantErr: true,
		},
		{
			name: "log level clamped to max",
			modify: func(c *Config) {
				c.LogLevel = 10
			},
		},
		{
			name: "log level at max is valid",
			modify: func(c *Config) {
				c.LogLevel = 5
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := Default()
			tt.modify(cfg)
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Not parallel: modifies environment variables.
	t.Setenv("WAGGLE_LISTEN_ADDR", "0.0.0.0:9090")
	t.Setenv("WAGGLE_DEFAULT_CPUS", "4")
	t.Setenv("WAGGLE_DEFAULT_MEMORY_MB", "2048")
	t.Setenv("WAGGLE_MAX_ENVIRONMENTS", "20")
	t.Setenv("WAGGLE_DEFAULT_TIMEOUT_MIN", "60")
	t.Setenv("WAGGLE_SSH_PORT_BASE", "20000")
	t.Setenv("WAGGLE_SSH_PORT_MAX", "21000")
	t.Setenv("WAGGLE_DATA_DIR", "/tmp/waggle-test")
	t.Setenv("WAGGLE_RUNNER_PATH", "/usr/bin/propolis-runner")
	t.Setenv("WAGGLE_INIT_PATH", "/usr/bin/waggle-init")
	t.Setenv("WAGGLE_LIB_DIR", "/usr/lib")
	t.Setenv("WAGGLE_REAPER_INTERVAL", "2m")
	t.Setenv("WAGGLE_IMAGE_CACHE_DIR", "/tmp/waggle-images")
	t.Setenv("WAGGLE_IMAGE_CACHE_MAX_AGE", "48h")
	t.Setenv("WAGGLE_LOG_LEVEL", "3")
	t.Setenv("WAGGLE_IMAGE_PYTHON", "ghcr.io/stacklok/waggle-python:latest")
	t.Setenv("WAGGLE_RUNTIME_PYTHON_EXEC_COMMAND", "/usr/bin/python")
	t.Setenv("WAGGLE_RUNTIME_PYTHON_INSTALL_COMMAND", "/usr/bin/pip install")

	cfg := LoadFromEnv()

	if cfg.ListenAddr != "0.0.0.0:9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, "0.0.0.0:9090")
	}
	if cfg.DefaultCPUs != 4 {
		t.Errorf("DefaultCPUs = %d, want %d", cfg.DefaultCPUs, 4)
	}
	if cfg.DefaultMemoryMB != 2048 {
		t.Errorf("DefaultMemoryMB = %d, want %d", cfg.DefaultMemoryMB, 2048)
	}
	if cfg.MaxEnvironments != 20 {
		t.Errorf("MaxEnvironments = %d, want %d", cfg.MaxEnvironments, 20)
	}
	if cfg.DefaultTimeoutMin != 60 {
		t.Errorf("DefaultTimeoutMin = %d, want %d", cfg.DefaultTimeoutMin, 60)
	}
	if cfg.SSHPortBase != 20000 {
		t.Errorf("SSHPortBase = %d, want %d", cfg.SSHPortBase, 20000)
	}
	if cfg.SSHPortMax != 21000 {
		t.Errorf("SSHPortMax = %d, want %d", cfg.SSHPortMax, 21000)
	}
	if cfg.DataDir != "/tmp/waggle-test" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "/tmp/waggle-test")
	}
	if cfg.RunnerPath != "/usr/bin/propolis-runner" {
		t.Errorf("RunnerPath = %q, want %q", cfg.RunnerPath, "/usr/bin/propolis-runner")
	}
	if cfg.InitPath != "/usr/bin/waggle-init" {
		t.Errorf("InitPath = %q, want %q", cfg.InitPath, "/usr/bin/waggle-init")
	}
	if cfg.LibDir != "/usr/lib" {
		t.Errorf("LibDir = %q, want %q", cfg.LibDir, "/usr/lib")
	}
	if cfg.ReaperInterval.String() != "2m0s" {
		t.Errorf("ReaperInterval = %v, want 2m0s", cfg.ReaperInterval)
	}
	if cfg.ImageCacheDir != "/tmp/waggle-images" {
		t.Errorf("ImageCacheDir = %q, want %q", cfg.ImageCacheDir, "/tmp/waggle-images")
	}
	if cfg.ImageCacheMaxAge != 48*time.Hour {
		t.Errorf("ImageCacheMaxAge = %v, want %v", cfg.ImageCacheMaxAge, 48*time.Hour)
	}
	if cfg.LogLevel != 3 {
		t.Errorf("LogLevel = %d, want %d", cfg.LogLevel, 3)
	}
	want := "ghcr.io/stacklok/waggle-python:latest"
	if cfg.ImageRef(environment.RuntimePython) != want {
		t.Errorf("ImageRef(python) = %q, want %q", cfg.ImageRef(environment.RuntimePython), want)
	}
	if cfg.RuntimeExecCommand(environment.RuntimePython) != "/usr/bin/python" {
		t.Errorf("RuntimeExecCommand(python) = %q, want %q", cfg.RuntimeExecCommand(environment.RuntimePython), "/usr/bin/python")
	}
	if cfg.RuntimeInstallCommand(environment.RuntimePython) != "/usr/bin/pip install" {
		t.Errorf("RuntimeInstallCommand(python) = %q, want %q", cfg.RuntimeInstallCommand(environment.RuntimePython), "/usr/bin/pip install")
	}
}

func TestLoadFromEnvInvalidValues(t *testing.T) {
	// Invalid values should fall back to defaults.
	t.Setenv("WAGGLE_DEFAULT_CPUS", "not-a-number")
	t.Setenv("WAGGLE_REAPER_INTERVAL", "bad-duration")

	cfg := LoadFromEnv()

	if cfg.DefaultCPUs != defaultCPUs {
		t.Errorf("DefaultCPUs = %d, want default %d", cfg.DefaultCPUs, defaultCPUs)
	}
	if cfg.ReaperInterval != defaultReaperInterval {
		t.Errorf("ReaperInterval = %v, want default %v", cfg.ReaperInterval, defaultReaperInterval)
	}
}

func TestImageRef(t *testing.T) {
	t.Parallel()

	cfg := Default()
	cfg.Images["python"] = "myimage:latest"

	if got := cfg.ImageRef(environment.RuntimePython); got != "myimage:latest" {
		t.Errorf("ImageRef(python) = %q, want %q", got, "myimage:latest")
	}
	if got := cfg.ImageRef(environment.RuntimeNode); got != defaultImageNode {
		t.Errorf("ImageRef(node) = %q, want %q", got, defaultImageNode)
	}
}

func TestValidateLogLevelClamping(t *testing.T) {
	t.Parallel()

	cfg := Default()
	cfg.LogLevel = 10

	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != 5 {
		t.Errorf("LogLevel = %d after clamping, want 5", cfg.LogLevel)
	}
}

func TestEnvUint32OverflowGuard(t *testing.T) {
	// Not parallel: modifies environment variables.
	// A value that overflows uint32 (math.MaxUint32 + 1 = 4294967296).
	t.Setenv("WAGGLE_DEFAULT_CPUS", "4294967296")

	cfg := LoadFromEnv()

	// Overflow should be treated as invalid and fall back to default.
	if cfg.DefaultCPUs != defaultCPUs {
		t.Errorf("DefaultCPUs = %d after overflow input, want default %d", cfg.DefaultCPUs, defaultCPUs)
	}
}
