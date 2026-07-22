// model.go declares the OFL (Open Fixture Library) JSON shape (FIXT-03):
// an intermediate representation sufficient to represent this plan's
// pinned test corpus (tests/fixtures/ofl -- a generic RGB PAR, an LED
// wash, a moving-head spot, and a moving-head wash) before normalize.go
// maps it onto GOLC's canonical fixture.FixtureDefinition (D-08: OFL's
// channel/mode shape never leaks downstream of this package).
//
// Decoding here is deliberately permissive (plain encoding/json.Unmarshal,
// not a strict/known-fields decode): every OFL field this struct does not
// model is simply ignored at decode time, and normalize.go is responsible
// for turning every unmodeled *construct* (an available/template channel,
// or one of its capability entries) into an explicit
// fixture.LossyImportWarning rather than silently dropping it
// (FIXT-06/D-06). Decode-time field strictness -- the discipline
// internal/fixture/decode.go rightly applies to hand-authored YAML --
// would defeat that goal here by rejecting any OFL fixture outside the v1
// target set instead of importing it with warnings, so it is intentionally
// not used for this format.
package ofl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Definition is the raw OFL fixture JSON shape
// (docs/fixture-format.md, github.com/OpenLightingProject/open-fixture-library),
// scoped to exactly what normalize.go needs: the display name, categories
// (informational only), fixture-level and template (pixel/matrix)
// channels, an optional pixel matrix marker, wheels (captured for
// structural completeness per this plan's Task 2 action -- it is the
// channels that *reference* a wheel via a WheelSlot/WheelRotation/
// WheelShake/WheelSlotRotation capability, not the wheel definition
// object itself, that normalize.go turns into a capability or a warning),
// and modes (name plus its ordered channel-key list, resolved into the
// canonical fixture.Mode.Channels field per D-16).
type Definition struct {
	Name              string                     `json:"name"`
	Categories        []string                   `json:"categories"`
	AvailableChannels map[string]Channel         `json:"availableChannels"`
	TemplateChannels  map[string]Channel         `json:"templateChannels,omitempty"`
	Matrix            *Matrix                    `json:"matrix,omitempty"`
	Wheels            map[string]json.RawMessage `json:"wheels,omitempty"`
	Modes             []Mode                     `json:"modes"`
}

// Matrix marks that OFL declares pixel-addressable channels (realized
// through TemplateChannels) for this fixture. Its exact pixelKeys/
// pixelGroups shape is never inspected -- presence alone is what
// normalize.go needs to know a pixel/matrix construct exists (D-06: an
// unmodeled-in-v1 construct that must surface as a warning, never a
// silent drop or a hard rejection).
type Matrix struct {
	PixelKeys json.RawMessage `json:"pixelKeys,omitempty"`
}

// Mode is one OFL operating mode. Channels is its ordered channel list:
// most entries are a plain channel-key string referencing the enclosing
// Definition's AvailableChannels/TemplateChannels, but OFL also allows a
// matrix/pixel expansion object (an "insert": "matrixChannels" directive)
// in this same array -- so each entry is captured as raw JSON and
// normalize.go's resolveModeChannels decodes it: a plain string resolves
// into a fixture.ChannelSlot (D-16); an expansion object is skipped at
// this mode-level resolution (its own per-pixel template channels are
// already surfaced as unmodeled-construct warnings via
// matrixChannelWarning's separate TemplateChannels walk). Previously this
// list had nothing to normalize into (fixture.Mode carried no
// channel-order field at all); D-16 makes it additive to the canonical
// model, so it is re-added here.
type Mode struct {
	Name     string            `json:"name"`
	Channels []json.RawMessage `json:"channels,omitempty"`
}

// Channel is one OFL availableChannels/templateChannels entry: either a
// single Capability (the common case for a simple channel) or an ordered
// Capabilities list keyed by dmxRange sub-ranges (for example a shutter
// channel with separate "closed"/"strobe" sub-ranges -- the same shape
// fixture.Capability's own doc comment describes for the canonical
// model). OFL declares exactly one of the two on any given channel.
type Channel struct {
	Capability   *Capability  `json:"capability,omitempty"`
	Capabilities []Capability `json:"capabilities,omitempty"`
}

// effectiveCapabilities returns c's capabilities in declared order,
// regardless of whether OFL expressed them as a single "capability" or a
// "capabilities" list, so every caller sees one ordered list.
func (c Channel) effectiveCapabilities() []Capability {
	if c.Capability != nil {
		return []Capability{*c.Capability}
	}
	return c.Capabilities
}

// Capability is one OFL capability entry. DMXRange is the 0..255 DMX
// sub-range this capability occupies within its channel -- present (as a
// two-element slice) for a multi-capability channel's entries and nil for
// a single-capability channel's implicit whole-channel capability. Type
// and ShutterEffect are the two fields normalize.go's mapping table keys
// off; every other OFL capability field (colors, speedStart/End,
// effectName, comment, soundControlled, ...) is display/behavioral
// metadata this v1 importer does not preserve to satisfy FIXT-03 -- an
// unmapped capability *type* itself (not its metadata fields) is what
// normalize.go's LossyImportWarning reports, so nothing about *why* a
// construct is unsupported is silently lost from the warning text, even
// though the metadata fields themselves are not carried into the
// canonical model.
type Capability struct {
	DMXRange      []int  `json:"dmxRange,omitempty"`
	Type          string `json:"type"`
	ShutterEffect string `json:"shutterEffect,omitempty"`
}

// decodeDefinition strictly decodes raw as JSON syntax (rejecting
// malformed JSON) and requires the minimal structural shape every real
// OFL fixture document has -- a non-empty name, at least one available
// channel, and at least one mode -- before returning it for
// normalize.go's field-by-field, warning-not-rejection mapping pass.
// GOLC_FIXTURE_OFL_INVALID is the sole diagnostic this function returns.
func decodeDefinition(raw []byte) (Definition, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Definition{}, fmt.Errorf("GOLC_FIXTURE_OFL_INVALID: OFL fixture document is empty")
	}
	var def Definition
	if err := json.Unmarshal(raw, &def); err != nil {
		return Definition{}, fmt.Errorf("GOLC_FIXTURE_OFL_INVALID: %v", err)
	}
	if strings.TrimSpace(def.Name) == "" {
		return Definition{}, fmt.Errorf("GOLC_FIXTURE_OFL_INVALID: OFL fixture document has no name")
	}
	if len(def.AvailableChannels) == 0 {
		return Definition{}, fmt.Errorf("GOLC_FIXTURE_OFL_INVALID: %s declares no availableChannels", def.Name)
	}
	if len(def.Modes) == 0 {
		return Definition{}, fmt.Errorf("GOLC_FIXTURE_OFL_INVALID: %s declares no modes", def.Name)
	}
	return def, nil
}
