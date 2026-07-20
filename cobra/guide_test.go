package cobra_test

import (
	"bytes"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
	gocobra "github.com/spf13/cobra"
)

// updateGuide regenerates guide golden files when passed to "go test -update-guide".
var updateGuide = flag.Bool("update-guide", false, "update agent guide golden files")

// guideValidFS is a well-formed two-guide filesystem used for guide command tests.
// bravo (order=1) sorts before alpha (no order) in canonical order.
var guideValidFS = fstest.MapFS{
	"bravo.md": {Data: []byte(strings.Join([]string{
		"---",
		"slug: bravo",
		"title: Bravo Guide",
		"description: The bravo runbook.",
		"when_to_use: When you need bravo.",
		"commands: [bravo run, bravo list]",
		"aliases: [bravo_guide]",
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

// mustLoadGuides calls LoadGuides and fatals if it returns an error. It is used
// in test setup to build a GuideSet from a well-formed FS.
func mustLoadGuides(t *testing.T, fsys fstest.MapFS) *docent.GuideSet {
	t.Helper()

	gs, err := docent.LoadGuides(fsys)
	if err != nil {
		t.Fatalf("LoadGuides: %v", err)
	}

	return gs
}

// checkGuideGolden compares got against the golden file under
// cobra/testdata/golden/<name>.txt. Run with -update-guide to regenerate.
func checkGuideGolden(t *testing.T, name, got string) {
	t.Helper()

	checkGolden(t, filepath.Join("testdata", "golden", name+".txt"), got, *updateGuide, "update-guide")
}

// TestAgentGuide_list verifies that "agent guide" with no args emits a
// frontmatter-only list index in canonical order and goldens the output.
func TestAgentGuide_list(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	out, err := executeAgent(cfg, nil, "agent", "guide")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	checkGuideGolden(t, "guide_list", string(out))
}

// TestAgentGuide_single verifies that "agent guide <slug>" emits the full guide
// content and goldens the output.
func TestAgentGuide_single(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	out, err := executeAgent(cfg, nil, "agent", "guide", "bravo")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	checkGuideGolden(t, "guide_single_bravo", string(out))
}

// TestAgentGuide_section verifies that "agent guide <slug> --section <heading>"
// emits only that section's body and goldens the output.
func TestAgentGuide_section(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	out, err := executeAgent(cfg, nil, "agent", "guide", "bravo", "--section", "Run")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	checkGuideGolden(t, "guide_section_bravo_run", string(out))
}

// TestAgentGuide_all verifies that "agent guide --all" emits all guides
// concatenated in canonical order and goldens the output.
func TestAgentGuide_all(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	out, err := executeAgent(cfg, nil, "agent", "guide", "--all")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	checkGuideGolden(t, "guide_all", string(out))
}

// TestAgentGuide_aliasResolves pins the rename contract: a guide's declared
// alias serves byte-identical output to its canonical slug, so agents that
// memorized a pre-rename name keep working.
func TestAgentGuide_aliasResolves(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	canonical, err := executeAgent(cfg, nil, "agent", "guide", "bravo")
	if err != nil {
		t.Fatalf("canonical lookup: %v", err)
	}

	viaAlias, err := executeAgent(cfg, nil, "agent", "guide", "bravo_guide")
	if err != nil {
		t.Fatalf("alias lookup: %v", err)
	}

	if string(canonical) != string(viaAlias) {
		t.Errorf("alias output differs from canonical:\n--- canonical\n%s\n+++ alias\n%s", canonical, viaAlias)
	}
}

// TestAgentGuide_indexContractVersion pins the Config.ContractVersion stamp
// on the guide index: a contract_version line directly after the header when
// set, absent when empty.
func TestAgentGuide_indexContractVersion(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)

	out, err := executeAgent(docent.Config{Guides: gs, ContractVersion: "2.1.0"}, nil, "agent", "guide")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 || lines[1] != "contract_version: 2.1.0" {
		t.Errorf("index line 2 = %q, want %q", lines[1], "contract_version: 2.1.0")
	}

	bare, err := executeAgent(docent.Config{Guides: gs}, nil, "agent", "guide")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if strings.Contains(string(bare), "contract_version") {
		t.Error("contract_version present in index with empty Config.ContractVersion")
	}
}

// TestAgentGuide_nearMissResolves pins the wiring promise that the guide
// command resolves near-miss names through GuideSet.Resolve: a case/separator
// variant and an unambiguous fragment both serve the same bytes as the
// canonical slug.
func TestAgentGuide_nearMissResolves(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	canonical, err := executeAgent(cfg, nil, "agent", "guide", "bravo")
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}

	for _, name := range []string{"BRAVO", "Bravo", "brav"} {
		got, err := executeAgent(cfg, nil, "agent", "guide", name)
		if err != nil {
			t.Fatalf("near-miss %q: %v", name, err)
		}

		if string(got) != string(canonical) {
			t.Errorf("near-miss %q output differs from canonical", name)
		}
	}
}

// TestAgentGuide_missingSlug verifies that an unknown slug causes Execute to
// return an error wrapping ErrGuideNotFound.
func TestAgentGuide_missingSlug(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	_, err := executeAgent(cfg, nil, "agent", "guide", "no-such-guide")
	if err == nil {
		t.Fatal("Execute: expected error for unknown slug, got nil")
	}

	if !errors.Is(err, docentcobra.ErrGuideNotFound) {
		t.Errorf("error = %v; want errors.Is(err, docentcobra.ErrGuideNotFound) to be true", err)
	}
}

// TestAgentGuide_missingSection verifies that an unknown --section value causes
// Execute to return an error wrapping ErrSectionNotFound.
func TestAgentGuide_missingSection(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	_, err := executeAgent(cfg, nil, "agent", "guide", "bravo", "--section", "NoSuchSection")
	if err == nil {
		t.Fatal("Execute: expected error for unknown section, got nil")
	}

	if !errors.Is(err, docentcobra.ErrSectionNotFound) {
		t.Errorf("error = %v; want errors.Is(err, docentcobra.ErrSectionNotFound) to be true", err)
	}
}

// TestAgentGuide_sectionWithoutSlug verifies that "--section" without a slug
// argument reports the exact error naming that specific incompatibility.
func TestAgentGuide_sectionWithoutSlug(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	_, err := executeAgent(cfg, nil, "agent", "guide", "--section", "Run")
	if err == nil {
		t.Fatal("Execute: expected error for --section without a slug, got nil")
	}

	const want = "docent: --section requires a slug argument"
	if err.Error() != want {
		t.Errorf("error = %q; want %q", err.Error(), want)
	}
}

// TestAgentGuide_allWithSlug verifies that "--all" combined with a slug
// argument reports the exact error naming that specific incompatibility.
func TestAgentGuide_allWithSlug(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	_, err := executeAgent(cfg, nil, "agent", "guide", "bravo", "--all")
	if err == nil {
		t.Fatal("Execute: expected error for --all with a slug, got nil")
	}

	const want = "docent: --all cannot be combined with a slug argument"
	if err.Error() != want {
		t.Errorf("error = %q; want %q", err.Error(), want)
	}
}

// TestAgentGuide_sectionAndAllMutuallyExclusive verifies that setting both
// --section and --all together triggers cobra's own mutual-exclusion error
// (naming both flags) rather than either RunE switch case above — the two
// flags being set together is its own distinct incompatibility.
func TestAgentGuide_sectionAndAllMutuallyExclusive(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	_, err := executeAgent(cfg, nil, "agent", "guide", "--section", "Run", "--all")
	if err == nil {
		t.Fatal("Execute: expected error for --section and --all together, got nil")
	}

	for _, want := range []string{"section", "all"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestAgentGuide_nilGuideSet verifies that when Guides is nil "agent guide"
// emits the no-guides-configured message without error.
func TestAgentGuide_nilGuideSet(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{} // Guides is nil

	out, err := executeAgent(cfg, nil, "agent", "guide")
	if err != nil {
		t.Fatalf("Execute: unexpected error with nil GuideSet: %v", err)
	}

	if !strings.Contains(string(out), "no guides configured") {
		t.Errorf("output = %q; want to contain %q", string(out), "no guides configured")
	}
}

// TestNewGuideCommand_identicalToAgentGuide pins the two-doors contract: a
// host mounting the standalone guide command serves byte-identical output to
// the agent-namespace mount for every view, so the human and agent surfaces
// cannot drift apart.
func TestNewGuideCommand_identicalToAgentGuide(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)

	run := func(t *testing.T, mount func(cfg docent.Config) *gocobra.Command, args []string) string {
		t.Helper()

		var buf bytes.Buffer

		host := &gocobra.Command{Use: "host", Short: "Test host."}
		host.SilenceErrors = true
		host.SilenceUsage = true
		host.SetOut(&buf)
		host.AddCommand(mount(docent.Config{Guides: gs}))
		host.SetArgs(args)

		if err := host.Execute(); err != nil {
			t.Fatalf("Execute %v: %v", args, err)
		}

		return buf.String()
	}

	for _, tc := range []struct {
		name  string
		views []string
	}{
		{"index", nil},
		{"single", []string{"bravo"}},
		{"section", []string{"bravo", "--section", "run"}},
		{"all", []string{"--all"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			viaAgent := run(t, func(cfg docent.Config) *gocobra.Command {
				return docentcobra.NewCommand(cfg)
			}, append([]string{"agent", "guide"}, tc.views...))
			standalone := run(t, docentcobra.NewGuideCommand, append([]string{"guide"}, tc.views...))

			if viaAgent != standalone {
				t.Errorf("surfaces diverge for %s:\n--- agent guide\n%s\n--- standalone\n%s", tc.name, viaAgent, standalone)
			}
		})
	}
}

// TestAgentGuide_configOut verifies that output goes to Config.Out instead of
// cmd.OutOrStdout() when the host supplies a writer.
func TestAgentGuide_configOut(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)

	var hostOut bytes.Buffer

	cfg := docent.Config{
		Guides: gs,
		Out:    &hostOut,
	}

	var cobraOut bytes.Buffer

	hostRoot := &gocobra.Command{Use: "host", Short: "Test host."}
	hostRoot.SilenceErrors = true
	hostRoot.SilenceUsage = true
	hostRoot.SetOut(&cobraOut)
	hostRoot.AddCommand(docentcobra.NewCommand(cfg))
	hostRoot.SetArgs([]string{"agent", "guide"})

	if err := hostRoot.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if cobraOut.Len() != 0 {
		t.Errorf("cobra output writer was written to (%d bytes); expected Config.Out to be used instead", cobraOut.Len())
	}

	if hostOut.Len() == 0 {
		t.Fatal("Config.Out was never written to")
	}

	if !strings.Contains(hostOut.String(), "bravo") {
		t.Errorf("Config.Out output = %q; expected guide index content", hostOut.String())
	}
}

// TestAgentGuide_listDeterminism verifies that "agent guide" emits byte-identical
// output on successive calls — canonical order and index format are stable.
func TestAgentGuide_listDeterminism(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	first, err := executeAgent(cfg, nil, "agent", "guide")
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	second, err := executeAgent(cfg, nil, "agent", "guide")
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("agent guide list is not deterministic:\nfirst:  %s\nsecond: %s", first, second)
	}
}

// TestAgentGuide_allDeterminism verifies that "agent guide --all" emits
// byte-identical output on successive calls.
func TestAgentGuide_allDeterminism(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	first, err := executeAgent(cfg, nil, "agent", "guide", "--all")
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	second, err := executeAgent(cfg, nil, "agent", "guide", "--all")
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("agent guide --all is not deterministic:\nfirst:  %s\nsecond: %s", first, second)
	}
}

// TestAgentGuide_listCanonicalOrder verifies that the index lists bravo (order=1)
// before alpha (no order) — proving canonical order is respected.
func TestAgentGuide_listCanonicalOrder(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	out, err := executeAgent(cfg, nil, "agent", "guide")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := string(out)
	bravoPos := strings.Index(output, "slug: bravo")
	alphaPos := strings.Index(output, "slug: alpha")

	if bravoPos == -1 {
		t.Error("bravo not found in guide index output")
	}

	if alphaPos == -1 {
		t.Error("alpha not found in guide index output")
	}

	if bravoPos > alphaPos {
		t.Errorf("bravo (order=1) appears after alpha (no order) in index; want bravo first")
	}
}

// TestAgentGuide_allCanonicalOrder verifies that --all concatenates bravo before
// alpha in canonical order.
func TestAgentGuide_allCanonicalOrder(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, guideValidFS)
	cfg := docent.Config{Guides: gs}

	out, err := executeAgent(cfg, nil, "agent", "guide", "--all")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := string(out)
	bravoPos := strings.Index(output, "slug: bravo")
	alphaPos := strings.Index(output, "slug: alpha")

	if bravoPos == -1 {
		t.Error("bravo not found in --all output")
	}

	if alphaPos == -1 {
		t.Error("alpha not found in --all output")
	}

	if bravoPos > alphaPos {
		t.Errorf("bravo (order=1) appears after alpha (no order) in --all output; want bravo first")
	}
}
