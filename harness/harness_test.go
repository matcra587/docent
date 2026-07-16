package harness_test

import (
	"testing"

	"github.com/matcra587/docent/harness"
)

// lookupFrom returns a lookup function backed by a fixed environment map,
// so detection is tested without touching the real process environment.
func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := env[name]

		return v, ok
	}
}

// TestDetect pins one case per supported harness marker plus the boundary
// cases: no markers, blank markers, and the claude-before-codex precedence.
func TestDetect(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		env        map[string]string
		wantName   string // "" means detection must fail
		wantFormat string
		wantDir    string
	}{
		{
			name:       "claude via CLAUDECODE",
			env:        map[string]string{"CLAUDECODE": "1"},
			wantName:   "claude-code",
			wantFormat: "claude-skill",
			wantDir:    ".claude/skills",
		},
		{
			name:       "claude via CLAUDE_CODE",
			env:        map[string]string{"CLAUDE_CODE": "1"},
			wantName:   "claude-code",
			wantFormat: "claude-skill",
			wantDir:    ".claude/skills",
		},
		{
			name:       "codex via CODEX_SANDBOX",
			env:        map[string]string{"CODEX_SANDBOX": "seatbelt"},
			wantName:   "codex",
			wantFormat: "agent-skill",
			wantDir:    ".agents/skills",
		},
		{
			name:       "codex via CODEX_CI",
			env:        map[string]string{"CODEX_CI": "true"},
			wantName:   "codex",
			wantFormat: "agent-skill",
			wantDir:    ".agents/skills",
		},
		{
			name:       "codex via CODEX_THREAD_ID",
			env:        map[string]string{"CODEX_THREAD_ID": "t-1"},
			wantName:   "codex",
			wantFormat: "agent-skill",
			wantDir:    ".agents/skills",
		},
		{
			name:       "codex via CODEX",
			env:        map[string]string{"CODEX": "1"},
			wantName:   "codex",
			wantFormat: "agent-skill",
			wantDir:    ".agents/skills",
		},
		{
			name:       "codex via OPENAI_CODEX",
			env:        map[string]string{"OPENAI_CODEX": "1"},
			wantName:   "codex",
			wantFormat: "agent-skill",
			wantDir:    ".agents/skills",
		},
		{
			name:     "no markers",
			env:      map[string]string{"PATH": "/usr/bin"},
			wantName: "",
		},
		{
			name:     "blank marker is not set",
			env:      map[string]string{"CLAUDECODE": "  "},
			wantName: "",
		},
		{
			name:       "claude wins over codex",
			env:        map[string]string{"CLAUDECODE": "1", "CODEX_THREAD_ID": "t-1"},
			wantName:   "claude-code",
			wantFormat: "claude-skill",
			wantDir:    ".claude/skills",
		},
		{
			name:     "explicit negative marker is not set",
			env:      map[string]string{"CLAUDECODE": "0"},
			wantName: "",
		},
		{
			name:       "AI_AGENT override with version suffix resolves harness",
			env:        map[string]string{"AI_AGENT": "claude-code_2-1-211_agent"},
			wantName:   "claude-code",
			wantFormat: "claude-skill",
			wantDir:    ".claude/skills",
		},
		{
			name:     "AI_AGENT override without conventions detects no harness",
			env:      map[string]string{"AI_AGENT": "aider"},
			wantName: "",
		},
		{
			name:     "prefix without separator does not match",
			env:      map[string]string{"AI_AGENT": "claude-codex"},
			wantName: "",
		},
		{
			name:       "hyphen-separated prefix resolves harness",
			env:        map[string]string{"AI_AGENT": "codex-v2"},
			wantName:   "codex",
			wantFormat: "agent-skill",
			wantDir:    ".agents/skills",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, ok := harness.Detect(lookupFrom(tc.env))

			if tc.wantName == "" {
				if ok {
					t.Fatalf("Detect = %+v, want no detection", h)
				}

				return
			}

			if !ok {
				t.Fatalf("Detect found nothing, want %s", tc.wantName)
			}

			if h.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", h.Name, tc.wantName)
			}

			if h.DefaultFormat != tc.wantFormat {
				t.Errorf("DefaultFormat = %q, want %q", h.DefaultFormat, tc.wantFormat)
			}

			if got := h.SkillsDir; got != tc.wantDir {
				t.Errorf("SkillsDir = %q, want %q", got, tc.wantDir)
			}
		})
	}
}

// TestDetectAgent pins the broad-detection ladder: the AI_AGENT and
// AGENT=amp overrides win over markers, one representative marker per
// runtime resolves its name, precedence follows Agents order, negative
// marker values read as unset, and a malformed override is ignored rather
// than passed through.
func TestDetectAgent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  map[string]string
		want string // "" means detection must fail
	}{
		{"AI_AGENT override wins over markers", map[string]string{"AI_AGENT": "my-wrapper", "CLAUDECODE": "1"}, "my-wrapper"},
		{"AI_AGENT with invalid characters ignored", map[string]string{"AI_AGENT": "not a name!", "CLAUDECODE": "1"}, "claude-code"},
		{"AGENT amp", map[string]string{"AGENT": "amp"}, "amp"},
		{"AGENT non-amp ignored", map[string]string{"AGENT": "vim"}, ""},
		{"gemini via GEMINI_CLI", map[string]string{"GEMINI_CLI": "1"}, "gemini-cli"},
		{"copilot via GITHUB_COPILOT", map[string]string{"GITHUB_COPILOT": "1"}, "copilot-cli"},
		{"cursor via CURSOR_AGENT", map[string]string{"CURSOR_AGENT": "1"}, "cursor"},
		{"opencode", map[string]string{"OPENCODE": "1"}, "opencode"},
		{"aider", map[string]string{"AIDER": "1"}, "aider"},
		{"cline", map[string]string{"CLINE": "1"}, "cline"},
		{"windsurf", map[string]string{"WINDSURF_AGENT": "1"}, "windsurf"},
		{"amazon-q via AWS_Q_DEVELOPER", map[string]string{"AWS_Q_DEVELOPER": "1"}, "amazon-q"},
		{"codeium", map[string]string{"CODEIUM": "1"}, "codeium"},
		{"cody via SRC_CODY", map[string]string{"SRC_CODY": "1"}, "cody"},
		{"claude precedes gemini", map[string]string{"CLAUDECODE": "1", "GEMINI_CLI": "1"}, "claude-code"},
		{"negative value reads unset", map[string]string{"GEMINI_CLI": "false", "CODEIUM": "1"}, "codeium"},
		{"negative no reads unset", map[string]string{"CLINE": "no"}, ""},
		{"negative OFF reads unset case-insensitively", map[string]string{"AIDER": "OFF"}, ""},
		{"nothing set", map[string]string{"PATH": "/usr/bin"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := harness.DetectAgent(lookupFrom(tc.env))

			if tc.want == "" {
				if ok {
					t.Fatalf("DetectAgent = %q, want no detection", got)
				}

				return
			}

			if !ok || got != tc.want {
				t.Errorf("DetectAgent = %q, %v; want %q, true", got, ok, tc.want)
			}
		})
	}
}

// TestAgents pins that every marker-detectable name is listed and that the
// override-only amp entry rides along — the list is documentation surface
// for hosts enumerating what DetectAgent can say.
func TestAgents(t *testing.T) {
	t.Parallel()

	agents := harness.Agents()

	if len(agents) == 0 {
		t.Fatal("Agents() is empty")
	}

	if agents[len(agents)-1] != "amp" {
		t.Errorf("Agents() last entry = %q, want the override-only \"amp\"", agents[len(agents)-1])
	}

	seen := make(map[string]bool, len(agents))
	for _, a := range agents {
		if seen[a] {
			t.Errorf("Agents() has duplicate %q", a)
		}

		seen[a] = true
	}

	for _, h := range harness.Supported() {
		if !seen[h.Name] {
			t.Errorf("Supported() harness %q missing from Agents()", h.Name)
		}
	}
}
