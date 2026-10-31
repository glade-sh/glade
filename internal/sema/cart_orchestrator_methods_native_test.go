package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestCartOrchestratorMethodsNativeConformance(t *testing.T) {
	dir := "testdata/cart_orchestrator_methods"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]struct{ ID, Name, Source string })
	for _, c := range cases {
		sources[c.ID] = c
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 11 || strings.Join(rows[0], ",") != "api,route,case,outcome,diagnostic" {
		t.Fatalf("unexpected native table: %d rows", len(rows))
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			c, ok := sources[row[2]]
			if !ok {
				t.Fatal("missing case", row[2])
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{c.Name + ".cls": c.Source}, row[0])
			switch row[3] {
			case "accepted":
				if result.HasErrors() {
					t.Fatalf("native accepted; diagnostics: %+v", result.Diagnostics)
				}
			case "rejected":
				if !result.HasErrors() {
					t.Fatalf("native rejected %q; Glade accepted", row[4])
				}
			default:
				t.Fatalf("unexpected native outcome %q", row[3])
			}
		})
	}
}

// Local route twins compile the captured declaration through one shared index.
// They do not execute a hosted calculator or claim an additional native capture.
func TestCartOrchestratorMethodLocalCompileRoutes(t *testing.T) {
	const probe = `public class ProbeShippingPromotions extends CartExtension.CartCalculate {
 public override void calculate(CartExtension.CartCalculateOrchestratorRequest r) {
  shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null);
 }
}`
	const observer = `@IsTest private class ProbeObserver {
 @IsTest static void observe() { CartExtension.CartCalculate value = null; try { value.shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null); } catch (NullPointerException expected) {} }
}`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			p := filepath.Join(root, "ProbeShippingPromotions.cls")
			test := filepath.Join(root, "ProbeObserver.cls")
			writeSemaFile(t, p, probe)
			writeSemaFile(t, test, observer)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{p, test}}, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			for i := 0; i < 2; i++ {
				if got := Analyze(index); got.HasErrors() {
					t.Fatalf("project pass %d: %+v", i, got.Diagnostics)
				}
				if got := AnalyzeAnonymous(index, "CartExtension.CartCalculate value = null; value.shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null);", api); got.HasErrors() {
					t.Fatalf("anonymous pass %d: %+v", i, got.Diagnostics)
				}
			}
		})
	}
}
