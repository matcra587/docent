package docenttest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matcra587/docent"
	"github.com/matcra587/docent/docenttest"
)

// schemaFixture is a small two-level tree with a volatile default, the shape
// a host pins in its own CI.
func schemaFixture() docent.Command {
	return docent.Command{
		Name: "tool",
		Path: "tool",
		Flags: []docent.Flag{
			{Name: "callback", Type: "string", Default: "http://localhost:41718/callback"},
			{Name: "output", Type: "string", Default: "auto", Persistent: true},
		},
		Children: []docent.Command{
			{
				Name: "run",
				Path: "tool run",
				Flags: []docent.Flag{
					{Name: "callback", Type: "string", Default: "http://localhost:52222/hook"},
				},
			},
		},
	}
}

// TestSchemaGolden_roundTrip pins the helper's promise: update writes the
// canonical emission (two-space JSON, trailing newline), and an unchanged
// tree matches it byte-for-byte.
func TestSchemaGolden_roundTrip(t *testing.T) {
	t.Parallel()

	golden := filepath.Join(t.TempDir(), "schema.json")

	docenttest.SchemaGolden(t, schemaFixture(), golden, true)

	data, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read written golden: %v", err)
	}

	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatal("golden must be non-empty and end with a trailing newline")
	}

	docenttest.SchemaGolden(t, schemaFixture(), golden, false)
}

// TestMaskFlagDefaults pins the volatile-default answer: named flags are
// masked wherever they appear in the tree, and the source tree is untouched.
func TestMaskFlagDefaults(t *testing.T) {
	t.Parallel()

	src := schemaFixture()
	masked := docenttest.MaskFlagDefaults(src, "callback")

	if masked.Flags[0].Default != "MASKED" {
		t.Errorf("root --callback not masked: %q", masked.Flags[0].Default)
	}

	if masked.Children[0].Flags[0].Default != "MASKED" {
		t.Errorf("child --callback not masked: %q", masked.Children[0].Flags[0].Default)
	}

	if masked.Flags[1].Default != "auto" {
		t.Errorf("unnamed flag must keep its default: %q", masked.Flags[1].Default)
	}

	if src.Flags[0].Default != "http://localhost:41718/callback" {
		t.Errorf("source tree mutated: %q", src.Flags[0].Default)
	}
}
