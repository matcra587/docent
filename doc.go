// Package docent loads, validates, and serves embedded Markdown runbooks
// (guides) for AI agents, and models a CLI's command tree as a
// framework-neutral schema. It is a library — it never writes to stdout and
// never terminates the host process; all results are returned values, and
// hosts own all output.
//
// Guides are YAML-frontmatter Markdown files embedded via go:embed. Each
// file must conform to the Agent Guide Standard
// (https://github.com/matcra587/docent/blob/main/docs/agent-guide-standard.md).
// LoadGuides is the primary entry point; all other types are its output.
//
// The module is organized as five public packages:
//
//   - docent (this package): the guide and schema model, loading, and
//     validation — Guide, Section, GuideSet, Command, Flag, FlagGroup,
//     SchemaRegistry, and Config, the host integration surface.
//   - docent/export: renderers that turn guides into exportable artifacts,
//     such as Agent Skills SKILL.md files.
//   - docent/cobra: the cobra adapter — a Tree walker producing the neutral
//     Command schema from a live cobra tree, and NewCommand, the mountable
//     "agent" command group.
//   - docent/harness: agent-runtime detection from environment markers —
//     which agent is driving the process, and which harness's skill
//     conventions apply.
//   - docent/docenttest: contract-test helpers so hosts can validate their
//     own guides in CI.
//
// Every emitted artifact — guide index, concatenated guides, schema JSON,
// exported skills — is deterministic and byte-stable by contract.
package docent
