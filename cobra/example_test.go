package cobra_test

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing/fstest"

	gocobra "github.com/spf13/cobra"

	"github.com/matcra587/docent"
	docentcobra "github.com/matcra587/docent/cobra"
)

// ExampleNewGuideCommand mounts the guide browser as a first-class human
// command and dispatches rendering by consumer: an agent harness or a pipe
// gets raw Markdown bytes, and only an interactive human terminal gets the
// host's renderer. docent emits Markdown through Config.Out and never grows
// a rendering dependency — the renderer belongs to the host.
func ExampleNewGuideCommand() {
	guides, err := docent.LoadGuides(fstest.MapFS{
		"hello.md": {Data: []byte(strings.Join([]string{
			"---",
			"slug: hello",
			"title: Hello Guide",
			"description: Greets from the guide set.",
			"when_to_use: When demonstrating the standalone mount.",
			"commands: [app hello]",
			"---",
			"",
			"## Decide",
			"",
			"## Run",
			"app hello",
			"",
			"## Save",
			"",
			"## Preconditions",
			"",
			"## Recover",
			"",
			"## Next",
		}, "\n"))},
	})
	if err != nil {
		fmt.Println("load guides:", err)

		return
	}

	// The dispatch: agents get clean bytes even inside a PTY, pipes get
	// clean bytes, and only a human at a terminal gets styled output.
	agentInvoked := os.Getenv("CLAUDECODE") != "" || os.Getenv("CODEX_THREAD_ID") != ""
	interactive := false // e.g. golang.org/x/term: term.IsTerminal(int(os.Stdout.Fd()))

	var out io.Writer = os.Stdout
	if !agentInvoked && interactive {
		// Wrap stdout with the host's Markdown renderer here — e.g. a
		// glamour-backed writer — without docent knowing it exists.
		out = os.Stdout
	}

	root := &gocobra.Command{Use: "app"}
	root.AddCommand(docentcobra.NewGuideCommand(docent.Config{Guides: guides, Out: out}))
	root.SetArgs([]string{"guide", "hello", "--section", "run"})

	if err := root.Execute(); err != nil {
		fmt.Println("execute:", err)
	}
	// Output: app hello
}
