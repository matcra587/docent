package export

import (
	"errors"
	"strings"
	"testing"
)

// TestValidateSkillNameAndPath_aggregatesFailures verifies callers receive
// every independent naming-contract violation while retaining sentinel
// matching for the aggregate.
func TestValidateSkillNameAndPath_aggregatesFailures(t *testing.T) {
	t.Parallel()

	name := strings.Repeat("A", maxSkillNameLen+1)
	err := validateSkillNameAndPath(name, "different/SKILL.md")
	if !errors.Is(err, ErrInvalidSkillName) {
		t.Fatalf("error = %v, want errors.Is(err, ErrInvalidSkillName)", err)
	}

	for _, want := range []string{
		"length is 65 characters",
		"expected lowercase alphanumerics and single hyphens",
		"frontmatter name does not match parent directory",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want aggregate to contain %q", err, want)
		}
	}
}
