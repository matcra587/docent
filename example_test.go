package docent_test

import (
	"fmt"
	"testing/fstest"

	"github.com/matcra587/docent"
)

// exampleGuide is a minimal valid guide file used by the examples.
const exampleGuide = `---
slug: safe-mutation
title: Preview every write before sending it
description: Validate a mutation with --dry-run, then submit.
when_to_use: Before any create/edit/delete against a live instance.
commands: [item create]
---

## Decide

Which command to use.

## Run

` + "```sh\nitem create --dry-run\n```" + `

## Save

The returned ID.

## Preconditions

Valid auth.

## Recover

Re-authenticate on 401.

## Next

See core-contract.
`

// ExampleLoadGuides loads a guide set from an in-memory filesystem. Real
// hosts pass a go:embed FS instead.
func ExampleLoadGuides() {
	fsys := fstest.MapFS{
		"safe-mutation.md": &fstest.MapFile{Data: []byte(exampleGuide)},
	}

	gs, err := docent.LoadGuides(fsys)
	if err != nil {
		fmt.Println("load:", err)

		return
	}

	for _, g := range gs.Guides() {
		fmt.Printf("%s: %s\n", g.Slug, g.Title)
	}

	// Output:
	// safe-mutation: Preview every write before sending it
}

// ExampleGuideSet_Get looks up one guide by slug with the comma-ok idiom.
func ExampleGuideSet_Get() {
	fsys := fstest.MapFS{
		"safe-mutation.md": &fstest.MapFile{Data: []byte(exampleGuide)},
	}

	gs, _ := docent.LoadGuides(fsys)

	g, ok := gs.Get("safe-mutation")
	fmt.Println(ok, g.Sections[0].Heading)

	_, ok = gs.Get("missing")
	fmt.Println(ok)

	// Output:
	// true Decide
	// false
}
