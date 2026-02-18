// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeChecker struct {
	name string
	err  error
}

func (f *fakeChecker) Name() string                  { return f.name }
func (f *fakeChecker) Check(_ context.Context) error { return f.err }

func TestHandleHealthz(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h.HandleHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp statusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}
}

func TestHandleReadyzAllHealthy(t *testing.T) {
	t.Parallel()

	h := NewHandler(
		&fakeChecker{name: "db", err: nil},
		&fakeChecker{name: "cache", err: nil},
	)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.HandleReadyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp statusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}
	if resp.Checks["db"] != "ok" {
		t.Errorf("checks[db] = %q, want %q", resp.Checks["db"], "ok")
	}
	if resp.Checks["cache"] != "ok" {
		t.Errorf("checks[cache] = %q, want %q", resp.Checks["cache"], "ok")
	}
}

func TestHandleReadyzOneUnhealthy(t *testing.T) {
	t.Parallel()

	h := NewHandler(
		&fakeChecker{name: "db", err: nil},
		&fakeChecker{name: "cache", err: errors.New("connection refused")},
	)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.HandleReadyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp statusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "unavailable" {
		t.Errorf("status = %q, want %q", resp.Status, "unavailable")
	}
	if resp.Checks["db"] != "ok" {
		t.Errorf("checks[db] = %q, want %q", resp.Checks["db"], "ok")
	}
	if resp.Checks["cache"] != "connection refused" {
		t.Errorf("checks[cache] = %q, want %q", resp.Checks["cache"], "connection refused")
	}
}

func TestHandleReadyzNoCheckers(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.HandleReadyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp statusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}
}
