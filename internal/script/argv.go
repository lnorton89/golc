// argv.go converts one script cmd-call frame's JSON Params into the
// exact argv shape the real internal/command route it targets expects
// (08-05-PLAN.md Task 2's "build the argv for its route from the frame's
// Params" instruction). Each builder mirrors the documented CLI usage
// string internal/scriptsdk/descriptors.go already records for that
// route's Params type (cross-checked directly against the real parser in
// internal/command: pool.go, programming.go, scene.go, playback.go,
// operatorsurface.go, artnet.go, fixture.go, apikey.go, show.go,
// deployment.go).
//
// Every builder that targets a route accepting "--show <path>" always
// uses the Host's own configured ShowPath, never any "show" field the
// script's Params JSON might carry (mirrors internal/api/translate.go's
// buildShowArgs discipline: no script-controlled path is ever trusted to
// select which show document a call touches). This is the same
// host-side-enforcement principle 08-RESEARCH.md Pitfall 1 applies to
// Deno permissions, extended to path selection.
package script

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lnorton89/golc/internal/scriptsdk"
)

// decodeParams unmarshals raw into a fresh *T, returning a
// GOLC_SCRIPT_PARAMS_INVALID error on malformed JSON -- the frame's
// Params is attacker-controlled child stdio (T-08-17), never trusted
// without a typed decode.
func decodeParams[T any](raw json.RawMessage) (T, error) {
	var value T
	if len(raw) == 0 {
		return value, nil
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, fmt.Errorf("GOLC_SCRIPT_PARAMS_INVALID: %v", err)
	}
	return value, nil
}

// appendRepeated appends "--flag value" once per non-empty entry of
// values, in order -- the repeated-flag convention pool.go/scene.go/
// programming.go's "--add"/"--instance"/"--pool"/"--group"/"--fixture"/
// "--attr"/"--at" flags all share.
func appendRepeated(args []string, flag string, values []string) []string {
	for _, v := range values {
		args = append(args, flag, v)
	}
	return args
}

// buildRouteArgs converts one cmd-call frame's Params into the argv the
// real internal/command route named by descriptor.Route expects,
// injecting showPath wherever the route needs a "--show" flag. It
// returns GOLC_SCRIPT_PARAMS_INVALID for a Params shape that fails to
// decode and GOLC_SCRIPT_ROUTE_UNSUPPORTED for a descriptor this table
// has no builder for -- the latter should never happen for any route
// scriptsdk.RegisteredSDKMethods() actually returns (coverage_test.go's
// exhaustiveness gate in internal/scriptsdk keeps that list in sync with
// the real command registry); it exists so an unmapped route fails
// loudly rather than silently producing an empty argv.
func buildRouteArgs(route, showPath string, raw json.RawMessage) ([]string, error) {
	builder, ok := routeArgvBuilders[route]
	if !ok {
		return nil, fmt.Errorf("GOLC_SCRIPT_ROUTE_UNSUPPORTED: no argv builder registered for route %q", route)
	}
	return builder(showPath, raw)
}

// routeArgvBuilders maps every route scriptsdk exposes to a script onto
// the function that converts its Params JSON (plus the run's fixed
// ShowPath) into that route's real argv.
var routeArgvBuilders = map[string]func(showPath string, raw json.RawMessage) ([]string, error){
	// --- shared shapes -----------------------------------------------

	"programmer inspect": showOnlyArgs,
	"programmer clear":   showOnlyArgs,
	"show save":          showOnlyArgs,
	"show diagnose":      showOnlyArgs,
	"show export":        showOnlyArgs,

	"deployment create":      nameShowArgs,
	"deployment activate":    nameShowArgs,
	"scene activate":         nameShowArgs,
	"theme create":           nameShowArgs,
	"motion create":          nameShowArgs,
	"operatorsurface create": nameShowArgs,
	"scene delete":           nameShowArgs,
	"theme delete":           nameShowArgs,
	"preset delete":          nameShowArgs,
	"chase delete":           nameShowArgs,
	"motion delete":          nameShowArgs,

	"theme rename":     renameArgs,
	"preset rename":    renameArgs,
	"motion rename":    renameArgs,
	"motion duplicate": renameArgs,
	"scene rename":     renameArgs,
	"scene duplicate":  renameArgs,
	"chase duplicate":  renameArgs,

	// --- pool ----------------------------------------------------------

	"pool create":     buildPoolCreateArgs,
	"pool update":     buildPoolUpdateArgs,
	"pool apply":      buildPoolApplyArgs,
	"pool substitute": buildPoolSubstituteArgs,

	// --- show ------------------------------------------------------------

	"show inspect": showOnlyArgs,
	"show open":    buildShowOpenArgs,
	"show save-as": buildShowSaveAsArgs,

	// --- scene / blend / preset / chase / motion / programmer -----------

	"scene create":    buildSceneCreateArgs,
	"scene layer set": buildSceneLayerSetArgs,
	"blend create":    buildBlendCreateArgs,
	"preset record":   buildPresetRecordArgs,
	"chase create":    buildChaseCreateArgs,
	"chase update":    buildChaseUpdateArgs,
	"chase reorder":   buildChaseReorderArgs,
	"programmer set":  buildProgrammerSetArgs,

	// --- playback ----------------------------------------------------

	"playback bpm set": buildPlaybackBPMSetArgs,
	"playback bpm tap": buildPlaybackBPMTapArgs,
	"playback switch":  buildPlaybackSwitchArgs,

	// --- operatorsurface -----------------------------------------------

	"operatorsurface list":     showOnlyArgs,
	"operatorsurface show":     buildOperatorSurfaceSurfaceArgs,
	"operatorsurface remove":   buildOperatorSurfaceSurfaceArgs,
	"operatorsurface assign":   buildOperatorSurfaceAssignArgs,
	"operatorsurface unassign": buildOperatorSurfaceAssignArgs,

	// --- artnet (no --show flag; --pipe/none as documented) -------------

	"artnet status":                   buildArtnetStatusArgs,
	"artnet interface list":           buildArtnetInterfaceListArgs,
	"artnet discover":                 buildArtnetDiscoverArgs,
	"artnet configure":                buildArtnetConfigureArgs,
	"artnet target enable":            buildArtnetTargetArgs,
	"artnet target disable":           buildArtnetTargetArgs,
	"artnet master set":               buildArtnetMasterSetArgs,
	"artnet safety blackout":          buildArtnetSafetyArgs,
	"artnet safety stop-all":          buildArtnetSafetyArgs,
	"artnet safety revoke-automation": buildArtnetSafetyArgs,

	// --- fixture (no --show flag) ---------------------------------------

	"fixture validate": buildFixtureFileArgs,
	"fixture inspect":  buildFixtureFileArgs,
	"fixture import":   buildFixtureImportArgs,

	// --- api-key ---------------------------------------------------------

	"api-key create": buildAPIKeyCreateArgs,
	"api-key list":   buildAPIKeyListArgs,
	"api-key revoke": buildAPIKeyRevokeArgs,
}

// --- shared-shape builders -------------------------------------------

func showOnlyArgs(showPath string, _ json.RawMessage) ([]string, error) {
	return []string{"--show", showPath}, nil
}

func nameShowArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.NameShowParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{p.Name, "--show", showPath}, nil
}

func renameArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.RenameParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{p.Name, p.NewName, "--show", showPath}, nil
}

// --- pool --------------------------------------------------------------

// buildPoolCreateArgs: "pool create <name> [--requires <cap1,cap2,...>] --show <path>".
func buildPoolCreateArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.PoolCreateParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{p.Name}
	if len(p.Requires) > 0 {
		args = append(args, "--requires", strings.Join(p.Requires, ","))
	}
	return append(args, "--show", showPath), nil
}

// buildPoolUpdateArgs: "pool update <pool> [--add ...]... [--remove <id>]...
// [--propagate immediate|preview] [--out <path>] [--json] --show <path>".
func buildPoolUpdateArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.PoolUpdateParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{p.Pool}
	args = appendRepeated(args, "--add", p.Add)
	args = appendRepeated(args, "--remove", p.Remove)
	if p.Propagate != "" {
		args = append(args, "--propagate", p.Propagate)
	}
	if p.Out != "" {
		args = append(args, "--out", p.Out)
	}
	// "pool update"'s Result is scriptsdk.JSONResult -- the typed Promise a
	// script awaits always needs the plan's canonical JSON, never the
	// plain-text summary, so --json is always requested here regardless of
	// p.JSONOutput (unless the caller redirected to --out instead).
	if p.Out == "" {
		args = append(args, "--json")
	}
	return append(args, "--show", showPath), nil
}

// buildPoolApplyArgs: "pool apply {plan-file} --plan-id <id> --show <path>".
func buildPoolApplyArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.PoolApplyParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{p.PlanFile, "--plan-id", p.PlanID, "--show", showPath}, nil
}

// buildPoolSubstituteArgs: "pool substitute <pool> --from <fixture-file>
// --to <fixture-file> [--out <path>] [--json] --show <path>".
func buildPoolSubstituteArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.PoolSubstituteParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{p.Pool, "--from", p.From, "--to", p.To}
	if p.Out != "" {
		args = append(args, "--out", p.Out)
	}
	// "pool substitute"'s Result is scriptsdk.JSONResult -- see
	// buildPoolUpdateArgs's identical rationale.
	if p.Out == "" {
		args = append(args, "--json")
	}
	return append(args, "--show", showPath), nil
}

// --- show --------------------------------------------------------------

// buildShowOpenArgs: "show open --show <path> [--accept-recovery <id>]
// [--discard-recovery]".
func buildShowOpenArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ShowOpenParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{"--show", showPath}
	if p.AcceptRecovery != "" {
		args = append(args, "--accept-recovery", p.AcceptRecovery)
	}
	if p.DiscardRecovery {
		args = append(args, "--discard-recovery")
	}
	return args, nil
}

// buildShowSaveAsArgs: "show save-as --show <src> --to <dest>".
func buildShowSaveAsArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ShowSaveAsParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{"--show", showPath, "--to", p.To}, nil
}

// --- scene / blend / preset / chase / motion / programmer -------------

// buildSceneCreateArgs: "scene create <name> --bars <n> --show <path>".
func buildSceneCreateArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.SceneCreateParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{p.Name, "--bars", strconv.Itoa(p.Bars), "--show", showPath}, nil
}

// buildSceneLayerSetArgs: "scene layer set <scene> --kind
// base_look|color_theme|chase|motion [--ref <id>] [--instance <id>]...
// [--pool <id>]... [--group <id>]... [--fixture <pool_id>|<pool_member_id>]...
// [--disable] --show <path>".
func buildSceneLayerSetArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.SceneLayerSetParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{p.Scene, "--kind", p.Kind}
	if p.Ref != "" {
		args = append(args, "--ref", p.Ref)
	}
	args = appendRepeated(args, "--instance", p.Instances)
	args = appendRepeated(args, "--pool", p.Pools)
	args = appendRepeated(args, "--group", p.Groups)
	args = appendRepeated(args, "--fixture", p.FixtureRefs)
	if p.Disable {
		args = append(args, "--disable")
	}
	return append(args, "--show", showPath), nil
}

// buildBlendCreateArgs: "blend create <name> --duration-bars <value>
// [--curve linear|ease_in|ease_out] --show <path>".
func buildBlendCreateArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.BlendCreateParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{p.Name, "--duration-bars", formatFloat(p.DurationBars)}
	if p.Curve != "" {
		args = append(args, "--curve", p.Curve)
	}
	return append(args, "--show", showPath), nil
}

// buildPresetRecordArgs: "preset record <name> --kind
// intensity|color|position|beam --show <path>".
func buildPresetRecordArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.PresetRecordParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{p.Name, "--kind", p.Kind, "--show", showPath}, nil
}

// buildChaseCreateArgs: "chase create <name> --unit bar|beat
// --step-duration <value> --show <path>".
func buildChaseCreateArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ChaseCreateParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{p.Name, "--unit", p.Unit, "--step-duration", formatFloat(p.StepDuration), "--show", showPath}, nil
}

// buildChaseUpdateArgs: "chase update <name> [--name <new-name>]
// [--unit bar|beat] [--step-duration <value>] --show <path>".
func buildChaseUpdateArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ChaseUpdateParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{p.Name}
	if p.NewName != "" {
		args = append(args, "--name", p.NewName)
	}
	if p.Unit != "" {
		args = append(args, "--unit", p.Unit)
	}
	if p.StepDuration != nil {
		args = append(args, "--step-duration", formatFloat(*p.StepDuration))
	}
	return append(args, "--show", showPath), nil
}

// buildChaseReorderArgs: "chase reorder <name> --order <i,j,k,...> --show <path>".
func buildChaseReorderArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ChaseReorderParams](raw)
	if err != nil {
		return nil, err
	}
	order := make([]string, len(p.Order))
	for i, v := range p.Order {
		order[i] = strconv.Itoa(v)
	}
	return []string{p.Name, "--order", strings.Join(order, ","), "--show", showPath}, nil
}

// buildProgrammerSetArgs: "programmer set [--instance <id>]...
// [--pool <id>]... [--group <id>]... [--fixture <pool_id>|<pool_member_id>]...
// --attr <capability>=<value>... --show <path>".
func buildProgrammerSetArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ProgrammerSetParams](raw)
	if err != nil {
		return nil, err
	}
	var args []string
	args = appendRepeated(args, "--instance", p.Instances)
	args = appendRepeated(args, "--pool", p.Pools)
	args = appendRepeated(args, "--group", p.Groups)
	args = appendRepeated(args, "--fixture", p.FixtureRefs)
	args = appendRepeated(args, "--attr", p.Attrs)
	return append(args, "--show", showPath), nil
}

// --- playback ------------------------------------------------------------

// buildPlaybackBPMSetArgs: "playback bpm set <bpm> --show <path>".
func buildPlaybackBPMSetArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.PlaybackBPMSetParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{formatFloat(p.BPM), "--show", showPath}, nil
}

// buildPlaybackBPMTapArgs: "playback bpm tap --at <RFC3339-timestamp>...
// --show <path>".
func buildPlaybackBPMTapArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.PlaybackBPMTapParams](raw)
	if err != nil {
		return nil, err
	}
	args := appendRepeated(nil, "--at", p.At)
	return append(args, "--show", showPath), nil
}

// buildPlaybackSwitchArgs: "playback switch <scene> --show <path>".
func buildPlaybackSwitchArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.PlaybackSwitchParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{p.Scene, "--show", showPath}, nil
}

// --- operatorsurface -------------------------------------------------

// buildOperatorSurfaceSurfaceArgs: "operatorsurface show|remove --surface
// <name> --show <path>".
func buildOperatorSurfaceSurfaceArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.OperatorSurfaceSurfaceParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{"--surface", p.Surface, "--show", showPath}, nil
}

// buildOperatorSurfaceAssignArgs: "operatorsurface assign|unassign
// --surface <name> [--scene <scene>|--layer <scene>:<kind>|--master
// grand|--master group:<group>|--safety <...>] --show <path>". Exactly
// one selector field is expected to be set; the first non-empty one in
// this fixed precedence order is used.
func buildOperatorSurfaceAssignArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.OperatorSurfaceAssignParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{"--surface", p.Surface}
	switch {
	case p.Scene != "":
		args = append(args, "--scene", p.Scene)
	case p.Layer != "":
		args = append(args, "--layer", p.Layer)
	case p.Master != "":
		args = append(args, "--master", p.Master)
	case p.Safety != "":
		args = append(args, "--safety", p.Safety)
	}
	return append(args, "--show", showPath), nil
}

// --- artnet (no --show flag) -------------------------------------------

// buildArtnetStatusArgs always requests --json and never --watch: "artnet
// status"'s Result is scriptsdk.JSONResult, and --watch is a
// continuously-refreshing interactive CLI loop that would never return --
// a script's one-shot request/response call could never observe it
// complete. p.Watch is therefore intentionally ignored.
func buildArtnetStatusArgs(_ string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ArtnetStatusParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{"--json"}
	if p.Pipe != "" {
		args = append(args, "--pipe", p.Pipe)
	}
	return args, nil
}

// buildArtnetInterfaceListArgs always requests --json: "artnet interface
// list"'s Result is scriptsdk.JSONResult.
func buildArtnetInterfaceListArgs(_ string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ArtnetInterfaceListParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{"--json"}
	if p.Pipe != "" {
		args = append(args, "--pipe", p.Pipe)
	}
	return args, nil
}

// buildArtnetDiscoverArgs always requests --json: "artnet discover"'s
// Result is scriptsdk.JSONResult.
func buildArtnetDiscoverArgs(_ string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ArtnetDiscoverParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{"--interface", strconv.Itoa(p.Interface), "--json"}
	if p.Window != "" {
		args = append(args, "--window", p.Window)
	}
	return args, nil
}

func buildArtnetConfigureArgs(_ string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ArtnetConfigureParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{"--universe", strconv.Itoa(p.Universe), "--ip", p.IP}
	if p.Port != 0 {
		args = append(args, "--port", strconv.Itoa(p.Port))
	}
	args = append(args, "--enabled", strconv.FormatBool(p.Enabled))
	if p.Pipe != "" {
		args = append(args, "--pipe", p.Pipe)
	}
	return args, nil
}

func buildArtnetTargetArgs(_ string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ArtnetTargetParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{"--universe", strconv.Itoa(p.Universe), "--ip", p.IP}
	if p.Port != 0 {
		args = append(args, "--port", strconv.Itoa(p.Port))
	}
	if p.Pipe != "" {
		args = append(args, "--pipe", p.Pipe)
	}
	return args, nil
}

func buildArtnetSafetyArgs(_ string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ArtnetSafetyParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{"--on", strconv.FormatBool(p.On)}
	if p.Pipe != "" {
		args = append(args, "--pipe", p.Pipe)
	}
	return args, nil
}

func buildArtnetMasterSetArgs(_ string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.ArtnetMasterSetParams](raw)
	if err != nil {
		return nil, err
	}
	var args []string
	if p.Group != "" {
		args = []string{"--group", p.Group, "--level", formatFloat(p.Level)}
	} else {
		args = []string{"--grand", formatFloat(p.Grand)}
	}
	if p.Pipe != "" {
		args = append(args, "--pipe", p.Pipe)
	}
	return args, nil
}

// --- fixture (no --show flag) -------------------------------------------

func buildFixtureFileArgs(_ string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.FixtureFileParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{p.File}, nil
}

func buildFixtureImportArgs(_ string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.FixtureImportParams](raw)
	if err != nil {
		return nil, err
	}
	var args []string
	switch {
	case p.OFL != "":
		args = append(args, "--ofl", p.OFL)
	case p.OFLFile != "":
		args = append(args, "--ofl-file", p.OFLFile)
	}
	if p.Mirror != "" {
		args = append(args, "--mirror", p.Mirror)
	}
	if p.AllowMirror {
		args = append(args, "--allow-mirror")
	}
	return append(args, "--out", p.Out), nil
}

// --- api-key -----------------------------------------------------------

// buildAPIKeyCreateArgs always requests --json: "api-key create"'s Result
// is the dedicated scriptsdk.APIKeyCreateResult shape, which only the
// --json rendering produces.
func buildAPIKeyCreateArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.APIKeyCreateParams](raw)
	if err != nil {
		return nil, err
	}
	return []string{"--scope", strings.Join(p.Scopes, ","), "--expires", p.Expires, "--show", showPath, "--json"}, nil
}

// buildAPIKeyListArgs always requests --json: "api-key list"'s Result is
// scriptsdk.JSONResult.
func buildAPIKeyListArgs(showPath string, raw json.RawMessage) ([]string, error) {
	if _, err := decodeParams[scriptsdk.APIKeyListParams](raw); err != nil {
		return nil, err
	}
	return []string{"--show", showPath, "--json"}, nil
}

func buildAPIKeyRevokeArgs(showPath string, raw json.RawMessage) ([]string, error) {
	p, err := decodeParams[scriptsdk.APIKeyRevokeParams](raw)
	if err != nil {
		return nil, err
	}
	args := []string{"--id", p.ID, "--show", showPath}
	if p.JSONOutput {
		args = append(args, "--json")
	}
	return args, nil
}

// formatFloat renders f as the shortest round-trippable decimal form
// (strconv.FormatFloat with -1 precision), matching how a script author
// would naturally write a numeric literal (e.g. "1" not "1.000000",
// "0.5" not "5e-01").
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
