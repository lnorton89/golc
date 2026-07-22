// playback.go is the playback command file: it owns the "playback" routing
// scope and self-registers "playback bpm set" / "playback bpm tap" (CONTEXT
// SCEN-02/SCEN-03): an operator sets the show-wide global BPM either by
// entering a numeric value or through tap tempo, both persisted on
// show.State.Tempo.BPM through the existing atomic Load/Save round trip.
// This plan does not add the Tempo field -- it was added by 03-04's
// internal/show/state.go -- it only reads/writes it. Handlers follow
// internal/command/pool.go's parse-args-then-Load-mutate-Save-Stdout shape;
// BPM/tap validation itself lives in internal/playback (clock.go's
// ValidateBPM/TapTempo), never re-derived here.
//
// This file also self-registers "playback evaluate" / "playback switch"
// (03-07-PLAN.md Task 3, CONTEXT SCEN-06/SCEN-09): the headless,
// deterministic demonstration surface for the compiler/evaluator/engine
// 03-07 Tasks 1/2 built. "playback evaluate" compiles the active scene and
// prints the deterministic Frame at a given musical position -- two runs
// against the same show always produce byte-identical output (SCEN-09 at
// the CLI surface). "playback switch" marks a scene active and saves,
// reusing scene.ActivateScene directly -- it never reimplements layer
// resolution.
package command

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "playback",
	Summary: "Global musical clock control: numeric and tap-tempo BPM entry persisted on show.State.Tempo (SCEN-02/SCEN-03).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "playback bpm set",
	Summary: "Set the show-wide global BPM to an explicit numeric value: playback bpm set <bpm> --show <path>.",
	Handler: runPlaybackBPMSet,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "playback bpm tap",
	Summary: "Set the show-wide global BPM from ordered tap timestamps: " +
		"playback bpm tap --at <RFC3339-timestamp> --at <RFC3339-timestamp> ... --show <path>.",
	Handler: runPlaybackBPMTap,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "playback evaluate",
	Summary: "Compile the active scene and print the deterministic Frame at a musical position (SCEN-06/SCEN-09): " +
		"playback evaluate --at <bar>.<beatfraction> [--json] --show <path>.",
	Handler: runPlaybackEvaluate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "playback switch",
	Summary: "Stage an active-scene switch: playback switch <scene> --show <path>.",
	Handler: runPlaybackSwitch,
})

// parsePlaybackBPMSetArgs accepts a positional BPM value followed by a
// required "--show <path>" (both --flag value and --flag=value forms),
// rejecting anything else (GOLC_PLAYBACK_USAGE). The BPM value's own
// numeric shape and range are not checked here: a missing positional
// argument is a usage error, but a present, non-numeric, or out-of-range
// value is a GOLC_PLAYBACK_BPM_INVALID validation failure the caller
// surfaces separately, not a usage error.
func parsePlaybackBPMSetArgs(usage string, args []string) (rawBPM, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", fmt.Errorf("GOLC_PLAYBACK_USAGE: usage: %s", usage)
	}
	rawBPM = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("GOLC_PLAYBACK_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_PLAYBACK_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", fmt.Errorf("GOLC_PLAYBACK_USAGE: --show is required; usage: %s", usage)
	}
	return rawBPM, showPath, nil
}

// runPlaybackBPMSet serves the self-registered "playback bpm set" route
// (SCEN-02): parse the positional BPM value, validate it
// (playback.ValidateBPM -- positive, finite, within the declared sane
// ceiling), load the ShowState at --show, write state.Tempo.BPM, and save
// atomically. Setting BPM to its current value is accepted as an
// idempotent no-op: no special-case comparison against the prior value is
// performed here, the same validated value is simply written and saved
// again.
func runPlaybackBPMSet(request Request) Result {
	usage := "playback bpm set <bpm> --show <path>"
	rawBPM, showPath, err := parsePlaybackBPMSetArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	bpm, parseErr := strconv.ParseFloat(rawBPM, 64)
	if parseErr != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
			"GOLC_PLAYBACK_BPM_INVALID: %q is not a valid number: %v\n", rawBPM, parseErr))}
	}
	if err := playback.ValidateBPM(bpm); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Tempo.BPM = bpm

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_PLAYBACK_BPM_SET: %v\n", bpm))}
}

// parseTapTimestamp parses one --at value as an RFC3339 timestamp
// (fractional seconds optional), rejecting anything else with
// GOLC_PLAYBACK_USAGE.
func parseTapTimestamp(usage, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("GOLC_PLAYBACK_USAGE: --at value %q is not a valid RFC3339 timestamp; usage: %s", value, usage)
	}
	return parsed, nil
}

// parsePlaybackBPMTapArgs accepts any number of "--at <RFC3339 timestamp>"
// flags (both --flag value and --flag=value forms), consumed in the exact
// order given on the command line -- arrival order, per
// playback.TapTempo's own ordering contract -- plus a required
// "--show <path>", rejecting anything else (GOLC_PLAYBACK_USAGE). Fewer
// than two --at flags is not a usage error here: it is surfaced as
// GOLC_PLAYBACK_TAP_INVALID once the route calls playback.TapTempo.
func parsePlaybackBPMTapArgs(usage string, args []string) (taps []time.Time, showPath string, err error) {
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--at":
			if i+1 >= len(args) {
				return nil, "", fmt.Errorf("GOLC_PLAYBACK_USAGE: --at requires a timestamp; usage: %s", usage)
			}
			tap, parseErr := parseTapTimestamp(usage, args[i+1])
			if parseErr != nil {
				return nil, "", parseErr
			}
			taps = append(taps, tap)
			i += 2
		case strings.HasPrefix(argument, "--at="):
			tap, parseErr := parseTapTimestamp(usage, strings.TrimPrefix(argument, "--at="))
			if parseErr != nil {
				return nil, "", parseErr
			}
			taps = append(taps, tap)
			i++
		case argument == "--show":
			if i+1 >= len(args) {
				return nil, "", fmt.Errorf("GOLC_PLAYBACK_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return nil, "", fmt.Errorf("GOLC_PLAYBACK_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return nil, "", fmt.Errorf("GOLC_PLAYBACK_USAGE: --show is required; usage: %s", usage)
	}
	return taps, showPath, nil
}

// runPlaybackBPMTap serves the self-registered "playback bpm tap" route
// (SCEN-03): parse the ordered --at tap timestamps, convert them to a BPM
// via playback.TapTempo (rejecting fewer than two taps or a zero/negative
// interval with GOLC_PLAYBACK_TAP_INVALID -- never persisting a change),
// load the ShowState at --show, write the resulting state.Tempo.BPM, and
// save atomically.
func runPlaybackBPMTap(request Request) Result {
	usage := "playback bpm tap --at <RFC3339-timestamp> --at <RFC3339-timestamp> ... --show <path>"
	taps, showPath, err := parsePlaybackBPMTapArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	bpm, err := playback.TapTempo(taps)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Tempo.BPM = bpm

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_PLAYBACK_BPM_TAP: %v\n", bpm))}
}

// playbackEvaluateArgs is the parsed shape of one "playback evaluate"
// invocation.
type playbackEvaluateArgs struct {
	at       float64
	json     bool
	showPath string
}

// parsePlaybackEvaluateArgs accepts a required "--at <bar>.<beatfraction>"
// numeric position, an optional "--json" flag, and a required
// "--show <path>" (both --flag value and --flag=value forms), rejecting
// anything else (GOLC_PLAYBACK_USAGE). --at's own numeric shape is checked
// here (a non-numeric value is a usage error); its musical-position
// decomposition (bar/beat-fraction split) happens in runPlaybackEvaluate.
func parsePlaybackEvaluateArgs(usage string, args []string) (playbackEvaluateArgs, error) {
	var parsed playbackEvaluateArgs
	var atSeen bool

	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--at":
			if i+1 >= len(args) {
				return playbackEvaluateArgs{}, fmt.Errorf("GOLC_PLAYBACK_USAGE: --at requires a value; usage: %s", usage)
			}
			value, err := strconv.ParseFloat(args[i+1], 64)
			if err != nil {
				return playbackEvaluateArgs{}, fmt.Errorf("GOLC_PLAYBACK_USAGE: --at value %q is not a valid number; usage: %s", args[i+1], usage)
			}
			parsed.at = value
			atSeen = true
			i += 2
		case strings.HasPrefix(argument, "--at="):
			raw := strings.TrimPrefix(argument, "--at=")
			value, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return playbackEvaluateArgs{}, fmt.Errorf("GOLC_PLAYBACK_USAGE: --at value %q is not a valid number; usage: %s", raw, usage)
			}
			parsed.at = value
			atSeen = true
			i++
		case argument == "--json":
			parsed.json = true
			i++
		case argument == "--show":
			if i+1 >= len(args) {
				return playbackEvaluateArgs{}, fmt.Errorf("GOLC_PLAYBACK_USAGE: --show requires a path; usage: %s", usage)
			}
			parsed.showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			parsed.showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return playbackEvaluateArgs{}, fmt.Errorf("GOLC_PLAYBACK_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if !atSeen {
		return playbackEvaluateArgs{}, fmt.Errorf("GOLC_PLAYBACK_USAGE: --at is required; usage: %s", usage)
	}
	if parsed.showPath == "" {
		return playbackEvaluateArgs{}, fmt.Errorf("GOLC_PLAYBACK_USAGE: --show is required; usage: %s", usage)
	}
	return parsed, nil
}

// positionFromAt decomposes a raw "--at" numeric value into a
// playback.MusicalPosition: the integer floor is the raw bar count,
// BeatFraction is the remaining fractional part -- so "--at 2.0" is bar=2
// beat_fraction=0, and "--at 2.75" is bar=2 beat_fraction=0.75. BarIndex is
// then wrapped into [0, barsPerLoop), matching playback.Position's own
// documented invariant ("already wrapped modulo barsPerLoop") that
// motionPhase and every other MusicalPosition consumer relies on -- a
// negative wrapped remainder (from a negative --at) is normalized back
// into [0, barsPerLoop) rather than left negative, mirroring Go's
// truncating (not flooring) % operator behavior for negative operands.
func positionFromAt(at float64, barsPerLoop int) playback.MusicalPosition {
	bar := math.Floor(at)
	beatFraction := at - bar
	wrapped := int(bar) % barsPerLoop
	if wrapped < 0 {
		wrapped += barsPerLoop
	}
	return playback.MusicalPosition{BarIndex: wrapped, BeatFraction: beatFraction}
}

// runPlaybackEvaluate serves the self-registered "playback evaluate" route
// (SCEN-06/SCEN-09): load the ShowState at --show, compile its active
// scene (Compile -- all-or-nothing, CONTEXT D-06), decompose --at into a
// MusicalPosition, and print the deterministic Evaluate(plan, pos) Frame.
// A State with no active scene or an otherwise-invalid compiled plan exits
// non-zero with Compile's own GOLC_PLAYBACK_NO_ACTIVE_SCENE/
// GOLC_PLAYBACK_PLAN_INVALID diagnostic. Two invocations against the same
// show and the same --at always print byte-identical output (SCEN-09 at
// the CLI surface): Evaluate is pure, and strictjson.CanonicalEncode's map
// -key sorting makes the --json encoding deterministic too.
func runPlaybackEvaluate(request Request) Result {
	usage := "playback evaluate --at <bar>.<beatfraction> [--json] --show <path>"
	parsed, err := parsePlaybackEvaluateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	plan, err := playback.Compile(state)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	pos := positionFromAt(parsed.at, plan.BarsPerLoop)
	frame := playback.Evaluate(plan, pos)

	if parsed.json {
		payload, err := strictjson.CanonicalEncode(frame)
		if err != nil {
			return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_PLAYBACK_FRAME_ENCODE_FAILED: %v\n", err))}
		}
		return Result{Stdout: payload}
	}
	return Result{Stdout: []byte(fmt.Sprintf(
		"GOLC_PLAYBACK_EVALUATE: bar=%d beat_fraction=%v instances=%d\n",
		pos.BarIndex, pos.BeatFraction, len(frame.Values)))}
}

// parsePlaybackSwitchArgs accepts a positional scene name followed by a
// required "--show <path>" (both --flag value and --flag=value forms),
// rejecting anything else (GOLC_PLAYBACK_USAGE).
func parsePlaybackSwitchArgs(usage string, args []string) (sceneName, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", fmt.Errorf("GOLC_PLAYBACK_USAGE: usage: %s", usage)
	}
	sceneName = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("GOLC_PLAYBACK_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_PLAYBACK_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", fmt.Errorf("GOLC_PLAYBACK_USAGE: --show is required; usage: %s", usage)
	}
	return sceneName, showPath, nil
}

// runPlaybackSwitch serves the self-registered "playback switch" route
// (SCEN-06): load the ShowState at --show, mark the named scene active via
// scene.ActivateScene (never reimplementing layer resolution or single-
// active-scene enforcement itself), and save atomically. An unknown scene
// name exits non-zero with GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE.
func runPlaybackSwitch(request Request) Result {
	usage := "playback switch <scene> --show <path>"
	sceneName, showPath, err := parsePlaybackSwitchArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	activated, err := scene.ActivateScene(state.Scenes, sceneName)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE: %v\n", err))}
	}
	state.Scenes = activated

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_PLAYBACK_SWITCH: %s\n", sceneName))}
}
