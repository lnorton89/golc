// descriptors.go registers every internal/command route this SDK exposes
// to scripts, plus every declared route deliberately excluded with a
// one-line reason (08-03-PLAN.md Task 2, CONTEXT SCRP-02). This file is the
// single source of truth internal/command/scriptsdk_parity_test.go checks
// every command-registry route against -- a route can never be silently
// absent from the SDK: TestEveryDeclaredRouteIsClassified fails the build
// the moment a new route in internal/command is neither exposed here nor
// excludedRoutes-with-reason.
//
// internal/scriptsdk never imports internal/command (see generate.go's
// package comment) -- every Route string below is a plain string literal,
// independently cross-checked against the real command registry only by
// the external parity test in internal/command, which is free to import
// both packages.
//
// Scope assignment (D-06, show.APIKeyScope's three-tier closed set):
//   - "playback": read-only/query routes, plus the "playback" route family
//     itself (bpm set/tap, switch) -- these are the exact live-operator
//     actions Phase 6's constrained operator surface exists to allow
//     without granting full show-authoring capability.
//   - "authoring": every show-content-mutating route (pool/deployment/
//     scene/theme/preset/chase/motion/blend/programmer/operatorsurface/
//     show save/save-as/open/fixture import). "programmer inspect" and the
//     read-only operatorsurface routes ("list"/"show") are pulled up from
//     playback into authoring rather than left at the general read-only
//     default: per this plan's "choose the more restrictive scope" rule,
//     exposing a playback-scoped key's ability to read authoring-domain
//     internal state (touched-attribute buffers, per-surface MIDI
//     mappings) is treated as authoring-sensitive, not merely a query.
//   - "admin": api-key lifecycle, every artnet safety/configure/target/
//     master route (live output topology and emergency controls), and
//     "show export" (a full unredacted document dump).
//
// Coverage discipline: SCRP-02's operative edge -- flagged in this plan's
// SUMMARY per the planner's judgment -- is capability-surface completeness:
// a route silently missing from the SDK is indistinguishable from a route
// deliberately withheld. excludedRoutes-with-reasons plus the external
// TestEveryDeclaredRouteIsClassified parity test is the chosen mitigation.
package scriptsdk

import "github.com/lnorton89/golc/internal/show"

// -----------------------------------------------------------------------
// Shared Params/Result shapes. Every field maps onto a real flag its
// underlying internal/command route already accepts -- no field here
// invents an argument the command itself does not have. A shape is shared
// across descriptors only when the underlying command files already share
// one parse helper for that exact flag set (e.g. internal/command/
// programming.go's parseDomainNameShowArgs/parseDomainRenameArgs).
// -----------------------------------------------------------------------

// ShowOnlyParams is the "--show <path>"-only shape: programmer inspect/
// clear, show save, show diagnose, show export, api-key list.
type ShowOnlyParams struct {
	Show string `json:"show"`
}

// NameShowParams is the "<name> --show <path>"-only shape: deployment
// create/activate, scene activate, theme create, motion create,
// operatorsurface create, and every {domain} delete route (theme/preset/
// chase/motion/scene delete).
type NameShowParams struct {
	Name string `json:"name"`
	Show string `json:"show"`
}

// RenameParams is the "<old-name> <new-name> --show <path>" shape shared by
// every {domain} rename/duplicate route (theme rename, preset rename,
// motion rename, motion duplicate, scene rename, scene duplicate, chase
// duplicate) -- internal/command/programming.go's parseDomainRenameArgs.
type RenameParams struct {
	Name    string `json:"name"`
	NewName string `json:"newName"`
	Show    string `json:"show"`
}

// DeploymentInstanceReassignParams: "deployment instance reassign
// <deployment-name> <instance-id> [--mode <mode>] [--universe <n>]
// [--address <n>] --show <path>". Mode/Universe/Address are all optional
// -- an omitted field keeps the instance's current value.
type DeploymentInstanceReassignParams struct {
	DeploymentName string `json:"deploymentName"`
	InstanceID     string `json:"instanceId"`
	Mode           string `json:"mode,omitempty"`
	Universe       int    `json:"universe,omitempty"`
	Address        int    `json:"address,omitempty"`
	Show           string `json:"show"`
}

// AckResult is the generic textual-acknowledgement result every mutation-
// only route returns: the command's own GOLC_..._CREATED/GOLC_..._RENAMED/
// etc. stdout line.
type AckResult struct {
	Message string `json:"message"`
}

// JSONResult is the generic result for every route whose CLI form already
// prints a JSON (or JSON-shaped review/diff) document to stdout: the raw
// text is round-tripped as one string field, and the caller parses it,
// rather than this registry re-declaring a second, independently
// maintained copy of each command's ad hoc report shape.
type JSONResult struct {
	JSON string `json:"json"`
}

// PoolCreateParams: "pool create <name> [--requires <cap1,cap2,...>] --show <path>".
type PoolCreateParams struct {
	Name     string   `json:"name"`
	Requires []string `json:"requires,omitempty"`
	Show     string   `json:"show"`
}

// PoolUpdateParams: "pool update <pool> [--add ...]... [--remove <id>]...
// [--propagate immediate|preview] [--out <path>] [--json] --show <path>".
type PoolUpdateParams struct {
	Pool       string   `json:"pool"`
	Add        []string `json:"add,omitempty"`
	Remove     []string `json:"remove,omitempty"`
	Propagate  string   `json:"propagate,omitempty"`
	Out        string   `json:"out,omitempty"`
	JSONOutput bool     `json:"json,omitempty"`
	Show       string   `json:"show"`
}

// PoolApplyParams: "pool apply {plan-file} --plan-id <id> --show <path>".
type PoolApplyParams struct {
	PlanFile string `json:"planFile"`
	PlanID   string `json:"planId"`
	Show     string `json:"show"`
}

// PoolSubstituteParams: "pool substitute <pool> --from <fixture-file> --to
// <fixture-file> [--out <path>] [--json] --show <path>".
type PoolSubstituteParams struct {
	Pool       string `json:"pool"`
	From       string `json:"from"`
	To         string `json:"to"`
	Out        string `json:"out,omitempty"`
	JSONOutput bool   `json:"json,omitempty"`
	Show       string `json:"show"`
}

// ShowOpenParams: "show open --show <path> [--accept-recovery <id>]
// [--discard-recovery]".
type ShowOpenParams struct {
	Show            string `json:"show"`
	AcceptRecovery  string `json:"acceptRecovery,omitempty"`
	DiscardRecovery bool   `json:"discardRecovery,omitempty"`
}

// ShowSaveAsParams: "show save-as --show <src> --to <dest>".
type ShowSaveAsParams struct {
	Show string `json:"show"`
	To   string `json:"to"`
}

// SceneCreateParams: "scene create <name> --bars <n> --show <path>".
type SceneCreateParams struct {
	Name string `json:"name"`
	Bars int    `json:"bars"`
	Show string `json:"show"`
}

// SceneLayerSetParams: "scene layer set <scene> --kind base_look|
// color_theme|chase|motion [--ref <id>] [--instance <id>]... [--pool
// <id>]... [--group <id>]... [--fixture <pool_id>|<pool_member_id>]...
// [--disable] --show <path>".
type SceneLayerSetParams struct {
	Scene       string   `json:"scene"`
	Kind        string   `json:"kind"`
	Ref         string   `json:"ref,omitempty"`
	Instances   []string `json:"instances,omitempty"`
	Pools       []string `json:"pools,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	FixtureRefs []string `json:"fixtureRefs,omitempty"`
	Disable     bool     `json:"disable,omitempty"`
	Show        string   `json:"show"`
}

// BlendCreateParams: "blend create <name> --duration-bars <value> [--curve
// linear|ease_in|ease_out] --show <path>".
type BlendCreateParams struct {
	Name         string  `json:"name"`
	DurationBars float64 `json:"durationBars"`
	Curve        string  `json:"curve,omitempty"`
	Show         string  `json:"show"`
}

// PresetRecordParams: "preset record <name> --kind intensity|color|
// position|beam --show <path>".
type PresetRecordParams struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Show string `json:"show"`
}

// ChaseCreateParams: "chase create <name> --unit bar|beat --step-duration
// <value> --show <path>".
type ChaseCreateParams struct {
	Name         string  `json:"name"`
	Unit         string  `json:"unit"`
	StepDuration float64 `json:"stepDuration"`
	Show         string  `json:"show"`
}

// ChaseUpdateParams: "chase update <name> [--name <new-name>] [--unit
// bar|beat] [--step-duration <value>] --show <path>".
type ChaseUpdateParams struct {
	Name         string  `json:"name"`
	NewName      string  `json:"newName,omitempty"`
	Unit         string  `json:"unit,omitempty"`
	StepDuration float64 `json:"stepDuration,omitempty"`
	Show         string  `json:"show"`
}

// ChaseReorderParams: "chase reorder <name> --order <i,j,k,...> --show <path>".
type ChaseReorderParams struct {
	Name  string `json:"name"`
	Order []int  `json:"order"`
	Show  string `json:"show"`
}

// ProgrammerSetParams: "programmer set [--instance <id>]... [--pool
// <id>]... [--group <id>]... [--fixture <pool_id>|<pool_member_id>]...
// --attr <capability>=<value>... --show <path>".
type ProgrammerSetParams struct {
	Instances   []string `json:"instances,omitempty"`
	Pools       []string `json:"pools,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	FixtureRefs []string `json:"fixtureRefs,omitempty"`
	Attrs       []string `json:"attrs"`
	Show        string   `json:"show"`
}

// PlaybackBPMSetParams: "playback bpm set <bpm> --show <path>".
type PlaybackBPMSetParams struct {
	BPM  float64 `json:"bpm"`
	Show string  `json:"show"`
}

// PlaybackBPMTapParams: "playback bpm tap --at <RFC3339-timestamp>... --show <path>".
type PlaybackBPMTapParams struct {
	At   []string `json:"at"`
	Show string   `json:"show"`
}

// PlaybackSwitchParams: "playback switch <scene> --show <path>".
type PlaybackSwitchParams struct {
	Scene string `json:"scene"`
	Show  string `json:"show"`
}

// OperatorSurfaceSurfaceParams is the "--surface <name> --show <path>"-only
// shape shared by "operatorsurface show" and "operatorsurface remove".
type OperatorSurfaceSurfaceParams struct {
	Surface string `json:"surface"`
	Show    string `json:"show"`
}

// OperatorSurfaceAssignParams: "operatorsurface assign|unassign --surface
// <name> [--scene <scene>|--layer <scene>:<kind>|--master grand|--master
// group:<group>|--safety <blackout|stop_release_all|revoke_automation>]
// --show <path>" -- shared by "operatorsurface assign" and "operatorsurface
// unassign", which take the identical selector shape.
type OperatorSurfaceAssignParams struct {
	Surface string `json:"surface"`
	Scene   string `json:"scene,omitempty"`
	Layer   string `json:"layer,omitempty"`
	Master  string `json:"master,omitempty"`
	Safety  string `json:"safety,omitempty"`
	Show    string `json:"show"`
}

// ArtnetStatusParams: "artnet status [--watch] [--json] [--pipe <name>]".
type ArtnetStatusParams struct {
	Watch      bool   `json:"watch,omitempty"`
	JSONOutput bool   `json:"json,omitempty"`
	Pipe       string `json:"pipe,omitempty"`
}

// ArtnetInterfaceListParams: "artnet interface list [--json] [--pipe <name>]".
type ArtnetInterfaceListParams struct {
	JSONOutput bool   `json:"json,omitempty"`
	Pipe       string `json:"pipe,omitempty"`
}

// ArtnetDiscoverParams: "artnet discover --interface <index> [--window
// <duration>] [--json]".
type ArtnetDiscoverParams struct {
	Interface  int    `json:"interface"`
	Window     string `json:"window,omitempty"`
	JSONOutput bool   `json:"json,omitempty"`
}

// ArtnetConfigureParams: "artnet configure --universe <n> --ip <address>
// [--port <port>] [--enabled true|false] [--pipe <name>]".
type ArtnetConfigureParams struct {
	Universe int    `json:"universe"`
	IP       string `json:"ip"`
	Port     int    `json:"port,omitempty"`
	Enabled  bool   `json:"enabled,omitempty"`
	Pipe     string `json:"pipe,omitempty"`
}

// ArtnetTargetParams: "artnet target enable|disable --universe <n> --ip
// <address> [--port <port>] [--pipe <name>]" -- shared by "artnet target
// enable" and "artnet target disable".
type ArtnetTargetParams struct {
	Universe int    `json:"universe"`
	IP       string `json:"ip"`
	Port     int    `json:"port,omitempty"`
	Pipe     string `json:"pipe,omitempty"`
}

// ArtnetSafetyParams: "artnet safety blackout|stop-all|revoke-automation
// [--on true|false] [--pipe <name>]" -- shared by all three safety routes.
type ArtnetSafetyParams struct {
	On   bool   `json:"on,omitempty"`
	Pipe string `json:"pipe,omitempty"`
}

// ArtnetMasterSetParams: "artnet master set --grand <0..1> | --group <id>
// --level <0..1> [--pipe <name>]".
type ArtnetMasterSetParams struct {
	Grand float64 `json:"grand,omitempty"`
	Group string  `json:"group,omitempty"`
	Level float64 `json:"level,omitempty"`
	Pipe  string  `json:"pipe,omitempty"`
}

// FixtureFileParams is the "<file>"-only shape shared by "fixture validate"
// and "fixture inspect".
type FixtureFileParams struct {
	File string `json:"file"`
}

// FixtureImportParams: "fixture import --ofl <manufacturer>/<key> [--mirror
// <url>] [--allow-mirror] --out <path> | fixture import --ofl-file <path>
// --out <path>".
type FixtureImportParams struct {
	OFL         string `json:"ofl,omitempty"`
	OFLFile     string `json:"oflFile,omitempty"`
	Mirror      string `json:"mirror,omitempty"`
	AllowMirror bool   `json:"allowMirror,omitempty"`
	Out         string `json:"out"`
}

// APIKeyCreateParams: "api-key create --scope <playback|authoring|admin>
// [,...] --expires <duration> --show <path> [--json]".
type APIKeyCreateParams struct {
	Scopes     []string `json:"scopes"`
	Expires    string   `json:"expires"`
	Show       string   `json:"show"`
	JSONOutput bool     `json:"json,omitempty"`
}

// APIKeyCreateResult is "api-key create"'s own result shape (never reused
// elsewhere): the one route whose RawToken is shown exactly once at mint
// time and is never persisted or logged again beyond this single response.
type APIKeyCreateResult struct {
	ID        string   `json:"id"`
	Prefix    string   `json:"prefix"`
	RawToken  string   `json:"rawToken"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expiresAt"`
}

// APIKeyListParams: "api-key list --show <path> [--json]".
type APIKeyListParams struct {
	Show       string `json:"show"`
	JSONOutput bool   `json:"json,omitempty"`
}

// APIKeyRevokeParams: "api-key revoke --id <id> --show <path> [--json]".
type APIKeyRevokeParams struct {
	ID         string `json:"id"`
	Show       string `json:"show"`
	JSONOutput bool   `json:"json,omitempty"`
}

// -----------------------------------------------------------------------
// sdkEntry is one table row this file's init loop below turns into a real
// MustRegisterSDKMethod call. A table (rather than 62 individually spelled
// var _ = MustRegisterSDKMethod(...) statements) keeps this file's
// enormous, mechanically-derived route classification reviewable as one
// flat list while still self-registering from a single package-level var
// initializer -- registerSDKMethodTable runs exactly once, at package
// init, before any test or generator call observes the registry.
// -----------------------------------------------------------------------

type sdkEntry struct {
	Route   string
	Method  string
	Summary string
	Scope   show.APIKeyScope
	Params  any
	Result  any
}

var sdkMethodTable = []sdkEntry{
	// pool (authoring)
	{"pool create", "pool.create", "Create a named logical fixture pool.", show.APIKeyScopeAuthoring, PoolCreateParams{}, AckResult{}},
	{"pool update", "pool.update", "Compute and write/print a deterministic pool impact-review plan.", show.APIKeyScopeAuthoring, PoolUpdateParams{}, JSONResult{}},
	{"pool apply", "pool.apply", "Validate and atomically apply an already-reviewed pool impact plan.", show.APIKeyScopeAuthoring, PoolApplyParams{}, AckResult{}},
	{"pool substitute", "pool.substitute", "Compute and write/print a deterministic fixture-substitution capability-diff review.", show.APIKeyScopeAuthoring, PoolSubstituteParams{}, JSONResult{}},
	{"pool rename", "pool.rename", "Rename a pool, preserving its identity.", show.APIKeyScopeAuthoring, RenameParams{}, AckResult{}},
	// Method is "remove", not "delete": see the "delete is a reserved
	// TypeScript keyword" note by "scene delete" below.
	{"pool delete", "pool.remove", "Delete a pool by name, cascading to its own deployment instances and group member refs.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},

	// deployment (authoring)
	{"deployment create", "deployment.create", "Create a named deployment.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},
	{"deployment activate", "deployment.activate", "Mark exactly one deployment active.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},
	{"deployment rename", "deployment.rename", "Rename a deployment, preserving its identity.", show.APIKeyScopeAuthoring, RenameParams{}, AckResult{}},
	{"deployment delete", "deployment.remove", "Delete a deployment by name; its own instances go with it.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},
	{"deployment instance reassign", "deployment.instance.reassign", "In-place reassign one deployment instance's mode/universe/address.", show.APIKeyScopeAuthoring, DeploymentInstanceReassignParams{}, AckResult{}},

	// show (mixed: inspect/diagnose are playback queries, open/save/save-as
	// are authoring mutations, export is admin -- see package doc comment)
	{"show inspect", "show.inspect", "Print a deterministic JSON summary of a ShowState document's pools and deployments.", show.APIKeyScopePlayback, ShowOnlyParams{}, JSONResult{}},
	{"show open", "show.open", "Open a ShowState document for edit, offering any interrupted-session recovery point found.", show.APIKeyScopeAuthoring, ShowOpenParams{}, AckResult{}},
	{"show save", "show.save", "Load and re-save a ShowState document, writing a fresh recovery point.", show.APIKeyScopeAuthoring, ShowOnlyParams{}, AckResult{}},
	{"show save-as", "show.saveAs", "Load a ShowState document read-only and save it to a new path.", show.APIKeyScopeAuthoring, ShowSaveAsParams{}, AckResult{}},
	{"show diagnose", "show.diagnose", "Run combined file-level and structural diagnostics on a .golc file.", show.APIKeyScopePlayback, ShowOnlyParams{}, JSONResult{}},
	// Method is "exportDocument", not "export": "export" is a reserved
	// TypeScript keyword and cannot be used as a `function export(...)`
	// ambient declaration identifier.
	{"show export", "show.exportDocument", "Print the full canonical, round-trippable JSON document for a .golc file.", show.APIKeyScopeAdmin, ShowOnlyParams{}, JSONResult{}},

	// scene (authoring)
	{"scene create", "scene.create", "Create a named bar-loop scene.", show.APIKeyScopeAuthoring, SceneCreateParams{}, AckResult{}},
	{"scene activate", "scene.activate", "Mark exactly one scene active, deactivating every other scene.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},
	{"scene layer set", "scene.layer.set", "Enable/point one of a scene's four fixed layers.", show.APIKeyScopeAuthoring, SceneLayerSetParams{}, AckResult{}},
	{"scene rename", "scene.rename", "Rename a scene, preserving its identity.", show.APIKeyScopeAuthoring, RenameParams{}, AckResult{}},
	{"scene duplicate", "scene.duplicate", "Duplicate a scene under a fresh, inactive identity.", show.APIKeyScopeAuthoring, RenameParams{}, AckResult{}},
	// Method is "remove", not "delete": "delete" is a reserved TypeScript
	// keyword and cannot be used as a `function delete(...)` ambient
	// declaration identifier. Every other {domain} delete route below
	// mints the same "remove" leaf name for the identical reason.
	{"scene delete", "scene.remove", "Delete a scene by name.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},

	// blend (authoring)
	{"blend create", "blend.create", "Create a named blend preset.", show.APIKeyScopeAuthoring, BlendCreateParams{}, AckResult{}},

	// theme (authoring)
	{"theme create", "theme.create", "Create a named color theme.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},
	{"theme rename", "theme.rename", "Rename a color theme, preserving its identity.", show.APIKeyScopeAuthoring, RenameParams{}, AckResult{}},
	{"theme delete", "theme.remove", "Delete a color theme by name.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},

	// preset (authoring)
	{"preset record", "preset.record", "Record a kind-scoped preset from the persisted Programmer buffer.", show.APIKeyScopeAuthoring, PresetRecordParams{}, AckResult{}},
	{"preset rename", "preset.rename", "Rename a preset, preserving its identity.", show.APIKeyScopeAuthoring, RenameParams{}, AckResult{}},
	{"preset delete", "preset.remove", "Delete a preset by name.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},

	// chase (authoring)
	{"chase create", "chase.create", "Create a named chase.", show.APIKeyScopeAuthoring, ChaseCreateParams{}, AckResult{}},
	{"chase update", "chase.update", "Update a chase's name/step-unit/step-duration.", show.APIKeyScopeAuthoring, ChaseUpdateParams{}, AckResult{}},
	{"chase reorder", "chase.reorder", "Permute a chase's steps deterministically.", show.APIKeyScopeAuthoring, ChaseReorderParams{}, AckResult{}},
	{"chase duplicate", "chase.duplicate", "Duplicate a chase under a fresh identity.", show.APIKeyScopeAuthoring, RenameParams{}, AckResult{}},
	{"chase delete", "chase.remove", "Delete a chase by name.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},

	// motion (authoring)
	{"motion create", "motion.create", "Create a named motion preset.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},
	{"motion rename", "motion.rename", "Rename a motion preset, preserving its identity.", show.APIKeyScopeAuthoring, RenameParams{}, AckResult{}},
	{"motion duplicate", "motion.duplicate", "Duplicate a motion preset under a fresh identity.", show.APIKeyScopeAuthoring, RenameParams{}, AckResult{}},
	{"motion delete", "motion.remove", "Delete a motion preset by name.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},

	// programmer (authoring -- see package doc comment: inspect is pulled
	// up from the general read-only default per the more-restrictive rule)
	{"programmer set", "programmer.set", "Resolve a selection and set semantic attribute values on it.", show.APIKeyScopeAuthoring, ProgrammerSetParams{}, AckResult{}},
	{"programmer inspect", "programmer.inspect", "Print every touched attribute in the Programmer buffer.", show.APIKeyScopeAuthoring, ShowOnlyParams{}, JSONResult{}},
	{"programmer clear", "programmer.clear", "Empty the Programmer buffer's touched-attribute set.", show.APIKeyScopeAuthoring, ShowOnlyParams{}, AckResult{}},

	// playback (playback scope: the constrained-operator-surface family)
	{"playback bpm set", "playback.bpm.set", "Set the show-wide global BPM to an explicit numeric value.", show.APIKeyScopePlayback, PlaybackBPMSetParams{}, AckResult{}},
	{"playback bpm tap", "playback.bpm.tap", "Set the show-wide global BPM from ordered tap timestamps.", show.APIKeyScopePlayback, PlaybackBPMTapParams{}, AckResult{}},
	// Method is "switchScene", not "switch": "switch" is a reserved
	// TypeScript keyword and cannot be used as a `function switch(...)`
	// ambient declaration identifier.
	{"playback switch", "playback.switchScene", "Stage an active-scene switch.", show.APIKeyScopePlayback, PlaybackSwitchParams{}, AckResult{}},

	// operatorsurface (authoring -- see package doc comment)
	{"operatorsurface create", "operatorsurface.create", "Create a named operator surface.", show.APIKeyScopeAuthoring, NameShowParams{}, AckResult{}},
	{"operatorsurface list", "operatorsurface.list", "List every operator surface on a ShowState document.", show.APIKeyScopeAuthoring, ShowOnlyParams{}, JSONResult{}},
	{"operatorsurface show", "operatorsurface.show", "Print a named operator surface's assigned items and MIDI mappings.", show.APIKeyScopeAuthoring, OperatorSurfaceSurfaceParams{}, JSONResult{}},
	{"operatorsurface assign", "operatorsurface.assign", "Assign one individual control to a named operator surface.", show.APIKeyScopeAuthoring, OperatorSurfaceAssignParams{}, AckResult{}},
	{"operatorsurface unassign", "operatorsurface.unassign", "Unassign one individual control from a named operator surface.", show.APIKeyScopeAuthoring, OperatorSurfaceAssignParams{}, AckResult{}},
	{"operatorsurface remove", "operatorsurface.remove", "Delete a named operator surface and all its assignments and MIDI mappings.", show.APIKeyScopeAuthoring, OperatorSurfaceSurfaceParams{}, AckResult{}},

	// artnet: status/interface-list/discover are playback queries; every
	// configure/target/master/safety route is admin (live output topology
	// and emergency controls).
	{"artnet status", "artnet.status", "Inspect per-universe/target Art-Net health as a snapshot or watch view.", show.APIKeyScopePlayback, ArtnetStatusParams{}, JSONResult{}},
	// Method namespace segment is "interfaces", not "interface": "interface"
	// is a reserved TypeScript keyword and cannot be used as a
	// `namespace interface { ... }` declaration identifier.
	{"artnet interface list", "artnet.interfaces.list", "List candidate Windows network interfaces for Art-Net output.", show.APIKeyScopePlayback, ArtnetInterfaceListParams{}, JSONResult{}},
	{"artnet discover", "artnet.discover", "Scan a pinned interface for compatible Art-Net nodes; suggestions only.", show.APIKeyScopePlayback, ArtnetDiscoverParams{}, JSONResult{}},
	{"artnet configure", "artnet.configure", "Add or update one unicast Art-Net output target for a universe.", show.APIKeyScopeAdmin, ArtnetConfigureParams{}, AckResult{}},
	{"artnet target enable", "artnet.target.enable", "Re-enable output to one configured unicast target without stopping the rig.", show.APIKeyScopeAdmin, ArtnetTargetParams{}, AckResult{}},
	{"artnet target disable", "artnet.target.disable", "Take one configured unicast target offline without stopping the rig.", show.APIKeyScopeAdmin, ArtnetTargetParams{}, AckResult{}},
	{"artnet master set", "artnet.master.set", "Set the grand master or one group's master level.", show.APIKeyScopeAdmin, ArtnetMasterSetParams{}, AckResult{}},
	{"artnet safety blackout", "artnet.safety.blackout", "Drive every configured universe's output to zero on the next Art-Net tick.", show.APIKeyScopeAdmin, ArtnetSafetyParams{}, AckResult{}},
	{"artnet safety stop-all", "artnet.safety.stopAll", "Drive every configured universe's output to the safe/zero state.", show.APIKeyScopeAdmin, ArtnetSafetyParams{}, AckResult{}},
	{"artnet safety revoke-automation", "artnet.safety.revokeAutomation", "Block any command carrying a non-manual source tag and freeze the current look.", show.APIKeyScopeAdmin, ArtnetSafetyParams{}, AckResult{}},

	// fixture: validate/inspect are read-only file-level queries with no
	// --show at all (playback); import is a show-content mutation
	// (authoring, explicit per plan).
	{"fixture validate", "fixture.validate", "Strictly decode and validate a hand-authored YAML fixture definition.", show.APIKeyScopePlayback, FixtureFileParams{}, AckResult{}},
	{"fixture inspect", "fixture.inspect", "Decode a fixture definition and print its content-addressed identity and provenance.", show.APIKeyScopePlayback, FixtureFileParams{}, JSONResult{}},
	// Method is "importDefinition", not "import": "import" is a reserved
	// TypeScript keyword and cannot be used as a `function import(...)`
	// ambient declaration identifier.
	{"fixture import", "fixture.importDefinition", "Import an Open Fixture Library definition through GOLC's canonical pipeline.", show.APIKeyScopeAuthoring, FixtureImportParams{}, AckResult{}},

	// api-key (admin, explicit per plan)
	{"api-key create", "apiKey.create", "Mint a new scoped, expiring API key.", show.APIKeyScopeAdmin, APIKeyCreateParams{}, APIKeyCreateResult{}},
	{"api-key list", "apiKey.list", "List every API key's metadata.", show.APIKeyScopeAdmin, APIKeyListParams{}, JSONResult{}},
	{"api-key revoke", "apiKey.revoke", "Revoke one API key by id.", show.APIKeyScopeAdmin, APIKeyRevokeParams{}, AckResult{}},
}

// excludedRouteTable names every route currently declared in
// internal/command that this SDK deliberately does not expose, each with
// its one-line reason (populated verbatim into excludedRoutes below).
var excludedRouteTable = map[string]string{
	"playback evaluate":  "frame evaluation is not a script-reachable capability (ROADMAP Phase 8 success criterion 2)",
	"artnet serve":       "daemon lifecycle, owned by the application not a script",
	"build":              "contributor build tooling, not a show-domain capability",
	"test":               "contributor build tooling, not a show-domain capability",
	"check":              "contributor build tooling, not a show-domain capability",
	"generate":           "contributor build tooling, not a show-domain capability",
	"package":            "contributor build tooling, not a show-domain capability",
	"run":                "contributor build tooling, not a show-domain capability",
	"dev":                "contributor build tooling, not a show-domain capability",
	"docs":               "contributor build tooling, not a show-domain capability",
	"lint":               "contributor build tooling, not a show-domain capability",
	"tools update":       "contributor build tooling, not a show-domain capability",
	"config inspect":     "repository configuration is a contributor concern outside the show domain",
	"config set":         "repository configuration is a contributor concern outside the show domain",
	"config explain":     "repository configuration is a contributor concern outside the show domain",
	"linear catalog":     "planning-traceability tooling, not a show-domain capability",
	"linear preview":     "planning-traceability tooling, not a show-domain capability",
	"linear apply":       "planning-traceability tooling, not a show-domain capability",
	"linear drift":       "planning-traceability tooling, not a show-domain capability",
	"linear status":      "planning-traceability tooling, not a show-domain capability",
	"linear archive":     "planning-traceability tooling, not a show-domain capability",
	"linear unlink":      "planning-traceability tooling, not a show-domain capability",
	"linear map migrate": "planning-traceability tooling, not a show-domain capability",
	"linear validate":    "planning-traceability tooling, not a show-domain capability",

	// script lifecycle (CLI/GUI authoring surface, 08-01): a running script
	// must not be able to create/list/inspect/edit/delete/reconfigure other
	// Script entities through its own SDK -- self-modification of the
	// sandbox's own catalog is an application/CLI concern, not a
	// show-domain capability exposed to script code (same class as
	// "artnet serve": daemon/catalog lifecycle, owned by the application).
	"script create":      "script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script",
	"script list":        "script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script",
	"script show":        "script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script",
	"script edit":        "script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script",
	"script delete":      "script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script",
	"script profile set": "script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script",

	// script lifecycle control (08-05/08-06/08-07): a running script must
	// not be able to launch another script, terminate itself or another
	// run, or validate/introspect another script's source, through the
	// SDK -- same class as "artnet serve" and the script CRUD routes above.
	"script run":      "script lifecycle control; a script must not be able to launch another script",
	"script stop":     "script lifecycle control; a script must not be able to terminate itself or another run through the SDK",
	"script validate": "script lifecycle control; a script must not validate or introspect other scripts through the SDK",

	// script debugger control (08-09): a running script must not be able
	// to launch, debug, or step another script's debug session through
	// the SDK -- same class as the script lifecycle control routes above.
	"script debug":     "script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK",
	"script continue":  "script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK",
	"script step-over": "script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK",
	"script step-into": "script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK",
	"script step-out":  "script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK",
}

func init() {
	for _, entry := range sdkMethodTable {
		MustRegisterSDKMethod(SDKMethodDescriptor{
			Route:   entry.Route,
			Method:  entry.Method,
			Summary: entry.Summary,
			Scope:   entry.Scope,
			Params:  entry.Params,
			Result:  entry.Result,
		})
	}
	for route, reason := range excludedRouteTable {
		MustRegisterExclusion(route, reason)
	}
}
