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

func runTarget(name string, ctx context.Context) error {
	target, ok := delivery.LookupMageTarget(name)
	if !ok {
		return fmt.Errorf("GOLC_MAGE_TARGET_UNKNOWN: %s", name)
	}
	runtime := activeTargetRuntime
	var bootstrapOptions bootstrap.Options
	if target.Kind == delivery.MageTargetKindBootstrap || target.Kind == delivery.MageTargetKindPR {
		var err error
		bootstrapOptions, err = parseBootstrapOptions(runtime)
		if err != nil {
			return err
		}
	}
	root, err := establishProjectRoot(runtime)
	if err != nil {
		return err
	}

	switch target.Kind {
	case delivery.MageTargetKindBootstrap:
		return runtime.bootstrap(ctx, root, bootstrapOptions)
	case delivery.MageTargetKindPR:
		return runPRTarget(ctx, runtime, root, bootstrapOptions)
	case delivery.MageTargetKindRoute:
		return runRouteTarget(runtime, root, target)
	default:
		return fmt.Errorf("GOLC_MAGE_TARGET_KIND: %s has unsupported kind %q", target.Name, target.Kind)
	}
}

func parseBootstrapOptions(runtime targetRuntime) (bootstrap.Options, error) {
	target, ok := delivery.LookupMageTarget("bootstrap")
	if !ok || len(target.EnvironmentOptions) != 1 {
		return bootstrap.Options{}, fmt.Errorf("GOLC_MAGE_BOOTSTRAP_OPTION: Bootstrap descriptor must declare exactly one environment option")
	}
	option := target.EnvironmentOptions[0]
	switch value := runtime.getenv(option.Name); value {
	case "":
		return bootstrap.Options{}, nil
	case option.EnablingValue:
		return bootstrap.Options{IncludeLinearSync: true}, nil
	default:
		return bootstrap.Options{}, fmt.Errorf(
			"GOLC_MAGE_BOOTSTRAP_OPTION: %s must be unset or %q",
			option.Name, option.EnablingValue,
		)
	}
}

func runRouteTarget(runtime targetRuntime, root string, target delivery.MageTarget) error {
	registry, err := runtime.newRegistry()
	if err != nil {
		return err
	}
	arguments := append([]string{target.Route}, target.Args...)
	result := registry.Execute(command.Request{Root: root, Args: append([]string(nil), arguments...)})
	if exitCode := command.WriteResult(runtime.stdout, runtime.stderr, result); exitCode != 0 {
		return fmt.Errorf("GOLC_MAGE_TARGET_FAILED: %s exited %d", target.Name, exitCode)
	}
	return nil
}

// Bootstrap provisions every pinned project-local tool through the Go API.
func Bootstrap(ctx context.Context) error { return runTarget("bootstrap", ctx) }

// Generate writes every registered schema.
func Generate() error { return runTarget("generate", context.Background()) }

// GenerateCheck checks generated-schema drift without writing.
func GenerateCheck() error { return runTarget("generatecheck", context.Background()) }

// Check runs the strict project concern.
func Check() error { return runTarget("check", context.Background()) }

// CheckOffline runs the network-denied core graph.
func CheckOffline() error { return runTarget("checkoffline", context.Background()) }

// Build compiles every project package.
func Build() error { return runTarget("build", context.Background()) }

// Test runs the complete project test route.
func Test() error { return runTarget("test", context.Background()) }

// Package builds the foundation developer-tool bundle.
func Package() error { return runTarget("package", context.Background()) }

// PackageFoundation is the explicit foundation-package alias.
func PackageFoundation() error { return runTarget("packagefoundation", context.Background()) }

// Pr executes the strict configured PR graph serially.
func Pr(ctx context.Context) error { return runTarget("pr", ctx) }

func runPRTarget(ctx context.Context, runtime targetRuntime, root string, bootstrapOptions bootstrap.Options) error {
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
			if err := runtime.bootstrap(ctx, root, bootstrapOptions); err != nil {
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
