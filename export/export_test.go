package export_test

import (
	"errors"
	"fmt"
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

// TestSkillRenderers_nameQualifier pins the shared qualified-name contract:
// both built-ins use the same final name for the direct-child directory and
// frontmatter without mutating the source guide.
func TestSkillRenderers_nameQualifier(t *testing.T) {
	t.Parallel()

	renderers := []struct {
		name     string
		renderer export.Renderer
	}{
		{name: "agent skill", renderer: export.AgentSkill{NameQualifier: "jira"}},
		{name: "claude skill", renderer: export.ClaudeSkill{NameQualifier: "jira"}},
	}

	for _, tc := range renderers {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := sampleGuide()
			got := tc.renderer.Render(g)

			if rel, want := tc.renderer.RelPath(g), "jira-bravo/SKILL.md"; rel != want {
				t.Errorf("RelPath = %q, want %q", rel, want)
			}

			if !strings.HasPrefix(got, "---\nname: jira-bravo\n") {
				t.Errorf("qualified frontmatter name missing:\n%s", got)
			}

			if g.Slug != "bravo" {
				t.Errorf("source Guide.Slug mutated to %q", g.Slug)
			}
		})
	}
}

// TestSkillRenderers_invalidFinalName verifies validation is applied after
// qualifier composition and covers the Agent Skills length, character, and
// hyphen rules for both built-in formats.
func TestSkillRenderers_invalidFinalName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		qualifier string
		slug      string
	}{
		{name: "empty", slug: ""},
		{name: "too long", qualifier: strings.Repeat("a", 64), slug: "bravo"},
		{name: "uppercase", qualifier: "Jira", slug: "bravo"},
		{name: "underscore", qualifier: "jira_cli", slug: "bravo"},
		{name: "leading hyphen", qualifier: "-jira", slug: "bravo"},
		{name: "consecutive hyphens", qualifier: "jira-", slug: "bravo"},
		{name: "trailing hyphen", slug: "bravo-"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := sampleGuide()
			g.Slug = tc.slug

			renderers := []export.ValidatingRenderer{
				export.AgentSkill{NameQualifier: tc.qualifier},
				export.ClaudeSkill{NameQualifier: tc.qualifier},
			}

			for _, r := range renderers {
				if err := r.Validate(g); !errors.Is(err, export.ErrInvalidSkillName) {
					t.Errorf("Validate error = %v, want errors.Is(err, ErrInvalidSkillName)", err)
				}
			}
		})
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

// ExampleAgentSkill demonstrates applying a host qualifier without changing
// the source guide slug.
func ExampleAgentSkill() {
	g := docent.Guide{
		Slug:        "core-contract",
		Title:       "Core contract",
		Description: "The host contract.",
		WhenToUse:   "When integrating the CLI.",
	}
	r := export.AgentSkill{NameQualifier: "jira"}

	fmt.Println(r.RelPath(g))
	fmt.Println(strings.Split(r.Render(g), "\n")[1])
	fmt.Println(g.Slug)

	// Output:
	// jira-core-contract/SKILL.md
	// name: jira-core-contract
	// core-contract
}
