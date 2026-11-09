package apextest

import (
	"os"
	"testing"
)

// gift-readonly rows capture the missing-field and noneditable exceptions.
func TestB5B9FeatureReadOnlyNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			source, err := os.ReadFile("testdata/conformance/standard_describe_profiles/b5b9_feature_objects/readonly/anon-" + api + ".apex")
			if err != nil {
				t.Fatal(err)
			}
			runStandardFieldEvidenceRoutes(t, b5b9AccessProject(t, "gift", api), api, string(source))
		})
	}
}
