# Phase 3: Deterministic Show Programming and Playback - Pattern Map

**Mapped:** 2026-07-21
**Files analyzed:** 16 (new domain/engine files) + 1 modified (`internal/show/state.go`)
**Analogs found:** 17 / 17 (all via role-match — this is greenfield domain territory, no exact prior analog exists for the real-time engine itself)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/programming/selection.go` | service (resolver) | request-response | `internal/pool/model.go` (`ValidateGroupReferences`, ref resolution) | role-match |
| `internal/programming/programmer.go` | model + service | CRUD (in-memory) | `internal/pool/model.go` (`Pool`/mutation funcs) | role-match |
| `internal/programming/theme.go` | model | CRUD | `internal/pool/model.go` (`Pool` type + `NewPool`/`Rename`) | exact (shape) |
| `internal/programming/preset.go` | model | CRUD | `internal/pool/model.go` (`Pool` type + `NewPool`/`Rename`) | exact (shape) |
| `internal/programming/chase.go` | model | CRUD + tempo-relative transform | `internal/deployment/model.go` (`Deployment`/`Instance` nested-collection shape) | role-match |
| `internal/programming/motion.go` | model | CRUD + tempo-relative transform | `internal/deployment/model.go` | role-match |
| `internal/programming/history.go` | service (stack) | event-driven (session-only) | none in-repo — no prior undo/history package | no analog |
| `internal/scene/scene.go` | model | CRUD | `internal/pool/model.go` (`Group` — named composite of refs) | role-match |
| `internal/scene/layer.go` | model + service | transform (pure reduce) | `internal/pool/impact.go` (`BuildImpactPlan` — deterministic pure transform over State) | role-match |
| `internal/scene/blend.go` | model | CRUD | `internal/pool/model.go` (`Pool` type) | role-match |
| `internal/playback/clock.go` | service (pure function) | transform (pure) | none in-repo — no prior pure-time-function package; closest shape is `internal/pool/impact.go`'s pure `BuildImpactPlan` (deterministic function of inputs, no I/O) | role-match |
| `internal/playback/compile.go` | service (compiler) | transform (validate + flatten) | `internal/pool/impact.go` (`BuildImpactPlan`) + `internal/show/state.go` (`validate`) | role-match |
| `internal/playback/evaluate.go` | service (pure function) | transform (pure) | `internal/pool/impact.go` (`BuildImpactPlan`) | role-match |
| `internal/playback/engine.go` | service (tick loop, stateful) | event-driven (real-time) | none in-repo — first stateful/concurrent engine in this codebase | no analog |
| `internal/playback/frame.go` | model | transform (output snapshot) | `internal/pool/impact.go` (`ImpactPlan` — deterministic output document) | role-match |
| `internal/command/programming.go` (new, PROG-01..07 CLI routes) | controller (CLI command) | request-response | `internal/command/pool.go` (`runPoolCreate`, arg parsing helpers) | exact |
| `internal/show/state.go` (modified: add `Themes`/`Presets`/`Chases`/`MotionPresets`/`Scenes`/`BlendPresets` fields + validate() extension) | model + service (persistence) | CRUD | itself (extend existing `State`, `Load`, `Save`, `validate`) | exact |

## Pattern Assignments

### `internal/programming/theme.go`, `preset.go` (model, CRUD)

**Analog:** `internal/pool/model.go`

**Imports pattern** (lines 8-17):
```go
package pool

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
)
```
Apply directly to `internal/programming`: import `github.com/google/uuid` for identity, `internal/fixture` where semantic capability types/ranges are referenced (PROG-02/04 normalized attribute values).

**Identity + construction pattern** (lines 25-30, 79-94):
```go
type Pool struct {
	ID                   uuid.UUID               `json:"id"`
	Name                 string                  `json:"name"`
	RequiredCapabilities []fixture.CapabilityType `json:"required_capabilities,omitempty"`
	Members              []PoolMember            `json:"members,omitempty"`
}

// NewPool mints a fresh UUIDv7-identified Pool with zero members. IDs are
// minted only at creation time -- never derived from Name, and
// never re-minted by Rename.
func NewPool(name string, requiredCapabilities []fixture.CapabilityType) (Pool, error) {
	if strings.TrimSpace(name) == "" {
		return Pool{}, fmt.Errorf("GOLC_POOL_NAME_EMPTY: pool name must not be empty")
	}
	if err := validateRequiredCapabilities(name, requiredCapabilities); err != nil {
		return Pool{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Pool{}, fmt.Errorf("GOLC_POOL_ID_MINT_FAILED: %v", err)
	}
	return Pool{ID: id, Name: name, RequiredCapabilities: requiredCapabilities}, nil
}
```
Every new Theme/Preset/Chase/MotionPreset/Scene/BlendPreset type should copy this exact shape: `uuid.NewV7()`-minted `ID` field, name-emptiness validation with a `GOLC_{DOMAIN}_NAME_EMPTY` diagnostic, a `New{Type}` constructor that never derives ID from Name, and a `Rename` function (lines 96-104) that mutates only `Name`, never `ID`.

**Rename pattern** (lines 96-104):
```go
func Rename(p Pool, newName string) (Pool, error) {
	if strings.TrimSpace(newName) == "" {
		return Pool{}, fmt.Errorf("GOLC_POOL_NAME_EMPTY: pool name must not be empty")
	}
	p.Name = newName
	return p, nil
}
```

**Duplicate-name validation pattern** (lines 130-142):
```go
func ValidateUniqueNames(pools []Pool) error {
	seen := make(map[string]bool, len(pools))
	for _, p := range pools {
		if seen[p.Name] {
			return fmt.Errorf("GOLC_POOL_DUPLICATE_NAME: a pool named %q already exists", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}
```
Apply identically for each new object type's unique-name check (`GOLC_THEME_DUPLICATE_NAME`, `GOLC_CHASE_DUPLICATE_NAME`, etc.) — these get called from `show.validate()`.

---

### `internal/programming/selection.go` (service, request-response)

**Analog:** `internal/pool/model.go`'s `ValidateGroupReferences` (lines 163-189)

**Reference-resolution pattern**:
```go
func ValidateGroupReferences(pools []Pool, groups []Group) error {
	membersByPool := make(map[uuid.UUID]map[uuid.UUID]bool, len(pools))
	for _, p := range pools {
		members := make(map[uuid.UUID]bool, len(p.Members))
		for _, m := range p.Members {
			members[m.ID] = true
		}
		membersByPool[p.ID] = members
	}

	for _, g := range groups {
		for _, ref := range g.MemberRefs {
			members, poolExists := membersByPool[ref.PoolID]
			if !poolExists {
				return fmt.Errorf(
					"GOLC_GROUP_DANGLING_REFERENCE: group %q references pool %s, which does not exist",
					g.Name, ref.PoolID)
			}
			if !members[ref.PoolMemberID] {
				return fmt.Errorf(
					"GOLC_GROUP_DANGLING_REFERENCE: group %q references pool member %s in pool %s, which does not exist",
					g.Name, ref.PoolMemberID, ref.PoolID)
			}
		}
	}
	return nil
}
```
PROG-01's selection resolver (pool/group/deployment-instance/direct-fixture) should copy this build-a-lookup-set-then-check-membership shape, and reuse the `GOLC_{DOMAIN}_DANGLING_REFERENCE` diagnostic pattern for a selection referencing a pool/group/deployment instance that no longer exists.

---

### `internal/scene/layer.go`, `internal/playback/compile.go`, `internal/playback/evaluate.go` (pure transform / compiler)

**Analog:** `internal/pool/impact.go`'s `BuildImpactPlan` — the existing "deterministic pure function of State inputs, produces a reviewable document, mutates nothing" pattern this repo already established for POOL-03's plan/apply split. Read `internal/pool/impact.go` directly during planning for the exact signature/error-aggregation shape (not excerpted here — already covered structurally by `internal/command/pool.go`'s `runPoolUpdate`, which calls `pool.BuildImpactPlan(state.Pools, state.Deployments, state.Groups, state.Revision, req)` and treats the result as an immutable plan object). This is the strongest available in-repo analog for `Compile(show.State) -> CompiledPlan` (D-05/D-06's whole-plan, all-or-nothing compile) and for `Evaluate(plan, position) -> Frame` (Pattern 2 in RESEARCH.md) — both are pure functions producing a new document from existing state, never mutating inputs, exactly like `BuildImpactPlan`.

**Compile reject-on-invalid pattern** (RESEARCH.md Code Examples, derived directly from D-05/D-06 — no existing in-repo function does this yet, but the shape matches `runPoolApply`'s "validate integrity then freshness before touching state" sequencing in `internal/command/pool.go` lines 436-486):
```go
func (e *Engine) StageEdit(state show.State) error {
	plan, err := Compile(state) // whole-plan compile; validates every reference
	if err != nil {
		// D-06: reject and keep running the last valid compiled version.
		return fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: %w", err)
	}
	e.pendingPlan.Store(&plan) // adopted atomically at the next bar boundary by tick()
	return nil
}
```

---

### `internal/show/state.go` (modified — add new fields + extend validate())

**Analog:** itself, current shape (lines 35-41, 92-117, 125-154)

**Field-addition pattern**:
```go
type State struct {
	SchemaVersion int                     `json:"schema_version"`
	Revision      int                     `json:"revision"`
	Pools         []pool.Pool             `json:"pools"`
	Deployments   []deployment.Deployment `json:"deployments"`
	Groups        []pool.Group            `json:"groups"`
}
```
Add `Themes []programming.Theme`, `Presets []programming.Preset`, `Chases []programming.Chase`, `MotionPresets []programming.MotionPreset`, `Scenes []scene.Scene`, `BlendPresets []scene.BlendPreset` as new `json` fields following the exact same tag/naming convention (snake_case plural, `omitempty` only where the existing `Groups`/`Pools` fields don't use it — note `Pools`/`Deployments`/`Groups` do NOT use `omitempty`, so new top-level State fields should match that, unlike nested `Members`/`Instances` which do use `omitempty`).

**validate() extension pattern** (lines 125-154):
```go
func validate(s State) error {
	for _, p := range s.Pools {
		if err := pool.Validate(p); err != nil {
			return err
		}
	}
	if err := pool.ValidateUniqueNames(s.Pools); err != nil {
		return err
	}
	// ... existing checks ...
	if err := pool.ValidateGroupReferences(s.Pools, s.Groups); err != nil {
		return err
	}
	return nil
}
```
Extend this exact function (not a parallel validation path) with per-object `Validate`, `ValidateUniqueNames`, and cross-reference checks (scene→layer→theme/preset/chase/motion-preset resolution) for every new field, matching the existing per-type validate-then-uniqueness-then-references sequencing.

**Load/Save unchanged pattern** (lines 65-117): `Load`/`Save`'s strict-decode, revision-bump, write-temp-then-rename shape needs no structural change — only `validate(s)`'s body grows. Keep the `GOLC_SHOW_STATE_INVALID` wrapping diagnostic exactly as-is.

---

### `internal/command/programming.go` (controller, request-response — new CLI routes for PROG-01..07/SCEN-01..09)

**Analog:** `internal/command/pool.go`

**Scope + route self-registration pattern** (lines 36-71):
```go
var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "pool",
	Summary: "Logical fixture pool definitions, independent of concrete count/address/hardware.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "pool create",
	Summary: "Create a named logical pool against a ShowState document: pool create <name> [--requires <cap1,cap2,...>] --show <path>.",
	Handler: runPoolCreate,
})
```
New scopes (`programming`/`scene`/`playback`, or finer-grained `theme`/`preset`/`chase`/`motion`/`scene`) should self-register the same way, one `var _ = MustDeclareScope(...)` per owning file plus one `var _ = MustDeclareRoute(...)` per verb.

**Handler load-mutate-save pattern** (lines 78-99):
```go
func runPoolCreate(request Request) Result {
	name, showPath, requires, err := parsePoolCreateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	newPool, err := pool.NewPool(name, requires)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Pools = append(state.Pools, newPool)
	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_POOL_CREATED: %s (%s)\n", newPool.Name, newPool.ID))}
}
```
Every PROG-04/05/06 "create theme/preset/chase/motion-preset" route and PROG-07's record/update/rename/reorder/duplicate/delete routes should copy this exact `parse args (exit 2 on usage error) -> show.Load (exit 1) -> mutate in-memory -> show.Save (exit 1) -> Stdout success line` shape, with a `GOLC_{DOMAIN}_{VERB}` success line.

**Arg-parsing pattern** (lines 105-141, 210-296): flag loop supporting both `--flag value` and `--flag=value` forms, `GOLC_{SCOPE}_USAGE` diagnostic on any unsupported argument, required `--show` checked last. Copy directly for every new command's arg parser.

---

## Shared Patterns

### UUIDv7 Identity (never derived from Name)
**Source:** `internal/pool/model.go` `NewPool`/`internal/deployment/model.go` `NewDeployment`
**Apply to:** every new `internal/programming`/`internal/scene` object constructor (Theme, Preset, Chase, MotionPreset, Scene, BlendPreset, ProgrammerSession if identified)
```go
id, err := uuid.NewV7()
if err != nil {
	return Pool{}, fmt.Errorf("GOLC_POOL_ID_MINT_FAILED: %v", err)
}
```

### Diagnostic Code Convention
**Source:** repo-wide, e.g. `GOLC_SHOW_STATE_INVALID`, `GOLC_POOL_DUPLICATE_NAME`, `GOLC_GROUP_DANGLING_REFERENCE`
**Apply to:** all new files — every error uses `fmt.Errorf("GOLC_{DOMAIN}_{CONDITION}: %v", ...)`. New Phase 3 codes should follow domain names already implied by RESEARCH.md: `GOLC_PLAYBACK_PLAN_INVALID` (D-06), `GOLC_THEME_*`, `GOLC_PRESET_*`, `GOLC_CHASE_*`, `GOLC_MOTION_PRESET_*`, `GOLC_SCENE_*`, `GOLC_BLEND_PRESET_*`, `GOLC_PROGRAMMER_*`, `GOLC_HISTORY_*`.

### Whole-Document Validation Before Trust
**Source:** `internal/show/state.go` `validate()` (lines 125-154), `internal/pool/model.go` `ValidateGroupReferences`
**Apply to:** `internal/show/state.go`'s extended `validate()` — every new object type gets a per-type `Validate`, a `ValidateUniqueNames`, and (where relevant) a `ValidateXReferences` function called from the single `validate()` entry point. Never introduce a second validation path.

### Atomic Persistence (write-temp-then-rename) + Revision Bump
**Source:** `internal/show/state.go` `Save` (lines 92-117)
```go
tmp := resolved + ".tmp"
if err := os.WriteFile(tmp, payload, 0o644); err != nil { ... }
if err := os.Rename(tmp, resolved); err != nil {
	os.Remove(tmp)
	return fmt.Errorf(...)
}
```
**Apply to:** unchanged — Phase 3's new object types persist through the existing `show.Save`, no new persistence path needed. Only `internal/playback`'s live in-memory engine state (loop epoch, pending/active plan) is explicitly exempted from persistence per RESEARCH.md Pitfall 3 — never serialize `time.Time` epochs into `show.State`.

### Strict Decode / Canonical Encode
**Source:** `internal/strictjson` used throughout `internal/show/state.go`, `internal/command/pool.go` (`writeImpactPlan`)
**Apply to:** any new file that reads/writes a plan-like document (e.g. a future `internal/playback` plan export, if PROG-04..07 gain a preview/apply split like `pool update`/`pool apply`). Not needed inside the real-time `internal/playback/engine.go` tick path itself (no I/O there per SCEN-09 isolation).

### Deterministic Plan/Apply Split (Terraform-style preview)
**Source:** `internal/command/pool.go` `runPoolUpdate`/`runPoolApply`, `internal/pool/impact.go` `BuildImpactPlan`
**Apply to:** `internal/playback/compile.go`'s `Compile(show.State) -> CompiledPlan` is structurally this same "pure builder, never mutates input, produces a reviewable/pluggable document" pattern — reuse the review-then-apply mental model even though D-05 makes plan *adoption* automatic (next-bar boundary) rather than an explicit second CLI verb.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/playback/engine.go` | service (tick loop) | event-driven (real-time) | First stateful/concurrent (goroutine + `atomic.Pointer`) component in this codebase — every existing package is synchronous CLI-request-scoped. RESEARCH.md's Pattern 3 (Lock-Free Snapshot Publish) and the `StageEdit`/`tick` code examples are the primary reference; no in-repo precedent exists to copy beyond the general "pure function of inputs" discipline already used in `internal/pool/impact.go`. |
| `internal/programming/history.go` | service (undo/redo stack) | event-driven (session-only) | No prior undo/history package exists in this repo. Implement as a plain in-memory linear slice/stack per D-12/13/14 — no persistence pattern applies (explicitly session-only, never through `show.Save`). |
| `internal/playback/clock.go` | service (pure function) | transform (pure) | No prior pure-time-arithmetic package exists. Copy the `Position(now, bpm, barsPerLoop, loopStart) MusicalPosition` function verbatim from RESEARCH.md Pattern 1 — it is already a complete, ready-to-use reference implementation, not merely a sketch. |

## Metadata

**Analog search scope:** `internal/pool`, `internal/deployment`, `internal/show`, `internal/command`, `internal/fixture`, `internal/strictjson`, `internal/command/router.go`
**Files scanned:** `internal/show/state.go`, `internal/pool/model.go`, `internal/pool/impact.go` (referenced, not fully excerpted), `internal/deployment/model.go`, `internal/command/pool.go`, `internal/command/router.go`, `internal/fixture/model.go`
**Pattern extraction date:** 2026-07-21
