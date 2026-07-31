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
	"strconv"
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

// RenamePool renames a pool via "pool rename" -- an immediate mutation
// (no impact-plan preview, since a rename only ever changes Pool.Name,
// which nothing else in persisted state references).
func (s *FixturePatchService) RenamePool(oldName, newName string) Result {
	return s.execute([]string{"pool", "rename", oldName, newName, "--show", s.showPath})
}

// DeletePool cascade-deletes a pool via "pool delete" -- removing it and
// every deployment instance/group member ref that references it in one
// atomic Save (POOL-04's review-before-apply doctrine doesn't apply here:
// deletion is a single deterministic operation, matching CreatePool's own
// immediacy, not a multi-op fan-out needing separate review).
func (s *FixturePatchService) DeletePool(name string) Result {
	return s.execute([]string{"pool", "delete", name, "--show", s.showPath})
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
// <stableKey>|<contentHash>|<mode>|<channelCount> --propagate preview
// --json" (POOL-04: review-before-apply). channelCount is the selected
// mode's real channel width (FixtureLibraryRowView.modeChannelCounts,
// svc_fixturelibrary.go); a value below 1 omits the field entirely, which
// falls back to pool.defaultInstanceChannelCount's 1-channel width
// (mirrors "pool update"'s own optional-field CLI contract). The pool's
// members remain unchanged until a matching ApplyPatch(planId) call
// commits the returned plan.
func (s *FixturePatchService) AddPoolMemberPreview(poolName, stableKey, contentHash, mode string, channelCount int) Result {
	for _, field := range []string{stableKey, contentHash, mode} {
		if strings.Contains(field, "|") {
			return Result{ExitCode: 2, Stderr: "GOLC_WAILS_POOL_MEMBER_FIELD_INVALID: fixture stable key/content hash/mode must not contain \"|\"\n"}
		}
	}
	spec := poolMemberSpecString(stableKey, contentHash, mode, channelCount)
	result := s.execute([]string{
		"pool", "update", poolName,
		"--add", spec,
		"--propagate", "preview",
		"--json",
		"--show", s.showPath,
	})
	return s.cachePlan(result)
}

// poolMemberSpecString builds one "pool update --add" value, appending the
// optional trailing channel_count field (internal/command/pool.go's
// parsePoolMemberSpec) only when channelCount is a real resolved width
// (>= 1) -- an unresolved/unknown channelCount (0 or below) omits the
// field, reproducing the pre-channel-count spec shape exactly.
func poolMemberSpecString(stableKey, contentHash, mode string, channelCount int) string {
	if channelCount < 1 {
		return fmt.Sprintf("%s|%s|%s", stableKey, contentHash, mode)
	}
	return fmt.Sprintf("%s|%s|%s|%d", stableKey, contentHash, mode, channelCount)
}

// AddPoolMembersPreview returns the backend's non-committing impact preview
// for adding `count` units of one fixture reference (stableKey/contentHash/
// mode) to pool in a single batch (CONTEXT: browse-library-and-add-to-
// project), optionally force-attaching attachDeploymentID (non-empty) so a
// pool with no existing dependent deployment can still receive proposed
// instances (closes the "adopt a never-before-used pool" gap -- see
// internal/pool/impact.go's ImpactRequest.AttachDeployments), and optionally
// anchoring the universe/address scan at startUniverse/startAddress
// (either left 0 to keep today's system-suggested next-free slot).
// channelCount is the selected mode's real channel width
// (FixtureLibraryRowView.modeChannelCounts, svc_fixturelibrary.go): every
// one of the `count` proposed instances is spaced by this width instead of
// pool.defaultInstanceChannelCount's 1-channel fallback, so N units of a
// wide fixture (for example 5-channel) land at addresses 1, 6, 11, ...
// instead of colliding one address apart. A value below 1 omits the field
// and reproduces the pre-channel-count 1-channel-apart behavior. The
// pool's members remain unchanged until a matching ApplyPatch(planId) call
// commits the returned plan.
func (s *FixturePatchService) AddPoolMembersPreview(
	poolName, stableKey, contentHash, mode string,
	count int,
	attachDeploymentID string,
	startUniverse, startAddress int,
	channelCount int,
) Result {
	for _, field := range []string{stableKey, contentHash, mode} {
		if strings.Contains(field, "|") {
			return Result{ExitCode: 2, Stderr: "GOLC_WAILS_POOL_MEMBER_FIELD_INVALID: fixture stable key/content hash/mode must not contain \"|\"\n"}
		}
	}
	if count < 1 {
		return Result{ExitCode: 2, Stderr: "GOLC_WAILS_POOL_MEMBER_COUNT_INVALID: count must be at least 1\n"}
	}
	spec := poolMemberSpecString(stableKey, contentHash, mode, channelCount)
	args := []string{"pool", "update", poolName}
	for i := 0; i < count; i++ {
		args = append(args, "--add", spec)
	}
	if attachDeploymentID != "" {
		args = append(args, "--attach-deployment", attachDeploymentID)
	}
	if startUniverse > 0 {
		args = append(args, "--start-universe", strconv.Itoa(startUniverse))
	}
	if startAddress > 0 {
		args = append(args, "--start-address", strconv.Itoa(startAddress))
	}
	args = append(args, "--propagate", "preview", "--json", "--show", s.showPath)
	return s.cachePlan(s.execute(args))
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

// RenameDeployment renames a deployment via "deployment rename" -- an
// immediate mutation, mirroring RenamePool's own immediacy.
func (s *FixturePatchService) RenameDeployment(oldName, newName string) Result {
	return s.execute([]string{"deployment", "rename", oldName, newName, "--show", s.showPath})
}

// DeleteDeployment deletes a deployment (and its own instances) via
// "deployment delete".
func (s *FixturePatchService) DeleteDeployment(name string) Result {
	return s.execute([]string{"deployment", "delete", name, "--show", s.showPath})
}

// ReassignInstance in-place reassigns one deployment instance's mode/
// universe/address via "deployment instance reassign". An empty mode or
// a universe/address of 0 means "keep the instance's current value" --
// the caller never needs to re-supply every field just to change one.
func (s *FixturePatchService) ReassignInstance(deploymentName, instanceID, mode string, universe, address int) Result {
	args := []string{"deployment", "instance", "reassign", deploymentName, instanceID}
	if mode != "" {
		args = append(args, "--mode", mode)
	}
	if universe > 0 {
		args = append(args, "--universe", strconv.Itoa(universe))
	}
	if address > 0 {
		args = append(args, "--address", strconv.Itoa(address))
	}
	args = append(args, "--show", s.showPath)
	return s.execute(args)
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
