package docent

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// slugPattern is the Agent Skills name rule (agentskills.io specification):
// lowercase alphanumeric runs separated by single hyphens — no uppercase, no
// underscores, no leading, trailing, or consecutive hyphens. Slugs become
// exported skill names and their directory names, so conformance is enforced
// here and exports conform by construction.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// maxSlugLen is the Agent Skills spec's name length cap.
const maxSlugLen = 64

// frontmatter holds the parsed YAML header of a guide file.
// Field tags name the serialized keys; they are part of the file contract
// and must not silently follow a field rename.
type frontmatter struct {
	Slug        string   `yaml:"slug"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	WhenToUse   string   `yaml:"when_to_use"`
	Commands    []string `yaml:"commands"`
	Aliases     []string `yaml:"aliases"`
	Order       *int     `yaml:"order"`

	// Optional Agent Skills passthrough fields, emitted in exported skill
	// frontmatter. Names track the spec (allowed_tools follows the
	// underscore style the rest of this frontmatter uses).
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  []string          `yaml:"allowed_tools"`
}

// LoadGuides reads every *.md file from fsys, parses its YAML frontmatter and
// Markdown sections, validates each guide against StandardVersion 1, and
// returns a canonically ordered GuideSet.
//
// Canonical order: guides with an order value ascending (ties broken by slug),
// followed by unordered guides sorted alphabetically by slug.
//
// Fatal errors (missing frontmatter, absent required fields, slug≠filename,
// wrong or misordered sections) cause LoadGuides to return nil and a joined
// error covering every failing guide.
//
// Duplicate order values are validation failures like every other class: the
// canonical order is a contract, and two guides claiming the same slot is a
// guide-authoring bug caught in the author's own CI (docenttest.Validate).
// On any failure the returned GuideSet is nil — a non-nil error is never
// paired with a usable set.
func LoadGuides(fsys fs.FS) (*GuideSet, error) {
	entries, err := fs.Glob(fsys, "*.md")
	if err != nil {
		return nil, fmt.Errorf("docent: glob guides: %w", err)
	}

	var (
		guides []Guide
		errs   []error
	)

	for _, entry := range entries {
		raw, readErr := fs.ReadFile(fsys, entry)
		if readErr != nil {
			errs = append(errs, fmt.Errorf("docent: read %s: %w", entry, readErr))

			continue
		}

		stem := strings.TrimSuffix(path.Base(entry), ".md")

		g, parseErr := parseGuide(stem, raw)
		if parseErr != nil {
			errs = append(errs, parseErr)

			continue
		}

		guides = append(guides, g)
	}

	// Duplicate order and alias-collision detection need every successfully
	// parsed guide, so they run after the per-file pass and join the same
	// fatal error set.
	errs = append(errs, collectDuplicateOrderErrors(guides)...)
	errs = append(errs, collectAliasCollisionErrors(guides)...)

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	canonicalSort(guides)

	return &GuideSet{guides: guides}, nil
}

// parseGuide parses a single guide from its stem (filename without .md) and raw bytes.
func parseGuide(stem string, raw []byte) (Guide, error) {
	filename := stem + ".md"

	// The form feed is reserved as the concatenation separator
	// (export.GuideSeparator); rejecting it here makes the separator's
	// "cannot appear in guide content" contract true by construction.
	if bytes.ContainsRune(raw, '\f') {
		return Guide{}, &ValidationError{
			File:    filename,
			Message: "form feed character in guide content",
			Err:     ErrFormFeed,
		}
	}

	fmBytes, body, err := splitFrontmatter(raw)
	if err != nil {
		return Guide{}, &ValidationError{
			File:    filename,
			Message: "missing frontmatter block",
			Err:     ErrMissingFrontmatter,
		}
	}

	var fm frontmatter
	if err = yaml.Unmarshal(fmBytes, &fm); err != nil {
		return Guide{}, &ValidationError{
			File:    filename,
			Message: fmt.Sprintf("invalid frontmatter YAML: %v", err),
			Err:     ErrInvalidFrontmatter,
		}
	}

	if fieldErr := validateFields(filename, fm); fieldErr != nil {
		return Guide{}, fieldErr
	}

	if fm.Slug != stem {
		return Guide{}, &ValidationError{
			File:    filename,
			Message: fmt.Sprintf("slug %q does not match filename %q", fm.Slug, filename),
			Err:     ErrSlugMismatch,
		}
	}

	sections, err := parseSections(filename, body)
	if err != nil {
		return Guide{}, err
	}

	// fm is function-local and freshly unmarshaled, so its fields alias
	// nothing; only raw is caller-owned and must be detached. Optional lists
	// normalize empty to nil so absent and explicitly-empty spell the same
	// downstream (Commands stays as parsed: empty-but-present is meaningful
	// there and validated above).
	aliases := fm.Aliases
	if len(aliases) == 0 {
		aliases = nil
	}

	metadata := fm.Metadata
	if len(metadata) == 0 {
		metadata = nil
	}

	allowedTools := fm.AllowedTools
	if len(allowedTools) == 0 {
		allowedTools = nil
	}

	return Guide{
		Slug:          fm.Slug,
		Title:         fm.Title,
		Description:   fm.Description,
		WhenToUse:     fm.WhenToUse,
		Commands:      fm.Commands,
		Aliases:       aliases,
		Order:         fm.Order,
		Sections:      sections,
		Raw:           bytes.Clone(raw),
		License:       fm.License,
		Compatibility: fm.Compatibility,
		Metadata:      metadata,
		AllowedTools:  allowedTools,
	}, nil
}

// splitFrontmatter splits a guide file into its YAML frontmatter bytes and
// Markdown body bytes. It normalizes CRLF to LF before splitting.
// An error is returned if no valid frontmatter block is found.
func splitFrontmatter(src []byte) (fm, body []byte, err error) {
	// bytes.ReplaceAll copies the input even with zero matches; skip the
	// normalization entirely for the common LF-only file.
	if bytes.ContainsRune(src, '\r') {
		src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
		src = bytes.ReplaceAll(src, []byte("\r"), []byte("\n"))
	}

	// A valid guide starts exactly with "---\n".
	if !bytes.HasPrefix(src, []byte("---\n")) {
		return nil, nil, ErrMissingFrontmatter
	}

	after := src[4:] // skip "---\n"

	// Find the closing "---" on its own line. A whole-line match (not a
	// bare substring search) so a frontmatter value containing a
	// column-0 "---" — e.g. inside malformed YAML — cannot falsely
	// terminate parsing early and silently truncate trailing fields.
	lines := bytes.Split(after, []byte("\n"))

	closeIdx := -1

	for i, line := range lines {
		if bytes.Equal(line, []byte("---")) {
			closeIdx = i

			break
		}
	}

	if closeIdx == -1 {
		return nil, nil, ErrMissingFrontmatter
	}

	fm = bytes.Join(lines[:closeIdx], []byte("\n"))
	rest := bytes.Join(lines[closeIdx+1:], []byte("\n"))

	return fm, rest, nil
}

// validateFields checks that all required frontmatter fields are present,
// that the slug satisfies the Agent Skills name rules, and that description
// and when_to_use fit the exported skill description budget.
// It joins multiple violations rather than stopping at the first.
func validateFields(filename string, fm frontmatter) error {
	var errs []error

	if fm.Slug == "" {
		errs = append(errs, &ValidationError{
			File:    filename,
			Message: "missing required field: slug",
			Err:     ErrMissingField,
		})
	} else if slugErr := validateSlugFormat(filename, fm.Slug); slugErr != nil {
		errs = append(errs, slugErr)
	}

	if fm.Title == "" {
		errs = append(errs, &ValidationError{
			File:    filename,
			Message: "missing required field: title",
			Err:     ErrMissingField,
		})
	}

	if fm.Description == "" {
		errs = append(errs, &ValidationError{
			File:    filename,
			Message: "missing required field: description",
			Err:     ErrMissingField,
		})
	}

	if fm.WhenToUse == "" {
		errs = append(errs, &ValidationError{
			File:    filename,
			Message: "missing required field: when_to_use",
			Err:     ErrMissingField,
		})
	}

	// An explicitly empty list (commands: []) is valid and means the guide
	// applies to the whole tool — cross-cutting guides have no single
	// command to cite honestly. Only an ABSENT field is a violation.
	if fm.Commands == nil {
		errs = append(errs, &ValidationError{
			File:    filename,
			Message: "missing required field: commands",
			Err:     ErrMissingField,
		})
	}

	errs = append(errs, collectMultilineFieldErrors(filename, fm)...)

	// The budget applies to the composed skill description an export emits;
	// the spec counts characters, so runes, not bytes. Empty fields are
	// already reported above; checking only non-empty pairs keeps each
	// violation class to a single error.
	if fm.Description != "" && fm.WhenToUse != "" {
		composed := utf8.RuneCountInString(composeSkillDescription(fm.Description, fm.WhenToUse))
		if composed > MaxSkillDescription {
			errs = append(errs, &ValidationError{
				File: filename,
				Message: fmt.Sprintf(
					"description and when_to_use compose to %d characters; exported skill descriptions cap at %d",
					composed, MaxSkillDescription),
				Err: ErrDescriptionTooLong,
			})
		}
	}

	// Compatibility is optional, but when present the spec caps it; runes,
	// not bytes, for the same reason as the description budget.
	if fm.Compatibility != "" {
		if n := utf8.RuneCountInString(fm.Compatibility); n > MaxSkillCompatibility {
			errs = append(errs, &ValidationError{
				File: filename,
				Message: fmt.Sprintf(
					"compatibility is %d characters; exported skill compatibility caps at %d",
					n, MaxSkillCompatibility),
				Err: ErrCompatibilityTooLong,
			})
		}
	}

	return errors.Join(errs...)
}

// collectMultilineFieldErrors reports every frontmatter value the guide
// index emits as a single key: value line that contains a newline: YAML
// block scalars legally carry newlines, and a multiline value would corrupt
// — or forge entries in — the line-oriented index shape. Absent fields are
// reported separately as missing.
func collectMultilineFieldErrors(filename string, fm frontmatter) []error {
	checks := []struct{ name, value string }{
		{"title", fm.Title},
		{"description", fm.Description},
		{"when_to_use", fm.WhenToUse},
	}

	for _, c := range fm.Commands {
		checks = append(checks, struct{ name, value string }{fmt.Sprintf("commands entry %q", c), c})
	}

	for _, a := range fm.Aliases {
		checks = append(checks, struct{ name, value string }{fmt.Sprintf("aliases entry %q", a), a})
	}

	var errs []error

	for _, c := range checks {
		if strings.ContainsAny(c.value, "\n\r") {
			errs = append(errs, &ValidationError{
				File:    filename,
				Message: c.name + " must be a single line",
				Err:     ErrMultilineField,
			})
		}
	}

	return errs
}

// validateSlugFormat checks a non-empty slug against the Agent Skills name
// rules: at most maxSlugLen characters, lowercase alphanumerics and hyphens
// only, no leading, trailing, or consecutive hyphens.
func validateSlugFormat(filename, slug string) error {
	if runes := utf8.RuneCountInString(slug); runes > maxSlugLen {
		return &ValidationError{
			File:    filename,
			Message: fmt.Sprintf("slug %q is %d characters; the maximum is %d", slug, runes, maxSlugLen),
			Err:     ErrInvalidSlug,
		}
	}

	if !slugPattern.MatchString(slug) {
		return &ValidationError{
			File: filename,
			Message: fmt.Sprintf(
				"slug %q must be lowercase alphanumerics and hyphens with no leading, trailing, or consecutive hyphens",
				slug),
			Err: ErrInvalidSlug,
		}
	}

	return nil
}

// parseSections parses the Markdown body into an ordered slice of Section
// values. It returns a *ValidationError wrapping ErrInvalidSections when the
// body does not have the six headings required by StandardVersion in the
// correct order.
func parseSections(filename string, body []byte) ([]Section, error) {
	// body arrives CRLF-normalized: splitFrontmatter normalizes the whole
	// file before splitting, so no second pass is needed here.
	lines := strings.Split(string(body), "\n")

	type rawSection struct {
		heading string
		lines   []string
	}

	var sections []rawSection

	for _, line := range lines {
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			sections = append(sections, rawSection{heading: heading})
		} else if len(sections) > 0 {
			sections[len(sections)-1].lines = append(sections[len(sections)-1].lines, line)
		} else if strings.TrimSpace(line) != "" {
			// Non-blank content before the first heading has nowhere to
			// live in the section model; silently dropping it would lose
			// guide content without any signal in the pipeline.
			return nil, &ValidationError{
				File:    filename,
				Message: fmt.Sprintf("content before first section heading: %q", strings.TrimSpace(line)),
				Err:     ErrInvalidSections,
			}
		}
	}

	if len(sections) != len(requiredSections) {
		return nil, &ValidationError{
			File: filename,
			Message: fmt.Sprintf("expected %d sections (%s), got %d",
				len(requiredSections),
				strings.Join(requiredSections[:], ", "),
				len(sections)),
			Err: ErrInvalidSections,
		}
	}

	for i, s := range sections {
		if s.heading != requiredSections[i] {
			return nil, &ValidationError{
				File: filename,
				Message: fmt.Sprintf("section %d: expected heading %q, got %q",
					i+1, requiredSections[i], s.heading),
				Err: ErrInvalidSections,
			}
		}
	}

	result := make([]Section, len(sections))
	for i, s := range sections {
		result[i] = Section{
			Heading: s.heading,
			Body:    strings.TrimRight(strings.Join(s.lines, "\n"), "\n"),
		}
	}

	return result, nil
}

// collectDuplicateOrderErrors scans guides for duplicate order values and
// returns a *ValidationError wrapping ErrDuplicateOrder for each collision.
// Results are sorted for deterministic output.
func collectDuplicateOrderErrors(guides []Guide) []error {
	orderToSlugs := make(map[int][]string)

	for _, g := range guides {
		if g.Order != nil {
			orderToSlugs[*g.Order] = append(orderToSlugs[*g.Order], g.Slug)
		}
	}

	// Collect duplicate order values in sorted order for determinism.
	var dupeOrders []int

	for o, slugs := range orderToSlugs {
		if len(slugs) > 1 {
			dupeOrders = append(dupeOrders, o)
		}
	}

	sort.Ints(dupeOrders)

	var violations []error

	for _, o := range dupeOrders {
		slugs := orderToSlugs[o]
		sort.Strings(slugs)

		violations = append(violations, &ValidationError{
			File:    strings.Join(slugs, ", "),
			Message: fmt.Sprintf("duplicate order value %d", o),
			Err:     ErrDuplicateOrder,
		})
	}

	return violations
}

// collectAliasCollisionErrors reports every alias that is empty, declared
// twice, or shadows a slug — an ambiguous name cannot resolve, so collisions
// fail the load like any other validation class. Results are deterministic.
func collectAliasCollisionErrors(guides []Guide) []error {
	slugs := make(map[string]struct{}, len(guides))
	for _, g := range guides {
		slugs[g.Slug] = struct{}{}
	}

	aliasOwner := make(map[string]string)

	var violations []error

	for _, g := range guides {
		for _, alias := range g.Aliases {
			_, shadowsSlug := slugs[alias]

			switch {
			case strings.TrimSpace(alias) == "":
				violations = append(violations, &ValidationError{
					File:    g.Slug + ".md",
					Message: "empty alias",
					Err:     ErrAliasCollision,
				})

			case shadowsSlug:
				violations = append(violations, &ValidationError{
					File:    g.Slug + ".md",
					Message: fmt.Sprintf("alias %q shadows a guide slug", alias),
					Err:     ErrAliasCollision,
				})

			case aliasOwner[alias] != "":
				violations = append(violations, &ValidationError{
					File:    g.Slug + ".md",
					Message: fmt.Sprintf("alias %q already declared by %s.md", alias, aliasOwner[alias]),
					Err:     ErrAliasCollision,
				})

			default:
				aliasOwner[alias] = g.Slug
			}
		}
	}

	return violations
}

// canonicalSort sorts guides in place: ordered guides ascending by order
// (ties broken by slug), then unordered guides alphabetically by slug.
func canonicalSort(guides []Guide) {
	sort.SliceStable(guides, func(i, j int) bool {
		gi, gj := guides[i], guides[j]

		switch {
		case gi.Order != nil && gj.Order != nil:
			if *gi.Order != *gj.Order {
				return *gi.Order < *gj.Order
			}

			return gi.Slug < gj.Slug

		case gi.Order != nil:
			return true

		case gj.Order != nil:
			return false

		default:
			return gi.Slug < gj.Slug
		}
	})
}
