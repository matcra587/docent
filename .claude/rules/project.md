---
description: >
  Architecture map and cross-cutting concerns: what docent is, the package
  layout and import direction, determinism as a contract, and the
  development workflow (mise, changie, releases).
paths:
  - "**/*.go"
  - "docs/**/*.md"
---

# Project

docent is a Go library that lets a CLI act as its own guide for AI agents:
embedded Markdown runbooks (guides), runtime schema introspection from the
live command tree, and generated skill export. It implements the
[Agent Guide Standard v1](../../docs/agent-guide-standard.md). It is a
**library, not an application** — no binary, no config, no HTTP, no output
of its own.

## Packages and import direction

```text
docent            core: Guide, Section, GuideSet, Command/Flag/FlagGroup,
                  LoadGuides(fs.FS), validation keyed to StandardVersion
docent/export     renderers (guide → agent-skill SKILL.md shape) plus the
                  standard's serving artifacts: Index, Concat, root-scoped
                  Write — adapters plumb flags around these, never re-spell
                  the byte shapes
docent/cobra      adapter: Tree(root) walker + NewCommand(cfg) mountable
                  `agent` command group; the ONLY package that may import
                  cobra (depguard-enforced)
docent/harness    agent-runtime detection: DetectAgent (which agent is
                  driving, ~12 runtimes + AI_AGENT override) and Detect
                  (which harness's skill conventions apply)
docent/docenttest contract-test helpers: Validate(t, fs, root)
```

Two non-library surfaces also live in the repo: `_examples/` holds
standalone host modules (own `go.mod`, underscore-invisible to Go tooling,
excluded from the library linters — hosts legitimately do everything the
library rules forbid docent itself), and `skills/` + `.claude-plugin/`
ship the companion agent skills, held to the same `skills-ref` conformance
gate as docent's exports.

Core and export are stdlib + one YAML parser only. Framework knowledge
lives solely in adapters; a kong adapter is deferred beyond v0 but the
neutral `Command` tree must keep permitting it. Hosts integrate through a
zero-value-usable `Config` (tool name, guide set, command tree, an `Out`
`io.Writer`) — docent produces values, hosts own all output.

## Determinism is a contract

Every emitted artifact — guide index, concatenated guides, schema JSON,
exported skills — must be byte-stable and golden-testable. Canonical guide
order: `order:` ascending (ties by slug), then unordered guides
alphabetically by slug. Never iterate a map into output without sorting.

## Workflow

*   Tasks are defined in `tasks.toml`; list with `mise tasks`, verify with
    `mise run check`, test with `mise run test`.
*   Tools are pinned in `.mise.toml` + `mise.lock`; run them via
    `mise x -- <tool>` (nothing is on the bare PATH).
*   Host CLIs consume docent cross-repo via `go.work` during development —
    no tag ceremony. Breaking changes are free pre-v1.
*   Releases: changie fragments → `changie batch` → goreleaser with
    `builds: skip: true` (library — no binaries, no archives). See
    [changelog.md](changelog.md).
