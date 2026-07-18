# Phase 1: Offline Foundation and Delivery Traceability - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-17
**Phase:** 01-offline-foundation-and-delivery-traceability
**Areas discussed:** Contributor setup flow, Configuration hierarchy, Linear authority and conflicts, Linear synchronization lifecycle

---

## Contributor Setup Flow

### Clean-checkout preparation

| Option | Description | Selected |
|--------|-------------|----------|
| One bootstrap command | Install/download pinned project-local tools where practical, then verify the complete environment | ✓ |
| Verification only | Report missing tools and require manual installation | |
| Manual setup | Document every setup step for the contributor | |

**User's choice:** One bootstrap command.
**Notes:** This becomes the supported clean-Windows-checkout path.

### Offline behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Offline core work | After bootstrap, generation, validation, build, and tests use pinned local caches; network-only tasks fail clearly | ✓ |
| Fetch when needed | Builds and tests may download missing dependencies | |
| Vendor dependencies | Commit tools and dependencies directly into the repository | |

**User's choice:** Offline-capable core work after bootstrap.
**Notes:** Linear sync and dependency refresh remain explicitly network-dependent.

### Contributor command surface

| Option | Description | Selected |
|--------|-------------|----------|
| Single repository command | Cohesive bootstrap, check, generate, build, test, package, and Linear subcommands | ✓ |
| Task runner | Use commands such as `task build` and `task test` | |
| Native commands | Document direct Go, Node, Wails, and packaging commands | |

**User's choice:** One repository command with subcommands.
**Notes:** The implementation technology and exact command name remain planner discretion.

### Tool and dependency updates

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit update | Produce reviewable manifest and lockfile changes; bootstrap never upgrades silently | ✓ |
| Bootstrap upgrade | Install newest compatible versions automatically | |
| Ecosystem-native updates | Update each language/tool ecosystem manually | |

**User's choice:** Explicit, reviewable updates.
**Notes:** Silent version drift is rejected.

---

## Configuration Hierarchy

### Root organization

| Option | Description | Selected |
|--------|-------------|----------|
| Root index manifest | Small machine-readable root manifest points to logically separated concern files | ✓ |
| Monolithic config | Store all development and application settings in one file | |
| Documentation index | Link independent tool-native files through documentation only | |

**User's choice:** Root index manifest with separated concerns.
**Notes:** Centralization means discoverability and authority, not one enormous file.

### Override precedence

| Option | Description | Selected |
|--------|-------------|----------|
| Layered and inspectable | Committed defaults, user config, local untracked config, environment, then CLI | ✓ |
| Repository only | Disallow overrides outside committed configuration | |
| Defaults plus environment | Support only committed defaults and environment variables | |

**User's choice:** Layered, inspectable precedence.
**Notes:** Effective values must identify the layer that supplied them.

### Generated artifacts

| Option | Description | Selected |
|--------|-------------|----------|
| Selective commit | Commit stable schemas, contracts, and types; ignore caches and machine-specific output | ✓ |
| Generate all | Generate during bootstrap and commit no generated files | |
| Commit all | Commit every generated artifact | |

**User's choice:** Selective commit.
**Notes:** Reviewability and downstream consumption determine whether generated output is committed.

### Validation strictness

| Option | Description | Selected |
|--------|-------------|----------|
| Strict | Unknown keys, duplicate authority, invalid values, and unresolved references fail; deprecated keys guide migration | ✓ |
| Warning-first | Unknown keys warn while builds continue | |
| Permissive | Accept broad inputs and fill defaults where possible | |

**User's choice:** Strict validation.
**Notes:** Deprecation is the only warning path; invalid or ambiguous configuration fails.

---

## Linear Authority and Conflicts

### Ownership boundary

| Option | Description | Selected |
|--------|-------------|----------|
| Split authority | Repository owns scope/structure; Linear owns operational execution fields | ✓ |
| Repository authority | Linear is a one-way mirror | |
| Linear authority | Repository planning synchronizes from Linear | |

**User's choice:** Split authority.
**Notes:** Linear-owned fields are status, assignee, priority, estimate, and completion timestamps. Comments remain in Linear.

### Same-field conflicts

| Option | Description | Selected |
|--------|-------------|----------|
| Block for review | Show field-by-field conflict and require explicit resolution | ✓ |
| Repository wins | Overwrite Linear automatically | |
| Linear wins | Overwrite repository fields automatically | |

**User's choice:** Block for explicit resolution.
**Notes:** Neither source wins automatically.

### Fields synchronized back

| Option | Description | Selected |
|--------|-------------|----------|
| Operational fields | Status, assignee, priority, estimate, and completion timestamps | ✓ |
| Minimal fields | Status and completion timestamps only | |
| All fields | Include comments and description edits | |

**User's choice:** Operational fields.
**Notes:** Discussion and comments are not duplicated into repository planning artifacts.

### Rename and removal behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Stable identity and reviewed archive | Renames change display text; removals require archive/unlink review | ✓ |
| Mirrored deletion | Automatically delete in both directions | |
| Add-only | Never remove Linear objects | |

**User's choice:** Stable identity with explicit archive/unlink.
**Notes:** Local IDs and Linear UUIDs survive renames.

---

## Linear Synchronization Lifecycle

### Synchronization trigger

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit mutation plus CI drift checks | Repository commands mutate at milestones; PR CI is read-only | ✓ |
| Commit-triggered mutation | Push planning changes automatically after every commit | |
| Manual only | No CI drift checks | |

**User's choice:** Explicit milestone sync and read-only PR drift checks.
**Notes:** CI must never mutate Linear during a pull-request check.

### Mutation approval

| Option | Description | Selected |
|--------|-------------|----------|
| Separate preview/apply | Apply the exact deterministic preview and reject it if state changed | ✓ |
| Prompt in one command | Preview and apply after an interactive prompt | |
| Apply then report | Mutate automatically and summarize afterward | |

**User's choice:** Separate deterministic preview and apply.
**Notes:** Stale previews fail closed.

### Credential source

| Option | Description | Selected |
|--------|-------------|----------|
| Credential Manager plus CI environment | Windows credential store locally, protected environment variable in CI | |
| Untracked `.env` | Local secret file with a committed example | ✓ |
| Environment only | Manually configure environment variables everywhere | |

**User's choice:** Untracked `.env` with an example file.
**Notes:** The user explicitly requested a committed example environment file.

### Local and CI environment files

| Option | Description | Selected |
|--------|-------------|----------|
| Example/local/ephemeral CI | Commit `.env.example`, ignore local `.env`, generate ephemeral CI `.env`, never print secrets | ✓ |
| Different handling | Define separate local and CI credential workflows | |

**User's choice:** Committed example, untracked local file, ephemeral CI file.
**Notes:** CI creates its file from protected secrets and removes it after the job.

---

## the agent's Discretion

- Exact technology and internal name for the repository command.
- Exact root-manifest format, concern-file names, and directory layout.
- Cache locations, checksum mechanism, transient retry/backoff policy, preview-plan serialization, and CI provider mechanics.

## Deferred Ideas

None. Discussion remained within Phase 1.
