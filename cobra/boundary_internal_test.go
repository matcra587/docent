package cobra

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	gocobra "github.com/spf13/cobra"

	"github.com/matcra587/docent"
)

// White-box by necessity: through the public command surface every RelPath
// derives from a load-validated slug, so runExport's lexical boundary check
// is unreachable from outside — it guards future renderers. This test drives
// runExport directly with a renderer whose RelPath escapes, so the guard can
// never rot into a green-by-skip again.

// escapeRenderer emits a lexically escaping artifact path.
type escapeRenderer struct{}

func (escapeRenderer) Render(docent.Guide) string  { return "escaped\n" }
func (escapeRenderer) RelPath(docent.Guide) string { return "../escape/SKILL.md" }

func TestRunExport_lexicalEscapeRejected(t *testing.T) {
	t.Parallel()

	gs, err := docent.LoadGuides(fstest.MapFS{
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
	})
	if err != nil {
		t.Fatalf("LoadGuides: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "skills")

	err = runExport(&gocobra.Command{}, docent.Config{}, gs, escapeRenderer{}, dir, "")
	if err == nil {
		t.Fatal("expected error for escaping RelPath, got nil")
	}

	if !strings.Contains(err.Error(), "escapes --dir") {
		t.Errorf("error %q does not name the boundary violation", err)
	}

	if _, statErr := os.Stat(filepath.Join(filepath.Dir(dir), "escape", "SKILL.md")); statErr == nil {
		t.Fatal("artifact was written outside --dir")
	}
}
