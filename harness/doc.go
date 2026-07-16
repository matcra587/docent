// Package harness detects which AI agent runtime is invoking the host CLI.
// Detection uses the environment markers each runtime sets for its child
// processes — the same signals the vercel-labs skills installer keys on —
// so it needs no configuration and no filesystem probing.
//
// Two detection surfaces serve two host needs. DetectAgent answers "is an
// agent driving this process, and which one" across every runtime with a
// known marker — the signal hosts use to resolve output modes (an agent
// gets machine-shaped output, a human terminal gets prose). Detect answers
// the narrower export question — "which runtime's skills-directory
// conventions apply" — and identifies only the harnesses with known
// conventions; the cobra adapter's "agent export --scope" resolution is
// built on it.
//
// The environment variable AI_AGENT overrides both surfaces, letting a user
// or wrapper declare the invoking agent explicitly when no marker is set.
package harness
