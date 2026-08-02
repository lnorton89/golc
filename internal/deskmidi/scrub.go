// scrub.go implements the patch-instance-delete cascade safety net for desk
// mappings -- mirrors internal/scene/scrub.go's own "silently drop an
// already-broken reference before validate() ever sees it" discipline
// (called from show.Save, internal/show/store.go), so deleting a patched
// instance (or an entire deployment) that happens to have a MIDI mapping
// never leaves the show permanently unsaveable with
// GOLC_DESKMIDI_DANGLING_REFERENCE.
package deskmidi

import "github.com/lnorton89/golc/internal/deployment"

// ScrubDangling returns mappings with any entry whose InstanceID no longer
// resolves to a patch instance present in deployments removed -- this can
// only ever remove an already-broken reference, never reject a state
// Validate would otherwise accept.
func ScrubDangling(mappings []Mapping, deployments []deployment.Deployment) []Mapping {
	if len(mappings) == 0 {
		return mappings
	}
	instanceExists := make(map[string]bool)
	for _, d := range deployments {
		for _, instance := range d.Instances {
			instanceExists[instance.ID.String()] = true
		}
	}
	filtered := make([]Mapping, 0, len(mappings))
	for _, m := range mappings {
		if instanceExists[m.InstanceID] {
			filtered = append(filtered, m)
		}
	}
	return filtered
}
