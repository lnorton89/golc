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
package command

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/show"
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
