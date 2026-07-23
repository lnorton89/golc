// state.go declares the ShowState substrate (CONTEXT POOL-01/POOL-02/
// D-16): the working document every pool.Pool, deployment.Deployment,
// pool.Group, and (03-01-PLAN.md PROG-02/PROG-03) programming.
// ProgrammerState scratch buffer live inside, revisioned so 02-05's
// impact-plan freshness guard can detect a stale plan against a moved
// Revision. As of Phase 5 (CONTEXT D-01/D-02/D-03), Load/Save/LoadForRead
// live in store.go, backed by a SQLite `.golc` file instead of a JSON
// file; this file keeps only the domain shape (State, Tempo),
// resolvePath, and validate -- the exact "nothing from disk is trusted
// before validate() passes" doctrine (CONTEXT threat T-02-10) store.go's
// Load/LoadForRead both still run before returning a State.
package show

import (
	"path/filepath"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
)

// SchemaVersion is the current State schema version Save always writes.
const SchemaVersion = 1

// State is the ShowState container: a working, JSON-persisted document
// carrying every logical Pool, Group, and concrete Deployment for one
// show (Phase 5 will later supersede this working representation with
// the durable .golc format). Revision is a monotonic counter every Save
// bumps; 02-05's impact-plan freshness guard (D-16) compares an expected
// Revision against this field to detect a stale plan.
type State struct {
	SchemaVersion int                          `json:"schema_version"`
	Revision      int                          `json:"revision"`
	Pools         []pool.Pool                  `json:"pools"`
	Deployments   []deployment.Deployment      `json:"deployments"`
	Groups        []pool.Group                 `json:"groups"`
	Programmer    *programming.ProgrammerState `json:"programmer,omitempty"`
	Themes        []programming.Theme          `json:"themes"`
	Presets       []programming.Preset         `json:"presets"`
	Chases        []programming.Chase          `json:"chases"`
	MotionPresets []programming.MotionPreset   `json:"motion_presets"`
	Scenes        []scene.Scene                `json:"scenes"`
	BlendPresets  []scene.BlendPreset           `json:"blend_presets"`
	Tempo         Tempo                         `json:"tempo"`
}

// Tempo is the show-wide musical tempo (SCEN-02/SCEN-03): a single BPM
// value the playback clock (03-06) reads to derive every scene's bar-based
// looping and chase/motion step timing (CONTEXT D-10 -- one authoritative
// musical clock for the whole engine). BPM = 0 is the fresh-show "not yet
// set" value; SCEN-02's own bounds validation (numeric entry) and SCEN-03's
// tap-tempo conversion are later plans' concern (03-06) -- this plan only
// adds the field and its persistence.
type Tempo struct {
	BPM float64 `json:"bpm"`
}

// resolvePath returns path unchanged when it is already absolute (the
// caller's own explicit choice of where to read/write); otherwise it is
// resolved relative to root. Mirrors internal/command/linear.go's
// resolveWritablePath: this package cannot import internal/command
// (internal/command imports internal/show, not the reverse), so the
// shape is duplicated rather than shared.
func resolvePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

// validate runs every whole-State invariant Load and Save both enforce
// before trusting or persisting a State: every pool individually valid,
// unique pool names, unique deployment names, at most one active
// deployment, every instance address within the valid DMX/Art-Net range,
// unique group names, every group's member refs resolving to an existing
// pool/pool member (WR-02), and -- when a Programmer buffer is present --
// every touched attribute still within the normalized [0,1] bound and a
// supported capability type (PROG-02/PROG-03). Every Chase's step order/
// unit/duration/count ceiling and unique name (PROG-05, D-09/D-10), and
// every MotionPreset's position/beam capability scope and unique name
// (PROG-06, D-04), are re-checked here too -- the single validate() entry
// point every new object type extends rather than a parallel path.
func validate(s State) error {
	for _, p := range s.Pools {
		if err := pool.Validate(p); err != nil {
			return err
		}
	}
	if err := pool.ValidateUniqueNames(s.Pools); err != nil {
		return err
	}
	if err := deployment.ValidateUniqueNames(s.Deployments); err != nil {
		return err
	}
	if err := deployment.ValidateSingleActive(s.Deployments); err != nil {
		return err
	}
	for _, d := range s.Deployments {
		for _, instance := range d.Instances {
			if err := deployment.ValidateInstanceAddress(instance); err != nil {
				return err
			}
		}
	}
	if err := pool.ValidateUniqueGroupNames(s.Groups); err != nil {
		return err
	}
	if err := pool.ValidateGroupReferences(s.Pools, s.Groups); err != nil {
		return err
	}
	if s.Programmer != nil {
		if err := programming.ValidateProgrammer(*s.Programmer); err != nil {
			return err
		}
	}
	for _, preset := range s.Presets {
		if err := programming.ValidatePreset(preset); err != nil {
			return err
		}
	}
	if err := programming.ValidateThemeUniqueNames(s.Themes); err != nil {
		return err
	}
	if err := programming.ValidatePresetUniqueNames(s.Presets); err != nil {
		return err
	}
	for _, chase := range s.Chases {
		if err := programming.ValidateChase(chase); err != nil {
			return err
		}
	}
	if err := programming.ValidateChaseUniqueNames(s.Chases); err != nil {
		return err
	}
	for _, motionPreset := range s.MotionPresets {
		if err := programming.ValidateMotionPreset(motionPreset); err != nil {
			return err
		}
	}
	if err := programming.ValidateMotionPresetUniqueNames(s.MotionPresets); err != nil {
		return err
	}
	for _, sc := range s.Scenes {
		if err := scene.ValidateScene(sc); err != nil {
			return err
		}
	}
	if err := scene.ValidateSceneUniqueNames(s.Scenes); err != nil {
		return err
	}
	if err := scene.ValidateSingleActiveScene(s.Scenes); err != nil {
		return err
	}
	for _, blendPreset := range s.BlendPresets {
		if err := scene.ValidateBlendPreset(blendPreset); err != nil {
			return err
		}
	}
	if err := scene.ValidateBlendPresetUniqueNames(s.BlendPresets); err != nil {
		return err
	}
	if err := scene.ValidateLayerReferences(s.Scenes, s.Themes, s.Presets, s.Chases, s.MotionPresets); err != nil {
		return err
	}
	return nil
}
