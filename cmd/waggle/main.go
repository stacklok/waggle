// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package main provides the entry point for the waggle MCP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/stacklok/waggle/pkg/cleanup"
	"github.com/stacklok/waggle/pkg/config"
	"github.com/stacklok/waggle/pkg/health"
	"github.com/stacklok/waggle/pkg/infra/ssh"
	"github.com/stacklok/waggle/pkg/infra/store"
	"github.com/stacklok/waggle/pkg/infra/vm"
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

	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
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
	provider := vm.NewPropolisProvider()
	portAlloc := vm.NewPortAllocator(cfg.SSHPortBase, cfg.SSHPortMax)

	// Create domain adapters.
	sshExecutor := ssh.NewExecutor()
	sshFS := ssh.NewFileSystem()

	// Create application services.
	envSvc := service.NewEnvironmentService(repo, provider, portAlloc, cfg)
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
