#!/usr/bin/env bash
# Conformance gate for exported skill artifacts. Every format in the export
# registry is exercised through a real host (_examples/glamour-host) and
# validated against its own authority:
#   agent-skill  -> the neutral spec's reference validator (skills-ref)
#   claude-skill -> Claude Code's documented schema (code.claude.com/docs/en/
#                   skills.md); the neutral validator rejects its when_to_use
#                   extension BY DESIGN, so it is checked as an emitted-key
#                   allowlist instead.
# A format with no authority mapping fails the gate: new formats must be
# consciously added here, never silently shipped unvalidated.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root/_examples/glamour-host"

out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT

# The registry is enumerated mechanically via cobra completion, so a format
# added to the code is gated from birth.
formats="$(go run . __complete agent export --format "" 2>/dev/null |
    grep -v '^[:_]' | cut -f1)"

[ -n "$formats" ] || { echo "no export formats enumerated"; exit 1; }

fail=0

for format in $formats; do
    go run . agent export --format "$format" --dir "$out/$format" >/dev/null

    case "$format" in
    agent-skill)
        for dir in "$out/$format"/*/; do
            # bun locally, node's npx on CI runners — same package either way.
            if command -v bunx >/dev/null 2>&1; then
                bunx skills-ref validate "$dir" || fail=1
            else
                npx -y skills-ref validate "$dir" || fail=1
            fi
        done
        ;;
    claude-skill)
        # Emitted frontmatter keys must stay within Claude Code's documented
        # fields that docent consciously emits. A new key here means the
        # emission surface changed: re-verify it against the Claude Code
        # docs, then extend the allowlist.
        for f in "$out/$format"/*/SKILL.md; do
            keys="$(awk '/^---$/{n++; next} n==1 && /^[a-zA-Z_-]+:/{sub(":.*",""); print}' "$f")"
            for key in $keys; do
                case "$key" in
                name | description | when_to_use) ;;
                *)
                    echo "claude-skill $f: undocumented emitted key: $key"
                    fail=1
                    ;;
                esac
            done
        done
        echo "claude-skill: emitted keys within the documented field set"
        ;;
    *)
        echo "format $format has no authority mapping in this gate; add one"
        fail=1
        ;;
    esac
done

# The skills this repo SHIPS (the .claude-plugin pair) meet the same bar as
# the skills docent exports: neutral-spec frontmatter, skills-ref-clean.
for dir in "$repo_root/skills"/*/; do
    if command -v bunx >/dev/null 2>&1; then
        bunx skills-ref validate "$dir" || fail=1
    else
        npx -y skills-ref validate "$dir" || fail=1
    fi
done

exit "$fail"
