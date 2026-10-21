package sema

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestClassicForLegacyNativeConformance(t *testing.T) {
	var fixture struct {
		Rows []struct {
			ID, APIVersion, Context, Source string
			Compiled                        bool
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/for_loop_legacy_scope.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Rows) != 2 {
		t.Fatalf("native rows=%d, want2", len(fixture.Rows))
	}
	for _, row := range fixture.Rows {
		t.Run(row.Context+"/"+row.APIVersion, func(t *testing.T) {
			var result Result
			switch row.Context {
			case "istest":
				result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"ScopeSourceProbe.cls": row.Source}, row.APIVersion)
			case "anonymous":
				result = AnalyzeAnonymous(typesys.Index{}, row.Source, row.APIVersion)
			default:
				t.Fatalf("unknown native context %q", row.Context)
			}
			if result.HasErrors() == row.Compiled {
				t.Fatalf("native compilation=%t; diagnostics=%#v", row.Compiled, result.Diagnostics)
			}
		})
	}
}

func TestClassicForInitializerMayPrecedeSameNamedLocalAtLegacySource(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe {
  public static Integer run() {
    Integer total = 0;
    for (Integer index = 1; index <= 3; index++) { total += index; }
    Integer index = 10;
    return total + index;
  }
}`,
			}, api)
			if result.HasErrors() {
				t.Fatalf("a completed loop initializer remains visible: %#v", result.Diagnostics)
			}
		})
	}
}

func TestClassicForInitializerStillRejectsVisibleOuterLocalAtLegacySource(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe {
  public static void run() {
    Integer index = 10;
    for (Integer index = 1; index <= 3; index++) {}
  }
}`,
			}, api)
			if !hasScopeRedeclareDiagnostic(result, "index") {
				t.Fatalf("visible outer local collision was accepted: %#v", result.Diagnostics)
			}
		})
	}
}
