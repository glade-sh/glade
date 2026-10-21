package apextest

import (
	"encoding/json"
	"os"
	"testing"
)

// Both family harnesses replay independently captured context-specific rows. These
// regressions require full coverage at each API and its accepted source binding.
func TestStandardExtensionPlainAPI67NativeCoverage(t *testing.T) {
	runStandardExtensionNativeCoverage(t, "67.0", "plain", "", "capture-revision-014375414057111566830152930984709727649268521287756473271958668964100331859797", "0bf6d8f8cd021d96668ce405efd8fcddf53246f4127f987c128f847fb4438cd8")
}

func TestStandardExtensionPlainAPI68NativeCoverage(t *testing.T) {
	runStandardExtensionNativeCoverage(t, "68.0", "plain", "", "capture-revision-036892595090090223725869012924619905863551730461453996790294583751656368536890", "8f0a7f0a248ce26f812f92f2a626e8848f24f910f24bc67c09b2744d401026ea")
}

func TestStandardExtensionNamespaceAPI67NativeCoverage(t *testing.T) {
	runStandardExtensionNativeCoverage(t, "67.0", "namespace", "gladens003", "capture-revision-019964600364505736066616465331432649321326015398954110825668543950358889392854", "3404ab8980a68b6c6d4eab9bf1e34ecaa4fa55740d2d4fdb3cdcc35f619e63a0")
}

func TestStandardExtensionNamespaceAPI68NativeCoverage(t *testing.T) {
	runStandardExtensionNativeCoverage(t, "68.0", "namespace", "gladens003", "capture-revision-077222582010901912844080985694539117916854697950956000968015992978221913654091", "6bfd022c26ffd8f5b7f388464ee3013d56bdae13ffea1191609cfdcdc3aadbe5")
}

func runStandardExtensionNativeCoverage(t *testing.T, api, context, namespace, binding, manifest string) {
	t.Helper()
	routeCount := 2
	if context == "namespace" {
		routeCount = 1
	}
	for family, rowCount := range map[string]int{"formula": 17, "lookup": 20} {
		raw, err := os.ReadFile("testdata/conformance/standard_extension_" + family + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var data standardExtensionNativeData
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatal(err)
		}
		routes := map[string]bool{}
		cells := 0
		for _, capture := range data.Captures {
			if capture.API != api || capture.Context != context {
				continue
			}
			if capture.Namespace != namespace || capture.Route != "anonymous" && capture.Route != "istest" || context == "namespace" && capture.Route != "istest" || routes[capture.Route] || len(capture.Rows) != rowCount || capture.BindingSHA256 != binding || capture.SourceManifestSHA256 != manifest {
				t.Fatalf("invalid independent %s API-%s capture", context, api)
			}
			routes[capture.Route] = true
			for _, row := range capture.Rows {
				var native struct {
					API, Context, Outcome, Expression string
				}
				if json.Unmarshal(row.Native, &native) != nil || native.API != api || native.Context != context || native.Outcome == "" || native.Outcome == "NO_ANSWER" || native.Expression != row.Expression {
					t.Fatalf("missing observed API-%s cell or captured source", api)
				}
				cells++
			}
		}
		if len(routes) != routeCount || cells != rowCount*routeCount {
			t.Errorf("%s %s API-%s native coverage = %d; want %d", family, context, api, cells, rowCount*routeCount)
		}
		for _, cell := range data.PendingCells {
			if cell.API == api && cell.Context == context {
				t.Errorf("observed API-%s %s cell still marked pending", api, context)
			}
		}
	}
}
