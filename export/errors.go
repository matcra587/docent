package export

import "errors"

// Sentinel errors for use with errors.Is, so hosts branch on failure
// classes instead of matching message text.
var (
	// ErrInvalidContractVersion is the sentinel Index wraps when a contract
	// version would corrupt the line-oriented index shape: a value
	// containing a newline, carriage return, or colon, or one that is
	// entirely whitespace.
	ErrInvalidContractVersion = errors.New("docent: invalid contract version")

	// ErrPathEscape is the sentinel Write wraps when a Renderer's RelPath
	// lexically escapes the export directory. The check runs before
	// anything touches disk, so an escaping path never leaves partial
	// artifacts behind.
	ErrPathEscape = errors.New("docent: export path escapes the export directory")
)
