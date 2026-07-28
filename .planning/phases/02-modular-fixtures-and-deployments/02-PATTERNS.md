# Phase 2: Modular Fixtures and Deployments - Pattern Map

**Mapped:** 2026-07-21
**Files analyzed:** 14 (greenfield packages; count reflects planning-level file groups, not final task split)
**Analogs found:** 14 / 14 (all role-match or partial-match — domain is greenfield, no exact prior instance exists)

## Context

Phase 2 is entirely greenfield (`internal/fixture`, `internal/pool`, `internal/deployment`, `internal/substitution`, `schemas/fixture.schema.json` — none exist yet per RESEARCH.md/STATE.md). There is no in-repo analog with the *same* domain; every mapping below is a structural/role analog from Phase 1's `internal/trace/apply` (plan/apply/integrity/freshness), `internal/strictjson` (canonical hash), `internal/command` (route self-registration), `internal/projectconfig` (strict decode diagnostics), and `internal/contracts` (schema generation registry). The planner should treat every excerpt below as "copy this shape," not "this exact file already does fixtures."

## File Classification

| New/Modified File (illustrative, per RESEARCH.md structure) | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/fixture/model.go` (`FixtureDefinition`, `Capability`) | model | transform | `internal/contracts/model.go` (typed Go struct → schema projection) | role-match |
| `internal/fixture/decode.go` (YAML strict decode) | model/decode | request-response (load) | `internal/projectconfig/decode.go` (`ValidateConcern`/strict diagnostics) + `internal/strictjson/decode.go` (`DecodeStrict`) | role-match |
| `internal/fixture/ofl/*.go` (OFL fetch/normalize) | service | file-I/O + event-driven (network fetch) | `internal/trace/transport/process.go` (external-process/network adapter boundary) — partial match | partial-match |
| `internal/fixture/provenance.go` | model | transform | `internal/trace/catalog/model.go` (record/metadata shape) — partial | partial-match |
| `internal/fixture/identity.go` (sha256 pinning) | utility | transform | `internal/strictjson/decode.go` (`CanonicalEncode`) + `internal/trace/apply/guard.go` (`recomputePlanID`) | exact (pattern-level) |
| `internal/pool/model.go`, `internal/deployment/model.go` | model | CRUD | `internal/trace/catalog/model.go` (domain model, no I/O) | role-match |
| `internal/pool/impact.go` (dependents walk, ImpactPlan builder) | service | transform | `internal/trace/reconcile/diff.go` (`BuildCompletePreview`-style diff/plan builder) | role-match |
| `internal/pool/plan.go` (`ValidatePlanIntegrity`/`ValidatePlanFreshness`/apply) | service | event-driven (plan/apply) | `internal/trace/apply/guard.go` + `internal/trace/apply/engine.go` | exact (pattern-level) |
| `internal/substitution/plan.go` (capability-diff impact plan) | service | transform | Same as `internal/pool/plan.go` — reuses the identical ImpactPlan shape (D-16) | exact (pattern-level) |
| `internal/command/fixture.go` (`fixture validate`, `fixture import` routes) | controller/route | request-response | `internal/command/linear.go` (route self-registration, arg parsing, Result shaping) | exact (pattern-level) |
| `internal/command/pool.go` / `internal/command/deployment.go` (`pool update`/`pool apply` dry-run/apply split) | controller/route | request-response | `internal/command/linear.go` (`linear preview`/`linear apply` dry-run/apply split) | exact (pattern-level) |
| `schemas/fixture.schema.json` (generated) | config | transform (generated artifact) | `schemas/golc-project.schema.json` + `internal/contracts/generate.go`/`model.go` (registry + `invopop/jsonschema` reflection) | exact (pattern-level) |
| `internal/fixture/*_test.go`, `internal/pool/*_test.go` etc. | test | — | `internal/trace/apply/*_test.go`, `internal/contracts/generate_test.go` (not read in detail; follow existing `_test.go` naming/table-driven convention in same package) | role-match |

## Pattern Assignments

### `internal/fixture/model.go` + `schemas/fixture.schema.json` (model, transform)

**Analog:** `internal/contracts/model.go` + `internal/contracts/generate.go`

**Reflector/schema-projection pattern** (`internal/contracts/model.go` lines 28-51):
```go
// newReflector returns the one Reflector configuration every projection
// in this file reflects through: Draft 2020-12 by default, and
// DoNotReference so every generated schema is self-contained.
func newReflector() *jsonschema.Reflector {
    return &jsonschema.Reflector{DoNotReference: true}
}

type RootIndexSchema struct {
    SchemaVersion int                   `json:"schema_version" jsonschema:"required,enum=1,description=Supported root index schema version."`
    Concerns      []RootIndexConcernRef `json:"concerns" jsonschema:"required,minItems=1,uniqueItems=true,description=..."`
}
```
**Struct-tag escaping gotcha called out in the same file's header comment (lines 15-23):** use bracket character classes (`[.]`) instead of `\.` in `jsonschema:"pattern=..."` tags — an unescaped backslash silently fails `reflect.StructTag.Lookup`'s unquoting. Apply this verbatim to any `FixtureDefinition`/`Capability` pattern-validated field (e.g. hex hash, semver).

**Registry self-registration pattern** (`internal/contracts/generate.go` lines 65-110):
```go
func RegisterSchema(descriptor SchemaDescriptor) error { /* dedupe by name+path */ }
func MustRegisterSchema(descriptor SchemaDescriptor) SchemaDescriptor {
    if err := RegisterSchema(descriptor); err != nil {
        panic(err.Error())
    }
    return descriptor
}
```
Fixture schema generation should add one `SchemaDescriptor{Name: "fixture", OutputPath: "schemas/fixture.schema.json", Schema: func() *jsonschema.Schema {...}}` registered the same way, so it becomes part of the existing `GenerateAll`/`CheckDrift` traversal (`internal/contracts/generate.go` lines 152-227) without editing a central switch.

---

### `internal/fixture/decode.go` (YAML strict decode, request-response/load)

**Analogs:** `internal/strictjson/decode.go` (`DecodeStrict`) and `internal/projectconfig/decode.go` (`ValidateConcern`)

**Strict-decode-then-typed-diagnostics shape** (`internal/strictjson/decode.go` lines 99-119):
```go
func DecodeStrict(data []byte, out any) error {
    if err := ValidateSingleValueNoDuplicateNames(data); err != nil {
        return err
    }
    decoder := json.NewDecoder(bytes.NewReader(data))
    decoder.DisallowUnknownFields()
    decoder.UseNumber()
    if err := decoder.Decode(out); err != nil {
        return fmt.Errorf("STRICTJSON_DECODE: %v", err)
    }
    ...
}
```
For YAML (FIXT-02), the same "reject before populating anything" discipline applies via `go.yaml.in/yaml/v4`'s `WithUniqueKeys()`/`WithKnownFields()` (per RESEARCH.md Code Examples) instead of `DisallowUnknownFields`. Wrap decode errors in the same `GOLC_{DOMAIN}_{CONDITION}` code style, e.g. `GOLC_FIXTURE_YAML_INVALID`.

**Diagnostic-with-actionable-message shape** (`internal/projectconfig/decode.go` lines 139-157, `validateLiteral`):
```go
func validateLiteral(key, value, origin string, spec KeySpec) error {
    if spec.Pattern != nil && !spec.Pattern.MatchString(value) {
        return fmt.Errorf(
            "GOLC_CONFIG_VALUE_INVALID: %q does not match the required shape for %s in %s",
            value, key, origin)
    }
    return nil
}
```
Copy this "value, key, origin" error-message shape for FIXT-02's range/semantics validation (e.g. `GOLC_FIXTURE_CAPABILITY_RANGE_INVALID: %v is outside [0,1] for capability %q in %s`).

---

### `internal/fixture/identity.go` (FIXT-05 pinning) and `internal/pool/plan.go` / `internal/substitution/plan.go` (impact-plan integrity/freshness)

**Analog:** `internal/trace/apply/guard.go` + `internal/strictjson.CanonicalEncode`

**Canonical hash binding** (`internal/strictjson/decode.go` lines 121-135):
```go
func CanonicalEncode(v any) ([]byte, error) {
    encoded, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("STRICTJSON_ENCODE: %v", err)
    }
    encoded = bytes.ReplaceAll(encoded, []byte("\r\n"), []byte("\n"))
    if !bytes.HasSuffix(encoded, []byte("\n")) {
        encoded = append(encoded, '\n')
    }
    return encoded, nil
}
```

**Integrity gate (self-hash recomputation)** (`internal/trace/apply/guard.go` lines 60-96):
```go
func recomputePlanID(plan reconcile.Plan) (string, error) {
    body := planBodyMirror{ /* exact field mirror of the hashed body */ }
    encoded, err := strictjson.CanonicalEncode(body)
    if err != nil {
        return "", fmt.Errorf("GOLC_APPLY_PLAN_HASH: %v", err)
    }
    return reconcile.PlanID(encoded), nil
}

func ValidatePlanIntegrity(plan reconcile.Plan) error {
    if plan.SchemaVersion != reconcile.SchemaVersion {
        return fmt.Errorf("GOLC_APPLY_PLAN_SCHEMA: ...")
    }
    recomputed, err := recomputePlanID(plan)
    if err != nil { return err }
    if recomputed != plan.PlanID {
        return fmt.Errorf("GOLC_APPLY_PLAN_HASH: plan_id %q does not match its own recomputed canonical hash %q; the plan bytes were altered after hashing", plan.PlanID, recomputed)
    }
    return nil
}
```

**Freshness gate (recompute-and-compare against current state)** (`internal/trace/apply/guard.go` lines 98-115):
```go
func ValidatePlanFreshness(plan reconcile.Plan, intents []reconcile.Intent, mappings []catalog.RemoteMapping, snapshot transport.Snapshot, baselines []reconcile.SyncBaseline) error {
    fresh, err := reconcile.BuildCompletePreview(intents, mappings, snapshot, baselines)
    if err != nil {
        return fmt.Errorf("GOLC_APPLY_PLAN_STALE: recomputing the current preview failed: %v", err)
    }
    if fresh.PlanID != plan.PlanID {
        return fmt.Errorf("GOLC_APPLY_PLAN_STALE: plan %s no longer matches current repository/remote state (recomputed %s); re-run linear preview", plan.PlanID, fresh.PlanID)
    }
    return nil
}
```
D-16 explicitly requires reusing this exact two-gate shape for `internal/pool/plan.go`'s `ValidatePlanIntegrity`/`ValidatePlanFreshness` (rename the diagnostic code prefix to `GOLC_POOL_PLAN_*` and message to "re-run pool review", per RESEARCH.md Pattern 3's own worked example) and again for `internal/substitution/plan.go` (D-16 also warns against building a second, independently-evolving mechanism — reuse the same helper, parameterized by pool vs. substitution request, rather than duplicating it).

**Apply engine's "stop at first non-clean result, journal the achieved prefix" discipline** (`internal/trace/apply/engine.go` lines 181-224, `applyOperations`/`achievedPrefix`) is the model for POOL-05's atomic-apply guarantee — copy the "exact contiguous prefix" pattern if pool apply also needs interruption-safety; if D-12's small-rig scale means pool apply is genuinely single-transaction/no-resume, at minimum reuse the status enum (`StatusCompleted`/`StatusNoop`/`StatusPending`/`StatusBlocked`, lines 86-104 of `model.go`) for POOL-03's impact-plan operation statuses.

---

### `internal/command/fixture.go`, `internal/command/pool.go`, `internal/command/deployment.go` (controller/route, request-response)

**Analog:** `internal/command/linear.go` + `internal/command/router.go`

**Scope + route self-registration** (`internal/command/linear.go` lines 29-97):
```go
var _ = MustDeclareScope(ScopeRegistration{
    Scope:   "linear",
    Summary: "Repository-owned planning identity catalog and Linear reconciliation operations.",
})

var _ = MustDeclareRoute(CommandRegistration{
    Route:   "linear catalog",
    Summary: "Print the offline repository-owned planning identity catalog as deterministic JSON: linear catalog --offline --format json.",
    Handler: runLinearCatalog,
})
```
D-04 requires `fixture`/`pool`/`deployment` operations to register through this exact contract. Declare scopes `"fixture"`, `"pool"`, `"deployment"` and routes like `"fixture validate"`, `"fixture import"`, `"pool update"`, `"pool apply"`, `"deployment create"` following this literal shape. `Request`/`Result`/`CommandRegistration`/`ScopeRegistration` types are defined in `internal/command/router.go` lines 22-56 — reuse as-is, no new dispatch shape needed.

**Flag-parsing convention** (`internal/command/linear.go` lines 133-164, `parseOfflineJSONArgs`):
```go
func parseOfflineJSONArgs(usage string, args []string) error {
    for i := 0; i < len(args); {
        argument := args[i]
        switch {
        case argument == "--offline":
            offline = true
            i++
        case argument == "--format":
            if i+1 >= len(args) {
                return fmt.Errorf("GOLC_LINEAR_USAGE: --format requires a value; usage: %s", usage)
            }
            ...
        case strings.HasPrefix(argument, "--format="):
            ...
        default:
            return fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", argument, usage)
        }
    }
    ...
}
```
Copy this exact "supports `--flag value` and `--flag=value`, rejects anything else" parser shape for `fixture validate <file>`, `pool update --add ... --remove ...`, `pool apply <plan-file> --plan-id <id>`.

**Dry-run/apply split** (D-15) modeled directly on `"linear preview"` (writes a plan file, never mutates — lines 388-432) vs. `"linear apply"` (validates then mutates — referenced at lines 92-97 and via `RunApply` in `engine.go` lines 244-290). `pool update`/`fixture import` compute-and-print-or-write-plan; a separate `pool apply`/`fixture import --confirm`-style route performs `ValidatePlanIntegrity` → `ValidatePlanFreshness` → apply, exactly mirroring `runLinearPreview`/`RunApply`'s separation. Never let one handler both compute and mutate (explicit anti-pattern already enforced in this repo).

**Canonical plan write** (`internal/command/linear.go` lines 374-386, `writePreviewPlan`):
```go
func writePreviewPlan(root, outPath string, plan reconcile.Plan) Result {
    payload, err := strictjson.CanonicalEncode(plan)
    if err != nil {
        return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_PREVIEW_ENCODE_FAILED: %v\n", err))}
    }
    destination := resolveWritablePath(root, outPath)
    if err := os.WriteFile(destination, payload, 0o644); err != nil {
        return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_PREVIEW_WRITE_FAILED: %v\n", err))}
    }
    return Result{Stdout: []byte(fmt.Sprintf("GOLC_LINEAR_PREVIEW: wrote %s\n", destination))}
}
```
Copy verbatim (renamed) for `pool update --out <path>` / impact-plan writers.

---

## Shared Patterns

### Diagnostic code naming convention
**Source:** repo-wide (`GOLC_{DOMAIN}_{CONDITION}`, e.g. `GOLC_APPLY_PLAN_STALE`, `GOLC_CONFIG_DUPLICATE_AUTHORITY`, `GOLC_LINEAR_USAGE`, `STRICTJSON_DUPLICATE_NAME`)
**Apply to:** every new diagnostic in `internal/fixture`, `internal/pool`, `internal/deployment`, `internal/substitution`, and their `internal/command` routes — use `GOLC_FIXTURE_*`, `GOLC_POOL_*`, `GOLC_DEPLOYMENT_*`, `GOLC_SUBSTITUTION_*` prefixes consistently.

### Canonical encoding + content hashing
**Source:** `internal/strictjson/decode.go` (`CanonicalEncode`), reused by `internal/trace/apply/guard.go` (`recomputePlanID`)
**Apply to:** FIXT-05 fixture identity/hash pinning, POOL-05/08 impact-plan `plan_id` binding. Never hand-roll a second canonical-JSON encoder or hash scheme (explicit Don't-Hand-Roll entry in RESEARCH.md).

### Plan integrity/freshness two-gate contract
**Source:** `internal/trace/apply/guard.go` (`ValidatePlanIntegrity`, `ValidatePlanFreshness`)
**Apply to:** `internal/pool/plan.go` (POOL-03/05) and `internal/substitution/plan.go` (POOL-06/07/08) — D-16 explicitly mandates reuse rather than a second mechanism.

### Command self-registration (`MustDeclareRoute`/`MustDeclareScope`)
**Source:** `internal/command/router.go` (contract) + `internal/command/linear.go` (concrete usage)
**Apply to:** every `fixture`/`pool`/`deployment` CLI route (D-04) — this is the seam Phase 6 (Wails UI) and Phase 7 (external API) will call unchanged later, so route/handler shape must match this contract exactly, not a bespoke dispatcher.

### Schema generation registry (`invopop/jsonschema`)
**Source:** `internal/contracts/generate.go` (`RegisterSchema`/`MustRegisterSchema`/`GenerateAll`/`CheckDrift`) + `internal/contracts/model.go` (struct-tag projection, including the `[.]`-not-`\.` pattern-escaping gotcha)
**Apply to:** `schemas/fixture.schema.json` generation from the canonical `FixtureDefinition` Go struct (FIXT-01).

## No Analog Found (novel domain logic — no internal or external precedent)

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/pool/impact.go` dependents walk (groups/themes/palettes/scenes/chases/motion presets/controller mappings) | service | transform | POOL-01/02's logical-pool-to-dependents graph is GOLC-original domain modeling; `internal/trace/reconcile/diff.go`'s intent-vs-remote diff is a structural analog for "walk and diff," not a dependents-graph analog. Planner should design this from RESEARCH.md's Architecture Patterns diagram, not from an existing diff implementation. |
| `internal/substitution/*` capability-diff severity taxonomy (missing/incompatible/unsupported) | service | transform | Confirmed in RESEARCH.md (Common Pitfalls #2): no OFL/GDTF field to borrow, no in-repo severity-taxonomy precedent exists (Phase 1 code has no capability/severity concept at all). Pure new design per D-14. |
| `internal/fixture/ofl/*.go` fetch/cache client | service | file-I/O + network | `internal/trace/transport/process.go` is a subprocess-based transport, not an HTTP client; no existing HTTP-fetch-plus-local-cache code exists in this repo yet. Use Go stdlib `net/http`+`context` per RESEARCH.md Standard Stack; no internal analog to copy beyond general timeout/error-handling hygiene. |

## Metadata

**Analog search scope:** `internal/command`, `internal/trace/apply`, `internal/trace/reconcile`, `internal/trace/catalog`, `internal/trace/transport`, `internal/strictjson`, `internal/projectconfig`, `internal/contracts`, `schemas/*.schema.json`
**Files read directly:** `internal/trace/apply/model.go`, `internal/trace/apply/guard.go`, `internal/trace/apply/engine.go`, `internal/strictjson/decode.go`, `internal/command/linear.go`, `internal/command/linear_sync.go`, `internal/command/router.go`, `internal/projectconfig/decode.go`, `internal/contracts/generate.go`, `internal/contracts/model.go`
**Pattern extraction date:** 2026-07-21
