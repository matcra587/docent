# Release Notes


## [0.1.0](https://github.com/matcra587/docent/releases/tag/v0.1.0) — 2026-07-16

### Added

- Add Flag.Extensions carrying per-flag metadata the neutral IR does not model; the cobra adapter surfaces gechr/clib extras (placeholder, hints, grouping) under its "clib" key instead of dropping them
- Add Config.ContractVersion: when set, the agent schema root gains a contract_version extensions entry and the guide index a contract_version line, letting agents pin behavior to the host's contract
- Add GuideSet.Resolve loose guide lookup — exact slug, alias, case/separator-normalized form, then unique substring; the cobra guide command uses it so near-miss names land on the right guide instead of failing
- Add WithSchemaTransform cobra option: hosts rewrite the agent schema command's output before emission, enabling response envelopes and alternate encodings; transforms compose in registration order
- Add public harness package: DetectAgent identifies which of ~12 agent runtimes is driving the process (AI_AGENT/AGENT overrides, truthy markers) for host output-mode resolution, and Detect resolves the invoking harness's skill-export conventions
- Initial release: embedded Markdown agent guides with load-time validation (Agent Guide Standard v1), framework-neutral command schema introspection, a mountable cobra agent command group with guide/schema/export subcommands, skill export to the Agent Skills open standard and Claude Code formats, and docenttest contract-test helpers
