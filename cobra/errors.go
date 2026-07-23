package cobra

import "errors"

// Sentinel errors for use with errors.Is. Flag-usage mistakes (conflicting
// flags, an empty --dir) stay plain errors — they are corrected at the
// command line, not branched on by hosts.
var (
	// ErrNoGuides is the sentinel agent export wraps when Config.Guides is
	// nil: exporting has nothing to render and, unlike the guide command's
	// index view, no meaningful empty output to emit.
	ErrNoGuides = errors.New("docent: no guides configured")

	// ErrUnsupportedFormat is the sentinel agent export wraps when --format
	// names no registered renderer.
	ErrUnsupportedFormat = errors.New("docent: unsupported export format")

	// ErrUnsupportedHarness is the sentinel agent export wraps when
	// --harness names no harness in harness.Supported.
	ErrUnsupportedHarness = errors.New("docent: unsupported agent harness")

	// ErrGuideNotFound is the sentinel the guide command wraps when a slug
	// lookup finds no guide in the set. Core's GuideSet.Get signals absence
	// with its comma-ok result and never returns this error.
	ErrGuideNotFound = errors.New("docent: guide not found")

	// ErrSectionNotFound is the sentinel the guide command wraps when
	// --section names no section in the guide. The core package never
	// returns it.
	ErrSectionNotFound = errors.New("docent: section not found")
)
