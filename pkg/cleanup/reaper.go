// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"context"
	"log/slog"
	"time"

	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/service"
)

// Reaper periodically checks for and destroys expired environments.
type Reaper struct {
	envSvc   *service.EnvironmentService
	repo     environment.Repository
	interval time.Duration
}

// NewReaper creates a new Reaper.
func NewReaper(
	envSvc *service.EnvironmentService,
	repo environment.Repository,
	interval time.Duration,
) *Reaper {
	return &Reaper{
		envSvc:   envSvc,
		repo:     repo,
		interval: interval,
	}
}

// Start runs the reaper loop until the context is cancelled.
// It blocks and should be called in a goroutine.
func (r *Reaper) Start(ctx context.Context) {
	slog.Info("reaper started", "interval", r.interval)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("reaper stopped")
			return
		case <-ticker.C:
			r.sweep(ctx)
		}
	}
}

// sweep checks all environments and destroys expired ones.
func (r *Reaper) sweep(ctx context.Context) {
	envs, err := r.repo.FindAll(ctx)
	if err != nil {
		slog.Error("reaper: failed to list environments", "error", err)
		return
	}

	for _, env := range envs {
		if !env.IsRunning() || !env.IsExpired() {
			continue
		}

		slog.Info("reaper: destroying expired environment",
			"id", env.ID,
			"name", env.Name,
			"last_used", env.LastUsed,
			"timeout", env.Timeout,
		)

		if destroyErr := r.envSvc.Destroy(ctx, env.ID); destroyErr != nil {
			slog.Error("reaper: failed to destroy environment",
				"id", env.ID, "error", destroyErr)
		}
	}
}
