// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package main provides the entry point for the waggle MCP server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/adrg/xdg"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stacklok/propolis/image"

	"github.com/stacklok/waggle/pkg/cleanup"
	"github.com/stacklok/waggle/pkg/config"
	"github.com/stacklok/waggle/pkg/health"
	"github.com/stacklok/waggle/pkg/infra/ssh"
	"github.com/stacklok/waggle/pkg/infra/store"
	"github.com/stacklok/waggle/pkg/infra/vm"
	"github.com/stacklok/waggle/pkg/infra/vm/runtimebin"
	wagmcp "github.com/stacklok/waggle/pkg/mcp"
	"github.com/stacklok/waggle/pkg/service"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("waggle %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	logFile, err := parseFlags(os.Args[1:])
	if err != nil {
		slog.Error("failed to parse flags", "error", err)
		os.Exit(2)
	}

	if err := run(logFile); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (string, error) {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var logFile string
	fs.StringVar(&logFile, "log-file", "", "write logs to file (append)")

	if err := fs.Parse(args); err != nil {
		return "", err
	}

	return logFile, nil
}

func run(logFile string) error {
	closeLog, err := configureLogger(logFile)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	defer func() {
		if err := closeLog(); err != nil {
			slog.Error("failed to close log file", "error", err)
		}
	}()

	// Load and validate configuration.
	cfg := config.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	slog.Info("starting waggle",
		"version", version,
		"listen", cfg.ListenAddr,
		"max_environments", cfg.MaxEnvironments,
	)

	// Create infrastructure components.
	repo := store.NewMemoryStore()

	imageCache := initImageCache(cfg)

	// Wire VM provider options (embedded runtime when available).
	var providerOpts []vm.ProviderOption
	providerOpts = append(providerOpts, vm.WithImageCache(imageCache))
	if cfg.LogLevel > 0 {
		providerOpts = append(providerOpts, vm.WithLogLevel(cfg.LogLevel))
	}

	if runtimebin.Available() {
		if cfg.CacheDir == "" {
			cfg.CacheDir = runtimeCacheDir()
		}
		providerOpts = append(providerOpts,
			vm.WithRuntimeSource(runtimebin.RuntimeSource()),
			vm.WithFirmwareSource(runtimebin.FirmwareSource()),
		)
		slog.Info("using embedded propolis runtime", "version", runtimebin.Version)
	}

	provider := vm.NewPropolisProvider(providerOpts...)
	portAlloc := vm.NewPortAllocator(cfg.SSHPortBase, cfg.SSHPortMax)

	// Create domain adapters.
	sshExecutor := ssh.NewExecutor()
	sshFS := ssh.NewFileSystem()
	sshProber := ssh.NewProber()

	// Create application services.
	envSvc := service.NewEnvironmentService(repo, provider, portAlloc, sshProber, cfg)
	execSvc := service.NewExecutionService(repo, sshExecutor, provider, envSvc, cfg)
	fsSvc := service.NewFilesystemService(repo, sshFS, provider, envSvc)

	// Create and configure MCP server.
	mcpServer := wagmcp.NewServer(version, envSvc, execSvc, fsSvc)

	// Health endpoints with readiness checkers.
	healthHandler := health.NewHandler(
		health.NewRepositoryChecker(repo),
	)

	// Use StreamableHTTPServer as http.Handler (not .Start()).
	mcpHTTPHandler := server.NewStreamableHTTPServer(mcpServer)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHTTPHandler)
	mux.HandleFunc("/healthz", healthHandler.HandleHealthz)
	mux.HandleFunc("/readyz", healthHandler.HandleReadyz)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start background reaper for expired environments.
	reaper := cleanup.NewReaper(envSvc, repo, cfg.ReaperInterval)

	// Handle shutdown gracefully.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go reaper.Start(ctx)

	// Start server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("MCP server listening", "addr", cfg.ListenAddr, "endpoint", "/mcp")
		errCh <- httpSrv.ListenAndServe()
	}()

	// Wait for shutdown signal or server error.
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
	}

	// Drain HTTP connections first, then clean up environments.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	slog.Info("cleaning up environments")
	envs, _ := envSvc.List(shutdownCtx)
	for _, env := range envs {
		if destroyErr := envSvc.Destroy(shutdownCtx, env.ID); destroyErr != nil {
			slog.Error("failed to destroy environment on shutdown",
				"id", env.ID, "error", destroyErr)
		}
	}
	if shutdownCtx.Err() != nil {
		slog.Warn("shutdown deadline exceeded, some environments may not have been cleaned up")
	}

	slog.Info("waggle stopped")
	return nil
}

// initImageCache creates a shared OCI image cache and evicts stale entries.
func initImageCache(cfg *config.Config) *image.Cache {
	dir := cfg.ImageCacheDir
	if dir == "" {
		dir = filepath.Join(xdg.CacheHome, "waggle", "images")
	}
	cache := image.NewCache(dir)

	if cfg.ImageCacheMaxAge > 0 {
		if removed, err := cache.Evict(cfg.ImageCacheMaxAge); err != nil {
			slog.Warn("image cache eviction failed", "error", err)
		} else if removed > 0 {
			slog.Info("evicted stale image cache entries",
				"count", removed)
		}
	}
	return cache
}

// runtimeCacheDir returns the directory used for extracting embedded runtime
// binaries, under the XDG cache directory (e.g. ~/.cache/waggle/runtime/).
func runtimeCacheDir() string {
	return filepath.Join(xdg.CacheHome, "waggle", "runtime")
}

func configureLogger(logFile string) (func() error, error) {
	if logFile == "" {
		return func() error { return nil }, nil
	}

	// #nosec G304 G703 -- log file path is operator-configured, not external input.
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", logFile, err)
	}

	handler := slog.NewTextHandler(io.MultiWriter(os.Stderr, f), nil)
	slog.SetDefault(slog.New(handler))

	return f.Close, nil
}
