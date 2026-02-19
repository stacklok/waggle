// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stacklok/waggle/pkg/config"
	"github.com/stacklok/waggle/pkg/domain/environment"
	"github.com/stacklok/waggle/pkg/domain/execution"
	"github.com/stacklok/waggle/pkg/infra/store"
	"github.com/stacklok/waggle/pkg/infra/vm"
)

// fakeExecutor implements execution.Executor for testing.
type fakeExecutor struct {
	lastCodeReq *execution.CodeExecution
	lastPkgReq  *execution.PackageInstallation
	codeReqs    []*execution.CodeExecution
	pkgReqs     []*execution.PackageInstallation
	codeResults []*execution.ExecResult
	pkgResults  []*execution.ExecResult
	result      *execution.ExecResult
	err         error
}

func (f *fakeExecutor) ExecuteCode(
	_ context.Context, _ string, _ execution.ConnInfo, req *execution.CodeExecution,
) (*execution.ExecResult, error) {
	f.lastCodeReq = req
	f.codeReqs = append(f.codeReqs, req)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.codeResults) > 0 {
		res := f.codeResults[0]
		f.codeResults = f.codeResults[1:]
		return res, nil
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
	f.pkgReqs = append(f.pkgReqs, req)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.pkgResults) > 0 {
		res := f.pkgResults[0]
		f.pkgResults = f.pkgResults[1:]
		return res, nil
	}
	if f.result != nil {
		return f.result, nil
	}
	return &execution.ExecResult{Stdout: "ok", ExitCode: 0}, nil
}

func setupExecTest(t *testing.T) (*ExecutionService, *fakeExecutor, string) {
	t.Helper()
	return setupExecTestWithConfig(t, testConfig())
}

func setupExecTestWithConfig(t *testing.T, cfg *config.Config) (*ExecutionService, *fakeExecutor, string) {
	t.Helper()
	ctx := context.Background()

	repo := store.NewMemoryStore()
	provider := newFakeProvider()
	portAlloc := vm.NewPortAllocator(20000, 20100)
	portAlloc.SetListenCheck(func(_ uint16) error { return nil })

	envSvc := NewEnvironmentService(repo, provider, portAlloc, nil, cfg)

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

	// Verify the request contains python exec command, .py extension, and user code.
	req := executor.lastCodeReq
	if req == nil {
		t.Fatal("ExecuteCode was not called")
	}
	if req.ExecCommand != defaultPythonCommand {
		t.Errorf("ExecCommand = %q, want %q", req.ExecCommand, defaultPythonCommand)
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
	if req.InstallCommand != defaultPipInstall {
		t.Errorf("InstallCommand = %q, want %q", req.InstallCommand, defaultPipInstall)
	}
	if len(req.Packages) != 2 || req.Packages[0] != "numpy" || req.Packages[1] != "pandas" {
		t.Errorf("Packages = %v, want [numpy pandas]", req.Packages)
	}
}

func TestExecutionServiceInstallPackagesPep668Fallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, executor, envID := setupExecTest(t)
	executor.pkgResults = []*execution.ExecResult{
		{ExitCode: 1, Stderr: "error: externally-managed-environment"},
		{ExitCode: 0},
	}
	executor.codeResults = []*execution.ExecResult{{ExitCode: 0}}

	result, err := svc.InstallPackages(ctx, envID, []string{"numpy"})
	if err != nil {
		t.Fatalf("InstallPackages: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if len(executor.pkgReqs) != 2 {
		t.Fatalf("InstallPackages calls = %d, want 2", len(executor.pkgReqs))
	}
	if executor.pkgReqs[1].InstallCommand != defaultPythonVenv+"/bin/pip install" {
		t.Errorf("InstallCommand = %q, want %q", executor.pkgReqs[1].InstallCommand, defaultPythonVenv+"/bin/pip install")
	}
	if len(executor.codeReqs) != 1 {
		t.Fatalf("ExecuteCode calls = %d, want 1", len(executor.codeReqs))
	}
	if executor.codeReqs[0].ExecCommand != defaultShellCommand {
		t.Errorf("ExecCommand = %q, want %q", executor.codeReqs[0].ExecCommand, defaultShellCommand)
	}
	if !strings.Contains(executor.codeReqs[0].Code, "-m venv "+defaultPythonVenv) {
		t.Errorf("Code = %q, want venv creation", executor.codeReqs[0].Code)
	}
}

func TestExecutionServiceInstallPackagesPep668FallbackUsesExplicitPython(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, executor, envID := setupExecTest(t)
	executor.pkgResults = []*execution.ExecResult{
		{ExitCode: 1, Stderr: "error: externally-managed-environment"},
		{ExitCode: 0},
	}
	executor.codeResults = []*execution.ExecResult{{ExitCode: 0}}

	_, err := svc.InstallPackages(ctx, envID, []string{"numpy"})
	if err != nil {
		t.Fatalf("InstallPackages: %v", err)
	}
	if len(executor.codeReqs) != 1 {
		t.Fatalf("ExecuteCode calls = %d, want 1", len(executor.codeReqs))
	}
	if !strings.HasPrefix(executor.codeReqs[0].Code, defaultPythonFallback+" -m venv ") {
		t.Errorf("Code = %q, want python fallback", executor.codeReqs[0].Code)
	}
}

func TestExecutionServiceInstallPackagesPersistsVenvCommands(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, executor, envID := setupExecTest(t)
	executor.pkgResults = []*execution.ExecResult{
		{ExitCode: 1, Stderr: "error: externally-managed-environment"},
		{ExitCode: 0},
	}
	executor.codeResults = []*execution.ExecResult{{ExitCode: 0}}

	_, err := svc.InstallPackages(ctx, envID, []string{"numpy"})
	if err != nil {
		t.Fatalf("InstallPackages: %v", err)
	}

	_, err = svc.Execute(ctx, envID, "print('ok')", "", 0)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if executor.lastCodeReq == nil {
		t.Fatal("ExecuteCode was not called")
	}
	if executor.lastCodeReq.ExecCommand != defaultPythonVenvExec {
		t.Errorf("ExecCommand = %q, want %q", executor.lastCodeReq.ExecCommand, defaultPythonVenvExec)
	}
}

func TestExecutionServiceInstallPackagesNoPepFallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, executor, envID := setupExecTest(t)
	executor.pkgResults = []*execution.ExecResult{{ExitCode: 1, Stderr: "no such package"}}

	result, err := svc.InstallPackages(ctx, envID, []string{"numpy"})
	if err != nil {
		t.Fatalf("InstallPackages: %v", err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", result.ExitCode)
	}
	if len(executor.pkgReqs) != 1 {
		t.Fatalf("InstallPackages calls = %d, want 1", len(executor.pkgReqs))
	}
	if len(executor.codeReqs) != 0 {
		t.Fatalf("ExecuteCode calls = %d, want 0", len(executor.codeReqs))
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

func TestExecutionServiceExecCommandOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := testConfig()
	cfg.RuntimeCommands = map[string]config.RuntimeCommandConfig{
		"python": {ExecCommand: "/usr/bin/python"},
	}

	svc, executor, envID := setupExecTestWithConfig(t, cfg)

	_, err := svc.Execute(ctx, envID, "print('hello')", "", 0)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	req := executor.lastCodeReq
	if req == nil {
		t.Fatal("ExecuteCode was not called")
	}
	if req.ExecCommand != "/usr/bin/python" {
		t.Errorf("ExecCommand = %q, want %q", req.ExecCommand, "/usr/bin/python")
	}
}

func TestExecutionServiceExecCommandOverrideUnsafe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := testConfig()
	cfg.RuntimeCommands = map[string]config.RuntimeCommandConfig{
		"python": {ExecCommand: "python3; rm -rf /"},
	}

	svc, executor, envID := setupExecTestWithConfig(t, cfg)

	_, err := svc.Execute(ctx, envID, "print('hello')", "", 0)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	req := executor.lastCodeReq
	if req == nil {
		t.Fatal("ExecuteCode was not called")
	}
	if req.ExecCommand != defaultPythonCommand {
		t.Errorf("ExecCommand = %q, want %q", req.ExecCommand, defaultPythonCommand)
	}
}

func TestExecutionServiceInstallCommandOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := testConfig()
	cfg.RuntimeCommands = map[string]config.RuntimeCommandConfig{
		"python": {InstallCommand: "/usr/bin/pip3 install --no-cache-dir"},
	}

	svc, executor, envID := setupExecTestWithConfig(t, cfg)

	_, err := svc.InstallPackages(ctx, envID, []string{"numpy"})
	if err != nil {
		t.Fatalf("InstallPackages: %v", err)
	}

	req := executor.lastPkgReq
	if req == nil {
		t.Fatal("InstallPackages was not called")
	}
	if req.InstallCommand != "/usr/bin/pip3 install --no-cache-dir" {
		t.Errorf("InstallCommand = %q, want %q", req.InstallCommand, "/usr/bin/pip3 install --no-cache-dir")
	}
}

func TestExecutionServiceInstallCommandOverrideUnsafe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := testConfig()
	cfg.RuntimeCommands = map[string]config.RuntimeCommandConfig{
		"python": {InstallCommand: "pip install; rm -rf /"},
	}

	svc, executor, envID := setupExecTestWithConfig(t, cfg)

	_, err := svc.InstallPackages(ctx, envID, []string{"numpy"})
	if err != nil {
		t.Fatalf("InstallPackages: %v", err)
	}

	req := executor.lastPkgReq
	if req == nil {
		t.Fatal("InstallPackages was not called")
	}
	if req.InstallCommand != defaultPipInstall {
		t.Errorf("InstallCommand = %q, want %q", req.InstallCommand, defaultPipInstall)
	}
}
