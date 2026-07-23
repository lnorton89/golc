// svc_fixturepatch.go fills FixturePatchService, the Wails binding
// closing VERIFICATION.md Gap B[0] for PLAY-10 (06-10-PLAN.md): every
// mutation (CreatePool/AddPoolMemberPreview/RemovePoolMemberPreview/
// ApplyPatch/CreateDeployment/ActivateDeployment) executes the matching
// already-implemented, already-tested "pool"/"deployment" CLI route
// (internal/command/pool.go, internal/command/deployment.go) via
// command.NewDefaultCommandRegistry -- exactly the SurfaceService pattern
// (svc_surface.go) this file mirrors -- so there is only one pool/
// deployment mutation implementation in this codebase, never a second one
// duplicated for the GUI.
//
// AddPoolMemberPreview/RemovePoolMemberPreview call "pool update
// --propagate preview --json", which never mutates the ShowState document
// (POOL-04/D-15); the returned pool.ImpactPlan is cached in-memory here,
// keyed by its own PlanID, so a later ApplyPatch(planId) call can hand the
// exact reviewed plan back to "pool apply" without the frontend ever
// needing to round-trip the plan's own bytes (a plan is written to a
// throwaway temp file only at apply time, and removed immediately
// afterward). This preserves the review-before-apply flow: a pool change
// is never committed on screen without the author first seeing the
// backend's own impact preview (PLAY-10 must_haves).
//
// ListPatch reads show.Load directly (never "show inspect --json", which
// only projects instance_count) and returns every pool's members plus
// every deployment's instances -- including each instance's persisted
// Mode/Universe/Address -- since PLAY-10's "assigning ... universes,
// addresses" clause is satisfied by SURFACING the backend's own
// system-computed values, never by adding a manual-entry control or a new
// command route (see 06-10-PLAN.md's flagged assumption).
package wails

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

// FixturePatchService is bound to the frontend via cmd/golc-desktop/
// main.go's options.App{Bind: [...]}. root/showPath are the exact
// ShowState location every method Loads/Saves against (mirrors
// SurfaceService's own fields); plans caches every previewed-but-not-yet-
// applied pool.ImpactPlan by its own PlanID for ApplyPatch to consume.
type FixturePatchService struct {
	pipeName string
	root     string
	showPath string

	mu    sync.Mutex
	plans map[string]pool.ImpactPlan
}

// NewFixturePatchService constructs a FixturePatchService targeting
// pipeName (reserved, unused by this ShowState-only CRUD -- mirrors
// SurfaceService's own unused pipeName field) and the ShowState at
// showPath, resolved against root.
func NewFixturePatchService(pipeName, root, showPath string) *FixturePatchService {
	return &FixturePatchService{
		pipeName: pipeName,
		root:     root,
		showPath: showPath,
		plans:    make(map[string]pool.ImpactPlan),
	}
}

// execute builds the default command registry and runs args against it,
// converting the internal/command.Result shape into this package's own
// Result shape (mirrors svc_surface.go's identical helper).
func (s *FixturePatchService) execute(args []string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// CreatePool creates a new named logical pool via "pool create", forwarding
// an optional comma-joined --requires capability list.
func (s *FixturePatchService) CreatePool(name string, requires []string) Result {
	args := []string{"pool", "create", name}
	if len(requires) > 0 {
		args = append(args, "--requires", strings.Join(requires, ","))
	}
	args = append(args, "--show", s.showPath)
	return s.execute(args)
}

// cachePlan decodes a "pool update --json" Result's Stdout into a
// pool.ImpactPlan and stores it keyed by its own PlanID, so a later
// ApplyPatch(planId) can hand it back to "pool apply" verbatim.
func (s *FixturePatchService) cachePlan(previewResult Result) Result {
	if previewResult.ExitCode != 0 {
		return previewResult
	}
	var plan pool.ImpactPlan
	if err := json.Unmarshal([]byte(previewResult.Stdout), &plan); err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_PLAN_DECODE_FAILED: %v", err)}
	}
	s.mu.Lock()
	s.plans[plan.PlanID] = plan
	s.mu.Unlock()
	return previewResult
}

// AddPoolMemberPreview returns the backend's non-committing impact preview
// for adding one fixture reference to pool at mode via "pool update --add
// <stableKey>|<contentHash>|<mode> --propagate preview --json" (POOL-04:
// review-before-apply). The pool's members remain unchanged until a
// matching ApplyPatch(planId) call commits the returned plan.
func (s *FixturePatchService) AddPoolMemberPreview(poolName, stableKey, contentHash, mode string) Result {
	for _, field := range []string{stableKey, contentHash, mode} {
		if strings.Contains(field, "|") {
			return Result{ExitCode: 2, Stderr: "GOLC_WAILS_POOL_MEMBER_FIELD_INVALID: fixture stable key/content hash/mode must not contain \"|\"\n"}
		}
	}
	spec := fmt.Sprintf("%s|%s|%s", stableKey, contentHash, mode)
	result := s.execute([]string{
		"pool", "update", poolName,
		"--add", spec,
		"--propagate", "preview",
		"--json",
		"--show", s.showPath,
	})
	return s.cachePlan(result)
}

// RemovePoolMemberPreview returns the backend's non-committing impact
// preview for removing memberID from pool via "pool update --remove
// <memberId> --propagate preview --json".
func (s *FixturePatchService) RemovePoolMemberPreview(poolName, memberID string) Result {
	result := s.execute([]string{
		"pool", "update", poolName,
		"--remove", memberID,
		"--propagate", "preview",
		"--json",
		"--show", s.showPath,
	})
	return s.cachePlan(result)
}

// ApplyPatch validates and atomically applies the previously-previewed
// impact plan identified by planID via "pool apply" (POOL-04/POOL-05/D-15
// two-gate integrity/freshness contract): an unrecognized planID (never
// previewed, already applied, or from a stale/expired cache) fails outright
// with GOLC_WAILS_PLAN_UNKNOWN rather than attempting a call the backend
// route has no plan file for; a recognized plan is written to a throwaway
// temp file, applied, and removed from both the temp filesystem and this
// service's own cache on success (single-use, mirrors the route's own
// revision-bump freshness guard).
func (s *FixturePatchService) ApplyPatch(planID string) Result {
	s.mu.Lock()
	plan, ok := s.plans[planID]
	s.mu.Unlock()
	if !ok {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf(
			"GOLC_WAILS_PLAN_UNKNOWN: no previewed impact plan with id %q is cached; re-run the add/remove preview before applying", planID)}
	}

	payload, err := strictjson.CanonicalEncode(plan)
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_PLAN_ENCODE_FAILED: %v", err)}
	}
	tmpFile, err := os.CreateTemp("", "golc-fixturepatch-plan-*.json")
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_PLAN_TEMP_FAILED: %v", err)}
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.Write(payload); err != nil {
		tmpFile.Close()
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_PLAN_TEMP_FAILED: %v", err)}
	}
	if err := tmpFile.Close(); err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_PLAN_TEMP_FAILED: %v", err)}
	}

	result := s.execute([]string{"pool", "apply", tmpPath, "--plan-id", planID, "--show", s.showPath})
	if result.ExitCode == 0 {
		s.mu.Lock()
		delete(s.plans, planID)
		s.mu.Unlock()
	}
	return result
}

// CreateDeployment creates a new named, inactive deployment via
// "deployment create".
func (s *FixturePatchService) CreateDeployment(name string) Result {
	return s.execute([]string{"deployment", "create", name, "--show", s.showPath})
}

// ActivateDeployment marks name the exactly-one active deployment via
// "deployment activate", deactivating every other deployment.
func (s *FixturePatchService) ActivateDeployment(name string) Result {
	return s.execute([]string{"deployment", "activate", name, "--show", s.showPath})
}

// PatchPoolMemberView is one PoolMember row in a PatchPoolView (id +
// fixture identity only -- never a filesystem path).
type PatchPoolMemberView struct {
	ID                 string `json:"id"`
	FixtureStableKey   string `json:"fixtureStableKey"`
	FixtureContentHash string `json:"fixtureContentHash"`
}

// PatchPoolView is one pool row for FixturePatch.tsx's pool list.
type PatchPoolView struct {
	ID                   string                `json:"id"`
	Name                 string                `json:"name"`
	RequiredCapabilities []string              `json:"requiredCapabilities,omitempty"`
	Members              []PatchPoolMemberView `json:"members"`
}

// PatchInstanceView is one deployment.Instance row, carrying the exact
// persisted Mode/Universe/Address fields (PLAY-10: system-computed
// addressing displayed, never manually entered).
type PatchInstanceView struct {
	ID           string `json:"id"`
	PoolID       string `json:"poolId"`
	PoolMemberID string `json:"poolMemberId"`
	Mode         string `json:"mode"`
	Universe     int    `json:"universe"`
	Address      int    `json:"address"`
}

// PatchDeploymentView is one deployment row for FixturePatch.tsx's
// deployment list, including every instance's mode/universe/address.
type PatchDeploymentView struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Active    bool                `json:"active"`
	Instances []PatchInstanceView `json:"instances"`
}

// PatchView is ListPatch's full return shape: every pool (with members)
// and every deployment (with instances) on the loaded ShowState.
type PatchView struct {
	Pools       []PatchPoolView       `json:"pools"`
	Deployments []PatchDeploymentView `json:"deployments"`
}

// ListPatch reads the ShowState at showPath DIRECTLY (never "show inspect
// --json", which only projects instance_count) and projects every pool's
// members plus every deployment's instances -- including each instance's
// persisted Universe/Address -- into a JSON-safe view for
// FixturePatch.tsx, mirroring ListSurfaces/ShowSurface's projection
// discipline.
func (s *FixturePatchService) ListPatch() (PatchView, error) {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return PatchView{}, err
	}

	view := PatchView{
		Pools:       make([]PatchPoolView, 0, len(state.Pools)),
		Deployments: make([]PatchDeploymentView, 0, len(state.Deployments)),
	}
	for _, p := range state.Pools {
		capabilities := make([]string, 0, len(p.RequiredCapabilities))
		for _, capabilityType := range p.RequiredCapabilities {
			capabilities = append(capabilities, string(capabilityType))
		}
		members := make([]PatchPoolMemberView, 0, len(p.Members))
		for _, m := range p.Members {
			members = append(members, PatchPoolMemberView{
				ID:                 m.ID.String(),
				FixtureStableKey:   m.FixtureStableKey,
				FixtureContentHash: m.FixtureContentHash,
			})
		}
		view.Pools = append(view.Pools, PatchPoolView{
			ID:                   p.ID.String(),
			Name:                 p.Name,
			RequiredCapabilities: capabilities,
			Members:              members,
		})
	}
	for _, d := range state.Deployments {
		instances := make([]PatchInstanceView, 0, len(d.Instances))
		for _, instance := range d.Instances {
			instances = append(instances, PatchInstanceView{
				ID:           instance.ID.String(),
				PoolID:       instance.PoolID.String(),
				PoolMemberID: instance.PoolMemberID.String(),
				Mode:         instance.Mode,
				Universe:     instance.Universe,
				Address:      instance.Address,
			})
		}
		view.Deployments = append(view.Deployments, PatchDeploymentView{
			ID:        d.ID.String(),
			Name:      d.Name,
			Active:    d.Active,
			Instances: instances,
		})
	}
	return view, nil
}
