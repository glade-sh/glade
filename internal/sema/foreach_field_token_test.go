package sema

import (
	"encoding/json"
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
  private static GaugePlan reading = new GaugePlan();
  private static GaugePlan openPlan(Boolean ready) { return new GaugePlan(); }
  public static List<Lead> readings(GaugePlan plan) {
    return new List<Lead>{ new Lead(LastName = 'Morrow', Company = 'Gauge Works', AnnualRevenue = 3) };
  }
`
	const loop = `
    for (Lead lead : GaugeRun.readings(openPlan(true)
      .mark(Lead.LastName)
      .mark(Lead.AnnualRevenue)
      .mark('marked'))) {
      Decimal observed = lead.AnnualRevenue;
      System.assertEquals(3, observed);
    }
`
	const shadowingLoop = `
    for (Lead reading : GaugeRun.readings(reading.mark(Lead.AnnualRevenue))) {
      Decimal observed = reading.AnnualRevenue;
      System.assertEquals(3, observed);
    }
    GaugeRun.readings(reading.mark('after loop'));
`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "same-name", body: loop},
		{name: "schema-qualified", body: strings.Replace(loop, "Lead lead", "Schema.Lead lead", 1)},
		{name: "different-name", body: strings.ReplaceAll(loop, "lead", "entry")},
		{name: "shadows-field", body: shadowingLoop},
		{
			name: "nested-loops",
			body: `
    for (Lead outer : new List<Lead>()) {
      for (Lead reading : GaugeRun.readings(reading.mark(outer.LastName))) {
        Decimal observed = reading.AnnualRevenue + outer.AnnualRevenue;
        System.assertEquals(6, observed);
      }
      GaugeRun.readings(reading.mark(outer.LastName));
    }`,
		},
		{
			name: "loop-body-value",
			body: `for (Lead lead : new List<Lead>()) {
      openPlan(true).mark(lead.AnnualRevenue);
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
				Name: "Lead",
				Fields: []schema.Field{
					{Name: "LastName", Type: "Text"},
					{Name: "AnnualRevenue", Type: "Number"},
				},
			}}})
			if len(index.Diagnostics) != 0 {
				t.Fatalf("index diagnostics: %+v", index.Diagnostics)
			}
			result := Analyze(index)
			if tc.name != "same-name" {
				for i := range result.Diagnostics {
					result.Diagnostics[i].File = filepath.Base(result.Diagnostics[i].File)
				}
				encoded, err := json.Marshal(result.Diagnostics)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("diagnostics: %s", encoded)
			}
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
