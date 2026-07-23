# GOLC — Brand Guidelines

**Go Lighting Control** · Identity System v1.0
Source of truth: `GOLC Brand Guidelines.html` (interactive, light/dark).

> Cross-platform desktop lighting control with deterministic Art-Net playback,
> TypeScript automation, a public API, and autonomous LLM control — built in Go.

---

## 01 · Positioning

GOLC is desktop lighting control that behaves like an instrument: deterministic
Art-Net playback, scriptable in TypeScript, open by default — precise enough for
professionals, calm enough for a volunteer on a Sunday.

Built for operators of small live shows (clubs, churches, schools, community
venues). Reliability is the feature: cues fire the same way every time, state is
inspectable, nothing hides behind a dongle.

**Taglines**
- Lighting control that behaves. *(primary)*
- Deterministic lighting control.
- Run the room, not the rack.
- Open. Scriptable. Repeatable.

---

## 02 · Voice

Calm, reliable, professional. Speaks mainly to operators at small venues.

| Principle | Meaning |
|-----------|---------|
| Plain over clever | Say the true thing simply; an operator mid-show has no time to decode marketing. |
| Precise, never vague | Name the protocol, the channel, the frame. Specificity earns trust. |
| Reassuring under pressure | Calm tone, no exclamation. Steady when the room is loud. |
| Open and documented | Nothing hidden; every behavior written down, every claim verifiable. |

**Say:** "Deterministic Art-Net playback." · "Cues fire the same way every time." ·
"Scriptable in TypeScript." · "Free and open source." · "Runs on your machine."

**Avoid:** "Next-gen AI lighting revolution." · "Unleash your creativity!" ·
"Enterprise-grade synergy." · "Magical, effortless, seamless." · Exclamation-heavy hype.

---

## 03 · Logo

**Mark — "Beam."** An abstract, geometric spotlight: a fixture bar throwing a fan
of light rendered as a subtle six-band spectrum, on a rounded-square tile.

- **Primary lockup:** mark + `GOLC` wordmark, with `Go Lighting Control` set in mono below.
- **Secondary:** stacked / reversed for dark surfaces.
- **Mark alone:** app tiles, favicons, avatars.

**Rules**
- Clear space ≥ ¼ of the mark's height on all sides.
- Minimum size 28px (full mark); a simplified form is used at 16px favicon.
- Do not rotate, stretch, recolor, or place on busy backgrounds.
- In dark mode the tile flips to Paper with an Ink fixture bar; the spectrum is unchanged.

Spectrum beams (left→right): `#C0554A #CC8A47 #B6A24C #4E9E68 #1B44D9 #6A50A8`

---

## 04 · Color

Instrument-grade neutral: warm gray, ink black, one deep signal-blue accent.

### Core
| Token | Light | Role |
|-------|-------|------|
| Ink | `#17181C` | Primary surface, text, mark tile |
| Signal Blue | `#1B44D9` | The single accent — live state, focus, links, the beam |
| Paper | `#E4E0D8` | Warm light-gray canvas |

### Neutrals & support
`Panel #F4F1EB` · `Line #D2CCC0` · `Muted #8A887F` · `Slate #4A4941` ·
`Blue-deep #1233A8` · `Amber #C8A24B` *(restricted: in-app warnings / "live but unsaved" only — never in logo or marketing)*

### Dark theme tokens
`page #131419` · `panel #1E2027` · `ink #ECEAE3` · `text #B7B5AC` ·
`muted #87857D` · `line #2E3038`

Signal Blue is the sole brand accent in every theme.

---

## 05 · Typography

- **Archivo** — display & text. A grotesque with tight, engineered proportions. Weights 400–900.
- **JetBrains Mono** — technical & labels. Channel values, code, cue numbers, metadata.

| Role | Font / weight | Size |
|------|---------------|------|
| Display | Archivo 800 | 52px+ |
| Heading | Archivo 700 | 32px |
| Subhead | Archivo 600 | 22px |
| Body | Archivo 400 | 16px / 1.6 |
| Mono | JetBrains Mono 500 | 15px |

---

## 06 · Iconography

Line icons on a 24px grid, 1.75px stroke, square caps, 2px radius. Monochrome ink;
Signal Blue only to mark an active or live state. Set: fixture, faders, beam, cue,
script, api, timeline, blackout.

---

## 07 · App icon

macOS squircle (27/120 corner radius), Ink tile, centered stepped-beam spectrum,
Paper fixture bar. Flat and instrument-like — no bevels or inner gradients.
The shipped icon stays Ink in both themes.

---

## 08 · UI system & states

The app is dark by default — a control room, not a document. One typed command
model drives the UI, API, scripts, and AI, so state reads the same everywhere.
Color is never the only signal; each state pairs with a text label.

| State | Color | Meaning |
|-------|-------|---------|
| Live | `#1B44D9` | Art-Net output active, a look is on stage |
| Frame lock | `#5AC26A` | Playback holds a steady frame rate, isolated from the UI |
| Armed | `#C8A24B` | An automation lease is armed, or a look has unsaved edits |
| Revoked | `#E23A2E` | Revoke Automation has blocked AI/scripts; look frozen |
| Blackout | `#17181C` | Separate, immediate intensity kill (INTENSITY · 0) |
| Offline | `#8A887F` | A provider/integration is unreachable; local work continues |

**Operator authority** always belongs to the person in the room and is the
heaviest UI element: **Blackout** (immediate intensity kill) and
**Revoke Automation** (blocks AI/scripts, freezes the look). Never hidden in a menu.

---

## 09 · Product UI (planned v1)

Wails desktop app, Windows first. Representative views: author once, deploy
repeatedly.

- **Live output & playback** — deterministic Art-Net, frame-locked channel output.
- **Patch & pools** — modular fixture pools; substitution shows an Impact preview before commit.
- **Scene editor** — tempo-aware (BPM / tap), attribute groups (Intensity, Color, Position, Beam), chases.
- **AI & automation** — armed, time-bounded lease with an audit log; Revoke always available.
- **TypeScript** — sandboxed, capability-limited scripts against a generated typed SDK.

---

## 10 · Applications

GitHub social preview (1280×640), README header (1280×300), stickers/swag.
README banner uses the Paper canvas; social preview uses the Ink control-room field.

---

## 11 · Lexicon

Use consistently across UI, docs, and marketing.

- **Patch** — mapping logical fixtures to Art-Net addresses and modes.
- **Fixture pool** — a reusable logical group a show targets, independent of concrete count/address.
- **Look / Scene** — a stored state of attributes; tempo-aware scenes loop in bars.
- **Chase** — an ordered sequence of steps advanced on tempo.
- **Transition preset** — a reusable blend applied between looks.
- **Impact preview** — a deterministic plan shown before a pool resize or fixture substitution.
- **Soft takeover** — MIDI control that engages only once a fader matches the live value.
- **Art-Net** — the Ethernet DMX protocol GOLC outputs (Art-Net 4).
- **Universe** — 512 DMX channels carried over Art-Net.
- **Deterministic playback** — output timing isolated from the UI, scripts, and providers.
- **Lease** — a time-bounded, armed grant that lets an AI operate the app.
- **Revoke Automation** — the control that blocks AI/scripts and freezes the current look.
- **Blackout** — a separate, immediate intensity kill.
- **Command model** — one typed command set shared by the UI, API, scripts, and AI.

---

## 12 · Motion & interaction

1. **Output before motion** — animation is cosmetic and always yields; a transition never delays a frame.
2. **Motion is a state cue** — movement confirms arm, revoke, blackout, commit; never decoration.
3. **On tempo** — show motion follows BPM/bars; interface motion follows short, predictable eases.
4. **Restraint** — 120–200ms ease-outs. Nothing bounces, spins, or lingers.

**Timing tokens:** `snap · 0ms` · `tap · 120ms ease-out` · `settle · 200ms ease` · `frame · 25ms (40 fps)`
