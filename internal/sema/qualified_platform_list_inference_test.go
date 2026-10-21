package sema

import "testing"

func TestAnalyzeQualifiedPlatformListCoalesce(t *testing.T) {
	for _, api := range []string{"64.0", "66.0"} {
		for _, initializer := range []string{"new List<Datacloud.MatchRecord>()", "new List<Datacloud.MatchRecord>{}"} {
			for _, siblings := range []string{"", "public class MatchRecord {}"} {
				t.Run(api+"/"+initializer+"/siblings="+siblings, func(t *testing.T) {
					result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
						"MatchEnvelope.cls": `public class MatchEnvelope {
  ` + siblings + `
  public class MatchResult {
    private transient Datacloud.MatchResult nativeResult;
    private List<Datacloud.MatchRecord> records() {
      return this.nativeResult?.matchRecords ?? ` + initializer + `;
    }
  }
}`,
					}, api)
					if result.HasErrors() {
						t.Fatalf("qualified platform list type was lost: %#v", result.Diagnostics)
					}
				})
			}
		}
	}
}

func TestAnalyzeSafeFieldPathCollectionControls(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		wantErrors       bool
	}{
		{"direct_field", "this.nativeResult.matchRecords", false},
		{"safe_field", "this.nativeResult?.matchRecords", false},
		{"unqualified_safe_field", "nativeResult?.matchRecords", false},
		{"unknown_safe_field", "this.nativeResult?.missingRecords", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"MatchEnvelope.cls": `public class MatchEnvelope {
  private Datacloud.MatchResult nativeResult;
  public List<Datacloud.MatchRecord> records() {
    return ` + tc.expression + ` ?? new List<Datacloud.MatchRecord>{};
  }
}`,
			}, "64.0")
			if result.HasErrors() != tc.wantErrors {
				t.Fatalf("errors = %v, want %v: %#v", result.HasErrors(), tc.wantErrors, result.Diagnostics)
			}
		})
	}
}
