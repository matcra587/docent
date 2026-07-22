package docenttest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matcra587/docent"
)

// SchemaGolden asserts that cmd's canonical schema JSON is byte-identical to
// the golden file at goldenPath, writing the file instead when update is
// true. The emission goes through [docent.MarshalSchema] — the same function
// the adapter's "agent schema" command writes through — so the golden pins
// the artifact agents actually read, not a lookalike. Adapter effects sit
// on top of the canonical shape and are not covered here: the
// contract-version stamp (Config.ContractVersion), the full-tree
// structure-only emission (docent.Command.StripShapes replaces embedded
// schema bodies with has_* markers unless --shapes or --path is used),
// shape pooling in embedding emissions (docent.Command.PoolShapes hoists
// repeated bodies into $defs), and host schema transforms. Hosts relying
// on any of these effects should golden the schema command's output
// itself — or golden the policy-shaped tree by passing
// cmd.StripShapes() / cmd.PoolShapes() here.
//
// Hosts whose trees carry volatile flag defaults (ephemeral ports, home-dir
// paths, timestamps) golden a masked copy via MaskFlagDefaults first;
// pinning volatile bytes makes the golden flake.
func SchemaGolden(tb testing.TB, cmd docent.Command, goldenPath string, update bool) {
	tb.Helper()

	data, err := docent.MarshalSchema(cmd)
	if err != nil {
		tb.Fatalf("marshal schema: %v", err)
	}

	got := string(data) + "\n"

	// The golden path is caller-supplied test fixture location by design;
	// Clean scopes it the way the gosec guidance asks.
	goldenPath = filepath.Clean(goldenPath)

	if update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o750); err != nil {
			tb.Fatalf("mkdir %s: %v", filepath.Dir(goldenPath), err)
		}

		if err := os.WriteFile(goldenPath, []byte(got), 0o600); err != nil {
			tb.Fatalf("write golden %s: %v", goldenPath, err)
		}

		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		tb.Fatalf("read golden %s: %v (run with the update flag to generate)", goldenPath, err)
	}

	if got != string(want) {
		tb.Errorf("schema does not match golden %s:\n--- want\n%s\n+++ got\n%s", goldenPath, want, got)
	}
}

// MaskFlagDefaults returns a deep copy of cmd in which every flag named in
// names — wherever it appears in the tree — has its Default replaced with
// "MASKED". Use it before SchemaGolden when a default is computed at startup
// (an ephemeral port, a home-directory path) and would flake the golden; the
// masked copy shares no storage with cmd.
func MaskFlagDefaults(cmd docent.Command, names ...string) docent.Command {
	masked := cmd.Clone()

	volatile := make(map[string]struct{}, len(names))
	for _, name := range names {
		volatile[name] = struct{}{}
	}

	maskCommand(&masked, volatile)

	return masked
}

// maskCommand rewrites volatile flag defaults in place, recursing through
// children. The caller owns the copy being rewritten.
func maskCommand(cmd *docent.Command, volatile map[string]struct{}) {
	for i := range cmd.Flags {
		if _, ok := volatile[cmd.Flags[i].Name]; ok {
			cmd.Flags[i].Default = "MASKED"
		}
	}

	for i := range cmd.Children {
		maskCommand(&cmd.Children[i], volatile)
	}
}
