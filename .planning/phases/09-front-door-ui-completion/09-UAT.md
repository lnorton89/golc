---
status: testing
phase: 09-front-door-ui-completion
source: [09-VERIFICATION.md]
started: 2026-07-28T01:40:00Z
updated: 2026-07-28T01:40:00Z
---

## Current Test

number: 1
name: Fixture Library lists local fixture YAML files with correct manufacturer text and status chips
expected: |
  Real local fixture rows render with manufacturer, model, and a validation-status chip; an empty
  directory renders the empty state, not a blank panel.
awaiting: user response

## Tests

### 1. Fixture Library browse — local fixture list and empty state
expected: Real local fixture rows render with manufacturer, model, and a validation-status chip; an empty directory renders the empty state, not a blank panel.
result: [pending]

### 2. Fixture Library search and inline inspect
expected: Search narrows the list live; selecting a row shows inline (never modal) inspect detail; a broken file shows the shared error copy.
result: [pending]

### 3. Show open / new via on-screen controls (supervised self-relaunch)
expected: The app performs a supervised self-relaunch into the newly chosen show; "New Show…" against a nonexistent path opens an empty show.
result: [pending]

### 4. Guided First Show auto-launch, Exit Guide, Start Guide resume
expected: Guide auto-launches on a genuinely empty show inside the real shell; Exit Guide returns to Overview without re-trapping; Start Guide resumes at the correct stage.
result: [pending]

### 5. Guided First Show Fixtures/Patch stage live evidence
expected: Fixtures stage blocker clears to evidence after a real import, driven by a live re-read; Patch stage hands off without mutating.
result: [pending]

### 6. Guided First Show full happy path to Verify/Perform
expected: Verify aggregates real state into zero blockers when everything is in place; a missing scene disables only Perform, never other workspaces.
result: [pending]

### 7. CLI-imported fixture visible in on-screen library
expected: A CLI-imported .json envelope fixture is visible in the on-screen library, not just the .yaml ones.
result: [pending]

### 8. OFL catalog search — online and offline states
expected: Live catalog search works online; goes to the explicit unreachable state offline, never a crash or blank panel.
result: [pending]

### 9. OFL preview-then-commit import, repeat-import Replace offer
expected: End-to-end preview-then-commit catalog import works live; a repeat import is refused with an explicit Replace offer.
result: [pending]

### 10. Hand-authored YAML fixture via native file picker
expected: The native OS file picker opens, inline validation runs against a real file, and a valid custom fixture is added to the library.
result: [pending]

## Summary

total: 10
passed: 0
issues: 0
pending: 10
skipped: 0
blocked: 0

## Gaps
