package cobra

import (
	"errors"
	"fmt"
	"os"
	"strings"

	gocobra "github.com/spf13/cobra"

	"github.com/matcra587/docent"
	"github.com/matcra587/docent/export"
	"github.com/matcra587/docent/harness"
)

// Option configures the behavior of NewCommand. Use the With* constructors
// to build options; the zero-argument call NewCommand(cfg) is always valid.
type Option func(*cmdOptions)

// cmdOptions holds adapter-specific settings collected from Option values.
type cmdOptions struct {
	extraCmds        []*gocobra.Command
	extraFormats     []exportFormat
	schemaTransforms []SchemaTransform
}

// SchemaTransform rewrites the agent schema command's output bytes before
// they are written. The input is the marshaled schema JSON (contract-version
// stamp included, no trailing newline); the returned bytes are emitted
// verbatim, so a transform owns the final shape — envelope, compaction,
// trailing newline. Returning an error fails the command with that error.
type SchemaTransform func([]byte) ([]byte, error)

// WithExtraCommands adds host-supplied subcommands under the agent command
// group. Extra commands are mounted alongside the built-in guide, schema, and
// export subcommands. Hosts use this to expose domain-specific agent utilities
// — for example a Jira host might mount "adf-matrix" and "fieldtypes" commands
// that let agents discover ADF schema and issue field types without leaving the
// agent namespace. Passing zero commands is valid and has no effect; nil
// commands are ignored rather than mounted.
func WithExtraCommands(cmds ...*gocobra.Command) Option {
	return func(o *cmdOptions) {
		o.extraCmds = append(o.extraCmds, cmds...)
	}
}

// WithExtraFormat registers a host-supplied export format under the given
// --format name. The renderer's artifacts are written through the same
// validated, root-scoped path handling as the built-in formats, and the name
// surfaces everywhere formats do: --format parsing, help text, error
// messages, and shell completion. Built-in names cannot be shadowed and the
// first registration of a name wins; an empty name or nil renderer is
// ignored rather than mounted, matching WithExtraCommands.
func WithExtraFormat(name string, r export.Renderer) Option {
	return func(o *cmdOptions) {
		if name == "" || r == nil {
			return
		}

		o.extraFormats = append(o.extraFormats, exportFormat{name: name, renderer: r})
	}
}

// WithSchemaTransform registers a rewrite of the agent schema command's
// output — the seam for hosts that wrap schema emission in their own
// envelope (status, metadata, schema-version fields) or re-encode it. It
// applies to the schema command only; the guide and export surfaces emit
// their documented shapes unchanged. Without a transform the schema JSON is
// written as-is with a trailing newline. Multiple transforms compose in
// registration order, each receiving the previous one's output; nil
// transforms are ignored rather than mounted, matching WithExtraCommands.
func WithSchemaTransform(t SchemaTransform) Option {
	return func(o *cmdOptions) {
		if t == nil {
			return
		}

		o.schemaTransforms = append(o.schemaTransforms, t)
	}
}

// NewCommand returns the mountable "agent" command group configured for the
// given host integration surface. The host adds it to their cobra root,
// importing this package under the conventional docentcobra alias (see the
// package documentation):
//
//	import docentcobra "github.com/matcra587/docent/cobra"
//
//	root.AddCommand(docentcobra.NewCommand(cfg))
//
// Optional behavior — such as mounting host-supplied extra subcommands — is
// controlled via Option values:
//
//	root.AddCommand(docentcobra.NewCommand(cfg, docentcobra.WithExtraCommands(adfMatrix, fieldTypes)))
//
// The agent command group provides agent-facing subcommands for guide
// retrieval, schema introspection, and skill export. It never modifies the
// host command tree.
//
// Cobra runs the host's persistent pre-run hooks around these subcommands,
// and docent cannot prevent that: a root hook that resolves credentials
// kills the discovery surface on machines with none. Exempt the agent
// group (and any NewGuideCommand mount) from credential resolution — the
// standard requires the agent surface to work unauthenticated.
func NewCommand(cfg docent.Config, opts ...Option) *gocobra.Command {
	o := &cmdOptions{}

	// Hosts building options conditionally can end up with nil entries; a
	// library must not take the host down over one, so nils are no-ops.
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	agent := &gocobra.Command{
		Use:   "agent",
		Short: "Commands for AI agent integration.",
	}

	agent.AddCommand(agentSchemaCmd(cfg, o.schemaTransforms))
	agent.AddCommand(agentGuideCmd(cfg))
	agent.AddCommand(agentExportCmd(cfg, o.extraFormats))

	// Same defense as nil options: cobra's AddCommand nil-derefs on a nil
	// child, which would panic the host at wiring time.
	for _, cmd := range o.extraCmds {
		if cmd != nil {
			agent.AddCommand(cmd)
		}
	}

	return agent
}

// NewGuideCommand returns a standalone guide browser command — index, single
// guide by slug, --section, --all, and slug completion, byte-identical to the
// "agent guide" subcommand NewCommand mounts. Cobra forbids mounting one
// *Command under two parents, so hosts that also want the guides as a
// first-class human command mount a second instance wherever it fits:
//
//	root.AddCommand(docentcobra.NewGuideCommand(cfg)) // "app guide <slug>"
//
// Both surfaces serve the same guide set in the same canonical order, so the
// human and agent views cannot drift apart. Rendering stays the host's job
// through Config.Out — docent emits Markdown and never grows a rendering
// dependency; see the example for consumer-aware dispatch (agents and pipes
// get raw bytes, an interactive terminal gets the host's renderer).
func NewGuideCommand(cfg docent.Config) *gocobra.Command {
	cmd := agentGuideCmd(cfg)

	// The shared constructor's help shows the agent-namespace invocation;
	// a standalone mount shows its own path. Behavior is untouched — the
	// two-doors contract covers command output, not help text.
	cmd.Example = `  # Frontmatter-only index of every guide
  app guide

  # One guide in full, or a single section of it
  app guide safe-mutation
  app guide safe-mutation --section run

  # Every guide, form-feed separated
  app guide --all`

	return cmd
}

// writeText emits output to cfg.Out when the host supplied one, falling back
// to cmd.OutOrStdout(). It is the single exit point for all text output in
// this adapter.
func writeText(cmd *gocobra.Command, cfg docent.Config, text string) error {
	w := cmd.OutOrStdout()
	if cfg.Out != nil {
		w = cfg.Out
	}

	_, err := fmt.Fprint(w, text)

	return err
}

// agentGuideCmd creates the "agent guide" subcommand. It serves:
//   - No args, no flags    → frontmatter-only list index in canonical order
//   - <slug>               → full guide content (raw bytes) for the named guide
//   - <slug> --section <h> → single section body from the named guide
//   - --all                → all guides concatenated in canonical order
//
// Invalid combinations (--section without a slug, --all with a slug) are
// errors rather than silently ignored flags: agents must be able to trust
// that every flag they pass had an effect.
func agentGuideCmd(cfg docent.Config) *gocobra.Command {
	var (
		section string
		all     bool
	)

	cmd := &gocobra.Command{
		Use:   "guide [slug]",
		Short: "Retrieve agent guides.",
		Long: `Retrieve agent guides from the loaded guide set.

Without arguments, emits a frontmatter-only index of all guides in canonical
order. With a slug argument, emits the full guide content. With --section the
named section body is emitted instead of the full guide. With --all all guides
are concatenated in canonical order, separated by form-feed lines.`,
		Example: `  # Frontmatter-only index of every guide
  app agent guide

  # One guide in full, or a single section of it
  app agent guide safe-mutation
  app agent guide safe-mutation --section run

  # Every guide, form-feed separated
  app agent guide --all`,
		Args: gocobra.MaximumNArgs(1),
		ValidArgsFunction: func(_ *gocobra.Command, args []string, _ string) ([]string, gocobra.ShellCompDirective) {
			if len(args) > 0 || cfg.Guides == nil {
				return nil, gocobra.ShellCompDirectiveNoFileComp
			}

			var slugs []string
			for g := range cfg.Guides.All() {
				slugs = append(slugs, g.Slug)
			}

			return slugs, gocobra.ShellCompDirectiveNoFileComp
		},
		// A runtime error (unknown slug, unknown section) must not dump the
		// usage wall: a mounted library cannot rely on the host root setting
		// SilenceUsage, and agents need the error line, not flag help.
		SilenceUsage: true,
		RunE: func(cmd *gocobra.Command, args []string) error {
			switch {
			case section != "" && len(args) == 0:
				return errors.New("docent: --section requires a slug argument")
			case all && len(args) == 1:
				return errors.New("docent: --all cannot be combined with a slug argument")
			}

			gs := cfg.Guides
			if gs == nil {
				return writeText(cmd, cfg, "(no guides configured)\n")
			}

			switch {
			case all:
				return runGuideAll(cmd, cfg, gs)
			case len(args) == 1:
				return runGuideSingle(cmd, cfg, gs, args[0], section)
			default:
				return runGuideList(cmd, cfg, gs)
			}
		},
	}

	cmd.Flags().StringVar(&section, "section", "", "Emit only the named section body (requires a slug argument).")
	cmd.Flags().BoolVar(&all, "all", false, "Emit all guides concatenated in canonical order.")

	cmd.MarkFlagsMutuallyExclusive("section", "all")

	// Completion derives from the core accessor so a StandardVersion revision
	// cannot leave the shell offering stale headings.
	mustRegisterCompletion(cmd, "section", docent.SectionHeadings)

	return cmd
}

// mustRegisterCompletion wires shell completion for a flag from a values
// source. Registration only fails when the flag is missing, which cannot
// happen — every caller registers completion for a flag defined on cmd
// moments earlier.
func mustRegisterCompletion(cmd *gocobra.Command, flag string, values func() []string) {
	err := cmd.RegisterFlagCompletionFunc(flag,
		func(_ *gocobra.Command, _ []string, _ string) ([]string, gocobra.ShellCompDirective) {
			return values(), gocobra.ShellCompDirectiveNoFileComp
		})
	if err != nil {
		panic(fmt.Sprintf("docent: register --%s completion: %v", flag, err))
	}
}

// runGuideList emits the frontmatter-only index for all guides in canonical
// order — the token-economical index view for agents. The artifact shape is
// owned by export.Index; this adapter only plumbs flags and output.
func runGuideList(cmd *gocobra.Command, cfg docent.Config, gs *docent.GuideSet) error {
	index, err := export.Index(gs.Guides(), cfg.ContractVersion)
	if err != nil {
		return err
	}

	return writeText(cmd, cfg, index)
}

// runGuideSingle emits a single guide by slug. When section is non-empty only
// that section's body is emitted; an unknown slug or section name is an error.
// The full-guide view renders as a runbook — title plus sections, frontmatter
// omitted: the caller already read the index to pick the slug, so the routing
// metadata is noise when they are trying to act.
func runGuideSingle(cmd *gocobra.Command, cfg docent.Config, gs *docent.GuideSet, slug, section string) error {
	// Resolve, not Get: an agent holding a near-miss name (old separator,
	// wrong case, unambiguous fragment) should land on the guide instead of
	// a failed lookup.
	g, ok := gs.Resolve(slug)
	if !ok {
		return fmt.Errorf("%w: %q", ErrGuideNotFound, slug)
	}

	if section != "" {
		s, ok := g.Section(section)
		if !ok {
			return fmt.Errorf("%w: %q in guide %q", ErrSectionNotFound, section, slug)
		}

		return writeText(cmd, cfg, s.Body+"\n")
	}

	return writeText(cmd, cfg, export.Runbook{}.Render(g))
}

// runGuideAll concatenates all guide raw content in canonical order via
// export.Concat, separated by form-feed lines so agents can split the
// output unambiguously (guide content itself never contains a form feed).
func runGuideAll(cmd *gocobra.Command, cfg docent.Config, gs *docent.GuideSet) error {
	return writeText(cmd, cfg, export.Concat(gs.Guides()))
}

// agentSchemaCmd creates the "agent schema" subcommand. It emits the
// framework-neutral command schema as JSON. When --path is given the output is
// scoped to the subtree rooted at that path; an unknown path is an error.
func agentSchemaCmd(cfg docent.Config, transforms []SchemaTransform) *gocobra.Command {
	var path string

	cmd := &gocobra.Command{
		Use:   "schema",
		Short: "Emit the command schema as JSON.",
		Long: `Emit the host's framework-neutral command schema as JSON.

Without --path the full tree is emitted. With --path only the subtree rooted
at that command is emitted, using the space-separated path form (e.g. "issue create").`,
		Example: `  # The full command tree
  app agent schema

  # One command's subtree only
  app agent schema --path "issue create"`,
		Args: gocobra.NoArgs,
		// See agentGuideCmd: runtime errors return the error line alone.
		SilenceUsage: true,
		RunE: func(cmd *gocobra.Command, _ []string) error {
			tree := cfg.Command

			if path != "" {
				found, ok := docent.FindByPath(tree, path)
				if !ok {
					return fmt.Errorf("%w: %q", docent.ErrCommandNotFound, path)
				}

				tree = found
			}

			tree = stampContractVersion(tree, cfg.ContractVersion)

			data, err := docent.MarshalSchema(tree)
			if err != nil {
				return err
			}

			// Transforms own the final shape, trailing newline included, and
			// receive the bare JSON per the SchemaTransform contract; only
			// the untransformed default appends the artifact newline.
			if len(transforms) == 0 {
				return writeText(cmd, cfg, string(data)+"\n")
			}

			for _, t := range transforms {
				data, err = t(data)
				if err != nil {
					return fmt.Errorf("docent: transform schema output: %w", err)
				}
			}

			return writeText(cmd, cfg, string(data))
		},
	}

	cmd.Flags().StringVar(&path, "path", "", `Subset the schema to the named command subtree (e.g. "issue create").`)

	mustRegisterCompletion(cmd, "path", func() []string { return commandPaths(cfg.Command) })

	return cmd
}

// commandPaths returns every path in the tree in depth-first pre-order.
func commandPaths(cmd docent.Command) []string {
	if cmd.Name == "" {
		return nil
	}

	paths := []string{cmd.Path}
	for _, child := range cmd.Children {
		paths = append(paths, commandPaths(child)...)
	}

	return paths
}

// exportFormat pairs a --format value with its renderer. Every format shares
// the common Agent Skills definition (<slug>/SKILL.md, spec-conformant name
// and description); harness-specific formats layer that harness's frontmatter
// extensions on top.
type exportFormat struct {
	name     string
	renderer export.Renderer
}

// exportFormats is the single ordered registry every format surface derives
// from — renderer lookup, help text, error messages, and shell completion —
// so a format cannot be registered in one place and missing from another.
// Built-ins come first, then host formats from WithExtraFormat in
// registration order; a later entry whose name is already registered is
// dropped, so built-in names cannot be shadowed and the first registration
// of a name wins across every derived surface. What distinguishes the
// built-in formats is documented on the renderer types: export.AgentSkill
// emits the portable open-standard shape (description and when_to_use
// joined into the spec's single description field), and export.ClaudeSkill
// keeps them as separate keys via Claude Code's native when_to_use
// frontmatter extension; layout, name rules, and body are identical.
func exportFormats(extras []exportFormat) []exportFormat {
	formats := []exportFormat{
		{name: "agent-skill", renderer: export.AgentSkill{}},
		{name: "claude-skill", renderer: export.ClaudeSkill{}},
	}

	for _, e := range extras {
		if _, taken := lookupFormat(formats, e.name); !taken {
			formats = append(formats, e)
		}
	}

	return formats
}

// lookupFormat returns the renderer registered under a --format value; the
// comma-ok result distinguishes unknown formats.
func lookupFormat(formats []exportFormat, name string) (export.Renderer, bool) {
	for _, f := range formats {
		if f.name == name {
			return f.renderer, true
		}
	}

	return nil, false
}

// formatNames lists every --format value in registry order.
func formatNames(formats []exportFormat) []string {
	names := make([]string, len(formats))
	for i, f := range formats {
		names[i] = f.name
	}

	return names
}

// agentExportCmd creates the "agent export" subcommand. It renders every guide
// in the configured guide set to the requested format and writes one artifact
// file per guide under the resolved directory, then reports each written path
// (relative to that directory, one per line) through the text output hook.
//
// The destination resolves in two modes: an explicit --dir (which requires an
// explicit --format and behaves exactly as it always has), or --scope, which
// detects the invoking agent harness and derives both the directory and the
// default format from that harness's conventions. Every format writes
// <slug>/SKILL.md per guide: "agent-skill" is the portable Agent Skills open
// standard, "claude-skill" adds Claude Code's native when_to_use frontmatter.
func agentExportCmd(cfg docent.Config, extraFormats []exportFormat) *gocobra.Command {
	var (
		format string
		dir    string
		scope  string
	)

	registry := exportFormats(extraFormats)
	formats := strings.Join(formatNames(registry), ", ")

	cmd := &gocobra.Command{
		Use:   "export",
		Short: "Export guides to an agent-consumable format.",
		Long: `Export every guide in the guide set to the requested format.

One artifact file per guide is written in canonical guide order, and each
written path is reported relative to the target directory, one per line.
Every format writes a SKILL.md file per guide under a directory named after
the guide's slug: "agent-skill" is the portable Agent Skills open standard
(agentskills.io) understood by Claude Code, Codex, and other harnesses;
"claude-skill" is the Claude Code variant carrying its native when_to_use
frontmatter field.

The target directory is either an explicit --dir (which requires --format),
or --scope, which detects the invoking agent harness from its environment
markers and resolves that harness's skills directory: "project" is relative
to the working directory, "user" to the home directory. With --scope, the
format defaults to the detected harness's native format, --format overrides,
and the report opens with an informational line — prefixed "#" — naming the
detected harness and resolved directory; every other report line is a
written path.`,
		Example: `  # Write one open-standard SKILL.md per guide under ./skills
  app agent export --format agent-skill --dir ./skills

  # Let the invoking harness pick the directory and format
  app agent export --scope project

  # Install into the user-level skills directory of the invoking harness
  app agent export --scope user`,
		Args: gocobra.NoArgs,
		// See agentGuideCmd: runtime errors return the error line alone.
		SilenceUsage: true,
		RunE: func(cmd *gocobra.Command, _ []string) error {
			resolvedFormat, resolvedDir, header, err := resolveExportDestination(format, dir, scope)
			if err != nil {
				return err
			}

			r, ok := lookupFormat(registry, resolvedFormat)
			if !ok {
				return fmt.Errorf("%w: %q; supported formats: %s", ErrUnsupportedFormat, resolvedFormat, formats)
			}

			gs := cfg.Guides
			if gs == nil {
				return ErrNoGuides
			}

			return runExport(cmd, cfg, gs, r, resolvedDir, header)
		},
	}

	cmd.Flags().StringVar(&format, "format", "",
		`Export format. Supported formats: `+formats+`. Required with --dir; defaults to the detected harness's format with --scope.`)
	cmd.Flags().StringVar(&dir, "dir", "", `Directory to write artifacts into; created if absent.`)
	cmd.Flags().StringVar(&scope, "scope", "",
		`Resolve the directory from the detected agent harness: "project" (working-directory skills dir) or "user" (home skills dir).`)

	cmd.MarkFlagsOneRequired("dir", "scope")
	cmd.MarkFlagsMutuallyExclusive("dir", "scope")
	mustRegisterCompletion(cmd, "format", func() []string { return formatNames(registry) })
	mustRegisterCompletion(cmd, "scope", func() []string {
		return []string{harness.ScopeProject, harness.ScopeUser}
	})

	return cmd
}

// resolveExportDestination turns the export flag surface into a concrete
// format and target directory, plus an optional report header. Explicit --dir
// wins and requires an explicit --format (byte-identical behavior to the
// pre-scope command). --scope detects the invoking harness and derives the
// directory — and, when --format is empty, the format — from its conventions;
// the header names the detected harness and resolved directory so the report
// states where the artifacts went.
func resolveExportDestination(format, dir, scope string) (resolvedFormat, resolvedDir, header string, err error) {
	// Cobra's one-required check counts a flag explicitly set to "" as
	// provided; catch that here so the error names the real problem instead
	// of blaming a flag the caller never touched.
	if dir == "" && scope == "" {
		return "", "", "", errors.New("docent: one of --dir or --scope must be non-empty")
	}

	if dir != "" {
		if format == "" {
			return "", "", "", errors.New("docent: --format is required with --dir")
		}

		return format, dir, "", nil
	}

	// Scope is validated before harness detection so a typo reports as a
	// scope problem regardless of the environment; the single spelling of
	// the check lives in harness.ValidateScope.
	if err := harness.ValidateScope(scope); err != nil {
		return "", "", "", err
	}

	h, ok := harness.Detect(os.LookupEnv)
	if !ok {
		return "", "", "", fmt.Errorf(
			"docent: no agent harness detected (supported: %s); pass --dir to export to an explicit directory",
			strings.Join(supportedHarnessNames(), ", "))
	}

	if format == "" {
		format = h.DefaultFormat
	}

	// The scope-to-directory mapping is harness convention, owned by
	// harness.SkillsPath; the adapter only decides flag interaction.
	resolvedDir, err = h.SkillsPath(scope)
	if err != nil {
		return "", "", "", err
	}

	header = fmt.Sprintf("# detected %s; exporting to %s\n", h.Name, resolvedDir)

	return format, resolvedDir, header, nil
}

// stampContractVersion returns tree with the host contract version written
// into the emitted root's extensions under "contract_version", so agents can
// pin behavior to the contract they read. The stamp lands on whatever node
// the command emits — the full tree or a --path subtree — and is applied to
// a copy at emission time only: cfg.Command is never mutated, and an empty
// version returns tree unchanged. A host-set "contract_version" extensions
// entry is overwritten — Config.ContractVersion is the single authority, and
// two competing stamps would be worse than either alone.
func stampContractVersion(tree docent.Command, version string) docent.Command {
	if version == "" {
		return tree
	}

	// Clone detaches the emitted node — and its extensions map — from
	// cfg.Command before the stamp below.
	stamped := tree.Clone()

	if stamped.Extensions == nil {
		stamped.Extensions = map[string]any{}
	}

	stamped.Extensions["contract_version"] = version

	return stamped
}

// supportedHarnessNames lists the detectable harness names for error messages.
func supportedHarnessNames() []string {
	supported := harness.Supported()

	names := make([]string, len(supported))
	for i, h := range supported {
		names[i] = h.Name
	}

	return names
}

// runExport writes every guide's artifact under dir through export.Write —
// which owns path validation and the os.Root boundary — then reports each
// written path relative to dir, preceded by header when one is given,
// through the text output hook.
func runExport(cmd *gocobra.Command, cfg docent.Config, gs *docent.GuideSet, r export.Renderer, dir, header string) error {
	rels, err := export.Write(dir, r, gs.Guides())
	if err != nil {
		return err
	}

	var report strings.Builder

	report.WriteString(header)

	for _, rel := range rels {
		report.WriteString(rel)
		report.WriteString("\n")
	}

	return writeText(cmd, cfg, report.String())
}
