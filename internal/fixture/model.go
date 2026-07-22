// model.go declares the canonical, capability-based fixture model (CONTEXT
// D-08: GDTF-friendly, not hard-wired to Open Fixture Library's
// channel/mode shape) that both the hand-authored-YAML decode path
// (decode.go, this plan) and the future OFL-import path (02-03) normalize
// into. Capability.Range values are normalized 0..1, never raw DMX, so the
// model stays protocol-agnostic (Art-Net today, other protocols/GDTF
// later).
package fixture

// CapabilityType names one semantically distinct fixture capability. The
// declared const set below is the exact enum FIXT-02 validates every
// decoded Capability.Type against
// (GOLC_FIXTURE_CAPABILITY_TYPE_UNSUPPORTED for anything else). A strobe,
// UV, or laser capability is never flattened into a generic intensity
// type: preserving the distinct CapabilityType here is what lets later
// output phases surface safety-relevant behavior (threat model
// prohibition).
type CapabilityType string

// The nine v1 capability types (CONTEXT D-05: PARs, washes, and
// moving-head spot/wash fixtures).
const (
	CapabilityIntensity CapabilityType = "intensity"
	CapabilityColor     CapabilityType = "color"
	CapabilityPan       CapabilityType = "pan"
	CapabilityTilt      CapabilityType = "tilt"
	CapabilityZoom      CapabilityType = "zoom"
	CapabilityFocus     CapabilityType = "focus"
	CapabilityGobo      CapabilityType = "gobo"
	CapabilityShutter   CapabilityType = "shutter"
	CapabilityStrobe    CapabilityType = "strobe"
)

// SupportedCapabilityTypes is the exact declared enum, in declaration
// order, decode.go validates every Capability.Type against.
var SupportedCapabilityTypes = []CapabilityType{
	CapabilityIntensity,
	CapabilityColor,
	CapabilityPan,
	CapabilityTilt,
	CapabilityZoom,
	CapabilityFocus,
	CapabilityGobo,
	CapabilityShutter,
	CapabilityStrobe,
}

// Capability is one declared, semantically typed behavior of a fixture,
// with its normalized [0,1] value range (protocol-agnostic: never raw
// DMX). A fixture may declare more than one Capability of the same Type
// to cover distinct, non-overlapping sub-ranges of that capability's value
// space (for example a shutter channel with separate "closed" and
// "strobe" sub-ranges); decode.go's validation allows adjacent (touching)
// same-type ranges but rejects an overlap.
type Capability struct {
	Type    CapabilityType `yaml:"type" json:"type" jsonschema:"required,description=Capability type; must be one of the declared CapabilityType enum values."`
	Range   [2]float64     `yaml:"range" json:"range" jsonschema:"required,description=Normalized [min max] value range within the 0 to 1 interval; never raw DMX."`
	Comment string         `yaml:"comment,omitempty" json:"comment,omitempty" jsonschema:"description=Optional human-readable note about this capability."`
}

// ChannelSlot names one entry in a Mode's ordered DMX channel layout
// (04-01-PLAN.md D-16): Type is the semantic CapabilityType this channel
// drives, and Occurrence is the 0-based index selecting which of the
// fixture's possibly-multiple same-Type Capabilities this channel
// corresponds to (see Capability's own doc comment on same-type
// sub-ranges); Occurrence: 0 selects the first/only one.
type ChannelSlot struct {
	Type       CapabilityType `yaml:"type" json:"type" jsonschema:"required,description=Capability type this channel drives; must be one of the declared CapabilityType enum values."`
	Occurrence int            `yaml:"occurrence" json:"occurrence" jsonschema:"minimum=0,description=0-based index selecting among the fixture's possibly-multiple same-Type Capabilities."`
}

// Mode is one named operating mode a fixture definition declares (for
// example a channel-count variant). Channels is the fixture's real DMX
// wiring order (D-16): channel offset i within the mode's addressed span
// is driven by Channels[i]'s named CapabilityType and Occurrence, in
// declared order -- GOLC never derives channel order from Capabilities'
// declaration order. A Mode declaring no Channels is a hard rejection at
// decode time (D-17, GOLC_FIXTURE_CHANNEL_LAYOUT_MISSING): v1 does not
// yet model per-mode capability subsets beyond this ordered layout;
// capabilities themselves are still declared once at the fixture level.
type Mode struct {
	Name     string        `yaml:"name" json:"name" jsonschema:"required,minLength=1,description=Mode name."`
	Channels []ChannelSlot `yaml:"channels" json:"channels" jsonschema:"required,minItems=1,description=Ordered DMX channel layout (D-16); each entry names the CapabilityType and same-type occurrence index this channel drives."`
}

// FixtureDefinition is the canonical, capability-based fixture model every
// fixture source (hand-authored YAML now, OFL import in 02-03 later)
// normalizes into. Capabilities preserves declared source order (a slice,
// never a map), so the canonical summary's capability order is stable and
// reflects the author's own YAML (FIXT-02 ordering probe).
type FixtureDefinition struct {
	SchemaVersion int          `yaml:"schema_version" json:"schema_version" jsonschema:"required,enum=1,description=Supported fixture schema version."`
	Manufacturer  string       `yaml:"manufacturer" json:"manufacturer" jsonschema:"required,minLength=1,description=Fixture manufacturer name."`
	Model         string       `yaml:"model" json:"model" jsonschema:"required,minLength=1,description=Fixture model name."`
	Modes         []Mode       `yaml:"modes" json:"modes" jsonschema:"required,minItems=1,description=Declared operating modes."`
	Capabilities  []Capability `yaml:"capabilities" json:"capabilities" jsonschema:"required,minItems=1,description=Declared capabilities in source order; must not be empty (GOLC_FIXTURE_EMPTY)."`
}
