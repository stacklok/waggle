// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/stacklok/propolis/guest/boot"
	"github.com/stacklok/propolis/guest/reaper"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Start zombie reaper — PID 1 responsibility.
	stopReaper := reaper.Start(logger)
	defer stopReaper()

	// Boot: mounts, network, hardening, SSH server.
	shutdown, err := boot.Run(logger,
		boot.WithSSHPort(22),
		boot.WithSSHKeysPath("/root/.ssh/authorized_keys"),
		boot.WithUser("root", "/root", "/bin/sh", 0, 0),
		// Root lockdown is unnecessary because SSH sessions run as root (UID 0).
		// The security boundary is the KVM hypervisor, not in-guest permissions.
		boot.WithLockdownRoot(false),
	)
	if err != nil {
		logger.Error("boot failed", "error", err)
		os.Exit(1)
	}

	// Block until SIGTERM or SIGINT.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	received := <-sig
	logger.Info("received signal, shutting down", "signal", received)

	shutdown()
}
