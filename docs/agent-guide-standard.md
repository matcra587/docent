# Agent Guide Standard (v1)

A standard for packaging agent knowledge inside a CLI. Guides are
**content, not code**; the binary is the **delivery mechanism**; skills are a
**generated view**, never a second source of truth.

This document defines the standard that docent implements and validates
(`StandardVersion = 1`). Any CLI that adopts docent — or independently
follows this document — is conformant.

## Principles

1.  **One source of truth, shipped with the tool.** Agent knowledge lives in the
    repo, versioned with the code, embedded in the binary. If the vendor
    landscape changes, nothing is lost and nothing needs rewriting.
2.  **Markdown files, not string literals.** Adding a guide is dropping a file
    in; removing one is deleting a file. No recompile-to-reword friction beyond
    the embed rebuild, no diff noise from string escaping.
3.  **Facts come from introspection, workflows come from guides.** `agent
    schema` derives commands/flags/types from the live command tree at runtime —
    it can never go stale. Guides carry only what can't be derived: workflows,
    invariants, and recovery steps.
4.  **Everything else points here.** AGENTS.md / CLAUDE.md / docs pages
    reference the guides; they never restate them.

## 1. Layout

```text
internal/agenthelp/
  guides/
    core-contract.md
    safe-mutation.md
    <slug>.md
  agenthelp.go        # go:embed guides/*.md + loader
```

One guide per file. Filename is the slug. The directory name is the host's
choice; the file contract below is not.

## 2. Guide file format

YAML frontmatter + Markdown body with a fixed section shape.

```markdown
---
slug: safe-mutation
title: Preview every write before sending it
description: Validate a mutation with --dry-run, then submit and verify.
when_to_use: Before any create/edit/delete against a live instance.
commands: [item create, item edit]
---

## Decide
Which command and payload form to use, and why.

## Run
The exact invocation(s), in fenced `sh` blocks.

## Save
What to capture from the response (fields, IDs) for later steps.

## Preconditions
What must be true before running (auth state, required flags, scopes).

## Recover
What each likely failure looks like and the corrective action.

## Next
Related guides by slug.
```

Rules:

*   Frontmatter keys `slug`, `title`, `description`, `when_to_use`, `commands`
    are required; `slug` must match the filename and be unique in the repo.
    A cross-cutting guide declares an explicitly empty list (`commands: []`),
    meaning it applies to the whole tool — never a token command it does not
    actually document. An absent `commands` key remains a violation.
    An optional `order` (sparse integers) controls canonical ordering; guides
    without one sort alphabetically after those with one.
*   `slug` follows the [Agent Skills](https://agentskills.io) name rules:
    1–64 characters, lowercase `a-z`, `0-9`, and hyphens only, with no
    leading, trailing, or consecutive hyphens. Slugs become exported skill
    names and directories (§6), so conformant slugs make every export
    conformant by construction.
*   `description` plus `when_to_use` (joined as `<description> Use when:
    <when_to_use>`) must fit the spec's 1024-character skill description cap.
    Front-load the trigger words: harnesses truncate long descriptions in
    their skill listings before anything else.
*   Section headings are exactly the six above, in order. Empty sections are
    allowed but must be present — agents pattern-match on the shape. When a
    section has nothing to say, leave the body empty rather than writing
    filler: an empty `## Preconditions` already states "none".
*   Cross-cutting behavior folds into the six sections rather than growing a
    seventh: invocation semantics belong in **Run**, response-parsing
    semantics in **Save**, failure semantics in **Recover**. Tool-wide
    contracts live in one reference-style guide the others point at from
    **Next**.
*   Reference-style guides (a tool's output contract, exit codes, envelope
    shape) fit the six sections read as "the task is using the tool":
    **Decide** chooses a mode, **Run** states the rules governing every
    invocation, **Save** describes the envelope to parse, **Recover** lists
    the fixed exit codes. Structured contract data additionally belongs in
    the schema `extensions` slot (§3), where it stays machine-readable.
*   Every command invocation in a fenced `sh` block must be a real command in
    the binary (enforced by contract test, §5).
*   Line-oriented prose, ~80 cols, so diffs review well.

## 3. Command surface (uniform across hosts)

| Command                         | Behavior                                                        |
| ------------------------------- | --------------------------------------------------------------- |
| `<tool> agent guide`            | List all guides: frontmatter index, no bodies.                  |
| `<tool> agent guide <slug>`     | Print one guide; `--section <name>` prints one section.         |
| `<tool> agent guide --all`      | Every guide concatenated in canonical order, form-feed separated. |
| `<tool> agent schema`           | Runtime-derived contract; `--path <cmd>` subsets one subtree.   |
| `<tool> agent export --dir <dir> --format <fmt> \| --scope <scope> [--format <fmt>]` | Materialize guides for a harness (§6). |

Output encoding (JSON envelopes, compact modes) is the host's contract, not
this standard's — the standard fixes *what* is retrievable, hosts fix *how*
it is rendered.

The agent surface MUST work without credentials: discovery precedes auth by
definition, and an agent bootstrapping on a fresh machine has nothing but
this surface to learn the tool from. Hosts that resolve credentials in a
persistent pre-run hook must exempt the agent command group (and any human
guide mount) — a keyring lookup that fails before `agent guide` can print a
byte makes the whole standard unreachable exactly when it is needed most.

Hosts MAY attach structured metadata the neutral schema does not model —
auth requirements, environment variables, exit codes, output contracts,
per-command examples — under a command's `extensions` key. The root
command's extensions describe the tool. Extension values are host-owned and
emission stays deterministic, with one documented exception: a host that
declares its own agent-contract version gets it stamped onto the emitted
schema root as a `contract_version` extensions entry and onto the guide
index as a `contract_version` line directly after the header, so an agent
can pin behavior to the contract it read. Adapters MAY also contribute
documented, namespaced per-flag extension entries derived from the host's
flag library.

The recommended shape for the single-guide view (`agent guide <slug>`) is
the reading-oriented runbook: the title as an H1 heading followed by the six
sections, frontmatter omitted. Frontmatter is routing metadata — it belongs
to the index (which lists it) and to skill export (which transforms it); a
caller who already picked a slug from the index is trying to act, not
route.

Hosts MAY additionally mount the same guide browser as a first-class human
command (e.g. top-level `<tool> guide <slug>`). The optional human mount
serves byte-identical output from the same guide set in the same canonical
order — one source of truth, two doors — and does not change the required
agent surface above. Rendering stays the host's concern: agents and pipes
receive raw Markdown; a host may style output for an interactive human
terminal.

## 4. Pointer files, not copies

`AGENTS.md` / `CLAUDE.md` contain repo-workflow guidance (build, test, commit
conventions) plus **one** pointer:

> The embedded agent guides and schema (`<tool> agent guide`, `<tool> agent
> schema`) are the authoritative CLI behavior spec. Update them with any
> behavior change; do not restate them elsewhere.

Any docs-site "for agents" page is generated from (or links to) the same
guide files.

## 5. Contract tests (run in every conformant repo's CI)

*   Every `guides/*.md` parses; required frontmatter present; slug == filename;
    slug satisfies the Agent Skills name rules; description + when_to_use fit
    the 1024-character skill description budget.
*   The six section headings are present, in order.
*   Every command path referenced in `sh` blocks and `commands:` frontmatter
    exists in the live command tree.
*   No guide content is duplicated in AGENTS.md/CLAUDE.md (spot-check via
    heading/sentence overlap, or simply forbid the six headings there).

## 6. Skills are a generated view

Every export format shares one common definition — the
[Agent Skills](https://agentskills.io) open standard: a `<slug>/SKILL.md`
layout whose frontmatter `name` equals the slug (and therefore the directory
name, as the spec requires). Harness-specific formats layer that harness's
extensions on top without diverging from the common shape:

*   `agent export --format agent-skill` — the portable open-standard shape:
    `description` and `when_to_use` join into the single skill `description`.
    Understood by Claude Code, Codex, and any spec-conformant harness.
*   `agent export --format claude-skill` — the Claude Code variant: Claude
    natively supports a `when_to_use` frontmatter field (appended to the
    description in its skill listing), so the two stay separate keys.

Each format answers to its own authority. `agent-skill` conforms to the
neutral spec and passes its reference validator (`skills-ref validate`);
`claude-skill` conforms to Claude Code's documented schema
(code.claude.com/docs/en/skills.md) and is rejected by `skills-ref` **by
design** — `when_to_use` is Claude Code's documented extension, and the
neutral spec's own extension slot is the `metadata` map. Validate each
format against its target, and reach for `agent-skill` when the consuming
harness is unknown. Budgets nest safely: Claude Code caps the combined
listing text at 1,536 characters, and this standard's 1,024-character
budget is stricter, so a valid guide never exceeds either.

The destination resolves in a fixed order: an explicit `--dir` wins and
requires an explicit `--format`; otherwise `--scope project|user` detects the
invoking harness from its environment markers and derives the directory (and
the default format) from that harness's conventions — e.g. Claude Code reads
`.claude/skills` / `~/.claude/skills`, Codex reads `.agents/skills` /
`~/.agents/skills`. The format describes the artifact; the destination
implies a default format; `--format` always overrides. An undetectable
harness with no `--dir` is an error, never a guess.

Exported files carry a generated-do-not-edit header, and frontmatter values
are emitted as single-line quoted scalars so line-oriented frontmatter
parsers never see folded YAML.

Harness-specific plugin/skill packages are **generated from `agent export`**
and never hand-edited. If a skills ecosystem dies, delete the export step;
nothing else changes.

## 7. Reference implementation

docent (this module) is the standard's reference implementation: the guide
loader, frontmatter schema, section validator, canonical ordering, export
renderers, and contract-test helpers (`docenttest.Validate`). A host CLI's
integration is ~10 lines: embed the guides directory, mount the adapter's
`agent` command group, wire its output hooks.

## Acceptance criteria (why this shape)

*   **Easy to add/edit/remove:** a guide is one Markdown file; CRUD is file CRUD.
    Contract tests catch drift automatically.
*   **Easy for humans and LLMs:** humans read the same files on GitHub and any
    docs site; LLMs read them via `agent guide` (or raw, in-repo) and get a
    predictable six-section shape plus runtime schema for the facts.
*   **Vendor-durable:** no knowledge lives only in a harness-specific format;
    skills/plugins are disposable generated artifacts.
