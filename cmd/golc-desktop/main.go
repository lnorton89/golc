// Command golc-desktop is the Wails v2 desktop entrypoint (06-04-PLAN.md
// Task 2): it constructs the Go host (internal/wails.App) and the four
// per-feature binding service stubs, embeds the compiled frontend/dist
// static bundle, and calls wails.Run with app as the lifecycle owner and
// all five structs bound to the webview. This is a sibling cmd/ target,
// never a modification of cmd/golc-project's existing CLI entrypoint
// (CONTEXT: "Phase 6's Wails app is just one more client").
package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/midi"
	golcwails "github.com/lnorton89/golc/internal/wails"
)

//go:embed all:frontend/dist
var assets embed.FS

// repoRootEnvName mirrors cmd/golc-project/main.go's own GOLC_PROJECT_ROOT
// convention -- the desktop entrypoint resolves the same project root a
// CLI invocation would, so a supervised "golc-project.exe artnet serve"
// spawn (internal/wails/app.go's ensureDaemon) resolves the identical
// show/config paths a matching CLI invocation would.
const repoRootEnvName = "GOLC_PROJECT_ROOT"

// showPathEnvName/interfaceIndexEnvName/interfaceNameEnvName/
// fixturesDirEnvName configure the supervised daemon spawn's "artnet
// serve" arguments (internal/command/artnet.go's runArtnetServe shape).
// This scaffold reads them from the environment rather than a first-launch
// settings UI -- a later plan may add a real configuration flow; for now
// an operator (or a launcher script) sets these before starting
// golc-desktop.exe. An empty ShowPath/zero InterfaceIndex still lets the
// app launch and register the safety-cluster hotkeys (PLAY-09 requires
// them independent of daemon-spawn success); only the daemon-spawn
// attempt itself would then fail with a clear diagnostic.
const (
	showPathEnvName       = "GOLC_DESKTOP_SHOW"
	interfaceIndexEnvName = "GOLC_DESKTOP_INTERFACE"
	interfaceNameEnvName  = "GOLC_DESKTOP_INTERFACE_NAME"
	fixturesDirEnvName    = "GOLC_DESKTOP_FIXTURES"
)

func main() {
	cfg := golcwails.Config{
		PipeName:       ipc.PipeName,
		ShowPath:       os.Getenv(showPathEnvName),
		InterfaceIndex: envInt(interfaceIndexEnvName, 0),
		InterfaceName:  os.Getenv(interfaceNameEnvName),
		FixturesDir:    os.Getenv(fixturesDirEnvName),
		ProjectRoot:    resolveProjectRoot(),
	}

	app := golcwails.NewApp(cfg)
	safetyService := golcwails.NewSafetyService(cfg.PipeName, cfg.ProjectRoot, cfg.ShowPath)
	playbackService := golcwails.NewPlaybackService(cfg.PipeName, cfg.ShowPath, cfg.ProjectRoot)
	surfaceService := golcwails.NewSurfaceService(cfg.PipeName, cfg.ProjectRoot, cfg.ShowPath)
	midiService := golcwails.NewMidiService(cfg.PipeName, cfg.ProjectRoot, cfg.ShowPath)
	fixturePatchService := golcwails.NewFixturePatchService(cfg.PipeName, cfg.ProjectRoot, cfg.ShowPath)
	artnetConfigService := golcwails.NewArtnetConfigService(cfg.PipeName, cfg.ProjectRoot)
	programmingService := golcwails.NewProgrammingService(cfg.PipeName, cfg.ProjectRoot, cfg.ShowPath)

	err := wails.Run(&options.App{
		Title:  "GOLC",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 19, G: 20, B: 25, A: 1},
		OnStartup: func(ctx context.Context) {
			// App's own daemon-supervision/hotkey lifecycle (app.go,
			// unmodified by this plan) starts first, then this
			// service's own throttled PLAY-07 "status:update" push
			// (svc_safety.go) -- mirroring app.go's own documented
			// ordered-start discipline without this plan touching
			// app.go itself.
			app.OnStartup(ctx)
			safetyService.StartStatusPush(ctx)

			// MIDI hardware remains optional (PROJECT.md): attaching a
			// live driver never blocks or fails startup. midi_driver.go's
			// blank import registers midicatdrv (this binary's only
			// registered driver) before this line runs; a missing/absent
			// device (GOLC_MIDI_NO_PORTS_AVAILABLE) is logged, not fatal.
			midiService.StartFeedback(ctx)
			if driver, driverErr := midi.OpenFirstAvailable(); driverErr != nil {
				log.Printf("GOLC_WAILS_MIDI_DRIVER_UNAVAILABLE: %v", driverErr)
			} else if attachErr := midiService.AttachDriver(driver); attachErr != nil {
				log.Printf("GOLC_WAILS_MIDI_DRIVER_UNAVAILABLE: %v", attachErr)
			}
		},
		OnShutdown: func(ctx context.Context) {
			// Reverse order: stop the status push before App's own
			// subsystems (hotkeys, daemon child process) shut down.
			midiService.DetachDriver()
			midiService.StopFeedback()
			safetyService.StopStatusPush()
			app.OnShutdown(ctx)
		},
		Bind: []interface{}{
			app,
			safetyService,
			playbackService,
			surfaceService,
			midiService,
			fixturePatchService,
			artnetConfigService,
			programmingService,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "GOLC_WAILS_RUN_FAILED:", err)
		os.Exit(1)
	}
}

// resolveProjectRoot prefers GOLC_PROJECT_ROOT (mirroring
// cmd/golc-project/main.go's resolveProjectRoot) and falls back to the
// current working directory.
func resolveProjectRoot() string {
	root := os.Getenv(repoRootEnvName)
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return ""
		}
		root = workingDirectory
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return absolute
}

// envInt parses name as an int, returning fallback when unset or invalid.
func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
