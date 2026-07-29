// scripts.go declares the Script/CapabilityProfile domain model (08-01-
// PLAN.md Task 1, CONTEXT D-06/D-07/D-09/D-14/D-17): a Script is a single
// self-contained TypeScript source file (D-14 -- no multi-file/project-
// style scripts in this phase) saved inside show.State as another entity
// in the single revisioned document (D-17), so it inherits autosave,
// recovery, migration, and export for free. Script copies
// internal/scene/scene.go's identity/construction/unique-name shape:
// identity is a durable UUIDv7 minted once at creation, never re-minted.
// CapabilityProfile.Scope reuses show.APIKeyScope directly (D-06) -- there
// is exactly one scope enum in the codebase, never a parallel
// script-specific set. ResolveResourceLimits follows
// internal/api/ratelimit.go's safe-default discipline (D-09): a named
// preset (quick-action/long-running-automation) always resolves to its own
// fixed values regardless of any custom fields set on the profile; only
// the "advanced" preset's escape hatch reads the profile's own numeric
// fields, and a zero, negative, or absent custom value there resolves to
// the package-level safe default -- an unset limit is never "unlimited."
package show

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ScriptRunStatus names a Script's last-known run outcome (D-16's library-
// row summary field).
type ScriptRunStatus string

// The closed set of run-status values a Script's LastRunStatus can carry.
const (
	ScriptRunStatusNeverRun   ScriptRunStatus = "never_run"
	ScriptRunStatusSucceeded  ScriptRunStatus = "succeeded"
	ScriptRunStatusFailed     ScriptRunStatus = "failed"
	ScriptRunStatusTerminated ScriptRunStatus = "terminated"
	ScriptRunStatusRunning    ScriptRunStatus = "running"
)

// ResourcePreset names one of D-09's two fixed named presets, plus the
// "advanced" escape hatch that reads a profile's own numeric override
// fields instead of a fixed preset value.
type ResourcePreset string

// The closed set of resource presets a CapabilityProfile.Preset can carry.
const (
	ResourcePresetQuickAction ResourcePreset = "quick-action"
	ResourcePresetLongRunning ResourcePreset = "long-running-automation"
	ResourcePresetAdvanced    ResourcePreset = "advanced"
)

// Package-level safe defaults (mirrors internal/api/ratelimit.go's
// defaultRatePerMinute/defaultRateBurst discipline): every one of these is
// also the exact set of fixed values the "quick-action" preset resolves
// to, and the fallback ResolveResourceLimits uses for any "advanced"
// profile field left at zero, negative, or unset.
const (
	defaultScriptDeadlineSeconds = 30
	defaultScriptRatePerSecond   = 20
	defaultScriptMemoryLimitMB   = 256
	defaultScriptCPUCapPercent   = 25
)

// The fixed values the "long-running-automation" preset always resolves
// to, regardless of any custom fields set on the profile.
const (
	longRunningDeadlineSeconds = 3600
	longRunningRatePerSecond   = 5
	longRunningMemoryLimitMB   = 512
	longRunningCPUCapPercent   = 25
)

// Sanity ceilings ValidateScript enforces on an "advanced" profile's own
// numeric fields (GOLC_SCRIPT_LIMIT_INVALID), mirroring
// internal/scene/scene.go's maxBarsPerLoop DoS-ceiling precedent: a
// pathologically large limit is rejected rather than silently accepted. A
// zero or negative value is never rejected here -- per D-09 it resolves to
// the package safe default in ResolveResourceLimits, it is not itself an
// error.
const (
	maxScriptDeadlineSeconds = 86400
	maxScriptRatePerSecond   = 1000
	maxScriptMemoryLimitMB   = 8192
	maxScriptCPUCapPercent   = 100
)

// CapabilityProfile is a Script's per-script saved default capability/
// resource-limit assignment (D-07): shown pre-filled and editable in the
// run dialog before each execution, never re-entered from scratch and
// never silently reused without review. Scope reuses show.APIKeyScope
// directly (D-06). Preset selects one of D-09's named presets or the
// "advanced" escape hatch; DeadlineSeconds/RatePerSecond/MemoryLimitMB/
// CPUCapPercent are only read by ResolveResourceLimits when Preset is
// ResourcePresetAdvanced.
type CapabilityProfile struct {
	Scope           APIKeyScope    `json:"scope"`
	Preset          ResourcePreset `json:"preset"`
	DeadlineSeconds int            `json:"deadline_seconds"`
	RatePerSecond   int            `json:"rate_per_second"`
	MemoryLimitMB   int            `json:"memory_limit_mb"`
	CPUCapPercent   int            `json:"cpu_cap_percent"`
}

// Script is a single self-contained TypeScript automation file (D-14)
// saved inside show.State (D-17). Identity is a durable UUIDv7 minted
// once at creation -- never derived from Name, and never re-minted by a
// rename or a source edit. Source is stored verbatim (byte-for-byte),
// with no normalization, transpilation, or reformatting applied at save
// time. LastRunStatus/LastRunReason/LastRunAt are D-16's library-row
// summary fields and D-12's "last logs/diagnostics/status remain visible"
// state -- later plans (08-05+) are the only writers of the Last* fields
// beyond their NewScript-seeded zero value.
type Script struct {
	ID                uuid.UUID         `json:"id"`
	Name              string            `json:"name"`
	Source            string            `json:"source"`
	CapabilityProfile CapabilityProfile `json:"capability_profile"`
	LastRunStatus     ScriptRunStatus   `json:"last_run_status"`
	LastRunReason     string            `json:"last_run_reason,omitempty"`
	LastRunAt         string            `json:"last_run_at,omitempty"`
}

// ResolvedLimits is CapabilityProfile.ResolveResourceLimits' return
// shape: the concrete, never-unlimited limits the script host (08-05+)
// enforces for one run.
type ResolvedLimits struct {
	Deadline      time.Duration
	RatePerSecond int
	MemoryLimitMB int
	CPUCapPercent int
}

// validResourcePresets is the closed set ValidateScript checks
// CapabilityProfile.Preset against.
var validResourcePresets = map[ResourcePreset]bool{
	ResourcePresetQuickAction: true,
	ResourcePresetLongRunning: true,
	ResourcePresetAdvanced:    true,
}

// resolvePositiveOrDefault returns value when it is strictly positive,
// otherwise fallback -- the single "zero/negative/absent is never
// unlimited" rule ResolveResourceLimits applies to every advanced-preset
// numeric field (D-09).
func resolvePositiveOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// NewScript mints a fresh UUIDv7-identified Script with an empty Source,
// the quick-action preset at the least-privileged playback scope (D-06:
// NewScript never defaults to a broader scope), and LastRunStatus never-
// run. IDs are minted only at creation time -- never derived from Name,
// and never re-minted by a later rename or source edit.
func NewScript(name string) (Script, error) {
	if strings.TrimSpace(name) == "" {
		return Script{}, errors.New("GOLC_SCRIPT_NAME_EMPTY: script name must not be empty")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Script{}, fmt.Errorf("GOLC_SCRIPT_ID_MINT_FAILED: %v", err)
	}
	return Script{
		ID:   id,
		Name: name,
		CapabilityProfile: CapabilityProfile{
			Scope:  APIKeyScopePlayback,
			Preset: ResourcePresetQuickAction,
		},
		LastRunStatus: ScriptRunStatusNeverRun,
	}, nil
}

// validateCapabilityProfile rejects a Scope outside show.APIKeyScope's
// closed set (GOLC_SCRIPT_SCOPE_INVALID, D-06: an unrecognized value must
// never default to a broader scope), a Preset outside the three declared
// ResourcePreset values (GOLC_SCRIPT_PRESET_INVALID), and any numeric
// field exceeding its sanity ceiling (GOLC_SCRIPT_LIMIT_INVALID). A zero
// or negative numeric field is never rejected here -- see the
// maxScript*/resolvePositiveOrDefault doc comments.
func validateCapabilityProfile(p CapabilityProfile) error {
	if !validAPIKeyScopes[p.Scope] {
		return fmt.Errorf("GOLC_SCRIPT_SCOPE_INVALID: %q is not one of playback, authoring, admin", p.Scope)
	}
	if !validResourcePresets[p.Preset] {
		return fmt.Errorf("GOLC_SCRIPT_PRESET_INVALID: %q is not one of quick-action, long-running-automation, advanced", p.Preset)
	}
	if p.DeadlineSeconds > maxScriptDeadlineSeconds {
		return fmt.Errorf("GOLC_SCRIPT_LIMIT_INVALID: deadline_seconds %d exceeds the maximum of %d", p.DeadlineSeconds, maxScriptDeadlineSeconds)
	}
	if p.RatePerSecond > maxScriptRatePerSecond {
		return fmt.Errorf("GOLC_SCRIPT_LIMIT_INVALID: rate_per_second %d exceeds the maximum of %d", p.RatePerSecond, maxScriptRatePerSecond)
	}
	if p.MemoryLimitMB > maxScriptMemoryLimitMB {
		return fmt.Errorf("GOLC_SCRIPT_LIMIT_INVALID: memory_limit_mb %d exceeds the maximum of %d", p.MemoryLimitMB, maxScriptMemoryLimitMB)
	}
	if p.CPUCapPercent > maxScriptCPUCapPercent {
		return fmt.Errorf("GOLC_SCRIPT_LIMIT_INVALID: cpu_cap_percent %d exceeds the maximum of %d", p.CPUCapPercent, maxScriptCPUCapPercent)
	}
	return nil
}

// ValidateScript re-checks every invariant a hand-edited or otherwise
// untrusted Script must satisfy before it is trusted: Name is non-empty
// (GOLC_SCRIPT_NAME_EMPTY), and its CapabilityProfile passes
// validateCapabilityProfile.
func ValidateScript(s Script) error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("GOLC_SCRIPT_NAME_EMPTY: script %s declares an empty name", s.ID)
	}
	return validateCapabilityProfile(s.CapabilityProfile)
}

// ValidateScriptUniqueNames rejects any two scripts in scripts sharing the
// same Name: a duplicate name is always rejected before any save commits,
// never silently permitted.
func ValidateScriptUniqueNames(scripts []Script) error {
	seen := make(map[string]bool, len(scripts))
	for _, s := range scripts {
		if seen[s.Name] {
			return fmt.Errorf("GOLC_SCRIPT_NAME_DUPLICATE: a script named %q already exists", s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}

// ResolveResourceLimits returns the concrete, never-unlimited limits for
// p (D-09): "quick-action" and "long-running-automation" both always
// resolve to their own fixed values, regardless of any custom fields set
// on p -- only "advanced" (and any unrecognized/zero-value Preset, which
// falls through to the same safe-default branch as quick-action) reads
// p's own numeric fields, each independently falling back to the package
// default when zero, negative, or unset.
func (p CapabilityProfile) ResolveResourceLimits() ResolvedLimits {
	switch p.Preset {
	case ResourcePresetLongRunning:
		return ResolvedLimits{
			Deadline:      longRunningDeadlineSeconds * time.Second,
			RatePerSecond: longRunningRatePerSecond,
			MemoryLimitMB: longRunningMemoryLimitMB,
			CPUCapPercent: longRunningCPUCapPercent,
		}
	case ResourcePresetAdvanced:
		return ResolvedLimits{
			Deadline:      time.Duration(resolvePositiveOrDefault(p.DeadlineSeconds, defaultScriptDeadlineSeconds)) * time.Second,
			RatePerSecond: resolvePositiveOrDefault(p.RatePerSecond, defaultScriptRatePerSecond),
			MemoryLimitMB: resolvePositiveOrDefault(p.MemoryLimitMB, defaultScriptMemoryLimitMB),
			CPUCapPercent: resolvePositiveOrDefault(p.CPUCapPercent, defaultScriptCPUCapPercent),
		}
	default:
		return ResolvedLimits{
			Deadline:      defaultScriptDeadlineSeconds * time.Second,
			RatePerSecond: defaultScriptRatePerSecond,
			MemoryLimitMB: defaultScriptMemoryLimitMB,
			CPUCapPercent: defaultScriptCPUCapPercent,
		}
	}
}
