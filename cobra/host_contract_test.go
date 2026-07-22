package cobra_test

// host_contract_test.go verifies that the docent cobra adapter captures
// a real-world host CLI's full schema fidelity — flag counts, enum values, mutex/required-
// together memberships, host-registered input/output schemas, and host-supplied
// extra commands — without information loss. It uses a faithful synthetic
// replica of a real-world host CLI's command tree rather than importing a real-world host CLI directly,
// so the test compiles without a go.work workspace and runs in CI with only
// the modules already in go.mod.
//
// The replica preserves every schema pattern that a real-world host CLI uses:
//   - Persistent (global) flags on the root command
//   - docent.enum annotations declaring allowed values on string flags
//   - Individually required flags (cobra_annotation_bash_completion_one_required_flag)
//   - Mutually-exclusive flag groups (cobra_annotation_mutually_exclusive)
//   - Required-together flag groups (cobra_annotation_required_if_others_set)
//   - Hidden subcommands
//   - Host input/output schemas registered via docent.SchemaRegistry
//   - Host-supplied extra commands mounted via WithExtraCommands
//
// Golden files live under testdata/golden/ and are updated with -update.

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
	gocobra "github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// updateGolden regenerates golden files when passed to "go test -update".
// Reuse the package-level flag already declared in tree_test.go would cause a
// redeclaration; we use a distinct variable named updateHostGolden for this
// file. The -update flag set in load_test.go is in the root package and
// therefore doesn't conflict.
var updateHostGolden = flag.Bool("update-host", false, "update host contract golden files")

// setEnumAnnotation declares a flag's allowed values via the docent.enum
// annotation — the bridge hosts use when their flag value types do not
// implement an Enum() method.
func setEnumAnnotation(f *pflag.Flag, values []string) {
	if f.Annotations == nil {
		f.Annotations = map[string][]string{}
	}

	f.Annotations[docentcobra.AnnotationEnum] = values
}

// buildHostFaithfulTree returns a synthetic cobra command tree that reproduces
// every schema-relevant pattern from a real-world host CLI's real command tree. It is not an
// exhaustive replica — it exercises every pattern class that the adapter must
// handle, not every command a real-world host CLI defines.
//
// Tree structure (matching key parts of a real-world host CLI's real tree):
//
//	host   root (persistent --output enum, --color enum, --profile string, --debug bool)
//	├── issue
//	│   ├── list   (--board/--board-id mutex, --key required; --output hidden in local)
//	│   ├── create (--summary required; --dry-run bool; enum annotation on --type)
//	│   └── edit   (--summary/--type required-together with --issue-key)
//	└── auth
//	    └── login  (--email/--token required-together; --email required)
func buildHostFaithfulTree() *gocobra.Command {
	root := &gocobra.Command{
		Use:   "host",
		Short: "Run Host developer workflows from a terminal.",
	}

	// Persistent (global) flags — mirror root.go's PersistentFlags.
	pf := root.PersistentFlags()

	pf.StringP("output", "o", "auto", "Output mode for command results.")
	setEnumAnnotation(pf.Lookup("output"), []string{"auto", "human", "json", "compact"})

	pf.String("color", "auto", "Color mode.")
	setEnumAnnotation(pf.Lookup("color"), []string{"auto", "always", "never"})

	pf.StringP("profile", "P", "", "Config profile to use.")
	pf.Bool("debug", false, "Enable debug output.")
	pf.Bool("no-input", false, "Disable interactive prompts.")

	// ── issue ────────────────────────────────────────────────────────────────

	issue := &gocobra.Command{Use: "issue", Short: "Manage Host issues."}

	// issue list — board mutex + required --key
	issueList := &gocobra.Command{
		Use:   "list",
		Short: "List issues matching filters.",
	}
	issueList.Flags().String("board", "", "Board name to scope the query.")
	issueList.Flags().String("board-id", "", "Board ID to scope the query.")
	issueList.Flags().StringSlice("key", nil, "Issue keys to retrieve.")
	issueList.Flags().Int("parallelism", 4, "Number of parallel requests.")
	issueList.Flags().StringSlice("status", nil, "Filter by status.")
	// mirror a real-world host CLI: board and board-id are mutually exclusive
	issueList.MarkFlagsMutuallyExclusive("board", "board-id")

	// issue create — required flag, enum annotation, schema registry candidate
	issueCreate := &gocobra.Command{
		Use:   "create",
		Short: "Create a new issue.",
	}
	issueCreate.Flags().String("summary", "", "Issue summary.")
	if err := issueCreate.MarkFlagRequired("summary"); err != nil {
		panic(err)
	}

	issueCreate.Flags().String("project-key", "", "Project key.")
	if err := issueCreate.MarkFlagRequired("project-key"); err != nil {
		panic(err)
	}

	issueCreate.Flags().String("type", "Task", "Issue type.")
	setEnumAnnotation(issueCreate.Flags().Lookup("type"), []string{"Bug", "Epic", "Story", "Sub-task", "Task"})

	issueCreate.Flags().Bool("dry-run", false, "Preview without submitting.")
	issueCreate.Flags().String("description", "", "Issue description (Markdown).")
	issueCreate.Flags().String("priority", "", "Priority name.")
	setEnumAnnotation(issueCreate.Flags().Lookup("priority"), []string{"High", "Highest", "Low", "Lowest", "Medium"})

	// issue edit — required-together group
	issueEdit := &gocobra.Command{
		Use:   "edit",
		Short: "Edit an existing issue.",
	}
	issueEdit.Flags().String("issue-key", "", "Issue key to edit (e.g. PROJECT-123).")
	if err := issueEdit.MarkFlagRequired("issue-key"); err != nil {
		panic(err)
	}

	issueEdit.Flags().String("summary", "", "New summary.")
	issueEdit.Flags().String("type", "", "New issue type.")
	// summary and type must be set together (both or neither)
	issueEdit.MarkFlagsRequiredTogether("summary", "type")
	issueEdit.Flags().Bool("dry-run", false, "Preview without submitting.")

	issue.AddCommand(issueList, issueCreate, issueEdit)

	// ── auth ─────────────────────────────────────────────────────────────────

	auth := &gocobra.Command{Use: "auth", Short: "Manage authentication."}

	authLogin := &gocobra.Command{
		Use:   "login",
		Short: "Log in to a Host instance.",
	}
	authLogin.Flags().String("email", "", "User email address.")
	if err := authLogin.MarkFlagRequired("email"); err != nil {
		panic(err)
	}

	authLogin.Flags().String("token", "", "Atlassian API token.")
	authLogin.Flags().String("server", "", "Host server base URL.")
	if err := authLogin.MarkFlagRequired("server"); err != nil {
		panic(err)
	}
	// email and token must be set together when either is provided
	authLogin.MarkFlagsRequiredTogether("email", "token")

	// hidden subcommand — mirrors host's completion command
	authLogout := &gocobra.Command{
		Use:    "logout",
		Short:  "Remove saved credentials.",
		Hidden: true,
	}

	auth.AddCommand(authLogin, authLogout)

	root.AddCommand(issue, auth)

	return root
}

// buildHostSchemaRegistry returns a docent.SchemaRegistry that mirrors the
// host-registered input/output schemas that a real-world host CLI attaches to its command
// tree. Only the schema paths relevant to the fixture tree are included.
func buildHostSchemaRegistry() docent.SchemaRegistry {
	errorSchema := map[string]any{
		"type":     "object",
		"required": []any{"type", "code", "message"},
		"properties": map[string]any{
			"type":    map[string]any{"type": "string"},
			"code":    map[string]any{"type": "string"},
			"message": map[string]any{"type": "string"},
			"hint":    map[string]any{"type": "string"},
		},
	}

	adfDocument := map[string]any{
		"type":     "object",
		"required": []any{"type", "version", "content"},
		"properties": map[string]any{
			"type":    map[string]any{"type": "string", "enum": []any{"doc"}},
			"version": map[string]any{"type": "integer"},
			"content": map[string]any{"type": "array"},
		},
	}

	issueListOutput := map[string]any{
		"type":     "object",
		"required": []any{"issues"},
		"properties": map[string]any{
			"issues": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"key", "summary", "status"},
					"properties": map[string]any{
						"key":     map[string]any{"type": "string"},
						"summary": map[string]any{"type": "string"},
						"status":  map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	issueCreateInput := map[string]any{
		"type":        "object",
		"description": "issue create --json-input payload.",
		"properties": map[string]any{
			"summary":     map[string]any{"type": "string"},
			"project_key": map[string]any{"type": "string"},
			"issue_type":  map[string]any{"type": "string"},
			"description": adfDocument,
		},
	}

	issueCreateOutput := map[string]any{
		"type":     "object",
		"required": []any{"dry_run"},
		"properties": map[string]any{
			"issue":   map[string]any{"type": "object"},
			"dry_run": map[string]any{"type": "boolean"},
		},
	}

	issueEditInput := map[string]any{
		"type":       "object",
		"properties": map[string]any{"fields": map[string]any{"type": "object"}},
	}

	// Identical to issueCreateOutput by design: real hosts register one
	// result shell on many sibling commands, and the shape-embedding
	// emissions must pool the repeat into $defs rather than inline it twice.
	issueEditOutput := map[string]any{
		"type":     "object",
		"required": []any{"dry_run"},
		"properties": map[string]any{
			"issue":   map[string]any{"type": "object"},
			"dry_run": map[string]any{"type": "boolean"},
		},
	}

	_ = errorSchema // referenced by host schema; not directly in the registry tree

	return docent.SchemaRegistry{
		"host issue list":   {Output: issueListOutput},
		"host issue create": {Input: issueCreateInput, Output: issueCreateOutput},
		"host issue edit":   {Input: issueEditInput, Output: issueEditOutput},
	}
}

// TestHostContract_agentSchemaGolden is the primary golden test for Sub-AC 3d.
// It:
//  1. Builds a a real-world host CLI-faithful cobra tree.
//  2. Applies docent's Tree() adapter to produce the neutral Command IR.
//  3. Applies the host SchemaRegistry to attach input/output schemas.
//  4. Runs "agent schema" and captures the JSON output.
//  5. Goldens the bytes (update with -update-host).
//
// The golden file is the byte-stable specification of docent's schema output
// for a real-world host CLI's command patterns. A diff in review IS a behavior change.
func TestHostContract_agentSchemaGolden(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)
	reg := buildHostSchemaRegistry()
	enriched := reg.Apply(tree)

	cfg := docent.Config{Command: enriched}

	out, err := executeAgent(cfg, nil, "agent", "schema")
	if err != nil {
		t.Fatalf("agent schema: %v", err)
	}

	checkHostGolden(t, "host_agent_schema", string(out))
}

// TestHostContract_agentSchemaShapesGolden pins the --shapes emission: the
// full tree with every registered input/output schema body embedded — the
// pre-policy whole-tree shape, kept behind an explicit flag for offline
// capture.
func TestHostContract_agentSchemaShapesGolden(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	reg := buildHostSchemaRegistry()
	cfg := docent.Config{Command: reg.Apply(docentcobra.Tree(root))}

	out, err := executeAgent(cfg, nil, "agent", "schema", "--shapes")
	if err != nil {
		t.Fatalf("agent schema --shapes: %v", err)
	}

	checkHostGolden(t, "host_agent_schema_shapes", string(out))
}

// TestHostContract_agentSchemaPathGolden pins the --path emission: one
// subtree with its schema bodies embedded — the shapes-on-demand side of
// the emission policy.
func TestHostContract_agentSchemaPathGolden(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	reg := buildHostSchemaRegistry()
	cfg := docent.Config{Command: reg.Apply(docentcobra.Tree(root))}

	out, err := executeAgent(cfg, nil, "agent", "schema", "--path", "host issue create")
	if err != nil {
		t.Fatalf("agent schema --path: %v", err)
	}

	checkHostGolden(t, "host_agent_schema_path", string(out))
}

// shapePolicyConfig builds the enriched host config the shape-emission
// tests share.
func shapePolicyConfig() docent.Config {
	root := buildHostFaithfulTree()
	reg := buildHostSchemaRegistry()

	return docent.Config{Command: reg.Apply(docentcobra.Tree(root))}
}

// findEmittedCommand unmarshals a schema emission and returns the node at
// path, failing the test when the emission does not parse or lacks it.
func findEmittedCommand(t *testing.T, out []byte, path string) docent.Command {
	t.Helper()

	var got docent.Command
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}

	cmd, ok := docent.FindByPath(got, path)
	if !ok {
		t.Fatalf("command %q not found in emission", path)
	}

	return cmd
}

// TestHostContract_shapeMarkersFullTree pins the structure-by-default side
// of the emission policy: the full tree replaces every registered schema
// body with a has_* marker, and a marker appears only where a body was
// omitted.
func TestHostContract_shapeMarkersFullTree(t *testing.T) {
	t.Parallel()

	out, err := executeAgent(shapePolicyConfig(), nil, "agent", "schema")
	if err != nil {
		t.Fatalf("agent schema: %v", err)
	}

	create := findEmittedCommand(t, out, "host issue create")
	if create.InputSchema != nil || create.OutputSchema != nil {
		t.Error("full-tree emission embeds schema bodies; they must be stripped to markers")
	}

	if !create.HasInputSchema || !create.HasOutputSchema {
		t.Error("full-tree emission lacks has_* markers for a command with registered schemas")
	}

	// issue list registers an output schema only: exactly one marker.
	list := findEmittedCommand(t, out, "host issue list")
	if list.HasInputSchema || !list.HasOutputSchema {
		t.Errorf("host issue list markers = input:%v output:%v; want output only",
			list.HasInputSchema, list.HasOutputSchema)
	}
}

// TestHostContract_shapesFlagEmbedsBodies pins the --shapes escape hatch:
// the full tree with every schema body embedded and no markers.
func TestHostContract_shapesFlagEmbedsBodies(t *testing.T) {
	t.Parallel()

	out, err := executeAgent(shapePolicyConfig(), nil, "agent", "schema", "--shapes")
	if err != nil {
		t.Fatalf("agent schema --shapes: %v", err)
	}

	create := findEmittedCommand(t, out, "host issue create")
	if create.InputSchema == nil || create.OutputSchema == nil {
		t.Error("--shapes emission lacks embedded schema bodies")
	}

	if create.HasInputSchema || create.HasOutputSchema {
		t.Error("--shapes emission carries has_* markers alongside embedded bodies")
	}
}

// TestHostContract_shapesPoolDuplicates pins $defs pooling in the
// shape-embedding emission: the output shell registered on both issue
// create and issue edit is hoisted once into the root $defs map, both
// commands reference it, and the unique input schemas stay inline.
func TestHostContract_shapesPoolDuplicates(t *testing.T) {
	t.Parallel()

	out, err := executeAgent(shapePolicyConfig(), nil, "agent", "schema", "--shapes")
	if err != nil {
		t.Fatalf("agent schema --shapes: %v", err)
	}

	var got docent.Command
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}

	if len(got.Defs) != 1 {
		t.Fatalf("root $defs = %v, want exactly the one shared output shell", got.Defs)
	}

	if _, ok := got.Defs["d1"].(map[string]any); !ok {
		t.Fatalf(`root $defs lacks the "d1" schema object: %v`, got.Defs)
	}

	for _, path := range []string{"host issue create", "host issue edit"} {
		cmd := findEmittedCommand(t, out, path)
		if len(cmd.OutputSchema) != 1 || cmd.OutputSchema["$ref"] != "#/$defs/d1" {
			t.Errorf(`%s output schema = %v, want {"$ref": "#/$defs/d1"}`, path, cmd.OutputSchema)
		}

		if _, isRef := cmd.InputSchema["$ref"]; isRef {
			t.Errorf("%s input schema was pooled; a unique body must stay inline", path)
		}
	}
}

// TestHostContract_pathEmbedsBodies pins the shapes-on-demand side: a
// --path subtree always embeds its schema bodies, and combining --path
// with --shapes is an error rather than a silent no-op.
func TestHostContract_pathEmbedsBodies(t *testing.T) {
	t.Parallel()

	cfg := shapePolicyConfig()

	out, err := executeAgent(cfg, nil, "agent", "schema", "--path", "host issue create")
	if err != nil {
		t.Fatalf("agent schema --path: %v", err)
	}

	create := findEmittedCommand(t, out, "host issue create")
	if create.InputSchema == nil || create.OutputSchema == nil {
		t.Error("--path emission lacks embedded schema bodies; a subtree always embeds")
	}

	if _, err := executeAgent(cfg, nil, "agent", "schema", "--path", "host issue create", "--shapes"); err == nil {
		t.Fatal("agent schema --path --shapes: expected mutual-exclusion error, got nil")
	}
}

// TestHostContract_determinism verifies that running each schema emission
// form twice on the same tree produces byte-identical JSON. Map-order bugs
// and sort instability would surface here — the pooled --shapes form walks
// a stats map and must stay byte-stable regardless of iteration order.
func TestHostContract_determinism(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	reg := buildHostSchemaRegistry()

	for _, args := range [][]string{
		{"agent", "schema"},
		{"agent", "schema", "--shapes"},
		{"agent", "schema", "--path", "host issue"},
	} {
		first, err := executeAgent(docent.Config{Command: reg.Apply(docentcobra.Tree(root))}, nil, args...)
		if err != nil {
			t.Fatalf("%v first run: %v", args, err)
		}

		second, err := executeAgent(docent.Config{Command: reg.Apply(docentcobra.Tree(root))}, nil, args...)
		if err != nil {
			t.Fatalf("%v second run: %v", args, err)
		}

		if !bytes.Equal(first, second) {
			t.Errorf("%v is not deterministic:\nfirst:\n%s\nsecond:\n%s", args, first, second)
		}
	}
}

// TestHostContract_noFlagCountLoss asserts that docent's IR contains at least
// as many flags per command as a real-world host CLI's reference schema defines for that
// command. This catches adapter regressions that silently drop flags.
func TestHostContract_noFlagCountLoss(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)

	// Reference counts from the host-faithful fixture. Each command carries
	// only the flags it defines — inherited flags live once on the ancestor
	// that defines them (TestHostContract_persistentFlagsHoisted pins that
	// side), so "no loss" here means every locally defined flag is present.
	type want struct {
		path     string
		minFlags int
	}

	cases := []want{
		// root defines 5 persistent flags: output, color, profile, debug, no-input
		{"host", 5},
		// issue list: 5 local (board, board-id, key, parallelism, status)
		{"host issue list", 5},
		// issue create: 6 local (summary, project-key, type, dry-run, description, priority)
		{"host issue create", 6},
		// issue edit: 4 local (issue-key, summary, type, dry-run)
		{"host issue edit", 4},
		// auth login: 3 local (email, token, server)
		{"host auth login", 3},
	}

	for _, tc := range cases {
		cmd, ok := docent.FindByPath(tree, tc.path)
		if !ok {
			t.Errorf("command %q not found in tree", tc.path)

			continue
		}

		if got := len(cmd.Flags); got < tc.minFlags {
			t.Errorf("command %q: got %d flags, want at least %d", tc.path, got, tc.minFlags)
		}
	}
}

// TestHostContract_enumValuesPresent asserts that flags with the host flag library enum
// annotations have their enum values captured in the docent IR. This is the
// critical "no information loss" check for annotation-declared enums.
func TestHostContract_enumValuesPresent(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)

	type enumCheck struct {
		cmdPath  string
		flagName string
		wantEnum []string
	}

	cases := []enumCheck{
		{
			// Root persistent --output has enum annotation (global on child, but check on root itself)
			cmdPath:  "host",
			flagName: "output",
			wantEnum: []string{"auto", "compact", "human", "json"},
		},
		{
			cmdPath:  "host",
			flagName: "color",
			wantEnum: []string{"always", "auto", "never"},
		},
		{
			cmdPath:  "host issue create",
			flagName: "type",
			wantEnum: []string{"Bug", "Epic", "Story", "Sub-task", "Task"},
		},
		{
			cmdPath:  "host issue create",
			flagName: "priority",
			wantEnum: []string{"High", "Highest", "Low", "Lowest", "Medium"},
		},
	}

	for _, tc := range cases {
		cmd, ok := docent.FindByPath(tree, tc.cmdPath)
		if !ok {
			t.Errorf("command %q not found", tc.cmdPath)

			continue
		}

		var found *docent.Flag
		for i := range cmd.Flags {
			if cmd.Flags[i].Name == tc.flagName {
				found = &cmd.Flags[i]

				break
			}
		}

		if found == nil {
			t.Errorf("command %q: flag --%s not found", tc.cmdPath, tc.flagName)

			continue
		}

		if !slices.Equal(found.Enum, tc.wantEnum) {
			t.Errorf("command %q flag --%s: Enum = %v, want %v",
				tc.cmdPath, tc.flagName, found.Enum, tc.wantEnum)
		}
	}
}

// TestHostContract_flagGroupsPresent asserts that mutually-exclusive and
// required-together flag groups are captured in the docent IR.
func TestHostContract_flagGroupsPresent(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)

	type groupCheck struct {
		cmdPath   string
		kind      docent.FlagGroupKind
		wantFlags []string
	}

	cases := []groupCheck{
		{"host issue list", docent.FlagGroupMutuallyExclusive, []string{"board", "board-id"}},
		{"host issue edit", docent.FlagGroupRequiredTogether, []string{"summary", "type"}},
		{"host auth login", docent.FlagGroupRequiredTogether, []string{"email", "token"}},
	}

	for _, tc := range cases {
		cmd, ok := docent.FindByPath(tree, tc.cmdPath)
		if !ok {
			t.Errorf("command %q not found", tc.cmdPath)

			continue
		}

		assertFlagGroupPresent(t, tc.cmdPath, cmd.FlagGroups, tc.kind, tc.wantFlags)
	}
}

// assertFlagGroupPresent verifies that a flag group of kind containing exactly
// wantFlags appears in groups.
func assertFlagGroupPresent(t *testing.T, cmdPath string, groups []docent.FlagGroup, kind docent.FlagGroupKind, wantFlags []string) {
	t.Helper()

	matching := flagGroupsByKind(groups, kind)
	if len(matching) == 0 {
		t.Errorf("command %q: no %s groups captured", cmdPath, kind)

		return
	}

	for _, g := range matching {
		if slices.Equal(g.Flags, wantFlags) {
			return
		}
	}

	t.Errorf("command %q: %s group %v not found in %v", cmdPath, kind, wantFlags, matching)
}

// TestHostContract_hostSchemasPresent asserts that host-registered input/output
// schemas appear in the docent IR after SchemaRegistry.Apply. This verifies the
// "host schemas all present" requirement of Sub-AC 3d.
func TestHostContract_hostSchemasPresent(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)
	reg := buildHostSchemaRegistry()
	enriched := reg.Apply(tree)

	type schemaCheck struct {
		cmdPath    string
		wantInput  bool
		wantOutput bool
	}

	cases := []schemaCheck{
		{"host issue list", false, true},
		{"host issue create", true, true},
		{"host issue edit", true, true},
	}

	for _, tc := range cases {
		cmd, ok := docent.FindByPath(enriched, tc.cmdPath)
		if !ok {
			t.Errorf("command %q not found in enriched tree", tc.cmdPath)

			continue
		}

		if tc.wantInput && cmd.InputSchema == nil {
			t.Errorf("command %q: InputSchema is nil; expected host schema to be registered", tc.cmdPath)
		}

		if !tc.wantInput && cmd.InputSchema != nil {
			t.Errorf("command %q: InputSchema is non-nil; no schema registered for this command", tc.cmdPath)
		}

		if tc.wantOutput && cmd.OutputSchema == nil {
			t.Errorf("command %q: OutputSchema is nil; expected host schema to be registered", tc.cmdPath)
		}

		if !tc.wantOutput && cmd.OutputSchema != nil {
			t.Errorf("command %q: OutputSchema is non-nil; no schema registered for this command", tc.cmdPath)
		}
	}
}

// TestHostContract_requiredFlagsMarked verifies that flags marked required via
// cobra's MarkFlagRequired appear as Required=true in the docent IR.
func TestHostContract_requiredFlagsMarked(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)

	type reqCheck struct {
		cmdPath  string
		flagName string
	}

	cases := []reqCheck{
		{"host issue create", "summary"},
		{"host issue create", "project-key"},
		{"host issue edit", "issue-key"},
		{"host auth login", "email"},
		{"host auth login", "server"},
	}

	for _, tc := range cases {
		cmd, ok := docent.FindByPath(tree, tc.cmdPath)
		if !ok {
			t.Errorf("command %q not found", tc.cmdPath)

			continue
		}

		var found *docent.Flag
		for i := range cmd.Flags {
			if cmd.Flags[i].Name == tc.flagName {
				found = &cmd.Flags[i]

				break
			}
		}

		if found == nil {
			t.Errorf("command %q: flag --%s not found", tc.cmdPath, tc.flagName)

			continue
		}

		if !found.Required {
			t.Errorf("command %q flag --%s: Required = false, want true", tc.cmdPath, tc.flagName)
		}
	}
}

// TestHostContract_persistentFlagsHoisted verifies the hoisting contract on a
// realistic tree: root persistent flags appear exactly once — on the root,
// marked Persistent — and are not repeated on any subcommand, so no flag
// information is lost while the schema stops paying for inherited repetition.
func TestHostContract_persistentFlagsHoisted(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)

	// The root has persistent --output, --color, --profile, --debug, --no-input.
	persistentFlags := []string{"output", "color", "profile", "debug", "no-input"}

	for _, name := range persistentFlags {
		var found *docent.Flag
		for i := range tree.Flags {
			if tree.Flags[i].Name == name {
				found = &tree.Flags[i]

				break
			}
		}

		if found == nil {
			t.Errorf("root: persistent flag --%s not found on its defining command", name)

			continue
		}

		if !found.Persistent {
			t.Errorf("root flag --%s: Persistent = false, want true", name)
		}
	}

	subPaths := []string{"host issue list", "host issue create", "host auth login"}

	for _, path := range subPaths {
		cmd, ok := docent.FindByPath(tree, path)
		if !ok {
			t.Errorf("command %q not found", path)

			continue
		}

		for _, name := range persistentFlags {
			for i := range cmd.Flags {
				if cmd.Flags[i].Name == name {
					t.Errorf("command %q repeats inherited flag --%s; it must appear only on the root", path, name)
				}
			}
		}
	}
}

// TestHostContract_hiddenCommandCaptured verifies that hidden subcommands are
// captured in the docent IR with Hidden=true.
func TestHostContract_hiddenCommandCaptured(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)

	logout, ok := docent.FindByPath(tree, "host auth logout")
	if !ok {
		t.Fatal("host auth logout not found in tree; hidden commands must be captured")
	}

	if !logout.Hidden {
		t.Error("host auth logout: Hidden = false, want true")
	}
}

// TestHostContract_pathFlag verifies that the adapter's --path flag correctly
// subsets the host tree to a named subtree.
func TestHostContract_pathFlag(t *testing.T) {
	t.Parallel()

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)
	cfg := docent.Config{Command: tree}

	out, err := executeAgent(cfg, nil, "agent", "schema", "--path", "host issue create")
	if err != nil {
		t.Fatalf("agent schema --path: %v", err)
	}

	var got docent.Command
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}

	if got.Name != "create" {
		t.Errorf("Name = %q, want %q", got.Name, "create")
	}

	if got.Path != "host issue create" {
		t.Errorf("Path = %q, want %q", got.Path, "host issue create")
	}

	// The subtree must not contain siblings like "list" or "edit".
	for _, child := range got.Children {
		if child.Name == "list" || child.Name == "edit" {
			t.Errorf("subtree contains sibling %q; --path must return only the targeted node", child.Name)
		}
	}
}

// buildHostExtraCommands returns the two host-supplied subcommands that
// a real-world host CLI mounts under the agent command group to expose domain-specific
// information to agents. Both are read-only discovery commands:
//
//   - adf-matrix: emits the Atlassian Document Format node-type compatibility
//     matrix as JSON so agents can construct valid ADF payloads for --description.
//   - fieldtypes: lists the field type identifiers that the connected Host
//     instance supports, enabling agents to pick the right type when creating
//     custom fields.
func buildHostExtraCommands(out io.Writer) (*gocobra.Command, *gocobra.Command) {
	emit := func(cmd *gocobra.Command, payload string) error {
		w := cmd.OutOrStdout()
		if out != nil {
			w = out
		}

		_, err := fmt.Fprint(w, payload)

		return err
	}

	adfMatrix := &gocobra.Command{
		Use:   "adf-matrix",
		Short: "Emit the ADF node-type compatibility matrix as JSON.",
		RunE: func(cmd *gocobra.Command, _ []string) error {
			return emit(cmd, `{"nodeTypes":["doc","paragraph","text","hardBreak","heading","bulletList","orderedList","listItem","codeBlock","blockquote","rule","panel","mention","emoji","date","status"]}`+"\n")
		},
	}

	fieldTypes := &gocobra.Command{
		Use:   "fieldtypes",
		Short: "List Host issue field type identifiers.",
		RunE: func(cmd *gocobra.Command, _ []string) error {
			return emit(cmd, `["string","number","date","datetime","option","user","array","issuelinks","priority","status","issuetype"]`+"\n")
		},
	}

	return adfMatrix, fieldTypes
}

// TestHostContract_extraCommandsMounted verifies that host-supplied extra
// commands are reachable under the agent command group when mounted via
// WithExtraCommands. This exercises a real-world host pattern of mounting
// adf-matrix and fieldtypes as agent-only discovery commands.
func TestHostContract_extraCommandsMounted(t *testing.T) {
	t.Parallel()

	adfMatrix, fieldTypes := buildHostExtraCommands(nil)
	cfg := docent.Config{}

	for _, tc := range []struct {
		name   string
		args   []string
		wantIn string
	}{
		{"adf-matrix", []string{"agent", "adf-matrix"}, "nodeTypes"},
		{"fieldtypes", []string{"agent", "fieldtypes"}, "issuetype"},
	} {
		var buf bytes.Buffer

		host := &gocobra.Command{Use: "host-test", Short: "Test host."}
		host.SilenceErrors = true
		host.SilenceUsage = true
		host.SetOut(&buf)
		host.AddCommand(docentcobra.NewCommand(cfg, docentcobra.WithExtraCommands(adfMatrix, fieldTypes)))
		host.SetArgs(tc.args)

		if err := host.Execute(); err != nil {
			t.Errorf("%s: Execute: %v", tc.name, err)

			continue
		}

		got := buf.String()
		if !strings.Contains(got, tc.wantIn) {
			t.Errorf("%s: output = %q; want to contain %q", tc.name, got, tc.wantIn)
		}
	}
}

// TestHostContract_nilOptionsIgnored verifies that nil Option values and nil
// extra commands are no-ops rather than panics — a library must not take the
// host down over a conditionally built option that stayed nil.
func TestHostContract_nilOptionsIgnored(t *testing.T) {
	t.Parallel()

	adfMatrix, _ := buildHostExtraCommands(nil)

	var buf bytes.Buffer

	host := &gocobra.Command{Use: "host-test", Short: "Test host."}
	host.SilenceErrors = true
	host.SilenceUsage = true
	host.SetOut(&buf)
	host.AddCommand(docentcobra.NewCommand(docent.Config{}, nil,
		docentcobra.WithExtraCommands(nil, adfMatrix)))
	host.SetArgs([]string{"agent", "adf-matrix"})

	if err := host.Execute(); err != nil {
		t.Fatalf("Execute with nil option and nil extra command: %v", err)
	}

	if !strings.Contains(buf.String(), "nodeTypes") {
		t.Errorf("non-nil extra command was not mounted alongside ignored nils: %q", buf.String())
	}
}

// TestHostContract_extraCommandsShareOut verifies that extra commands mounted
// via WithExtraCommands can write to the same Config.Out as the built-in
// subcommands. This confirms the host integration contract: one writer serves
// the whole agent command group.
func TestHostContract_extraCommandsShareOut(t *testing.T) {
	t.Parallel()

	var hostOut bytes.Buffer

	adfMatrix, fieldTypes := buildHostExtraCommands(&hostOut)
	cfg := docent.Config{Out: &hostOut}

	for _, tc := range []struct {
		args   []string
		wantIn string
	}{
		{[]string{"agent", "adf-matrix"}, "nodeTypes"},
		{[]string{"agent", "fieldtypes"}, "issuetype"},
	} {
		hostOut.Reset()

		var cobraOut bytes.Buffer

		host := &gocobra.Command{Use: "host-test", Short: "Test host."}
		host.SilenceErrors = true
		host.SilenceUsage = true
		host.SetOut(&cobraOut)
		host.AddCommand(docentcobra.NewCommand(cfg, docentcobra.WithExtraCommands(adfMatrix, fieldTypes)))
		host.SetArgs(tc.args)

		if err := host.Execute(); err != nil {
			t.Errorf("%v: Execute: %v", tc.args, err)

			continue
		}

		if hostOut.Len() == 0 {
			t.Errorf("%v: Config.Out was never written to", tc.args)

			continue
		}

		if !strings.Contains(hostOut.String(), tc.wantIn) {
			t.Errorf("%v: Config.Out output = %q; want to contain %q", tc.args, hostOut.String(), tc.wantIn)
		}
	}
}

// TestHostContract_extraCommandsDontConflict verifies that mounting extra
// commands via WithExtraCommands does not shadow or remove any built-in
// agent subcommand (guide, schema, export).
func TestHostContract_extraCommandsDontConflict(t *testing.T) {
	t.Parallel()

	adfMatrix, fieldTypes := buildHostExtraCommands(nil)

	root := buildHostFaithfulTree()
	tree := docentcobra.Tree(root)
	cfg := docent.Config{Command: tree}

	// Built-in commands must still work after mounting extra commands.
	out, err := executeAgent(
		cfg,
		[]docentcobra.Option{docentcobra.WithExtraCommands(adfMatrix, fieldTypes)},
		"agent", "schema",
	)
	if err != nil {
		t.Fatalf("agent schema with extra commands: %v", err)
	}

	var got docent.Command
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}

	if got.Name != "host" {
		t.Errorf("schema root Name = %q, want %q", got.Name, "host")
	}
}

// checkHostGolden compares got against the golden file under
// testdata/golden/<name>.json. Run with -update-host to regenerate.
func checkHostGolden(t *testing.T, name, got string) {
	t.Helper()

	checkGolden(t, filepath.Join("testdata", "golden", name+".json"), got, *updateHostGolden, "update-host")
}
