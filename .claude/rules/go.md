---
description: >
  Go conventions for a published library: the stdlib-only core boundary,
  no-stdout/no-init rules, error design as public API, portability,
  determinism, and where panics are allowed.
paths:
  - "**/*.go"
---

# Go conventions

docent is a published module: its exported identifiers, error values, and
godocs are the product. Every rule here protects the consumer.

## Library boundaries (lint-enforced — do not nolint around them)

*   **Core purity**: core, export, and docenttest import stdlib plus one
    YAML parser only. `cobra`/`kong` are depguard-denied outside the cobra
    adapter package.
*   **No stdout**: docent never prints. `fmt.Print*`/`print`/`println` are
    forbidigo-banned; results are return values, hosts own output through
    `Config.Out` (an `io.Writer`, per the accept-interfaces rule).
*   **No init()**: gochecknoinits is on. Work happens in explicit calls
    (`LoadGuides`), never behind the importer's back. Validation failures
    surface as returned errors, not init-time panics.
*   **go.mod is a contract**: no `replace` directives on main
    (gomoddirectives) — a committed replace breaks every consumer.
*   **Never terminate the host**: no `os.Exit`, `log.Fatal*`, or equivalents
    anywhere in the library — failures are returned errors; only a host's
    `main` may decide to exit.

## Module layout (per go.dev/doc/modules/layout)

*   The root package IS the API: `docent` lives at the module root; the only
    other top-level packages are the deliberate public surface (`export`,
    `cobra`, `docenttest`).
*   **New packages default to `internal/`** — the official guidance is "keep
    packages in internal as much as possible". Moving a package out of
    `internal/` is a public-API decision, not a refactor — make it
    deliberately. This also keeps helper-package dependencies out
    of the surface consumers can reach.
*   Nesting a package under another (`export/skill/`) is naming, not
    protection — the compiler enforces nothing, and it is public API like any
    top-level package. Only `internal/` restricts imports; a supporting
    package that isn't API belongs there, however strongly it "belongs to"
    its parent.
*   Never create `pkg/` — it adds a dead path segment and is absent from the
    official layout.
*   If a binary ever ships, it goes under `cmd/<name>/` with a minimal main;
    today there is none by design (goreleaser skips builds).

## Errors are API

*   Wrap with `fmt.Errorf("context: %w", err)`; strings lowercase, no
    trailing punctuation; include the package name in sentinel messages
    (`"docent: guide not found"`).
*   Expected conditions get sentinel `Err*` variables; errors carrying data
    get `*Error` types (errname enforces both shapes). Consumers must be
    able to branch with `errors.Is`/`errors.As` — never make them match
    message substrings.
*   Validation reports every violation, not just the first: collect with
    `errors.Join` so a guide with three problems fails once with three
    causes.
*   Errors are returned, never logged — docent has no logger by design.
*   `panic` only for programmer invariants unreachable from data. A
    malformed guide file is never a panic — that's a returned error.

## API design

*   Zero values work: optional behavior hangs off a zero-value-usable
    `Config`; required inputs are explicit parameters.
*   Accept interfaces (`fs.FS`, `io.Writer`), return concrete types
    (`*GuideSet`, `Guide`). Deliberate, documented extension points
    (`Extension`) are the exception.
*   No stuttering across the package boundary: `docent.Guide`, not
    `docent.DocentGuide`; `export.AgentSkill`, not
    `export.ExportAgentSkill`.
*   `Sections` stays an ordered slice — the six-section requirement lives
    in validation keyed to `StandardVersion`, never in the type. Standard
    revisions must not break the parse types.
*   Assert interface compliance at compile time where a type's purpose is to
    satisfy an interface: `var _ Renderer = (*AgentSkill)(nil)`.
*   Copy slices and maps at API boundaries, both directions: never store a
    caller-provided slice/map, never return internal ones — a `GuideSet`
    must stay immutable after `LoadGuides` no matter what callers do to
    returned values.
*   No embedding in exported structs — it leaks implementation, inhibits
    evolution, and obscures godocs; delegate explicitly.
*   No mutable package-level state; the only package-level vars are sentinel
    `Err*` values and compiled regexps.
*   Marshaled types spell out their field tags (`yaml:"slug"`,
    `json:"flag_groups"`) — the serialized name is a contract and must not
    silently follow a field rename.
*   Optional behavior hangs off the `Config` struct, deliberately not
    functional options — a deliberate decision; don't "upgrade" to options
    without agreement.
*   Getters drop the `Get` prefix (`g.Order()`, not `g.GetOrder()`);
    setters keep `Set`. One-method interfaces take `-er` names
    (`Renderer`), and canonical method names (`Read`, `Write`, `Close`,
    `String`) are never reused with non-canonical signatures or semantics.
*   Simple lookups signal absence with comma-ok — `Get(slug) (Guide, bool)`
    — errors are for failures, not for "not present".
*   Every exported identifier gets a godoc comment ending in a period
    (godot); the comment states behavior and constraints, not a
    restatement of the signature.
*   Where Effective Go (2009, unmaintained by its own disclaimer) conflicts
    with these rules — notably its endorsement of `init()` — these rules
    win.

## Portability and determinism

*   docent is cross-platform: Linux, macOS, and Windows are equal targets.
    Pure Go (`CGO_ENABLED=0`); `filepath` for paths; no OS-specific
    assumptions (separators, filesystem case-sensitivity, line endings).
    Guide files may arrive with CRLF — parsing must normalize.
*   Deterministic output everywhere: sort before emitting, no map-order
    iteration into artifacts, no timestamps or randomness in generated
    files (exported skills carry a static generated-do-not-edit header).

## Security posture

*   Guide content is first-party, but export writes into host-supplied
    directories: writes go through `os.Root` so a path component resolving
    outside the target — a planted symlink, `..`, an absolute path — is
    refused at open time. Lexical checks (`filepath.IsLocal`) are
    pre-flight only; they provably cannot catch symlinks (gosec G304
    applies outside testdata loading).
*   No secrets anywhere in this repo — no tokens in fixtures, examples, or
    docs; gosec G101 gates CI.
