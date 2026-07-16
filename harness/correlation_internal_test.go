// White-box: TestSupportedMarkersCorrelation reaches the unexported
// agentMarkers() function directly. Separated from harness_test.go (package
// harness_test) rather than converted in place — merging this file's
// literals into package harness_test's TestDetect table would push several
// harness-name strings past goconst's occurrence threshold in harness.go.
package harness

import "testing"

// TestSupportedMarkersCorrelation pins the structural link between Supported
// and agentMarkers: Detect matches a detected agent name against Supported
// by string-literal equality, so a Supported harness with no agentMarkers
// entry would silently never be detected — DetectAgent never returns its
// name, nothing fails to compile or panics. This test fails loudly instead.
// It also pins that every marker entry carries at least one variable, and
// that entry names are unique — a duplicate would shadow the later entry.
func TestSupportedMarkersCorrelation(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)

	for _, m := range agentMarkers() {
		if seen[m.name] {
			t.Errorf("agentMarkers() has duplicate entry %q", m.name)
		}

		seen[m.name] = true

		if len(m.envVars) == 0 {
			t.Errorf("agentMarkers() entry %q has no marker variables", m.name)
		}
	}

	for _, h := range Supported() {
		if !seen[h.Name] {
			t.Errorf("Supported() harness %q has no agentMarkers() entry", h.Name)
		}
	}
}
