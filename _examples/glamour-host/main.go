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
	"strings"

	"charm.land/glamour/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
	"github.com/matcra587/docent/harness"
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
	// harness.DetectAgent covers the known runtimes and the AI_AGENT
	// override — no per-host env-var lists to maintain.
	//
	// Styling is scoped to the human guide door alone: the agent namespace
	// ("host agent ...") emits machine-shaped output — JSON schema,
	// artifact paths — that a Markdown renderer would mangle, so it stays
	// raw even for an interactive human.
	interactive := term.IsTerminal(int(os.Stdout.Fd()))
	_, agentInvoked := harness.DetectAgent(os.LookupEnv)
	humanGuide := len(os.Args) > 1 && os.Args[1] == "guide"

	var buf bytes.Buffer

	out := io.Writer(os.Stdout) // agents, pipes, agent namespace: raw
	if interactive && !agentInvoked && humanGuide {
		out = &buf // human guide view: capture now, style after Execute
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

	md := buf.String()

	// The no-slug index is frontmatter-style key:value lines by design —
	// agents parse that shape natively, and the index is first an agent
	// surface. It is not Markdown prose, so feeding it to glamour as-is
	// would collapse it into run-on paragraphs. Presentation is the
	// host's choice: this host reformats the index into real Markdown
	// before styling, so humans get a scannable listing from the same
	// bytes agents parse.
	if strings.HasPrefix(md, "# Agent Guide Index") {
		md = indexToMarkdown(md)
	}

	// glamour styles complete documents, so rendering happens once the
	// command has written everything — never inside a streaming writer.
	// Wrapping follows the live terminal width instead of glamour's
	// 80-column default.
	styled := md // fall back to the raw bytes if rendering fails

	opts := []glamour.TermRendererOption{glamour.WithEnvironmentConfig()}
	if w, _, sizeErr := term.GetSize(int(os.Stdout.Fd())); sizeErr == nil && w > 0 {
		opts = append(opts, glamour.WithWordWrap(w))
	}

	if r, rerr := glamour.NewTermRenderer(opts...); rerr == nil {
		if s, serr := r.Render(md); serr == nil {
			styled = s
		}
	}

	_, err = fmt.Print(styled)

	return err
}

// indexToMarkdown reformats docent's frontmatter-style guide index into
// Markdown for human presentation: one section per guide headed by its
// title, the slug as its invocation name, prose fields as text, command
// references as code. Each guide block opens with its slug line, so the
// slug is buffered until the title arrives to head the section. Unknown
// keys render generically, so an index with fields this host predates
// still shows every value.
func indexToMarkdown(index string) string {
	var b strings.Builder

	slug := "" // buffered: slug precedes title, but the title heads the block

	for line := range strings.Lines(index) {
		line = strings.TrimSuffix(line, "\n")
		key, value, isKV := strings.Cut(line, ": ")

		switch {
		case strings.HasPrefix(line, "# Agent Guide Index"), line == "", !isKV:
			b.WriteString(line + "\n")
		case key == "slug":
			slug = value
		case key == "title":
			b.WriteString("## " + value + "\n\n`guide " + slug + "`\n\n")
		case key == "description":
			b.WriteString(value + "\n\n")
		case key == "when_to_use":
			b.WriteString("**When:** " + value + "\n\n")
		case key == "commands":
			// A cross-cutting guide declares no commands; skip the line
			// rather than render empty code.
			if value != "" {
				b.WriteString("**Commands:** `" + value + "`\n")
			}
		default:
			// aliases, order, contract_version, future fields.
			b.WriteString("**" + strings.ReplaceAll(key, "_", " ") + ":** " + value + "\n")
		}
	}

	return b.String()
}
