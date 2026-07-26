// svc_script.go fills ScriptService, the Wails binding closing SCRP-01
// (08-04-PLAN.md Task 1): a user creates, inspects, edits, deletes, and
// assigns capability profiles to TypeScript automation scripts through the
// exact already-implemented, already-tested "script"* CLI routes
// (internal/command/script.go) via command.NewDefaultCommandRegistry --
// exactly the ProgrammingService/ShowService pattern (svc_programming.go/
// svc_show.go) this file mirrors -- so there is only one script mutation
// implementation in this codebase, never a second one duplicated for the
// GUI.
//
// SaveScriptSource cannot pass a multi-line TypeScript body as an argv
// value safely, and "script edit" takes --source-file: SaveScriptSource
// writes the source to a temp file (os.CreateTemp inside os.TempDir(),
// removed via defer) and passes that path to "script edit --source-file",
// guarded by the same maxScriptSourceBytes (1 MiB) bound 08-01's
// internal/command/script.go declares, checked before any write
// (T-08-03/T-08-12).
//
// ListScripts/GetScript decode "script list"/"script show"'s Stdout JSON
// with internal/strictjson.DecodeStrict into unexported wire types that
// mirror internal/command/script.go's own scriptListEntryView/scriptView
// shapes field-for-field/tag-for-tag, then flatten the nested
// capability_profile member into this package's own flat
// ScriptSummaryView/ScriptDetailView shape for the frontend.
package wails

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/script"
	"github.com/lnorton89/golc/internal/strictjson"
)

// maxScriptSourceBytes bounds SaveScriptSource's input (T-08-03/T-08-12 DoS
// mitigation) -- mirrors internal/command/script.go's identical constant
// (unexported to that package, so this file keeps its own copy); a source
// larger than this is rejected with GOLC_SCRIPT_SOURCE_TOO_LARGE before any
// temp file is ever written.
const maxScriptSourceBytes = 1 << 20 // 1 MiB

// ScriptService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. root/showPath are the exact ShowState location
// every method acts against (mirrors ProgrammingService/ShowService's own
// fields). events is this service's own EventPusher (events.go's throttle
// scaffold) -- mirrors SafetyService/MidiService's identical
// self-constructed-in-NewXService pattern (own field, own Start/Stop
// lifecycle called by this service's own Start/StopScriptEventStream
// methods) rather than threading a shared *EventPusher through the
// constructor, so this service's lifecycle stays self-contained exactly
// like its siblings and cmd/golc-desktop/main.go needs no new wiring.
type ScriptService struct {
	pipeName string
	root     string
	showPath string
	events   *EventPusher

	mu           sync.Mutex
	streamCancel context.CancelFunc
	streamDone   chan struct{}
}

// NewScriptService constructs a ScriptService targeting pipeName (reserved,
// unused by this ShowState-only CRUD -- mirrors ProgrammingService/
// ShowService's own unused pipeName field) and the ShowState at showPath,
// resolved against root.
func NewScriptService(pipeName, root, showPath string) *ScriptService {
	return &ScriptService{pipeName: pipeName, root: root, showPath: showPath, events: NewEventPusher()}
}

// execute builds the default command registry and runs args against it,
// converting the internal/command.Result shape into this package's own
// Result shape (mirrors svc_programming.go/svc_show.go's identical
// helper).
func (s *ScriptService) execute(args ...string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// scriptCapabilityProfileWire mirrors internal/command/script.go's
// scriptCapabilityProfileView JSON shape exactly, so this package can
// strictjson.DecodeStrict "script list"/"script show"'s Stdout without
// duplicating parsing logic.
type scriptCapabilityProfileWire struct {
	Scope           string `json:"scope"`
	Preset          string `json:"preset"`
	DeadlineSeconds int    `json:"deadline_seconds"`
	RatePerSecond   int    `json:"rate_per_second"`
	MemoryLimitMB   int    `json:"memory_limit_mb"`
	CPUCapPercent   int    `json:"cpu_cap_percent"`
}

// scriptListEntryWire mirrors internal/command/script.go's
// scriptListEntryView JSON shape exactly ("script list"'s per-script
// payload -- no Source).
type scriptListEntryWire struct {
	ID                string                      `json:"id"`
	Name              string                      `json:"name"`
	LastRunStatus     string                      `json:"last_run_status"`
	LastRunReason     string                      `json:"last_run_reason,omitempty"`
	CapabilityProfile scriptCapabilityProfileWire `json:"capability_profile"`
}

// scriptWire mirrors internal/command/script.go's scriptView JSON shape
// exactly (the full per-script payload "script create"/"script show"/
// "script edit" write to Stdout, including Source).
type scriptWire struct {
	ID                string                      `json:"id"`
	Name              string                      `json:"name"`
	Source            string                      `json:"source"`
	CapabilityProfile scriptCapabilityProfileWire `json:"capability_profile"`
	LastRunStatus     string                      `json:"last_run_status"`
	LastRunReason     string                      `json:"last_run_reason,omitempty"`
	LastRunAt         string                      `json:"last_run_at,omitempty"`
}

// ScriptSummaryView is the D-16 library-row projection: one script's
// identity, last-run status, and flattened capability-profile summary
// (Source omitted -- mirrors "script list"'s own cheap-listing rationale).
type ScriptSummaryView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	LastRunStatus   string `json:"lastRunStatus"`
	LastRunReason   string `json:"lastRunReason,omitempty"`
	Scope           string `json:"scope"`
	Preset          string `json:"preset"`
	DeadlineSeconds int    `json:"deadlineSeconds"`
	RatePerSecond   int    `json:"ratePerSecond"`
	MemoryLimitMB   int    `json:"memoryLimitMB"`
	CPUCapPercent   int    `json:"cpuCapPercent"`
}

// ScriptDetailView is GetScript's return shape: ScriptSummaryView's fields
// plus Source, the script's full TypeScript text.
type ScriptDetailView struct {
	ScriptSummaryView
	Source string `json:"source"`
}

// summaryFromProfile flattens id, name, lastRunStatus/Reason, and a
// scriptCapabilityProfileWire into ScriptSummaryView -- shared by
// ListScripts (from scriptListEntryWire) and GetScript (from scriptWire).
func summaryFromProfile(id, name, lastRunStatus, lastRunReason string, profile scriptCapabilityProfileWire) ScriptSummaryView {
	return ScriptSummaryView{
		ID:              id,
		Name:            name,
		LastRunStatus:   lastRunStatus,
		LastRunReason:   lastRunReason,
		Scope:           profile.Scope,
		Preset:          profile.Preset,
		DeadlineSeconds: profile.DeadlineSeconds,
		RatePerSecond:   profile.RatePerSecond,
		MemoryLimitMB:   profile.MemoryLimitMB,
		CPUCapPercent:   profile.CPUCapPercent,
	}
}

// ListScripts returns a []ScriptSummaryView decoded from "script list"'s
// JSON (D-16), or an error when the show cannot be read (a non-zero exit
// from the CLI route, or a malformed/unparseable Stdout payload). A show
// with no scripts returns an explicit empty (non-nil) slice.
func (s *ScriptService) ListScripts() ([]ScriptSummaryView, error) {
	result := s.execute("script", "list", "--show", s.showPath)
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s", result.Stderr)
	}

	var wire []scriptListEntryWire
	if err := strictjson.DecodeStrict([]byte(result.Stdout), &wire); err != nil {
		return nil, fmt.Errorf("GOLC_WAILS_SCRIPT_LIST_DECODE_FAILED: %v", err)
	}

	views := make([]ScriptSummaryView, 0, len(wire))
	for _, w := range wire {
		views = append(views, summaryFromProfile(w.ID, w.Name, w.LastRunStatus, w.LastRunReason, w.CapabilityProfile))
	}
	return views, nil
}

// GetScript returns a ScriptDetailView (including Source) decoded from
// "script show <name>"'s JSON. An unknown script name surfaces the route's
// own GOLC_SCRIPT_NOT_FOUND diagnostic as the returned error.
func (s *ScriptService) GetScript(name string) (ScriptDetailView, error) {
	result := s.execute("script", "show", name, "--show", s.showPath)
	if result.ExitCode != 0 {
		return ScriptDetailView{}, fmt.Errorf("%s", result.Stderr)
	}

	var wire scriptWire
	if err := strictjson.DecodeStrict([]byte(result.Stdout), &wire); err != nil {
		return ScriptDetailView{}, fmt.Errorf("GOLC_WAILS_SCRIPT_SHOW_DECODE_FAILED: %v", err)
	}

	return ScriptDetailView{
		ScriptSummaryView: summaryFromProfile(wire.ID, wire.Name, wire.LastRunStatus, wire.LastRunReason, wire.CapabilityProfile),
		Source:            wire.Source,
	}, nil
}

// CreateScript creates a new named, empty script via "script create <name>
// --show <path>" (SCRP-01): Result{ExitCode:0} on success,
// Result{ExitCode:1, Stderr: "...GOLC_SCRIPT_NAME_DUPLICATE..."} when name
// is already taken.
func (s *ScriptService) CreateScript(name string) Result {
	return s.execute("script", "create", name, "--show", s.showPath)
}

// SaveScriptSource persists source verbatim as the named script's Source
// (D-14) via "script edit <name> --source-file <path> --show <path>" --
// see the package doc comment for the temp-file argv-safety rationale
// (T-08-12). A source exceeding maxScriptSourceBytes is rejected with
// GOLC_SCRIPT_SOURCE_TOO_LARGE before any temp file is written.
func (s *ScriptService) SaveScriptSource(name, source string) Result {
	if len(source) > maxScriptSourceBytes {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf(
			"GOLC_SCRIPT_SOURCE_TOO_LARGE: source is %d bytes, exceeding the %d byte maximum", len(source), maxScriptSourceBytes)}
	}

	tempFile, err := os.CreateTemp("", "golc-script-source-*.ts")
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_SCRIPT_TEMP_FILE_FAILED: %v", err)}
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, writeErr := tempFile.WriteString(source); writeErr != nil {
		_ = tempFile.Close()
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_SCRIPT_TEMP_FILE_FAILED: %v", writeErr)}
	}
	if closeErr := tempFile.Close(); closeErr != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_SCRIPT_TEMP_FILE_FAILED: %v", closeErr)}
	}

	return s.execute("script", "edit", name, "--source-file", tempPath, "--show", s.showPath)
}

// DeleteScript removes the named script via "script delete <name> --show
// <path>"; a subsequent ListScripts omits it. An unknown script name
// surfaces the route's own GOLC_SCRIPT_NOT_FOUND diagnostic.
func (s *ScriptService) DeleteScript(name string) Result {
	return s.execute("script", "delete", name, "--show", s.showPath)
}

// appendPositiveIntFlag appends --flagName <value> to args when value is
// strictly positive -- the shared "only forward what the caller actually
// supplied" rule SetScriptProfile applies to every numeric limit field
// (D-09's partial-edit discipline: an omitted/non-positive field must
// never overwrite the script's existing saved value).
func appendPositiveIntFlag(args []string, flagName string, value int) []string {
	if value > 0 {
		return append(args, "--"+flagName, strconv.Itoa(value))
	}
	return args
}

// SetScriptProfile sets the named script's saved capability/resource-limit
// profile via "script profile set <name> [...] --show <path>" (D-07/D-09),
// forwarding only the non-empty/positive values as flags -- an empty
// scope/preset or a non-positive numeric limit is omitted from the CLI
// invocation entirely, leaving that field untouched on the existing
// profile (mirrors internal/command/script.go's runScriptProfileSet
// partial-edit discipline exactly).
func (s *ScriptService) SetScriptProfile(name, scope, preset string, deadlineSeconds, ratePerSecond, memoryLimitMB, cpuCapPercent int) Result {
	args := []string{"script", "profile", "set", name}
	if scope != "" {
		args = append(args, "--scope", scope)
	}
	if preset != "" {
		args = append(args, "--preset", preset)
	}
	args = appendPositiveIntFlag(args, "deadline-seconds", deadlineSeconds)
	args = appendPositiveIntFlag(args, "rate-per-second", ratePerSecond)
	args = appendPositiveIntFlag(args, "memory-limit-mb", memoryLimitMB)
	args = appendPositiveIntFlag(args, "cpu-cap-percent", cpuCapPercent)
	args = append(args, "--show", s.showPath)
	return s.execute(args...)
}

// toScriptEventView projects a script.ScriptEvent (internal/script/
// events.go, 08-08-PLAN.md Task 1) into this package's own JSON-safe
// ScriptEventView shape -- every field a plain string/number/bool, never
// the raw uuid.UUID/show.ScriptRunStatus/time.Time types Wails' TypeScript
// binding generator cannot render directly.
func toScriptEventView(ev script.ScriptEvent) ScriptEventView {
	return ScriptEventView{
		Seq:        ev.Seq,
		Kind:       string(ev.Kind),
		RunID:      ev.RunID.String(),
		ScriptName: ev.ScriptName,
		At:         ev.At.Format(time.RFC3339Nano),
		Level:      ev.Level,
		Message:    ev.Message,
		Source:     ev.Source,
		Method:     ev.Method,
		Route:      ev.Route,
		DurationMS: ev.DurationMS,
		Ok:         ev.Ok,
		Code:       ev.Code,
		Status:     string(ev.Status),
		Reason:     ev.Reason,
	}
}

// StartScriptEventStream begins forwarding every live script.ScriptEvent
// (D-04/D-05, 08-08-PLAN.md Task 3) from internal/script's process-wide bus
// to the desktop webview: it starts this service's own throttled
// EventPusher flush loop (mirrors SafetyService.StartStatusPush's own
// s.events.Start(ctx) call), subscribes to script.SubscribeScriptEvents
// with no prior Seq (a fresh live subscription -- no replay), and runs a
// forwarding goroutine that stages each received event via
// QueueScriptEvent until StopScriptEventStream cancels it or the
// subscription channel closes. Calling StartScriptEventStream again
// without an intervening StopScriptEventStream is a no-op (mirrors
// EventPusher.Start's own idempotency).
func (s *ScriptService) StartScriptEventStream(ctx context.Context) {
	s.mu.Lock()
	if s.streamCancel != nil {
		s.mu.Unlock()
		return
	}
	streamCtx, cancel := context.WithCancel(ctx)
	s.streamCancel = cancel
	s.streamDone = make(chan struct{})
	s.mu.Unlock()

	s.events.Start(ctx)

	_, _, ch, unsubscribe := script.SubscribeScriptEvents(0)
	go s.forwardScriptEvents(streamCtx, ch, unsubscribe)
}

// forwardScriptEvents drains ch (the live subscription StartScriptEventStream
// opened) until ctx is done (StopScriptEventStream) or ch closes, staging
// every received event via QueueScriptEvent so EventPusher's own flush
// loop emits it under "script:event" -- ordered, non-coalescing (Task 3's
// exact requirement). unsubscribe is always called on exit, and
// streamDone is always closed exactly once, so StopScriptEventStream can
// block until this goroutine has actually finished.
func (s *ScriptService) forwardScriptEvents(ctx context.Context, ch chan script.ScriptEvent, unsubscribe func()) {
	defer close(s.streamDone)
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			s.events.QueueScriptEvent(toScriptEventView(ev))
		}
	}
}

// StopScriptEventStream cancels the forwarding goroutine, waits for it to
// exit (guaranteeing unsubscribe has already run), and stops the
// underlying EventPusher -- mirrors SafetyService.StopStatusPush's own
// reverse-order subsystem stop discipline. Safe to call more than once or
// before StartScriptEventStream.
func (s *ScriptService) StopScriptEventStream() {
	s.mu.Lock()
	cancel := s.streamCancel
	done := s.streamDone
	s.streamCancel = nil
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		<-done
	}
	s.events.Stop()
}
