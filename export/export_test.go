package export_test

import (
	"strings"
	"testing"

	"github.com/matcra587/docent"
	"github.com/matcra587/docent/export"
)

// sampleGuide returns a well-formed Guide for use in renderer tests.
func sampleGuide() docent.Guide {
	order := 1

	return docent.Guide{
		Slug:        "bravo",
		Title:       "Bravo Guide",
		Description: "The bravo runbook.",
		WhenToUse:   "When you need bravo.",
		Commands:    []string{"bravo run", "bravo list"},
		Order:       &order,
		Sections: []docent.Section{
			{Heading: "Decide", Body: "Which bravo variant to use."},
			{Heading: "Run", Body: "```sh\nbravo run\n```"},
			{Heading: "Save", Body: "Capture the run ID."},
			{Heading: "Preconditions", Body: "Auth must be valid."},
			{Heading: "Recover", Body: "Re-authenticate on 401."},
			{Heading: "Next", Body: "See alpha."},
		},
	}
}

// TestAgentSkill_relPath verifies the artifact path is <slug>/SKILL.md.
func TestAgentSkill_relPath(t *testing.T) {
	t.Parallel()

	r := export.AgentSkill{}

	if got, want := r.RelPath(sampleGuide()), "bravo/SKILL.md"; got != want {
		t.Errorf("RelPath = %q, want %q", got, want)
	}
}

// TestAgentSkill_frontmatterSingleLine verifies that a description far past
// any YAML folding width still renders as one frontmatter line — the promise
// that line-oriented frontmatter parsers are never handed folded scalars.
func TestAgentSkill_frontmatterSingleLine(t *testing.T) {
	t.Parallel()

	g := sampleGuide()
	g.Description = strings.Repeat("very long description ", 20)

	got := export.AgentSkill{}.Render(g)

	fmEnd := strings.Index(got[4:], "\n---\n")
	if fmEnd < 0 {
		t.Fatalf("no closing frontmatter delimiter:\n%s", got)
	}

	fm := got[4 : 4+fmEnd]
	if lines := strings.Count(fm, "\n") + 1; lines != 2 {
		t.Errorf("frontmatter has %d lines, want 2 (one per field):\n%s", lines, fm)
	}
}

// TestClaudeSkill_relPath verifies the Claude variant shares the common
// <slug>/SKILL.md layout.
func TestClaudeSkill_relPath(t *testing.T) {
	t.Parallel()

	if got, want := (export.ClaudeSkill{}).RelPath(sampleGuide()), "bravo/SKILL.md"; got != want {
		t.Errorf("RelPath = %q, want %q", got, want)
	}
}

// TestClaudeSkill_frontmatterSeparateKeys verifies the Claude Code variant's
// distinguishing behavior: description and when_to_use render as separate
// frontmatter keys, unlike AgentSkill's single composed description field.
func TestClaudeSkill_frontmatterSeparateKeys(t *testing.T) {
	t.Parallel()

	g := sampleGuide()

	got := export.ClaudeSkill{}.Render(g)

	want := "---\n" +
		"name: bravo\n" +
		`description: "The bravo runbook."` + "\n" +
		`when_to_use: "When you need bravo."` + "\n" +
		"---\n\n"

	if !strings.HasPrefix(got, want) {
		t.Errorf("frontmatter = %q, want prefix %q", got, want)
	}
}

// TestAgentSkill_emptySectionBody verifies that a section with an empty body
// renders without extra blank lines collapsing.
func TestAgentSkill_emptySectionBody(t *testing.T) {
	t.Parallel()

	g := docent.Guide{
		Slug:        "sparse",
		Title:       "Sparse Guide",
		Description: "A guide with empty section bodies.",
		WhenToUse:   "Always.",
		Commands:    []string{"sparse run"},
		Sections: []docent.Section{
			{Heading: "Decide", Body: ""},
			{Heading: "Run", Body: ""},
			{Heading: "Save", Body: ""},
			{Heading: "Preconditions", Body: ""},
			{Heading: "Recover", Body: ""},
			{Heading: "Next", Body: ""},
		},
	}

	r := export.AgentSkill{}
	got := r.Render(g)

	// Must contain all section headings even when bodies are empty.
	for _, h := range []string{"Decide", "Run", "Save", "Preconditions", "Recover", "Next"} {
		if !strings.Contains(got, "## "+h) {
			t.Errorf("empty-body render missing section %q\ngot: %s", h, got)
		}
	}
}

// TestAgentSkill_optionalFields verifies renderSkill's optional-field
// passthrough: license, compatibility, allowed-tools, and metadata (sorted by
// key) all render as single-line quoted scalars when populated.
func TestAgentSkill_optionalFields(t *testing.T) {
	t.Parallel()

	g := sampleGuide()
	g.License = "MIT"
	g.Compatibility = "Requires bravo CLI >= 2.0."
	g.AllowedTools = []string{"Bash", "Read"}
	g.Metadata = map[string]string{
		"zeta":  "last",
		"alpha": "first",
		"mid":   "middle",
	}

	got := export.AgentSkill{}.Render(g)

	want := "---\n" +
		"name: bravo\n" +
		`description: "The bravo runbook. Use when: When you need bravo."` + "\n" +
		`license: "MIT"` + "\n" +
		`compatibility: "Requires bravo CLI >= 2.0."` + "\n" +
		`allowed-tools: "Bash Read"` + "\n" +
		"metadata:\n" +
		`  "alpha": "first"` + "\n" +
		`  "mid": "middle"` + "\n" +
		`  "zeta": "last"` + "\n" +
		"---\n\n"

	if !strings.HasPrefix(got, want) {
		t.Errorf("frontmatter = %q, want prefix %q", got, want)
	}
}

// TestRunbook_shape pins the reading-oriented render: H1 title, every section
// as an H2 (empty bodies keep their heading, same contract as the skill
// renderers), and neither frontmatter nor the generated header.
func TestRunbook_shape(t *testing.T) {
	t.Parallel()

	g := docent.Guide{
		Slug:  "sparse",
		Title: "Sparse Guide",
		Sections: []docent.Section{
			{Heading: "Decide", Body: "Pick one."},
			{Heading: "Run", Body: ""},
			{Heading: "Save", Body: ""},
			{Heading: "Preconditions", Body: ""},
			{Heading: "Recover", Body: ""},
			{Heading: "Next", Body: ""},
		},
	}

	got := export.Runbook{}.Render(g)

	// The exact bytes are the spec: H1 title, every section as an H2 in
	// order (empty bodies keep their heading), no frontmatter, no generated
	// header, trailing newline.
	want := "# Sparse Guide\n\n" +
		"## Decide\n\nPick one.\n\n" +
		"## Run\n\n\n\n" +
		"## Save\n\n\n\n" +
		"## Preconditions\n\n\n\n" +
		"## Recover\n\n\n\n" +
		"## Next\n\n\n"

	if got != want {
		t.Errorf("runbook shape mismatch:\n--- want\n%q\n+++ got\n%q", want, got)
	}
}
