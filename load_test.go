package docent_test

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/matcra587/docent"
)

// update regenerates golden files when passed to "go test -update".
var update = flag.Bool("update", false, "update golden files")

// validFS is a well-formed two-guide filesystem used for the passing-case test.
// bravo (order=1) comes before alpha (no order) in canonical output.
var validFS = fstest.MapFS{
	"bravo.md": {Data: []byte(strings.Join([]string{
		"---",
		"slug: bravo",
		"title: Bravo Guide",
		"description: The bravo runbook.",
		"when_to_use: When you need bravo.",
		"commands: [bravo run, bravo list]",
		"order: 1",
		"---",
		"",
		"## Decide",
		"Which bravo variant to use.",
		"",
		"## Run",
		"```sh",
		"bravo run",
		"```",
		"",
		"## Save",
		"Capture the run ID.",
		"",
		"## Preconditions",
		"Auth must be valid.",
		"",
		"## Recover",
		"Re-authenticate on 401.",
		"",
		"## Next",
		"See alpha.",
	}, "\n"))},

	"alpha.md": {Data: []byte(strings.Join([]string{
		"---",
		"slug: alpha",
		"title: Alpha Guide",
		"description: The alpha runbook.",
		"when_to_use: When you need alpha.",
		"commands: [alpha run]",
		"---",
		"",
		"## Decide",
		"Which alpha form.",
		"",
		"## Run",
		"```sh",
		"alpha run",
		"```",
		"",
		"## Save",
		"Note the result.",
		"",
		"## Preconditions",
		"Token required.",
		"",
		"## Recover",
		"Retry on timeout.",
		"",
		"## Next",
		"See bravo.",
	}, "\n"))},
}

// TestLoadGuides runs each validation case through LoadGuides and compares the
// result (or error) against a golden file under testdata/golden/.
// Run with -update to regenerate golden files.
func TestLoadGuides(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		fsys    fstest.MapFS
		wantErr error // sentinel expected in the returned error; nil means success
	}{
		{
			name: "valid",
			fsys: validFS,
		},
		{
			name: "missing_frontmatter",
			fsys: fstest.MapFS{
				"no_fm.md": {Data: []byte(strings.Join([]string{
					"# Not a guide",
					"",
					"No frontmatter block here.",
				}, "\n"))},
			},
			wantErr: docent.ErrMissingFrontmatter,
		},
		{
			name: "missing_field_title",
			fsys: fstest.MapFS{
				"no-title.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: no-title",
					"description: Missing the title field.",
					"when_to_use: Always.",
					"commands: [no-title run]",
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
			wantErr: docent.ErrMissingField,
		},
		{
			// commands: [] is explicit whole-tool scope, valid.
			name: "empty_commands_whole_tool",
			fsys: fstest.MapFS{
				"contract.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: contract",
					"title: Contract Guide",
					"description: Applies to every command.",
					"when_to_use: Always.",
					"commands: []",
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
		},
		{
			name: "missing_field_commands",
			fsys: fstest.MapFS{
				"no-commands.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: no-commands",
					"title: No Commands",
					"description: Missing the commands field.",
					"when_to_use: Always.",
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
			wantErr: docent.ErrMissingField,
		},
		{
			name: "slug_mismatch",
			fsys: fstest.MapFS{
				"correct-name.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: wrong-slug",
					"title: Slug Mismatch Guide",
					"description: The slug does not match the filename.",
					"when_to_use: Never.",
					"commands: [correct-name run]",
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
			wantErr: docent.ErrSlugMismatch,
		},
		{
			name: "invalid_slug",
			fsys: fstest.MapFS{
				"bad_slug.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: bad_slug",
					"title: Invalid Slug Guide",
					"description: Underscores violate the Agent Skills name rules.",
					"when_to_use: Never.",
					"commands: [bad run]",
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
			wantErr: docent.ErrInvalidSlug,
		},
		{
			name: "description_budget",
			fsys: fstest.MapFS{
				"verbose.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: verbose",
					"title: Verbose Guide",
					"description: " + strings.Repeat("d", docent.MaxSkillDescription),
					"when_to_use: Never.",
					"commands: [verbose run]",
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
			wantErr: docent.ErrDescriptionTooLong,
		},
		{
			name: "missing_sections",
			fsys: fstest.MapFS{
				"few-sections.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: few-sections",
					"title: Too Few Sections",
					"description: Only three sections present.",
					"when_to_use: Never.",
					"commands: [few-sections run]",
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
				}, "\n"))},
			},
			wantErr: docent.ErrInvalidSections,
		},
		{
			name: "wrong_section_order",
			fsys: fstest.MapFS{
				"bad-order.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: bad-order",
					"title: Wrong Section Order",
					"description: Sections in the wrong order.",
					"when_to_use: Never.",
					"commands: [bad-order run]",
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
			wantErr: docent.ErrInvalidSections,
		},
		{
			// Content before the first heading has nowhere to live in the
			// section model; it must be flagged, not silently dropped from
			// the loaded guide.
			name: "content_before_first_heading",
			fsys: fstest.MapFS{
				"leaky.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: leaky",
					"title: Leaky Guide",
					"description: Content leaks before the first heading.",
					"when_to_use: Never.",
					"commands: [leaky run]",
					"---",
					"",
					"Stray paragraph before any heading.",
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
			wantErr: docent.ErrInvalidSections,
		},
		{
			name: "invalid_frontmatter",
			fsys: fstest.MapFS{
				"broken.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: [unclosed",
					"---",
					"",
					"## Decide",
				}, "\n"))},
			},
			wantErr: docent.ErrInvalidFrontmatter,
		},
		{
			// A column-0 line that STARTS WITH "---" but is not itself a
			// bare "---" line (it has trailing content, "not-a-real-
			// delimiter") sits before the real closing delimiter. A loose
			// substring search on "\n---" would false-match this line,
			// silently truncating the frontmatter and dropping
			// when_to_use/commands — surfacing a misleading ErrMissingField
			// instead of the real problem. The fix's whole-line match must
			// skip past it to the true closing delimiter; the frontmatter
			// is then correctly reassembled, which is invalid YAML on its
			// own (the stray line has no ":"), so this must fail with a
			// clear ErrInvalidFrontmatter.
			name: "frontmatter_stray_delimiter",
			fsys: fstest.MapFS{
				"stray.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: stray",
					"title: Stray Guide",
					"description: A stray delimiter appears below.",
					"---not-a-real-delimiter",
					"when_to_use: Always.",
					"commands: [stray run]",
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
			wantErr: docent.ErrInvalidFrontmatter,
		},
		{
			// Composes to exactly MaxSkillDescription runes — and uses a
			// multibyte rune so byte-counting would reject it: pins both the
			// > boundary and the characters-not-bytes promise.
			name: "description_budget_exact",
			fsys: fstest.MapFS{
				"exact.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: exact",
					"title: Exact Guide",
					"description: " + strings.Repeat("é", docent.MaxSkillDescription-13),
					"when_to_use: W.",
					"commands: [exact run]",
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
		},
		{
			name: "description_budget_over_by_one",
			fsys: fstest.MapFS{
				"overone.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: overone",
					"title: Over Guide",
					"description: " + strings.Repeat("d", docent.MaxSkillDescription-12),
					"when_to_use: W.",
					"commands: [overone run]",
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
			wantErr: docent.ErrDescriptionTooLong,
		},
		{
			name: "compatibility_budget",
			fsys: fstest.MapFS{
				"needy.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: needy",
					"title: Needy Guide",
					"description: Compatibility exceeds the budget.",
					"when_to_use: Never.",
					"commands: [needy run]",
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
			wantErr: docent.ErrCompatibilityTooLong,
		},
		{
			// Exactly the documented maximum must load; pins the > boundary.
			name: "compatibility_budget_exact",
			fsys: fstest.MapFS{
				"fits.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: fits",
					"title: Fits Guide",
					"description: Compatibility at exactly the cap.",
					"when_to_use: Always.",
					"commands: [fits run]",
					"compatibility: " + strings.Repeat("c", docent.MaxSkillCompatibility),
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
		},
		{
			// Two guides declaring the same alias: ambiguous, fatal.
			name: "alias_collision",
			fsys: fstest.MapFS{
				"first.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: first",
					"title: First Guide",
					"description: Declares old-name.",
					"when_to_use: When first.",
					"commands: [first run]",
					"aliases: [old-name]",
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
				"second.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: second",
					"title: Second Guide",
					"description: Also declares old-name.",
					"when_to_use: When second.",
					"commands: [second run]",
					"aliases: [old-name]",
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
			wantErr: docent.ErrAliasCollision,
		},
		{
			// An alias shadowing another guide's slug: ambiguous, fatal.
			name: "alias_shadows_slug",
			fsys: fstest.MapFS{
				"first.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: first",
					"title: First Guide",
					"description: Aliases the second slug.",
					"when_to_use: When first.",
					"commands: [first run]",
					"aliases: [second]",
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
				"second.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: second",
					"title: Second Guide",
					"description: The shadowed guide.",
					"when_to_use: When second.",
					"commands: [second run]",
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
			wantErr: docent.ErrAliasCollision,
		},
		{
			name: "duplicate_order",
			fsys: fstest.MapFS{
				"first.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: first",
					"title: First Guide",
					"description: First guide with order 5.",
					"when_to_use: When first.",
					"commands: [first run]",
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
				"second.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: second",
					"title: Second Guide",
					"description: Second guide also with order 5.",
					"when_to_use: When second.",
					"commands: [second run]",
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
			wantErr: docent.ErrDuplicateOrder,
		},
		{
			// A YAML block scalar legally carries newlines; the index emits
			// this field as one key: value line, so a multiline value is a
			// forgery vector and fails the load.
			name: "multiline_description",
			fsys: fstest.MapFS{
				"sneaky.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: sneaky",
					"title: Sneaky Guide",
					"description: |-",
					"  A description.",
					"  slug: forged",
					"when_to_use: When sneaking.",
					"commands: [sneaky run]",
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
			wantErr: docent.ErrMultilineField,
		},
		{
			// The form feed is the reserved concatenation separator; content
			// containing one would split "agent guide --all" ambiguously.
			name: "form_feed_in_body",
			fsys: fstest.MapFS{
				"feed.md": {Data: []byte(strings.Join([]string{
					"---",
					"slug: feed",
					"title: Feed Guide",
					"description: The feed runbook.",
					"when_to_use: When feeding.",
					"commands: [feed run]",
					"---",
					"",
					"## Decide",
					"\f",
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
			wantErr: docent.ErrFormFeed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gs, err := docent.LoadGuides(tc.fsys)

			// Determine expected outcome and format the golden content.
			var got string

			switch {
			case tc.wantErr != nil:
				// Fatal error: GuideSet must be nil, error must wrap the sentinel.
				if gs != nil {
					t.Errorf("LoadGuides(%q): expected nil GuideSet on error, got non-nil", tc.name)
				}

				if err == nil {
					t.Fatalf("LoadGuides(%q): expected error wrapping %v, got nil", tc.name, tc.wantErr)
				}

				if !errors.Is(err, tc.wantErr) {
					t.Errorf("LoadGuides(%q): error %v does not wrap %v", tc.name, err, tc.wantErr)
				}

				var ve *docent.ValidationError
				if !errors.As(err, &ve) {
					t.Errorf("LoadGuides(%q): expected *docent.ValidationError in chain, got %T", tc.name, err)
				}

				got = err.Error()

			default:
				// Success: GuideSet non-nil, no error.
				if err != nil {
					t.Fatalf("LoadGuides(%q): unexpected error: %v", tc.name, err)
				}

				if gs == nil {
					t.Fatalf("LoadGuides(%q): expected non-nil GuideSet, got nil", tc.name)
				}

				got = formatGuideSet(gs)
			}

			checkGolden(t, tc.name, got)
		})
	}
}

// TestGuideSet_nilReceiver pins the promise that a nil *GuideSet behaves as an
// empty set (Config.Guides documents nil as "no guides available"); a host
// calling methods on it must not be taken down.
func TestGuideSet_nilReceiver(t *testing.T) {
	t.Parallel()

	var gs *docent.GuideSet

	if got := gs.Len(); got != 0 {
		t.Errorf("nil GuideSet Len() = %d, want 0", got)
	}

	if got := gs.Guides(); got != nil {
		t.Errorf("nil GuideSet Guides() = %v, want nil", got)
	}

	if g, ok := gs.Get("any"); ok {
		t.Errorf("nil GuideSet Get() = %v, true; want zero Guide, false", g)
	}

	if g, ok := gs.Resolve("any"); ok {
		t.Errorf("nil GuideSet Resolve() = %v, true; want zero Guide, false", g)
	}

	for g := range gs.All() {
		t.Errorf("nil GuideSet All() yielded %v, want nothing", g)
	}
}

// TestGuideSet_All pins the iterator's promises: canonical order matching
// Guides, boundary deep copies (mutating a yielded guide cannot affect the
// set), and early exit stopping the iteration.
func TestGuideSet_All(t *testing.T) {
	t.Parallel()

	gs, err := docent.LoadGuides(fstest.MapFS{
		"alpha.md": {Data: guideFile("alpha", "Alpha", "")},
		"bravo.md": {Data: guideFile("bravo", "Bravo", "order: 1")},
	})
	if err != nil {
		t.Fatalf("LoadGuides: %v", err)
	}

	var slugs []string

	for g := range gs.All() {
		slugs = append(slugs, g.Slug)

		// Mutating the yielded copy must not reach the set.
		g.Commands[0] = "mutated"
	}

	if want := []string{"bravo", "alpha"}; !slices.Equal(slugs, want) {
		t.Errorf("All() order = %v, want %v", slugs, want)
	}

	if g, _ := gs.Get("bravo"); g.Commands[0] == "mutated" {
		t.Error("mutation through a yielded guide reached the GuideSet")
	}

	count := 0
	for range gs.All() {
		count++

		break
	}

	if count != 1 {
		t.Errorf("early exit yielded %d guides, want 1", count)
	}
}

// guideFile builds a minimal valid guide file body: the given slug and
// title, one optional extra frontmatter line (e.g. "aliases: [x]"; empty
// adds none), and the six standard sections.
func guideFile(slug, title, extra string) []byte {
	fm := []string{
		"---",
		"slug: " + slug,
		"title: " + title,
		"description: The " + slug + " runbook.",
		"when_to_use: When you need " + slug + ".",
		"commands: [" + slug + " run]",
	}

	if extra != "" {
		fm = append(fm, extra)
	}

	fm = append(fm,
		"---",
		"",
		"## Decide", "d.", "",
		"## Run", "r.", "",
		"## Save", "s.", "",
		"## Preconditions", "p.", "",
		"## Recover", "rec.", "",
		"## Next", "n.",
	)

	return []byte(strings.Join(fm, "\n"))
}

// TestGuideSet_Resolve pins the loose-lookup ladder: exact slug, alias,
// normalized form (case fold, underscore=hyphen), unique substring — and
// that an ambiguous substring or empty query resolves to nothing.
func TestGuideSet_Resolve(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"auth-setup.md":  {Data: guideFile("auth-setup", "Auth setup", "aliases: [login-help]")},
		"auth-tokens.md": {Data: guideFile("auth-tokens", "Auth tokens", "")},
		"safe-write.md":  {Data: guideFile("safe-write", "Safe write", "")},
	}

	gs, err := docent.LoadGuides(fsys)
	if err != nil {
		t.Fatalf("LoadGuides: %v", err)
	}

	tests := []struct {
		name     string
		query    string
		wantSlug string
		wantOK   bool
	}{
		{"exact slug", "auth-setup", "auth-setup", true},
		{"declared alias", "login-help", "auth-setup", true},
		{"case folded", "Auth-Setup", "auth-setup", true},
		{"underscore separator", "auth_setup", "auth-setup", true},
		{"normalized alias", "Login_Help", "auth-setup", true},
		{"unique substring", "safe", "safe-write", true},
		{"unique substring of alias", "login", "auth-setup", true},
		{"ambiguous substring", "auth", "", false},
		{"no match", "missing", "", false},
		{"empty query", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g, ok := gs.Resolve(tt.query)
			if ok != tt.wantOK {
				t.Fatalf("Resolve(%q) ok = %v, want %v", tt.query, ok, tt.wantOK)
			}

			if g.Slug != tt.wantSlug {
				t.Errorf("Resolve(%q) slug = %q, want %q", tt.query, g.Slug, tt.wantSlug)
			}
		})
	}

	// Normalized-name collision: validation compares raw names only, so a
	// mixed-case alias can normalize to another guide's slug. The tie is
	// ambiguous and must resolve to nothing — not to iteration order.
	t.Run("normalized collision is ambiguous", func(t *testing.T) {
		t.Parallel()

		collideFS := fstest.MapFS{
			"auth-setup.md": {Data: guideFile("auth-setup", "Auth setup", "")},
			"other.md":      {Data: guideFile("other", "Other", "aliases: [Auth_Setup]")},
		}

		cgs, err := docent.LoadGuides(collideFS)
		if err != nil {
			t.Fatalf("LoadGuides: %v", err)
		}

		if g, ok := cgs.Resolve("AUTH_SETUP"); ok {
			t.Errorf("Resolve(AUTH_SETUP) = %q; want ambiguous no-match", g.Slug)
		}

		// The exact raw slug still resolves — ambiguity applies to the
		// normalized tier only.
		if _, ok := cgs.Resolve("auth-setup"); !ok {
			t.Error("Resolve(auth-setup) failed; exact slug must still win")
		}
	})
}

// TestLoadGuides_crlfNormalized pins the cross-platform promise that guide
// files arriving with CRLF line endings parse identically to LF ones: the
// sections parse and their bodies carry no carriage returns. (Guide.Raw is
// documented as the complete original bytes and deliberately stays verbatim.)
func TestLoadGuides_crlfNormalized(t *testing.T) {
	t.Parallel()

	crlfFS := fstest.MapFS{
		"windows.md": {Data: []byte(strings.Join([]string{
			"---",
			"slug: windows",
			"title: Windows Guide",
			"description: Authored with CRLF endings.",
			"when_to_use: On Windows checkouts.",
			"commands: [windows run]",
			"---",
			"",
			"## Decide",
			"Pick the variant.",
			"",
			"## Run",
			"windows run",
			"",
			"## Save",
			"",
			"## Preconditions",
			"",
			"## Recover",
			"",
			"## Next",
		}, "\r\n"))},
	}

	gs, err := docent.LoadGuides(crlfFS)
	if err != nil {
		t.Fatalf("LoadGuides on CRLF input: %v", err)
	}

	g, ok := gs.Get("windows")
	if !ok {
		t.Fatal("windows guide missing after CRLF load")
	}

	if len(g.Sections) != 6 {
		t.Fatalf("CRLF guide parsed %d sections, want 6", len(g.Sections))
	}

	for _, s := range g.Sections {
		if strings.Contains(s.Body, "\r") {
			t.Errorf("section %q body retains carriage returns: %q", s.Heading, s.Body)
		}
	}
}

// formatGuideSet returns a stable text representation of a GuideSet for golden
// comparison. It captures canonical order, slugs, titles, orders, and section
// headings — the fields that LoadGuides is responsible for.
func formatGuideSet(gs *docent.GuideSet) string {
	var b strings.Builder

	guides := gs.Guides()
	fmt.Fprintf(&b, "n=%d\n", len(guides))

	for i, g := range guides {
		order := "-"
		if g.Order != nil {
			order = fmt.Sprintf("%d", *g.Order)
		}

		headings := make([]string, len(g.Sections))
		for j, s := range g.Sections {
			headings[j] = s.Heading
		}

		fmt.Fprintf(&b, "[%d] slug=%s title=%q order=%s sections=[%s]\n",
			i, g.Slug, g.Title, order, strings.Join(headings, ", "))
	}

	return b.String()
}

// checkGolden compares got against the golden file for name.
// When -update is set it writes got to the golden file instead.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", "golden", name+".txt")

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(goldenPath), err)
		}

		if err := os.WriteFile(goldenPath, []byte(got), 0o600); err != nil {
			t.Fatalf("write golden %s: %v", goldenPath, err)
		}

		return
	}

	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to generate)", goldenPath, err)
	}

	want := string(wantBytes)
	if got != want {
		t.Errorf("golden mismatch for %q:\n--- want\n%s\n+++ got\n%s", name, want, got)
	}
}
