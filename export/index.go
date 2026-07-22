package export

import (
	"fmt"
	"strings"

	"github.com/matcra587/docent"
)

// GuideSeparator divides guides in concatenated output (Concat, served by
// the cobra adapter as "agent guide --all"). Load validation rejects form
// feeds in guide content (docent.ErrFormFeed), so splitting on a form-feed
// line is unambiguous by construction — unlike "---", which every guide's
// frontmatter opens with.
const GuideSeparator = "\f\n"

// Index renders the frontmatter-only guide index — the token-economical
// discovery view every agent reads first: a header line with the guide
// count, an optional contract_version line, then one key: value block per
// guide in the given order (pass GuideSet.Guides() for canonical order).
//
// contractVersion is emitted verbatim on its own line when non-empty; a
// value that would corrupt the line-oriented shape — one containing a
// newline, carriage return, or colon, or entirely whitespace — errors with
// ErrInvalidContractVersion, so a misconfigured host fails loudly instead
// of serving an unparsable index. Guide fields need no such guard: load
// validation already rejects multiline values for every field emitted here.
func Index(guides []docent.Guide, contractVersion string) (string, error) {
	if contractVersion != "" &&
		(strings.ContainsAny(contractVersion, "\n\r:") || strings.TrimSpace(contractVersion) == "") {
		return "", fmt.Errorf(
			"%w: %q must be a non-blank single line without colons", ErrInvalidContractVersion, contractVersion)
	}

	var sb strings.Builder

	fmt.Fprintf(&sb, "# Agent Guide Index (%d guides)\n", len(guides))

	// The host contract version rides the index — the one surface every
	// agent reads first — so behavior can be pinned without a schema call.
	if contractVersion != "" {
		fmt.Fprintf(&sb, "contract_version: %s\n", contractVersion)
	}

	for _, g := range guides {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "slug: %s\n", g.Slug)
		fmt.Fprintf(&sb, "title: %s\n", g.Title)
		fmt.Fprintf(&sb, "description: %s\n", g.Description)
		fmt.Fprintf(&sb, "when_to_use: %s\n", g.WhenToUse)
		fmt.Fprintf(&sb, "commands: %s\n", strings.Join(g.Commands, ", "))

		// Aliases surface in the index so an agent holding an old name can
		// map it to the canonical slug without a failed lookup first.
		if len(g.Aliases) > 0 {
			fmt.Fprintf(&sb, "aliases: %s\n", strings.Join(g.Aliases, ", "))
		}

		if g.Order != nil {
			fmt.Fprintf(&sb, "order: %d\n", *g.Order)
		}
	}

	return sb.String(), nil
}

// Concat returns every guide's raw content concatenated in the given order,
// each terminated by a newline and separated by GuideSeparator lines so
// consumers can split the output unambiguously.
func Concat(guides []docent.Guide) string {
	var sb strings.Builder

	for i, g := range guides {
		if i > 0 {
			sb.WriteString(GuideSeparator)
		}

		sb.Write(g.Raw)
		sb.WriteString("\n")
	}

	return sb.String()
}
