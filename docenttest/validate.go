package docenttest

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/matcra587/docent"
)

// Check loads guides from fsys, verifies that every command path listed in
// guide frontmatter exists in root, and returns one error per violation. Each
// returned error wraps the sentinel that classifies its class:
//
//   - [docent.ErrMissingFrontmatter] – guide file has no YAML frontmatter block
//   - [docent.ErrInvalidFrontmatter] – frontmatter block is not valid YAML
//   - [docent.ErrMissingField] – a required frontmatter field is absent
//   - [docent.ErrSlugMismatch] – frontmatter slug does not match the filename
//   - [docent.ErrInvalidSlug] – slug violates the Agent Skills name rules
//   - [docent.ErrDescriptionTooLong] – description and when_to_use exceed the skill description budget
//   - [docent.ErrCompatibilityTooLong] – compatibility exceeds the skill budget
//   - [docent.ErrInvalidSections] – wrong number or order of body sections
//   - [docent.ErrDuplicateOrder] – two guides share the same order value
//   - [docent.ErrAliasCollision] – an alias is empty, declared twice, or shadows a slug
//   - [docent.ErrCommandNotFound] – a guide lists a command not present in root
//
// When root is the zero Command (root.Name == ""), command-reference checking
// is skipped, and a guide with an explicitly empty commands list (a
// whole-tool guide) has nothing to check by construction. A nil return value
// means no violations were found.
func Check(fsys fs.FS, root docent.Command) []error {
	gs, err := docent.LoadGuides(fsys)
	if err != nil {
		// Load failed; command-reference checking cannot proceed.
		return flattenErrors(err)
	}

	// Skip command-reference checking when no root is provided.
	if root.Name == "" {
		return nil
	}

	var errs []error

	for _, g := range gs.Guides() {
		for _, cmd := range g.Commands {
			if _, ok := docent.FindByPath(root, cmd); !ok {
				errs = append(errs, &docent.ValidationError{
					File:    g.Slug + ".md",
					Message: fmt.Sprintf("command %q not found in command tree", cmd),
					Err:     docent.ErrCommandNotFound,
				})
			}
		}
	}

	return errs
}

// Validate loads guides from fsys and reports each violation as a distinct
// t.Errorf call. Each call is prefixed with the violation class so test output
// identifies the problem without inspecting message strings:
//
//   - "missing frontmatter: …" wraps [docent.ErrMissingFrontmatter]
//   - "invalid frontmatter: …" wraps [docent.ErrInvalidFrontmatter]
//   - "missing required field: …" wraps [docent.ErrMissingField]
//   - "slug mismatch: …" wraps [docent.ErrSlugMismatch]
//   - "invalid slug: …" wraps [docent.ErrInvalidSlug]
//   - "description budget: …" wraps [docent.ErrDescriptionTooLong]
//   - "compatibility budget: …" wraps [docent.ErrCompatibilityTooLong]
//   - "invalid sections: …" wraps [docent.ErrInvalidSections]
//   - "duplicate order: …" wraps [docent.ErrDuplicateOrder]
//   - "alias collision: …" wraps [docent.ErrAliasCollision]
//   - "unknown command reference: …" wraps [docent.ErrCommandNotFound]
//
// When root is the zero Command, command-reference checking is skipped.
func Validate(tb testing.TB, fsys fs.FS, root docent.Command) {
	tb.Helper()

	for _, err := range Check(fsys, root) {
		switch {
		case errors.Is(err, docent.ErrMissingFrontmatter):
			tb.Errorf("missing frontmatter: %v", err)
		case errors.Is(err, docent.ErrInvalidFrontmatter):
			tb.Errorf("invalid frontmatter: %v", err)
		case errors.Is(err, docent.ErrMissingField):
			tb.Errorf("missing required field: %v", err)
		case errors.Is(err, docent.ErrSlugMismatch):
			tb.Errorf("slug mismatch: %v", err)
		case errors.Is(err, docent.ErrInvalidSlug):
			tb.Errorf("invalid slug: %v", err)
		case errors.Is(err, docent.ErrDescriptionTooLong):
			tb.Errorf("description budget: %v", err)
		case errors.Is(err, docent.ErrCompatibilityTooLong):
			tb.Errorf("compatibility budget: %v", err)
		case errors.Is(err, docent.ErrInvalidSections):
			tb.Errorf("invalid sections: %v", err)
		case errors.Is(err, docent.ErrDuplicateOrder):
			tb.Errorf("duplicate order: %v", err)
		case errors.Is(err, docent.ErrAliasCollision):
			tb.Errorf("alias collision: %v", err)
		case errors.Is(err, docent.ErrCommandNotFound):
			tb.Errorf("unknown command reference: %v", err)
		default:
			tb.Errorf("guide validation: %v", err)
		}
	}
}

// flattenErrors returns all leaf errors from a possibly joined or wrapped error.
// A joined error (whose Unwrap returns []error) is expanded recursively; any
// other error is returned as a single-element slice.
func flattenErrors(err error) []error {
	if err == nil {
		return nil
	}

	type joinUnwrapper interface {
		Unwrap() []error
	}

	if je, ok := err.(joinUnwrapper); ok {
		var all []error
		for _, e := range je.Unwrap() {
			all = append(all, flattenErrors(e)...)
		}

		return all
	}

	return []error{err}
}
