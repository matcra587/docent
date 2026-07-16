---
slug: agent-guides
title: Agent Guides
description: Browse and retrieve embedded runbooks via the agent guide subcommand.
when_to_use: When an AI agent needs a workflow runbook or wants to enumerate available guides for the host tool.
commands: [example agent guide, example agent schema]
order: 1
---

## Decide

Determine whether you need the full guide list, a single guide by slug,
or a specific section within a guide. Use `example agent guide` for the
index, `example agent guide <slug>` for a full guide, and
`--section <heading>` to retrieve one section.

## Run

```sh
# List all available guides (frontmatter-only index)
example agent guide

# Retrieve a specific guide by slug
example agent guide agent-guides

# Retrieve a single section from a guide
example agent guide agent-guides --section Run
```

## Save

Record the slug of any guide retrieved so you can reference it in subsequent calls
without repeating the full listing step.

## Preconditions

The host tool must have the docent agent command group mounted via `docentcobra.NewCommand`
and a populated `Config.Guides` field pointing to the loaded `GuideSet`.

## Recover

If no guides are returned, verify that the host embeds the guide files with `go:embed`
and passes the resulting `GuideSet` in `Config.Guides`. Re-run `example agent guide`
after the host is restarted with guides configured.

## Next

Use `example agent schema` to retrieve the neutral command schema and identify which
commands are available for structured invocation.
