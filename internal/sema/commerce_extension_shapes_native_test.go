package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestCommerceExtensionShapesNativeConformance(t *testing.T) {
	dir := "../../testdata/conformance/commerce_extension_shapes"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source, Anonymous string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	sources := map[string]struct{ name, source, anonymous string }{}
	for _, c := range cases {
		if c.ID == "" || c.Name == "" || c.Source == "" {
			t.Fatal("incomplete source case")
		}
		if _, exists := sources[c.ID]; exists {
			t.Fatal("duplicate source case")
		}
		sources[c.ID] = struct{ name, source, anonymous string }{c.Name, c.Source, c.Anonymous}
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "api,route,case,outcome,diagnostic" {
		t.Fatal("invalid native table")
	}
	seen := map[string]bool{}
	checked := 0
	for _, row := range rows[1:] {
		anonymous := row[1] == "anonymous" && strings.HasPrefix(row[2], "pricing_contract_")
		if row[1] != "deploy" && !anonymous {
			continue
		}
		key := row[0] + "/" + row[1] + "/" + row[2]
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native key %q", key)
		}
		seen[key] = true
		c, exists := sources[row[2]]
		if !exists {
			t.Fatal("missing source: " + row[2])
		}
		checked++
		t.Run(key, func(t *testing.T) {
			var result Result
			if anonymous {
				if c.anonymous == "" {
					t.Fatal("missing anonymous source")
				}
				result = AnalyzeAnonymous(typesys.Index{}, c.anonymous, row[0])
			} else {
				result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{c.name + ".cls": c.source}, row[0])
			}
			if row[3] == "accepted" {
				if result.HasErrors() {
					t.Fatalf("native accepted; Glade diagnostics: %+v", result.Diagnostics)
				}
				return
			}
			if (row[3] != "rejected" && !(anonymous && row[3] == "compile_error")) || row[4] == "" {
				t.Fatal("unexpected native outcome: " + row[3])
			}
			for _, d := range result.Diagnostics {
				if d.NativeMessage == row[4] || d.Message == row[4] ||
					(strings.HasPrefix(row[2], "pricing_contract_") && strings.Contains(d.NativeMessage, "Invalid type: commercestorepricing.ContractResolution")) {
					return
				}
			}
			t.Fatalf("native rejected: %s; Glade diagnostics: %+v", row[4], result.Diagnostics)
		})
	}
	if checked != 72 || len(seen) != 72 {
		t.Fatalf("incomplete native table: checked=%d keys=%d", checked, len(seen))
	}
}

func TestCommerceExtensionContractVisibilityObservationBounds(t *testing.T) {
	for _, name := range []string{"commercestorepricing.ContractResolutionRequest", "commercestorepricing.ContractResolutionResponse"} {
		for _, api := range []string{"62.0", "67.0", "v62.0", "067.0"} {
			if !semaPlatformTypeUnavailable(api, name) {
				t.Errorf("captured rejection lost: API %s %s", api, name)
			}
		}
		// These are compatibility bounds, not claims of native visibility at
		// versions for which this family has no captured rows.
		for _, api := range []string{"61.0", "63.0", "64.0", "65.0", "66.0", "68.0", ""} {
			if semaPlatformTypeUnavailable(api, name) {
				t.Errorf("uncaptured version behavior changed: API %s %s", api, name)
			}
		}
	}
}

func TestCommerceExtensionContractVisibilityPreservesOtherTypes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			// The initial enabled-Commerce capture accepts PricingService at
			// both APIs. The uncatalogued names only protect the existing open
			// namespace behavior; they are not native acceptance observations.
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe {
                    commercestorepricing.PricingService pricing;
                    commercestorepricing.UncataloguedShape partialCatalog;
                    syntheticpackage.UncataloguedShape partialPackage;
                }`,
			}, api)
			if result.HasErrors() {
				t.Fatalf("unrelated type references changed: %+v", result.Diagnostics)
			}
			result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"commercestorepricing.cls": `public class commercestorepricing {
                    public class ContractResolutionRequest { public Integer size() { return 1; } }
                    public class ContractResolutionResponse { public Integer size() { return 2; } }
                }`,
				"Probe.cls": `public class Probe {
                    commercestorepricing.ContractResolutionRequest request;
                    commercestorepricing.ContractResolutionResponse response;
                    public static Integer run() {
                        commercestorepricing.ContractResolutionRequest localRequest = new commercestorepricing.ContractResolutionRequest();
                        commercestorepricing.ContractResolutionResponse localResponse = new commercestorepricing.ContractResolutionResponse();
                        return localRequest.size() + localResponse.size();
                    }
                }`,
			}, api)
			if result.HasErrors() {
				t.Fatalf("source-owned types must shadow platform observations: %+v", result.Diagnostics)
			}
		})
	}
}
