// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

const (
	// EnvPrefix is the prefix for all waggle environment variables.
	EnvPrefix = "WAGGLE_"

	defaultListenAddr      = "127.0.0.1:8080"
	defaultCPUs            = 1
	defaultMemoryMB        = 512
	defaultMaxEnvironments = 10
	defaultTimeoutMin      = 30
	defaultBootTimeout     = 2 * time.Minute
	defaultExecTimeout     = 30 * time.Second
	defaultMaxExecTimeout  = 5 * time.Minute
	defaultSSHPortBase     = 10000
	defaultSSHPortMax      = 11000
	defaultReaperInterval  = time.Minute
	defaultDataDirName     = "waggle"
	defaultImagePython     = "ghcr.io/stacklok/waggle/python:latest"
	defaultImageNode       = "ghcr.io/stacklok/waggle/node:latest"
	defaultImageShell      = "ghcr.io/stacklok/waggle/shell:latest"
)

// Config holds the waggle server configuration.
type Config struct {
	// ListenAddr is the address the MCP server listens on.
	ListenAddr string

	// DefaultCPUs is the number of vCPUs per environment.
	DefaultCPUs uint32

	// DefaultMemoryMB is the RAM in MiB per environment.
	DefaultMemoryMB uint32

	// MaxEnvironments is the maximum number of concurrent environments.
	MaxEnvironments int

	// DefaultTimeoutMin is the default inactivity timeout in minutes
	// before an environment is automatically destroyed.
	DefaultTimeoutMin int

	// BootTimeout is the maximum time to wait for a VM to boot and SSH
	// to become ready.
	BootTimeout time.Duration

	// DefaultExecTimeout is the default execution timeout for commands.
	DefaultExecTimeout time.Duration

	// MaxExecTimeout is the maximum allowed execution timeout.
	MaxExecTimeout time.Duration

	// SSHPortBase is the start of the port range for SSH port allocation.
	SSHPortBase uint16

	// SSHPortMax is the end of the port range for SSH port allocation.
	SSHPortMax uint16

	// DataDir is the directory for environment state, SSH keys, and cache.
	DataDir string

	// RunnerPath is an optional explicit path to the propolis-runner binary.
	RunnerPath string

	// InitPath is an optional explicit path to the waggle-init binary
	// that runs as PID 1 inside guest VMs.
	InitPath string

	// LibDir is an optional path to the libkrun library directory.
	LibDir string

	// Images maps runtime names to OCI image references.
	Images map[string]string

	// ReaperInterval is how often the background reaper checks for
	// expired environments.
	ReaperInterval time.Duration
}

// Default returns a Config with all default values.
func Default() *Config {
	return &Config{
		ListenAddr:         defaultListenAddr,
		DefaultCPUs:        defaultCPUs,
		DefaultMemoryMB:    defaultMemoryMB,
		MaxEnvironments:    defaultMaxEnvironments,
		DefaultTimeoutMin:  defaultTimeoutMin,
		BootTimeout:        defaultBootTimeout,
		DefaultExecTimeout: defaultExecTimeout,
		MaxExecTimeout:     defaultMaxExecTimeout,
		SSHPortBase:        defaultSSHPortBase,
		SSHPortMax:         defaultSSHPortMax,
		DataDir:            defaultDataDir(),
		Images:             defaultImages(),
		ReaperInterval:     defaultReaperInterval,
	}
}

// LoadFromEnv returns a Config populated from environment variables,
// falling back to defaults for any unset variables.
func LoadFromEnv() *Config {
	cfg := Default()
	loadEnvStrings(cfg)
	loadEnvNumerics(cfg)

	// Load runtime images from WAGGLE_IMAGE_<RUNTIME> env vars.
	for _, rt := range environment.ValidRuntimes {
		key := EnvPrefix + "IMAGE_" + envKey(string(rt))
		if v := os.Getenv(key); v != "" {
			cfg.Images[string(rt)] = v
		}
	}

	return cfg
}

// loadEnvStrings applies string-typed environment variables to cfg.
func loadEnvStrings(cfg *Config) {
	if v := os.Getenv(EnvPrefix + "LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv(EnvPrefix + "DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv(EnvPrefix + "RUNNER_PATH"); v != "" {
		cfg.RunnerPath = v
	}
	if v := os.Getenv(EnvPrefix + "INIT_PATH"); v != "" {
		cfg.InitPath = v
	}
	if v := os.Getenv(EnvPrefix + "LIB_DIR"); v != "" {
		cfg.LibDir = v
	}
}

// loadEnvNumerics applies numeric-typed environment variables to cfg.
func loadEnvNumerics(cfg *Config) {
	if v := envUint32("DEFAULT_CPUS"); v > 0 {
		cfg.DefaultCPUs = v
	}
	if v := envUint32("DEFAULT_MEMORY_MB"); v > 0 {
		cfg.DefaultMemoryMB = v
	}
	if v := envInt("MAX_ENVIRONMENTS"); v > 0 {
		cfg.MaxEnvironments = v
	}
	if v := envInt("DEFAULT_TIMEOUT_MIN"); v > 0 {
		cfg.DefaultTimeoutMin = v
	}
	if v := envDuration("BOOT_TIMEOUT"); v > 0 {
		cfg.BootTimeout = v
	}
	if v := envDuration("DEFAULT_EXEC_TIMEOUT"); v > 0 {
		cfg.DefaultExecTimeout = v
	}
	if v := envDuration("MAX_EXEC_TIMEOUT"); v > 0 {
		cfg.MaxExecTimeout = v
	}
	if v := envUint16("SSH_PORT_BASE"); v > 0 {
		cfg.SSHPortBase = v
	}
	if v := envUint16("SSH_PORT_MAX"); v > 0 {
		cfg.SSHPortMax = v
	}
	if v := envDuration("REAPER_INTERVAL"); v > 0 {
		cfg.ReaperInterval = v
	}
}

// Validate checks the configuration for logical consistency.
func (c *Config) Validate() error {
	if c.ListenAddr == "" {
		return fmt.Errorf("listen address must not be empty")
	}
	if c.DefaultCPUs == 0 {
		return fmt.Errorf("default CPUs must be greater than zero")
	}
	if c.DefaultMemoryMB == 0 {
		return fmt.Errorf("default memory must be greater than zero")
	}
	if c.MaxEnvironments <= 0 {
		return fmt.Errorf("max environments must be greater than zero")
	}
	if c.SSHPortBase >= c.SSHPortMax {
		return fmt.Errorf("SSH port base (%d) must be less than max (%d)", c.SSHPortBase, c.SSHPortMax)
	}
	if c.DataDir == "" {
		return fmt.Errorf("data directory must not be empty")
	}
	if c.DefaultExecTimeout > c.MaxExecTimeout {
		return fmt.Errorf("default exec timeout (%v) must not exceed max (%v)", c.DefaultExecTimeout, c.MaxExecTimeout)
	}
	if c.InitPath != "" {
		if _, err := os.Stat(c.InitPath); err != nil {
			return fmt.Errorf("init binary not found at %s: %w", c.InitPath, err)
		}
	}
	return nil
}

// ImageRef returns the OCI image reference for the given runtime,
// or an empty string if none is configured.
func (c *Config) ImageRef(rt environment.Runtime) string {
	return c.Images[string(rt)]
}

func defaultDataDir() string {
	if dir := os.Getenv(EnvPrefix + "DATA_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), defaultDataDirName)
	}
	return filepath.Join(home, ".config", defaultDataDirName)
}

func defaultImages() map[string]string {
	return map[string]string{
		string(environment.RuntimePython): defaultImagePython,
		string(environment.RuntimeNode):   defaultImageNode,
		string(environment.RuntimeShell):  defaultImageShell,
	}
}

func envInt(name string) int {
	v := os.Getenv(EnvPrefix + name)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func envUint32(name string) uint32 {
	n := envInt(name)
	if n < 0 || n > math.MaxUint32 {
		return 0
	}
	return uint32(n) //nolint:gosec // n is validated to be non-negative above
}

func envUint16(name string) uint16 {
	n := envInt(name)
	if n < 0 || n > 65535 {
		return 0
	}
	return uint16(n)
}

func envDuration(name string) time.Duration {
	v := os.Getenv(EnvPrefix + name)
	if v == "" {
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0
	}
	return d
}

func envKey(s string) string {
	result := make([]byte, len(s))
	for i := range s {
		if s[i] >= 'a' && s[i] <= 'z' {
			result[i] = s[i] - 'a' + 'A'
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}
