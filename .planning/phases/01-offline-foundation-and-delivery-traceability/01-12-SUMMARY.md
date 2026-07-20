---
phase: 01-offline-foundation-and-delivery-traceability
plan: 12
subsystem: infra
tags: [npm, supply-chain, linear-sdk, typescript, human-verify]

requires: []
provides:
  - Explicit exact-version human approval for @linear/sdk@88.1.0 and typescript@7.0.2
  - Registry/source/integrity evidence trail for both pins re-verified live at approval time
affects: [01-13, 01-14, 01-15, 01-25, 01-26, 01-27, linear-sync]

tech-stack:
  added: []
  patterns:
    - Blocking human supply-chain gate before first npm execution; approval is exact-version scoped

key-files:
  created: []
  modified: []

key-decisions:
  - "Both exact pins approved: @linear/sdk@88.1.0 and typescript@7.0.2. Approval is scoped to these exact versions only; any version change requires a new human gate."
  - "@linear/sdk stays pinned at 88.1.0 even though registry latest is 88.2.0, per AGENTS.md lock; updates flow through `tools update`, not floating tags."

patterns-established:
  - "Supply-chain gate evidence: recorded research integrity hashes are re-checked against the live registry at approval time before presenting to the human."

requirements-completed: [CONF-03, LINR-03, LINR-04]

coverage:
  - id: D1
    description: "The user explicitly approved both exact npm pins against official registry/source evidence before any npm execution."
    requirement: LINR-04
    verification:
      - kind: other
        ref: "01-12 automated research-audit check (pins + 'no postinstall' present in 01-RESEARCH.md)"
        status: pass
    human_judgment: true
    rationale: "Package legitimacy approval is inherently a human supply-chain decision; the explicit approval is recorded verbatim in this summary."

duration: 8min
completed: 2026-07-20
status: complete
---

# Phase 1 Plan 12: Package Legitimacy Approval Summary

**Explicit human approval of the exact pins @linear/sdk@88.1.0 and typescript@7.0.2 with live registry re-verification — npm installation in Plans 13-15/25-27 is now unblocked**

## Performance

- **Duration:** 8 min (this session; a prior interrupted session posed the same question without recording an answer)
- **Started:** 2026-07-20T05:55:00Z
- **Completed:** 2026-07-20T06:03:00Z
- **Tasks:** 1
- **Files modified:** 0

## Accomplishments

- Ran the plan's automated research-audit verify: 01-RESEARCH.md records both exact pins with integrity hashes and `no postinstall` — PASS.
- Re-verified both pins against the live npm registry at approval time:
  - `@linear/sdk@88.1.0` — published 2026-07-08, source `github.com/linear/linear`, integrity `sha512-l0U5O2hFcNFYbjH+YQXJyO14obFX5mb6jdSjWstAkoDaIwy6oBnkd4P9uPy/4PnUDfSf5bPWSI9STCxo5STFxw==` (matches research record), no install/preinstall/postinstall lifecycle scripts, ~1.66M weekly downloads. Registry latest is 88.2.0 (2026-07-14); pin intentionally held at 88.1.0 per AGENTS.md.
  - `typescript@7.0.2` — published 2026-07-08, source `github.com/microsoft/TypeScript`, integrity `sha512-8FYa96o3NKOhbjKi/qNvG/W5jhzxkbdm5sj9AbZ/5T5sWqn3hJgLfGx27sRKZWTvyzCP8dLRBTf5tBTSRVUNA==` (matches research record), no lifecycle scripts, current `latest` tag, ~218M weekly downloads.
- Presented the blocking-human checkpoint with both npm registry version pages and obtained explicit approval.

## Recorded Approval

The user was presented the gate "Approve installing these two exact npm pins? (@linear/sdk@88.1.0 and typescript@7.0.2 — both verified against official sources, integrity hashes match research, no install scripts)" and explicitly selected:

> **Approve both exact pins** — recorded per the plan's resume-signal as: `approved @linear/sdk@88.1.0 and typescript@7.0.2`

Recorded 2026-07-20 via interactive checkpoint in the resumed Claude Code session. No npm command executed during this plan.

## Task Commits

Each task was committed atomically:

1. **Task 1: Verify the two exact recent npm packages** - no repository files changed (approval-only checkpoint); recorded in this summary's metadata commit

**Plan metadata:** committed with this summary

## Files Created/Modified

None — this plan intentionally modifies no repository files. The approval record lives in this summary.

## Decisions Made

- Approved both exact pins after live registry evidence matched the recorded research evidence exactly.
- The prior interrupted session (different runtime, ran out of credits mid-gate) recorded no answer; the gate was re-presented from scratch rather than assuming any prior consent.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plans 01-13/14/15 and 01-25/26/27 may run `npm ci`/install strictly for the approved exact pins.
- Rejection path was not needed; no package-manager execution occurred in this plan.

## Self-Check: PASSED

- This summary exists on disk and records the exact approval text.
- The automated research-audit verify command exits 0.
- No npm process ran during this plan; working tree contains no package artifacts.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-20*
