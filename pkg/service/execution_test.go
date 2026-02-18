// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/execution"
	"github.com/stacklok/waggle/pkg/infra/store"
	"github.com/stacklok/waggle/pkg/infra/vm"
)

// fakeExecutor implements execution.Executor for testing.
type fakeExecutor struct {
	lastCodeReq *execution.CodeExecution
	lastPkgReq  *execution.PackageInstallation
	result      *execution.ExecResult
	err         error
}

func (f *fakeExecutor) ExecuteCode(
	_ context.Context, _ string, _ execution.ConnInfo, req *execution.CodeExecution,
) (*execution.ExecResult, error) {
	f.lastCodeReq = req
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &execution.ExecResult{Stdout: "ok", ExitCode: 0}, nil
}

func (f *fakeExecutor) InstallPackages(
	_ context.Context, _ string, _ execution.ConnInfo, req *execution.PackageInstallation,
) (*execution.ExecResult, error) {
	f.lastPkgReq = req
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
	execSvc := NewExecutionService(repo, executor, provider, envSvc, cfg)

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

	// Verify the request contains python3 exec command, .py extension, and user code.
	req := executor.lastCodeReq
	if req == nil {
		t.Fatal("ExecuteCode was not called")
	}
	if req.ExecCommand != "python3" {
		t.Errorf("ExecCommand = %q, want %q", req.ExecCommand, "python3")
	}
	if req.FileExtension != ".py" {
		t.Errorf("FileExtension = %q, want %q", req.FileExtension, ".py")
	}
	if req.Code != "print('hello')" {
		t.Errorf("Code = %q, want %q", req.Code, "print('hello')")
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

	req := executor.lastCodeReq
	if req == nil {
		t.Fatal("ExecuteCode was not called")
	}
	if req.ExecCommand != "node" {
		t.Errorf("ExecCommand = %q, want %q", req.ExecCommand, "node")
	}
	if req.FileExtension != ".js" {
		t.Errorf("FileExtension = %q, want %q", req.FileExtension, ".js")
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

	svc := NewExecutionService(repo, &fakeExecutor{}, newFakeProvider(), nil, testConfig())

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

	req := executor.lastPkgReq
	if req == nil {
		t.Fatal("InstallPackages was not called")
	}
	if req.InstallCommand != "pip install" {
		t.Errorf("InstallCommand = %q, want %q", req.InstallCommand, "pip install")
	}
	if len(req.Packages) != 2 || req.Packages[0] != "numpy" || req.Packages[1] != "pandas" {
		t.Errorf("Packages = %v, want [numpy pandas]", req.Packages)
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
