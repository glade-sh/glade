package apextest

import "testing"

// A custom exception keeps its inherited no-argument constructor after it
// declares another constructor, so subclasses compile without super(...).
func TestExceptionImplicitSuperNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "exception_implicit_super", "FaultCtorTest", []string{"62.0", "67.0"}, 1)
}

func TestExceptionExplicitImplicitSuperNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "exception_explicit_implicit_super", "CountedFaultTest", []string{"62.0", "67.0"}, 1)
}
