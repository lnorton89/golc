// script.go is the script command file (08-01-PLAN.md Task 2, CONTEXT
// SCRP-01/SCRP-04, D-06/D-07/D-09/D-14/D-16/D-17): it owns the "script"
// routing scope and self-registers "script create"/"script list"/
// "script show"/"script edit"/"script delete"/"script profile set" -- a
// show author creates, inspects, edits, deletes, and assigns capability
// profiles to single-file TypeScript automation scripts that live inside
// show.State (internal/show/scripts.go). Handlers follow scene.go's
// parse-args-then-Load-mutate-Save-Stdout shape; every route writes
// deterministic JSON to Stdout (never plain text) so the D-16 library view
// and D-07 run dialog can consume it directly. internal/script (this
// phase's later plans, 08-05+) stays a pure library the same way
// internal/projectconfig does (STATE.md's recorded decision): every
// "script *" route lives here in package command, resolving any future
// command<->script import cycle the same mechanical way config.go already
// does for internal/projectconfig.
package command

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

// maxScriptSourceBytes bounds "script edit --source-file"'s input (T-08-03
// DoS mitigation): a source file larger than this is rejected with
// GOLC_SCRIPT_SOURCE_TOO_LARGE before it can ever enter the show blob, so
// a single pathological script can never bloat every autosave/recovery
// point.
const maxScriptSourceBytes = 1 << 20 // 1 MiB

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "script",
	Summary: "Single-file TypeScript automation scripts saved inside a show, with per-script capability/resource-limit profiles (SCRP-01/SCRP-04, D-06/D-07/D-09/D-14/D-17).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "script create",
	Summary: "Create a named, empty script against a ShowState document: script create <name> --show <path>.",
	Handler: runScriptCreate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "script list",
	Summary: "List every script's name, last-run status, and capability profile (D-16's library projection -- source is omitted): script list --show <path>.",
	Handler: runScriptList,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "script show",
	Summary: "Show one script in full, including its source: script show <name> --show <path>.",
	Handler: runScriptShow,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "script edit",
	Summary: "Replace a script's source with a file's bytes verbatim (max 1MiB): script edit <name> --source-file <path> --show <path>.",
	Handler: runScriptEdit,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "script delete",
	Summary: "Delete a named script: script delete <name> --show <path>.",
	Handler: runScriptDelete,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "script profile set",
	Summary: "Set a script's saved capability/resource-limit profile (D-07: a per-script default, not re-entered every run): script profile set <name> " +
		"[--scope <playback|authoring|admin>] [--preset <quick-action|long-running-automation|advanced>] " +
		"[--deadline-seconds <n>] [--rate-per-second <n>] [--memory-limit-mb <n>] [--cpu-cap-percent <n>] --show <path>.",
	Handler: runScriptProfileSet,
})

// scriptCapabilityProfileView is CapabilityProfile's rendered JSON shape.
type scriptCapabilityProfileView struct {
	Scope           string `json:"scope"`
	Preset          string `json:"preset"`
	DeadlineSeconds int    `json:"deadline_seconds"`
	RatePerSecond   int    `json:"rate_per_second"`
	MemoryLimitMB   int    `json:"memory_limit_mb"`
	CPUCapPercent   int    `json:"cpu_cap_percent"`
}

// scriptView is "script create"/"script show"/"script edit"/"script
// profile set"'s full per-script JSON shape, including Source.
type scriptView struct {
	ID                string                      `json:"id"`
	Name              string                      `json:"name"`
	Source            string                      `json:"source"`
	CapabilityProfile scriptCapabilityProfileView `json:"capability_profile"`
	LastRunStatus     string                      `json:"last_run_status"`
	LastRunReason     string                      `json:"last_run_reason,omitempty"`
	LastRunAt         string                      `json:"last_run_at,omitempty"`
}

// scriptListEntryView is "script list"'s per-script JSON shape -- the
// exact D-16 library-view projection: name, last-run status, and
// capability profile. It deliberately omits Source so listing a show with
// many large scripts stays cheap.
type scriptListEntryView struct {
	ID                string                      `json:"id"`
	Name              string                      `json:"name"`
	LastRunStatus     string                      `json:"last_run_status"`
	LastRunReason     string                      `json:"last_run_reason,omitempty"`
	CapabilityProfile scriptCapabilityProfileView `json:"capability_profile"`
}

// toCapabilityProfileView projects a show.CapabilityProfile into its
// rendered view shape.
func toCapabilityProfileView(p show.CapabilityProfile) scriptCapabilityProfileView {
	return scriptCapabilityProfileView{
		Scope:           string(p.Scope),
		Preset:          string(p.Preset),
		DeadlineSeconds: p.DeadlineSeconds,
		RatePerSecond:   p.RatePerSecond,
		MemoryLimitMB:   p.MemoryLimitMB,
		CPUCapPercent:   p.CPUCapPercent,
	}
}

// toScriptView projects a show.Script into its full rendered view shape.
func toScriptView(s show.Script) scriptView {
	return scriptView{
		ID:                s.ID.String(),
		Name:              s.Name,
		Source:            s.Source,
		CapabilityProfile: toCapabilityProfileView(s.CapabilityProfile),
		LastRunStatus:     string(s.LastRunStatus),
		LastRunReason:     s.LastRunReason,
		LastRunAt:         s.LastRunAt,
	}
}

// toScriptListEntryView projects a show.Script into "script list"'s
// compact per-entry view shape.
func toScriptListEntryView(s show.Script) scriptListEntryView {
	return scriptListEntryView{
		ID:                s.ID.String(),
		Name:              s.Name,
		LastRunStatus:     string(s.LastRunStatus),
		LastRunReason:     s.LastRunReason,
		CapabilityProfile: toCapabilityProfileView(s.CapabilityProfile),
	}
}

// encodeScriptResult canonically encodes view as this file's uniform
// success-output shape -- every "script *" route writes deterministic
// JSON to Stdout, never plain text (unlike scene.go/api-key.go's default-
// text-with-optional---json convention).
func encodeScriptResult(view any) Result {
	payload, err := strictjson.CanonicalEncode(view)
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_ENCODE_FAILED: %v\n", err)}
	}
	return Result{Stdout: payload}
}

// scriptByName returns the script in scripts whose Name matches name,
// plus its index (so the caller can splice a mutated copy back into
// place), mirroring scene.go's sceneByName exactly.
func scriptByName(scripts []show.Script, name string) (show.Script, int, bool) {
	for i, s := range scripts {
		if s.Name == name {
			return s, i, true
		}
	}
	return show.Script{}, -1, false
}

// parseScriptPositionalArgs accepts a required positional <name> followed
// by any number of "--flag value"/"--flag=value" pairs, rejecting
// anything else (GOLC_SCRIPT_USAGE). It returns the full flag map so each
// route validates its own required/optional flags -- mirrors scene.go's
// parseSceneNameShowArgs shape, generalized beyond a single --show flag.
func parseScriptPositionalArgs(usage string, args []string) (name string, flags map[string]string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", nil, fmt.Errorf("GOLC_SCRIPT_USAGE: usage: %s", usage)
	}
	name = args[0]
	flags = map[string]string{}

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		if !strings.HasPrefix(argument, "--") {
			return "", nil, fmt.Errorf("GOLC_SCRIPT_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
		if eq := strings.Index(argument, "="); eq >= 0 {
			flags[argument[2:eq]] = argument[eq+1:]
			i++
			continue
		}
		flagName := strings.TrimPrefix(argument, "--")
		if i+1 >= len(rest) {
			return "", nil, fmt.Errorf("GOLC_SCRIPT_USAGE: --%s requires a value; usage: %s", flagName, usage)
		}
		flags[flagName] = rest[i+1]
		i += 2
	}
	return name, flags, nil
}

// parseScriptFlags is parseScriptPositionalArgs without a positional name
// -- used only by "script list", the one script route with no target
// script.
func parseScriptFlags(usage string, args []string) (map[string]string, error) {
	flags := map[string]string{}
	for i := 0; i < len(args); {
		argument := args[i]
		if !strings.HasPrefix(argument, "--") {
			return nil, fmt.Errorf("GOLC_SCRIPT_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
		if eq := strings.Index(argument, "="); eq >= 0 {
			flags[argument[2:eq]] = argument[eq+1:]
			i++
			continue
		}
		flagName := strings.TrimPrefix(argument, "--")
		if i+1 >= len(args) {
			return nil, fmt.Errorf("GOLC_SCRIPT_USAGE: --%s requires a value; usage: %s", flagName, usage)
		}
		flags[flagName] = args[i+1]
		i += 2
	}
	return flags, nil
}

// rejectUnknownScriptFlags fails with GOLC_SCRIPT_USAGE (a malformed
// invocation, ExitCode 2) if flags carries any key outside allowed --
// every script route has an exact known flag set, and an unrecognized
// flag is never silently ignored.
func rejectUnknownScriptFlags(usage string, flags map[string]string, allowed map[string]bool) error {
	for name := range flags {
		if !allowed[name] {
			return fmt.Errorf("GOLC_SCRIPT_USAGE: unsupported argument %q; usage: %s", "--"+name, usage)
		}
	}
	return nil
}

// runScriptCreate serves the self-registered "script create" route
// (SCRP-01): load the ShowState at --show, append a new empty script
// (show.NewScript: quick-action preset, least-privileged playback scope),
// and save atomically. A duplicate script name is rejected by show.Save's
// whole-State validation (surfaced as GOLC_SCRIPT_NAME_DUPLICATE inside
// the wrapping GOLC_SHOW_STATE_INVALID diagnostic) -- never a silent
// duplicate, and the show is left unchanged.
func runScriptCreate(request Request) Result {
	usage := "script create <name> --show <path>"
	name, flags, err := parseScriptPositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownScriptFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newScript, err := show.NewScript(name)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Scripts = append(state.Scripts, newScript)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return encodeScriptResult(toScriptView(newScript))
}

// runScriptList serves the self-registered "script list" route (D-16):
// project every script in the ShowState at --show into the compact
// library-view JSON shape (name, last-run status, capability profile --
// never source). A show with no scripts writes an empty JSON array, never
// null.
func runScriptList(request Request) Result {
	usage := "script list --show <path>"
	flags, err := parseScriptFlags(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownScriptFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	views := make([]scriptListEntryView, 0, len(state.Scripts))
	for _, s := range state.Scripts {
		views = append(views, toScriptListEntryView(s))
	}
	return encodeScriptResult(views)
}

// runScriptShow serves the self-registered "script show" route: return
// one script in full, including its source. An unknown name fails with
// GOLC_SCRIPT_NOT_FOUND.
func runScriptShow(request Request) Result {
	usage := "script show <name> --show <path>"
	name, flags, err := parseScriptPositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownScriptFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	target, _, found := scriptByName(state.Scripts, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_NOT_FOUND: no script named %q exists\n", name)}
	}
	return encodeScriptResult(toScriptView(target))
}

// runScriptEdit serves the self-registered "script edit" route (D-14):
// read --source-file's bytes verbatim -- no normalization, no transpile,
// no reformat -- into the named script's Source, rejecting a source file
// larger than maxScriptSourceBytes (GOLC_SCRIPT_SOURCE_TOO_LARGE) before
// it can ever enter the show blob. An unknown script name fails with
// GOLC_SCRIPT_NOT_FOUND.
func runScriptEdit(request Request) Result {
	usage := "script edit <name> --source-file <path> --show <path>"
	name, flags, err := parseScriptPositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownScriptFlags(usage, flags, map[string]bool{"show": true, "source-file": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	sourceFile, ok := flags["source-file"]
	if !ok || sourceFile == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_USAGE: --source-file is required; usage: %s\n", usage)}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	target, index, found := scriptByName(state.Scripts, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_NOT_FOUND: no script named %q exists\n", name)}
	}

	resolvedSourcePath := resolveWritablePath(request.Root, sourceFile)
	info, statErr := os.Stat(resolvedSourcePath)
	if statErr != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_SOURCE_READ_FAILED: %v\n", statErr)}
	}
	if info.Size() > maxScriptSourceBytes {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil,
			"GOLC_SCRIPT_SOURCE_TOO_LARGE: source file %q is %d bytes, exceeding the %d byte maximum\n", sourceFile, info.Size(), maxScriptSourceBytes)}
	}
	sourceBytes, readErr := os.ReadFile(resolvedSourcePath)
	if readErr != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_SOURCE_READ_FAILED: %v\n", readErr)}
	}

	target.Source = string(sourceBytes)
	state.Scripts[index] = target

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return encodeScriptResult(toScriptView(target))
}

// runScriptDelete serves the self-registered "script delete" route:
// remove the named script from the ShowState at --show and save
// atomically. An unknown script name fails with GOLC_SCRIPT_NOT_FOUND
// rather than a silent no-op.
func runScriptDelete(request Request) Result {
	usage := "script delete <name> --show <path>"
	name, flags, err := parseScriptPositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownScriptFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	_, index, found := scriptByName(state.Scripts, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_NOT_FOUND: no script named %q exists\n", name)}
	}
	state.Scripts = append(state.Scripts[:index], state.Scripts[index+1:]...)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_SCRIPT_DELETED: %s\n", name)}
}

// parseScriptLimitFlag parses one optional integer limit flag, returning
// GOLC_SCRIPT_LIMIT_INVALID (a malformed invocation, ExitCode 2) if
// present but not a valid integer. A negative or zero value is not itself
// rejected here -- per D-09 it resolves to the package safe default at
// CapabilityProfile.ResolveResourceLimits time, it is not a usage error.
func parseScriptLimitFlag(usage, flagName string, flags map[string]string, current int) (int, error) {
	raw, ok := flags[flagName]
	if !ok {
		return current, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("GOLC_SCRIPT_LIMIT_INVALID: --%s value %q is not a valid integer; usage: %s", flagName, raw, usage)
	}
	return value, nil
}

// runScriptProfileSet serves the self-registered "script profile set"
// route (D-07/D-09): load the ShowState, apply only the flags the caller
// actually supplied on this invocation to the named script's
// CapabilityProfile -- every field the caller did not mention is carried
// forward unchanged from the existing profile, so a partial edit never
// silently resets the other fields -- and save atomically. An invalid
// --scope or --preset is rejected by show.Save's whole-State validation
// (surfaced as GOLC_SCRIPT_SCOPE_INVALID/GOLC_SCRIPT_PRESET_INVALID
// inside the wrapping GOLC_SHOW_STATE_INVALID diagnostic); nothing is
// saved when that happens. An unknown script name fails with
// GOLC_SCRIPT_NOT_FOUND.
func runScriptProfileSet(request Request) Result {
	usage := "script profile set <name> [--scope <playback|authoring|admin>] [--preset <quick-action|long-running-automation|advanced>] " +
		"[--deadline-seconds <n>] [--rate-per-second <n>] [--memory-limit-mb <n>] [--cpu-cap-percent <n>] --show <path>"
	name, flags, err := parseScriptPositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownScriptFlags(usage, flags, map[string]bool{
		"show": true, "scope": true, "preset": true,
		"deadline-seconds": true, "rate-per-second": true, "memory-limit-mb": true, "cpu-cap-percent": true,
	}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	target, index, found := scriptByName(state.Scripts, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_NOT_FOUND: no script named %q exists\n", name)}
	}

	profile := target.CapabilityProfile
	if raw, ok := flags["scope"]; ok {
		profile.Scope = show.APIKeyScope(raw)
	}
	if raw, ok := flags["preset"]; ok {
		profile.Preset = show.ResourcePreset(raw)
	}
	deadlineSeconds, limitErr := parseScriptLimitFlag(usage, "deadline-seconds", flags, profile.DeadlineSeconds)
	if limitErr != nil {
		return Result{ExitCode: 2, Stderr: []byte(limitErr.Error() + "\n")}
	}
	profile.DeadlineSeconds = deadlineSeconds

	ratePerSecond, limitErr := parseScriptLimitFlag(usage, "rate-per-second", flags, profile.RatePerSecond)
	if limitErr != nil {
		return Result{ExitCode: 2, Stderr: []byte(limitErr.Error() + "\n")}
	}
	profile.RatePerSecond = ratePerSecond

	memoryLimitMB, limitErr := parseScriptLimitFlag(usage, "memory-limit-mb", flags, profile.MemoryLimitMB)
	if limitErr != nil {
		return Result{ExitCode: 2, Stderr: []byte(limitErr.Error() + "\n")}
	}
	profile.MemoryLimitMB = memoryLimitMB

	cpuCapPercent, limitErr := parseScriptLimitFlag(usage, "cpu-cap-percent", flags, profile.CPUCapPercent)
	if limitErr != nil {
		return Result{ExitCode: 2, Stderr: []byte(limitErr.Error() + "\n")}
	}
	profile.CPUCapPercent = cpuCapPercent

	target.CapabilityProfile = profile
	state.Scripts[index] = target

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return encodeScriptResult(toScriptView(target))
}
