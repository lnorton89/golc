# MIDI-HW-02 Evidence Checklist

Per-device physical acceptance checklist for the Phase 6 acceptance set selected under
`MIDI-HW-01` (resolved 2026-07-19). Selection and manual review do **not** establish
compatibility or support; each device below must independently pass physical acceptance
for its exact hardware revision, firmware, Windows version, and GOLC build before any
named compatibility or support claim (see STATE.md blockers and the Phase 6 constraint
in PROJECT.md). Device-specific profiles and LED feedback remain v1.x work under EXTN-04.

Source basis: the four manuals in this directory, extracted 2026-07-20. Where a manual
does not document a value (most factory CC/note numbers), the gap is listed as a
**physical capture step** — record the value from the real device with a MIDI monitor
during acceptance; do not copy numbers from third-party sources.

What Phase 6 acceptance must show per device (roadmap, Phase 6 success criterion 3):
generic MIDI Note and Control Change **learn** for supported playback commands, and
fader **soft takeover** without unintended value jumps.

---

## 1. Akai MIDImix

Manual: `Akai-MIDImix-UserGuide-v1.0.pdf` (User Guide v1.0, English pp. 3-5, appendix p. 18)

### Documented control surface
| Control | Count | Documented MIDI behavior |
|---|---|---|
| Knobs (270°, 3 per channel) | 24 | Continuous controller (CC) messages; numbers not documented |
| Channel faders (30 mm) | 8 | CC messages; numbers not documented |
| Master fader (30 mm) | 1 | CC message; "controls software master volume by default" |
| Mute buttons (amber backlit) | 8 | Message type not documented in this guide |
| Record-arm buttons (red backlit) | 8 | Message type not documented in this guide |
| Solo button | 1 | Modifier: hold + Mute button = solo; own message not documented |
| Bank Left / Bank Right | 2 | Shifts the 8 controlled channels; message not documented |
| Send All | 1 | Dumps all current controller values to the host |

### Manual facts relevant to GOLC
- USB bus-powered, class-compliant (no driver mentioned); powered hub required if hubbed.
- Mapping is customizable only through the MIDImix Editor (download from akaipro.com).
- **Takeover warning (p. 5):** "if you are using a 'pickup' or 'takeover' mode in your
  software, this [Send All] button may not do anything." Send All interacts directly
  with GOLC soft-takeover logic — test both with and without a Send All press.

### Not documented — capture from the physical unit
- [ ] Factory CC number for each knob, fader, and the master fader
- [ ] Message type (Note vs CC) and numbers for Mute / Rec-Arm / Solo / Bank / Send All
- [ ] Transmit MIDI channel(s)
- [ ] Button press/release values (velocity or CC value pairs)
- [ ] Behavior of Bank Left/Right on the transmitted numbers (same CCs re-used vs shifted)
- [ ] Firmware/revision identifier (not exposed in manual; record purchase/PCB/USB
      descriptor info from Device Manager)

---

## 2. Novation Launch Control XL Mk2

Manuals: `launch_control_xl_programmer_s_reference_guide.pdf` (Programmer's Reference v2)
and `Novation-Launch Control XL GSG v2.pdf` (Getting Started Guide)

### Documented control surface
| Control | Count | Documented MIDI behavior |
|---|---|---|
| Rotary pots (centre detent, LED ring) | 24 | CC in factory templates; editable in user templates |
| Faders (long-throw) | 8 | CC; numbers per template, not tabulated in these PDFs |
| Channel buttons (bi-colour LED, 2 rows) | 16 | Note **or** CC (per template); press = velocity 127, release = 0 |
| Device / Mute / Solo / Record Arm | 4 | Programmable buttons (yellow LED) |
| Up / Down / Left / Right | 4 | Navigation buttons (red LED) |
| Template buttons (User / Factory) | 2 | Template switching (hold + press pad 1-8) |

### Manual facts relevant to GOLC
- Class-compliant USB MIDI; single port named `Launch Control XL n` (n = device ID, hidden for ID 1).
- **Template system is the key learn-mode hazard:** 16 templates — user templates in
  slots 0-7 (editable via the Launch Control XL Editor), factory templates in slots 8-15
  (fixed CC/Note sets). The zero-indexed **MIDI channel of a message identifies the
  template**, so the same physical control transmits on a different channel per template.
  GOLC learn must record channel + number, and acceptance must pin which template the
  operator profile assumes.
- Buttons transmit Note-on/CC with velocity/value 7Fh (127) on press and a second
  message with 0 on release (Programmer's Reference p. 8).
- Device→host SysEx `F0 00 20 29 02 11 77 <template> F7` is emitted on template change —
  GOLC can detect template switches even in v1 (logging only; feedback is EXTN-04).
- Full LED control (Note/CC Launchpad protocol + SysEx set-LED `78h`, toggle `7Bh`,
  set-template `77h`, double-buffer/flash) is documented — v1.x feedback material.
- Low-power mode toggle exists (hold User+Factory while inserting USB, then Record Arm /
  Solo, then right arrow); acceptance should run in full-power mode and note the state.

### Not documented in these PDFs — capture from the physical unit
- [ ] Exact CC numbers of pots/faders and Note numbers of buttons for the factory
      template chosen for acceptance (record template slot + channel + numbers)
- [ ] Firmware version (record from Novation Components / USB descriptor)
- [ ] Confirm "Mk2" revision identity of the physical unit (these manuals are not
      Mk2-labelled; verify the model/revision printed on the unit and USB product ID)

---

## 3. Worlde EasyControl 9

Manual: `Worlde-EasyControl-9-UserManual.pdf`

### Documented control surface
| Control | Count | Documented MIDI behavior |
|---|---|---|
| Assignable knobs | 9 | CC only; number + left/right values set in editor |
| Assignable sliders | 9 | CC only; number + upper/lower values set in editor |
| Assignable buttons | 11 | Note **or** CC; momentary or toggle; on/off values |
| Transport buttons (REW/PLAY/FF/LOOP/STOP/REC) | 6 | CC **or** MMC (13 commands, device ID 0-127) |
| Bank buttons + LEDs | 4 banks | Switch between 4 stored assignment banks |
| Fixed program-change knob | 1 | Program change; **not editable** |
| Fixed buttons | 2 | Send CC64 / CC67; **not editable** |
| Fixed general slider | 1 | General controller; **not editable** |

### Manual facts relevant to GOLC
- USB bus-powered (mini-B), driver-free, hot-plug.
- Channels: global "Scene MIDI Channel" 1-16; transport and control-group channels
  are independently configurable or can follow the scene channel. Record all three.
- All assignments require the Worlde software editor (worlde.com.cn) — nothing is
  editable from the device. Acceptance must record which bank (1-4) and editor
  configuration was active; a bank switch changes every assignment at once.
- **Hazard for learn mode:** the two fixed CC64/CC67 buttons collide with the MIDI
  sustain/soft-pedal convention, and the fixed program-change knob emits non-CC
  traffic. GOLC learn must ignore or explicitly handle Program Change and should be
  tested against accidental CC64/67 presses during learn.
- Momentary buttons send 127 on press / 0 on release; toggle alternates — verify GOLC
  learn handles both behaviors for the same physical button type.

### Not documented — capture from the physical unit
- [ ] Factory default CC numbers for all knobs/sliders/buttons in each of the 4 banks
      (or the specific edited bank used for acceptance — export/screenshot the editor state)
- [ ] Default scene/transport/group MIDI channels as shipped
- [ ] Firmware/revision identifier (USB descriptor; manual documents none)

---

## Acceptance procedure (all devices)

1. Connect the device to the acceptance Windows machine directly (no hub).
2. Record identity evidence (table below) before any test: USB VID/PID and device
   strings from Device Manager, firmware where obtainable, photos of the unit and any
   revision markings.
3. With a MIDI monitor, move **every** control and log channel + message type +
   number + value range. Attach the log to this file's evidence section.
4. In GOLC, learn each playback command from the device per the Phase 6 workflow;
   verify every learned control triggers only its command.
5. Soft takeover: set a GOLC-side value away from the physical fader position, move
   the fader, and verify no value jump until pickup; repeat after a bank/template
   switch and (MIDImix) after pressing Send All.
6. Record pass/fail per row. A device passes MIDI-HW-02 only when every row is PASS
   for the exact combination recorded.

## Evidence tables

### Akai MIDImix
| Field | Value |
|---|---|
| Hardware revision / markings | |
| USB VID:PID / descriptor strings | |
| Firmware | |
| Windows version + build | |
| GOLC build (commit/tag) | |
| Full control capture log attached | ☐ |
| Note/CC learn test | ☐ PASS ☐ FAIL |
| Fader soft-takeover test (incl. Send All case) | ☐ PASS ☐ FAIL |
| Result / date / operator | |

### Novation Launch Control XL Mk2
| Field | Value |
|---|---|
| Hardware revision / markings (confirm Mk2) | |
| USB VID:PID / descriptor strings | |
| Firmware | |
| Template used (slot + user/factory) | |
| Windows version + build | |
| GOLC build (commit/tag) | |
| Full control capture log attached | ☐ |
| Note/CC learn test | ☐ PASS ☐ FAIL |
| Fader soft-takeover test (incl. template-switch case) | ☐ PASS ☐ FAIL |
| Result / date / operator | |

### Worlde EasyControl 9
| Field | Value |
|---|---|
| Hardware revision / markings | |
| USB VID:PID / descriptor strings | |
| Firmware | |
| Bank + editor configuration (export attached) | |
| Windows version + build | |
| GOLC build (commit/tag) | |
| Full control capture log attached | ☐ |
| Note/CC learn test (incl. PC-knob / CC64-67 interference) | ☐ PASS ☐ FAIL |
| Fader soft-takeover test (incl. bank-switch case) | ☐ PASS ☐ FAIL |
| Result / date / operator | |
