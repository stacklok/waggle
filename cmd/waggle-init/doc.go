// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

// Package main provides the entry point for waggle-init, a minimal init
// process that runs as PID 1 inside guest VMs. It starts a zombie reaper,
// configures the system (mounts, network, hardening), and launches an
// embedded SSH server via propolis guest/boot.
package main
