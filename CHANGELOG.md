# Release Notes


## [0.3.0](https://github.com/matcra587/docent/releases/tag/v0.3.0) — 2026-07-22

### Breaking Changes

- Emit schema JSON compact: MarshalSchema, the agent schema command, and docenttest.SchemaGolden now produce single-line JSON with no insignificant whitespace. Re-golden pinned schemas on upgrade and delete host-side compaction transforms.
- Omit zero-valued fields from the cobra adapter's clib flag extensions: empty strings, false, null, and empty collections are no longer serialized, and an all-zero extras blob yields no extensions entry. Re-golden pinned schemas and delete host-side extension-thinning workarounds.
- Emit the full agent schema tree with structure only: registered input/output schema bodies are replaced by has_input_schema/has_output_schema markers. agent schema --path <cmd> keeps bodies embedded, and a new --shapes flag embeds them tree-wide; --path with --shapes is an error. Command gains HasInputSchema/HasOutputSchema fields and a StripShapes method.
- Pool repeated schema bodies in shape-embedding emissions: agent schema --shapes and --path hoist a body registered on multiple commands into a root $defs map and replace each occurrence with {"$ref": "#/$defs/<name>"}; Command gains a Defs field and a PoolShapes method.

### Fixed

- Resolve the rootless form in agent schema --path: "item create" now matches the same subtree as the root-inclusive "app item create" instead of failing the lookup.

## [0.2.0](https://github.com/matcra587/docent/releases/tag/v0.2.0) — 2026-07-21

### Breaking Changes

- Remove the unused Config.ToolName field: like the previously removed Config.Version, it was documented and populated by hosts but read by nothing — the tool's name already lives on the schema root via the adapter's tree walker

### Added

- Add `harness.EnvVars` listing every environment variable agent detection consults, so hosts and tests can scrub or pin the full detection surface
- Add `SectionHeadings` returning the standard's required section headings in order, so tooling can derive the section vocabulary instead of hardcoding it
- Add `Command.Clone` returning a deep copy of a schema IR command tree
- Add `MarshalSchema` as the single canonical schema JSON emission, shared by the cobra adapter's schema command and `docenttest.SchemaGolden` so goldens cannot drift from the emitted artifact
- Add `Guide.Section` for case-insensitive section lookup, owning the matching semantics behind the guide command's --section flag
- Add `export.Index`, `export.Concat`, and `export.GuideSeparator` rendering the guide index and form-feed concatenation, so non-cobra hosts emit the standard's artifacts without re-implementing their shapes
- Add `export.Write` writing rendered artifacts under an os.Root-guarded directory, hoisting the path-escape protection out of the cobra adapter for any host or adapter to reuse
- Add `Harness.SkillsPath` resolving a scope's skills directory, making the scope-to-root mapping a harness-owned contract
- Add `GuideSet.All` returning a lazy iterator over guides in canonical order, yielding boundary copies per guide so early-exit consumers skip the cost of copying the whole corpus
- Add error sentinels `export.ErrInvalidContractVersion`, `export.ErrPathEscape`, and `harness.ErrUnsupportedScope` plus `harness.ValidateScope`, so hosts branch on failure classes with errors.Is instead of matching message text

### Changed

- `harness.DetectAgent` now canonicalizes version-qualified runtime identifiers ("claude-code_2-1-211_agent" returns "claude-code"), so every consumer of the name sees the same spelling
- A `Config.ContractVersion` containing a newline, carriage return, or colon now fails guide index emission with an error instead of corrupting the index's line-oriented shape
- Guide frontmatter fields the index emits (title, description, when_to_use, commands and aliases elements) must be single-line: LoadGuides now rejects multiline values with `ErrMultilineField`, closing an index-forgery vector via YAML block scalars
- Guide files may no longer contain form-feed characters: LoadGuides rejects them with `ErrFormFeed`, making the form-feed guide separator in concatenated output unambiguous by construction

## [0.1.0](https://github.com/matcra587/docent/releases/tag/v0.1.0) — 2026-07-16

### Added

- Add Flag.Extensions carrying per-flag metadata the neutral IR does not model; the cobra adapter surfaces gechr/clib extras (placeholder, hints, grouping) under its "clib" key instead of dropping them
- Add Config.ContractVersion: when set, the agent schema root gains a contract_version extensions entry and the guide index a contract_version line, letting agents pin behavior to the host's contract
- Add GuideSet.Resolve loose guide lookup — exact slug, alias, case/separator-normalized form, then unique substring; the cobra guide command uses it so near-miss names land on the right guide instead of failing
- Add WithSchemaTransform cobra option: hosts rewrite the agent schema command's output before emission, enabling response envelopes and alternate encodings; transforms compose in registration order
- Add public harness package: DetectAgent identifies which of ~12 agent runtimes is driving the process (AI_AGENT/AGENT overrides, truthy markers) for host output-mode resolution, and Detect resolves the invoking harness's skill-export conventions
- Initial release: embedded Markdown agent guides with load-time validation (Agent Guide Standard v1), framework-neutral command schema introspection, a mountable cobra agent command group with guide/schema/export subcommands, skill export to the Agent Skills open standard and Claude Code formats, and docenttest contract-test helpers
