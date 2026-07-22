// Package cobra is the docent cobra adapter. It is the only package in
// github.com/matcra587/docent that may import github.com/spf13/cobra —
// depguard enforces this boundary.
//
// Because the package name collides with github.com/spf13/cobra, consumers
// import it under an alias:
//
//	import (
//		"github.com/spf13/cobra"
//
//		"github.com/matcra587/docent"
//		docentcobra "github.com/matcra587/docent/cobra"
//		"github.com/matcra587/docent/harness"
//	)
//
// The two entry points are Tree, which walks a cobra.Command tree and
// returns the framework-neutral Command IR defined in the parent package,
// and NewCommand, which returns the mountable "agent" command group:
//
//	cfg := docent.Config{Guides: guides, Command: docentcobra.Tree(root)}
//	root.AddCommand(docentcobra.NewCommand(cfg))
//
// NewGuideCommand additionally mounts the same guide browser as a
// first-class human command (e.g. top-level "app guide").
//
// # Consumer-aware rendering
//
// docent emits plain Markdown through Config.Out and carries no rendering
// dependency; styling belongs to the host. Because Markdown renderers such
// as charm.land/glamour/v2 style complete documents, the seam is
// buffer-then-render, not a streaming writer: agents and pipes get the raw
// bytes, and only an interactive human terminal gets the styled form.
//
//	interactive := term.IsTerminal(int(os.Stdout.Fd())) // golang.org/x/term
//	_, agentInvoked := harness.DetectAgent(os.LookupEnv) // docent/harness
//
//	var buf bytes.Buffer
//	out := io.Writer(os.Stdout) // agents and pipes: raw Markdown
//	if interactive && !agentInvoked {
//		out = &buf // humans: capture now, style after Execute
//	}
//
//	root.AddCommand(docentcobra.NewGuideCommand(docent.Config{Guides: guides, Out: out}))
//	err := root.Execute()
//
//	if out == &buf {
//		styled := buf.String() // fall back to the raw bytes if rendering fails
//		// charm.land/glamour/v2; WithEnvironmentConfig respects GLAMOUR_STYLE
//		if r, rerr := glamour.NewTermRenderer(glamour.WithEnvironmentConfig()); rerr == nil {
//			if s, serr := r.Render(buf.String()); serr == nil {
//				styled = s
//			}
//		}
//		fmt.Print(styled)
//	}
//
// A runnable version of this dispatch lives in _examples/glamour-host, a
// nested module (in a directory all Go tooling ignores) so its rendering
// dependencies never enter docent's go.mod.
// Users without host styling can pipe the raw output through a Markdown
// pager instead: "app guide <slug> | glow -p -".
package cobra
