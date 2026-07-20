# MIDI Hardware Evidence Index

**Reviewed:** 2026-07-19

Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together are the selected Phase 6 physical acceptance set for generic MIDI Note/CC learn and soft-takeover qualification. The devices are equal members of that set. Selection and manual review do not establish compatibility or support.

MIDI-HW-01 is resolved by the selected set and the four immutable user-supplied manuals below. MIDI-HW-02 remains open: each controller must independently pass its independent MIDI-HW-02 evidence set for the exact hardware revision, firmware, Windows version, and GOLC build before GOLC makes a named compatibility or support claim.

## Canonical Manuals

| Controller | Manual and role | SHA-256 | Manual review | Physical evidence |
|---|---|---|---|---|
| Akai MIDImix | [Akai-MIDImix-UserGuide-v1.0.pdf](./Akai-MIDImix-UserGuide-v1.0.pdf) - setup, controls, bidirectional USB MIDI, Send All/pickup warning, and specifications | `203D4859E9C15364E7C228842BCAC0BF1AEED68E38ED56E6E5B964DF6BA5ECDA` | Reviewed, PDF pages 3-5 and 18 | PENDING independently under MIDI-HW-02 |
| Novation Launch Control XL Mk2 | [launch_control_xl_programmer_s_reference_guide.pdf](./launch_control_xl_programmer_s_reference_guide.pdf) - MIDI, SysEx, LED, template, toggle, reset, flashing, and double-buffer protocol | `076985FA9A0859A2ECCE0C35D1E843FC5815BD67062C0019DE9A8CDCA07F7C06` | Reviewed, all 9 PDF pages | PENDING independently under MIDI-HW-02 |
| Novation Launch Control XL Mk2 | [Novation-Launch Control XL GSG v2.pdf](<./Novation-Launch Control XL GSG v2.pdf>) - hardware, class-compliant USB, templates, and editor workflow | `5EC473BE4CEFAE0F694171A02DAEF686C134791D6EB7EB2A2E71D6A36E48CB1F` | Reviewed, all 7 PDF pages | PENDING independently under MIDI-HW-02 |
| Worlde EasyControl 9 | [Worlde-EasyControl-9-UserManual.pdf](./Worlde-EasyControl-9-UserManual.pdf) - controls, banks, editor fields, Note/CC/MMC output, platform statement, and specifications | `D4CCD8244410C3F9ECD7143350AE12ADF3A623B72630A0A55017C9FC858990B7` | Reviewed, all 9 PDF pages | PENDING independently under MIDI-HW-02 |

The PDF files are immutable user-supplied canonical inputs. Research and planning documents may link to them and verify their hashes, but must not rewrite them.

## Evidence Status and Remaining Probes

### Akai MIDImix

- Manual evidence: USB sends and receives MIDI; knobs/faders send continuous-controller messages; Mute and Record Arm controls are backlit; Send All reports current controller settings and may be suppressed by software pickup/takeover.
- Physical evidence still required: exact default channels and CC/Note addresses, button press/release behavior, Bank Left/Right identity, Send All ordering, current Windows endpoints, editor persistence, host LED protocol, reconnect, and duplicate-unit behavior.

### Novation Launch Control XL Mk2

- Manual evidence: class-compliant USB MIDI; 8 user and 8 factory templates; Note/CC input; device-to-host template reports; host template selection; per-template LED/toggle SysEx; reset, palette, flashing, and double buffering.
- Physical evidence still required: exact unit revision and firmware, current Windows endpoints, factory raw map, editor persistence, control noise/ranges, every claimed SysEx/LED behavior, reconnect/resync, rate limits, and feedback-loop safety.
- Novation has the most thoroughly documented supplied LED/SysEx protocol, but that documentation difference creates no priority, compatibility, or support implication.

### Worlde EasyControl 9

- Manual evidence: four banks; configurable Note/CC controls and channels; configurable ranges; momentary/toggle buttons; CC/MMC transport; selected-bank LED; older driver-free Windows platform statement.
- Physical evidence still required: current Windows behavior and endpoints, exact defaults, bank-change observability/address reuse, editor availability and persistence, fixed-control map, runtime host feedback capability, reconnect, and duplicate-unit behavior.

Every device's physical acceptance is independent. A passing result for one set member does not transfer to another member or to a different revision, firmware, Windows version, or GOLC build. Device-specific profiles and feedback remain v1.x work gated by both MIDI-HW-02 and EXTN-04.

## Reproducible Hash Verification

Run from the repository root:

```powershell
Get-ChildItem -LiteralPath .planning\midi -Filter *.pdf | Get-FileHash -Algorithm SHA256 | Sort-Object Path
```

The four results must match the uppercase SHA-256 values recorded in the table before the manuals are used as evidence.
