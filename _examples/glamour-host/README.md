# glamour-host

A runnable docent host demonstrating consumer-aware guide rendering: docent
emits plain Markdown through `Config.Out`, and the host — not docent —
decides who gets styling.

*   Agent harness detected (`CLAUDECODE`, `CODEX_THREAD_ID`) → raw Markdown,
    even inside a PTY.
*   No TTY (pipe, redirect) → raw Markdown.
*   Interactive human terminal → styled output via
    [glamour v2](https://github.com/charmbracelet/glamour) (`charm.land/glamour/v2`).

This directory is its own Go module on purpose: glamour and its transitive
dependencies must never enter docent's `go.mod`. The `replace` directive
points at the repository root; a consumer copying this pattern uses a
released docent version instead.

```sh
go run . guide safe-mutation           # styled when stdout is a TTY
go run . guide safe-mutation | cat     # raw Markdown (pipe)
CLAUDECODE=1 go run . guide safe-mutation  # raw Markdown (agent harness)
```

No host styling at all is also fine — the raw output pipes cleanly through
any Markdown pager: `go run . guide safe-mutation | glow -p -`.
