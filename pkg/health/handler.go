// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Checker is a minimal interface for readiness probes. Each checker
// verifies one dependency and reports its name and health status.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Handler serves HTTP health check endpoints.
type Handler struct {
	checkers []Checker
}

// NewHandler creates a Handler with the given readiness checkers.
func NewHandler(checkers ...Checker) *Handler {
	return &Handler{checkers: checkers}
}

type statusResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// HandleHealthz is an unconditional liveness probe. It always returns
// 200 OK with {"status":"ok"}.
func (*Handler) HandleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(statusResponse{Status: "ok"})
}

// HandleReadyz runs all registered checkers. It returns 200 if all pass,
// or 503 if any fail.
func (h *Handler) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string, len(h.checkers))
	healthy := true

	for _, c := range h.checkers {
		if err := c.Check(r.Context()); err != nil {
			checks[c.Name()] = err.Error()
			healthy = false
			slog.Warn("readiness check failed", "checker", c.Name(), "error", err)
		} else {
			checks[c.Name()] = "ok"
		}
	}

	resp := statusResponse{Checks: checks}
	status := http.StatusOK
	if healthy {
		resp.Status = "ok"
	} else {
		resp.Status = "unavailable"
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
