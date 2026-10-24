package vm_test

import (
	"path/filepath"
	"testing"
)

// Replays captured anonymous requests and @IsTest declarations through direct
// execution and the project runner, including @TestSetup transaction boundaries.
func TestMixedDMLActionsAnonymousAndTestNativeConformance(t *testing.T) {
	runExecutionCaptureFixture(t, "mixed_dml_actions", 42, filepath.Join("testdata", "conformance", "mixed_dml_actions"), "seed.apex")
}

func TestMixedDMLPartialScalarAndMixedListNativeConformance(t *testing.T) {
	runExecutionCaptureFixture(t, "mixed_dml_partial_shapes", 18, filepath.Join("testdata", "conformance", "mixed_dml_partial_shapes"), "seed.apex")
}
