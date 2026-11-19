package apextest

import "testing"

// Compare the query count after accepted and rejected searches, including the
// uncaught limit error at the twenty-first call in a fresh transaction.
func TestSOSLValidationConformance(t *testing.T) {
	soslFidelityFixture{dir: "testdata/conformance/sosl_validation", countQueries: true}.replay(t)
}
