package cobra_test

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	gocobra "github.com/spf13/cobra"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
	"github.com/matcra587/docent/export"
	"github.com/matcra587/docent/harness"
)

// updateExportGolden regenerates golden files when passed to "go test -update-export".
var updateExportGolden = flag.Bool("update-export", false, "update export golden files")

// exportFS is a minimal two-guide filesystem mirroring the root-package validFS.
// bravo (order=1) comes before alpha (no order) in canonical output.
var exportFS = fstest.MapFS{
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
		"license: MIT",
		"compatibility: Requires network access.",
		"allowed_tools: [Bash(alpha:*), Read]",
		"metadata:",
		"  author: acme",
		"  version: \"1.0\"",
		"  \"vendor: x\": tricky",
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

// runExportToDir executes "agent export --format agent-skill --dir <tmp>" and
// returns the report output and the export directory.
func runExportToDir(t *testing.T, cfg docent.Config) (report, dir string) {
	t.Helper()

	dir = t.TempDir()

	out, err := executeAgent(cfg, nil, "agent", "export", "--format", "agent-skill", "--dir", dir)
	if err != nil {
		t.Fatalf("agent export: %v", err)
	}

	return string(out), dir
}

// readExported reads one written artifact from the export directory.
func readExported(t *testing.T, dir, rel string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read exported %s: %v", rel, err)
	}

	return string(data)
}

// TestAgentExport_writesOneFilePerGuide is the primary AC 4 test: one
// SKILL.md file per guide is written under --dir, and each written path is
// reported relative to --dir in canonical order.
func TestAgentExport_writesOneFilePerGuide(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	report, dir := runExportToDir(t, cfg)

	wantReport := "bravo/SKILL.md\nalpha/SKILL.md\n"
	if report != wantReport {
		t.Errorf("report mismatch:\nwant: %q\ngot:  %q", wantReport, report)
	}

	for _, rel := range []string{"bravo/SKILL.md", "alpha/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected artifact %s: %v", rel, err)
		}
	}
}

// TestAgentExport_skillGolden goldens each written SKILL.md byte-for-byte.
// A golden diff IS a behavior change.
func TestAgentExport_skillGolden(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	_, dir := runExportToDir(t, cfg)

	checkExportGolden(t, "skill_bravo", readExported(t, dir, "bravo/SKILL.md"))
	checkExportGolden(t, "skill_alpha", readExported(t, dir, "alpha/SKILL.md"))
}

// TestAgentExport_claudeSkillGolden goldens the claude-skill variant: the
// same layout as agent-skill, with description and when_to_use as separate
// frontmatter keys.
func TestAgentExport_claudeSkillGolden(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	dir := t.TempDir()

	if _, err := executeAgent(cfg, nil, "agent", "export", "--format", "claude-skill", "--dir", dir); err != nil {
		t.Fatalf("agent export: %v", err)
	}

	checkExportGolden(t, "skill_bravo_claude", readExported(t, dir, "bravo/SKILL.md"))
}

// TestAgentExport_skillNameQualifier verifies the public integration option
// reaches both built-in renderers, qualifying the direct-child directory and
// frontmatter name while leaving guide lookup and index output unchanged.
func TestAgentExport_skillNameQualifier(t *testing.T) {
	t.Parallel()

	gs := mustLoadGuides(t, exportFS)
	cfg := docent.Config{Guides: gs}
	opts := []docentcobra.Option{
		docentcobra.WithSkillNameQualifier("replaced"),
		docentcobra.WithSkillNameQualifier("jira"),
	}

	for _, format := range []string{"agent-skill", "claude-skill"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			out, err := executeAgent(
				cfg,
				opts,
				"agent",
				"export",
				"--format",
				format,
				"--dir",
				dir,
			)
			if err != nil {
				t.Fatalf("agent export --format %s: %v", format, err)
			}

			const rel = "jira-bravo/SKILL.md"
			if !strings.Contains(string(out), rel+"\n") {
				t.Errorf("report = %q, want %s", out, rel)
			}

			got := readExported(t, dir, rel)
			if !strings.HasPrefix(got, "---\nname: jira-bravo\n") {
				t.Errorf("qualified frontmatter name missing:\n%s", got)
			}

			if _, statErr := os.Stat(filepath.Join(dir, "jira", "jira-bravo", "SKILL.md")); !os.IsNotExist(statErr) {
				t.Errorf("unexpected grouping directory created: %v", statErr)
			}
		})
	}

	qualifiedIndex, err := executeAgent(cfg, opts, "agent", "guide")
	if err != nil {
		t.Fatalf("qualified agent guide: %v", err)
	}

	unqualifiedIndex, err := executeAgent(docent.Config{Guides: gs}, nil, "agent", "guide")
	if err != nil {
		t.Fatalf("unqualified agent guide: %v", err)
	}

	if !bytes.Equal(qualifiedIndex, unqualifiedIndex) {
		t.Errorf("WithSkillNameQualifier changed guide index:\nqualified:\n%s\nunqualified:\n%s", qualifiedIndex, unqualifiedIndex)
	}

	if g, ok := gs.Get("bravo"); !ok || g.Slug != "bravo" {
		t.Errorf("source GuideSet changed: guide = %+v, found = %v", g, ok)
	}

	t.Run("empty option preserves unqualified bytes", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		emptyOpts := []docentcobra.Option{docentcobra.WithSkillNameQualifier("")}

		out, err := executeAgent(
			cfg,
			emptyOpts,
			"agent",
			"export",
			"--format",
			"agent-skill",
			"--dir",
			dir,
		)
		if err != nil {
			t.Fatalf("agent export with empty qualifier: %v", err)
		}

		if want := "bravo/SKILL.md\nalpha/SKILL.md\n"; string(out) != want {
			t.Errorf("report = %q, want historical bytes %q", out, want)
		}

		checkExportGolden(t, "skill_bravo", readExported(t, dir, "bravo/SKILL.md"))
	})
}

// TestAgentExport_invalidQualifiedName verifies a bad host qualifier fails
// before the export directory is created.
func TestAgentExport_invalidQualifiedName(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "not-created")
	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}
	opts := []docentcobra.Option{docentcobra.WithSkillNameQualifier("Jira")}

	_, err := executeAgent(cfg, opts, "agent", "export", "--format", "agent-skill", "--dir", dir)
	if !errors.Is(err, export.ErrInvalidSkillName) {
		t.Fatalf("error = %v, want errors.Is(err, ErrInvalidSkillName)", err)
	}

	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Errorf("export directory exists after validation failure: %v", statErr)
	}
}

// TestAgentExport_skillFrontmatter verifies each written artifact begins with
// YAML frontmatter carrying name and description — the fields agent harnesses
// use to discover and selectively load skills.
func TestAgentExport_skillFrontmatter(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	_, dir := runExportToDir(t, cfg)

	got := readExported(t, dir, "bravo/SKILL.md")

	if !strings.HasPrefix(got, "---\n") {
		t.Fatalf("artifact does not start with YAML frontmatter:\n%s", got)
	}

	for _, want := range []string{"name: bravo", "description:"} {
		if !strings.Contains(got, want) {
			t.Errorf("artifact frontmatter missing %q:\n%s", want, got)
		}
	}
}

// TestAgentExport_generatedHeader verifies the generated-do-not-edit header
// appears in every written artifact.
func TestAgentExport_generatedHeader(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	_, dir := runExportToDir(t, cfg)

	const header = "<!-- Code generated by docent; DO NOT EDIT. -->"

	for _, rel := range []string{"bravo/SKILL.md", "alpha/SKILL.md"} {
		if !strings.Contains(readExported(t, dir, rel), header) {
			t.Errorf("artifact %s missing generated header", rel)
		}
	}
}

// TestAgentExport_flagsRequired verifies that omitting --format or --dir is
// an error rather than a silent no-op.
func TestAgentExport_flagsRequired(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	if _, err := executeAgent(cfg, nil, "agent", "export", "--dir", t.TempDir()); err == nil {
		t.Error("expected error when --format is omitted, got nil")
	}

	if _, err := executeAgent(cfg, nil, "agent", "export", "--format", "agent-skill"); err == nil {
		t.Error("expected error when --dir is omitted, got nil")
	}
}

// TestAgentExport_dirEmptyString verifies that an empty --dir value —
// explicitly set, which cobra's MarkFlagsOneRequired counts as provided — is
// caught by resolveExportDestination's own check and reports the real
// problem instead of silently treating an empty directory as unset.
func TestAgentExport_dirEmptyString(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	_, err := executeAgent(cfg, nil, "agent", "export", "--dir", "")
	if err == nil {
		t.Fatal("expected error for --dir '', got nil")
	}

	const want = "docent: one of --dir or --scope must be non-empty"
	if err.Error() != want {
		t.Errorf("error = %q; want %q", err.Error(), want)
	}
}

// TestAgentExport_scopeEmptyString verifies that an empty --scope value —
// explicitly set, satisfying cobra's MarkFlagsOneRequired — is caught by
// resolveExportDestination's own check and reports the real problem.
func TestAgentExport_scopeEmptyString(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	_, err := executeAgent(cfg, nil, "agent", "export", "--scope", "")
	if err == nil {
		t.Fatal("expected error for --scope '', got nil")
	}

	const want = "docent: one of --dir or --scope must be non-empty"
	if err.Error() != want {
		t.Errorf("error = %q; want %q", err.Error(), want)
	}
}

// TestAgentExport_scopeInvalid verifies that an unrecognized --scope value
// reports the exact unsupported-scope error naming the supported scopes.
func TestAgentExport_scopeInvalid(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	_, err := executeAgent(cfg, nil, "agent", "export", "--scope", "bogus")
	if err == nil {
		t.Fatal("expected error for unsupported --scope value, got nil")
	}

	const want = `docent: unsupported scope "bogus"; supported scopes: project, user`
	if err.Error() != want {
		t.Errorf("error = %q; want %q", err.Error(), want)
	}
}

// TestAgentExport_unknownFormat verifies that an unsupported --format value
// returns an error.
func TestAgentExport_unknownFormat(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	_, err := executeAgent(cfg, nil, "agent", "export", "--format", "unknown", "--dir", t.TempDir())
	if err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
}

// stubRenderer is a minimal host-supplied export format for WithExtraFormat
// tests. Its output and layout are deliberately unlike the SKILL.md formats
// so the assertions can tell which renderer produced an artifact.
type stubRenderer struct{}

func (stubRenderer) Render(g docent.Guide) string { return "stub: " + g.Slug + "\n" }

func (stubRenderer) RelPath(g docent.Guide) string { return g.Slug + ".txt" }

// TestAgentExport_withExtraFormat pins the WithExtraFormat contract: a
// registered format is selectable via --format and writes through the host
// renderer; its name appears in the unknown-format error's supported list; a
// built-in name cannot be shadowed; and empty-name or nil-renderer
// registrations are ignored rather than mounted.
func TestAgentExport_withExtraFormat(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	t.Run("renders through host renderer", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		opts := []docentcobra.Option{docentcobra.WithExtraFormat("stub", stubRenderer{})}

		out, err := executeAgent(cfg, opts, "agent", "export", "--format", "stub", "--dir", dir)
		if err != nil {
			t.Fatalf("agent export --format stub: %v", err)
		}

		if !strings.Contains(string(out), "alpha.txt") {
			t.Errorf("report = %q; want alpha.txt listed", out)
		}

		if got := readExported(t, dir, "alpha.txt"); got != "stub: alpha\n" {
			t.Errorf("artifact = %q; want %q", got, "stub: alpha\n")
		}
	})

	t.Run("listed in unknown-format error", func(t *testing.T) {
		t.Parallel()

		opts := []docentcobra.Option{docentcobra.WithExtraFormat("stub", stubRenderer{})}

		_, err := executeAgent(cfg, opts, "agent", "export", "--format", "nope", "--dir", t.TempDir())
		if err == nil {
			t.Fatal("expected error for unknown format, got nil")
		}

		if !strings.Contains(err.Error(), "agent-skill, claude-skill, stub") {
			t.Errorf("error = %v; want supported list ending in stub", err)
		}
	})

	t.Run("builtin name not shadowed", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		opts := []docentcobra.Option{docentcobra.WithExtraFormat("agent-skill", stubRenderer{})}

		if _, err := executeAgent(cfg, opts, "agent", "export", "--format", "agent-skill", "--dir", dir); err != nil {
			t.Fatalf("agent export: %v", err)
		}

		// The built-in renderer's layout, not the stub's.
		if got := readExported(t, dir, "alpha/SKILL.md"); !strings.Contains(got, "name: alpha") {
			t.Errorf("artifact = %q; want built-in SKILL.md output", got)
		}
	})

	t.Run("empty name and nil renderer ignored", func(t *testing.T) {
		t.Parallel()

		opts := []docentcobra.Option{
			docentcobra.WithExtraFormat("", stubRenderer{}),
			docentcobra.WithExtraFormat("stub", nil),
		}

		_, err := executeAgent(cfg, opts, "agent", "export", "--format", "stub", "--dir", t.TempDir())
		if err == nil {
			t.Fatal("expected unknown-format error for ignored registrations, got nil")
		}
	})

	t.Run("host qualifier does not rewrite extra format", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		opts := []docentcobra.Option{
			docentcobra.WithExtraFormat("stub", stubRenderer{}),
			docentcobra.WithSkillNameQualifier("jira"),
		}

		if _, err := executeAgent(cfg, opts, "agent", "export", "--format", "stub", "--dir", dir); err != nil {
			t.Fatalf("agent export --format stub: %v", err)
		}

		if got := readExported(t, dir, "alpha.txt"); got != "stub: alpha\n" {
			t.Errorf("artifact = %q; qualifier should not rewrite an extra format", got)
		}
	})
}

// Compile-time assertion: the stub must satisfy the exported Renderer
// interface, pinning that WithExtraFormat accepts consumer implementations.
var _ export.Renderer = stubRenderer{}

// TestAgentExport_configOut verifies the path report goes to Config.Out (not
// cobra stdout) while artifacts land on disk.
func TestAgentExport_configOut(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var hostOut bytes.Buffer

	cfg := docent.Config{
		Guides: mustLoadGuides(t, exportFS),
		Out:    &hostOut,
	}

	var cobraOut bytes.Buffer

	hostRoot := &gocobra.Command{Use: "host", Short: "Test host."}
	hostRoot.SilenceErrors = true
	hostRoot.SilenceUsage = true
	hostRoot.SetOut(&cobraOut)
	hostRoot.AddCommand(docentcobra.NewCommand(cfg))
	hostRoot.SetArgs([]string{"agent", "export", "--format", "agent-skill", "--dir", dir})

	if err := hostRoot.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if cobraOut.Len() != 0 {
		t.Errorf("cobra output writer was written to (%d bytes); Config.Out should be used instead",
			cobraOut.Len())
	}

	if !strings.Contains(hostOut.String(), "bravo/SKILL.md") {
		t.Errorf("Config.Out did not receive the path report: %q", hostOut.String())
	}

	if _, err := os.Stat(filepath.Join(dir, "bravo", "SKILL.md")); err != nil {
		t.Errorf("artifact missing despite Config.Out: %v", err)
	}
}

// TestAgentExport_determinism verifies that two exports of the same guide set
// produce byte-identical artifacts.
func TestAgentExport_determinism(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	_, dir1 := runExportToDir(t, cfg)
	_, dir2 := runExportToDir(t, cfg)

	for _, rel := range []string{"bravo/SKILL.md", "alpha/SKILL.md"} {
		first := readExported(t, dir1, rel)

		second := readExported(t, dir2, rel)
		if first != second {
			t.Errorf("export of %s is not deterministic:\nfirst:\n%s\nsecond:\n%s", rel, first, second)
		}
	}
}

// TestAgentExport_noGuides verifies that the command returns an error when no
// guides are configured rather than silently producing empty output.
func TestAgentExport_noGuides(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{} // Guides is nil

	_, err := executeAgent(cfg, nil, "agent", "export", "--format", "agent-skill", "--dir", t.TempDir())
	if err == nil {
		t.Fatal("expected error when no guides are configured, got nil")
	}
}

// TestAgentExport_symlinkEscapeRejected verifies the export write boundary
// against a planted symlink: when <dir>/<slug> is a symlink pointing outside
// --dir, the write must be refused, not followed — the lexical path looks
// local, so only the os.Root-based writer catches it.
func TestAgentExport_symlinkEscapeRejected(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	dir := t.TempDir()
	outside := t.TempDir()

	// bravo is the first guide in canonical order; plant its artifact
	// directory as a symlink escaping --dir.
	if err := os.Symlink(outside, filepath.Join(dir, "bravo")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	_, err := executeAgent(cfg, nil, "agent", "export", "--format", "agent-skill", "--dir", dir)
	if err == nil {
		t.Fatal("expected error for symlinked artifact directory, got nil")
	}

	if _, statErr := os.Stat(filepath.Join(outside, "SKILL.md")); statErr == nil {
		t.Fatal("artifact was written through the symlink, outside --dir")
	}
}

// TestAgentExport_currentDirTarget pins that "--dir ." exports into the
// working directory: the natural "export here" invocation must not be
// rejected by the path boundary.
func TestAgentExport_currentDirTarget(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	out, err := executeAgent(cfg, nil, "agent", "export", "--format", "agent-skill", "--dir", ".")
	if err != nil {
		t.Fatalf("agent export --dir .: %v", err)
	}

	if !strings.Contains(string(out), "bravo/SKILL.md") {
		t.Errorf("report missing bravo/SKILL.md:\n%s", out)
	}

	if _, statErr := os.Stat(filepath.Join(work, "bravo", "SKILL.md")); statErr != nil {
		t.Errorf("bravo/SKILL.md not written under the working directory: %v", statErr)
	}
}

// setHarnessEnv pins every detection input to a known state: blank (treated
// as unset) except the ones in set. The AI_AGENT/AGENT overrides are pinned
// too — the test process itself runs under an agent that sets them. Tests
// using it cannot be parallel.
func setHarnessEnv(t *testing.T, set map[string]string) {
	t.Helper()

	// Every detection variable is pinned, not just the harness markers: any
	// runtime's leaked marker would otherwise satisfy detection and break
	// the undetected-path tests. harness.EnvVars is the authoritative list,
	// so a new marker cannot silently escape the pinning.
	for _, name := range harness.EnvVars() {
		t.Setenv(name, set[name])
	}
}

// TestAgentExport_scopeProject verifies that under a detected harness,
// "--scope project" resolves the harness's working-directory skills dir and
// its native format without --dir or --format, and that the report names the
// detected harness and resolved directory.
func TestAgentExport_scopeProject(t *testing.T) {
	setHarnessEnv(t, map[string]string{"CLAUDECODE": "1"})
	t.Chdir(t.TempDir())

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	out, err := executeAgent(cfg, nil, "agent", "export", "--scope", "project")
	if err != nil {
		t.Fatalf("agent export --scope project: %v", err)
	}

	report := string(out)
	if !strings.Contains(report, "claude-code") || !strings.Contains(report, filepath.Join(".claude", "skills")) {
		t.Errorf("report does not name the detected harness and resolved dir: %q", report)
	}

	got, readErr := os.ReadFile(filepath.Join(".claude", "skills", "bravo", "SKILL.md"))
	if readErr != nil {
		t.Fatalf("artifact not written to the project skills dir: %v", readErr)
	}

	// claude-skill is the claude-code default: when_to_use stays a native key.
	if !strings.Contains(string(got), "\nwhen_to_use: ") {
		t.Errorf("default format for claude-code is not claude-skill:\n%s", got)
	}
}

// TestAgentExport_scopeUser verifies that "--scope user" resolves the
// harness's skills directory against the home directory.
func TestAgentExport_scopeUser(t *testing.T) {
	setHarnessEnv(t, map[string]string{"CODEX_THREAD_ID": "t-1"})

	home := t.TempDir()
	t.Setenv("HOME", home)        // unix os.UserHomeDir
	t.Setenv("USERPROFILE", home) // windows os.UserHomeDir

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	if _, err := executeAgent(cfg, nil, "agent", "export", "--scope", "user"); err != nil {
		t.Fatalf("agent export --scope user: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "bravo", "SKILL.md")); err != nil {
		t.Errorf("artifact not written to the user skills dir: %v", err)
	}
}

// TestAgentExport_scopeFormatOverride verifies an explicit --format wins over
// the detected harness's default.
func TestAgentExport_scopeFormatOverride(t *testing.T) {
	setHarnessEnv(t, map[string]string{"CLAUDECODE": "1"})
	t.Chdir(t.TempDir())

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	if _, err := executeAgent(cfg, nil, "agent", "export", "--scope", "project", "--format", "agent-skill"); err != nil {
		t.Fatalf("agent export: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(".claude", "skills", "bravo", "SKILL.md"))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}

	if strings.Contains(string(got), "\nwhen_to_use: ") {
		t.Errorf("--format agent-skill did not override the claude-skill default:\n%s", got)
	}
}

// TestAgentExport_scopeUndetected verifies that --scope with no detectable
// harness fails with an actionable error naming the supported harnesses and
// the --dir fallback, and writes nothing.
func TestAgentExport_scopeUndetected(t *testing.T) {
	setHarnessEnv(t, map[string]string{})
	t.Chdir(t.TempDir())

	cfg := docent.Config{Guides: mustLoadGuides(t, exportFS)}

	_, err := executeAgent(cfg, nil, "agent", "export", "--scope", "project")
	if err == nil {
		t.Fatal("expected error when no harness is detected, got nil")
	}

	for _, want := range []string{"claude-code", "codex", "--dir"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}

	if _, statErr := os.Stat(".claude"); statErr == nil {
		t.Error("artifacts were written despite the detection failure")
	}
}

// checkExportGolden compares got against the golden file under
// cobra/testdata/golden/<name>.txt. Run with -update-export to regenerate.
func checkExportGolden(t *testing.T, name, got string) {
	t.Helper()

	checkGolden(t, filepath.Join("testdata", "golden", name+".txt"), got, *updateExportGolden, "update-export")
}
