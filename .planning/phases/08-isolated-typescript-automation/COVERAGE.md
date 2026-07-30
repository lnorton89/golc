# API Coverage — GOLC Internal Script SDK

> This is not a third-party API integration. Phase 8 exposes GOLC's own already-implemented command surface (internal/command) to sandboxed TypeScript automation scripts through a generated, typed SDK (golc.d.ts / golc-runtime.ts). The api-coverage gate's vocabulary ("wire sdk", "wrap sdk") triggered on this phase's own requirement text (SCRP-02: "generated typed GOLC SDK"), not an external API/SDK dependency. This matrix documents the existing, already-implemented include/exclude decisions in internal/scriptsdk/descriptors.go (sdkMethodTable / excludedRouteTable) for the gate's record-keeping purpose.

| capability | decision | reason |
|---|---|---|
| `pool create` (`pool.create`) | INTEGRATE | Create a named logical fixture pool. |
| `pool update` (`pool.update`) | INTEGRATE | Compute and write/print a deterministic pool impact-review plan. |
| `pool apply` (`pool.apply`) | INTEGRATE | Validate and atomically apply an already-reviewed pool impact plan. |
| `pool substitute` (`pool.substitute`) | INTEGRATE | Compute and write/print a deterministic fixture-substitution capability-diff review. |
| `deployment create` (`deployment.create`) | INTEGRATE | Create a named deployment. |
| `deployment activate` (`deployment.activate`) | INTEGRATE | Mark exactly one deployment active. |
| `show inspect` (`show.inspect`) | INTEGRATE | Print a deterministic JSON summary of a ShowState document's pools and deployments. |
| `show open` (`show.open`) | INTEGRATE | Open a ShowState document for edit, offering any interrupted-session recovery point found. |
| `show save` (`show.save`) | INTEGRATE | Load and re-save a ShowState document, writing a fresh recovery point. |
| `show save-as` (`show.saveAs`) | INTEGRATE | Load a ShowState document read-only and save it to a new path. |
| `show diagnose` (`show.diagnose`) | INTEGRATE | Run combined file-level and structural diagnostics on a .golc file. |
| `show export` (`show.exportDocument`) | INTEGRATE | Print the full canonical, round-trippable JSON document for a .golc file. |
| `scene create` (`scene.create`) | INTEGRATE | Create a named bar-loop scene. |
| `scene activate` (`scene.activate`) | INTEGRATE | Mark exactly one scene active, deactivating every other scene. |
| `scene layer set` (`scene.layer.set`) | INTEGRATE | Enable/point one of a scene's four fixed layers. |
| `scene rename` (`scene.rename`) | INTEGRATE | Rename a scene, preserving its identity. |
| `scene duplicate` (`scene.duplicate`) | INTEGRATE | Duplicate a scene under a fresh, inactive identity. |
| `scene delete` (`scene.remove`) | INTEGRATE | Delete a scene by name. |
| `blend create` (`blend.create`) | INTEGRATE | Create a named blend preset. |
| `theme create` (`theme.create`) | INTEGRATE | Create a named color theme. |
| `theme rename` (`theme.rename`) | INTEGRATE | Rename a color theme, preserving its identity. |
| `theme delete` (`theme.remove`) | INTEGRATE | Delete a color theme by name. |
| `preset record` (`preset.record`) | INTEGRATE | Record a kind-scoped preset from the persisted Programmer buffer. |
| `preset rename` (`preset.rename`) | INTEGRATE | Rename a preset, preserving its identity. |
| `preset delete` (`preset.remove`) | INTEGRATE | Delete a preset by name. |
| `chase create` (`chase.create`) | INTEGRATE | Create a named chase. |
| `chase update` (`chase.update`) | INTEGRATE | Update a chase's name/step-unit/step-duration. |
| `chase reorder` (`chase.reorder`) | INTEGRATE | Permute a chase's steps deterministically. |
| `chase duplicate` (`chase.duplicate`) | INTEGRATE | Duplicate a chase under a fresh identity. |
| `chase delete` (`chase.remove`) | INTEGRATE | Delete a chase by name. |
| `motion create` (`motion.create`) | INTEGRATE | Create a named motion preset. |
| `motion rename` (`motion.rename`) | INTEGRATE | Rename a motion preset, preserving its identity. |
| `motion duplicate` (`motion.duplicate`) | INTEGRATE | Duplicate a motion preset under a fresh identity. |
| `motion delete` (`motion.remove`) | INTEGRATE | Delete a motion preset by name. |
| `programmer set` (`programmer.set`) | INTEGRATE | Resolve a selection and set semantic attribute values on it. |
| `programmer inspect` (`programmer.inspect`) | INTEGRATE | Print every touched attribute in the Programmer buffer. |
| `programmer clear` (`programmer.clear`) | INTEGRATE | Empty the Programmer buffer's touched-attribute set. |
| `playback bpm set` (`playback.bpm.set`) | INTEGRATE | Set the show-wide global BPM to an explicit numeric value. |
| `playback bpm tap` (`playback.bpm.tap`) | INTEGRATE | Set the show-wide global BPM from ordered tap timestamps. |
| `playback switch` (`playback.switchScene`) | INTEGRATE | Stage an active-scene switch. |
| `operatorsurface create` (`operatorsurface.create`) | INTEGRATE | Create a named operator surface. |
| `operatorsurface list` (`operatorsurface.list`) | INTEGRATE | List every operator surface on a ShowState document. |
| `operatorsurface show` (`operatorsurface.show`) | INTEGRATE | Print a named operator surface's assigned items and MIDI mappings. |
| `operatorsurface assign` (`operatorsurface.assign`) | INTEGRATE | Assign one individual control to a named operator surface. |
| `operatorsurface unassign` (`operatorsurface.unassign`) | INTEGRATE | Unassign one individual control from a named operator surface. |
| `operatorsurface remove` (`operatorsurface.remove`) | INTEGRATE | Delete a named operator surface and all its assignments and MIDI mappings. |
| `artnet status` (`artnet.status`) | INTEGRATE | Inspect per-universe/target Art-Net health as a snapshot or watch view. |
| `artnet interface list` (`artnet.interfaces.list`) | INTEGRATE | List candidate Windows network interfaces for Art-Net output. |
| `artnet discover` (`artnet.discover`) | INTEGRATE | Scan a pinned interface for compatible Art-Net nodes; suggestions only. |
| `artnet configure` (`artnet.configure`) | INTEGRATE | Add or update one unicast Art-Net output target for a universe. |
| `artnet target enable` (`artnet.target.enable`) | INTEGRATE | Re-enable output to one configured unicast target without stopping the rig. |
| `artnet target disable` (`artnet.target.disable`) | INTEGRATE | Take one configured unicast target offline without stopping the rig. |
| `artnet master set` (`artnet.master.set`) | INTEGRATE | Set the grand master or one group's master level. |
| `artnet safety blackout` (`artnet.safety.blackout`) | INTEGRATE | Drive every configured universe's output to zero on the next Art-Net tick. |
| `artnet safety stop-all` (`artnet.safety.stopAll`) | INTEGRATE | Drive every configured universe's output to the safe/zero state. |
| `artnet safety revoke-automation` (`artnet.safety.revokeAutomation`) | INTEGRATE | Block any command carrying a non-manual source tag and freeze the current look. |
| `fixture validate` (`fixture.validate`) | INTEGRATE | Strictly decode and validate a hand-authored YAML fixture definition. |
| `fixture inspect` (`fixture.inspect`) | INTEGRATE | Decode a fixture definition and print its content-addressed identity and provenance. |
| `fixture import` (`fixture.importDefinition`) | INTEGRATE | Import an Open Fixture Library definition through GOLC's canonical pipeline. |
| `api-key create` (`apiKey.create`) | INTEGRATE | Mint a new scoped, expiring API key. |
| `api-key list` (`apiKey.list`) | INTEGRATE | List every API key's metadata. |
| `api-key revoke` (`apiKey.revoke`) | INTEGRATE | Revoke one API key by id. |
| `playback evaluate` | OPT-OUT | frame evaluation is not a script-reachable capability (ROADMAP Phase 8 success criterion 2) |
| `artnet serve` | OPT-OUT | daemon lifecycle, owned by the application not a script |
| `build` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `test` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `check` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `generate` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `package` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `run` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `dev` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `docs` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `lint` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `tools update` | OPT-OUT | contributor build tooling, not a show-domain capability |
| `config inspect` | OPT-OUT | repository configuration is a contributor concern outside the show domain |
| `config set` | OPT-OUT | repository configuration is a contributor concern outside the show domain |
| `config explain` | OPT-OUT | repository configuration is a contributor concern outside the show domain |
| `linear catalog` | OPT-OUT | planning-traceability tooling, not a show-domain capability |
| `linear preview` | OPT-OUT | planning-traceability tooling, not a show-domain capability |
| `linear apply` | OPT-OUT | planning-traceability tooling, not a show-domain capability |
| `linear drift` | OPT-OUT | planning-traceability tooling, not a show-domain capability |
| `linear status` | OPT-OUT | planning-traceability tooling, not a show-domain capability |
| `linear archive` | OPT-OUT | planning-traceability tooling, not a show-domain capability |
| `linear unlink` | OPT-OUT | planning-traceability tooling, not a show-domain capability |
| `linear map migrate` | OPT-OUT | planning-traceability tooling, not a show-domain capability |
| `linear validate` | OPT-OUT | planning-traceability tooling, not a show-domain capability |
| `script create` | OPT-OUT | script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script |
| `script list` | OPT-OUT | script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script |
| `script show` | OPT-OUT | script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script |
| `script edit` | OPT-OUT | script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script |
| `script delete` | OPT-OUT | script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script |
| `script profile set` | OPT-OUT | script entity lifecycle, owned by the application/CLI, not a capability exposed to a running script |
| `script run` | OPT-OUT | script lifecycle control; a script must not be able to launch another script |
| `script stop` | OPT-OUT | script lifecycle control; a script must not be able to terminate itself or another run through the SDK |
| `script validate` | OPT-OUT | script lifecycle control; a script must not validate or introspect other scripts through the SDK |
| `script debug` | OPT-OUT | script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK |
| `script continue` | OPT-OUT | script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK |
| `script step-over` | OPT-OUT | script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK |
| `script step-into` | OPT-OUT | script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK |
| `script step-out` | OPT-OUT | script lifecycle and debugger control; a script must not launch, debug, or step another script through the SDK |

**Source of truth:** `internal/scriptsdk/descriptors.go` (`sdkMethodTable`, `excludedRouteTable`). This file is a record for the api-coverage gate, not a second source of truth — if the descriptor table changes, regenerate this table to match rather than hand-editing it out of sync.
