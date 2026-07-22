package cobra_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
	gocobra "github.com/spf13/cobra"
)

// buildSchemaHost returns a minimal cobra command tree and the corresponding
// docent.Command schema produced by the adapter. It is used by agent-schema
// tests to avoid a live cobra root in the schema output.
//
// Tree structure:
//
//	app
//	├── create  (--name string required)
//	└── list    (hidden)
func buildSchemaHost() (*gocobra.Command, docent.Command) {
	root := &gocobra.Command{Use: "app", Short: "App root command."}

	create := &gocobra.Command{Use: "create", Short: "Create a resource."}
	create.Flags().String("name", "", "Name of the resource.")

	list := &gocobra.Command{Use: "list", Short: "List resources.", Hidden: true}

	root.AddCommand(create, list)

	// Build the schema before adding agent commands so they do not appear in
	// the schema output — matching the real host integration pattern.
	schema := docentcobra.Tree(root)

	return root, schema
}

// TestAgentSchema_fullTree verifies that "agent schema" with no flags emits
// the full command tree as valid JSON with the correct root name.
func TestAgentSchema_fullTree(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()
	cfg := docent.Config{Command: schema}

	out, err := executeAgent(cfg, nil, "agent", "schema")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var got docent.Command
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v\nraw: %s", err, out)
	}

	if got.Name != "app" {
		t.Errorf("root Name = %q, want %q", got.Name, "app")
	}

	if got.Path != "app" {
		t.Errorf("root Path = %q, want %q", got.Path, "app")
	}
}

// TestAgentSchema_contractVersion pins the Config.ContractVersion stamp: the
// emitted root — full tree or --path subtree — carries a "contract_version"
// extensions entry, a host-set entry of the same name is overwritten,
// cfg.Command itself is never mutated, and an empty version stamps nothing.
func TestAgentSchema_contractVersion(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()
	schema.Extensions = map[string]any{"contract_version": "stale", "kept": "yes"}
	cfg := docent.Config{Command: schema, ContractVersion: "2.1.0"}

	assertStamp := func(t *testing.T, out []byte, wantPath string) {
		t.Helper()

		var got docent.Command
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("unmarshal output: %v\nraw: %s", err, out)
		}

		if got.Path != wantPath {
			t.Fatalf("root Path = %q, want %q", got.Path, wantPath)
		}

		if v := got.Extensions["contract_version"]; v != "2.1.0" {
			t.Errorf(`extensions contract_version = %v, want "2.1.0"`, v)
		}
	}

	t.Run("full tree stamps and overwrites host entry", func(t *testing.T) {
		t.Parallel()

		out, err := executeAgent(cfg, nil, "agent", "schema")
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		assertStamp(t, out, "app")

		var got docent.Command
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("unmarshal output: %v", err)
		}

		if v := got.Extensions["kept"]; v != "yes" {
			t.Errorf(`extensions kept = %v; host entries besides the stamp must survive`, v)
		}

		if v := cfg.Command.Extensions["contract_version"]; v != "stale" {
			t.Errorf("cfg.Command mutated: contract_version = %v, want untouched %q", v, "stale")
		}
	})

	t.Run("path subtree stamps the emitted node", func(t *testing.T) {
		t.Parallel()

		out, err := executeAgent(cfg, nil, "agent", "schema", "--path", "app create")
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		assertStamp(t, out, "app create")
	})

	t.Run("empty version stamps nothing", func(t *testing.T) {
		t.Parallel()

		bare := docent.Config{Command: schema}

		out, err := executeAgent(bare, nil, "agent", "schema", "--path", "app create")
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		var got docent.Command
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("unmarshal output: %v", err)
		}

		if _, present := got.Extensions["contract_version"]; present {
			t.Error("contract_version present with empty Config.ContractVersion")
		}
	})
}

// TestAgentSchema_withSchemaTransform pins the WithSchemaTransform contract:
// transforms receive the marshaled schema JSON and own the final output
// verbatim, multiple transforms compose in registration order, a transform
// error fails the command, and nil transforms are ignored.
func TestAgentSchema_withSchemaTransform(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()
	cfg := docent.Config{Command: schema}

	t.Run("envelope owns final shape and sees the stamp", func(t *testing.T) {
		t.Parallel()

		envelope := func(data []byte) ([]byte, error) {
			return []byte(`{"ok":true,"data":` + string(data) + "}\n"), nil
		}
		opts := []docentcobra.Option{docentcobra.WithSchemaTransform(envelope)}
		stamped := docent.Config{Command: schema, ContractVersion: "3.0.0"}

		out, err := executeAgent(stamped, opts, "agent", "schema")
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		var got struct {
			OK   bool           `json:"ok"`
			Data docent.Command `json:"data"`
		}

		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("unmarshal enveloped output: %v\nraw: %s", err, out)
		}

		if !got.OK || got.Data.Name != "app" {
			t.Errorf("envelope = ok:%v data.name:%q; want ok:true data.name:\"app\"", got.OK, got.Data.Name)
		}

		// The documented transform input includes the contract-version
		// stamp — the transform wraps stamped JSON, not the bare tree.
		if v := got.Data.Extensions["contract_version"]; v != "3.0.0" {
			t.Errorf(`enveloped contract_version = %v, want "3.0.0"`, v)
		}
	})

	t.Run("transforms compose in order", func(t *testing.T) {
		t.Parallel()

		opts := []docentcobra.Option{
			docentcobra.WithSchemaTransform(func([]byte) ([]byte, error) { return []byte("first"), nil }),
			docentcobra.WithSchemaTransform(nil),
			docentcobra.WithSchemaTransform(func(b []byte) ([]byte, error) { return append(b, "+second"...), nil }),
		}

		out, err := executeAgent(cfg, opts, "agent", "schema")
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		if string(out) != "first+second" {
			t.Errorf("output = %q, want %q", out, "first+second")
		}
	})

	t.Run("transform error fails the command", func(t *testing.T) {
		t.Parallel()

		boom := errors.New("boom")
		opts := []docentcobra.Option{
			docentcobra.WithSchemaTransform(func([]byte) ([]byte, error) { return nil, boom }),
		}

		_, err := executeAgent(cfg, opts, "agent", "schema")
		if !errors.Is(err, boom) {
			t.Errorf("error = %v; want errors.Is(err, boom)", err)
		}
	})
}

// TestAgentSchema_pathFlag_found verifies that --path returns only the targeted
// subtree and that the returned node matches the requested path exactly.
func TestAgentSchema_pathFlag_found(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()
	cfg := docent.Config{Command: schema}

	out, err := executeAgent(cfg, nil, "agent", "schema", "--path", "app create")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var got docent.Command
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v\nraw: %s", err, out)
	}

	if got.Name != "create" {
		t.Errorf("subtree Name = %q, want %q", got.Name, "create")
	}

	if got.Path != "app create" {
		t.Errorf("subtree Path = %q, want %q", got.Path, "app create")
	}

	// The subtree must not contain siblings — only the targeted node and its
	// own descendants.
	for _, child := range got.Children {
		if child.Name == "list" {
			t.Error("subtree contains sibling \"list\"; --path must return only the targeted subtree")
		}
	}
}

// TestAgentSchema_pathFlag_rootless verifies that --path resolves the
// rootless path form: an agent that writes the subcommand path it sees in
// help ("create") lands on the same node as the root-inclusive form
// ("app create") instead of a failed lookup.
func TestAgentSchema_pathFlag_rootless(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()
	cfg := docent.Config{Command: schema}

	out, err := executeAgent(cfg, nil, "agent", "schema", "--path", "create")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var got docent.Command
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v\nraw: %s", err, out)
	}

	if got.Path != "app create" {
		t.Errorf("subtree Path = %q, want %q", got.Path, "app create")
	}
}

// TestAgentSchema_pathFlag_found_root verifies that --path with the root
// command's path returns the full tree (the root is its own subtree).
func TestAgentSchema_pathFlag_found_root(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()
	cfg := docent.Config{Command: schema}

	out, err := executeAgent(cfg, nil, "agent", "schema", "--path", "app")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var got docent.Command
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v\nraw: %s", err, out)
	}

	if got.Name != "app" {
		t.Errorf("Name = %q, want %q", got.Name, "app")
	}
}

// TestAgentSchema_pathFlag_missing verifies that an unknown --path value causes
// Execute to return an error wrapping ErrCommandNotFound.
func TestAgentSchema_pathFlag_missing(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()
	cfg := docent.Config{Command: schema}

	_, err := executeAgent(cfg, nil, "agent", "schema", "--path", "app nonexistent")
	if err == nil {
		t.Fatal("Execute: expected error for unknown --path, got nil")
	}

	if !errors.Is(err, docent.ErrCommandNotFound) {
		t.Errorf("error = %v; want errors.Is(err, docent.ErrCommandNotFound) to be true", err)
	}
}

// TestAgentSchema_pathFlag_emptyTree verifies that --path on a zero-value Config
// (empty command tree) correctly returns ErrCommandNotFound for any non-empty path.
func TestAgentSchema_pathFlag_emptyTree(t *testing.T) {
	t.Parallel()

	cfg := docent.Config{} // zero-value: Command is zero Command (Path == "")

	_, err := executeAgent(cfg, nil, "agent", "schema", "--path", "anything")
	if err == nil {
		t.Fatal("Execute: expected error for path on empty tree, got nil")
	}

	if !errors.Is(err, docent.ErrCommandNotFound) {
		t.Errorf("error = %v; want ErrCommandNotFound", err)
	}
}

// TestAgentSchema_onlyTargetedSubtree verifies that --path returns a subtree
// that contains the target node and its own descendants, not its siblings.
func TestAgentSchema_onlyTargetedSubtree(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()
	cfg := docent.Config{Command: schema}

	// Ask for "app list" — a hidden leaf with no children.
	out, err := executeAgent(cfg, nil, "agent", "schema", "--path", "app list")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var got docent.Command
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v\nraw: %s", err, out)
	}

	if got.Name != "list" {
		t.Errorf("Name = %q, want %q", got.Name, "list")
	}

	if got.Path != "app list" {
		t.Errorf("Path = %q, want %q", got.Path, "app list")
	}

	// "list" is a leaf — no children and no sibling ("create") in output.
	if len(got.Children) != 0 {
		t.Errorf("Children = %v; want nil (leaf command)", got.Children)
	}

	// The sibling "create" must not appear at the top level of the output.
	var raw map[string]any
	if err = json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("re-unmarshal to map: %v", err)
	}

	if _, hasSiblingCreate := raw["create"]; hasSiblingCreate {
		t.Error("output contains sibling field \"create\"; only the targeted subtree should be returned")
	}
}

// TestAgentSchema_configOut verifies that output goes to Config.Out instead
// of cmd.OutOrStdout() when the host supplies a writer.
func TestAgentSchema_configOut(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()

	var hostOut bytes.Buffer

	cfg := docent.Config{
		Command: schema,
		Out:     &hostOut,
	}

	// Use a buffer for cobra output; since Out is set, the command must
	// NOT write to cmd.OutOrStdout().
	var cobraOut bytes.Buffer

	hostRoot := &gocobra.Command{Use: "host", Short: "Test host."}
	hostRoot.SilenceErrors = true
	hostRoot.SilenceUsage = true
	hostRoot.SetOut(&cobraOut)
	hostRoot.AddCommand(docentcobra.NewCommand(cfg))
	hostRoot.SetArgs([]string{"agent", "schema"})

	if err := hostRoot.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if cobraOut.Len() != 0 {
		t.Errorf("cobra output writer was written to (%d bytes); expected Config.Out to be used instead", cobraOut.Len())
	}

	if hostOut.Len() == 0 {
		t.Fatal("Config.Out was never written to")
	}

	var got docent.Command
	if err := json.Unmarshal(hostOut.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal Config.Out output: %v\nraw: %s", err, hostOut.String())
	}

	if got.Name != "app" {
		t.Errorf("Config.Out output root Name = %q, want %q", got.Name, "app")
	}
}

// TestAgentSchema_determinism verifies that running "agent schema" twice
// produces byte-identical output.
func TestAgentSchema_determinism(t *testing.T) {
	t.Parallel()

	_, schema := buildSchemaHost()
	cfg := docent.Config{Command: schema}

	first, err := executeAgent(cfg, nil, "agent", "schema")
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	second, err := executeAgent(cfg, nil, "agent", "schema")
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("agent schema is not deterministic:\nfirst:  %s\nsecond: %s", first, second)
	}
}
