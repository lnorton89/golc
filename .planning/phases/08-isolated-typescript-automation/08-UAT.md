---
status: testing
phase: 08-isolated-typescript-automation
source: [08-VERIFICATION.md]
started: 2026-07-30T19:20:00Z
updated: 2026-07-30T19:20:00Z
---

## Current Test

number: 1
name: Task 1 live-observation steps (sandbox denial + resource enforcement watched in real time), including the new memory-limit kill
expected: |
  No visible Art-Net interruption during any termination (deadline, rate, scope, or the new memory kill); Stop is a genuine single click with no confirmation; a plain Run opens no inspector socket (verified by OS-level port enumeration, not just the textual absence check the automated test uses).
awaiting: user response

## Tests

### 1. Task 1 live-observation steps (sandbox denial + resource enforcement watched in real time), including the new memory-limit kill
expected: No visible Art-Net interruption during any termination (deadline, rate, scope, or the new memory kill); Stop is a genuine single click with no confirmation; a plain Run opens no inspector socket (verified by OS-level port enumeration, not just the textual absence check the automated test uses).
result: [pending]

### 2. Task 2's sixteen-step authoring-to-debugging GUI walkthrough
expected: Every step matches 08-UI-SPEC.md's Copywriting Contract and D-01/D-04/D-05/D-12 behavior exactly as specified.
result: [pending]

### 3. 08-14 Task 3's live desktop check: create an Advanced-profile script at Memory limit 64 MB with a retained allocating loop, click Run, and confirm the debug panel's terminal banner reads exactly "Stopped: memory limit exceeded (64 MB). Increase the limit in this script's profile if this is expected." with Dismiss and Run Again present, and that Art-Net output is uninterrupted throughout.
expected: Terminal banner renders the exact Copywriting Contract sentence with Dismiss/Run Again present; Art-Net output shows no interruption.
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
