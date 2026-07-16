// glamour-host is a runnable demonstration of consumer-aware guide
// rendering: docent emits plain Markdown through Config.Out, and the host —
// not docent — decides who gets styling. Agents and pipes receive raw bytes;
// only an interactive human terminal gets glamour's styled form.
//
// Try it:
//
//	go run .  guide safe-mutation          # styled when stdout is a TTY
//	go run .  guide safe-mutation | cat    # raw Markdown (pipe)
//	CLAUDECODE=1 go run . guide safe-mutation  # raw Markdown (agent harness)
package main

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"

	"charm.land/glamour/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
)

//go:embed guides/*.md
var guidesFS embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "glamour-host:", err)
		os.Exit(1)
	}
}

func run() error {
	sub, err := fs.Sub(guidesFS, "guides")
	if err != nil {
		return err
	}

	guides, err := docent.LoadGuides(sub)
	if err != nil {
		return err
	}

	// The dispatch: agents get clean bytes even inside a PTY, pipes get
	// clean bytes, and only a human at a terminal gets styled output.
	interactive := term.IsTerminal(int(os.Stdout.Fd()))
	agentInvoked := os.Getenv("CLAUDECODE") != "" || os.Getenv("CODEX_THREAD_ID") != ""

	var buf bytes.Buffer

	out := io.Writer(os.Stdout) // agents and pipes: raw Markdown
	if interactive && !agentInvoked {
		out = &buf // humans: capture now, style after Execute
	}

	root := &cobra.Command{Use: "host", Short: "Example docent host."}
	cfg := docent.Config{
		ToolName: "host",
		Guides:   guides,
		Command:  docentcobra.Tree(root),
		Out:      out,
	}

	// Two doors, one guide set: the agent namespace and the human command.
	root.AddCommand(docentcobra.NewCommand(cfg))
	root.AddCommand(docentcobra.NewGuideCommand(cfg))

	if err := root.Execute(); err != nil {
		return err
	}

	if out != &buf {
		return nil
	}

	// glamour styles complete documents, so rendering happens once the
	// command has written everything — never inside a streaming writer.
	styled := buf.String() // fall back to the raw bytes if rendering fails
	if r, rerr := glamour.NewTermRenderer(glamour.WithEnvironmentConfig()); rerr == nil {
		if s, serr := r.Render(buf.String()); serr == nil {
			styled = s
		}
	}

	fmt.Print(styled)

	return nil
}
