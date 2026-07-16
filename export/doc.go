// Package export renders docent guides to exportable text formats.
//
// All skill renderers share one common definition — the Agent Skills open
// standard (agentskills.io): a <name>/SKILL.md layout whose frontmatter
// carries a name matching the parent directory and a description within the
// spec's 1024-character cap. AgentSkill renders exactly that portable shape;
// harness-specific renderers (ClaudeSkill) layer their harness's extensions
// on top of it without ever diverging from the layout or the name rules.
//
// Hosts use the export package directly when they need fine-grained control
// over rendered output, or indirectly through the cobra adapter's
// "agent export --format <fmt> --dir <dir>" command.
package export
