# Contributing

These notes exist so any contributor — human or agent — can get productive
fast. Read [README.md](../README.md) for what docent is.

## Setup

Tooling is pinned with [mise](https://mise.jdx.dev); nothing is assumed on
your PATH:

```sh
mise install          # install pinned tools (Go, golangci-lint, rumdl, hk, …)
mise x -- hk install  # wire the git hooks (pre-commit, pre-push, commit-msg)
mise tasks            # list available tasks
```

## Working on it

```sh
mise run check      # fmt check + lint + rumdl + go test
mise run test       # run the tests
mise run fix        # apply auto-fixes (gofumpt via golangci-lint, rumdl)
```

`mise run check` and the pre-push hook are *not* the same suite. `check`
runs `go test`, which pre-push never does. Pre-push instead runs hk's
hygiene checks (merge-conflict, case-conflict, large-files,
line-ending/EOF) plus `golangci-lint`, `gomod-tidy`, `pkl`, `rumdl`,
`actionlint`, `zizmor`, `betterleaks`, and `govulncheck` — `mise run ci`
covers the `gomod-tidy`/`govulncheck` equivalents (its `tidy` and
`security` tasks) but, like `check`, never runs `pkl`, `actionlint`,
`zizmor`, or `betterleaks`. See `hk.pkl` for the exact hook steps and
`tasks.toml` for the task graph. Passing `check` locally does not mean
pre-push (or CI) will pass; run `mise x -- hk run pre-push` if you want
that suite before pushing.

Gate on exit codes, never on grepped output — piping a linter through
`tail`/`grep` has silently passed failures before.

Consumer-visible changes are verified through a real host, not just the
suite: build a host CLI (or `_examples/glamour-host`) against the change
and diff its captured outputs — guide index, runbook, schema, exported
skills — against the previous captures. Unexpected drift is a finding;
expected drift is the change's evidence.

Ground rules the linters enforce (see `.claude/rules/` for the full set):

*   Core, export, and docenttest stay stdlib + YAML only — framework imports
    live in adapters (depguard).
*   docent never prints — no `fmt.Print*`; return values, hosts own output
    (forbidigo).
*   Every emitted artifact is deterministic and golden-tested.

## Commits and changelog

*   Conventional-commit messages, enforced by the commit-msg hook.
*   A fix to unreleased work folds into the commit that introduced it —
    pre-release history tells the product story, not the development story.
*   Every commit builds and passes its own tests. After any rebase, sweep
    the rewritten range commit-by-commit; bisectability is a contract.
*   Any consumer-facing `feat`/`fix` ships a [changie](https://changie.dev)
    fragment in the same commit:

    ```sh
    changie new -k <added|changed|fixed|breaking|…> -b "<outcome>" --interactive=false
    git add .changes/unreleased/
    ```

    Not consumer-facing? Add a `Changelog: skip` trailer.
*   Never hand-edit `CHANGELOG.md`; changie assembles it at release time.

## Working against a host CLI

docent is proven against real host CLIs. While an integration is still in
flight, consume it cross-repo with `go.work` so both sides move together
without a tag round-trip; pin a released tag once the integration lands.

## Releases

Tag `vX.Y.Z` → goreleaser publishes a GitHub release with changie-assembled
notes. docent is a library: no binaries, no archives (`builds: skip: true`).
Pre-1.0, breaking changes bump minor.

Group completed work into coherent cuts rather than cutting per change:
fixes accumulate into a 0.0.x, feature waves into a 0.x.0 — and don't
spiral the minor. Fragments land with each change; `changie batch` fires
only at the cut boundary.
