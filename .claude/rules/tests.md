---
description: >
  Testing philosophy and conventions: few, outcome-level tests that exercise
  docent through a real mounted CLI surface; golden tests as the spec;
  docenttest self-hosting; no coverage-driven test sprawl.
paths:
  - "**/*_test.go"
  - "testdata/**"
---

# Tests

## Philosophy: few tests, real surface

Tests are executable specifications, and every test is a maintenance cost.
We are NOT aiming for thousands of tests or a coverage number — we are
aiming for one test per promise the library makes; if a test doesn't pin
a promise (or a validation failure class), question whether it should
exist.

*   **Test docent as a CLI, not as functions.** The primary suite mounts
    docent's `agent` command group on a small fixture host (a minimal cobra
    root under `testdata/` or an internal test harness), then drives it the
    way an agent would: `SetArgs(...)`, execute, capture output — and
    goldens the bytes. `agent guide`, `agent guide <slug> --section run`,
    `agent schema --path x`, `agent export` — each exercised through the
    real command surface, because that IS the product.
*   **One behavior, one test.** A new unit test must justify itself:
    internals get tested directly only when a behavior can't be reached
    through the command surface (e.g. a parser corner the CLI would mask).
    Prefer adding a case to an existing table over adding a test function.
*   **A test that can skip its own subject is worse than no test.** A
    guard whose fixture gets rejected upstream goes green-by-skip forever
    and reads as coverage — one such test hid a real path-escape
    vulnerability. If the subject is unreachable through the public
    surface, test it white-box instead of skipping.
*   **Don't test the framework.** No tests that cobra parses flags, that
    go:embed embeds, or that YAML unmarshals — test docent's contracts
    only.

## Conventions

*   **Framework**: standard library `testing`, table-driven, lowercase
    named subtests. testify allowed as a helper (testifylint enforces
    idiomatic use), not as a framework.
*   **Golden tests are the spec** for every emitted artifact (index,
    concatenation, schema JSON, exported skills): fixtures under
    `testdata/`, byte-exact comparison, an `-update` flag to regenerate.
    A golden diff in review IS the behavior change — regenerate
    deliberately, never loosen a comparison to unblock a change.
*   **Validation failure classes** (missing frontmatter, slug≠filename,
    wrong/misordered sections, unknown command reference, duplicate order)
    are one table, one case each — asserted via `errors.Is`/`errors.As`,
    never message strings.
*   **docenttest self-hosts**: docent's own example guides are validated by
    `docenttest.Validate` in this repo's CI — the library eats its own
    contract.
*   **Black-box by default**: `package docent_test` against the public API;
    white-box only where unavoidable, with a comment saying why.
*   **Examples are documentation**: exported API gets `Example*` functions
    with `// Output:` (testableexamples enforces) — pkg.go.dev renders
    them, the compiler checks them. These count as tests; don't duplicate
    them with an assertion-style twin.
*   **Parallel and race-clean**: `t.Parallel()` on independent tests, CI
    runs `-race`, `t.TempDir`/`t.Setenv`/`t.Helper` per usetesting/thelper.
*   **Determinism check**: artifact tests run the emission twice and assert
    identical bytes — map-order bugs surface here, not in a consumer's
    diff.
