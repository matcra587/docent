package cobra_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	gocobra "github.com/spf13/cobra"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
)

// executeAgent mounts the agent command group on a fresh cobra host, sets
// args, captures stdout, and executes, returning the captured bytes and any
// error. opts are forwarded to NewCommand; pass nil when no options are under
// test. Errors and usage output are silenced so test output stays clean.
func executeAgent(cfg docent.Config, opts []docentcobra.Option, args ...string) ([]byte, error) {
	var buf bytes.Buffer

	hostRoot := &gocobra.Command{Use: "host", Short: "Test host."}
	hostRoot.SilenceErrors = true
	hostRoot.SilenceUsage = true
	hostRoot.SetOut(&buf)

	hostRoot.AddCommand(docentcobra.NewCommand(cfg, opts...))
	hostRoot.SetArgs(args)

	err := hostRoot.Execute()

	return buf.Bytes(), err
}

// checkGolden compares got against the golden file at goldenPath, rewriting
// the file instead when update is set. updateFlag names the -update-* flag in
// the regeneration hint. The per-surface check*Golden wrappers own their
// golden directory layout and update flag; this holds the shared mechanics.
func checkGolden(t *testing.T, goldenPath, got string, update bool, updateFlag string) {
	t.Helper()

	if update {
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
		t.Fatalf("read golden %s: %v (run with -%s to generate)", goldenPath, err, updateFlag)
	}

	if want := string(wantBytes); got != want {
		t.Errorf("golden mismatch for %s:\n--- want\n%s\n+++ got\n%s", goldenPath, want, got)
	}
}
