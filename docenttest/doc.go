// Package docenttest provides contract-test helpers for docent consumers.
// The primary entry point, Validate, loads guides from an embedded FS and
// reports each violation class as a distinct test failure, so test output
// identifies the class without needing to inspect error message strings.
package docenttest
