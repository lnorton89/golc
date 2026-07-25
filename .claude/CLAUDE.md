# Project Skill Routing

- **Sketch findings for GOLC** (validated layout, navigation, programming, performance, MIDI,
  onboarding, and readiness decisions) → load `.planning/sketches/SKILL.md`

# MCP Tool Routing

- **Picking up work / session start** → call `golc_project_status` first (authoritative milestone/phase/progress)
- **Roadmap phase questions** → `golc_list_phases` / `golc_get_phase_detail` over reading ROADMAP.md directly
- **Config concern/key questions** → `golc_config_inspect` / `golc_config_explain` over reading config/*.toml
- **Command routes / Mage targets** → `golc_list_command_routes` / `golc_list_mage_targets`
- **Schema or internal-package doc lookups** → `golc_get_schema` / `golc_get_reference_doc` over reading schemas/*.json or source directly
