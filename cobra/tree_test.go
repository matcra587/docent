package cobra_test

import (
	"testing"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
	gocobra "github.com/spf13/cobra"
)

// enumValue is a pflag.Value implementation that exposes a finite set of
// allowed values via the enumProvider interface. This exercises the Enum
// extraction path in the adapter without any extra dependency.
type enumValue struct {
	val     string
	allowed []string
}

func (e *enumValue) String() string { return e.val }
func (e *enumValue) Type() string   { return "string" }
func (e *enumValue) Enum() []string { return e.allowed }

func (e *enumValue) Set(s string) error {
	e.val = s

	return nil
}

// buildTestTree constructs a synthetic cobra command tree with a known
// configuration for use in all Tree tests.
//
// Structure:
//
//	app  (root, persistent --verbose bool flag)
//	├── create  (--name string required; --format enum; --output/--quiet mutually exclusive; --user/--token required together)
//	└── list    (hidden; inherits --verbose from root)
func buildTestTree() *gocobra.Command {
	root := &gocobra.Command{
		Use:   "app",
		Short: "App root command.",
	}
	root.PersistentFlags().Bool("verbose", false, "Enable verbose output.")

	create := &gocobra.Command{
		Use:   "create",
		Short: "Create a resource.",
	}

	// --name: required string flag.
	create.Flags().String("name", "", "Name of the resource.")
	if err := create.MarkFlagRequired("name"); err != nil {
		panic(err)
	}

	// --format: enum flag via custom pflag.Value.
	fmtVal := &enumValue{val: "json", allowed: []string{"json", "yaml", "table"}}
	create.Flags().Var(fmtVal, "format", "Output format.")

	// --output and --quiet: mutually exclusive.
	create.Flags().String("output", "", "Output file path.")
	create.Flags().Bool("quiet", false, "Suppress output.")
	create.MarkFlagsMutuallyExclusive("output", "quiet")

	// --user and --token: required together.
	create.Flags().String("user", "", "Username for authentication.")
	create.Flags().String("token", "", "Auth token.")
	create.MarkFlagsRequiredTogether("user", "token")

	// --json and --yaml: at least one required.
	create.Flags().Bool("json", false, "Emit JSON.")
	create.Flags().Bool("yaml", false, "Emit YAML.")
	create.MarkFlagsOneRequired("json", "yaml")

	// --level: enum declared via the clib extras annotation (the JSON blob
	// gechr/clib's Extend writes), read by the adapter without importing clib.
	create.Flags().String("level", "info", "Log level.")
	if err := create.Flags().SetAnnotation("level", "clib.extra",
		[]string{`{"enum":["warn","info","debug"],"placeholder":"level"}`}); err != nil {
		panic(err)
	}

	// --broken: a malformed clib extras blob must be ignored, never fatal.
	create.Flags().String("broken", "", "Flag with malformed clib extras.")
	if err := create.Flags().SetAnnotation("broken", "clib.extra",
		[]string{`{not json`}); err != nil {
		panic(err)
	}

	// --mixed: a clib enum with a non-string entry disqualifies the whole
	// list, but the rest of the blob still surfaces as extensions.
	create.Flags().String("mixed", "", "Flag with a non-string clib enum entry.")
	if err := create.Flags().SetAnnotation("mixed", "clib.extra",
		[]string{`{"enum":[1,"a"],"placeholder":"m"}`}); err != nil {
		panic(err)
	}

	// --project: persistent on a MID-TREE command (not the root), inherited
	// by apply below. --dup: the same name defined both local and persistent
	// on create — local wins for create itself, persistent applies below.
	create.PersistentFlags().String("project", "", "Project key.")
	create.Flags().String("dup", "local-default", "Local variant.")
	create.PersistentFlags().String("dup", "persist-default", "Persistent variant.")

	// apply: child of create. Its mutex group spans a local flag and the
	// root's inherited --verbose, pinning where such a group is emitted.
	apply := &gocobra.Command{
		Use:   "apply",
		Short: "Apply a resource.",
	}
	apply.Flags().Bool("force", false, "Skip confirmation.")
	create.AddCommand(apply)

	list := &gocobra.Command{
		Use:        "list",
		Short:      "List resources.",
		Hidden:     true,
		Aliases:    []string{"ls", "enumerate"},
		Deprecated: "use \"create\" listings instead.",
	}

	root.AddCommand(create, list)

	// Marked after assembly: cobra resolves inherited members through the
	// parent chain, so the tree must exist before a group can span --verbose.
	apply.MarkFlagsMutuallyExclusive("force", "verbose")

	return root
}

// TestTree_rootCommand verifies the root command's name and path.
func TestTree_rootCommand(t *testing.T) {
	t.Parallel()

	root := buildTestTree()
	cmd := docentcobra.Tree(root)

	if cmd.Name != "app" {
		t.Errorf("Name = %q, want %q", cmd.Name, "app")
	}

	if cmd.Path != "app" {
		t.Errorf("Path = %q, want %q", cmd.Path, "app")
	}

	if cmd.Description != "App root command." {
		t.Errorf("Description = %q, want %q", cmd.Description, "App root command.")
	}

	if cmd.Hidden {
		t.Error("Root command should not be hidden")
	}
}

// TestTree_children verifies child commands are present and sorted.
func TestTree_children(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())

	// cobra adds a __complete hidden command; filter to our known children by name.
	children := namedChildren(cmd, "create", "list")

	if len(children) != 2 {
		t.Fatalf("want 2 named children (create, list), got %d", len(children))
	}

	// Children must be sorted by name: create < list.
	if children[0].Name != "create" {
		t.Errorf("children[0].Name = %q, want %q", children[0].Name, "create")
	}

	if children[1].Name != "list" {
		t.Errorf("children[1].Name = %q, want %q", children[1].Name, "list")
	}
}

// TestTree_childPaths verifies child paths include the parent name.
func TestTree_childPaths(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	children := namedChildren(cmd, "create", "list")

	for _, tc := range []struct{ name, wantPath string }{
		{"create", "app create"},
		{"list", "app list"},
	} {
		child := findChild(children, tc.name)
		if child == nil {
			t.Fatalf("child %q not found", tc.name)
		}

		if child.Path != tc.wantPath {
			t.Errorf("child %q Path = %q, want %q", tc.name, child.Path, tc.wantPath)
		}
	}
}

// TestTree_hiddenCommand verifies the Hidden field is propagated.
func TestTree_hiddenCommand(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	children := namedChildren(cmd, "create", "list")

	create := findChild(children, "create")
	list := findChild(children, "list")

	if create == nil || list == nil {
		t.Fatal("expected both create and list children")
	}

	if create.Hidden {
		t.Error("create should not be hidden")
	}

	if !list.Hidden {
		t.Error("list should be hidden")
	}
}

// TestTree_requiredFlag verifies that MarkFlagRequired produces Required=true in the IR.
func TestTree_requiredFlag(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	nameFlag := findFlag(create.Flags, "name")
	if nameFlag == nil {
		t.Fatal("flag --name not found on create")
	}

	if !nameFlag.Required {
		t.Error("--name should be Required=true")
	}
}

// TestTree_enumFlag verifies that an enumProvider pflag.Value exposes Enum in the IR.
func TestTree_enumFlag(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	fmtFlag := findFlag(create.Flags, "format")
	if fmtFlag == nil {
		t.Fatal("flag --format not found on create")
	}

	if fmtFlag.Type != "string" {
		t.Errorf("--format Type = %q, want %q", fmtFlag.Type, "string")
	}

	wantEnum := []string{"json", "table", "yaml"}
	if !stringSliceEq(fmtFlag.Enum, wantEnum) {
		t.Errorf("--format Enum = %v, want %v", fmtFlag.Enum, wantEnum)
	}
}

// TestTree_mutuallyExclusiveGroup verifies mutual-exclusion flag groups.
func TestTree_mutuallyExclusiveGroup(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	mexGroups := flagGroupsByKind(create.FlagGroups, docent.FlagGroupMutuallyExclusive)
	if len(mexGroups) != 1 {
		t.Fatalf("want 1 mutually_exclusive group, got %d", len(mexGroups))
	}

	want := []string{"output", "quiet"}
	if !stringSliceEq(mexGroups[0].Flags, want) {
		t.Errorf("mutually_exclusive flags = %v, want %v", mexGroups[0].Flags, want)
	}
}

// TestTree_requiredTogetherGroup verifies required-together flag groups.
func TestTree_requiredTogetherGroup(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	rtGroups := flagGroupsByKind(create.FlagGroups, docent.FlagGroupRequiredTogether)
	if len(rtGroups) != 1 {
		t.Fatalf("want 1 required_together group, got %d", len(rtGroups))
	}

	want := []string{"token", "user"}
	if !stringSliceEq(rtGroups[0].Flags, want) {
		t.Errorf("required_together flags = %v, want %v", rtGroups[0].Flags, want)
	}
}

// TestTree_oneRequiredGroup verifies one-required flag groups (at least one
// of the flags must be set) surface in the schema.
func TestTree_oneRequiredGroup(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	orGroups := flagGroupsByKind(create.FlagGroups, docent.FlagGroupOneRequired)
	if len(orGroups) != 1 {
		t.Fatalf("want 1 one_required group, got %d", len(orGroups))
	}

	want := []string{"json", "yaml"}
	if !stringSliceEq(orGroups[0].Flags, want) {
		t.Errorf("one_required flags = %v, want %v", orGroups[0].Flags, want)
	}
}

// TestTree_aliasesAndDeprecated verifies command aliases surface sorted and
// the deprecation message is carried, so an agent can recognize alternate
// names and avoid deprecated commands.
func TestTree_aliasesAndDeprecated(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	list := findChild(namedChildren(cmd, "list"), "list")

	if list == nil {
		t.Fatal("list command not found")
	}

	wantAliases := []string{"enumerate", "ls"}
	if !stringSliceEq(list.Aliases, wantAliases) {
		t.Errorf("Aliases = %v, want %v (sorted)", list.Aliases, wantAliases)
	}

	if list.Deprecated != "use \"create\" listings instead." {
		t.Errorf("Deprecated = %q, want the deprecation message", list.Deprecated)
	}

	create := findChild(namedChildren(cmd, "create"), "create")
	if create == nil {
		t.Fatal("create command not found")
	}

	if len(create.Aliases) != 0 || create.Deprecated != "" {
		t.Errorf("create should carry no aliases/deprecation: %v %q", create.Aliases, create.Deprecated)
	}
}

// TestTree_clibEnumBridge verifies enum values declared through gechr/clib's
// extras annotation surface in the IR (sorted), and that a malformed extras
// blob is ignored rather than failing the walk — another library's metadata
// must never break schema emission.
func TestTree_clibEnumBridge(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	levelFlag := findFlag(create.Flags, "level")
	if levelFlag == nil {
		t.Fatal("flag --level not found on create")
	}

	wantEnum := []string{"debug", "info", "warn"}
	if !stringSliceEq(levelFlag.Enum, wantEnum) {
		t.Errorf("--level Enum = %v, want %v (from clib.extra, sorted)", levelFlag.Enum, wantEnum)
	}

	// The rest of the blob surfaces under the "clib" extensions namespace,
	// with the modeled enum key stripped so the two cannot drift.
	clib, ok := levelFlag.Extensions["clib"].(map[string]any)
	if !ok {
		t.Fatalf("--level Extensions = %v, want a clib namespace map", levelFlag.Extensions)
	}

	if got := clib["placeholder"]; got != "level" {
		t.Errorf(`clib extensions placeholder = %v, want "level"`, got)
	}

	if _, present := clib["enum"]; present {
		t.Error("clib extensions still carry the enum key; it must be hoisted to Enum only")
	}

	brokenFlag := findFlag(create.Flags, "broken")
	if brokenFlag == nil {
		t.Fatal("flag --broken not found on create")
	}

	if brokenFlag.Enum != nil {
		t.Errorf("--broken Enum = %v, want nil for a malformed extras blob", brokenFlag.Enum)
	}

	if brokenFlag.Extensions != nil {
		t.Errorf("--broken Extensions = %v, want nil for a malformed extras blob", brokenFlag.Extensions)
	}
}

// TestTree_clibMixedEnumDisqualified pins that a non-string clib enum entry
// disqualifies the whole list — a half-usable enum would misrepresent the
// flag's contract — while the blob's other keys still surface as extensions.
func TestTree_clibMixedEnumDisqualified(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	mixedFlag := findFlag(create.Flags, "mixed")
	if mixedFlag == nil {
		t.Fatal("flag --mixed not found on create")
	}

	if mixedFlag.Enum != nil {
		t.Errorf("--mixed Enum = %v, want nil for a non-string enum entry", mixedFlag.Enum)
	}

	mixedClib, ok := mixedFlag.Extensions["clib"].(map[string]any)
	if !ok || mixedClib["placeholder"] != "m" {
		t.Errorf("--mixed Extensions = %v; want clib namespace with placeholder intact", mixedFlag.Extensions)
	}
}

// TestTree_midTreePersistentFlag pins hoisting for a persistent flag defined
// on a non-root command: it appears once on its definer, marked Persistent,
// and is not repeated on the definer's descendants.
func TestTree_midTreePersistentFlag(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	project := findFlag(create.Flags, "project")
	if project == nil {
		t.Fatal("--project not found on create, its defining command")
	}

	if !project.Persistent {
		t.Error("--project should be Persistent=true on create")
	}

	apply := findChild(create.Children, "apply")
	if apply == nil {
		t.Fatal("apply command not found under create")
	}

	if inherited := findFlag(apply.Flags, "project"); inherited != nil {
		t.Errorf("inherited --project must not repeat on apply: %+v", inherited)
	}
}

// TestTree_sameNameLocalAndPersistent pins the degenerate cobra setup where
// one command defines the same flag name both local and persistent: both
// variants are emitted on the definer — local first, correctly unmarked, so
// neither the descendant-facing persistent variant nor the command's own
// local override is lost.
func TestTree_sameNameLocalAndPersistent(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	var dups []docent.Flag
	for _, f := range create.Flags {
		if f.Name == "dup" {
			dups = append(dups, f)
		}
	}

	if len(dups) != 2 {
		t.Fatalf("want both --dup variants on create, got %d: %+v", len(dups), dups)
	}

	if dups[0].Persistent || dups[0].Default != "local-default" {
		t.Errorf("first --dup should be the local variant: %+v", dups[0])
	}

	if !dups[1].Persistent || dups[1].Default != "persist-default" {
		t.Errorf("second --dup should be the persistent variant: %+v", dups[1])
	}

	apply := findChild(create.Children, "apply")
	if apply == nil {
		t.Fatal("apply command not found under create")
	}

	if f := findFlag(apply.Flags, "dup"); f != nil {
		t.Errorf("inherited --dup must not repeat on apply: %+v", f)
	}
}

// TestTree_groupWithInheritedMember pins where a flag group spanning an
// inherited flag is emitted: on the marking command (one member local, all
// visible), and nowhere else — cobra's shared-flag annotations would
// otherwise leak the group to every command in the tree.
func TestTree_groupWithInheritedMember(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	apply := findChild(create.Children, "apply")
	if apply == nil {
		t.Fatal("apply command not found under create")
	}

	want := []string{"force", "verbose"}

	mex := flagGroupsByKind(apply.FlagGroups, docent.FlagGroupMutuallyExclusive)
	if len(mex) != 1 || !stringSliceEq(mex[0].Flags, want) {
		t.Errorf("apply mutually_exclusive groups = %+v, want one group %v", mex, want)
	}

	for _, g := range flagGroupsByKind(cmd.FlagGroups, docent.FlagGroupMutuallyExclusive) {
		if stringSliceEq(g.Flags, want) {
			t.Errorf("the force/verbose group leaked to the root: %+v", g)
		}
	}

	for _, g := range flagGroupsByKind(create.FlagGroups, docent.FlagGroupMutuallyExclusive) {
		if stringSliceEq(g.Flags, want) {
			t.Errorf("the force/verbose group leaked to create: %+v", g)
		}
	}
}

// TestTree_persistentFlagHoisted pins the hoisting contract: a persistent
// flag appears exactly once in the tree — on the command that defines it,
// marked Persistent — and is not repeated on descendants that inherit it.
func TestTree_persistentFlagHoisted(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())

	verboseFlag := findFlag(cmd.Flags, "verbose")
	if verboseFlag == nil {
		t.Fatal("--verbose flag not found on root, its defining command")
	}

	if !verboseFlag.Persistent {
		t.Error("--verbose should be Persistent=true on the root that defines it")
	}

	list := findChild(namedChildren(cmd, "list"), "list")
	if list == nil {
		t.Fatal("list command not found")
	}

	if inherited := findFlag(list.Flags, "verbose"); inherited != nil {
		t.Errorf("inherited --verbose must not repeat on list; each flag appears once, on its definer: %+v", inherited)
	}
}

// TestTree_localFlag verifies that a plain local flag is not marked Persistent.
func TestTree_localFlag(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(buildTestTree())
	create := findChild(namedChildren(cmd, "create"), "create")

	if create == nil {
		t.Fatal("create command not found")
	}

	nameFlag := findFlag(create.Flags, "name")
	if nameFlag == nil {
		t.Fatal("--name flag not found on create")
	}

	if nameFlag.Persistent {
		t.Error("--name is a local flag and should be Persistent=false")
	}
}

// TestTree_nilRoot verifies that a nil root returns the zero-value Command.
func TestTree_nilRoot(t *testing.T) {
	t.Parallel()

	cmd := docentcobra.Tree(nil)

	if cmd.Name != "" || cmd.Path != "" || cmd.Children != nil {
		t.Errorf("nil root: got non-zero Command %+v", cmd)
	}
}

// TestTree_determinism verifies that running Tree twice on the same tree
// produces identical results (no map-order or sort instability).
func TestTree_determinism(t *testing.T) {
	t.Parallel()

	first := docentcobra.Tree(buildTestTree())
	second := docentcobra.Tree(buildTestTree())

	if !commandsEqual(first, second) {
		t.Error("Tree produced different results on two identical runs (non-deterministic)")
	}
}

// TestTree_flagTypeAndDefault verifies type and default extraction for basic pflag types.
func TestTree_flagTypeAndDefault(t *testing.T) {
	t.Parallel()

	root := &gocobra.Command{Use: "root", Short: "Root."}
	root.Flags().String("strflag", "hello", "A string flag.")
	root.Flags().Bool("boolflag", true, "A bool flag.")
	root.Flags().Int("intflag", 42, "An int flag.")

	cmd := docentcobra.Tree(root)

	cases := []struct {
		name        string
		wantType    string
		wantDefault string
	}{
		{"boolflag", "bool", "true"},
		{"intflag", "int", "42"},
		{"strflag", "string", "hello"},
	}

	for _, tc := range cases {
		f := findFlag(cmd.Flags, tc.name)
		if f == nil {
			t.Errorf("flag --%s not found", tc.name)

			continue
		}

		if f.Type != tc.wantType {
			t.Errorf("--%s Type = %q, want %q", tc.name, f.Type, tc.wantType)
		}

		if f.Default != tc.wantDefault {
			t.Errorf("--%s Default = %q, want %q", tc.name, f.Default, tc.wantDefault)
		}
	}
}

// TestTree_shorthand verifies shorthand extraction.
func TestTree_shorthand(t *testing.T) {
	t.Parallel()

	root := &gocobra.Command{Use: "root", Short: "Root."}
	root.Flags().StringP("output", "o", "", "Output path.")

	cmd := docentcobra.Tree(root)

	f := findFlag(cmd.Flags, "output")
	if f == nil {
		t.Fatal("flag --output not found")
	}

	if f.Shorthand != "o" {
		t.Errorf("Shorthand = %q, want %q", f.Shorthand, "o")
	}
}

// TestTree_multipleMutexGroups verifies that multiple independent mutual-exclusion
// groups on the same command are all captured and sorted.
func TestTree_multipleMutexGroups(t *testing.T) {
	t.Parallel()

	root := &gocobra.Command{Use: "root", Short: "Root."}
	root.Flags().String("alpha", "", "")
	root.Flags().String("beta", "", "")
	root.Flags().String("gamma", "", "")
	root.Flags().String("delta", "", "")
	root.MarkFlagsMutuallyExclusive("alpha", "beta")
	root.MarkFlagsMutuallyExclusive("gamma", "delta")

	cmd := docentcobra.Tree(root)

	mexGroups := flagGroupsByKind(cmd.FlagGroups, docent.FlagGroupMutuallyExclusive)
	if len(mexGroups) != 2 {
		t.Fatalf("want 2 mutually_exclusive groups, got %d: %v", len(mexGroups), mexGroups)
	}

	// Groups must be sorted: {alpha, beta} < {delta, gamma}.
	if !stringSliceEq(mexGroups[0].Flags, []string{"alpha", "beta"}) {
		t.Errorf("group[0] = %v, want [alpha beta]", mexGroups[0].Flags)
	}

	if !stringSliceEq(mexGroups[1].Flags, []string{"delta", "gamma"}) {
		t.Errorf("group[1] = %v, want [delta gamma]", mexGroups[1].Flags)
	}
}

// TestTree_flagsSorted verifies that flags are always returned in sorted order.
func TestTree_flagsSorted(t *testing.T) {
	t.Parallel()

	root := &gocobra.Command{Use: "root", Short: "Root."}
	// Add flags in reverse alphabetical order; output must be sorted.
	root.Flags().String("zulu", "", "")
	root.Flags().String("alpha", "", "")
	root.Flags().String("mike", "", "")

	cmd := docentcobra.Tree(root)

	var names []string
	for _, f := range cmd.Flags {
		names = append(names, f.Name)
	}

	// --help is injected by cobra; find our three flags in the sorted output.
	alpha := findFlag(cmd.Flags, "alpha")
	mike := findFlag(cmd.Flags, "mike")
	zulu := findFlag(cmd.Flags, "zulu")

	if alpha == nil || mike == nil || zulu == nil {
		t.Fatalf("expected all three flags, got: %v", names)
	}

	for i := 1; i < len(cmd.Flags); i++ {
		if cmd.Flags[i].Name < cmd.Flags[i-1].Name {
			t.Errorf("flags not sorted at index %d: %q < %q", i, cmd.Flags[i].Name, cmd.Flags[i-1].Name)
		}
	}
}

// --- helpers ---

// namedChildren returns only the children whose Name is in names.
func namedChildren(cmd docent.Command, names ...string) []docent.Command {
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		want[n] = struct{}{}
	}

	var out []docent.Command
	for _, c := range cmd.Children {
		if _, ok := want[c.Name]; ok {
			out = append(out, c)
		}
	}

	return out
}

// findChild returns the first child in cs whose Name equals name, or nil.
func findChild(cs []docent.Command, name string) *docent.Command {
	for i := range cs {
		if cs[i].Name == name {
			return &cs[i]
		}
	}

	return nil
}

// findFlag returns the flag with the given name, or nil.
func findFlag(flags []docent.Flag, name string) *docent.Flag {
	for i := range flags {
		if flags[i].Name == name {
			return &flags[i]
		}
	}

	return nil
}

// flagGroupsByKind returns all groups of the given kind.
func flagGroupsByKind(groups []docent.FlagGroup, kind docent.FlagGroupKind) []docent.FlagGroup {
	var out []docent.FlagGroup
	for _, g := range groups {
		if g.Kind == kind {
			out = append(out, g)
		}
	}

	return out
}

// stringSliceEq reports whether a and b contain the same strings in the same order.
func stringSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// commandsEqual does a structural equality check on two Commands for determinism testing.
func commandsEqual(a, b docent.Command) bool {
	if a.Name != b.Name || a.Path != b.Path || a.Description != b.Description ||
		a.Hidden != b.Hidden || a.ReadOnly != b.ReadOnly {
		return false
	}

	return flagsEqual(a.Flags, b.Flags) &&
		groupsEqual(a.FlagGroups, b.FlagGroups) &&
		childrenEqual(a.Children, b.Children)
}

// flagsEqual reports whether two flag slices are structurally identical.
func flagsEqual(a, b []docent.Flag) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		af, bf := a[i], b[i]
		if af.Name != bf.Name || af.Type != bf.Type || af.Default != bf.Default ||
			af.Required != bf.Required || af.Persistent != bf.Persistent || af.Shorthand != bf.Shorthand ||
			!stringSliceEq(af.Enum, bf.Enum) {
			return false
		}
	}

	return true
}

// groupsEqual reports whether two flag-group slices are structurally identical.
func groupsEqual(a, b []docent.FlagGroup) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].Kind != b[i].Kind || !stringSliceEq(a[i].Flags, b[i].Flags) {
			return false
		}
	}

	return true
}

// childrenEqual reports whether two child-command slices are structurally identical.
func childrenEqual(a, b []docent.Command) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !commandsEqual(a[i], b[i]) {
			return false
		}
	}

	return true
}
