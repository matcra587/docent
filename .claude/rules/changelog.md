---
description: >
  Changelog fragments for agents: every user-facing feat/fix ships a changie
  fragment, created non-interactively; the Changelog: skip escape (CI/workflow
  and other non-user-facing changes); and how fragments become CHANGELOG.md.
  Applies to any change, since the convention keys on the commit type, not the
  files touched. Convention, not a hook — nothing enforces it mechanically.
---

# Changelog

User-facing changes are recorded as [changie](https://changie.dev) fragments
under `.changes/unreleased/`, assembled into root `CHANGELOG.md` at release
time. "User-facing" for a library means: anything a consumer sees — exported
API, validation behavior, emitted artifact shapes, error values.

*   **Any user-facing `feat`/`fix` needs a fragment.** Create it
    **non-interactively** — you cannot answer `changie new`'s prompts — and
    stage it, as part of the change:

    ```sh
    changie new -k <added|changed|fixed> -b "<consumer-visible outcome>" --interactive=false
    git add .changes/unreleased/
    ```

    `-k` takes the lowercase kind **key** (not the label): `added`, `changed`,
    `fixed`, `breaking`, `removed`, `deprecated`, `security`, `dependencies`.
    The release notes render the label (`### Added`, `### Breaking Changes`).
    Use `dependencies` for dependency-version bumps, and `breaking` for any
    exported-API change a consumer must react to (pre-1.0, breaking bumps
    minor — see `.changie.yaml`).
*   **Write the body for the release-notes reader**: the consumer-visible
    outcome, imperative, no emoji, no commit SHAs or PR numbers, scope prefix
    stripped.
*   **Only `feat`/`fix` need fragments.** Everything else — `docs`, `chore`,
    `ci`, `build`, `style`, `refactor`, `test` — is exempt; just use the
    honest type. A `feat`/`fix` that genuinely isn't consumer-facing takes a
    `Changelog: skip` commit trailer.
*   **Fragment policy edges**, learned the hard way:
    *   A fix to an *unreleased* feature rides that feature's existing
        fragment — no new fragment; the feature's entry describes the final
        behavior.
    *   One commit with two consumer-visible behaviors gets two fragments
        (each renders as its own release-notes line).
    *   Changes to the Agent Guide Standard's requirements are
        consumer-facing — a new conformance MUST needs a `changed`
        fragment even when no docent code changes.
*   **This is convention, not a hook** — no commit-time check enforces it, so
    it's on you (the agent) to remember the fragment with every feat/fix.
*   **Never hand-edit `CHANGELOG.md`** — changie assembles it.
