package docent

import (
	"errors"
	"fmt"
)

// Sentinel errors for use with errors.Is and errors.As.
// Validation errors are reported by wrapping one of these sentinels in a
// *ValidationError so callers can branch on the class without matching strings.
var (
	// ErrMissingFrontmatter is returned when a guide file has no YAML frontmatter block.
	ErrMissingFrontmatter = errors.New("docent: missing frontmatter")

	// ErrInvalidFrontmatter is returned when a guide file has a frontmatter
	// block that is not valid YAML.
	ErrInvalidFrontmatter = errors.New("docent: invalid frontmatter")

	// ErrMissingField is returned when a required frontmatter field is absent or empty.
	ErrMissingField = errors.New("docent: missing required field")

	// ErrSlugMismatch is returned when the frontmatter slug value does not match the filename (without extension).
	ErrSlugMismatch = errors.New("docent: slug does not match filename")

	// ErrInvalidSlug is returned when a slug does not satisfy the Agent Skills
	// name rules: 1-64 lowercase alphanumeric characters and hyphens, with no
	// leading, trailing, or consecutive hyphens. Slugs become exported skill
	// names and directory names, so the constraint is enforced at load time.
	ErrInvalidSlug = errors.New("docent: invalid slug")

	// ErrDescriptionTooLong is returned when a guide's description and
	// when_to_use together exceed the exported skill description budget
	// (MaxSkillDescription), which the Agent Skills spec caps at 1024
	// characters.
	ErrDescriptionTooLong = errors.New("docent: description budget exceeded")

	// ErrInvalidSections is returned when the guide body does not have the
	// six section headings required by StandardVersion, in the expected order.
	ErrInvalidSections = errors.New("docent: invalid sections")

	// ErrDuplicateOrder is returned when two or more guides share the same
	// sparse-integer order value; the canonical guide order is a contract,
	// so a collision fails the load like any other validation class.
	ErrDuplicateOrder = errors.New("docent: duplicate order value")

	// ErrAliasCollision is returned when a guide alias is empty, declared by
	// two guides, or shadows a guide slug — an ambiguous name cannot
	// resolve, so lookup behavior would depend on iteration order.
	ErrAliasCollision = errors.New("docent: alias collision")

	// ErrCompatibilityTooLong is returned when a guide's compatibility field
	// exceeds the exported skill budget (MaxSkillCompatibility), which the
	// Agent Skills spec caps at 500 characters.
	ErrCompatibilityTooLong = errors.New("docent: compatibility budget exceeded")

	// ErrMultilineField is returned when a frontmatter field the guide index
	// emits line-oriented — title, description, when_to_use, or a commands or
	// aliases element — contains a newline. A multiline value would corrupt
	// the index's key: value shape (or forge extra entries), so it fails the
	// load.
	ErrMultilineField = errors.New("docent: multiline frontmatter field")

	// ErrFormFeed is returned when a guide file contains a form-feed
	// character. The form feed is reserved as the guide-concatenation
	// separator (export.GuideSeparator, the adapter's "agent guide --all"),
	// so content containing one would make the concatenation split
	// ambiguously.
	ErrFormFeed = errors.New("docent: form feed in guide content")

	// ErrCommandNotFound is the sentinel adapters and docenttest wrap when a
	// path or reference names no command in the tree. FindByPath itself
	// signals absence with its comma-ok result and never returns this error.
	ErrCommandNotFound = errors.New("docent: command not found")
)

// ValidationError carries per-file context about a single validation failure.
// Unwrap returns the sentinel that classifies the failure, enabling errors.Is
// and errors.As to reach the sentinel from a joined or wrapped error.
type ValidationError struct {
	// File is the guide filename or identifier where the violation was found.
	File string

	// Message describes the specific violation in human-readable form.
	Message string

	// Err is the sentinel that classifies the failure class.
	Err error
}

// Error returns a human-readable description of the validation failure.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("docent: %s: %s", e.File, e.Message)
}

// Unwrap allows errors.Is and errors.As to traverse the error chain to the
// sentinel that classifies this failure.
func (e *ValidationError) Unwrap() error {
	return e.Err
}
