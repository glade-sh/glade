package sema

import (
	"strings"
	"testing"
)

func TestAbstractImplementationOverrideRegression(t *testing.T) {
	t.Parallel()
	base := `public abstract class OracleAbstractBase { public abstract String value(); }`
	files := map[string]string{
		"OracleAbstractBase.cls": base,
		"OracleAbstractChild.cls": `public class OracleAbstractChild extends OracleAbstractBase {
  public String value() { return 'value'; }
}
`,
	}
	t.Run("reject without override at API 67", func(t *testing.T) {
		result := analyzeDeclarationProjectWithAPIVersion(t, files, "67.0")
		if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA016") {
			t.Fatalf("abstract implementation without override was not rejected at API 67: %#v", result.Diagnostics)
		}
		for _, diag := range result.Diagnostics {
			if diag.Code == "GLADESEMA016" && !strings.Contains(diag.Message, "value") {
				t.Fatalf("override diagnostic did not name the abstract method: %#v", diag)
			}
		}
	})
	t.Run("accept with override at API 67", func(t *testing.T) {
		withOverride := map[string]string{
			"OracleAbstractBase.cls": base,
			"OracleAbstractChild.cls": `public class OracleAbstractChild extends OracleAbstractBase {
  public override String value() { return 'value'; }
}
`,
		}
		result := analyzeDeclarationProjectWithAPIVersion(t, withOverride, "67.0")
		if result.HasErrors() {
			t.Fatalf("abstract implementation with override was rejected at API 67: %#v", result.Diagnostics)
		}
	})
}