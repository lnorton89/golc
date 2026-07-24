package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lnorton89/golc/internal/bootstrap"
	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/delivery"
)

type commandExecutor interface {
	Execute(command.Request) command.Result
}

type targetRuntime struct {
	getenv      func(string) string
	getwd       func() (string, error)
	setenv      func(string, string) error
	bootstrap   func(context.Context, string, bootstrap.Options) error
	newRegistry func() (commandExecutor, error)
	loadPRGraph func(string) (delivery.Graph, error)
	runGraph    func(delivery.Graph, delivery.StepExecutor) ([]delivery.StepResult, error)
	stdout      io.Writer
	stderr      io.Writer
}

var activeTargetRuntime = targetRuntime{
	getenv:    os.Getenv,
	getwd:     os.Getwd,
	setenv:    os.Setenv,
	bootstrap: bootstrap.Bootstrap,
	newRegistry: func() (commandExecutor, error) {
		return command.NewDefaultCommandRegistry()
	},
	loadPRGraph: delivery.LoadPRGraph,
	runGraph:    delivery.Run,
	stdout:      os.Stdout,
	stderr:      os.Stderr,
}

func resolveProjectRoot(runtime targetRuntime) (string, error) {
	if candidate := runtime.getenv("GOLC_PROJECT_ROOT"); candidate != "" {
		if root, ok := normalizeRepositoryRoot(candidate); ok {
			return root, nil
		}
	}
	current, err := runtime.getwd()
	if err != nil {
		return "", fmt.Errorf("GOLC_MAGE_ROOT: %w", err)
	}
	absolute, err := filepath.Abs(current)
	if err != nil {
		return "", fmt.Errorf("GOLC_MAGE_ROOT: %w", err)
	}
	for {
		if root, ok := normalizeRepositoryRoot(absolute); ok {
			return root, nil
		}
		parent := filepath.Dir(absolute)
		if parent == absolute {
			return "", fmt.Errorf("GOLC_MAGE_ROOT: no golc.project.toml found from %s", current)
		}
		absolute = parent
	}
}

func normalizeRepositoryRoot(candidate string) (string, bool) {
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(filepath.Join(resolved, "golc.project.toml"))
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return filepath.Clean(resolved), true
}

func establishProjectRoot(runtime targetRuntime) (string, error) {
	root, err := resolveProjectRoot(runtime)
	if err != nil {
		return "", err
	}
	if err := runtime.setenv("GOLC_PROJECT_ROOT", root); err != nil {
		return "", fmt.Errorf("GOLC_MAGE_ROOT: set GOLC_PROJECT_ROOT: %w", err)
	}
	return root, nil
}

func runRoute(arguments ...string) error {
	runtime := activeTargetRuntime
	root, err := establishProjectRoot(runtime)
	if err != nil {
		return err
	}
	registry, err := runtime.newRegistry()
	if err != nil {
		return err
	}
	result := registry.Execute(command.Request{Root: root, Args: append([]string(nil), arguments...)})
	if exitCode := command.WriteResult(runtime.stdout, runtime.stderr, result); exitCode != 0 {
		return fmt.Errorf("GOLC_MAGE_TARGET_FAILED: %s exited %d", arguments[0], exitCode)
	}
	return nil
}

// Bootstrap provisions every pinned project-local tool through the Go API.
func Bootstrap(ctx context.Context) error {
	runtime := activeTargetRuntime
	root, err := establishProjectRoot(runtime)
	if err != nil {
		return err
	}
	return runtime.bootstrap(ctx, root, bootstrap.Options{})
}

// Generate writes every registered schema.
func Generate() error { return runRoute("generate") }

// GenerateCheck checks generated-schema drift without writing.
func GenerateCheck() error { return runRoute("generate", "--check") }

// Check runs the strict project concern.
func Check() error { return runRoute("check", "--concern", "project") }

// CheckOffline runs the network-denied core graph.
func CheckOffline() error { return runRoute("check", "--offline") }

// Build compiles every project package.
func Build() error { return runRoute("build") }

// Test runs the complete project test route.
func Test() error { return runRoute("test") }

// Package builds the foundation developer-tool bundle.
func Package() error { return runRoute("package", "--foundation") }

// PackageFoundation is the explicit foundation-package alias.
func PackageFoundation() error { return runRoute("package", "--foundation") }

// Pr executes the strict configured PR graph serially.
func Pr(ctx context.Context) error {
	runtime := activeTargetRuntime
	root, err := establishProjectRoot(runtime)
	if err != nil {
		return err
	}
	graph, err := runtime.loadPRGraph(root)
	if err != nil {
		return err
	}
	registry, err := runtime.newRegistry()
	if err != nil {
		return err
	}
	executor := func(route string, args []string) (int, []byte, []byte) {
		if route == "bootstrap" {
			if err := runtime.bootstrap(ctx, root, bootstrap.Options{}); err != nil {
				return 1, nil, []byte(err.Error() + "\n")
			}
			return 0, nil, nil
		}
		result := registry.Execute(command.Request{
			Root: root,
			Args: append([]string{route}, args...),
		})
		return result.ExitCode, result.Stdout, result.Stderr
	}
	results, runErr := runtime.runGraph(graph, executor)
	for _, result := range results {
		exitCode := command.WriteResult(runtime.stdout, runtime.stderr, command.Result{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		})
		if exitCode != result.ExitCode && runErr == nil {
			runErr = fmt.Errorf("GOLC_MAGE_TARGET_FAILED: write output for step %q", result.Step.Name)
		}
	}
	return runErr
}
