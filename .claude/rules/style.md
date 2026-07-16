---
description: >
  Go style and the lint roster golangci-lint enforces: the enable-list
  rationale, depguard core-purity bans, the forbidigo no-stdout ban, and
  nolint discipline.
paths:
  - "**/*.go"
---

# Go Style & Lint

Formatting is tooling's job: gofumpt (`extra-rules: true`), gated by
`mise run check`. See [go.md](go.md) for library boundaries and error
design, [project.md](project.md) for the architecture map.

## Code style (judgment calls the tools can't make)

*   US spelling in code, comments, and identifiers — misspell runs locale
    `US`, so `behaviour`/`colour` fail the build.
*   Comments state rationale and constraints — why this design, what breaks
    without it — never what the next line does.
*   Handle errors and edge cases first (early return); drop `else` when the
    `if` body returns; keep the happy path at minimal indentation.
*   Composite literals use field names, never positional fields.
*   No stuttering across package boundaries: `docent.Guide`, not
    `docent.DocentGuide`.
*   Short names at short scopes (`g` for a guide in a three-line loop);
    exported names spell it out.

## Enforced: golangci-lint (`.golangci.yml`)

`default: standard` plus a curated enable list where every entry protects a
bug class or a published-module contract: correctness (errorlint, nilerr,
nilnesserr, errchkjson, forcetypeassert, reassign,
gocheckcompilerdirectives), public-API style (gocritic, revive, godot,
misspell, predeclared, errname, asciicheck), complexity (gocyclo, nestif),
quality (unconvert, goconst), security (gosec, bidichk), library hygiene
(depguard, gomoddirectives, gochecknoinits, forbidigo), testing
(testifylint, thelper, usetesting, tparallel, testableexamples), and
modernization (modernize, exhaustive, nolintlint). No ruleguard — docent
has no custom rules yet.

### depguard bans (core purity — see [go.md](go.md))

| Banned | Where | Why |
|--------|-------|-----|
| `github.com/spf13/cobra` | everywhere except the cobra adapter | framework knowledge is adapter-only |
| `github.com/alecthomas/kong` | everywhere | kong adapter deferred beyond v0 |
| `io/ioutil` | everywhere | deprecated since Go 1.16 |

### forbidigo ban (no stdout)

`fmt.Print`/`Printf`/`Println` and builtin `print`/`println` are banned —
docent never writes to stdout; results are return values and hosts own
output. There is no logger to reach for either; return errors instead.

### Test relaxations

`_test.go` files are exempt from goconst (repeated literals are the point
of table tests) and gosec G304 (golden/fixture loading by constructed
path). Nothing else is relaxed — tests meet the same bar as production
code.

## Known linter deadlock

`exhaustive` rejects a default-only switch over a huge enum (`reflect.Kind`)
while `staticcheck` QF1002 demands the tagged switch back. Resolve with
early-return `if` chains — which the control-flow rule above prefers
anyway — never with a `nolint` or by weakening either linter.

## nolint discipline

`nolintlint` is on: every `//nolint` needs a specific linter and a reason
(`//nolint:gosec // G304: path is cleaned and scoped two lines up`). The
repo currently carries zero; keep additions rare and reviewed — especially
never around the depguard/forbidigo boundary rules.
