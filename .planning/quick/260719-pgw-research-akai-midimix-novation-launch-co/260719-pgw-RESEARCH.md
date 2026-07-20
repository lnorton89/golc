# MIDI Controller Research: MIDImix, Launch Control XL Mk2, and EasyControl 9

**Researched:** 2026-07-19
**Decision confidence:** MEDIUM - manufacturer manuals establish the documented protocol surface, but no physical controller, firmware revision, Windows MIDI endpoint, or packaged GOLC build was exercised in this research.

## Decision

**Primary recommendation:** Record Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together as the selected Phase 6 physical acceptance set for generic MIDI Note/CC learn and soft-takeover qualification, with no hierarchy among the three members.

This resolves the selection question in MIDI-HW-01, but selection does **not** make any member compatible or supported. Each controller must pass an independent MIDI-HW-02 evidence set for the exact hardware revision, firmware, Windows version, and GOLC build before GOLC makes a named compatibility or support claim. Phase 6 may use the set to qualify the existing generic Note/CC learn and soft-takeover requirements. Prebuilt device profiles, automatic template programming, SysEx initialization, packaged controller assets, and device-specific feedback remain in EXTN-04/v1.x until the applicable physical evidence passes and the roadmap is deliberately changed. [VERIFIED: .planning/REQUIREMENTS.md:70-71,152; .planning/ROADMAP.md:93-107; .planning/research/FEATURES.md:32,125,135]

The manuals establish different evidence surfaces without assigning different planning roles. Akai documents a mixer-shaped bidirectional MIDI surface but not its default wire map or LED protocol. Novation documents class-compliant USB, templates, bidirectional template selection/reporting, and a detailed LED/SysEx protocol. Worlde documents configurable banks, Note/CC/MMC output, and editor fields, while leaving runtime feedback, exact defaults, bank observability, and current Windows behavior to physical probing. [VERIFIED: Akai manual, PDF pp. 3-5,18; Novation programmer guide, PDF pp. 3-9; Novation GSG, PDF pp. 3-6; Worlde manual, PDF pp. 3-9]

## 1. Sources Reviewed

### Primary controller manuals

- [**Akai-MIDImix-UserGuide-v1.0.pdf**](../../midi/Akai-MIDImix-UserGuide-v1.0.pdf) - English setup, controls, USB behavior, Send All/pickup warning, and specifications on PDF pp. 3-5 and 18. PDF metadata identifies Akai Professional as author. [VERIFIED: local PDF metadata and cited pages]
- [**launch_control_xl_programmer_s_reference_guide.pdf**](../../midi/launch_control_xl_programmer_s_reference_guide.pdf) - MIDI overview, LED messages, Launchpad-compatible messages, SysEx, template change, toggle state, reset, brightness/color values, flashing, and double buffering on PDF pp. 3-9. [VERIFIED: cited manual pages]
- [**Novation-Launch Control XL GSG v2.pdf**](<../../midi/Novation-Launch Control XL GSG v2.pdf>) - physical controls, class-compliant USB, Mac/PC/iPad power mode, templates, and editor behavior on PDF pp. 2-6. [VERIFIED: cited manual pages]
- [**Worlde-EasyControl-9-UserManual.pdf**](../../midi/Worlde-EasyControl-9-UserManual.pdf) - physical controls, USB/platform notes, editor fields, value ranges, bank behavior, fixed controls, button modes, MMC, and specifications on PDF pp. 3-9. [VERIFIED: cited manual pages]

### Visual verification

- Temporary contact sheets rendered outside the repository were reviewed for every page. Visual review confirmed control layouts and diagrams that text extraction did not fully preserve, especially the Worlde fixed-control block and the Novation knob-LED note diagram. Claims below cite the originating PDF page rather than the temporary render. [VERIFIED: temporary renders compared with all four source PDFs]

### Project and planning sources

- AGENTS.md; .planning/PROJECT.md; .planning/REQUIREMENTS.md; .planning/ROADMAP.md; .planning/STATE.md; .planning/research/FEATURES.md; .planning/research/ARCHITECTURE.md; .planning/research/PITFALLS.md; Phase 1 CONTEXT.md and VALIDATION.md. [VERIFIED: repository inspection on 2026-07-19]

No external web source was needed. Where a supplied manual does not document a property, this report records it as an acceptance unknown instead of filling it from an unofficial mapping.

## 2. Controller Capability Matrix

| Capability | Akai MIDImix | Novation Launch Control XL Mk2 | Worlde EasyControl 9 |
|---|---|---|---|
| Physical continuous controls | 24 assignable 270-degree knobs; 8 channel faders and 1 master fader, all 30 mm. [VERIFIED: Akai manual, PDF pp. 5,18] | 24 center-detent rotary pots and 8 faders. [VERIFIED: Novation GSG, PDF p. 3] | 9 assignable knobs and 9 assignable sliders, plus one fixed program-change knob and one uneditable general slider. [VERIFIED: Worlde manual, PDF pp. 3-5] |
| Physical buttons | 8 amber-backlit Mute, 1 Solo modifier, 8 red-backlit Record Arm, Bank Left/Right, and Send All: 20 buttons total. [VERIFIED: Akai manual, PDF pp. 5,18] | 16 channel buttons, 4 direction buttons, 4 Device/Mute/Solo/Record Arm buttons, plus User and Factory template selectors: 26 physical buttons, of which 24 are protocol-addressable in the programmer guide. [VERIFIED: Novation programmer guide, PDF pp. 3,7; GSG, PDF p. 3] | 9 group buttons plus 2 assignable buttons, 6 transport buttons, a bank switch, and 2 fixed CC buttons; the manual advertises 11 assignable buttons. [VERIFIED: Worlde manual, PDF pp. 3-5] |
| USB, power, platform | One USB port; USB carries MIDI both directions and bus power; a powered hub is required if a hub is used. The manual does not explicitly use the term class-compliant. [VERIFIED: Akai manual, PDF pp. 4-5,18] | Class-compliant USB MIDI, no computer driver required; Mac/PC and iPad use are documented. iPad operation uses a remembered low-power mode; USB supplies power. [VERIFIED: Novation GSG, PDF p. 4] | Mini-B USB 2.0 full-speed, bus powered, 100 mA or less; manual claims driver-free hot plug for listed Windows versions through Windows 10 and Mac OS X. Windows 11 is not documented. [VERIFIED: Worlde manual, PDF pp. 3,5,9] |
| MIDI port / direction | Ableton setup requires both Input and Output set to MIDI Mix; USB is documented as sending and receiving MIDI. [VERIFIED: Akai manual, PDF pp. 4-5] | One MIDI port named Launch Control XL n, where n is the device ID and is omitted for device ID 1. Full computer-to-device and device-to-computer protocols are documented. [VERIFIED: Novation programmer guide, PDF p. 3] | The manual documents controls transmitting to the computer and an editor, but does not document a runtime host-to-device feedback port or message protocol. [VERIFIED: Worlde manual, PDF pp. 3-9] |
| Templates / banks | Bank Left/Right shifts the eight controlled channels; the manual points to MIDImix Editor but does not state bank count, default addresses, or bank-change messages. [VERIFIED: Akai manual, PDF pp. 3,5,18] | 16 templates: user 0-7 are editable and factory 8-15 are fixed. Hardware selects either family and pads 1-8; SysEx can select any template and the device reports a change. [VERIFIED: Novation programmer guide, PDF pp. 3,8; GSG, PDF p. 5] | Four editable banks with a selected-bank LED. Bank assignments are edited in the Worlde editor; no bank-change MIDI notification is documented. [VERIFIED: Worlde manual, PDF pp. 3-6] |
| Documented default output | Knobs and faders send continuous-controller messages. Exact channels, CC numbers, button messages, and values are not tabulated in the supplied manual. [VERIFIED: Akai manual, PDF p. 5] | Factory templates output a fixed set of CCs for pots/mode buttons and Notes for pads; user templates output editable CCs and Notes. Exact factory address tables are not included in the supplied guides. [VERIFIED: Novation GSG, PDF p. 5] | Scene, transport, and group channels are configurable from 1-16. Knobs/sliders use configurable CC 0-127; assignable buttons use Note C-1..G9 or CC 0-127. Two fixed buttons use controller numbers 64 and 67; the fixed knob sends Program Change. [VERIFIED: Worlde manual, PDF pp. 5-8] |
| Absolute / relative behavior | Physical 270-degree knobs and faders are documented; no relative mode is documented. End values require capture. [VERIFIED: Akai manual, PDF pp. 5,18] | Physical pots/faders and CC output are documented; no relative encoder mode is documented. Treat a learned control as absolute only after capture confirms its sequence and range. [VERIFIED: Novation GSG, PDF pp. 3,5] | Knob left/right and slider upper/lower endpoint values are independently configurable 0-127, so reverse ranges can be represented. No relative mode is documented. [VERIFIED: Worlde manual, PDF pp. 6-7] |
| Momentary / toggle behavior | Mute/record/solo use is described as DAW behavior, but wire-level press/release or toggle behavior is not defined. [VERIFIED: Akai manual, PDF p. 5] | Buttons may emit Note or CC; normal press is 127 and release is 0, and the editor can change press/release values. Toggle buttons have a dedicated state-setting SysEx. [VERIFIED: Novation programmer guide, PDF pp. 7-8] | Assignable and transport CC buttons support Momentary or Toggle. Transport buttons may instead send one of 13 MMC commands. [VERIFIED: Worlde manual, PDF pp. 7-9] |
| Runtime LED feedback | Mute and Record Arm buttons are backlit and USB receives MIDI, but the supplied manual does not define host LED messages, colors, reset, or resync. [VERIFIED: Akai manual, PDF pp. 5,18] | Complete LED control: red/green mix, off/low/medium/full, normal/flashing values, per-template SysEx, Launchpad Note/CC compatibility, reset, all-on test, state retention per template, and two buffers. [VERIFIED: Novation programmer guide, PDF pp. 3-9] | Selected-bank indication is documented. No host-controlled button LED, color, brightness, flashing, reset, or feedback message is documented. [VERIFIED: Worlde manual, PDF pp. 4-5] |
| Phase 6 planning role | **Selected Phase 6 physical acceptance-set member**; this role does not change scope, and default addresses, button behavior, banks, and feedback remain independent MIDI-HW-02 probes. | **Selected Phase 6 physical acceptance-set member**; this role does not change scope, generic Note/CC qualification is in scope, and the documented SysEx profile remains under independent MIDI-HW-02 evidence and EXTN-04/v1.x. | **Selected Phase 6 physical acceptance-set member**; this role does not change scope, and current Windows, bank observability, defaults, editor behavior, and feedback remain independent MIDI-HW-02 probes. |

## 3. Per-Device Findings

### 3.1 Akai MIDImix

Akai MIDImix is one of the three equal selected Phase 6 hardware acceptance targets for generic Note/CC learn and soft-takeover qualification alongside Novation Launch Control XL Mk2 and Worlde EasyControl 9.

#### Controls and connection

- The device has 24 assignable 270-degree knobs, eight 30 mm channel faders, one 30 mm master fader, eight amber-backlit Mute buttons, one Solo modifier, eight red-backlit Record Arm buttons, Bank Left/Right, and Send All. [VERIFIED: Akai manual, PDF pp. 5,18]
- USB carries MIDI to and from the computer and supplies power. Ableton configuration explicitly selects MIDI Mix for both input and output. [VERIFIED: Akai manual, PDF pp. 4-5]
- The supplied guide names Windows and Mac OS X only in Ableton menu instructions; it does not publish a current supported-OS matrix or explicitly state USB class compliance. [VERIFIED: Akai manual, PDF p. 4]

#### Messages, banks, editor, and values

- Knobs, channel faders, and the master fader send continuous-controller messages. The manual does not list their default MIDI channel, CC numbers, endpoint values, or resolution. [VERIFIED: Akai manual, PDF p. 5]
- The manual describes Mute, Solo, Record Arm, Bank Left/Right, and Send All by DAW function, but does not define each button's Note/CC type, number, channel, press/release values, or toggle ownership. [VERIFIED: Akai manual, PDF p. 5]
- Bank Left/Right shifts the eight controlled channels. The guide does not state whether bank changes emit a distinct message or only change later control addresses. [VERIFIED: Akai manual, PDF p. 5]
- Akai directs users to download MIDImix Editor Software, but the supplied guide does not document editor fields, template count, persistence, SysEx, reset, or factory mappings. [VERIFIED: Akai manual, PDF p. 3]

#### Feedback and state synchronization

- Send All sends the current controller settings to software. The manual warns that software pickup/takeover can make this appear to do nothing. [VERIFIED: Akai manual, PDF p. 5]
- The backlit Mute and Record Arm buttons and bidirectional USB make host feedback plausible, but no wire protocol is supplied; it is therefore not a supportable claim without a host-output capture. [VERIFIED: Akai manual, PDF pp. 5,18]
- GOLC should never apply a Send All burst directly to live targets on connect. It should capture the burst as physical-position state, arm pickup, and apply only after each absolute control crosses its current software value. [RECOMMENDED: derived from PLAY-05 and the manual's pickup warning]

#### Mapping and test limitations

- Default CC/Note addresses, button release behavior, LED control, bank identity, editor persistence, power-cycle state, and reconnect behavior all remain hardware acceptance items.
- Until those probes pass, the defensible statement is that MIDImix is a member of the selected physical acceptance set with physical evidence pending. It must not be called compatible, a supported profile, or a feedback-capable controller.

### 3.2 Novation Launch Control XL Mk2

Novation Launch Control XL Mk2 is one of the three equal selected Phase 6 hardware acceptance targets for generic Note/CC learn and soft-takeover qualification alongside Akai MIDImix and Worlde EasyControl 9.

Novation has the most thoroughly documented supplied LED/SysEx protocol. This documentation difference creates no priority, compatibility, or support implication.

#### Controls and connection

- The device has 24 center-detent pots, eight faders, 16 channel buttons, four direction buttons, four Device/Mute/Solo/Record Arm buttons, and two template-family selectors. [VERIFIED: Novation GSG, PDF p. 3; programmer guide, PDF p. 3]
- Sixteen channel buttons use red/green elements that mix to amber; direction buttons have red LEDs; Device/Mute/Solo/Record Arm have yellow LEDs. The pot positions are also LED-addressable by index. [VERIFIED: Novation programmer guide, PDF pp. 3,7]
- It is a class-compliant USB MIDI device with no computer driver required. Mac/PC and iPad are documented; the iPad requires a Camera Connection Kit and a remembered low-power mode selected at cable insertion. [VERIFIED: Novation GSG, PDF p. 4]
- It exposes one MIDI port named Launch Control XL n, with n derived from device ID. [VERIFIED: Novation programmer guide, PDF p. 3]

#### Templates, output, and default-map limits

- There are eight editable user templates in slots 0-7 and eight non-editable factory templates in slots 8-15. Hardware template selection holds User or Factory and presses pad 1-8. [VERIFIED: Novation programmer guide, PDF p. 3; GSG, PDF p. 5]
- Factory templates send fixed CCs for pots/mode buttons and Notes for pads. User templates send editable CCs and Notes and store chosen pot-LED colors. The supplied guides do not provide the exact factory CC/Note table, so acceptance must capture it rather than rely on a third-party map. [VERIFIED: Novation GSG, PDF p. 5]
- Buttons can emit Note On or CC. The documented default event form is value 127 on press and 0 on release; the editor can change note/CC and press/release velocity. [VERIFIED: Novation programmer guide, PDF p. 8]
- No relative-encoder mode is documented. Pots and faders should be learned as absolute controls only after capture confirms monotonic values and endpoints. [VERIFIED: Novation GSG, PDF pp. 3,5]

#### SysEx, template selection, and feedback

- Set LEDs on any template with: F0 00 20 29 02 11 78, followed by Template and one or more Index/Value pairs, then F7. Template is 0-7 user or 8-15 factory. LED indices 0-23 address pot LEDs, 24-39 the two channel-button rows, 40-43 Device/Mute/Solo/Record Arm, and 44-47 Up/Down/Left/Right. [VERIFIED: Novation programmer guide, PDF p. 7]
- Set toggle state with: F0 00 20 29 02 11 7B Template Index Value F7, where Value is 0 off or 127 on; non-toggle buttons ignore it. Multiple Index/Value pairs are allowed. [VERIFIED: Novation programmer guide, PDF p. 7]
- Select a template with: F0 00 20 29 02 11 77 Template F7. The device emits the same message when its template changes. [VERIFIED: Novation programmer guide, PDF p. 8]
- Launchpad-compatible Note On/Off can light controls in the currently selected template only when note/CC and channel match. The guide recommends SysEx because it addresses any template independently of its learned values. [VERIFIED: Novation programmer guide, PDF pp. 3-5]
- The knob-LED Note diagram assigns three consecutive notes to each physical column: decimal 1-3, 17-19, 33-35, 49-51, 65-67, 81-83, 97-99, and 113-115 from columns 1 through 8. The printed note names are C#-1/D-1/D#-1, F0/F#0/G0, A1/A#1/B1, C#3/D3/D#3, F4/F#4/G4, A5/A#5/B5, C#7/D7/D#7, and F8/F#8/G8. These are LED-lighting addresses, not a complete factory control-output map. [VERIFIED: Novation programmer guide, PDF p. 8, visually checked against the rendered page]
- Red and green each have brightness 0-3. Normal-use examples are 12 off, 13 low red, 15 full red, 29 low amber, 63 full amber, 62 full yellow, 28 low green, and 60 full green; flashing examples include 11 red, 59 amber, 58 yellow, and 56 green. [VERIFIED: Novation programmer guide, PDF pp. 4-5]
- Per-template reset is CC 0 value 0 on the template-number channel; all-on tests use CC 0 values 125-127. Double-buffer control uses CC 0 values 32-61, and the guide supplies buffer-swap and flash sequences. [VERIFIED: Novation programmer guide, PDF pp. 5-6,9]
- LED state is retained per template and restored when reselected. Background SysEx can update inactive templates. Two buffers permit atomic-looking large refreshes and automatic flashing; the guide notes a large update may take about 100 ms to prepare. [VERIFIED: Novation programmer guide, PDF pp. 3,9]

#### Mapping and test limitations

- The manuals specify the protocol but not the exact unit firmware, Windows endpoint identity, factory control address table, editor version/persistence, physical control noise/deadband, or whether a given Mk2 unit behaves identically after suspend/resume.
- Phase 6 qualification uses generic Note/CC learn with an explicitly chosen template. The SysEx codec is valuable design input, but automatic template selection, state repaint, toggle synchronization, and background LED buffers remain a later device-profile capability unless independent MIDI-HW-02 evidence passes and EXTN-04 is explicitly promoted.

### 3.3 Worlde EasyControl 9

Worlde EasyControl 9 is one of the three equal selected Phase 6 hardware acceptance targets for generic Note/CC learn and soft-takeover qualification alongside Akai MIDImix and Novation Launch Control XL Mk2.

#### Controls and connection

- The advertised editable surface comprises nine knobs, nine sliders, eleven assignable buttons, six transport buttons, and four banks. The layout groups one knob, slider, and button into each of nine control groups, with two additional assignable buttons. [VERIFIED: Worlde manual, PDF pp. 3-4]
- A separate fixed block includes a non-editable Program Change knob, two non-editable buttons using controller numbers 64 and 67, and one uneditable general slider. [VERIFIED: Worlde manual, PDF p. 5]
- USB is mini-B, USB 2.0 full-speed, bus powered, and consumes no more than 100 mA. The manual claims driver-free hot plug for Windows 10/8/7/XP/Vista and Mac OS X, but does not cover Windows 11. [VERIFIED: Worlde manual, PDF pp. 3,5,9]

#### Editor, banks, messages, and values

- Four banks hold different controller assignments, and the selected bank LED lights. Assignments are edited with the Worlde software editor rather than from the device. [VERIFIED: Worlde manual, PDF pp. 4-6]
- Scene MIDI channel is configurable 1-16. Transport and each group can use 1-16 or inherit the scene channel. [VERIFIED: Worlde manual, PDF p. 6]
- Each assignable knob can be enabled/disabled, assigned CC 0-127, and given independent left/right values 0-127. Each slider has the same controls with upper/lower values 0-127. [VERIFIED: Worlde manual, PDF pp. 6-7]
- Assignable buttons can be disabled or assigned Note/CC, with Note C-1..G9, CC 0-127, On 0-127, and Off 0-127 for CC. Momentary sends On at press and Off at release; Toggle alternates them on each press. [VERIFIED: Worlde manual, PDF pp. 7-8]
- Transport buttons can send CC or MMC, and CC behavior can be momentary or toggle. MMC offers 13 named commands and device ID 0-127, with 127 broadcast to all devices. [VERIFIED: Worlde manual, PDF pp. 8-9]
- The manual does not tabulate the factory/default assignments for the editable controls or the controller number of the fixed general slider. [VERIFIED: Worlde manual, PDF pp. 5-9]

#### Feedback, bank identity, and test limitations

- Only the selected-bank LED is behaviorally documented. No host-to-device runtime feedback, button LED addressing, SysEx format, template select message, bank change notification, brightness/color/flashing control, or reset is documented. [VERIFIED: Worlde manual, PDF pp. 4-9]
- A four-bank design is useful only if GOLC can deterministically distinguish banked raw addresses or the operator can bind one bank explicitly. If two banks emit identical raw addresses and no bank event exists, GOLC must reject ambiguous simultaneous mappings rather than guess. [RECOMMENDED: deterministic conflict policy based on the documented absence of bank notification]
- Editor availability on current Windows, power-cycle persistence, endpoint naming, exact default messages, bank address behavior, and any bidirectional USB MIDI capability remain physical probes.

## 4. Generic MIDI Profile Contract

The contract below keeps generic input learning in Phase 6 while providing a stable extension seam for later device profiles. It follows the project's rule that all adapters submit shared domain commands and never own playback state or timing. [VERIFIED: .planning/PROJECT.md:66-69,87-95; .planning/research/ARCHITECTURE.md sections "Adapters Sharing the Same Capabilities" and "Concurrency and Mutable-State Ownership"]

### 4.1 Data contract

| Contract element | Required representation and rule |
|---|---|
| Profile identity | Stable profile ID, schema version, vendor, product, hardware revision range if known, firmware range if known, and support status: experimental, acceptance-target, or validated. A profile never claims compatibility outside recorded acceptance evidence. |
| Device identity | Store the observed Windows input/output endpoint names and, when the platform API exposes them, USB vendor/product IDs and serial/device-instance data. The profile defines ordered match rules; identical ambiguous devices require explicit user selection rather than first-port guessing. |
| Endpoint binding | Input and output endpoints are separate optional bindings. Generic input must work without an output endpoint. Losing or replacing either endpoint emits a visible connection-state change but never blocks the headless engine. |
| Raw control address | Device/profile ID, endpoint direction, template/bank identity, message family (Note, CC, Program Change, MMC, or SysEx), channel, data number, and any profile index. Channel representation is normalized at the boundary and must not mix zero-based wire channels with one-based UI labels. |
| Raw learn sample | Timestamped raw bytes plus parsed family/channel/number/value, observed minimum/maximum, press/release pair, candidate bank/template, and device endpoint. Learn completes only after explicit operator confirmation; noise and unrelated traffic remain visible but unbound. |
| Normalized control | Stable ControlID, kind (button, absolute knob, absolute fader, relative encoder, selector), canonical value, configured source range, inversion, deadband, button behavior, and current trust state. Relative mode is never inferred merely from a rotary physical shape. |
| Button semantics | Normalize Note/CC press and release separately. Momentary, toggle-on-device, toggle-in-host, and one-shot are distinct. A toggle-capable profile may expose a state-write codec, but generic learn does not assume one. |
| Soft takeover | Per binding: target value, last physical value, armed/disarmed state, crossing direction, hysteresis/tolerance, and reason for rearm. Show load, reconnect, template/bank change, mapping edit, external target change, and Send All snapshot all rearm non-motorized absolute controls. |
| Feedback capability | Explicit capability set: none, Note/CC LED, SysEx LED, toggle-state write, template select/report, reset, brightness, color palette, flashing, double buffer. Each codec declares address scope, rate limit, batching, and whether inactive templates may be updated. Unsupported feedback is a no-op with a visible capability reason, never guessed output. |
| Template/bank identity | Stable profile identity plus device-native number/name, selection source, host-select message if any, device-report message if any, and observability status: explicit, inferred-from-address, manual, or unknown. Saved mappings always include this context. |
| Reconnect/resync policy | On disconnect, retain authoritative GOLC state and disarm physical absolute controls. On reconnect, re-identify endpoints, suppress command emission during resync, determine/select template only if the validated profile permits it, seed raw positions without applying them, repaint supported feedback from a complete GOLC snapshot, then resume input. |
| Saved mapping | Mapping schema version, show/surface ID, stable target command and entity IDs, raw control address, normalized transform, soft-takeover settings, feedback address/policy, template/bank identity, and acceptance/profile version. Device profiles and show-specific action bindings are separate records. |
| Deterministic conflict key | The exact tuple of device instance/profile, template/bank, message family, channel, and data number/index. One raw input has at most one active v1 binding. Exact duplicates fail validation; device-specific and generic candidates with equal specificity require explicit resolution. No conflict is settled by registration order or event arrival time. |
| Safety routing | Mapped actions submit typed playback commands through the same command boundary as UI/API/script/LLM. MIDI callbacks never write show state, DMX buffers, or timing state directly. Blackout and Revoke Automation retain their dedicated local-priority semantics. |

### 4.2 Learn and normalization flow

1. Bind one explicit input endpoint and start a time-bounded raw capture.
2. Require the operator to move or press exactly one intended control through a representative cycle.
3. Group messages by message family, channel, and data number; show every candidate and its observed value sequence.
4. Classify only proven patterns: press/release, absolute monotonic, or validated profile-relative. Do not infer hidden bank/template state.
5. Let the operator choose the playback command and target, transform/invert behavior, and soft-takeover threshold.
6. Reject a duplicate active conflict or require the prior binding to be replaced explicitly.
7. Save the mapping with profile/version/template context and immediately test it in a non-output or preview mode before live use.

### 4.3 Feedback and loop prevention

- GOLC feedback is a projection of authoritative command/playback state, never a second state owner.
- Host feedback messages are written only through a profile codec and output endpoint. The input parser tags locally originated output when the platform exposes correlation; otherwise the profile ignores known feedback-only message forms and rate-limits repeated equal states.
- Reconnect uses one canonical state repaint, not a replay of every historical transition.
- Novation double buffering should be used only by the later validated profile when repaint size warrants it. Generic Phase 6 operation does not require SysEx or feedback to pass PLAY-04/05.
- Akai Send All and any Worlde bank snapshot are input-state discovery only. They must not bypass soft takeover or issue a burst of live playback commands.

## 5. Hardware Acceptance Matrix

All rows are currently **PENDING** because this research used manuals and renders only. "G" is required to use the device as Phase 6 evidence for generic PLAY-04/PLAY-05. "P" is additionally required before a device-specific EXTN-04 profile or feedback/support claim.

| ID | Level | Acceptance test | Novation Launch Control XL Mk2 | Akai MIDImix | Worlde EasyControl 9 | Required evidence |
|---|---|---|---|---|---|---|
| HW-01 | G | Identify exact hardware revision, firmware, USB IDs/serial behavior, Windows endpoint names, input/output port count, and behavior with two identical units. | Verify Launch Control XL n and device-ID behavior. | Verify MIDI Mix names and whether both ports enumerate without a driver. | Verify class/driver behavior and names on supported Windows target. | Machine/OS/build, firmware/editor version, USB descriptor and endpoint capture. |
| HW-02 | G | Clean Windows install, cold plug, launch, hot plug, unplug/replug, different USB port, powered/unpowered hub as applicable, suspend/resume, and app restart. Playback must continue without MIDI. | Test full and low-power states separately if relevant. | Test direct and powered-hub behavior from manual. | Verify Windows 11 despite its absence from the manual. | Repeatable test log with connection state and zero engine/output interruption. |
| HW-03 | G | Capture every physical control in every relevant factory/user template or bank. Record raw bytes, channel, type, number, range, press/release, and duplicates. | Capture factory 1 and chosen user template first, then all templates intended for claims. | Capture knobs, all faders, all buttons, Bank Left/Right, and Send All in at least three bank positions. | Capture 4 banks, 9 groups, 2 assignables, 6 transport controls, and fixed program/CC/general controls. | Versioned raw MIDI corpus and reviewed address table. |
| HW-04 | G | Continuous-control quality: endpoints, monotonicity, inversion, deadband/jitter, repeated values, physical center detent, and full sweep speed. | 24 pots and 8 faders. | 24 knobs, 8 channel faders, master fader. | 9 editable knobs/sliders plus fixed slider; test reversed editor endpoints. | Histograms/traces and normalized expected results. |
| HW-05 | G | Button semantics: press/release, velocity/value, momentary/toggle, long hold, rapid repeat, and simultaneous presses. | Test Note and CC user assignments plus device-toggle behavior. | Establish undocumented Mute/Solo/Record/Bank/Send All wire behavior. | Test Note/CC momentary/toggle and all 13 MMC options only if claimed. | Raw event pairs and deterministic normalized-control assertions. |
| HW-06 | G | Template/bank identity and mapping conflict: select each bank/template from hardware, observe messages, detect address reuse, power cycle, and reconnect. | Verify outgoing template-change SysEx and remembered template. | Determine whether bank emits identity or only remaps addresses. | Determine whether a bank event exists; reject indistinguishable overlapping banks. | State-transition trace, ambiguity cases, and conflict-validation results. |
| HW-07 | G | Soft takeover: connect with mismatched physical/software positions, show load, target changed by UI/API, template/bank change, Send All, rapid pass-through, and reconnect. No unintended target jump is allowed. | Test all 8 faders and representative pots. | Explicitly capture Send All as seed-only, then cross to acquire. | Test all editable sliders and fixed slider only if it is allowed to bind. | Before/after target timeline proving zero pre-acquisition commands. |
| HW-08 | G | Saved mapping survives app restart, show save/load, rename/reorder of target display labels, and controller reconnect. Missing device degrades visibly without blocking keyboard/on-screen operation. | Include template number/profile version. | Include observed bank/address and fallback prompt. | Include bank observability status and editor configuration hash/export if available. | Mapping round trip and missing/ambiguous-device scenarios. |
| HW-09 | P | Host feedback exactness, unsupported-message safety, and no input feedback loop. | Verify per-index SysEx and currently selected-template Note/CC paths. | Discover and document any host LED messages before claiming feedback. | Discover whether any host feedback exists; absence means input-only profile. | Independent MIDI monitor capture plus expected physical LED observations. |
| HW-10 | P | Full feedback palette and state: off/on, brightness, colors, flashing, toggle synchronization, template-scoped state, and reset. | Verify listed values, reset/all-on, toggle state, inactive-template update, and power-cycle behavior. | Test only behaviors discovered from official/editor or captured protocol; do not infer colors. | Selected-bank LED is local baseline; host-controlled claims require proof. | Photo/video or operator observation linked to raw outbound bytes. |
| HW-11 | P | Reconnect/resync from authoritative GOLC snapshot while live commands are suppressed. | Select known template if configured, repaint all LED/toggle states, test background templates, and confirm no transient false action. | Prompt for Send All if needed, seed positions, rearm pickup; validate LED state separately. | Re-identify/manual bank, seed positions, rearm pickup; do not claim LED repaint without protocol. | Disconnect/reconnect trace showing one bounded resync and no action burst. |
| HW-12 | P | Editor/template persistence and reproducibility. Export or record exact template, write it to a second/power-cycled unit, and compare raw map. | Components/editor user template and LED choices. | MIDImix Editor fields and persistence. | Worlde editor on current Windows, four banks, reversed ranges, button modes, MMC. | Versioned template artifact or complete reproducible configuration procedure. |
| HW-13 | G/P | Soak and load: at least one representative show session with rapid faders/buttons, UI churn, save, script/API load as available, repeated reconnect, and no growing queues or timing dependency. | G for input; P with high-rate LED updates. | G for input; P only if feedback is implemented. | G for input; P only if feedback is implemented. | Metrics, raw traffic rate, errors, dropped/coalesced feedback, and final pass report. |
| HW-14 | G/P | Clean packaged Windows build acceptance and support-scope sign-off. | **Selected Phase 6 physical acceptance-set member**; required independently before naming Novation compatibility or support. | **Selected Phase 6 physical acceptance-set member**; required independently before naming Akai compatibility or support. | **Selected Phase 6 physical acceptance-set member**; required independently before naming Worlde compatibility or support. | Signed acceptance record naming exact controller revision, firmware, Windows versions, app build, and supported capability subset. |

### Acceptance outcomes

Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 form the selected Phase 6 physical acceptance set, and every outcome remains independent.

- **Akai MIDImix - Selected Phase 6 physical acceptance-set member:** generic learn/soft-takeover qualification and any named compatibility remain pending its G rows; feedback/profile support additionally requires its P rows and EXTN-04 scope.
- **Novation Launch Control XL Mk2 - Selected Phase 6 physical acceptance-set member:** generic learn/soft-takeover qualification and any named compatibility remain pending its G rows; the documented feedback protocol additionally requires its P rows and EXTN-04 scope.
- **Worlde EasyControl 9 - Selected Phase 6 physical acceptance-set member:** generic learn/soft-takeover qualification and any named compatibility remain pending its G rows; its older platform statement plus undocumented bank event and feedback make the independent physical/editor probes mandatory.

## 6. Planning Updates Required

This quick task applies the planning updates below while preserving release scope, Phase 1 plans, and the credential-free Linear boundary.

### Canonical blocker wording

**Resolve MIDI-HW-01 with:**

> **MIDI-HW-01 - RESOLVED 2026-07-19:** Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together are the selected Phase 6 physical acceptance set for generic MIDI Note/CC learn and soft-takeover qualification. Selection does not establish compatibility or support and does not promote device-specific profiles, SysEx initialization, feedback, or packaging into v1.

**Track remaining acceptance with:**

> **MIDI-HW-02 - OPEN:** Complete and record an independent physical Windows evidence set for each of Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9, naming the exact hardware revision, firmware, Windows version, and GOLC build. Each device must independently pass enumeration/hot plug, full raw map, ranges, button behavior, bank/template persistence, reconnect, conflicts, saved mappings, and PLAY-05 soft takeover before a named compatibility/support claim. Any device-specific feedback/profile claim additionally requires its applicable SysEx/LED/toggle/resync evidence and remains under EXTN-04/v1.x.

### Exact locations

Every location records Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together as the selected Phase 6 physical acceptance set without extending Phase 6 or Phase 1 implementation scope.

| Location | Current planning meaning | Required change |
|---|---|---|
| .planning/PROJECT.md:67 and :92 | Controller is "not yet selected"; selection blocks device acceptance. [VERIFIED: current lines] | Replace the unresolved statement with MIDI-HW-01 resolved for the complete three-device set; change the constraint from selection-gated to evidence-gated support claims. Add the equal set decision to Key Decisions. Keep complete keyboard/on-screen playback unchanged. |
| .planning/REQUIREMENTS.md:70-71 | PLAY-04 generic Note/CC and PLAY-05 soft takeover are v1 Phase 6 requirements. [VERIFIED: current lines] | No scope change. Record the complete set only in the documentation gate section, not in requirement behavior. |
| .planning/REQUIREMENTS.md:152 | EXTN-04 places installable device-specific profiles after validation in v1.x. [VERIFIED: current line] | Keep unchanged. Link MIDI-HW-02 as its validation prerequisite. |
| .planning/REQUIREMENTS.md:185-187 | MIDI-HW-01 is currently open because no target is selected. [VERIFIED: current lines] | Mark MIDI-HW-01 resolved and add MIDI-HW-02 open with the wording above. |
| .planning/ROADMAP.md:93-107 | Phase 6 is generic MIDI only and currently says selection is open. [VERIFIED: current lines] | Replace only the Blocker line with MIDI-HW-01 resolved for the complete set and MIDI-HW-02 open for independent evidence. Do not add SysEx/profile tasks to Phase 6. |
| .planning/STATE.md:65,74,83 | State says hardware is unresolved and device profiles/feedback are blocked by MIDI-HW-01. [VERIFIED: current lines] | Record the complete equal-set decision, close MIDI-HW-01, open MIDI-HW-02, and change the deferred row to "Device-specific profiles and feedback - v1.x, gated by MIDI-HW-02/EXTN-04." |
| .planning/research/FEATURES.md:32,125,135,238 | v1 is generic learn/soft takeover/supported feedback; advanced profiles/SysEx are v1.x; target is an open question. [VERIFIED: current lines] | Preserve generic Note/CC learn and soft takeover; replace the open target question with the complete set and independent per-device evidence question; gate device-specific profiles/feedback in v1.x by MIDI-HW-02 and EXTN-04. |
| AGENTS.md:27 | Generated project constraint still says selection is pending. [VERIFIED: current line] | Regenerate AGENTS.md from canonical planning sources after those sources change; do not hand-edit generated text. |

### Traceability update

MIDI-HW-01 and MIDI-HW-02 are documentation gate labels only for the selected Phase 6 physical acceptance set of Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9. They create no new Phase 1 entity class, catalog entry, requirement, plan, Linear mapping, or implementation work; the existing dynamic traceability catalog remains unchanged. Remote Linear UUIDs remain pending until credentialed reconciliation is explicitly authorized, and this task neither calls Linear nor invents remote IDs. [VERIFIED: .planning/ROADMAP.md:24-36; Phase 1 CONTEXT.md decisions D-11 through D-21]

## 7. Phase 1 Readiness Impact

### Scope

The selected Phase 6 physical acceptance set comprises Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 and does not alter Phase 1 implementation. Phase 1 owns CONF-01 through CONF-04 and LINR-01 through LINR-04 only. Its locked boundary explicitly excludes lighting behavior, UI, playback, Art-Net, scripting, AI, and MIDI implementation. [VERIFIED: .planning/ROADMAP.md:24-36; .planning/phases/01-offline-foundation-and-delivery-traceability/01-CONTEXT.md:7-9]

MIDI-HW-01 and MIDI-HW-02 are documentation gate labels only. They create no new Phase 1 entity class, catalog entry, requirement, plan, Linear mapping, or implementation work, and the existing dynamic traceability catalog remains unchanged. Do **not** add MIDI libraries, controller enumeration, learn UI, soft takeover, profiles, SysEx codecs, controller template assets, or hardware tests to any Phase 1 plan. This quick task may inspect and validate Phase 1 only through read-only checks.

### Readiness

- MIDI-HW-01 is resolved by the complete three-device set. MIDI-HW-02 remains open and belongs to Phase 6 independent physical acceptance and EXTN-04/v1.x profile validation, not Phase 1.
- Current STATE.md says Phase 1 is executing and "Ready to execute." [VERIFIED: .planning/STATE.md:7,33]
- Read-only validation confirms the existing 29-plan set remains structurally valid, covers all 21 locked decisions and eight Phase 1 ROADMAP requirement IDs, and initializes as 29 incomplete executable plans with no missing agents.
- `nyquist_compliant: false` and `wave_0_complete: false` remain correct before execution creates and passes the planned Wave 0 test infrastructure. [VERIFIED: Phase 1 VALIDATION.md frontmatter and Wave 0 requirements]

**Readiness conclusion:** Phase 1 is **MIDI-ready, scope-stable, and ready to execute** with its existing plans unchanged and without any MIDI implementation work.

## 8. Remaining Hardware Unknowns and Claim Language

The items below are gaps left after comparing every supplied manual page with its temporary render; they are not claims that the device cannot perform the function. [VERIFIED: complete review of the four supplied PDFs and temporary renders]

### Akai MIDImix unknowns

- Exact default channel, CC/Note numbers, values, and button press/release behavior.
- Whether Bank Left/Right emits identity, changes addresses only, wraps, or persists across reconnect.
- Exact Send All ordering/content and behavior after bank changes.
- MIDImix Editor fields, bank/template persistence, current Windows compatibility, and reproducible export.
- Host-controlled LED protocol, colors/brightness if any, feedback-loop behavior, reset, and resync.
- Windows endpoint/USB identity, duplicate-unit behavior, and suspend/resume.

### Novation Launch Control XL Mk2 unknowns

- Exact hardware revision, firmware, editor/Components version, USB IDs, device-ID persistence, and Windows endpoint names for the unit to be accepted.
- Exact factory CC/Note address table for the selected template and raw values/noise of every physical control.
- User-template editor persistence and reproducibility across power cycles or a second unit.
- Physical confirmation of all documented SysEx, colors, brightness, flashing, toggle state, inactive-template update, double buffering, reset, and reconnect behavior on the chosen Mk2 unit.
- No-loop behavior when host feedback and input are active together, and bandwidth/rate limits during complete repaints.

### Worlde EasyControl 9 unknowns

- Current Windows 11 enumeration, driver/class behavior, endpoint names, USB IDs, duplicate-unit behavior, and editor availability.
- Exact factory/default channel, CC/Note map, fixed general-slider controller number, and the mapping of the two fixed buttons to CC 64 versus 67.
- Whether bank changes emit a message, whether banks reuse addresses, and what persists across power/reconnect.
- Whether any runtime host-to-device MIDI input or LED feedback protocol exists.
- Editor persistence/export, reversed endpoint behavior, MMC bytes, and hot-plug/suspend behavior on current systems.

### Required claim language

Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 are the selected Phase 6 physical acceptance set. Until each device's applicable matrix passes independently:

- **Akai MIDImix - Selected Phase 6 physical acceptance-set member:** selection is not compatibility or support; a named claim requires its independent MIDI-HW-02 evidence for the exact hardware revision, firmware, Windows version, and GOLC build.
- **Novation Launch Control XL Mk2 - Selected Phase 6 physical acceptance-set member:** selection and richer protocol documentation are not compatibility or support; a named claim requires its independent MIDI-HW-02 evidence for the exact hardware revision, firmware, Windows version, and GOLC build.
- **Worlde EasyControl 9 - Selected Phase 6 physical acceptance-set member:** selection is not compatibility or support; a named claim requires its independent MIDI-HW-02 evidence for the exact hardware revision, firmware, Windows version, and GOLC build.
- A Phase 6 claim may say **generic MIDI Note/CC learn and soft takeover were validated with [exact device/firmware/Windows/app build]** only after all G rows pass.
- A device-specific claim may say **profile/feedback validated for [exact device/firmware/Windows/app build and listed capabilities]** only after all applicable P rows pass and EXTN-04 is in scope.
- Do not extrapolate from one unit to all firmware revisions, from editor success to runtime feedback, from illuminated hardware to host-controllable LEDs, or from manual protocol text to packaged Windows interoperability.

## Final Planning Recommendation

Record **Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9** together as the selected Phase 6 physical acceptance set, mark MIDI-HW-01 resolved, and keep MIDI-HW-02 open for an independent evidence set per device. Selection is not support. Leave PLAY-04 and PLAY-05 generic and unchanged, leave EXTN-04 in v1.x, preserve the existing dynamic Phase 1 traceability catalog, and execute the existing Phase 1 plans without MIDI implementation work.
