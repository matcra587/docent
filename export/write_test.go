package export_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/matcra587/docent"
	"github.com/matcra587/docent/export"
)

// pathRenderer maps each guide slug to a fixed relative path, defaulting to
// the slug-derived layout. Through the SKILL.md renderers every RelPath
// derives from a load-validated slug, so Write's validation loop is
// unreachable with them — it guards future renderers. Driving Write with
// hostile paths keeps the guards from rotting into green-by-skip.
type pathRenderer struct {
	paths map[string]string
}

func (pathRenderer) Render(g docent.Guide) string { return "content for " + g.Slug + "\n" }

func (r pathRenderer) RelPath(g docent.Guide) string {
	if p, ok := r.paths[g.Slug]; ok {
		return p
	}

	return g.Slug + "/SKILL.md"
}

// loadTwoGuides returns a set of two guides in canonical order (bravo then
// alpha, via order: 1).
func loadTwoGuides(t *testing.T) *docent.GuideSet {
	t.Helper()

	guide := func(slug, extra string) []byte {
		lines := []string{
			"---",
			"slug: " + slug,
			"title: " + slug + " guide",
			"description: The " + slug + " runbook.",
			"when_to_use: When you need " + slug + ".",
			"commands: [" + slug + " run]",
		}

		if extra != "" {
			lines = append(lines, extra)
		}

		lines = append(lines, "---", "",
			"## Decide", "", "## Run", "", "## Save", "",
			"## Preconditions", "", "## Recover", "", "## Next")

		return []byte(strings.Join(lines, "\n"))
	}

	gs, err := docent.LoadGuides(fstest.MapFS{
		"alpha.md": {Data: guide("alpha", "")},
		"bravo.md": {Data: guide("bravo", "order: 1")},
	})
	if err != nil {
		t.Fatalf("LoadGuides: %v", err)
	}

	return gs
}

// TestWrite_escapeRejectedBeforeAnyWrite pins two promises at once: an
// escaping RelPath errors with ErrPathEscape, and validation completes
// before anything is written — the first guide's perfectly local artifact
// must not exist even though the escaping path belongs to the second.
func TestWrite_escapeRejectedBeforeAnyWrite(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"../escape/SKILL.md", "/abs/SKILL.md"} {
		gs := loadTwoGuides(t)
		dir := filepath.Join(t.TempDir(), "skills")

		// bravo is first in canonical order; alpha (second) escapes.
		r := pathRenderer{paths: map[string]string{"alpha": bad}}

		rels, err := export.Write(dir, r, gs.Guides())
		if !errors.Is(err, export.ErrPathEscape) {
			t.Fatalf("RelPath %q: error = %v, want errors.Is(err, ErrPathEscape)", bad, err)
		}

		if len(rels) != 0 {
			t.Errorf("RelPath %q: Write reported %v written, want none", bad, rels)
		}

		if _, statErr := os.Stat(filepath.Join(dir, "bravo", "SKILL.md")); statErr == nil {
			t.Errorf("RelPath %q: first guide's artifact was written before validation finished", bad)
		}

		if _, statErr := os.Stat(filepath.Join(filepath.Dir(dir), "escape", "SKILL.md")); statErr == nil {
			t.Errorf("RelPath %q: artifact was written outside the export directory", bad)
		}
	}
}

// TestWrite_duplicateRelPathRejected pins that two guides mapping to one
// artifact path fail loudly instead of silently last-write-winning.
func TestWrite_duplicateRelPathRejected(t *testing.T) {
	t.Parallel()

	gs := loadTwoGuides(t)
	dir := t.TempDir()

	r := pathRenderer{paths: map[string]string{
		"alpha": "same/SKILL.md",
		"bravo": "same/SKILL.md",
	}}

	rels, err := export.Write(dir, r, gs.Guides())
	if err == nil || !strings.Contains(err.Error(), "duplicate export path") {
		t.Fatalf("error = %v, want duplicate export path error", err)
	}

	if len(rels) != 0 {
		t.Errorf("Write reported %v written, want none", rels)
	}
}
