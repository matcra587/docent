package docenttest_test

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/matcra587/docent"
	"github.com/matcra587/docent/docenttest"
)

//go:embed testdata/guides/*.md
var exampleGuides embed.FS

// exampleRoot is the minimal command tree that matches the commands listed in
// the example guides under testdata/guides/. It reflects the "example" host
// used as a fixture throughout this package's tests.
var exampleRoot = docent.Command{
	Name: "example",
	Path: "example",
	Children: []docent.Command{
		{
			Name: "agent",
			Path: "example agent",
			Children: []docent.Command{
				{Name: "guide", Path: "example agent guide"},
				{Name: "schema", Path: "example agent schema"},
			},
		},
	},
}

// TestValidate_SelfHosting validates docent's own example guides using
// docenttest.Validate — the library eats its own contract. A failure here
// means the bundled guides are out of spec with the Agent Guide Standard.
func TestValidate_SelfHosting(t *testing.T) {
	t.Parallel()

	sub, err := fs.Sub(exampleGuides, "testdata/guides")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}

	docenttest.Validate(t, sub, exampleRoot)
}

// buildValidGuide returns a well-formed guide file for the given slug and
// optional command list. It is used in violation tests to construct controlled
// fixtures quickly.
func buildValidGuide(slug string, commands []string) []byte {
	lines := []string{
		"---",
		"slug: " + slug,
		"title: " + slug + " title",
		"description: A description for " + slug + ".",
		"when_to_use: When you need " + slug + ".",
		"commands: [" + strings.Join(commands, ", ") + "]",
		"---",
		"",
		"## Decide",
		"Content.",
		"",
		"## Run",
		"Content.",
		"",
		"## Save",
		"Content.",
		"",
		"## Preconditions",
		"Content.",
		"",
		"## Recover",
		"Content.",
		"",
		"## Next",
		"Content.",
	}

	return []byte(strings.Join(lines, "\n"))
}

// captureTB is a minimal testing.TB that records Errorf calls instead of
// failing the test. TestCheck_Violations feeds deliberately invalid guide
// fixtures through Validate and must inspect the message it produces without
// failing this package's own test run.
type captureTB struct {
	testing.TB
	messages []string
}

// Helper is a no-op: captureTB has no caller frame to mark.
func (c *captureTB) Helper() {}

// Errorf records the formatted message instead of failing the test.
func (c *captureTB) Errorf(format string, args ...any) {
	c.messages = append(c.messages, fmt.Sprintf(format, args...))
}

// sentinelPrefixes mirrors the error-class prefixes documented on Validate's
// godoc, letting TestCheck_Violations assert Validate chose the switch branch
// matching each case's sentinel without duplicating the prefix strings.
var sentinelPrefixes = map[error]string{
	docent.ErrMissingFrontmatter:   "missing frontmatter:",
	docent.ErrInvalidFrontmatter:   "invalid frontmatter:",
	docent.ErrMissingField:         "missing required field:",
	docent.ErrSlugMismatch:         "slug mismatch:",
	docent.ErrInvalidSlug:          "invalid slug:",
	docent.ErrDescriptionTooLong:   "description budget:",
	docent.ErrCompatibilityTooLong: "compatibility budget:",
	docent.ErrInvalidSections:      "invalid sections:",
	docent.ErrDuplicateOrder:       "duplicate order:",
	docent.ErrAliasCollision:       "alias collision:",
	docent.ErrMultilineField:       "multiline field:",
	docent.ErrFormFeed:             "form feed:",
	docent.ErrCommandNotFound:      "unknown command reference:",
}

// violationCase describes one guide fixture and the violation Check and
// Validate must report for it.
type violationCase struct {
	name     string
	fsys     fs.FS
	root     docent.Command
	wantErr  error
	wantNone bool // true when Check must return nil
}

// assertCheckViolations checks Check's returned errors against tc: no
// violations for wantNone cases, otherwise at least one error wrapping
// tc.wantErr, with every returned error extractable as *docent.ValidationError.
func assertCheckViolations(t *testing.T, tc violationCase, errs []error) {
	t.Helper()

	if tc.wantNone {
		if len(errs) != 0 {
			t.Errorf("Check: expected no violations, got %d: %v", len(errs), errs)
		}

		return
	}

	if len(errs) == 0 {
		t.Fatalf("Check: expected violations wrapping %v, got none", tc.wantErr)
	}

	found := false
	for _, e := range errs {
		if errors.Is(e, tc.wantErr) {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("Check: expected at least one error wrapping %v; got: %v", tc.wantErr, errs)
	}

	for _, e := range errs {
		var ve *docent.ValidationError
		if !errors.As(e, &ve) {
			t.Errorf("Check: error %v is not *docent.ValidationError", e)
		}
	}
}

// assertValidateViolations checks Validate's captured tb.Errorf messages
// against tc: none for wantNone cases, otherwise at least one message
// carrying the error-class prefix documented on Validate's godoc — proving
// the errors.Is switch chose the branch matching tc.wantErr.
func assertValidateViolations(t *testing.T, tc violationCase, capture *captureTB) {
	t.Helper()

	if tc.wantNone {
		if len(capture.messages) != 0 {
			t.Errorf("Validate: expected no errors, got %d: %v", len(capture.messages), capture.messages)
		}

		return
	}

	wantPrefix, ok := sentinelPrefixes[tc.wantErr]
	if !ok {
		t.Fatalf("Validate: no expected prefix registered for sentinel %v", tc.wantErr)
	}

	for _, m := range capture.messages {
		if strings.HasPrefix(m, wantPrefix) {
			return
		}
	}

	t.Errorf("Validate: expected a message with prefix %q; got: %v", wantPrefix, capture.messages)
}

// TestCheck_Violations verifies that Check detects and distinctly reports each
// violation class by asserting the returned errors wrap the expected
// sentinel, and that Validate reports the same violation through the
// matching tb.Errorf prefix — exercising both functions' error-class
// switches across all 11 documented sentinels.
func TestCheck_Violations(t *testing.T) {
	t.Parallel()

	root := docent.Command{
		Name: "tool",
		Path: "tool",
		Children: []docent.Command{
			{Name: "run", Path: "tool run"},
		},
	}

	cases := []violationCase{
		{
			name: "valid",
			fsys: fstest.MapFS{
				"myguide.md": {Data: buildValidGuide("myguide", []string{"tool run"})},
			},
			root:     root,
			wantNone: true,
		},
		{
			name: "missing_frontmatter",
			fsys: fstest.MapFS{
				"no_fm.md": {Data: []byte("# Not a guide\n\nNo frontmatter block here.")},
			},
			root:    root,
			wantErr: docent.ErrMissingFrontmatter,
		},
		{
			name: "invalid_frontmatter",
			fsys: fstest.MapFS{
				"broken.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: [unbalanced",
					"title: Broken",
					"---",
					"",
					"## Decide",
					"",
					"## Run",
					"",
					"## Save",
					"",
					"## Preconditions",
					"",
					"## Recover",
					"",
					"## Next",
				}, "\n"))},
			},
			root:    root,
			wantErr: docent.ErrInvalidFrontmatter,
		},
		{
			name: "missing_field",
			fsys: fstest.MapFS{
				"missingfield.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: missingfield",
					"description: A guide missing its title.",
					"when_to_use: When testing missing fields.",
					"commands: [tool run]",
					"---",
					"",
					"## Decide",
					"",
					"## Run",
					"",
					"## Save",
					"",
					"## Preconditions",
					"",
					"## Recover",
					"",
					"## Next",
				}, "\n"))},
			},
			root:    root,
			wantErr: docent.ErrMissingField,
		},
		{
			name: "slug_mismatch",
			fsys: fstest.MapFS{
				"mismatched.md": {Data: buildValidGuide("wrongslug", []string{"tool run"})},
			},
			root:    root,
			wantErr: docent.ErrSlugMismatch,
		},
		{
			name: "invalid_slug",
			fsys: fstest.MapFS{
				"bad_slug.md": {Data: buildValidGuide("Bad_Slug", []string{"tool run"})},
			},
			root:    root,
			wantErr: docent.ErrInvalidSlug,
		},
		{
			name: "description_too_long",
			fsys: fstest.MapFS{
				"longdesc.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: longdesc",
					"title: Long Description",
					"description: " + strings.Repeat("a", docent.MaxSkillDescription),
					"when_to_use: When testing the description budget.",
					"commands: [tool run]",
					"---",
					"",
					"## Decide",
					"",
					"## Run",
					"",
					"## Save",
					"",
					"## Preconditions",
					"",
					"## Recover",
					"",
					"## Next",
				}, "\n"))},
			},
			root:    root,
			wantErr: docent.ErrDescriptionTooLong,
		},
		{
			name: "compatibility_too_long",
			fsys: fstest.MapFS{
				"longcompat.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: longcompat",
					"title: Long Compatibility",
					"description: A guide with an oversized compatibility field.",
					"when_to_use: When testing the compatibility budget.",
					"commands: [tool run]",
					"compatibility: " + strings.Repeat("c", docent.MaxSkillCompatibility+1),
					"---",
					"",
					"## Decide",
					"",
					"## Run",
					"",
					"## Save",
					"",
					"## Preconditions",
					"",
					"## Recover",
					"",
					"## Next",
				}, "\n"))},
			},
			root:    root,
			wantErr: docent.ErrCompatibilityTooLong,
		},
		{
			name: "wrong_sections",
			fsys: fstest.MapFS{
				"bad-sections.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: bad-sections",
					"title: Bad Sections",
					"description: Sections in the wrong order.",
					"when_to_use: Never.",
					"commands: [tool run]",
					"---",
					"",
					"## Run",
					"",
					"## Decide",
					"",
					"## Save",
					"",
					"## Preconditions",
					"",
					"## Recover",
					"",
					"## Next",
				}, "\n"))},
			},
			root:    root,
			wantErr: docent.ErrInvalidSections,
		},
		{
			name: "unknown_command_reference",
			fsys: fstest.MapFS{
				"reftest.md": {Data: buildValidGuide("reftest", []string{"tool run", "tool no-such-command"})},
			},
			root:    root,
			wantErr: docent.ErrCommandNotFound,
		},
		{
			name: "duplicate_order",
			fsys: fstest.MapFS{
				"alpha.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: alpha",
					"title: Alpha",
					"description: Alpha guide.",
					"when_to_use: When alpha.",
					"commands: [tool run]",
					"order: 5",
					"---",
					"",
					"## Decide",
					"",
					"## Run",
					"",
					"## Save",
					"",
					"## Preconditions",
					"",
					"## Recover",
					"",
					"## Next",
				}, "\n"))},
				"beta.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: beta",
					"title: Beta",
					"description: Beta guide.",
					"when_to_use: When beta.",
					"commands: [tool run]",
					"order: 5",
					"---",
					"",
					"## Decide",
					"",
					"## Run",
					"",
					"## Save",
					"",
					"## Preconditions",
					"",
					"## Recover",
					"",
					"## Next",
				}, "\n"))},
			},
			root:    root,
			wantErr: docent.ErrDuplicateOrder,
		},
		{
			name: "alias_collision",
			fsys: fstest.MapFS{
				"gamma.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: gamma",
					"title: Gamma",
					"description: Gamma guide.",
					"when_to_use: When gamma.",
					"commands: [tool run]",
					"aliases: [twin]",
					"---",
					"",
					"## Decide",
					"",
					"## Run",
					"",
					"## Save",
					"",
					"## Preconditions",
					"",
					"## Recover",
					"",
					"## Next",
				}, "\n"))},
				"delta.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: delta",
					"title: Delta",
					"description: Delta guide.",
					"when_to_use: When delta.",
					"commands: [tool run]",
					"aliases: [twin]",
					"---",
					"",
					"## Decide",
					"",
					"## Run",
					"",
					"## Save",
					"",
					"## Preconditions",
					"",
					"## Recover",
					"",
					"## Next",
				}, "\n"))},
			},
			root:    root,
			wantErr: docent.ErrAliasCollision,
		},
		{
			name: "zero_root_skips_command_check",
			fsys: fstest.MapFS{
				"anyguide.md": {Data: buildValidGuide("anyguide", []string{"totally-nonexistent-command"})},
			},
			root:     docent.Command{}, // zero value: command check is skipped
			wantNone: true,
		},
		{
			name: "multiline_description",
			fsys: fstest.MapFS{
				"multi.md": {Data: []byte(strings.Replace(
					string(buildValidGuide("multi", []string{"multi run"})),
					"description: A description for multi.",
					"description: |-\n  A description.\n  slug: forged",
					1))},
			},
			root:    root,
			wantErr: docent.ErrMultilineField,
		},
		{
			name: "form_feed_in_body",
			fsys: fstest.MapFS{
				"feed.md": {Data: []byte(strings.Replace(
					string(buildValidGuide("feed", []string{"feed run"})),
					"## Decide\nContent.",
					"## Decide\n\f",
					1))},
			},
			root:    root,
			wantErr: docent.ErrFormFeed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errs := docenttest.Check(tc.fsys, tc.root)
			assertCheckViolations(t, tc, errs)

			capture := &captureTB{}
			docenttest.Validate(capture, tc.fsys, tc.root)
			assertValidateViolations(t, tc, capture)
		})
	}
}
