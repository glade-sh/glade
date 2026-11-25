package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestForeachIterableKeepsFluentFieldTokenOverload(t *testing.T) {
	const plan = `
public class GaugePlan {
  public GaugePlan mark(Schema.SObjectField field) { return this; }
  public GaugePlan mark(String label) { return this; }
}
`
	const helpers = `
  private static GaugePlan openPlan(Boolean ready) { return new GaugePlan(); }
  public static List<CrateGauge__c> readings(GaugePlan plan) {
    return new List<CrateGauge__c>{ new CrateGauge__c(Load__c = 3) };
  }
`
	const loop = `
    for (CrateGauge__c crategauge__c : GaugeRun.readings(openPlan(true)
      .mark(CrateGauge__c.Name)
      .mark(CrateGauge__c.Load__c)
      .mark('marked'))) {
      Decimal observed = crategauge__c.Load__c;
      System.assertEquals(3, observed);
    }
`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "same-name", body: loop},
		{name: "different-name", body: strings.ReplaceAll(loop, "crategauge__c", "reading")},
		{
			name: "loop-body-value",
			body: `for (CrateGauge__c crategauge__c : new List<CrateGauge__c>()) {
      openPlan(true).mark(crategauge__c.Load__c);
    }`,
			want: `method "run" has no matching overload for call "mark" with 1 argument(s)`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			planPath := filepath.Join(root, "GaugePlan.cls")
			runPath := filepath.Join(root, "GaugeRun.cls")
			writeSemaFile(t, planPath, plan)
			writeSemaFile(t, runPath, "public class GaugeRun {"+helpers+"public static void run() {"+tc.body+"} }")
			index := typesys.Build(project.Project{
				Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{planPath, runPath},
			}, schema.Schema{Objects: []schema.Object{{
				Name: "CrateGauge__c",
				Fields: []schema.Field{
					{Name: "Name", Type: "Text"},
					{Name: "Load__c", Type: "Number"},
				},
			}}})
			if len(index.Diagnostics) != 0 {
				t.Fatalf("index diagnostics: %+v", index.Diagnostics)
			}
			result := Analyze(index)
			if tc.want == "" {
				if len(result.Diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
				}
				return
			}
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA009" ||
				result.Diagnostics[0].Message != tc.want {
				t.Fatalf("want one overload diagnostic %q, got %+v", tc.want, result.Diagnostics)
			}
		})
	}
}
