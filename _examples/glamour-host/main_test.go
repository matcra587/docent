package main

import (
	"strings"
	"testing"
)

// TestIndexToMarkdown pins the host-side index reformat: docent's
// frontmatter-style index becomes Markdown with the title heading each
// guide, the buffered slug as its invocation, decorated known fields, and
// unknown keys rendered generically instead of dropped.
func TestIndexToMarkdown(t *testing.T) {
	t.Parallel()

	index := strings.Join([]string{
		"# Agent Guide Index (2 guides)",
		"contract_version: 1.0.0",
		"",
		"slug: safe-mutation",
		"title: Safe Mutation",
		"description: Change state safely.",
		"when_to_use: Before any write.",
		"commands: host apply",
		"",
		"slug: core-contract",
		"title: Core Contract",
		"description: Output contract.",
		"when_to_use: Parsing output.",
		"commands: ",
		"aliases: contract",
		"",
	}, "\n")

	got := indexToMarkdown(index)

	for _, want := range []string{
		"# Agent Guide Index (2 guides)",
		"**contract version:** 1.0.0",
		"## Safe Mutation\n\n`guide safe-mutation`",
		"Change state safely.",
		"**When:** Before any write.",
		"**Commands:** `host apply`",
		"## Core Contract\n\n`guide core-contract`",
		"**aliases:** contract",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("reformatted index missing %q\ngot:\n%s", want, got)
		}
	}

	if strings.Contains(got, "slug:") {
		t.Errorf("raw slug line leaked into Markdown:\n%s", got)
	}

	if strings.Contains(got, "**Commands:** ``") {
		t.Errorf("empty commands rendered as empty code span:\n%s", got)
	}
}
