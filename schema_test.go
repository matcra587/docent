package docent_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/matcra587/docent"
)

// buildSchemaTestTree constructs a synthetic Command tree for schema registry tests.
//
//	tool
//	├── create  (leaf)
//	└── list    (leaf)
func buildSchemaTestTree() docent.Command {
	return docent.Command{
		Name: "tool",
		Path: "tool",
		Children: []docent.Command{
			{Name: "create", Path: "tool create"},
			{Name: "list", Path: "tool list"},
		},
	}
}

// findSchemaChild returns a pointer to the first direct child of cmd whose
// Name equals name, or nil when not found.
func findSchemaChild(cmd docent.Command, name string) *docent.Command {
	for i := range cmd.Children {
		if cmd.Children[i].Name == name {
			return &cmd.Children[i]
		}
	}

	return nil
}

// TestFindByPath_returnsDeepCopy verifies the boundary-copy contract on
// lookups: mutating the returned command's slice and map fields must not
// affect the searched tree.
func TestFindByPath_returnsDeepCopy(t *testing.T) {
	t.Parallel()

	tree := buildSchemaTestTree()
	create := findSchemaChild(tree, "create")
	create.Flags = []docent.Flag{{
		Name: "name", Type: "string", Enum: []string{"a", "b"},
		Extensions: map[string]any{"clib": map[string]any{"placeholder": "n"}},
	}}
	create.InputSchema = map[string]any{"type": "object"}

	found, ok := docent.FindByPath(tree, "tool create")
	if !ok {
		t.Fatal("tool create not found")
	}

	foundClib, ok := found.Flags[0].Extensions["clib"].(map[string]any)
	if !ok {
		t.Fatalf("found flag extensions clib = %v, want a map", found.Flags[0].Extensions)
	}

	found.Flags[0].Name = "mutated"
	found.Flags[0].Enum[0] = "mutated"
	foundClib["placeholder"] = "mutated"
	found.InputSchema["type"] = "mutated"

	orig := findSchemaChild(tree, "create")
	if orig.Flags[0].Name != "name" || orig.Flags[0].Enum[0] != "a" || orig.InputSchema["type"] != "object" {
		t.Errorf("mutating the FindByPath result changed the searched tree: %+v", orig)
	}

	origClib, ok := orig.Flags[0].Extensions["clib"].(map[string]any)
	if !ok {
		t.Fatalf("original flag extensions clib = %v, want a map", orig.Flags[0].Extensions)
	}

	if origClib["placeholder"] != "n" {
		t.Errorf("mutating the result's flag extensions changed the tree: placeholder = %v", origClib["placeholder"])
	}
}

// TestSchemaRegistry_inputSchema verifies that a registered input schema
// appears on the matching command after Apply.
func TestSchemaRegistry_inputSchema(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"required": []any{"name"},
	}

	reg := docent.SchemaRegistry{
		"tool create": {Input: input},
	}

	tree := reg.Apply(buildSchemaTestTree())
	create := findSchemaChild(tree, "create")

	if create == nil {
		t.Fatal("create command not found in tree")
	}

	if create.InputSchema == nil {
		t.Fatal("InputSchema is nil on create; want the registered schema")
	}

	if create.InputSchema["type"] != "object" {
		t.Errorf("InputSchema[\"type\"] = %v, want %q", create.InputSchema["type"], "object")
	}
}

// TestSchemaRegistry_outputSchema verifies that a registered output schema
// appears on the matching command after Apply.
func TestSchemaRegistry_outputSchema(t *testing.T) {
	t.Parallel()

	output := map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}

	reg := docent.SchemaRegistry{
		"tool list": {Output: output},
	}

	tree := reg.Apply(buildSchemaTestTree())
	list := findSchemaChild(tree, "list")

	if list == nil {
		t.Fatal("list command not found in tree")
	}

	if list.OutputSchema == nil {
		t.Fatal("OutputSchema is nil on list; want the registered schema")
	}

	if list.OutputSchema["type"] != "array" {
		t.Errorf("OutputSchema[\"type\"] = %v, want %q", list.OutputSchema["type"], "array")
	}
}

// TestSchemaRegistry_bothSchemas verifies that both input and output schemas
// can be registered for the same command simultaneously.
func TestSchemaRegistry_bothSchemas(t *testing.T) {
	t.Parallel()

	reg := docent.SchemaRegistry{
		"tool create": {
			Input:  map[string]any{"type": "object", "description": "the input"},
			Output: map[string]any{"type": "object", "description": "the output"},
		},
	}

	tree := reg.Apply(buildSchemaTestTree())
	create := findSchemaChild(tree, "create")

	if create == nil {
		t.Fatal("create command not found in tree")
	}

	if create.InputSchema == nil {
		t.Error("InputSchema is nil; want input schema")
	}

	if create.OutputSchema == nil {
		t.Error("OutputSchema is nil; want output schema")
	}
}

// TestSchemaRegistry_rootCommandSchema verifies that schemas can be registered
// for the root command path.
func TestSchemaRegistry_rootCommandSchema(t *testing.T) {
	t.Parallel()

	reg := docent.SchemaRegistry{
		"tool": {Input: map[string]any{"type": "object"}},
	}

	tree := reg.Apply(buildSchemaTestTree())

	if tree.InputSchema == nil {
		t.Fatal("InputSchema is nil on root; want the registered schema")
	}

	if tree.InputSchema["type"] != "object" {
		t.Errorf("root InputSchema[\"type\"] = %v, want %q", tree.InputSchema["type"], "object")
	}
}

// TestSchemaRegistry_unregisteredCommandUnchanged verifies that commands with
// no registry entry remain without schemas after Apply.
func TestSchemaRegistry_unregisteredCommandUnchanged(t *testing.T) {
	t.Parallel()

	reg := docent.SchemaRegistry{
		"tool create": {Input: map[string]any{"type": "object"}},
	}

	tree := reg.Apply(buildSchemaTestTree())
	list := findSchemaChild(tree, "list")

	if list == nil {
		t.Fatal("list command not found in tree")
	}

	if list.InputSchema != nil {
		t.Errorf("unregistered command has InputSchema = %v, want nil", list.InputSchema)
	}

	if list.OutputSchema != nil {
		t.Errorf("unregistered command has OutputSchema = %v, want nil", list.OutputSchema)
	}
}

// TestSchemaRegistry_emptyRegistryIsNoop verifies that an empty registry
// leaves the entire tree unchanged.
func TestSchemaRegistry_emptyRegistryIsNoop(t *testing.T) {
	t.Parallel()

	tree := docent.SchemaRegistry{}.Apply(buildSchemaTestTree())

	if tree.InputSchema != nil || tree.OutputSchema != nil {
		t.Error("empty registry added schemas to root; want none")
	}

	for _, child := range tree.Children {
		if child.InputSchema != nil || child.OutputSchema != nil {
			t.Errorf("child %q has non-nil schema after empty registry Apply", child.Name)
		}
	}
}

// TestSchemaRegistry_applyDoesNotMutateSource verifies that Apply does not
// modify the original Command tree that was passed in.
func TestSchemaRegistry_applyDoesNotMutateSource(t *testing.T) {
	t.Parallel()

	source := buildSchemaTestTree()

	reg := docent.SchemaRegistry{
		"tool create": {Input: map[string]any{"type": "object"}},
	}

	_ = reg.Apply(source)

	// The original tree's create command must still have nil schemas.
	original := findSchemaChild(source, "create")
	if original == nil {
		t.Fatal("create not found in original tree after Apply")
	}

	if original.InputSchema != nil {
		t.Error("Apply mutated the source tree; create.InputSchema should remain nil")
	}
}

// TestSchemaRegistry_applyIsolatesSchemaMap verifies that mutating the
// registered schema map after Apply does not affect the applied tree.
func TestSchemaRegistry_applyIsolatesSchemaMap(t *testing.T) {
	t.Parallel()

	schema := map[string]any{"type": "object"}

	reg := docent.SchemaRegistry{
		"tool create": {Input: schema},
	}

	tree := reg.Apply(buildSchemaTestTree())

	// Mutate the original map after Apply.
	schema["type"] = "string"

	create := findSchemaChild(tree, "create")
	if create == nil {
		t.Fatal("create not found in applied tree")
	}

	if create.InputSchema["type"] != "object" {
		t.Errorf("schema isolation failed: InputSchema[\"type\"] = %v after mutation, want %q",
			create.InputSchema["type"], "object")
	}
}

// TestSchemaRegistry_nestedSchemaIsolation verifies that nested objects within
// a registered schema are deep-copied, so mutations to the original do not
// affect the applied tree.
func TestSchemaRegistry_nestedSchemaIsolation(t *testing.T) {
	t.Parallel()

	props := map[string]any{"type": "string"}
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": props},
	}

	reg := docent.SchemaRegistry{
		"tool create": {Input: schema},
	}

	tree := reg.Apply(buildSchemaTestTree())

	// Mutate the nested properties map after Apply.
	props["type"] = "integer"

	create := findSchemaChild(tree, "create")
	if create == nil {
		t.Fatal("create not found in applied tree")
	}

	applied := create.InputSchema
	if applied == nil {
		t.Fatal("InputSchema is nil on create")
	}

	appliedProps, _ := applied["properties"].(map[string]any)
	if appliedProps == nil {
		t.Fatal("properties missing from applied InputSchema")
	}

	appliedName, _ := appliedProps["name"].(map[string]any)
	if appliedName == nil {
		t.Fatal("name missing from applied properties")
	}

	if appliedName["type"] != "string" {
		t.Errorf("nested deep copy failed: name.type = %v after mutation, want %q",
			appliedName["type"], "string")
	}
}

// TestSchemaRegistry_typedContainerIsolation pins the deep-copy contract for
// containers that are JSON-marshalable but not JSON-native: hosts building
// schemas as Go literals use []string (not []any) for values like "required",
// and mutating a lookup result must not reach the registry's backing storage.
func TestSchemaRegistry_typedContainerIsolation(t *testing.T) {
	t.Parallel()

	required := []string{"id"}
	labels := map[string]string{"kind": "issue"}
	schema := map[string]any{
		"type":     "object",
		"required": required,
		"labels":   labels,
	}

	reg := docent.SchemaRegistry{
		"tool create": {Input: schema},
	}

	tree := reg.Apply(buildSchemaTestTree())

	got, ok := docent.FindByPath(tree, "tool create")
	if !ok {
		t.Fatal("tool create not found in applied tree")
	}

	gotRequired, _ := got.InputSchema["required"].([]string)
	if len(gotRequired) != 1 {
		t.Fatalf("required missing from looked-up InputSchema: %v", got.InputSchema["required"])
	}

	gotLabels, _ := got.InputSchema["labels"].(map[string]string)
	if gotLabels == nil {
		t.Fatalf("labels missing from looked-up InputSchema: %v", got.InputSchema["labels"])
	}

	// Mutate the lookup result; the registry's storage must be unaffected.
	gotRequired[0] = "MUTATED"
	gotLabels["kind"] = "MUTATED"

	if required[0] != "id" {
		t.Errorf("typed slice aliased: registry required[0] = %q, want %q", required[0], "id")
	}

	if labels["kind"] != "issue" {
		t.Errorf("typed map aliased: registry labels[kind] = %q, want %q", labels["kind"], "issue")
	}

	// The applied tree must be isolated from the lookup result too: a second
	// lookup after mutating the first must still see the original values.
	again, ok := docent.FindByPath(tree, "tool create")
	if !ok {
		t.Fatal("tool create missing on second lookup")
	}

	againRequired, _ := again.InputSchema["required"].([]string)
	if len(againRequired) != 1 || againRequired[0] != "id" {
		t.Errorf("lookup results share storage: second lookup required = %v, want [id]", againRequired)
	}
}

// TestSchemaRegistry_nilElementsPreserved pins that nil elements inside
// interface-typed containers survive the deep copy: a nil in a []error must
// not panic the reflect copy, and a nil-valued map entry must keep its key
// rather than being silently deleted.
func TestSchemaRegistry_nilElementsPreserved(t *testing.T) {
	t.Parallel()

	reg := docent.SchemaRegistry{
		"tool create": {Input: map[string]any{
			"readers": []error{nil},
			"lookup":  map[string]error{"missing": nil},
		}},
	}

	tree := reg.Apply(buildSchemaTestTree())

	got, ok := docent.FindByPath(tree, "tool create")
	if !ok {
		t.Fatal("tool create not found in applied tree")
	}

	readers, _ := got.InputSchema["readers"].([]error)
	if len(readers) != 1 || readers[0] != nil {
		t.Errorf("nil slice element not preserved: %v", got.InputSchema["readers"])
	}

	lookup, _ := got.InputSchema["lookup"].(map[string]error)
	if v, present := lookup["missing"]; !present || v != nil {
		t.Errorf("nil-valued map entry not preserved: %v", got.InputSchema["lookup"])
	}
}

// TestCommand_extensionsBoundary pins the host-metadata slot: Extensions
// survive FindByPath as a deep copy (mutating a lookup result never reaches
// the source tree) and marshal under the "extensions" key.
func TestCommand_extensionsBoundary(t *testing.T) {
	t.Parallel()

	exitCodes := map[string]any{"auth": 1, "not_found": 2}
	tree := docent.Command{
		Name: "tool",
		Path: "tool",
		Extensions: map[string]any{
			"exit_codes": exitCodes,
			"env":        []string{"TOOL_TOKEN"},
		},
	}

	got, ok := docent.FindByPath(tree, "tool")
	if !ok {
		t.Fatal("tool not found")
	}

	gotCodes, _ := got.Extensions["exit_codes"].(map[string]any)
	if gotCodes == nil {
		t.Fatalf("exit_codes missing from looked-up Extensions: %v", got.Extensions)
	}

	gotCodes["auth"] = 99

	gotEnv, ok := got.Extensions["env"].([]string)
	if !ok || len(gotEnv) != 1 {
		t.Fatalf("env missing from looked-up Extensions: %v", got.Extensions)
	}

	gotEnv[0] = "MUTATED"

	if exitCodes["auth"] != 1 {
		t.Errorf("Extensions aliased: source exit_codes[auth] = %v, want 1", exitCodes["auth"])
	}

	if env, _ := tree.Extensions["env"].([]string); env[0] != "TOOL_TOKEN" {
		t.Errorf("Extensions typed slice aliased: %v", env)
	}

	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(data), `"extensions"`) {
		t.Errorf("extensions key missing from JSON: %s", data)
	}
}

// TestSchemaRegistry_appearsInJSONOutput verifies that a registered schema
// appears under the correct key in the JSON serialization of the command tree.
func TestSchemaRegistry_appearsInJSONOutput(t *testing.T) {
	t.Parallel()

	inputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}

	reg := docent.SchemaRegistry{
		"tool create": {Input: inputSchema},
	}

	tree := reg.Apply(buildSchemaTestTree())

	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var got map[string]any
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	children, _ := got["children"].([]any)
	if len(children) == 0 {
		t.Fatal("no children in JSON output")
	}

	var createJSON map[string]any

	for _, c := range children {
		child, _ := c.(map[string]any)
		if child["name"] == "create" {
			createJSON = child

			break
		}
	}

	if createJSON == nil {
		t.Fatal("create child not found in JSON output")
	}

	if createJSON["input_schema"] == nil {
		t.Error("input_schema key missing from JSON output for create command")
	}

	// list must not carry an input_schema key (omitempty drops nil maps).
	var listJSON map[string]any

	for _, c := range children {
		child, _ := c.(map[string]any)
		if child["name"] == "list" {
			listJSON = child

			break
		}
	}

	if listJSON == nil {
		t.Fatal("list child not found in JSON output")
	}

	if _, present := listJSON["input_schema"]; present {
		t.Error("input_schema present in JSON output for unregistered list command; want absent")
	}
}

// TestSchemaRegistry_outputSchemaInJSON verifies that a registered output
// schema appears under the output_schema key in JSON serialization.
func TestSchemaRegistry_outputSchemaInJSON(t *testing.T) {
	t.Parallel()

	outputSchema := map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "object"},
	}

	reg := docent.SchemaRegistry{
		"tool list": {Output: outputSchema},
	}

	data, err := json.Marshal(reg.Apply(buildSchemaTestTree()))
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var got map[string]any
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	children, _ := got["children"].([]any)

	var listJSON map[string]any

	for _, c := range children {
		child, _ := c.(map[string]any)
		if child["name"] == "list" {
			listJSON = child

			break
		}
	}

	if listJSON == nil {
		t.Fatal("list child not found in JSON output")
	}

	if listJSON["output_schema"] == nil {
		t.Error("output_schema key missing from JSON output for list command")
	}
}

// TestSchemaRegistry_determinism verifies that applying the same registry
// twice on the same tree produces byte-identical JSON output.
func TestSchemaRegistry_determinism(t *testing.T) {
	t.Parallel()

	reg := docent.SchemaRegistry{
		"tool create": {
			Input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":   map[string]any{"type": "integer"},
					"name": map[string]any{"type": "string"},
				},
			},
			Output: map[string]any{
				"type": "object",
			},
		},
		"tool list": {
			Output: map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "object"},
			},
		},
	}

	first, err := json.Marshal(reg.Apply(buildSchemaTestTree()))
	if err != nil {
		t.Fatalf("first json.Marshal: %v", err)
	}

	second, err := json.Marshal(reg.Apply(buildSchemaTestTree()))
	if err != nil {
		t.Fatalf("second json.Marshal: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("Apply is not deterministic:\nfirst:  %s\nsecond: %s", first, second)
	}
}

// TestCommand_StripShapes pins the full-tree emission policy primitive:
// embedded schema bodies are replaced by has_* markers wherever they exist
// in the tree, nodes without a schema stay unmarked, the source tree is
// never mutated, and the markers serialize under their documented JSON
// keys.
func TestCommand_StripShapes(t *testing.T) {
	t.Parallel()

	tree := buildSchemaTestTree()
	create := findSchemaChild(tree, "create")
	create.InputSchema = map[string]any{"type": "object"}
	create.OutputSchema = map[string]any{"type": "array"}
	tree.OutputSchema = map[string]any{"type": "object"}

	stripped := tree.StripShapes()

	assertShapeMarkers(t, stripped, false, true)
	assertShapeMarkers(t, *findSchemaChild(stripped, "create"), true, true)
	assertShapeMarkers(t, *findSchemaChild(stripped, "list"), false, false)

	if tree.OutputSchema == nil || findSchemaChild(tree, "create").InputSchema == nil {
		t.Error("StripShapes mutated its receiver; it must return a copy")
	}

	data, err := docent.MarshalSchema(*findSchemaChild(stripped, "create"))
	if err != nil {
		t.Fatalf("MarshalSchema: %v", err)
	}

	for _, marker := range []string{`"has_input_schema":true`, `"has_output_schema":true`} {
		if !strings.Contains(string(data), marker) {
			t.Errorf("emission %s lacks marker %s", data, marker)
		}
	}
}

// assertShapeMarkers fails the test unless c carries no embedded schema
// bodies and exactly the expected has_* markers.
func assertShapeMarkers(t *testing.T, c docent.Command, wantIn, wantOut bool) {
	t.Helper()

	if c.InputSchema != nil || c.OutputSchema != nil {
		t.Errorf("%s still embeds a schema body after StripShapes", c.Path)
	}

	if c.HasInputSchema != wantIn || c.HasOutputSchema != wantOut {
		t.Errorf("%s markers = input:%v output:%v; want input:%v output:%v",
			c.Path, c.HasInputSchema, c.HasOutputSchema, wantIn, wantOut)
	}
}

// TestMarshalSchema pins the canonical emission shape — compact, single
// line, no trailing newline — and the wrapped error for a tree carrying a
// non-marshalable extension value.
func TestMarshalSchema(t *testing.T) {
	t.Parallel()

	data, err := docent.MarshalSchema(docent.Command{Name: "app", Path: "app"})
	if err != nil {
		t.Fatalf("MarshalSchema: %v", err)
	}

	if want := `{"name":"app","path":"app"}`; string(data) != want {
		t.Errorf("MarshalSchema = %q, want %q", data, want)
	}

	_, err = docent.MarshalSchema(docent.Command{
		Name:       "app",
		Path:       "app",
		Extensions: map[string]any{"bad": make(chan int)},
	})
	if err == nil {
		t.Fatal("MarshalSchema: expected error for non-marshalable extension, got nil")
	}
}
