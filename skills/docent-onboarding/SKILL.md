---
name: docent-onboarding
description: Wire docent into a Go CLI that has no agent surface yet — embedded guides, runtime schema, skill export, and the optional human guide command. Use when adding agent support to a cobra-based CLI or adopting the Agent Guide Standard from scratch.
license: MIT
metadata:
  origin: docent
---

# docent onboarding

Wire [docent](https://github.com/matcra587/docent) into a cobra-based Go
CLI so it serves its own agent knowledge: embedded Markdown guides, a
runtime-derived command schema, and generated skills — one source of truth,
never a hand-maintained twin.

## Prerequisites

*   A cobra-based Go CLI (docent's only adapter today).
*   docent as a dependency: `go get github.com/matcra587/docent`, or a
    `go.work` pointing at a checkout while developing pre-tag (never commit
    `go.work`).
*   Read the [Agent Guide Standard](https://github.com/matcra587/docent/blob/main/docs/agent-guide-standard.md)
    — it is the contract everything below implements.

## Step 1 — Create the guides package

```go
// internal/agentguides/agentguides.go
package agentguides

import (
	"embed"
	"io/fs"

	"github.com/matcra587/docent"
)

//go:embed guides/*.md
var guidesFS embed.FS

// Load parses and validates the embedded guide set.
func Load() (*docent.GuideSet, error) {
	sub, err := fs.Sub(guidesFS, "guides")
	if err != nil {
		return nil, err
	}

	return docent.LoadGuides(sub)
}
```

## Step 2 — Author the first guides

One file per guide under `internal/agentguides/guides/`, six sections in
order, frontmatter per the standard:

```markdown
---
slug: safe-mutation
title: Preview every write before sending it
description: Validate a mutation with --dry-run, then submit and verify.
when_to_use: Before any create/edit/delete against a live instance.
commands: [tool item create, tool item edit]
---

## Decide
## Run
## Save
## Preconditions
## Recover
## Next
```

Conventions that save rework later:

*   `commands:` entries are full paths including the root name; a
    cross-cutting guide declares `commands: []` (whole-tool scope) — never
    a token command it does not document.
*   A section with nothing to say stays **empty** — an empty
    `## Preconditions` already states "none"; filler prose costs tokens.
*   Reference-style guides read the six sections as "the task is using the
    tool": Decide picks a mode, Run states the rules, Save the output
    fields, Recover the fixed exit codes.
*   `description` + `when_to_use` compose into the exported skill
    description and must fit 1024 characters; front-load trigger words.

## Step 3 — Mount the surface

Mount **after** the tree is fully assembled — never in `init`, where later
`init` functions register commands the schema walk would miss:

```go
guides, err := agentguides.Load()
if err != nil {
	return fmt.Errorf("load agent guides: %w", err)
}

tree := docentcobra.Tree(root)
tree.Extensions = map[string]any{ // host contract metadata, host-owned
	"auth":       map[string]any{"env_var": "TOOL_TOKEN"},
	"exit_codes": map[string]any{"validation": 4},
}

cfg := docent.Config{Guides: guides, Command: tree}
root.AddCommand(docentcobra.NewCommand(cfg))
```

Optionally add the human door — same guide set, byte-identical output,
host-owned styling (see the `_examples/glamour-host` dispatch:
`charm.land/glamour/v2`, buffer-then-render, agents and pipes get raw
Markdown):

```go
root.AddCommand(docentcobra.NewGuideCommand(cfg))
```

## Step 4 — Keep the surface credential-free

The standard requires `agent guide`/`schema`/`export` to work without
credentials: discovery precedes auth. Cobra runs your root's persistent
hooks around docent's commands, so exempt them:

```go
for c := cmd; c != nil; c = c.Parent() {
	if c.Name() == "agent" || c.Name() == "guide" {
		return nil // skip credential resolution
	}
}
```

## Step 5 — Verify

*   Self-host the contract: `docenttest.Validate(t, guidesFS, tree)` in
    your CI — every guide violation class fails distinctly.
*   Pin the schema: `docenttest.SchemaGolden(t, tree, "testdata/schema.json",
    *update)`; mask startup-computed defaults (ephemeral ports, home paths)
    with `docenttest.MaskFlagDefaults` or the pin flakes.
*   Validate exports: `tool agent export --format agent-skill --dir /tmp/s`
    then `bunx skills-ref validate /tmp/s/<slug>` — every artifact must
    pass. `claude-skill` answers to Claude Code's schema instead and is
    rejected by skills-ref **by design**.

## Pitfalls

*   ❌ Mounting in `init` — the schema silently misses late-registered
    commands. ✅ Mount from `Execute`/`main` after assembly.
*   ❌ Credential resolution in a persistent hook killing `agent guide` on
    keyring-less machines. ✅ Step 4.
*   ❌ Restating guide content in AGENTS.md/CLAUDE.md. ✅ One pointer line;
    the embedded guides are the authority.
*   ❌ Adding a rendering library to the CLI for the human door. ✅ Styling
    stays behind `Config.Out`; docent emits plain Markdown.

## Success criteria

*   `tool agent guide` prints a frontmatter index; `--section`, `--all`,
    `schema --path`, and `export` all answer — with no credentials
    configured.
*   `docenttest.Validate` and the schema golden run in CI.
*   Exported `agent-skill` artifacts pass `skills-ref validate`.

## Related skills

*   `docent-migrate` — when the CLI already has a hand-rolled agent
    surface to replace.
