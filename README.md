# docent

[![CI](https://github.com/matcra587/docent/actions/workflows/test.yml/badge.svg)](https://github.com/matcra587/docent/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/matcra587/docent.svg)](https://pkg.go.dev/github.com/matcra587/docent)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> **docent** — a person who leads guided tours, especially through a museum
> or art gallery.

docent is a Go library that turns a CLI into its own tour guide.

Guides ship in the binary. Command schemas come from the live command tree.
Harness-specific skills are generated views from the same source of truth.

It implements the [Agent Guide Standard v1](docs/agent-guide-standard.md).

## Why

Most CLIs already contain the information an agent needs — scattered across
help text, docs, runbooks, and framework-specific command definitions.
docent brings those pieces into one model, so the schema cannot go stale and
exports never become hand-maintained twins.

## Status

Released and usable — guides, schema introspection, and skill export are
all golden-pinned. The API is pre-1.0 and not yet stable: breaking changes
are free until the shape is proven against more host integrations and wider
framework support.

## Install

```sh
go get github.com/matcra587/docent
```

## Quick start

Embed Markdown guides, load them at startup, and mount the `agent` command
group on your cobra root:

```go
import (
	"embed"
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
)

//go:embed guides/*.md
var guidesFS embed.FS

func mountAgent(root *cobra.Command) error {
	sub, err := fs.Sub(guidesFS, "guides")
	if err != nil {
		return err
	}

	guides, err := docent.LoadGuides(sub)
	if err != nil {
		return err
	}

	cfg := docent.Config{Guides: guides, Command: docentcobra.Tree(root)}
	root.AddCommand(docentcobra.NewCommand(cfg))

	return nil
}
```

Agents then discover the CLI through `app agent guide`, `app agent schema`,
and `app agent export`. See the [package docs](https://pkg.go.dev/github.com/matcra587/docent)
and [_examples/glamour-host](_examples/glamour-host) for a full host.

### Export configuration

Hosts sharing a harness skills root can qualify the two built-in skill
formats when mounting docent, without changing guide slugs or guide lookup:

```go
cfg := docent.Config{
	Guides:  guides,
	Command: docentcobra.Tree(root),
}

root.AddCommand(docentcobra.NewCommand(
	cfg,
	docentcobra.WithSkillNameQualifier("jira"),
))
```

A guide with slug `core-contract` then exports directly beneath the skills
root as `jira-core-contract/SKILL.md`, with `name: jira-core-contract` in its
frontmatter. Omit `WithSkillNameQualifier` (the default) to preserve the
unqualified `<slug>/SKILL.md` output. Formats registered through
`WithExtraFormat` receive the original guide unchanged and own their naming
policy; docent applies the qualifier only to `agent-skill` and
`claude-skill`.

## Layout

```text
docent/            core: guide and schema model, loading, and validation
docent/export/     agent-skill rendering plus the standard's serving
                   artifacts: guide index, concatenation, root-scoped writes
docent/cobra/      cobra adapter: tree walker + mountable `agent` command group
docent/harness/    agent-runtime detection from environment markers
docent/docenttest/ contract-test helpers
```

Core is standard library plus YAML only; framework knowledge lives in
adapters. Cobra is the first adapter target; kong is deferred until the v0
shape is proven.

## Non-goals

Not a general documentation generator, not tied to a single agent harness,
and never a second hand-maintained skill definition format.

## Agent skills

The repo ships a Claude Code plugin with two companion skills —
`docent-onboarding` (wire docent into a fresh CLI) and `docent-migrate`
(replace a hand-rolled agent surface). They version with the API they
describe:

```sh
npx skills add github.com/matcra587/docent
```

## Development

See [CONTRIBUTING.md](.github/CONTRIBUTING.md) for setup and workflow.
