package export

import (
	"strings"

	"github.com/matcra587/docent"
)

// Runbook renders a docent.Guide to a reading-oriented Markdown runbook: the
// title as an H1 heading, then each section as an H2 heading and its body, in
// the guide's validated canonical order. The YAML frontmatter is omitted —
// slug, commands, and order are machine-routing metadata, redundant with the
// index the reader already consulted to pick the guide — and there is no
// generated-do-not-edit header, because the output is served, not written to
// a file a user could edit.
//
// This is the recommended shape for a single-guide view such as
// "agent guide <slug>"; frontmatter belongs to the index and skill export.
// Runbook is presentation, not a file artifact, so it deliberately does not
// implement Renderer.
//
// Output is deterministic and byte-stable for identical input. A section
// with an empty body keeps its heading rather than collapsing, the same
// contract the skill renderers honor.
type Runbook struct{}

// Render returns the runbook text for g.
func (Runbook) Render(g docent.Guide) string {
	var b strings.Builder

	b.WriteString("# ")
	b.WriteString(g.Title)

	for _, s := range g.Sections {
		b.WriteString("\n\n## ")
		b.WriteString(s.Heading)
		b.WriteString("\n\n")
		b.WriteString(s.Body)
	}

	b.WriteString("\n")

	return b.String()
}
