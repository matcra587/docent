package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrUnsupportedScope is the sentinel ValidateScope and SkillsPath wrap when
// a scope names neither ScopeProject nor ScopeUser, so hosts can branch with
// errors.Is instead of matching message text.
var ErrUnsupportedScope = errors.New("docent: unsupported scope")

// The two skill-installation scopes every supported harness distinguishes:
// skills committed with the project versus skills installed for the user
// account. Plain strings — they name a convention, carry no invariant, and
// flow straight into flag values.
const (
	// ScopeProject is the repo-local skills directory, resolved against the
	// current working directory.
	ScopeProject = "project"

	// ScopeUser is the per-user skills directory, resolved against the
	// user's home directory.
	ScopeUser = "user"
)

// Harness identifies a detected agent runtime and its export conventions.
// All fields are plain data; Detect and Supported return copies, so mutating
// one affects nothing.
type Harness struct {
	// Name is the runtime's identifier as reported to the user.
	Name string

	// DefaultFormat is the export format best suited to this runtime, used
	// when --scope is given without an explicit --format.
	DefaultFormat string

	// SkillsDir is the skills directory in forward-slash form, relative to
	// the scope's root: the working directory for ScopeProject, the user's
	// home directory for ScopeUser. The relative path is scope-independent
	// for every supported harness; only the root differs. SkillsPath
	// resolves it for a concrete scope.
	SkillsDir string
}

// ValidateScope reports whether scope names a known skill-installation
// scope, returning an error wrapping ErrUnsupportedScope otherwise. It is
// the single spelling of scope validation: SkillsPath calls it before
// resolving, and adapters call it to pre-validate flag input before any
// harness detection.
func ValidateScope(scope string) error {
	switch scope {
	case ScopeProject, ScopeUser:
		return nil
	default:
		return fmt.Errorf("%w %q; supported scopes: %s, %s",
			ErrUnsupportedScope, scope, ScopeProject, ScopeUser)
	}
}

// SkillsPath resolves the harness's skills directory for a scope, in OS
// path form: ScopeProject returns SkillsDir relative to the working
// directory, ScopeUser joins it under the user's home directory. The
// scope-to-root mapping lives here — one authority for what a scope means —
// so adapters and hosts cannot disagree on where a harness's skills
// install. An unknown scope errors with ErrUnsupportedScope; a home
// directory that cannot be resolved is also an error.
func (h Harness) SkillsPath(scope string) (string, error) {
	if err := ValidateScope(scope); err != nil {
		return "", err
	}

	dir := filepath.FromSlash(h.SkillsDir)

	if scope == ScopeUser {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("docent: resolve home directory for scope %q: %w", scope, err)
		}

		return filepath.Join(home, dir), nil
	}

	return dir, nil
}

// Supported lists every harness Detect can identify — the runtimes with
// known skills-directory conventions — in detection precedence order.
// Callers use it to enumerate options in error messages and documentation.
// DetectAgent recognizes a wider set of runtimes; see Agents.
func Supported() []Harness {
	return []Harness{
		{
			Name:          "claude-code",
			DefaultFormat: "claude-skill",
			SkillsDir:     ".claude/skills",
		},
		{
			Name:          "codex",
			DefaultFormat: "agent-skill",
			SkillsDir:     ".agents/skills",
		},
	}
}

// agentMarker pairs a detectable agent name with the environment variables
// that identify it.
type agentMarker struct {
	name    string
	envVars []string
}

// agentMarkers lists every runtime DetectAgent can identify, in precedence
// order: when markers for several agents are present (nested sessions, dirty
// environments), the earliest entry wins. Harnesses with export conventions
// come first so DetectAgent and Detect agree on precedence. A function
// rather than a package-level var: the repo bans mutable package-level
// state, and a returned slice keeps callers from mutating the shared
// definition.
func agentMarkers() []agentMarker {
	return []agentMarker{
		{name: "claude-code", envVars: []string{"CLAUDECODE", "CLAUDE_CODE"}},
		{name: "codex", envVars: []string{"CODEX_SANDBOX", "CODEX_CI", "CODEX_THREAD_ID", "CODEX", "OPENAI_CODEX"}},
		{name: "gemini-cli", envVars: []string{"GEMINI_CLI", "GEMINI_CODE_ASSIST"}},
		{name: "copilot-cli", envVars: []string{"COPILOT_CLI", "COPILOT", "GITHUB_COPILOT"}},
		{name: "cursor", envVars: []string{"CURSOR_TERMINAL", "CURSOR_AGENT"}},
		{name: "opencode", envVars: []string{"OPENCODE"}},
		{name: "aider", envVars: []string{"AIDER"}},
		{name: "cline", envVars: []string{"CLINE"}},
		{name: "windsurf", envVars: []string{"WINDSURF", "WINDSURF_AGENT"}},
		{name: "amazon-q", envVars: []string{"AMAZON_Q", "AWS_Q_DEVELOPER"}},
		{name: "codeium", envVars: []string{"CODEIUM"}},
		{name: "cody", envVars: []string{"SRC_CODY"}},
	}
}

// Agents lists the names DetectAgent can return from environment markers,
// in detection precedence order. AI_AGENT and AGENT=amp overrides can yield
// names beyond this list.
func Agents() []string {
	ms := agentMarkers()

	names := make([]string, 0, len(ms)+1)
	for _, m := range ms {
		names = append(names, m.name)
	}

	// amp identifies through the generic AGENT variable rather than a
	// marker of its own, so it rides the override path but is still a
	// known, documented name.
	return append(names, "amp")
}

// EnvVars lists every environment variable detection consults: the AI_AGENT
// and AGENT overrides, then each agent's markers in Agents order. Hosts and
// tests use it to scrub or pin the whole detection surface — clearing only
// some variables lets a marker leaked by the invoking runtime satisfy
// detection. The returned slice is a fresh copy.
func EnvVars() []string {
	ms := agentMarkers()

	vars := []string{"AI_AGENT", "AGENT"}
	for _, m := range ms {
		vars = append(vars, m.envVars...)
	}

	return vars
}

// validAgentName constrains AI_AGENT override values to a plain identifier
// so an arbitrary environment string cannot ride into host output unvetted.
var validAgentName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// DetectAgent identifies the AI agent runtime driving this process from the
// environment via lookup (typically os.LookupEnv; injected for testability).
//
// The AI_AGENT variable, when set to a plain identifier, overrides all
// markers — the escape hatch for wrappers and runtimes without one; AGENT
// set to "amp" identifies Amp the same way. Otherwise the first entry in
// Agents order with a truthy marker wins. Marker values "0", "false", "no",
// and "off" (any case) count as unset, so an explicit CLAUDECODE=0 reads as
// "not this agent" rather than "agent present".
//
// The returned name is the detected runtime's canonical identifier: a
// version/role qualifier some runtimes append to their override — Claude
// Code sets AI_AGENT to values like "claude-code_2-1-211_agent" — is
// stripped once here, so every consumer of the name sees the same spelling
// (see canonicalAgentName for the exact rule). ok is false when no agent is
// detected. Hosts branch on the boolean for output-mode resolution and may
// surface the name in diagnostics.
func DetectAgent(lookup func(string) (string, bool)) (string, bool) {
	if v, ok := lookup("AI_AGENT"); ok && validAgentName.MatchString(v) {
		return canonicalAgentName(v), true
	}

	if v, _ := lookup("AGENT"); v == "amp" {
		return "amp", true
	}

	for _, m := range agentMarkers() {
		for _, name := range m.envVars {
			if v, ok := lookup(name); ok && truthy(v) {
				return m.name, true
			}
		}
	}

	return "", false
}

// canonicalAgentName strips the version/role qualifier some runtimes
// underscore-append to their identifier ("claude-code_2-1-211_agent"): the
// name is truncated before the first underscore-separated segment that is
// version-shaped — digits, dots, and hyphens only, starting with a digit.
// Names without such a segment ("my_agent", "my_2nd_agent") pass through
// unchanged, and truncation never yields an empty name — a name opening
// with the qualifier ("_1") passes through rather than vanishing.
func canonicalAgentName(name string) string {
	segs := strings.Split(name, "_")
	for i := 1; i < len(segs); i++ {
		if !isVersionSegment(segs[i]) {
			continue
		}

		if prefix := strings.Join(segs[:i], "_"); prefix != "" {
			return prefix
		}

		return name
	}

	return name
}

// isVersionSegment reports whether s looks like a version qualifier:
// non-empty, digits, dots, and hyphens only, starting with a digit.
func isVersionSegment(s string) bool {
	if s == "" || s[0] < '0' || s[0] > '9' {
		return false
	}

	for i := range len(s) {
		if c := s[i]; (c < '0' || c > '9') && c != '.' && c != '-' {
			return false
		}
	}

	return true
}

// Detect identifies the invoking agent runtime's export conventions from the
// environment via lookup (typically os.LookupEnv; injected for testability).
// It resolves the agent with DetectAgent — AI_AGENT override included — and
// returns the matching Supported harness, or false when no agent is detected
// or the detected agent has no known skills-directory conventions.
//
// A detected name matches a harness exactly or as a separated prefix.
// DetectAgent already canonicalizes underscore-joined version qualifiers,
// so the prefix match is defense in depth for qualifier shapes that rule
// does not cover (e.g. hyphen-joined suffixes).
func Detect(lookup func(string) (string, bool)) (Harness, bool) {
	name, ok := DetectAgent(lookup)
	if !ok {
		return Harness{}, false
	}

	for _, h := range Supported() {
		if name == h.Name {
			return h, true
		}

		if rest, found := strings.CutPrefix(name, h.Name); found && (rest[0] == '_' || rest[0] == '-') {
			return h, true
		}
	}

	return Harness{}, false
}

// truthy reports whether a marker value counts as set: non-blank and not an
// explicit negative ("0", "false", "no", "off", any case).
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
