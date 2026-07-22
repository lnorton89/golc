// normalize.go implements FIXT-03/FIXT-06's OFL-import normalization
// pipeline (D-06/D-08): map OFL's channel/capability shape onto GOLC's
// canonical, capability-based fixture.FixtureDefinition, run the exact
// same validation + pinning pipeline internal/fixture/decode.go's
// hand-authored YAML path uses (fixture.Validate, fixture.Pin -- never a
// second, independently-evolving copy of that logic), and turn every OFL
// construct the v1 canonical model does not represent into an explicit
// fixture.LossyImportWarning on the resulting Provenance rather than
// dropping it silently or rejecting the fixture outright.
//
// Capability mapping is deliberately fixture-level, not channel/DMX-level
// (D-08: the canonical model has no channel-index concept at all): every
// OFL channel that maps onto the same v1 fixture.CapabilityType
// contributes to exactly one canonical Capability per type, whose Range
// is the union of every contributing channel's normalized [0,1] DMX
// occupancy. This also sidesteps decode.go's same-type overlap rejection
// entirely, since at most one Capability per Type is ever produced.
package ofl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lnorton89/golc/internal/fixture"
)

// oflSchemaVersion is the fixture.FixtureDefinition.SchemaVersion every
// OFL import produces -- OFL fixtures carry no schema_version field of
// their own that corresponds to GOLC's fixture schema; the normalized
// result always declares GOLC's current (and, in v1, only) supported
// version.
const oflSchemaVersion = 1

// dmxMax is the top of OFL's 8-bit DMX value range, the denominator every
// dmxRange sub-range is normalized against to land in fixture.Capability's
// [0,1] Range.
const dmxMax = 255.0

// shutterEffectStates is the set of OFL ShutterStrobe shutterEffect
// values this importer treats as a static shutter aperture state
// (fixture.CapabilityShutter) rather than a dynamic, timed effect
// (fixture.CapabilityStrobe) -- OFL's own capability-types.md documents
// both families under the single "ShutterStrobe" capability type, so the
// distinction GOLC's canonical model draws between the two is resolved
// here, once, from shutterEffect.
var shutterEffectStates = map[string]bool{
	"Open":   true,
	"Closed": true,
	"Frost":  true,
	"Iris":   true,
}

// Normalize maps raw OFL fixture JSON onto GOLC's canonical
// FixtureDefinition, running it through the same fixture.Validate +
// fixture.Pin pipeline internal/fixture/decode.go's hand-authored YAML
// path uses, and returns a Provenance whose Source records the OFL
// origin and whose Warnings lists every OFL construct the v1 canonical
// model does not represent (FIXT-06/D-06 -- never a silent drop, never a
// hard rejection for an out-of-v1-target-set construct). source is the
// fixture's stable "<manufacturer>/<fixture-key>" identity (mirroring
// OFL's own fixtures/<manufacturer>/<key>.json repository layout); both
// call sites in this repository -- OFLRef.Source() for a live/mirror
// fetch and the command layer's --ofl-file filename convention -- supply
// it in that exact shape.
func Normalize(raw []byte, source string) (fixture.FixtureDefinition, fixture.Provenance, error) {
	def, err := decodeDefinition(raw)
	if err != nil {
		return fixture.FixtureDefinition{}, fixture.Provenance{}, err
	}

	canonical := fixture.FixtureDefinition{
		SchemaVersion: oflSchemaVersion,
		Manufacturer:  manufacturerFromSource(source),
		Model:         def.Name,
		Modes:         canonicalModes(def.Modes),
	}

	ranges := map[fixture.CapabilityType][2]float64{}
	var warnings []fixture.LossyImportWarning

	for _, name := range sortedChannelNames(def.AvailableChannels) {
		warnings = append(warnings, normalizeChannel(name, def.AvailableChannels[name], ranges)...)
	}
	for _, name := range sortedChannelNames(def.TemplateChannels) {
		// Every template (pixel/matrix) channel construct is unmodeled in
		// v1 regardless of its own capability type: per-pixel addressing
		// is a structurally different capability shape than the flat,
		// fixture-level Capabilities list the canonical model declares,
		// so folding a template channel's ColorIntensity/Intensity/etc.
		// into the same fixture-level capability a plain channel produces
		// would silently misrepresent "N independently addressable
		// pixels" as "one fixture-wide capability" (D-06 forbids exactly
		// this kind of silent lossy merge).
		warnings = append(warnings, matrixChannelWarning(name))
	}

	canonical.Capabilities = capabilitiesFromRanges(ranges)

	if err := fixture.Validate(canonical); err != nil {
		return fixture.FixtureDefinition{}, fixture.Provenance{}, err
	}
	identity, err := fixture.Pin(canonical)
	if err != nil {
		return fixture.FixtureDefinition{}, fixture.Provenance{}, err
	}

	provenance := fixture.NewProvenance(canonical, identity, "ofl:"+source)
	if warnings == nil {
		warnings = []fixture.LossyImportWarning{}
	}
	provenance.Warnings = warnings

	return canonical, provenance, nil
}

// manufacturerFromSource extracts the manufacturer key from source's
// canonical "<manufacturer>/<fixture-key>" shape. A source with no "/" is
// used verbatim as the manufacturer key (defensive default; every actual
// call site in this repository always supplies the man/key form).
func manufacturerFromSource(source string) string {
	if idx := strings.IndexByte(source, '/'); idx >= 0 {
		return source[:idx]
	}
	return source
}

// canonicalModes projects OFL's mode list onto fixture.Mode, preserving
// declared order (mirrors fixture.FixtureDefinition.Capabilities' own
// source-order-preservation discipline).
func canonicalModes(modes []Mode) []fixture.Mode {
	result := make([]fixture.Mode, 0, len(modes))
	for _, mode := range modes {
		result = append(result, fixture.Mode{Name: mode.Name})
	}
	return result
}

// sortedChannelNames returns channels' keys in deterministic (sorted)
// order, so repeated normalization of the identical input always walks
// channels in the identical order and produces byte-identical Warnings
// ordering -- Go map iteration order is randomized, and an OFL author's
// own declared field order is not preserved through encoding/json's
// map[string]Channel decode.
func sortedChannelNames(channels map[string]Channel) []string {
	names := make([]string, 0, len(channels))
	for name := range channels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// normalizeChannel maps one OFL channel's capability entries: each
// capability that maps onto a v1 fixture.CapabilityType merges into
// ranges; each that does not becomes an explicit LossyImportWarning
// naming the channel and the unrepresented OFL construct (FIXT-06/D-06).
func normalizeChannel(name string, channel Channel, ranges map[fixture.CapabilityType][2]float64) []fixture.LossyImportWarning {
	capabilities := channel.effectiveCapabilities()
	if len(capabilities) == 0 {
		return []fixture.LossyImportWarning{{
			Severity: "warning",
			Detail:   fmt.Sprintf("channel %q declares no capability", name),
		}}
	}

	var warnings []fixture.LossyImportWarning
	for _, capability := range capabilities {
		capabilityType, ok := mapCapabilityType(name, capability)
		if !ok {
			warnings = append(warnings, fixture.LossyImportWarning{
				Severity: "warning",
				Detail:   unmappedCapabilityDetail(name, capability),
			})
			continue
		}
		mergeRange(ranges, capabilityType, normalizedRange(capability.DMXRange))
	}
	return warnings
}

// mapCapabilityType maps one OFL capability (in the context of its
// enclosing channel's name, needed only to disambiguate a wheel-based
// capability belonging to a gobo wheel from one belonging to a color
// wheel -- OFL's WheelSlot/WheelRotation/WheelShake/WheelSlotRotation
// types carry no channel-independent semantic of their own) onto a v1
// fixture.CapabilityType. ok is false for every OFL construct this v1
// importer does not represent; the caller turns that into an explicit
// LossyImportWarning, never a silent drop.
func mapCapabilityType(channelName string, capability Capability) (fixture.CapabilityType, bool) {
	switch capability.Type {
	case "Intensity":
		return fixture.CapabilityIntensity, true
	case "ColorIntensity":
		return fixture.CapabilityColor, true
	case "Pan":
		return fixture.CapabilityPan, true
	case "Tilt":
		return fixture.CapabilityTilt, true
	case "Zoom":
		return fixture.CapabilityZoom, true
	case "Focus":
		return fixture.CapabilityFocus, true
	case "ShutterStrobe":
		return mapShutterStrobe(capability.ShutterEffect)
	case "WheelSlot", "WheelRotation", "WheelShake", "WheelSlotRotation":
		if isGoboChannel(channelName) {
			return fixture.CapabilityGobo, true
		}
		return "", false
	default:
		// NoFunction, Effect, EffectSpeed, EffectDuration, ColorPreset,
		// Maintenance, PanTiltSpeed, Prism, and every other OFL
		// capability type v1 does not model a canonical equivalent for.
		return "", false
	}
}

// mapShutterStrobe resolves OFL's single "ShutterStrobe" capability type
// onto GOLC's two distinct canonical types: a static aperture state
// (Open/Closed/Frost/Iris) maps to CapabilityShutter; a dynamic, timed
// effect (Strobe/Pulse/RampUp/RampDown/RampUpDown/Lightning/... or any
// value this importer does not specifically recognize as a static state)
// maps to CapabilityStrobe. An empty shutterEffect (which no
// well-formed OFL ShutterStrobe capability declares) is unmapped rather
// than guessed.
func mapShutterStrobe(shutterEffect string) (fixture.CapabilityType, bool) {
	if shutterEffect == "" {
		return "", false
	}
	if shutterEffectStates[shutterEffect] {
		return fixture.CapabilityShutter, true
	}
	return fixture.CapabilityStrobe, true
}

// isGoboChannel reports whether name plausibly names a gobo wheel/gobo
// rotation channel (case-insensitive substring match), the only context
// in which this importer maps a wheel-based capability (WheelSlot/
// WheelRotation/WheelShake/WheelSlotRotation) onto the canonical gobo
// capability -- a color wheel's identically-typed capabilities stay
// unmapped (a discrete wheel-slot color selection is not the same
// semantic as ColorIntensity's continuous RGB control, so treating it as
// one would misrepresent fixture behavior rather than just losing
// display metadata).
func isGoboChannel(name string) bool {
	return strings.Contains(strings.ToLower(name), "gobo")
}

// unmappedCapabilityDetail renders a human-readable LossyImportWarning
// detail naming the channel, the unrepresented OFL capability type, and
// (for ShutterStrobe) the specific shutterEffect that went unmapped.
func unmappedCapabilityDetail(channelName string, capability Capability) string {
	if capability.Type == "ShutterStrobe" && capability.ShutterEffect != "" {
		return fmt.Sprintf(
			"channel %q capability type %q (shutterEffect %q) is not represented in the v1 canonical model",
			channelName, capability.Type, capability.ShutterEffect)
	}
	return fmt.Sprintf(
		"channel %q capability type %q is not represented in the v1 canonical model",
		channelName, capability.Type)
}

// matrixChannelWarning renders the LossyImportWarning for one template
// (pixel/matrix) channel construct (D-06: pixel/matrix fixtures still
// import, with their unmodeled constructs surfaced as warnings).
func matrixChannelWarning(templateChannelName string) fixture.LossyImportWarning {
	return fixture.LossyImportWarning{
		Severity: "warning",
		Detail: fmt.Sprintf(
			"template channel %q is a pixel/matrix construct; per-pixel addressing is not represented in the v1 canonical model (D-06)",
			templateChannelName),
	}
}

// normalizedRange converts an OFL dmxRange (present on a multi-capability
// channel's entries) into fixture.Capability's normalized [0,1] Range. A
// nil dmxRange (a single-capability channel's implicit whole-channel
// capability) occupies the entire channel: [0, 1].
func normalizedRange(dmxRange []int) [2]float64 {
	if len(dmxRange) != 2 {
		return [2]float64{0, 1}
	}
	return [2]float64{float64(dmxRange[0]) / dmxMax, float64(dmxRange[1]) / dmxMax}
}

// mergeRange folds r into ranges' entry for capabilityType by union
// (widening the low/high bound), so every OFL channel/capability that
// maps onto the same canonical type still produces exactly one
// Capability per type in the final result -- never two same-type entries
// that could overlap and trip decode.go's rejectOverlappingRanges check.
func mergeRange(ranges map[fixture.CapabilityType][2]float64, capabilityType fixture.CapabilityType, r [2]float64) {
	existing, ok := ranges[capabilityType]
	if !ok {
		ranges[capabilityType] = r
		return
	}
	if r[0] < existing[0] {
		existing[0] = r[0]
	}
	if r[1] > existing[1] {
		existing[1] = r[1]
	}
	ranges[capabilityType] = existing
}

// capabilitiesFromRanges renders ranges into fixture.Capability entries in
// fixture.SupportedCapabilityTypes' declared order, mirroring
// decode.go's own stable-order convention so a normalized OFL import's
// capability ordering is as deterministic as a hand-authored fixture's.
func capabilitiesFromRanges(ranges map[fixture.CapabilityType][2]float64) []fixture.Capability {
	var capabilities []fixture.Capability
	for _, capabilityType := range fixture.SupportedCapabilityTypes {
		r, ok := ranges[capabilityType]
		if !ok {
			continue
		}
		capabilities = append(capabilities, fixture.Capability{Type: capabilityType, Range: r})
	}
	return capabilities
}
