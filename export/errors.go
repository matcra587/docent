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

	// ErrInvalidSkillName is the sentinel built-in skill renderers wrap when
	// a qualifier and guide slug do not compose to an Agent Skills name: 1-64
	// lowercase alphanumeric characters and hyphens, with no leading,
	// trailing, or consecutive hyphens. Write validates names before touching
	// the export directory.
	ErrInvalidSkillName = errors.New("docent: invalid exported skill name")

	// ErrPathEscape is the sentinel Write wraps when a Renderer's RelPath
	// lexically escapes the export directory. The check runs before
	// anything touches disk, so an escaping path never leaves partial
	// artifacts behind.
	ErrPathEscape = errors.New("docent: export path escapes the export directory")
)
