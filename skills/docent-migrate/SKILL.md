---
name: docent-migrate
description: Replace a CLI's hand-rolled agent guide/schema surface with docent, with captured before/after evidence and no information loss. Use when a Go CLI already ships bespoke agent docs, guide strings, or a custom schema walker.
license: MIT
metadata:
  origin: docent
---

# docent migration

Replace a hand-rolled agent surface — guide strings in Go files, bespoke
schema walkers, custom guide commands — with docent, proving along the way
that nothing an agent relied on was lost. Distilled from executed trial
migrations of real hosts (one replaced ~1,700 hand-rolled lines with ~300;
another carried domain-reference guides and an envelope schema).

## Step 1 — Capture the before, first

Build the current binary and capture every agent-facing output **before
touching anything**; these are the baseline for every later diff:

```sh
tool agent guide           > before/guide-list.txt
tool agent guide <name>    > before/guide-name.txt
tool agent schema          > before/schema.json
```

If capture fails on a clean machine (keyring, credentials), that is your
first finding: the standard requires a credential-free agent surface.

## Step 2 — Recon the entanglement

The old surface's package almost never contains *only* the surface. Both
executed migrations found envelope types, attribution logic, or
agent-detection helpers sharing the package with the docs code. Map what
stays before deleting anything — expect a package split, not a removal.

## Step 3 — Port the guides

*   Guide prose trapped in Go strings becomes one `.md` file per guide
    (see `docent-onboarding` for the format and package).
*   Slug rules forbid underscores and uppercase — renamed guides declare
    their old names so nothing memorized breaks:

    ```yaml
    slug: send-msg
    aliases: [send_msg]
    ```

*   Headings that do not match the six sections fold by what they govern:
    invocation semantics → Run, parsing semantics → Save, failure
    semantics → Recover. Cross-cutting "contract" documents map as
    reference-style guides with `commands: []`.

## Step 4 — Re-home the schema's top level

Hand-rolled schemas carry contract metadata the neutral IR deliberately
does not model — auth, env vars, exit codes, output modes, examples. Diff
your before-schema's top-level keys and move them into the host-owned
extensions slot:

```go
tree := docentcobra.Tree(root)
tree.Extensions = map[string]any{
	"auth":       map[string]any{"env_var": "TOOL_TOKEN"},
	"exit_codes": map[string]any{"auth": 1, "not_found": 2},
	"output":     map[string]any{"modes": []string{"json", "human"}},
}
```

Enums declared through gechr/clib extras surface automatically; other flag
libraries need `docent.enum` annotations or an `Enum()` method.

## Step 5 — Swap the mount

Remove the old commands, mount `docentcobra.NewCommand(cfg)` after full
tree assembly, and fix what falls out: completion hooks referencing old
guide-name lists, help text, and the tests pinned to the old schema shape
(point those at `docenttest.SchemaGolden`).

## Step 6 — Prove it with after-captures

Re-capture everything from Step 1 and diff. Expected changes are the
migration's evidence; **any unexpected drift is a finding**. Record the
numbers — they justify the migration:

*   Index vs old full-dump (one trial host: ~870 lines → 19).
*   Schema size vs hand-rolled (expect near-parity after hoisting).
*   `bunx skills-ref validate` on every exported `agent-skill` artifact.
*   The old name still answering: `tool agent guide old_name`.

## Pitfalls

*   ❌ Deleting the old package wholesale — it hides envelope/detection
    code the rest of the CLI imports. ✅ Step 2's split.
*   ❌ Citing a token command to satisfy `commands:` validation on a
    cross-cutting guide. ✅ `commands: []` means whole-tool.
*   ❌ Trusting the migration because the code compiles. ✅ Step 6's
    before/after diffs are the acceptance test.
*   ❌ Golden-pinning a schema containing startup-computed defaults
    (ephemeral ports). ✅ `docenttest.MaskFlagDefaults` first.

## Success criteria

*   Every view the old surface served has a docent equivalent, verified by
    captured before/after outputs.
*   Old guide names resolve via aliases; the before-schema's top-level
    metadata is machine-readable under `extensions`.
*   The suite passes with old schema-shape tests replaced by
    `docenttest.SchemaGolden`, and exported artifacts are
    `skills-ref`-clean.

## Related skills

*   `docent-onboarding` — the target state this migration lands on, and
    the authoring/mounting reference.
