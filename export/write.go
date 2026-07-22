package export

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/matcra587/docent"
)

// Write renders every guide through r and writes one artifact per guide
// under dir, creating the directory if absent. It returns the written
// artifact paths relative to dir, in guide order and forward-slash form.
//
// Every relative path is validated — local (else ErrPathEscape) and unique
// — before anything is written, so a path failing validation never leaves
// partial artifacts behind. The writes themselves go through an os.Root,
// which refuses any path component that resolves outside dir — a planted
// symlink, a ".." segment, an absolute path — at open time instead of
// following it; dir itself is trusted, containment applies beneath it. When
// a write fails after some artifacts have landed (permissions, disk, an
// os.Root refusal), the returned slice holds the paths already written,
// alongside the error.
func Write(dir string, r Renderer, guides []docent.Guide) ([]string, error) {
	// Each guide's RelPath is resolved exactly once: the slash form feeds
	// the returned report and error messages, the OS form feeds the writes,
	// and no later call can diverge from what was validated here.
	slashRels := make([]string, len(guides))
	rels := make([]string, len(guides))
	seen := make(map[string]struct{}, len(guides))

	for i, g := range guides {
		slashRels[i] = r.RelPath(g)

		rels[i] = filepath.FromSlash(slashRels[i])
		if !filepath.IsLocal(rels[i]) {
			return nil, fmt.Errorf("%w: %q", ErrPathEscape, slashRels[i])
		}

		// Two guides mapping to one path would silently last-write-win;
		// unreachable with the slug-derived built-in renderers, but Write
		// accepts arbitrary Renderers now.
		if _, dup := seen[slashRels[i]]; dup {
			return nil, fmt.Errorf("docent: duplicate export path %q", slashRels[i])
		}

		seen[slashRels[i]] = struct{}{}
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("docent: create export directory: %w", err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("docent: open export directory: %w", err)
	}

	// Close releases the directory handle only; every write is already
	// flushed and closed by WriteFile, so its error carries nothing.
	defer func() { _ = root.Close() }()

	for i, g := range guides {
		if err := root.MkdirAll(filepath.Dir(rels[i]), 0o750); err != nil {
			return slashRels[:i], fmt.Errorf("docent: create export subdirectory for %s: %w", slashRels[i], err)
		}

		if err := root.WriteFile(rels[i], []byte(r.Render(g)), 0o600); err != nil {
			return slashRels[:i], fmt.Errorf("docent: write %s: %w", slashRels[i], err)
		}
	}

	return slashRels, nil
}
