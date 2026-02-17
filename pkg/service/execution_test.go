// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/execution"
	"github.com/stacklok/waggle/pkg/infra/store"
	"github.com/stacklok/waggle/pkg/infra/vm"
)

// fakeExecutor implements execution.Executor for testing.
type fakeExecutor struct {
	lastCommand string
	result      *execution.ExecResult
	err         error
}

func (f *fakeExecutor) Execute(
	_ context.Context, _ string, command string, _ time.Duration,
) (*execution.ExecResult, error) {
	f.lastCommand = command
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &execution.ExecResult{Stdout: "ok", ExitCode: 0}, nil
}

func setupExecTest(t *testing.T) (*ExecutionService, *fakeExecutor, string) {
	t.Helper()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })
	cfg := testConfig()

	envSvc := NewEnvironmentService(repo, provider, portAlloc, cfg)

	env, err := envSvc.Create(ctx, environment.RuntimePython, "exec-test", 30)
	if err != nil {
		t.Fatalf("Create env: %v", err)
	}

	executor := &fakeExecutor{}
	execSvc := NewExecutionService(repo, executor, envSvc, cfg)

	return execSvc, executor, env.ID
}

func TestExecutionServiceExecute(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, executor, envID := setupExecTest(t)

	result, err := svc.Execute(ctx, envID, "print('hello')", "", 0)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Stdout != "ok" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "ok")
	}

	// Verify the command uses python3 and writes to a temp file.
	if !strings.Contains(executor.lastCommand, "python3") {
		t.Errorf("command should contain python3, got: %s", executor.lastCommand)
	}
	if !strings.Contains(executor.lastCommand, "/tmp/waggle_") {
		t.Errorf("command should use temp file, got: %s", executor.lastCommand)
	}
	if !strings.Contains(executor.lastCommand, "print('hello')") {
		t.Errorf("command should contain user code, got: %s", executor.lastCommand)
	}
	if !strings.Contains(executor.lastCommand, "rm -f") {
		t.Errorf("command should clean up temp file, got: %s", executor.lastCommand)
	}
}

func TestExecutionServiceExecuteWithLanguageOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, executor, envID := setupExecTest(t)

	_, err := svc.Execute(ctx, envID, "console.log('hi')", "node", 0)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(executor.lastCommand, "node") {
		t.Errorf("command should use node, got: %s", executor.lastCommand)
	}
	if !strings.Contains(executor.lastCommand, ".js") {
		t.Errorf("command should use .js extension, got: %s", executor.lastCommand)
	}
}

func TestExecutionServiceExecuteInvalidLanguage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, _, envID := setupExecTest(t)

	_, err := svc.Execute(ctx, envID, "code", "ruby", 0)
	if !errors.Is(err, environment.ErrInvalidRuntime) {
		t.Errorf("err = %v, want ErrInvalidRuntime", err)
	}
}

func TestExecutionServiceExecuteNotRunning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	env := environment.New("stopped-id", "stopped", environment.RuntimePython, 10000, time.Hour)
	// Don't transition to Running, stays in Creating.
	_ = repo.Save(ctx, env)

	svc := NewExecutionService(repo, &fakeExecutor{}, nil, testConfig())

	_, err := svc.Execute(ctx, "stopped-id", "code", "", 0)
	if !errors.Is(err, environment.ErrNotRunning) {
		t.Errorf("err = %v, want ErrNotRunning", err)
	}
}

func TestExecutionServiceInstallPackages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, executor, envID := setupExecTest(t)

	_, err := svc.InstallPackages(ctx, envID, []string{"numpy", "pandas"})
	if err != nil {
		t.Fatalf("InstallPackages: %v", err)
	}

	if !strings.Contains(executor.lastCommand, "pip install") {
		t.Errorf("command should use pip install, got: %s", executor.lastCommand)
	}
	if !strings.Contains(executor.lastCommand, "numpy pandas") {
		t.Errorf("command should contain package names, got: %s", executor.lastCommand)
	}
}

func TestExecutionServiceInstallPackagesEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, _, envID := setupExecTest(t)

	result, err := svc.InstallPackages(ctx, envID, nil)
	if err != nil {
		t.Fatalf("InstallPackages: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0 for empty packages", result.ExitCode)
	}
}
