package docent

import (
	"bytes"
	"io"
	"iter"
	"maps"
	"slices"
	"strings"
)

// StandardVersion is the Agent Guide Standard revision that validation
// enforces. Version 1 requires exactly six sections in the prescribed order.
const StandardVersion = 1

// MaxSkillDescription is the character budget for an exported skill's
// description frontmatter, taken from the Agent Skills spec
// (agentskills.io), which caps skill descriptions at 1024 characters.
// Validation enforces it at load time against SkillDescription — the exact
// composed value the agent-skill export emits.
const MaxSkillDescription = 1024

// MaxSkillCompatibility is the character budget for an exported skill's
// optional compatibility frontmatter, taken from the Agent Skills spec,
// which caps it at 500 characters.
const MaxSkillCompatibility = 500

// requiredSections lists the section headings for StandardVersion 1 in the
// required order. Changing these requires bumping StandardVersion.
//
// This is a package-level array rather than a constant because Go has no
// constant slices; it is never mutated.
var requiredSections = [...]string{
	"Decide",
	"Run",
	"Save",
	"Preconditions",
	"Recover",
	"Next",
}

// SectionHeadings returns the section headings StandardVersion requires, in
// their required order. Surfaces that enumerate the section vocabulary —
// shell completion, documentation, tooling — derive from this accessor so
// they cannot drift from validation when the standard revises. The returned
// slice is a copy; mutating it affects nothing.
func SectionHeadings() []string {
	return slices.Clone(requiredSections[:])
}

// Section is a single named heading and its prose body within a guide.
type Section struct {
	// Heading is the bare heading text without the leading "## ".
	Heading string

	// Body is the content that follows the heading, up to the next heading
	// or end of file, with trailing newlines stripped. Leading blank lines
	// and other trailing whitespace (spaces, tabs) are preserved verbatim.
	// Empty bodies are valid per the standard.
	Body string
}

// Guide is a single loaded and validated runbook.
// All fields are populated by LoadGuides; the zero value is not meaningful.
// Guide is a plain, field-only struct: constructing one directly (rather
// than through LoadGuides) bypasses load-time validation — slug charset,
// single-line title, and the other invariants LoadGuides enforces — and
// nothing in the type itself will catch the omission. Downstream consumers,
// including this package's export functionality, assume those invariants
// already hold and may produce malformed output for a hand-built Guide that
// violates them.
type Guide struct {
	// Slug is the unique identifier derived from the filename.
	Slug string

	// Title is the human-readable name of the guide.
	Title string

	// Description is a short summary for index views.
	Description string

	// WhenToUse describes the conditions under which an agent should consult this guide.
	WhenToUse string

	// Commands lists the CLI command paths relevant to this guide.
	Commands []string

	// License names the license applied to exported skills built from this
	// guide, or references a bundled license file. Optional; emitted in
	// skill frontmatter when present.
	License string

	// Compatibility states environment requirements for exported skills
	// (intended product, system packages, network access). Optional; the
	// Agent Skills spec caps it at MaxSkillCompatibility characters.
	Compatibility string

	// Metadata carries additional string key-value properties for exported
	// skill frontmatter — the spec's slot for anything it does not define.
	// Optional; emitted with sorted keys for determinism.
	Metadata map[string]string

	// AllowedTools lists tools pre-approved for exported skills (the spec's
	// experimental allowed-tools field). Optional; emitted space-separated.
	AllowedTools []string

	// Aliases lists alternative names Get resolves to this guide — typically
	// the names a guide carried before a rename, so agents and scripts that
	// memorized them keep working. Aliases are not held to the slug charset
	// (legacy names are the point) but must be unique across the set and
	// must not shadow any slug.
	Aliases []string

	// Order is the optional sparse-integer for canonical ordering. Nil means
	// the guide has no explicit order and sorts alphabetically after ordered guides.
	Order *int

	// Sections contains the guide body in its required six-section shape,
	// in the order they appear in the file.
	Sections []Section

	// Raw holds the complete original file bytes, including frontmatter.
	Raw []byte
}

// Section returns the guide section whose heading matches name
// case-insensitively — the lookup behind section-scoped serving surfaces
// like the cobra adapter's --section flag, owned here so adapters cannot
// drift in matching semantics. The second return value is false when no
// section matches, mirroring GuideSet.Get.
func (g Guide) Section(name string) (Section, bool) {
	for _, s := range g.Sections {
		if strings.EqualFold(s.Heading, name) {
			return s, true
		}
	}

	return Section{}, false
}

// SkillDescription returns the guide's description and when_to_use composed
// into the single description field the Agent Skills open standard defines
// for exported skills. It is the value the agent-skill export emits and the
// value load-time validation holds to MaxSkillDescription — a single source
// so the two can never drift apart.
func (g Guide) SkillDescription() string {
	return composeSkillDescription(g.Description, g.WhenToUse)
}

// composeSkillDescription is the one place the description/when_to_use
// composition is spelled out; Guide.SkillDescription and frontmatter
// validation both delegate here.
func composeSkillDescription(description, whenToUse string) string {
	return description + " Use when: " + whenToUse
}

// GuideSet is a validated, canonically ordered collection of guides.
// It is immutable: LoadGuides populates it once, and no method mutates it.
// A nil *GuideSet behaves as an empty set — Guides returns nil, Get reports
// false, Len returns 0 — so hosts holding an optional set (Config.Guides
// documents nil as "no guides") can call methods without a guard.
type GuideSet struct {
	guides []Guide
}

// Guides returns the guides in canonical order.
// The returned guides are deep copies; callers cannot mutate the GuideSet
// through them, including through slice and pointer fields.
func (gs *GuideSet) Guides() []Guide {
	if gs == nil {
		return nil
	}

	out := make([]Guide, len(gs.guides))
	for i, g := range gs.guides {
		out[i] = copyGuide(g)
	}

	return out
}

// All returns an iterator over the guides in canonical order, yielding a
// deep copy per guide exactly as Guides does — but lazily, so a consumer
// that stops early never pays for copying the rest of the corpus, and no
// second slice is retained. Prefer it over Guides when iterating; Guides
// remains the snapshot form. A nil *GuideSet yields nothing.
func (gs *GuideSet) All() iter.Seq[Guide] {
	return func(yield func(Guide) bool) {
		if gs == nil {
			return
		}

		for _, g := range gs.guides {
			if !yield(copyGuide(g)) {
				return
			}
		}
	}
}

// Get returns the guide with the given slug, or — when no slug matches —
// the guide declaring the name as an alias, so renamed guides keep answering
// to their old names. The second return value is false when the name resolves
// to nothing. The returned guide is a deep copy; mutating it cannot affect
// the GuideSet.
func (gs *GuideSet) Get(slug string) (Guide, bool) {
	if gs == nil {
		return Guide{}, false
	}

	for _, g := range gs.guides {
		if g.Slug == slug {
			return copyGuide(g), true
		}
	}

	// Slugs always win over aliases; validation forbids an alias shadowing
	// a slug, so this order only matters for defense in depth.
	for _, g := range gs.guides {
		if slices.Contains(g.Aliases, slug) {
			return copyGuide(g), true
		}
	}

	return Guide{}, false
}

// Resolve returns the guide a typed name refers to, applying progressively
// looser matching so agents holding an imperfect name still land on the
// right guide: exact slug, declared alias (both as Get), then the normalized
// form — case-folded with underscores read as hyphens, so "Auth_Setup"
// resolves auth-setup — and finally a substring of a normalized slug or
// alias, accepted only when it matches exactly one guide. An ambiguous
// substring resolves to nothing rather than guessing. The second return
// value is false when nothing matches; the returned guide is a deep copy.
//
// Use Get when only declared names should answer — Resolve is for lookup
// surfaces serving humans and agents, like the cobra adapter's guide
// command.
func (gs *GuideSet) Resolve(name string) (Guide, bool) {
	if g, ok := gs.Get(name); ok {
		return g, true
	}

	if gs == nil {
		return Guide{}, false
	}

	norm := normalizeGuideName(name)
	if norm == "" {
		return Guide{}, false
	}

	// Normalized-exact pass, then substring pass, both uniqueness-guarded:
	// a name matching two guides is ambiguous — in either tier — and
	// guessing would hand back the wrong runbook silently. Validation
	// compares raw names only, so two guides CAN collide post-normalization
	// (slug "auth-setup", alias "Auth_Setup" elsewhere).
	if g, ok := uniqueMatch(gs.guides, func(s string) bool {
		return normalizeGuideName(s) == norm
	}); ok {
		return copyGuide(g), true
	}

	if g, ok := uniqueMatch(gs.guides, func(s string) bool {
		return strings.Contains(normalizeGuideName(s), norm)
	}); ok {
		return copyGuide(g), true
	}

	return Guide{}, false
}

// uniqueMatch returns the single guide whose slug or any alias satisfies
// match; ok is false for zero or multiple matching guides.
func uniqueMatch(guides []Guide, match func(string) bool) (Guide, bool) {
	var (
		found Guide
		count int
	)

	for _, g := range guides {
		if match(g.Slug) || slices.ContainsFunc(g.Aliases, match) {
			found = g
			count++
		}
	}

	if count == 1 {
		return found, true
	}

	return Guide{}, false
}

// normalizeGuideName folds a guide name for loose matching: lowercased, with
// underscores read as hyphens — the two separators humans and agents swap
// most.
func normalizeGuideName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "-")
}

// copyGuide returns a deep copy of g so API boundaries never share backing
// memory with the GuideSet's internal state. The stdlib Clone helpers
// preserve nil-for-nil, so absent optional fields stay absent.
func copyGuide(g Guide) Guide {
	out := g
	out.Commands = slices.Clone(g.Commands)
	out.Aliases = slices.Clone(g.Aliases)
	out.Metadata = maps.Clone(g.Metadata)
	out.AllowedTools = slices.Clone(g.AllowedTools)
	out.Sections = slices.Clone(g.Sections)
	out.Raw = bytes.Clone(g.Raw)

	if g.Order != nil {
		order := *g.Order
		out.Order = &order
	}

	return out
}

// Len returns the number of guides in the set.
func (gs *GuideSet) Len() int {
	if gs == nil {
		return 0
	}

	return len(gs.guides)
}

// Config is the host integration surface for docent. The zero value is usable;
// fields are populated by the host as needed. Hosts mount the agent command
// group by passing a populated Config to the adapter's NewCommand function.
type Config struct {
	// Guides is the guide set to serve. Nil means no guides are available.
	Guides *GuideSet

	// Command is the framework-neutral schema IR for the host's command tree.
	// Hosts build this from an adapter (e.g. cobra.Tree(root)) before mounting
	// the agent command group, so the schema represents the host CLI without
	// the agent commands themselves.
	Command Command

	// Out is the host-supplied destination for all emitted output. When nil,
	// adapter commands write to their framework's output channel (e.g.
	// cobra's cmd.OutOrStdout()). docent itself never writes to stdout.
	Out io.Writer

	// ContractVersion is the host's own agent-contract version — the number
	// a host bumps when its envelope, exit codes, or output shapes change,
	// distinct from docent's guide StandardVersion. When set, adapters stamp
	// it on discovery surfaces: the schema root gains a "contract_version"
	// extensions entry and the guide index gains a contract_version line, so
	// an agent can pin behavior to the contract it read. Empty omits the
	// stamp everywhere. The value is emitted verbatim into a single
	// "key: value" index line — keep it one line with no colons; a plain
	// semver string always qualifies. A value containing a newline,
	// carriage return, or colon fails index emission with an error
	// (export.ErrInvalidContractVersion) rather than corrupting the
	// line-oriented shape.
	ContractVersion string
}
