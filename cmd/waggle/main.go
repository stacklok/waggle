// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package main provides the entry point for the waggle MCP server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"

	"github.com/stacklok/waggle/pkg/cleanup"
	"github.com/stacklok/waggle/pkg/config"
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
	sshExecutor := ssh.NewExecutor(repo, provider)
	sshFS := ssh.NewFileSystem(repo, provider)

	// Create application services.
	envSvc := service.NewEnvironmentService(repo, provider, portAlloc, cfg)
	execSvc := service.NewExecutionService(repo, sshExecutor, envSvc, cfg)
	fsSvc := service.NewFilesystemService(repo, sshFS, envSvc)

	// Create and configure MCP server.
	mcpServer := wagmcp.NewServer(version, envSvc, execSvc, fsSvc)

	httpServer := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath("/mcp"),
	)

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
		errCh <- httpServer.Start(cfg.ListenAddr)
	}()

	// Wait for shutdown signal or server error.
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	// Clean up all environments on shutdown.
	slog.Info("cleaning up environments")
	envs, _ := envSvc.List(context.Background())
	for _, env := range envs {
		if destroyErr := envSvc.Destroy(context.Background(), env.ID); destroyErr != nil {
			slog.Error("failed to destroy environment on shutdown",
				"id", env.ID, "error", destroyErr)
		}
	}

	slog.Info("waggle stopped")
	return nil
}
