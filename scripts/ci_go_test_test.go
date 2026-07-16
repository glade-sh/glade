package scripts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type apexHistoryFixture struct {
	root   string
	shards [2]string
	output string
}

const semaFixturePackage = "github.com/glade-sh/glade/internal/sema"

var nodeIntegrationTests = map[string][]string{
	"github.com/glade-sh/glade/internal/gladecli": {
		"TestRunDoctorReportsParser",
		"TestRunDoctorJSON",
		"TestRunDoctorShortFlags",
		"TestRunDoctorReportsProjectLocalDataEnvironment",
	},
	"github.com/glade-sh/glade/internal/gladehome": {
		"TestValidateRootFindsRepoCheckout",
		"TestInstallFromCWDSkipsGlobalShareAsSource",
		"TestInstallFromCopiesToolchain",
		"TestEnsureRootHonorsExplicitGladeHomeBeforeUserShare",
	},
	"github.com/glade-sh/glade/internal/lwc/compile": {
		"TestCompileProjectLWCBundles",
		"TestCompileRewritesTemplateStylesheetImports",
		"TestCompileEmitsSiblingJSModules",
		"TestCompileEmitsUtilityOnlyLWCModules",
		"TestCompileEmitsAdditionalHTMLTemplateModules",
		"TestCompileTransformsCustomRenderComponentWithoutSameNameTemplate",
		"TestCompileEnablesLwcOnDirective",
	},
	"github.com/glade-sh/glade/internal/lwcbrowser": {
		"TestSetupBundleIncludesLabelsSibling",
		"TestSetupImportMapIncludesLocalComponents",
	},
	"github.com/glade-sh/glade/internal/server": {
		"TestVFPageBootstrapsLightningOut",
		"TestVFPageBootstrapsMultiWidgetLightningOut",
		"TestLightningModulesServesCompiledJS",
		"TestLightningModulesServesSiblingModuleWithoutJSExtension",
		"TestLWCShellComponentRouteServesHTML",
		"TestLWCShellRootRendersHomeWithFormalTabsAndBuilderLink",
		"TestLWCShellBuilderRouteRendersBuilderNavigationLayoutAndSampleRecord",
		"TestLWCShellTabRouteIncludesPreviewRouteCatalog",
		"TestServerRootRendersLWCHomeWhenProjectHasLWCs",
		"TestLWCShellRendersApplicationNavAndConsoleMode",
		"TestLWCShellAppRouteFallsBackToApplicationDefaultTab",
		"TestLWCShellUnsupportedCustomTabReturnsDiagnostic",
		"TestLWCShellMixedPageDiagnosticsStillRendersValidComponents",
	},
}

var nodeIntegrationRunNames = []string{
	"TestCompileProjectLWCBundles", "TestCompileRewritesTemplateStylesheetImports", "TestCompileEmitsSiblingJSModules",
	"TestCompileEmitsUtilityOnlyLWCModules", "TestCompileEmitsAdditionalHTMLTemplateModules",
	"TestCompileTransformsCustomRenderComponentWithoutSameNameTemplate", "TestCompileEnablesLwcOnDirective",
	"TestSetupBundleIncludesLabelsSibling", "TestSetupImportMapIncludesLocalComponents",
	"TestVFPageBootstrapsLightningOut", "TestVFPageBootstrapsMultiWidgetLightningOut", "TestLightningModulesServesCompiledJS",
	"TestLightningModulesServesSiblingModuleWithoutJSExtension", "TestLWCShellComponentRouteServesHTML",
	"TestLWCShellRootRendersHomeWithFormalTabsAndBuilderLink", "TestLWCShellBuilderRouteRendersBuilderNavigationLayoutAndSampleRecord",
	"TestLWCShellTabRouteIncludesPreviewRouteCatalog", "TestServerRootRendersLWCHomeWhenProjectHasLWCs",
	"TestLWCShellRendersApplicationNavAndConsoleMode", "TestLWCShellAppRouteFallsBackToApplicationDefaultTab",
	"TestLWCShellUnsupportedCustomTabReturnsDiagnostic", "TestLWCShellMixedPageDiagnosticsStillRendersValidComponents",
	"TestValidateRootFindsRepoCheckout", "TestInstallFromCWDSkipsGlobalShareAsSource", "TestInstallFromCopiesToolchain",
	"TestEnsureRootHonorsExplicitGladeHomeBeforeUserShare", "TestRunDoctorReportsParser", "TestRunDoctorJSON",
	"TestRunDoctorShortFlags", "TestRunDoctorReportsProjectLocalDataEnvironment",
}

func runSemaShardFixture(t *testing.T, index, discovery, plan, events string, nativeRC int) (string, error, string, string) {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	artifacts := filepath.Join(dir, "artifacts")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{"discovery": discovery, "events": events} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goScript := `#!/usr/bin/env bash
if [[ "$*" == "test -list ^Test ./internal/sema" ]]; then cat "$FIXTURE_DISCOVERY"; exit 0; fi
if [[ "$1" == "test" && "$2" == "-json" ]]; then printf '%s\n' "$*" >>"$FIXTURE_CALLS"; cat "$FIXTURE_EVENTS"; exit "$FIXTURE_NATIVE_RC"; fi
exit 97
`
	plannerScript := "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >\"$FIXTURE_PLANNER_ARGS\"\ncat <<'EOF'\n" + plan + "\nEOF\n"
	for name, contents := range map[string]string{"go": goScript, "planner": plannerScript} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(contents), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	calls := filepath.Join(dir, "calls")
	plannerArgs := filepath.Join(dir, "planner-args")
	cmd := exec.Command("bash", "ci-go-test.sh", "sema-shard", index)
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"), "CI_SHARD_PLANNER="+filepath.Join(binDir, "planner"), "CI_SEMA_ARTIFACT_DIR="+artifacts, "FIXTURE_DISCOVERY="+filepath.Join(dir, "discovery"), "FIXTURE_EVENTS="+filepath.Join(dir, "events"), "FIXTURE_CALLS="+calls, "FIXTURE_PLANNER_ARGS="+plannerArgs, fmt.Sprintf("FIXTURE_NATIVE_RC=%d", nativeRC))
	out, err := cmd.CombinedOutput()
	return string(out), err, artifacts, plannerArgs
}

func semaFixturePlan() string {
	return `{"version":1,"package":"` + semaFixturePackage + `","historyUsed":false,"shards":[{"index":0,"tests":["TestAlpha"],"estimatedDurationMillis":0,"regex":"^(?:TestAlpha)$"},{"index":1,"tests":["TestBeta"],"estimatedDurationMillis":0,"regex":"^(?:TestBeta)$"}]}`
}

func semaPassEvent(name string) string {
	return `{"Action":"pass","Package":"` + semaFixturePackage + `","Test":"` + name + `","Elapsed":1}` + "\n" +
		`{"Action":"pass","Package":"` + semaFixturePackage + `","Elapsed":1}` + "\n"
}

func realGoCommand(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func writeJSONFixture(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func newApexHistoryFixture(t *testing.T, shardElapsed [2]float64) apexHistoryFixture {
	t.Helper()
	fixture := apexHistoryFixture{root: t.TempDir()}
	fixture.output = filepath.Join(fixture.root, "history", "apextest-duration-history.json")
	names := make([]string, 279)
	for i := range names {
		names[i] = fmt.Sprintf("TestHistory%03d", i)
	}
	shardTests := [2][]string{names[:140], names[140:]}
	shards := make([]map[string]any, 2)
	for index := range shards {
		regex := "^(?:" + strings.Join(shardTests[index], "|") + ")$"
		shards[index] = map[string]any{
			"index":                   index,
			"tests":                   shardTests[index],
			"estimatedDurationMillis": 0,
			"regex":                   regex,
		}
	}
	plan := map[string]any{
		"version": 1, "package": fixturePackage, "historyUsed": false, "shards": shards,
	}
	for index := 0; index < 2; index++ {
		dir := filepath.Join(fixture.root, fmt.Sprintf("apex-shard-%d", index))
		fixture.shards[index] = dir
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "discovery.txt"), []byte(strings.Join(names, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		writeJSONFixture(t, filepath.Join(dir, "plan.json"), plan)
		writeJSONFixture(t, filepath.Join(dir, "selected-shard.json"), shards[index])
		writeJSONFixture(t, filepath.Join(dir, "validation-summary.json"), map[string]any{
			"valid": true, "expected": shardTests[index], "passed": shardTests[index], "errors": []string{},
		})
		var events strings.Builder
		for testIndex, name := range shardTests[index] {
			elapsed := 0.0
			if testIndex == 0 {
				elapsed = shardElapsed[index]
			}
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%g}\n", fixturePackage, name, elapsed)
		}
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func runApexHistoryRefresh(t *testing.T, fixture apexHistoryFixture) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "ci-go-test.sh", "apex-history-refresh", fixture.shards[0], fixture.shards[1], fixture.output)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCIApexDurationHistoryRefresh(t *testing.T) {
	fixture := newApexHistoryFixture(t, [2]float64{1, 1})
	out, err := runApexHistoryRefresh(t, fixture)
	if err != nil {
		t.Fatalf("refresh failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(fixture.output)
	if err != nil {
		t.Fatal(err)
	}
	var history struct {
		Version  int    `json:"version"`
		Package  string `json:"package"`
		Complete bool   `json:"complete"`
		Tests    []struct {
			Name           string `json:"name"`
			DurationMillis int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	if history.Version != 1 || history.Package != fixturePackage || !history.Complete || len(history.Tests) != 279 {
		t.Fatalf("history header/count = %+v / %d", history, len(history.Tests))
	}
	for i, item := range history.Tests {
		if want := fmt.Sprintf("TestHistory%03d", i); item.Name != want {
			t.Fatalf("history test %d = %q, want %q", i, item.Name, want)
		}
	}
	if history.Tests[0].DurationMillis != 1000 || history.Tests[140].DurationMillis != 1000 {
		t.Fatalf("history durations did not come from shard events: first=%d second=%d", history.Tests[0].DurationMillis, history.Tests[140].DurationMillis)
	}
}

func TestCIApexDurationHistoryProducerCanonicalizesDiscovery(t *testing.T) {
	fixture := newApexHistoryFixture(t, [2]float64{1, 1})
	names := make([]string, 279)
	for i := range names {
		names[i] = fmt.Sprintf("TestHistory%03d", 278-i)
	}
	rawPath := filepath.Join(fixture.root, "go-test-list.txt")
	canonicalPath := filepath.Join(fixture.root, "canonical-discovery.txt")
	raw := strings.Join(names, "\n") + "\nok  \t" + fixturePackage + "\n"
	if err := os.WriteFile(rawPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", `source ./ci-go-test.sh; validate_discovery "$1" "$2"`, "producer-fixture", rawPath, canonicalPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("discovery producer failed: %v\n%s", err, out)
	}
	canonical, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	var expected strings.Builder
	for i := 0; i < 279; i++ {
		fmt.Fprintf(&expected, "TestHistory%03d\n", i)
	}
	if string(canonical) != expected.String() {
		t.Fatalf("producer discovery is not canonical; first lines: %q", strings.Join(strings.Split(string(canonical), "\n")[:3], "\n"))
	}
	for _, dir := range fixture.shards {
		if err := os.WriteFile(filepath.Join(dir, "discovery.txt"), canonical, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runApexHistoryRefresh(t, fixture)
	if err != nil {
		t.Fatalf("refresh rejected producer-canonical discovery: %v\n%s", err, out)
	}
}

func TestCIApexDurationHistoryRejectsInvalidEvidence(t *testing.T) {
	mutateBothPlans := func(t *testing.T, fixture apexHistoryFixture, old, replacement string) {
		t.Helper()
		for _, dir := range fixture.shards {
			path := filepath.Join(dir, "plan.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			mutated := strings.Replace(string(data), old, replacement, 1)
			if mutated == string(data) {
				t.Fatalf("plan mutation did not replace %q", old)
			}
			if err := os.WriteFile(path, []byte(mutated), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	setPlanField := func(t *testing.T, fixture apexHistoryFixture, mutate func(map[string]any)) {
		t.Helper()
		for _, dir := range fixture.shards {
			path := filepath.Join(dir, "plan.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var plan map[string]any
			if err := json.Unmarshal(data, &plan); err != nil {
				t.Fatal(err)
			}
			mutate(plan)
			writeJSONFixture(t, path, plan)
		}
	}
	cases := map[string]func(*testing.T, apexHistoryFixture){
		"duplicate terminal": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			data = append(data, []byte(fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestHistory000\",\"Elapsed\":0}\n", fixturePackage))...)
			_ = os.WriteFile(path, data, 0o600)
		},
		"missing terminal": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			_ = os.WriteFile(path, []byte(strings.Join(lines[1:], "\n")+"\n"), 0o600)
		},
		"failed terminal": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"Action":"pass"`, `"Action":"fail"`, 1)), 0o600)
		},
		"skipped terminal": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"Action":"pass"`, `"Action":"skip"`, 1)), 0o600)
		},
		"wrong package": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), fixturePackage, "example.invalid/apextest", 1)), 0o600)
		},
		"negative duration": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"Elapsed":1`, `"Elapsed":-1`, 1)), 0o600)
		},
		"nonfinite duration": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "events.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"Elapsed":1`, `"Elapsed":NaN`, 1)), 0o600)
		},
		"plan mismatch": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[1], "plan.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"historyUsed": false`, `"historyUsed": true`, 1)), 0o600)
		},
		"discovery mismatch": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[1], "discovery.txt")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), "TestHistory278", "TestHistory999", 1)), 0o600)
		},
		"swapped noncanonical discovery": func(t *testing.T, fixture apexHistoryFixture) {
			for _, dir := range fixture.shards {
				path := filepath.Join(dir, "discovery.txt")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				mutated := strings.Replace(string(data), "TestHistory000\nTestHistory001", "TestHistory001\nTestHistory000", 1)
				if err := os.WriteFile(path, []byte(mutated), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		},
		"invalid historyUsed type": func(t *testing.T, fixture apexHistoryFixture) {
			mutateBothPlans(t, fixture, `"historyUsed": false`, `"historyUsed": "false"`)
		},
		"empty regex": func(t *testing.T, fixture apexHistoryFixture) {
			setPlanField(t, fixture, func(plan map[string]any) {
				plan["shards"].([]any)[0].(map[string]any)["regex"] = ""
			})
		},
		"noncanonical regex": func(t *testing.T, fixture apexHistoryFixture) {
			mutateBothPlans(t, fixture, `TestHistory000|TestHistory001`, `TestHistory001|TestHistory000`)
		},
		"string estimate": func(t *testing.T, fixture apexHistoryFixture) {
			mutateBothPlans(t, fixture, `"estimatedDurationMillis": 0`, `"estimatedDurationMillis": "0"`)
		},
		"extra plan field": func(t *testing.T, fixture apexHistoryFixture) {
			mutateBothPlans(t, fixture, `"historyUsed": false,`, `"historyUsed": false, "unexpected": true,`)
		},
		"invalid summary": func(t *testing.T, fixture apexHistoryFixture) {
			writeJSONFixture(t, filepath.Join(fixture.shards[0], "validation-summary.json"), map[string]any{"valid": false})
		},
		"extra summary field": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[0], "validation-summary.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			mutated := strings.Replace(string(data), `"valid": true`, `"valid": true, "unexpected": true`, 1)
			if err := os.WriteFile(path, []byte(mutated), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"selected union duplicate": func(t *testing.T, fixture apexHistoryFixture) {
			path := filepath.Join(fixture.shards[1], "selected-shard.json")
			data, _ := os.ReadFile(path)
			_ = os.WriteFile(path, []byte(strings.Replace(string(data), "TestHistory140", "TestHistory000", 1)), 0o600)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newApexHistoryFixture(t, [2]float64{1, 1})
			mutate(t, fixture)
			out, err := runApexHistoryRefresh(t, fixture)
			if err == nil {
				t.Fatalf("invalid evidence was accepted\n%s", out)
			}
			if _, statErr := os.Stat(fixture.output); !os.IsNotExist(statErr) {
				t.Fatalf("refresh published history on failure: %v", statErr)
			}
		})
	}
}

func TestCIApexDurationHistoryRejectsAggregateOverflow(t *testing.T) {
	// Every individual duration is below MaxInt64 milliseconds, while 279 of
	// them overflow the schema-v1 aggregate. Equal per-test values keep the two
	// shard elapsed totals balanced, isolating the overflow guard.
	fixture := newApexHistoryFixture(t, [2]float64{})
	for _, dir := range fixture.shards {
		selectedData, err := os.ReadFile(filepath.Join(dir, "selected-shard.json"))
		if err != nil {
			t.Fatal(err)
		}
		var selected struct {
			Tests []string `json:"tests"`
		}
		if err := json.Unmarshal(selectedData, &selected); err != nil {
			t.Fatal(err)
		}
		var events strings.Builder
		for _, name := range selected.Tests {
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":40000000000000}\n", fixturePackage, name)
		}
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runApexHistoryRefresh(t, fixture)
	if err == nil {
		t.Fatalf("aggregate duration overflow was accepted\n%s", out)
	}
	if _, statErr := os.Stat(fixture.output); !os.IsNotExist(statErr) {
		t.Fatalf("overflow refresh published history: %v", statErr)
	}
}

func TestCIApexDurationHistoryAcceptsExactAggregateLimit(t *testing.T) {
	fixture := newApexHistoryFixture(t, [2]float64{})
	const maxInt64 = int64(^uint64(0) >> 1)
	// Keep the large seconds value exactly representable as float64, calculate
	// the generator's millisecond rounding, then fill the small remainder with
	// a second event in each shard.
	seconds := maxInt64/2000 - 10
	baseMillis := int64(math.Floor(float64(seconds)*1000 + 0.5))
	remainder := maxInt64 - 2*baseMillis
	if remainder < 0 || remainder > 100000 {
		t.Fatalf("unexpected exact-boundary fixture remainder %d", remainder)
	}
	extra := [2]int64{remainder / 2, remainder - remainder/2}
	for index, dir := range fixture.shards {
		selectedData, err := os.ReadFile(filepath.Join(dir, "selected-shard.json"))
		if err != nil {
			t.Fatal(err)
		}
		var selected struct {
			Tests []string `json:"tests"`
		}
		if err := json.Unmarshal(selectedData, &selected); err != nil {
			t.Fatal(err)
		}
		var events strings.Builder
		for testIndex, name := range selected.Tests {
			elapsed := "0"
			if testIndex == 0 {
				elapsed = strconv.FormatInt(seconds, 10)
			} else if testIndex == 1 {
				elapsed = strconv.FormatFloat(float64(extra[index])/1000, 'f', 3, 64)
			}
			fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%s}\n", fixturePackage, name, elapsed)
		}
		if err := os.WriteFile(filepath.Join(dir, "events.json"), []byte(events.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runApexHistoryRefresh(t, fixture)
	if err != nil {
		t.Fatalf("exact MaxInt64 aggregate was rejected: %v\n%s", err, out)
	}
	data, err := os.ReadFile(fixture.output)
	if err != nil {
		t.Fatal(err)
	}
	var history struct {
		Tests []struct {
			DurationMillis int64 `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	var total uint64
	for _, test := range history.Tests {
		total += uint64(test.DurationMillis)
	}
	if total != uint64(maxInt64) {
		t.Fatalf("history duration total = %d, want %d", total, uint64(maxInt64))
	}
}

func TestCIApexDurationHistoryImbalanceGuard(t *testing.T) {
	for _, tc := range []struct {
		name    string
		elapsed [2]float64
		wantErr bool
	}{
		{name: "balanced", elapsed: [2]float64{1, 1}},
		{name: "exact 1.5 median boundary", elapsed: [2]float64{3, 1}},
		{name: "above 1.5 median", elapsed: [2]float64{3.001, 1}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newApexHistoryFixture(t, tc.elapsed)
			out, err := runApexHistoryRefresh(t, fixture)
			if (err != nil) != tc.wantErr {
				t.Fatalf("refresh error = %v, wantErr %v\n%s", err, tc.wantErr, out)
			}
		})
	}
}

func TestCIApexDurationHistoryWorkflowOwnership(t *testing.T) {
	scriptData, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(scriptData)
	for _, want := range []string{
		`if [[ -s "${CI_APEXTEST_HISTORY_PATH:-}" ]]`,
		`planner+=(--history "${CI_APEXTEST_HISTORY_PATH}")`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("planner history input wiring missing %q", want)
		}
	}

	workflowPath := filepath.Join("..", ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	jobs := workflowJobBlocks(t, workflow)
	matrix := jobs["apextest"]
	refresh := jobs["apextest-history"]
	for _, want := range []string{
		"actions/cache/restore@v4", "apextest-duration-history-v1", "runner.os", "runner.arch", "1.26.5", "hashFiles('go.sum')",
		"CI_APEXTEST_HISTORY_PATH", "github.sha", "github.run_id", "github.run_attempt",
	} {
		if !strings.Contains(matrix, want) {
			t.Errorf("matrix history restore missing %q", want)
		}
	}
	if strings.Contains(matrix, "actions/cache/save") {
		t.Error("matrix job must be read-only for duration history")
	}
	for _, want := range []string{
		"needs: apextest", "if: ${{ success() }}", "actions/download-artifact@v7", "apex-shard-*", "scripts/ci-go-test.sh apex-history-refresh",
		"actions/cache/save@v4", "actions/upload-artifact@v6", "apextest-duration-history-v1", "github.sha", "github.run_id", "github.run_attempt",
	} {
		if !strings.Contains(refresh, want) {
			t.Errorf("single-writer refresh job missing %q", want)
		}
	}
	if strings.Count(workflow, "actions/cache/save@v4") != 16 {
		t.Errorf("cache save count = %d, want existing DAG writers plus Apex and sema history writers", strings.Count(workflow, "actions/cache/save@v4"))
	}
	if strings.Count(workflow, "shard: [0, 1]") != 2 {
		t.Error("workflow must contain the Apex and sema two-native-shard matrices")
	}
	if matches, _ := filepath.Glob(filepath.Join("..", "**", "*duration-history*.json")); len(matches) != 0 {
		t.Fatalf("tracked duration history candidates: %v", matches)
	}
}

func TestCIGoCacheReportsMissingDirectoriesAsZero(t *testing.T) {
	readWorkflow := func(name string) string {
		t.Helper()
		workflowPath := filepath.Join("..", ".github", "workflows", name)
		workflow, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatalf("read %s: %v", workflowPath, err)
		}
		return string(workflow)
	}
	extractReportScripts := func(workflow string) []string {
		lines := strings.Split(workflow, "\n")
		var scripts []string
		for i, line := range lines {
			if !strings.Contains(line, "- name: Report Go ") {
				continue
			}
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) != "run: |" {
					continue
				}
				runIndent := len(lines[j]) - len(strings.TrimLeft(lines[j], " "))
				var script strings.Builder
				for k := j + 1; k < len(lines); k++ {
					indent := len(lines[k]) - len(strings.TrimLeft(lines[k], " "))
					if strings.TrimSpace(lines[k]) != "" && indent <= runIndent {
						break
					}
					script.WriteString(strings.TrimPrefix(lines[k], strings.Repeat(" ", runIndent+2)))
					script.WriteByte('\n')
				}
				scripts = append(scripts, script.String())
				break
			}
		}
		return scripts
	}

	for _, workflow := range []struct {
		name string
		text string
		want int
	}{
		{name: "ci.yml", text: readWorkflow("ci.yml"), want: 2},
		{name: "security.yml", text: readWorkflow("security.yml"), want: 6},
	} {
		scripts := extractReportScripts(workflow.text)
		if len(scripts) != workflow.want {
			t.Fatalf("%s report scripts = %d, want %d", workflow.name, len(scripts), workflow.want)
		}
		for i, script := range scripts {
			cmd := exec.Command("bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script)
			cmd.Env = []string{
				"HOME=" + t.TempDir(),
				"MATCHED_KEY=",
				"PATH=" + os.Getenv("PATH"),
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s report script %d failed for missing cache directory: %v\n%s", workflow.name, i+1, err, out)
			}
			if !strings.Contains(string(out), "bytes=0") {
				t.Fatalf("%s report script %d output missing bytes=0:\n%s", workflow.name, i+1, out)
			}
		}
	}
}

func TestCIGoCacheOwnership(t *testing.T) {
	readWorkflow := func(name string) string {
		t.Helper()
		workflowPath := filepath.Join("..", ".github", "workflows", name)
		workflow, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatalf("read %s: %v", workflowPath, err)
		}
		return string(workflow)
	}

	ciWorkflow := readWorkflow("ci.yml")
	ci := workflowJobBlocks(t, ciWorkflow)["test"]
	security := readWorkflow("security.yml")

	if got := strings.Count(ci, "actions/setup-go@v6"); got != 1 {
		t.Fatalf("ci.yml setup-go uses = %d, want 1", got)
	}
	if got := strings.Count(ci, "cache: false"); got != 1 {
		t.Fatalf("ci.yml setup-go cache opt-outs = %d, want 1", got)
	}
	if got := strings.Count(security, "actions/setup-go@v6"); got != 3 {
		t.Fatalf("security.yml setup-go uses = %d, want 3", got)
	}
	if got := strings.Count(security, "cache: false"); got != 3 {
		t.Fatalf("security.yml setup-go cache opt-outs = %d, want 3", got)
	}

	if got := strings.Count(ci, "actions/cache/restore@v4"); got != 2 {
		t.Fatalf("ci.yml cache restores = %d, want 2", got)
	}
	if got := strings.Count(ci, "actions/cache/save@v4"); got != 2 {
		t.Fatalf("ci.yml cache saves = %d, want 2", got)
	}
	if got := strings.Count(security, "actions/cache/restore@v4"); got != 6 {
		t.Fatalf("security.yml cache restores = %d, want 6", got)
	}
	if got := strings.Count(security, "actions/cache/save@v4"); got != 0 {
		t.Fatalf("security.yml cache saves = %d, want 0", got)
	}
	if got := strings.Count(ci, "continue-on-error: true"); got != 4 {
		t.Fatalf("ci.yml non-fatal cache actions = %d, want 4", got)
	}
	if got := strings.Count(security, "continue-on-error: true"); got != 6 {
		t.Fatalf("security.yml non-fatal cache actions = %d, want 6", got)
	}

	for _, workflow := range []struct {
		name string
		text string
	}{
		{name: "ci.yml", text: ci},
		{name: "security.yml", text: security},
	} {
		for _, want := range []string{"path: ~/go/pkg/mod", "path: ~/.cache/go-build", "go-mod-cache-v1", "go-build-cache-v1"} {
			if !strings.Contains(workflow.text, want) {
				t.Errorf("%s missing separate cache marker %q", workflow.name, want)
			}
		}
	}

	fullKeys := func(workflow string) []string {
		var keys []string
		for _, line := range strings.Split(workflow, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "key: ") && strings.Contains(line, "github.sha") {
				keys = append(keys, strings.TrimPrefix(line, "key: "))
			}
		}
		return keys
	}
	assertFullKeys := func(name, workflow, sumExpression string, want int) {
		t.Helper()
		keys := fullKeys(workflow)
		if len(keys) != want {
			t.Errorf("%s full cache keys = %d, want %d", name, len(keys), want)
		}
		for _, key := range keys {
			for _, dimension := range []string{
				"cache-v1", "1.26.5", "runner.os", "runner.arch", sumExpression,
				"github.sha", "github.run_id", "github.run_attempt",
			} {
				if !strings.Contains(key, dimension) {
					t.Errorf("%s full key %q missing dimension %q", name, key, dimension)
				}
			}
			if !strings.Contains(key, "go-mod-") && !strings.Contains(key, "go-build-") {
				t.Errorf("%s full key %q has no cache-class namespace", name, key)
			}
			if !strings.Contains(key, "ci-test") && !strings.Contains(key, "security-") {
				t.Errorf("%s full key %q has no lane namespace", name, key)
			}
		}
	}
	assertFullKeys("ci.yml", ci, "hashFiles('go.sum')", 2)
	assertFullKeys("security.yml", security, "hashFiles('go.sum')", 6)

	for _, want := range []string{
		"key: ${{ steps.restore-go-mod-cache.outputs.cache-primary-key }}",
		"key: ${{ steps.restore-go-build-cache.outputs.cache-primary-key }}",
	} {
		if !strings.Contains(ci, want) {
			t.Errorf("ci.yml save missing matching restore primary key %q", want)
		}
	}
	for _, lane := range []string{"security-govulncheck", "security-codeql", "security-gosec"} {
		if got := strings.Count(security, lane); got != 2 {
			t.Errorf("security.yml lane %q occurrences = %d, want 2 full keys", lane, got)
		}
	}
	if got := strings.Count(security, "ci-test-${{ hashFiles('go.sum') }}-"); got != 6 {
		t.Errorf("security.yml digest-scoped ci-test restore prefixes = %d, want 6", got)
	}
	if got := strings.Count(security, "1.26.5-ci-test-"); got != 12 {
		t.Errorf("security.yml ci-test restore prefixes = %d, want 12", got)
	}

	for _, workflow := range []struct {
		name string
		text string
		want int
	}{
		{name: "ci.yml", text: ci, want: 2},
		{name: "security.yml", text: security, want: 6},
	} {
		if got := strings.Count(workflow.text, "cache-matched-key"); got != workflow.want {
			t.Errorf("%s restored-key reports = %d, want %d", workflow.name, got, workflow.want)
		}
		if got := strings.Count(workflow.text, " bytes=$bytes"); got != workflow.want {
			t.Errorf("%s byte-size reports = %d, want %d", workflow.name, got, workflow.want)
		}
	}

	if strings.Contains(ci, "cache-dependency-path: glade/go.sum") {
		t.Error("ci.yml retains setup-go cache-dependency-path")
	}
	if strings.Contains(security, "cache-dependency-path: go.sum") {
		t.Error("security.yml retains setup-go cache-dependency-path")
	}
	for _, want := range []string{
		"cache: npm",
		"third_party/lwc/package-lock.json",
		"site/package-lock.json",
		"contrib/vscode-glade/package-lock.json",
	} {
		if !strings.Contains(ciWorkflow, want) {
			t.Errorf("ci.yml npm cache block missing %q", want)
		}
	}
}

func TestSecurityWorkflowContract(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "security.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	if problem := securityWorkflowHeaderProblem(workflowText); problem != "" {
		t.Errorf("security.yml header contract: %s", problem)
	}
	jobs := workflowJobBlocks(t, workflowText)

	wantJobs := []string{"codeql", "dependency-review", "gosec", "govulncheck", "npm-audit", "scorecard"}
	gotJobs := make([]string, 0, len(jobs))
	for name := range jobs {
		gotJobs = append(gotJobs, name)
	}
	sort.Strings(gotJobs)
	if strings.Join(gotJobs, ",") != strings.Join(wantJobs, ",") {
		t.Fatalf("security.yml jobs = %v, want %v", gotJobs, wantJobs)
	}
	for _, jobName := range wantJobs {
		for _, marker := range []string{"runs-on: ubuntu-latest", "uses: actions/checkout@v6"} {
			if !strings.Contains(jobs[jobName], marker) {
				t.Errorf("%s job missing preserved marker %q", jobName, marker)
			}
		}
	}

	const gosecPin = "securego/gosec@9e6a9843d7a4a6e3e9a8539b02612c8a4aa3f889 # v2.27.1"
	if count := strings.Count(workflowText, gosecPin); count != 1 {
		t.Fatalf("security.yml exact gosec pin count = %d, want 1", count)
	}
	if strings.Contains(workflowText, "securego/gosec@v2") {
		t.Fatal("security.yml must not use the moving securego/gosec@v2 tag")
	}

	const scorecardPin = "ossf/scorecard-action@4eaacf0543bb3f2c246792bd56e8cdeffafb205a # v2.4.3"
	if count := strings.Count(workflowText, scorecardPin); count != 1 {
		t.Fatalf("security.yml exact Scorecard pin count = %d, want 1", count)
	}
	if strings.Contains(workflowText, "ossf/scorecard-action@v2") {
		t.Fatal("security.yml must not use the moving ossf/scorecard-action@v2 tag")
	}

	codeql := jobs["codeql"]
	for _, want := range []string{
		"timeout-minutes: 60",
		"languages: go",
		"config: |",
		"- uses: security-extended",
		"id: go/allocation-size-overflow",
		"build-mode: manual",
		`run: go build -o "${RUNNER_TEMP}/glade-codeql" ./cmd/glade`,
		"github/codeql-action/init@v4",
		"github/codeql-action/analyze@v4",
	} {
		if !strings.Contains(codeql, want) {
			t.Errorf("codeql job missing %q", want)
		}
	}
	if strings.Contains(codeql, "queries: +security-extended") {
		t.Error("codeql job must use inline config so the pathological allocation-size query can be filtered")
	}
	if strings.Contains(codeql, "github/codeql-action/autobuild@") {
		t.Error("codeql job must not use autobuild")
	}

	for _, jobName := range []string{"govulncheck", "codeql", "gosec"} {
		setupGo := workflowStepBlock(t, jobs[jobName], "uses: actions/setup-go@v6")
		if !strings.Contains(setupGo, "cache: false") {
			t.Errorf("%s setup-go step missing cache: false", jobName)
		}
		if strings.Contains(setupGo, "cache-dependency-path:") {
			t.Errorf("%s setup-go step retains cache-dependency-path", jobName)
		}
	}
	if count := strings.Count(workflowText, "uses: actions/setup-go@v6"); count != 3 {
		t.Errorf("security.yml setup-go step count = %d, want 3", count)
	}
	if strings.Contains(workflowText, "cache-dependency-path:") {
		t.Error("security.yml must not retain cache-dependency-path")
	}
	coverageMarkers := map[string][]string{
		"govulncheck":       {"go run golang.org/x/vuln/cmd/govulncheck@latest ./..."},
		"codeql":            {"actions: read", "contents: read", "security-events: write"},
		"gosec":             {"actions: read", "contents: read", "security-events: write", gosecPin, "args: -no-fail -fmt sarif -out gosec.sarif ./...", "github/codeql-action/upload-sarif@v4", "sarif_file: gosec.sarif"},
		"npm-audit":         {"actions/setup-node@v6", `node-version: "22"`, "npm audit --omit=dev --audit-level=high", "working-directory: third_party/lwc", "working-directory: contrib/vscode-glade"},
		"dependency-review": {"if: github.event_name == 'pull_request'", "contents: read", "actions/dependency-review-action@v5", "fail-on-severity: high"},
		"scorecard":         {"actions: read", "contents: read", "id-token: write", "security-events: write", scorecardPin, "results_file: scorecard.sarif", "results_format: sarif", "publish_results: true", "github/codeql-action/upload-sarif@v4", "sarif_file: scorecard.sarif"},
	}
	for jobName, markers := range coverageMarkers {
		for _, marker := range markers {
			if !strings.Contains(jobs[jobName], marker) {
				t.Errorf("%s job missing preserved marker %q", jobName, marker)
			}
		}
	}
}

func TestSecurityWorkflowHeaderRejectsNestedPermissionAndTriggerMutations(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "security.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	mutations := map[string]string{
		"job permission substitute": strings.Replace(workflowText, "\npermissions:\n  contents: read\n\njobs:\n", "\njobs:\n", 1),
		"changed trigger":           strings.Replace(workflowText, "  pull_request:\n", "  pull_request_target:\n", 1),
	}
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if mutated == workflowText {
				t.Fatal("test fixture did not mutate workflow header")
			}
			if !strings.Contains(mutated, "contents: read") {
				t.Fatal("test mutation unexpectedly removed all job-level contents permissions")
			}
			if problem := securityWorkflowHeaderProblem(mutated); problem == "" {
				t.Fatal("header contract accepted invalid top-level workflow structure")
			}
		})
	}
}

func securityWorkflowHeaderProblem(workflow string) string {
	header, _, found := strings.Cut(workflow, "\njobs:\n")
	if !found {
		return "missing top-level jobs boundary"
	}
	blocks, problem := workflowTopLevelBlocks(header)
	if problem != "" {
		return problem
	}
	if len(blocks) != 3 {
		return fmt.Sprintf("top-level header block count = %d, want 3", len(blocks))
	}
	wantBlocks := map[string]string{
		"name": "name: Security",
		"on": `on:
  pull_request:
  push:
    branches:
      - main
  schedule:
    - cron: "17 9 * * 1"
  workflow_dispatch:`,
		"permissions": `permissions:
  contents: read`,
	}
	for name, want := range wantBlocks {
		got, ok := blocks[name]
		if !ok {
			return fmt.Sprintf("missing top-level %s block", name)
		}
		if got != want {
			return fmt.Sprintf("top-level %s block does not match required structure", name)
		}
	}
	return ""
}

func workflowTopLevelBlocks(header string) (map[string]string, string) {
	lines := strings.Split(header, "\n")
	blocks := make(map[string]string)
	for i := 0; i < len(lines); {
		if lines[i] == "" {
			i++
			continue
		}
		if strings.HasPrefix(lines[i], " ") {
			return nil, fmt.Sprintf("unexpected indented header line %q", lines[i])
		}
		name, _, ok := strings.Cut(lines[i], ":")
		if !ok || name == "" {
			return nil, fmt.Sprintf("invalid top-level header line %q", lines[i])
		}
		start := i
		i++
		for i < len(lines) && (lines[i] == "" || strings.HasPrefix(lines[i], " ")) {
			i++
		}
		if _, duplicate := blocks[name]; duplicate {
			return nil, fmt.Sprintf("duplicate top-level %s block", name)
		}
		blocks[name] = strings.TrimSpace(strings.Join(lines[start:i], "\n"))
	}
	return blocks, ""
}

func workflowJobBlocks(t *testing.T, workflow string) map[string]string {
	t.Helper()
	lines := strings.Split(workflow, "\n")
	jobsLine := -1
	for i, line := range lines {
		if line == "jobs:" {
			jobsLine = i
			break
		}
	}
	if jobsLine < 0 {
		t.Fatal("security.yml missing top-level jobs block")
	}

	blocks := make(map[string]string)
	for i := jobsLine + 1; i < len(lines); {
		line := lines[i]
		if line == "" || strings.HasPrefix(line, "  ") {
			if len(line) >= 3 && strings.HasPrefix(line, "  ") && line[2] != ' ' && strings.HasSuffix(line, ":") {
				name := strings.TrimSuffix(strings.TrimSpace(line), ":")
				start := i
				i++
				for i < len(lines) && (lines[i] == "" || strings.HasPrefix(lines[i], "    ")) {
					i++
				}
				blocks[name] = strings.Join(lines[start:i], "\n")
				continue
			}
			i++
			continue
		}
		break
	}
	return blocks
}

func workflowStepBlock(t *testing.T, jobBlock, marker string) string {
	t.Helper()
	lines := strings.Split(jobBlock, "\n")
	for i, line := range lines {
		if !strings.Contains(line, marker) {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		start := i
		for start > 0 {
			previous := lines[start-1]
			previousIndent := len(previous) - len(strings.TrimLeft(previous, " "))
			if strings.HasPrefix(strings.TrimSpace(previous), "- ") && previousIndent == indent {
				start--
				break
			}
			if previousIndent < indent {
				break
			}
			start--
		}
		end := i + 1
		for end < len(lines) {
			next := lines[end]
			nextIndent := len(next) - len(strings.TrimLeft(next, " "))
			if strings.HasPrefix(strings.TrimSpace(next), "- ") && nextIndent <= indent {
				break
			}
			end++
		}
		return strings.Join(lines[start:end], "\n")
	}
	t.Fatalf("job block missing step marker %q", marker)
	return ""
}

func readCIWorkflow(t *testing.T) (string, map[string]string) {
	t.Helper()
	path := filepath.Join("..", ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	workflow := string(data)
	return workflow, workflowJobBlocks(t, workflow)
}

const benchmarkCachePairInput = `  workflow_dispatch:
    inputs:
      benchmark_cache_pair:
        description: "Cache pair (0 normal; 1-999999 isolated benchmark)"
        required: true
        default: 0
        type: number`

const benchmarkCachePairPrefix = `${{ github.event_name == 'workflow_dispatch' && inputs.benchmark_cache_pair > 0 && inputs.benchmark_cache_pair <= 999999 && format('w2-6d-{0}-', inputs.benchmark_cache_pair) || '' }}`

func readWorkflowFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", ".github", "workflows", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func explicitWorkflowCacheKeys(workflow string) []string {
	lines := strings.Split(workflow, "\n")
	var keys []string
	restoreIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if restoreIndent >= 0 {
			if trimmed == "" {
				continue
			}
			if indent > restoreIndent {
				keys = append(keys, trimmed)
				continue
			}
			restoreIndent = -1
		}
		if trimmed == "restore-keys: |" {
			restoreIndent = indent
			continue
		}
		if !strings.HasPrefix(trimmed, "key: ") {
			continue
		}
		key := strings.TrimPrefix(trimmed, "key: ")
		if strings.Contains(key, ".outputs.cache-primary-key") {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func workflowIfConditionCount(workflow, condition string) int {
	count := 0
	for _, line := range strings.Split(workflow, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "if: ") && strings.Contains(line, condition) {
			count++
		}
	}
	return count
}

func workflowStepBlocksText(job string) []string {
	var steps []string
	for searchAt := 0; searchAt < len(job); {
		relativeStart := strings.Index(job[searchAt:], "\n      - ")
		if relativeStart < 0 {
			break
		}
		start := searchAt + relativeStart + 1
		end := len(job)
		if relativeEnd := strings.Index(job[start+1:], "\n      - "); relativeEnd >= 0 {
			end = start + 1 + relativeEnd
		}
		steps = append(steps, job[start:end])
		searchAt = end
	}
	return steps
}

func workflowCacheAccessSteps(job string) []string {
	var cacheSteps []string
	for _, step := range workflowStepBlocksText(job) {
		explicitCache := strings.Contains(step, "uses: actions/cache")
		setupNodeCache := strings.Contains(step, "uses: actions/setup-node@") &&
			strings.Contains(step, "\n          cache:")
		if explicitCache || setupNodeCache {
			cacheSteps = append(cacheSteps, step)
		}
	}
	return cacheSteps
}

func workflowFirstCacheAccessAt(job string) int {
	first := -1
	for _, step := range workflowCacheAccessSteps(job) {
		at := strings.Index(job, step)
		if first < 0 || at < first {
			first = at
		}
	}
	return first
}

func moveWorkflowStepBeforeValidation(t *testing.T, workflow, jobName, step, validationCommand string) string {
	t.Helper()
	job := workflowJobBlocksText(workflow)[jobName]
	if job == "" {
		t.Fatalf("workflow missing job %q", jobName)
	}
	if strings.Count(job, step) != 1 {
		t.Fatalf("%s cache step count = %d, want 1", jobName, strings.Count(job, step))
	}
	validationAt := strings.Index(job, validationCommand)
	if validationAt < 0 {
		t.Fatalf("%s missing validation command", jobName)
	}
	validationStepStart := strings.LastIndex(job[:validationAt], "\n      - ")
	if validationStepStart < 0 {
		t.Fatalf("%s validation command is outside a workflow step", jobName)
	}
	validationStepStart++
	withoutStep := strings.Replace(job, step, "", 1)
	validationAt = strings.Index(withoutStep, validationCommand)
	validationStepStart = strings.LastIndex(withoutStep[:validationAt], "\n      - ") + 1
	mutatedJob := withoutStep[:validationStepStart] + step + "\n" + withoutStep[validationStepStart:]
	job = strings.TrimSuffix(job, "\n")
	mutatedJob = strings.TrimSuffix(mutatedJob, "\n")
	if strings.Count(workflow, job) != 1 {
		t.Fatalf("%s workflow job block count = %d, want 1", jobName, strings.Count(workflow, job))
	}
	return strings.Replace(workflow, job, mutatedJob, 1)
}

func benchmarkCachePairWorkflowProblem(name, workflow string) string {
	if strings.Count(workflow, benchmarkCachePairInput) != 1 {
		return "workflow_dispatch benchmark_cache_pair input is not exact"
	}
	keys := explicitWorkflowCacheKeys(workflow)
	if len(keys) == 0 {
		return "no explicit cache keys found"
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, benchmarkCachePairPrefix) {
			return fmt.Sprintf("explicit cache key is not pair-prefixed: %q", key)
		}
	}
	wantOpaqueNPM := 1
	wantBenchmarkNPM := 1
	if name == "ci.yml" {
		wantOpaqueNPM = 3
		wantBenchmarkNPM = 3
	}
	if got := strings.Count(workflow, "          cache: npm"); got != wantOpaqueNPM {
		return fmt.Sprintf("setup-node opaque npm caches = %d, want %d normal-only caches", got, wantOpaqueNPM)
	}
	if got := workflowIfConditionCount(workflow, "inputs.benchmark_cache_pair == 0"); got != wantOpaqueNPM {
		return fmt.Sprintf("normal-only setup-node conditions = %d, want %d", got, wantOpaqueNPM)
	}
	if got := strings.Count(workflow, "      - name: Set up Node for benchmark cache pair"); got != wantBenchmarkNPM {
		return fmt.Sprintf("benchmark setup-node steps = %d, want %d", got, wantBenchmarkNPM)
	}
	if got := strings.Count(workflow, "      - name: Restore and save benchmark npm cache"); got != wantBenchmarkNPM {
		return fmt.Sprintf("explicit benchmark npm caches = %d, want %d", got, wantBenchmarkNPM)
	}
	if got := strings.Count(workflow, "      - name: Resolve benchmark npm cache path"); got != wantBenchmarkNPM {
		return fmt.Sprintf("benchmark npm cache path resolvers = %d, want %d", got, wantBenchmarkNPM)
	}
	if got := strings.Count(workflow, "          path: ${{ steps.benchmark-npm-cache.outputs.path }}"); got != wantBenchmarkNPM {
		return fmt.Sprintf("explicit benchmark npm cache paths = %d, want %d", got, wantBenchmarkNPM)
	}
	if got := workflowIfConditionCount(workflow, "inputs.benchmark_cache_pair > 0"); got != wantBenchmarkNPM*3 {
		return fmt.Sprintf("positive-pair benchmark npm conditions = %d, want %d", got, wantBenchmarkNPM*3)
	}
	if name == "browser.yml" {
		if strings.Count(workflow, "        if: steps.scope.outputs.run_expensive == 'true' && (github.event_name != 'workflow_dispatch' || inputs.benchmark_cache_pair == 0)") != 1 {
			return "browser normal setup-node condition is not exact"
		}
		if strings.Count(workflow, "        if: steps.scope.outputs.run_expensive == 'true' && github.event_name == 'workflow_dispatch' && inputs.benchmark_cache_pair > 0") != 3 {
			return "browser benchmark npm conditions are not exact"
		}
		browser := workflowJobBlocksText(workflow)["browser"]
		validation := `source/scripts/ci-benchmark-cache-pair.sh "${{ inputs.benchmark_cache_pair }}"`
		if strings.Count(browser, validation) != 1 {
			return "browser benchmark cache-pair validation command is not exact"
		}
		checkoutAt := strings.Index(browser, "      - uses: actions/checkout@v6")
		validationAt := strings.Index(browser, validation)
		cacheAt := workflowFirstCacheAccessAt(browser)
		if checkoutAt < 0 || validationAt <= checkoutAt || cacheAt <= validationAt {
			return "browser cache-pair validation must run after checkout and before cache access"
		}
		return ""
	}
	jobs := workflowJobBlocksText(workflow)
	if strings.Count(workflow, "        if: github.event_name != 'workflow_dispatch' || inputs.benchmark_cache_pair == 0") != wantOpaqueNPM {
		return "CI normal setup-node conditions are not exact"
	}
	if strings.Count(workflow, "        if: github.event_name == 'workflow_dispatch' && inputs.benchmark_cache_pair > 0") != wantBenchmarkNPM*3 {
		return "CI benchmark npm conditions are not exact"
	}
	guard := "    if: ${{ github.event_name != 'workflow_dispatch' || (inputs.benchmark_cache_pair >= 0 && inputs.benchmark_cache_pair <= 999999) }}"
	for _, jobName := range []string{
		"site", "vet", "gladecli", "node-integration", "sema", "server-and-playground",
		"test", "smoke-runtime", "smoke-distribution", "apextest",
	} {
		job := jobs[jobName]
		if strings.Count(job, guard) != 1 {
			return fmt.Sprintf("%s cache-pair bounds guard is not exact", jobName)
		}
		validation := `scripts/ci-benchmark-cache-pair.sh "${{ inputs.benchmark_cache_pair }}"`
		if strings.Count(job, validation) != 1 {
			return fmt.Sprintf("%s benchmark cache-pair validation command is not exact", jobName)
		}
		checkoutAt := strings.Index(job, "      - uses: actions/checkout@v6")
		validationAt := strings.Index(job, validation)
		cacheAt := workflowFirstCacheAccessAt(job)
		if checkoutAt < 0 || validationAt <= checkoutAt || cacheAt <= validationAt {
			return fmt.Sprintf("%s cache-pair validation must run after checkout and before cache access", jobName)
		}
	}
	if strings.Count(jobs["sema-full"], "    if: ${{ github.event_name == 'schedule' }}") != 1 {
		return "sema-full schedule condition changed"
	}
	return ""
}

func TestCIBenchmarkCachePairContract(t *testing.T) {
	workflows := map[string]string{
		"ci.yml":      readWorkflowFile(t, "ci.yml"),
		"browser.yml": readWorkflowFile(t, "browser.yml"),
	}
	for name, workflow := range workflows {
		t.Run(name, func(t *testing.T) {
			if problem := benchmarkCachePairWorkflowProblem(name, workflow); problem != "" {
				t.Fatal(problem)
			}
		})
	}
	if gotCI, gotBrowser := strings.Count(workflows["ci.yml"], benchmarkCachePairInput), strings.Count(workflows["browser.yml"], benchmarkCachePairInput); gotCI != gotBrowser {
		t.Fatalf("workflow_dispatch benchmark_cache_pair input count differs: ci=%d browser=%d", gotCI, gotBrowser)
	}
}

func TestCIBenchmarkCachePairContractRejectsAnyCacheBeforeValidation(t *testing.T) {
	for _, tc := range []struct {
		workflowName      string
		jobNames          []string
		validationCommand string
	}{
		{
			workflowName:      "browser.yml",
			jobNames:          []string{"browser"},
			validationCommand: `source/scripts/ci-benchmark-cache-pair.sh "${{ inputs.benchmark_cache_pair }}"`,
		},
		{
			workflowName: "ci.yml",
			jobNames: []string{
				"site", "vet", "gladecli", "node-integration", "sema", "server-and-playground",
				"test", "smoke-runtime", "smoke-distribution", "apextest",
			},
			validationCommand: `scripts/ci-benchmark-cache-pair.sh "${{ inputs.benchmark_cache_pair }}"`,
		},
	} {
		workflow := readWorkflowFile(t, tc.workflowName)
		for _, jobName := range tc.jobNames {
			job := workflowJobBlocksText(workflow)[jobName]
			cacheSteps := workflowCacheAccessSteps(job)
			if len(cacheSteps) == 0 {
				t.Fatalf("%s/%s has no cache access steps", tc.workflowName, jobName)
			}
			for index, cacheStep := range cacheSteps {
				t.Run(fmt.Sprintf("%s/%s/cache-%d", tc.workflowName, jobName, index), func(t *testing.T) {
					mutated := moveWorkflowStepBeforeValidation(t, workflow, jobName, cacheStep, tc.validationCommand)
					mutatedJob := workflowJobBlocksText(mutated)[jobName]
					cacheAt := workflowFirstCacheAccessAt(mutatedJob)
					validationAt := strings.Index(mutatedJob, tc.validationCommand)
					if cacheAt < 0 || cacheAt >= validationAt {
						t.Fatalf("mutation did not put cache access before validation: cache=%d validation=%d", cacheAt, validationAt)
					}
					if problem := benchmarkCachePairWorkflowProblem(tc.workflowName, mutated); problem == "" {
						t.Fatal("workflow accepted cache access before benchmark cache-pair validation")
					}
				})
			}
		}
	}
}

func TestCIBenchmarkCachePairValidator(t *testing.T) {
	script := "./ci-benchmark-cache-pair.sh"
	for _, tc := range []struct {
		value   string
		wantErr bool
	}{
		{value: "0"},
		{value: "1"},
		{value: "999999"},
		{value: "", wantErr: true},
		{value: "-1", wantErr: true},
		{value: "1.5", wantErr: true},
		{value: "1000000", wantErr: true},
		{value: "not-a-number", wantErr: true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			cmd := exec.Command(script, tc.value)
			out, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantErr {
				t.Fatalf("validator error = %v, wantErr %v\n%s", err, tc.wantErr, out)
			}
		})
	}
}

func ciWorkflowConcurrencyProblem(workflow string) string {
	header, _, found := strings.Cut(workflow, "\njobs:\n")
	if !found {
		return "missing top-level jobs boundary"
	}
	blocks, problem := workflowTopLevelBlocks(header)
	if problem != "" {
		return problem
	}
	want := `concurrency:
  group: ci-${{ github.event.pull_request.number || github.run_id }}
  cancel-in-progress: true`
	if blocks["concurrency"] != want {
		return "top-level concurrency block does not match required structure"
	}
	return ""
}

func ciRequiredAggregateProblem(workflow string) string {
	jobs := workflowJobBlocks(&testing.T{}, workflow)
	job, ok := jobs["required-ci"]
	if !ok || strings.Count(workflow, "\n  required-ci:\n") != 1 {
		return "missing single required-ci job"
	}
	wantNeeds := `    needs:
      - site
      - vet
      - apextest
      - apextest-history
      - gladecli
      - node-integration
      - sema
      - sema-history
      - server-and-playground
      - test
      - smoke-runtime
      - smoke-distribution`
	needsStart := strings.Index(job, "    needs:\n")
	if needsStart < 0 {
		return "required-ci missing needs block"
	}
	needsLines := strings.Split(job[needsStart:], "\n")
	needsEnd := len(needsLines)
	for i, line := range needsLines[1:] {
		if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     ") {
			needsEnd = i + 1
			break
		}
	}
	if got := strings.Join(needsLines[:needsEnd], "\n"); got != wantNeeds {
		return "required-ci needs do not exactly match required jobs"
	}
	for _, forbidden := range []struct {
		name   string
		marker string
	}{
		{name: "job continue-on-error", marker: "\n    continue-on-error:"},
		{name: "step continue-on-error", marker: "\n        continue-on-error:"},
		{name: "step condition", marker: "\n        if:"},
	} {
		if strings.Contains(job, forbidden.marker) {
			return "required-ci must not define " + forbidden.name
		}
	}
	checks := []struct {
		name string
		want string
	}{
		{name: "name", want: "    name: Required CI"},
		{name: "needs", want: wantNeeds},
		{name: "always condition", want: "    if: always()"},
		{name: "runner", want: "    runs-on: ubuntu-latest"},
		{name: "timeout", want: "    timeout-minutes: 5"},
		{name: "needs JSON environment", want: `          REQUIRED_RESULTS: ${{ toJSON(needs) }}`},
		{name: "fail-closed predicate", want: `select(.value.result != "success")`},
		{name: "failure exit", want: "            exit 1"},
	}
	for _, check := range checks {
		if strings.Count(job, check.want) != 1 {
			return fmt.Sprintf("required-ci %s marker count = %d, want 1", check.name, strings.Count(job, check.want))
		}
	}
	_, steps, found := strings.Cut(job, "    steps:\n")
	if !found || strings.Count(steps, "      - ") != 1 {
		return fmt.Sprintf("required-ci step count = %d, want 1", strings.Count(steps, "      - "))
	}
	return ""
}

func requiredAggregateShell(t *testing.T, job string) string {
	t.Helper()
	marker := "        run: |\n"
	_, script, found := strings.Cut(job, marker)
	if !found {
		t.Fatal("required-ci missing shell run block")
	}
	var lines []string
	for _, line := range strings.Split(script, "\n") {
		if line == "" {
			lines = append(lines, "")
			continue
		}
		if !strings.HasPrefix(line, "          ") {
			break
		}
		lines = append(lines, strings.TrimPrefix(line, "          "))
	}
	return strings.Join(lines, "\n")
}

func TestCIWorkflowConcurrencyContract(t *testing.T) {
	workflow, _ := readCIWorkflow(t)
	if problem := ciWorkflowConcurrencyProblem(workflow); problem != "" {
		t.Fatal(problem)
	}
}

func TestCISemaShardWorkflowContract(t *testing.T) {
	workflow, jobs := readCIWorkflow(t)
	if !strings.Contains(workflow, "  schedule:\n    - cron: '17 9 * * 1'") {
		t.Fatal("CI workflow is missing the weekly sema full-oracle schedule")
	}
	sema := jobs["sema"]
	for _, marker := range []string{
		"fail-fast: false",
		"shard: [0, 1]",
		`scripts/ci-go-test.sh sema-shard "${{ matrix.shard }}"`,
		"name: sema-shard-${{ matrix.shard }}",
	} {
		if !strings.Contains(sema, marker) {
			t.Errorf("sema matrix contract missing %q", marker)
		}
	}
	for _, name := range []string{"sema-history", "sema-full", "sema-equivalence"} {
		if _, ok := jobs[name]; !ok {
			t.Errorf("CI workflow is missing %s job", name)
		}
	}
	for _, tc := range []struct {
		name string
		job  string
		want []string
	}{
		{name: "matrix", job: sema, want: []string{
			"GOMODCACHE: /tmp/go-mod-ci-sema-${{ matrix.shard }}",
			"GOCACHE: /tmp/go-build-ci-sema-${{ matrix.shard }}",
		}},
		{name: "full", job: jobs["sema-full"], want: []string{
			"GOMODCACHE: /tmp/go-mod-ci-sema-full",
			"GOCACHE: /tmp/go-build-ci-sema-full",
		}},
	} {
		t.Run(tc.name+" cache paths", func(t *testing.T) {
			if strings.Contains(tc.job, "runner.temp") {
				t.Fatal("job-level env uses unavailable runner context")
			}
			for _, marker := range tc.want {
				if strings.Count(tc.job, marker) != 1 {
					t.Errorf("cache path marker %q count = %d, want 1", marker, strings.Count(tc.job, marker))
				}
			}
		})
	}
	if strings.Contains(jobs["required-ci"], "sema-full") || strings.Contains(jobs["required-ci"], "sema-equivalence") || strings.Contains(jobs["required-ci"], "required-scheduled-ci") {
		t.Fatal("normal Required CI depends on a scheduled-only job")
	}
	scheduled := jobs["required-scheduled-ci"]
	for _, marker := range []string{
		"name: Required Scheduled CI",
		"      - required-ci",
		"      - sema-full",
		"      - sema-equivalence",
		"if: ${{ always() && github.event_name == 'schedule' }}",
		`select(.value.result != "success")`,
	} {
		if !strings.Contains(scheduled, marker) {
			t.Errorf("scheduled aggregate contract missing %q", marker)
		}
	}
	if strings.Contains(scheduled, "continue-on-error:") {
		t.Fatal("scheduled aggregate is allowed to continue on error")
	}
}

type browserEvidenceFile struct {
	name    string
	command string
}

func browserEvidenceFiles() []browserEvidenceFile {
	return []browserEvidenceFile{
		{name: "npm-ci-third-party-lwc.log", command: `printf 'not run\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/npm-ci-third-party-lwc.log"`},
		{name: "npm-ci-lwcruntime.log", command: `printf 'not run\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/npm-ci-lwcruntime.log"`},
		{name: "playwright-install.log", command: `printf 'not run\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/playwright-install.log"`},
		{name: "node-test.log", command: `printf 'not run\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/node-test.log"`},
		{name: "go-test.json", command: `printf 'not run\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/go-test.json"`},
		{name: "resource-usage.txt", command: `printf 'not run\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/resource-usage.txt"`},
		{name: "validation-summary.json", command: `printf '{"scope":"pending"}\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/validation-summary.json"`},
	}
}

func browserWorkflowProblem(workflow string) string {
	required := []string{
		"name: Browser",
		"permissions:\n  contents: read",
		"  pull_request:",
		"  push:\n    branches: [main]\n    tags: ['v*']",
		"  schedule:\n    - cron: '17 11 * * *'",
		"  workflow_dispatch:",
		"jobs:\n  browser:",
		"name: Browser",
		"runs-on: ubuntu-latest",
		"timeout-minutes: 30",
		"GOMAXPROCS: \"2\"",
		"GLADE_LWC_BROWSER: \"1\"",
		"PLAYWRIGHT_BROWSERS_PATH: /home/runner/.cache/ms-playwright",
		"uses: actions/checkout@v6",
		"fetch-depth: 0",
		"persist-credentials: false",
		"path: source",
		"working-directory: source",
		"github.event.pull_request.base.sha",
		"github.event.pull_request.head.sha",
		`changed_paths_file=$(mktemp "$RUNNER_TEMP/browser-changed-paths.XXXXXX")`,
		`trap 'rm -f "$changed_paths_file"' EXIT`,
		`if ! git diff --name-only -z --no-renames --diff-filter=ACDMRTUXB "$BASE_SHA" "$HEAD_SHA" >"$changed_paths_file"; then`,
		`while IFS= read -r -d '' changed_path; do`,
		`done <"$changed_paths_file"`,
		`if [[ -z "$BASE_SHA" || -z "$HEAD_SHA" ]]; then
              echo "pull request base/head SHA is missing" >&2
              exit 1`,
		`echo "git diff failed for $BASE_SHA..$HEAD_SHA" >&2
              exit 1`,
		"exit 1",
		"*.go",
		"go.mod|go.sum",
		"lwcruntime/**",
		"third_party/lwc/**",
		"testdata/local-tests/lwc-shell/**",
		"testdata/local-tests/lightning-out-vf/**",
		".github/workflows/browser.yml",
		"github.event_name != 'pull_request'",
		"run_expensive=$UNCONDITIONAL",
		"uses: actions/setup-go@v6",
		"go-version: \"1.26.5\"",
		"cache: false",
		"uses: actions/setup-node@v6",
		"node-version: \"22\"",
		"third_party/lwc/package-lock.json",
		"lwcruntime/package-lock.json",
		`npm ci --prefix third_party/lwc 2>&1 | tee "$GITHUB_WORKSPACE/ci-artifacts/browser/npm-ci-third-party-lwc.log"`,
		`npm ci --prefix lwcruntime 2>&1 | tee "$GITHUB_WORKSPACE/ci-artifacts/browser/npm-ci-lwcruntime.log"`,
		`npm exec --prefix lwcruntime -- playwright install --with-deps chromium 2>&1 | tee "$GITHUB_WORKSPACE/ci-artifacts/browser/playwright-install.log"`,
		`printf '{"scope":"%s"}\n' "$run_expensive" >"$GITHUB_WORKSPACE/ci-artifacts/browser/validation-summary.json"`,
		"timeout-minutes: 10",
		"uses: actions/cache/restore@v4",
		"uses: actions/cache/save@v4",
		"go-mod-ci-browser",
		"go-build-ci-browser",
		"playwright-browser",
		"ci-test-${{ hashFiles('source/go.sum') }}-",
		"${{ runner.os }}-${{ runner.arch }}",
		"${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
		"continue-on-error: true",
		"set -o pipefail",
		"REAL_NPM=$(command -v npm)",
		`: >"$BROWSER_NODE_TEST_LOG"`,
		`cat >"$RUNNER_TEMP/browser-bin/npm" <<'WRAPPER'`,
		"<<'WRAPPER'\n          #!/usr/bin/env bash\n          set -o pipefail",
		`"$REAL_NPM" "$@" 2>&1 | tee -a "$BROWSER_NODE_TEST_LOG"`,
		`npm_pipeline_status=("${PIPESTATUS[@]}")`,
		`npm_rc=${npm_pipeline_status[0]}`,
		`npm_tee_rc=${npm_pipeline_status[1]}`,
		`if (( npm_rc != 0 )); then`,
		`if (( npm_tee_rc != 0 )); then`,
		`exit "$npm_rc"`,
		`exit "$npm_tee_rc"`,
		`export PATH="$RUNNER_TEMP/browser-bin:$PATH"`,
		`/usr/bin/time -v -o "$GITHUB_WORKSPACE/ci-artifacts/browser/resource-usage.txt"`,
		"go test -json -vet=off -p=1 -count=1 -timeout=25m",
		"-run '^(TestBrowserRuntimeSuite|TestGeneratedPhase3BaseComponentsRunInBrowser)$'",
		"./internal/lwcruntime ./internal/lwcbrowser",
		`tee "$GITHUB_WORKSPACE/ci-artifacts/browser/go-test.json"`,
		"PIPESTATUS[@]",
		"native_rc",
		"pipeline_rc",
		"validation-summary.json",
		`artifact_dir = Path(os.environ["GITHUB_WORKSPACE"]) / "ci-artifacts" / "browser"`,
		`node_events_path = artifact_dir / "node-test.log"`,
		"TestBrowserRuntimeSuite",
		"TestGeneratedPhase3BaseComponentsRunInBrowser",
		`action in {"pass", "fail", "skip"}`,
		`if action == "skip":`,
		`elif actions[0] != "pass":`,
		"Go test stream reported skip events",
		"duplicate terminal",
		"missing terminal",
		"unexpected top-level terminal",
		`if node_skips:`,
		`node_skips = re.findall(r"(?im)^[ \t]*(?:ok|not ok)[ \t]+[0-9]+\b.*(?<!\\)#[ \t]*SKIP\b.*$", node_text)`,
		`Node TAP reported numbered skip points`,
		`node_failures = re.findall(r"(?im)^[ \t]*not ok[ \t]+[0-9]+\b.*$", node_text)`,
		`if node_failures:`,
		`Node TAP reported numbered failing points`,
		`Node TAP reported cannot-launch text`,
		`if "cannot launch chromium" in node_text.lower():`,
		`for field in ("tests", "pass", "fail", "skipped"):`,
		`tap["tests"] <= 0`,
		`tap["pass"] != tap["tests"]`,
		`tap["fail"] != 0`,
		`tap["skipped"] != 0`,
		"if (( native_rc != 0 ))",
		"if (( pipeline_rc != 0 ))",
		"if (( validator_rc != 0 ))",
		"if: always()",
		"uses: actions/upload-artifact@v6",
		"name: browser-${{ github.run_id }}-${{ github.run_attempt }}",
		"path: ci-artifacts/browser/**",
		"if-no-files-found: error",
		"retention-days: 7",
	}
	for _, marker := range required {
		if !strings.Contains(workflow, marker) {
			return fmt.Sprintf("missing %q", marker)
		}
	}
	for _, forbidden := range []string{
		"pull_request_target:",
		"permissions: write",
		"contents: write",
		"secrets.",
		"continue-on-error: true\n        run: go test",
	} {
		if strings.Contains(workflow, forbidden) {
			return fmt.Sprintf("contains forbidden %q", forbidden)
		}
	}
	if strings.Contains(workflowHeader(workflow), "paths:") {
		return "workflow-level paths filter is forbidden"
	}
	jobs := workflowJobBlocksText(workflow)
	if len(jobs) != 1 || jobs["browser"] == "" {
		return fmt.Sprintf("jobs = %v, want only browser", reflect.ValueOf(jobs).MapKeys())
	}
	job := jobs["browser"]
	_, steps, found := strings.Cut(job, "    steps:\n")
	if !found || !strings.HasPrefix(steps, "      - name: Precreate browser evidence\n") {
		return "browser evidence precreation is not the first job step"
	}
	precreate := workflowStepBlockText(job, "      - name: Precreate browser evidence")
	if precreate == "" {
		return "browser evidence precreation step is missing"
	}
	var expectedPrecreate strings.Builder
	expectedPrecreate.WriteString("      - name: Precreate browser evidence\n")
	expectedPrecreate.WriteString("        run: |\n")
	expectedPrecreate.WriteString(`          mkdir -p "$GITHUB_WORKSPACE/ci-artifacts/browser"` + "\n")
	for _, evidence := range browserEvidenceFiles() {
		expectedPrecreate.WriteString("          ")
		expectedPrecreate.WriteString(evidence.command)
		expectedPrecreate.WriteByte('\n')
	}
	if strings.TrimSpace(precreate) != strings.TrimSpace(expectedPrecreate.String()) {
		return fmt.Sprintf("browser evidence precreation body is not exact\ngot:\n%s\nwant:\n%s", precreate, expectedPrecreate.String())
	}
	for _, evidence := range browserEvidenceFiles() {
		if strings.Count(workflow, evidence.command) != 1 {
			return fmt.Sprintf("evidence %s creation count = %d, want 1", evidence.name, strings.Count(workflow, evidence.command))
		}
	}
	for _, later := range []string{
		"      - uses: actions/checkout@v6",
		"      - name: Determine browser test scope",
		"      - uses: actions/setup-go@v6",
		"      - name: Install LWC compiler dependencies",
		"      - name: Run browser authorities",
	} {
		if strings.Index(job, "      - name: Precreate browser evidence") >= strings.Index(job, later) || strings.Index(job, later) < 0 {
			return fmt.Sprintf("browser evidence precreation must precede %q", later)
		}
	}
	if strings.Count(workflow, "go test -json") != 1 {
		return fmt.Sprintf("native go test execution count = %d, want 1", strings.Count(workflow, "go test -json"))
	}
	if strings.Count(workflow, "working-directory: source") != 5 {
		return fmt.Sprintf("source working-directory count = %d, want 5 repo-dependent run steps", strings.Count(workflow, "working-directory: source"))
	}
	if strings.Count(workflow, "uses: actions/cache/save@v4") != 3 {
		return fmt.Sprintf("cache save count = %d, want 3", strings.Count(workflow, "uses: actions/cache/save@v4"))
	}
	if strings.Count(workflow, "if: steps.scope.outputs.run_expensive == 'true' && success()") != 3 {
		return fmt.Sprintf("success-only cache save conditions = %d, want 3", strings.Count(workflow, "if: steps.scope.outputs.run_expensive == 'true' && success()"))
	}
	if strings.Count(workflow, "continue-on-error: true") != 7 {
		return fmt.Sprintf("continue-on-error count = %d, want 7 cache operations only", strings.Count(workflow, "continue-on-error: true"))
	}
	for marker, want := range map[string]int{
		"          REAL_NPM=$(command -v npm)\n":                                              1,
		"          export REAL_NPM\n":                                                         1,
		`export BROWSER_NODE_TEST_LOG="$GITHUB_WORKSPACE/ci-artifacts/browser/node-test.log"`: 1,
		`          : >"$BROWSER_NODE_TEST_LOG"` + "\n":                                        1,
		`cat >"$RUNNER_TEMP/browser-bin/npm" <<'WRAPPER'`:                                     1,
		`"$REAL_NPM" "$@" 2>&1 | tee -a "$BROWSER_NODE_TEST_LOG"`:                             1,
		`npm_pipeline_status=("${PIPESTATUS[@]}")`:                                            1,
		`npm_rc=${npm_pipeline_status[0]}`:                                                    1,
		`npm_tee_rc=${npm_pipeline_status[1]}`:                                                1,
		`export PATH="$RUNNER_TEMP/browser-bin:$PATH"`:                                        1,
		"go test -json -vet=off -p=1 -count=1 -timeout=25m":                                   1,
	} {
		if got := strings.Count(workflow, marker); got != want {
			return fmt.Sprintf("browser wrapper/authority marker %q count = %d, want %d", marker, got, want)
		}
	}
	authority := workflowStepBlockText(job, "      - name: Run browser authorities")
	wrapperSetup := "          export BROWSER_NODE_TEST_LOG=\"$GITHUB_WORKSPACE/ci-artifacts/browser/node-test.log\"\n" +
		"          : >\"$BROWSER_NODE_TEST_LOG\"\n" +
		"          mkdir -p \"$RUNNER_TEMP/browser-bin\""
	if strings.Count(authority, wrapperSetup) != 1 {
		return "Node evidence truncation must occur exactly once immediately before wrapper installation"
	}
	return ""
}

func workflowHeader(workflow string) string {
	if index := strings.Index(workflow, "\njobs:\n"); index >= 0 {
		return workflow[:index]
	}
	return workflow
}

func workflowJobBlocksText(workflow string) map[string]string {
	jobs := make(map[string]string)
	_, body, ok := strings.Cut(workflow, "\njobs:\n")
	if !ok {
		return jobs
	}
	var name string
	var block strings.Builder
	flush := func() {
		if name != "" {
			jobs[name] = block.String()
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(line, ":") {
			flush()
			name = strings.TrimSuffix(strings.TrimSpace(line), ":")
			block.Reset()
		}
		if name != "" {
			block.WriteString(line)
			block.WriteByte('\n')
		}
	}
	flush()
	return jobs
}

func workflowStepBlockText(job, header string) string {
	start := strings.Index(job, header)
	if start < 0 {
		return ""
	}
	tail := job[start:]
	if next := strings.Index(tail[len(header):], "\n      - "); next >= 0 {
		return tail[:len(header)+next]
	}
	return tail
}

func workflowRunScriptText(step string) string {
	_, body, ok := strings.Cut(step, "        run: |\n")
	if !ok {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if line == "" {
			lines = append(lines, "")
			continue
		}
		if !strings.HasPrefix(line, "          ") {
			break
		}
		lines = append(lines, strings.TrimPrefix(line, "          "))
	}
	return strings.Join(lines, "\n")
}

func TestCIBrowserWorkflowContract(t *testing.T) {
	path := filepath.Join("..", ".github", "workflows", "browser.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	workflow := string(data)
	if problem := browserWorkflowProblem(workflow); problem != "" {
		t.Fatal(problem)
	}

	mutations := map[string]func(string) string{
		"workflow-level paths": func(s string) string {
			return strings.Replace(s, "  pull_request:\n", "  pull_request:\n    paths: ['**.go']\n", 1)
		},
		"pull request target": func(s string) string { return strings.Replace(s, "  pull_request:\n", "  pull_request_target:\n", 1) },
		"checkout overwrites early evidence": func(s string) string {
			return strings.Replace(s, "          path: source\n", "", 1)
		},
		"missing Chromium": func(s string) string { return strings.Replace(s, " --with-deps chromium", " --with-deps", 1) },
		"missing selector": func(s string) string {
			return strings.Replace(s, "|TestGeneratedPhase3BaseComponentsRunInBrowser", "", 1)
		},
		"weakened skip": func(s string) string {
			return strings.Replace(s, `action in {"pass", "fail", "skip"}`, `action in {"pass", "fail"}`, 1)
		},
		"weakened native status": func(s string) string {
			return strings.Replace(s, "if (( native_rc != 0 ))", "if (( native_rc == 0 ))", 1)
		},
		"deleted paths excluded": func(s string) string {
			return strings.Replace(s, "--diff-filter=ACDMRTUXB", "--diff-filter=ACMRTUXB", 1)
		},
		"nested Go skip ignored": func(s string) string {
			return strings.Replace(s, `if action == "skip":`, `if action == "never":`, 1)
		},
		"pipeline status ignored": func(s string) string {
			return strings.Replace(s, "if (( pipeline_rc != 0 ))", "if (( pipeline_rc == 0 ))", 1)
		},
		"Node guard disabled": func(s string) string {
			return strings.Replace(s, "if node_skips:", "if False and node_skips:", 1)
		},
		"Go pass weakened": func(s string) string {
			return strings.Replace(s, `elif actions[0] != "pass":`, `elif actions[0] not in {"pass", "skip"}:`, 1)
		},
		"PR diff operands reversed": func(s string) string {
			return strings.Replace(s, `"$BASE_SHA" "$HEAD_SHA"`, `"$HEAD_SHA" "$BASE_SHA"`, 1)
		},
		"PR diff fail open": func(s string) string {
			return strings.Replace(s, "echo \"git diff failed for $BASE_SHA..$HEAD_SHA\" >&2\n              exit 1", "echo \"git diff failed for $BASE_SHA..$HEAD_SHA\" >&2\n              run_expensive=false", 1)
		},
		"missing SHA fail open": func(s string) string {
			return strings.Replace(s, "echo \"pull request base/head SHA is missing\" >&2\n              exit 1", "echo \"pull request base/head SHA is missing\" >&2\n              run_expensive=false", 1)
		},
		"non PR condition reversed": func(s string) string {
			return strings.Replace(s, "github.event_name != 'pull_request'", "github.event_name == 'pull_request'", 1)
		},
		"non PR default disabled": func(s string) string {
			return strings.Replace(s, "run_expensive=$UNCONDITIONAL", "run_expensive=false", 1)
		},
		"cache save after failure": func(s string) string {
			return strings.Replace(s, "if: steps.scope.outputs.run_expensive == 'true' && success()", "if: steps.scope.outputs.run_expensive == 'true'", 1)
		},
		"wrapper pipefail disabled": func(s string) string {
			return strings.Replace(s, "<<'WRAPPER'\n          #!/usr/bin/env bash\n          set -o pipefail", "<<'WRAPPER'\n          #!/usr/bin/env bash\n          set +o pipefail", 1)
		},
		"wrapper native status discarded": func(s string) string {
			return strings.Replace(s, `exit "$npm_rc"`, "exit 0", 1)
		},
		"missing REAL_NPM export": func(s string) string {
			return strings.Replace(s, "          export REAL_NPM\n", "", 1)
		},
		"missing Node evidence truncation": func(s string) string {
			return strings.Replace(s, "          : >\"$BROWSER_NODE_TEST_LOG\"\n", "", 1)
		},
		"Node evidence truncation after authority": func(s string) string {
			line := "          : >\"$BROWSER_NODE_TEST_LOG\"\n"
			s = strings.Replace(s, line, "", 1)
			return strings.Replace(s, "          pipeline_status=(\"${PIPESTATUS[@]}\")\n", "          pipeline_status=(\"${PIPESTATUS[@]}\")\n"+line, 1)
		},
		"duplicate REAL_NPM resolution": func(s string) string {
			line := "          REAL_NPM=$(command -v npm)"
			return strings.Replace(s, line, line+"\n"+line, 1)
		},
		"duplicate npm wrapper creation": func(s string) string {
			line := `          cat >"$RUNNER_TEMP/browser-bin/npm" <<'WRAPPER'`
			return strings.Replace(s, line, line+"\n"+line, 1)
		},
		"duplicate npm capture": func(s string) string {
			line := `          "$REAL_NPM" "$@" 2>&1 | tee -a "$BROWSER_NODE_TEST_LOG"`
			return strings.Replace(s, line, line+"\n"+line, 1)
		},
		"duplicate Go authority": func(s string) string {
			line := "            go test -json -vet=off -p=1 -count=1 -timeout=25m"
			return strings.Replace(s, line, line+"\n"+line, 1)
		},
		"wrapper tee target weakened": func(s string) string {
			return strings.Replace(s, `tee -a "$BROWSER_NODE_TEST_LOG"`, `tee "$BROWSER_NODE_TEST_LOG"`, 1)
		},
		"wrapper PIPESTATUS weakened": func(s string) string {
			return strings.Replace(s, `npm_pipeline_status=("${PIPESTATUS[@]}")`, `npm_pipeline_status=(0 0)`, 1)
		},
		"scope newline parsing": func(s string) string {
			old := `if ! git diff --name-only -z --no-renames --diff-filter=ACDMRTUXB "$BASE_SHA" "$HEAD_SHA" >"$changed_paths_file"; then`
			replacement := `if ! changed_paths=$(git diff --name-only --no-renames --diff-filter=ACDMRTUXB "$BASE_SHA" "$HEAD_SHA"); then`
			s = strings.Replace(s, old, replacement, 1)
			s = strings.Replace(s, `while IFS= read -r -d '' changed_path; do`, `while IFS= read -r changed_path; do`, 1)
			return strings.Replace(s, `done <"$changed_paths_file"`, `done <<<"$changed_paths"`, 1)
		},
		"eighth evidence file": func(s string) string {
			line := `          printf 'not run\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/extra.log"`
			return strings.Replace(s, "      - uses: actions/checkout@v6", line+"\n\n      - uses: actions/checkout@v6", 1)
		},
		"evidence files reordered": func(s string) string {
			first := `          printf 'not run\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/npm-ci-third-party-lwc.log"`
			second := `          printf 'not run\n' >"$GITHUB_WORKSPACE/ci-artifacts/browser/npm-ci-lwcruntime.log"`
			return strings.Replace(s, first+"\n"+second, second+"\n"+first, 1)
		},
		"TAP pass equality weakened": func(s string) string {
			return strings.Replace(s, `tap["pass"] != tap["tests"]`, `tap["pass"] > tap["tests"]`, 1)
		},
		"TAP skipped allowed": func(s string) string {
			return strings.Replace(s, `tap["skipped"] != 0`, `tap["skipped"] < 0`, 1)
		},
		"TAP escaped skip hash rejected": func(s string) string {
			return strings.Replace(s, `(?<!\\)#[ \t]*SKIP`, `#[ \t]*SKIP`, 1)
		},
		"TAP empty suite allowed": func(s string) string {
			return strings.Replace(s, `tap["tests"] <= 0`, `tap["tests"] < 0`, 1)
		},
		"TAP failures allowed": func(s string) string {
			return strings.Replace(s, `tap["fail"] != 0`, `tap["fail"] < 0`, 1)
		},
		"TAP skipped summary not parsed": func(s string) string {
			return strings.Replace(s, `for field in ("tests", "pass", "fail", "skipped"):`, `for field in ("tests", "pass", "fail"):`, 1)
		},
		"TAP not ok guard disabled": func(s string) string {
			return strings.Replace(s, `if node_failures:`, `if False and node_failures:`, 1)
		},
		"TAP cannot launch guard disabled": func(s string) string {
			return strings.Replace(s, `if "cannot launch chromium" in node_text.lower():`, `if False:`, 1)
		},
		"continue on error": func(s string) string {
			return strings.Replace(s, "      - name: Run browser authorities", "      - name: Run browser authorities\n        continue-on-error: true", 1)
		},
	}
	for _, evidence := range browserEvidenceFiles() {
		evidence := evidence
		mutations["missing evidence "+evidence.name] = func(s string) string {
			return strings.Replace(s, "          "+evidence.command+"\n", "", 1)
		}
	}
	mutations["evidence moved into authority step"] = func(s string) string {
		var moved strings.Builder
		for _, evidence := range browserEvidenceFiles() {
			line := "          " + evidence.command + "\n"
			if strings.Contains(s, line) {
				s = strings.Replace(s, line, "", 1)
				moved.WriteString("          ")
				moved.WriteString(evidence.command)
				moved.WriteByte('\n')
			}
		}
		needle := "      - name: Run browser authorities\n        if: steps.scope.outputs.run_expensive == 'true'\n        working-directory: source\n        run: |\n"
		return strings.Replace(s, needle, needle+moved.String(), 1)
	}
	for name, mutate := range mutations {
		t.Run("rejects "+name, func(t *testing.T) {
			mutated := mutate(workflow)
			if mutated == workflow {
				t.Fatal("mutation did not change workflow")
			}
			if problem := browserWorkflowProblem(mutated); problem == "" {
				t.Fatal("mutated workflow passed contract")
			}
		})
	}
}

func TestCIBrowserScopeHandlesNewlinePath(t *testing.T) {
	path := filepath.Join("..", ".github", "workflows", "browser.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	job := workflowJobBlocksText(string(data))["browser"]
	script := workflowRunScriptText(workflowStepBlockText(job, "      - name: Determine browser test scope"))
	script = strings.ReplaceAll(script, `${{ github.event_name }}`, "pull_request")
	if script == "" {
		t.Fatal("scope script is missing")
	}

	repo := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init", "-q")
	runGit("config", "user.email", "browser-ci@example.invalid")
	runGit("config", "user.name", "Browser CI")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-qm", "base")
	base := runGit("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "newline\ncomponent.go"), []byte("package newline\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "-A")
	runGit("commit", "-qm", "newline go path")
	head := runGit("rev-parse", "HEAD")

	output := filepath.Join(repo, "github-output")
	if err := os.MkdirAll(filepath.Join(repo, "ci-artifacts", "browser"), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "BASE_SHA="+base, "HEAD_SHA="+head, "UNCONDITIONAL=false", "GITHUB_OUTPUT="+output, "GITHUB_WORKSPACE="+repo, "RUNNER_TEMP="+t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("scope script failed: %v\n%s", err, out)
	}
	result, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != "run_expensive=true\n" {
		t.Fatalf("newline-bearing .go path scope = %q, want true", result)
	}
}

func TestCIBrowserTAPValidationFixtures(t *testing.T) {
	path := filepath.Join("..", ".github", "workflows", "browser.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	job := workflowJobBlocksText(string(data))["browser"]
	runScript := workflowRunScriptText(workflowStepBlockText(job, "      - name: Run browser authorities"))
	_, tail, ok := strings.Cut(runScript, "python3 - <<'PY'\n")
	if !ok {
		t.Fatal("browser validator Python is missing")
	}
	python, _, ok := strings.Cut(tail, "\nPY\n")
	if !ok {
		t.Fatal("browser validator Python terminator is missing")
	}
	goEvents := strings.Join([]string{
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwcruntime","Test":"TestBrowserRuntimeSuite"}`,
		`{"Action":"pass","Package":"github.com/glade-sh/glade/internal/lwcbrowser","Test":"TestGeneratedPhase3BaseComponentsRunInBrowser"}`,
	}, "\n") + "\n"
	cases := []struct {
		name    string
		nodeTAP string
		valid   bool
	}{
		{name: "harmless diagnostic words", valid: true, nodeTAP: "TAP version 13\n# Subtest: name mentions not ok and # SKIP harmlessly\nok 1 - name mentions not ok and \\# SKIP harmlessly\n1..1\n# tests 1\n# pass 1\n# fail 0\n# skipped 0\n"},
		{name: "numbered failure", nodeTAP: "TAP version 13\nnot ok 1 - failed\n1..1\n# tests 1\n# pass 1\n# fail 0\n# skipped 0\n"},
		{name: "numbered skip", nodeTAP: "TAP version 13\nok 1 - unavailable # SKIP browser absent\n1..1\n# tests 1\n# pass 1\n# fail 0\n# skipped 0\n"},
		{name: "cannot launch", nodeTAP: "TAP version 13\n# cannot launch chromium: denied\nok 1 - passes\n1..1\n# tests 1\n# pass 1\n# fail 0\n# skipped 0\n"},
		{name: "successful evidence retains sentinel", nodeTAP: "not run\nTAP version 13\nok 1 - passes\n1..1\n# tests 1\n# pass 1\n# fail 0\n# skipped 0\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			artifacts := filepath.Join(workspace, "ci-artifacts", "browser")
			if err := os.MkdirAll(artifacts, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(artifacts, "go-test.json"), []byte(goEvents), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(artifacts, "node-test.log"), []byte(tc.nodeTAP), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("python3", "-c", python)
			cmd.Env = append(os.Environ(), "GITHUB_WORKSPACE="+workspace)
			out, err := cmd.CombinedOutput()
			if tc.valid && err != nil {
				t.Fatalf("valid TAP rejected: %v\n%s", err, out)
			}
			if !tc.valid && err == nil {
				t.Fatalf("invalid TAP accepted:\n%s", out)
			}
		})
	}
}

func TestCIRequiredScheduledAggregateIsFailClosed(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	script := requiredAggregateShell(t, jobs["required-scheduled-ci"])
	for _, tc := range []struct {
		name       string
		results    string
		wantStatus int
	}{
		{name: "all success", results: `{"required-ci":{"result":"success"},"sema-full":{"result":"success"},"sema-equivalence":{"result":"success"}}`},
		{name: "normal CI failure", results: `{"required-ci":{"result":"failure"},"sema-full":{"result":"success"},"sema-equivalence":{"result":"success"}}`, wantStatus: 1},
		{name: "full skipped", results: `{"required-ci":{"result":"success"},"sema-full":{"result":"skipped"},"sema-equivalence":{"result":"success"}}`, wantStatus: 1},
		{name: "equivalence cancelled", results: `{"required-ci":{"result":"success"},"sema-full":{"result":"success"},"sema-equivalence":{"result":"cancelled"}}`, wantStatus: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "REQUIRED_RESULTS="+tc.results)
			out, err := cmd.CombinedOutput()
			got := 0
			if err != nil {
				got = err.(*exec.ExitError).ExitCode()
			}
			if got != tc.wantStatus {
				t.Fatalf("scheduled aggregate status=%d want=%d output=%s", got, tc.wantStatus, out)
			}
		})
	}
}

func TestCISemaShardRunsOneNativePackageExecution(t *testing.T) {
	out, err, artifacts, plannerArgs := runSemaShardFixture(t, "1", "TestBeta\nTestAlpha\nok  \t"+semaFixturePackage+"\n", semaFixturePlan(), semaPassEvent("TestBeta"), 0)
	if err != nil {
		t.Fatalf("sema shard failed: %v\n%s", err, out)
	}
	args, err := os.ReadFile(plannerArgs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--package "+semaFixturePackage) {
		t.Fatalf("planner args omit requested package: %s", args)
	}
	calls, err := os.ReadFile(filepath.Join(filepath.Dir(artifacts), "calls"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "test -json") != 1 || !strings.Contains(string(calls), "-run ^(?:TestBeta)$ ./internal/sema") {
		t.Fatalf("native sema calls = %q", calls)
	}
	var summary struct {
		Valid bool `json:"valid"`
	}
	b, err := os.ReadFile(filepath.Join(artifacts, "validation-summary.json"))
	if err != nil || json.Unmarshal(b, &summary) != nil || !summary.Valid {
		t.Fatalf("invalid sema summary %s err=%v", b, err)
	}
}

func TestCISemaShardRejectsMissingOrNonPassingTerminal(t *testing.T) {
	for _, events := range []string{"", `{"Action":"skip","Package":"` + semaFixturePackage + `","Test":"TestAlpha"}` + "\n"} {
		out, err, _, _ := runSemaShardFixture(t, "0", "TestAlpha\nTestBeta\nok  \t"+semaFixturePackage+"\n", semaFixturePlan(), events, 0)
		if err == nil {
			t.Fatalf("invalid terminal evidence accepted:\n%s", out)
		}
	}
}

func TestCISemaDiscoveryRequiresExactPackageTrailer(t *testing.T) {
	for _, discovery := range []string{
		"TestAlpha\nTestBeta\n",
		"TestAlpha\n\nTestBeta\nok  \t" + semaFixturePackage + "\n",
		"TestAlpha\nTestBeta\nok  \t" + semaFixturePackage + "\nok  \t" + semaFixturePackage + "\n",
	} {
		out, err, _, _ := runSemaShardFixture(t, "0", discovery, semaFixturePlan(), semaPassEvent("TestAlpha"), 0)
		if err == nil {
			t.Fatalf("non-exact sema discovery accepted:\n%s", out)
		}
	}
}

func TestCISemaCommandsRejectInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"sema-history-refresh", "a", "b", "c", "extra"},
		{"sema-shard"},
		{"sema-shard", "2"},
		{"sema-shard", "0", "extra"},
		{"sema-history-refresh", "a", "b"},
		{"sema-full", "extra"},
		{"sema-equivalence", "a", "b", "c"},
		{"sema-equivalence", "a", "b", "c", "d", "extra"},
	} {
		cmd := exec.Command("bash", append([]string{"ci-go-test.sh"}, args...)...)
		cmd.Env = append(os.Environ(), "CI_SEMA_ARTIFACT_DIR="+filepath.Join(t.TempDir(), "artifacts"))
		out, err := cmd.CombinedOutput()
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 2 {
			t.Fatalf("invalid args %v status=%v want 2\n%s", args, err, out)
		}
	}
}

func semaPairFixture(t *testing.T) apexHistoryFixture {
	t.Helper()
	fixture := newApexHistoryFixture(t, [2]float64{140, 139})
	fixture.output = filepath.Join(fixture.root, "history", "sema-duration-history.json")
	for _, dir := range fixture.shards {
		for _, name := range []string{"plan.json", "events.json"} {
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data = bytes.ReplaceAll(data, []byte(fixturePackage), []byte(semaFixturePackage))
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return fixture
}

func TestCISemaHistoryAndScheduledEquivalence(t *testing.T) {
	fixture := semaPairFixture(t)
	cmd := exec.Command("bash", "ci-go-test.sh", "sema-history-refresh", fixture.shards[0], fixture.shards[1], fixture.output)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sema history refresh failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(fixture.output)
	if err != nil {
		t.Fatal(err)
	}
	var history struct {
		Package string `json:"package"`
		Tests   []struct {
			Name string `json:"name"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &history); err != nil || history.Package != semaFixturePackage || len(history.Tests) != 279 {
		t.Fatalf("sema history = %+v err=%v", history, err)
	}

	fullDir := filepath.Join(fixture.root, "full")
	if err := os.Mkdir(fullDir, 0o700); err != nil {
		t.Fatal(err)
	}
	discovery, err := os.ReadFile(filepath.Join(fixture.shards[0], "discovery.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fullDir, "discovery.txt"), discovery, 0o600); err != nil {
		t.Fatal(err)
	}
	var fullEvents []byte
	for _, dir := range fixture.shards {
		part, err := os.ReadFile(filepath.Join(dir, "events.json"))
		if err != nil {
			t.Fatal(err)
		}
		fullEvents = append(fullEvents, part...)
	}
	fullEventsPath := filepath.Join(fullDir, "events.json")
	if err := os.WriteFile(fullEventsPath, fullEvents, 0o600); err != nil {
		t.Fatal(err)
	}
	equivalence := filepath.Join(fixture.root, "equivalence", "summary.json")
	run := func() error {
		return exec.Command("bash", "ci-go-test.sh", "sema-equivalence", fixture.shards[0], fixture.shards[1], fullDir, equivalence).Run()
	}
	if err := run(); err != nil {
		t.Fatalf("matching full and shard maps rejected: %v", err)
	}
	corrupt := bytes.Replace(fullEvents, []byte(`"Action":"pass"`), []byte(`"Action":"skip"`), 1)
	if err := os.WriteFile(fullEventsPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(); err == nil {
		t.Fatal("full terminal mismatch was accepted")
	}
	if _, err := os.Stat(equivalence); !os.IsNotExist(err) {
		t.Fatalf("failed equivalence left stale success evidence: %v", err)
	}
}

func TestCIRequiredAggregateContract(t *testing.T) {
	workflow, jobs := readCIWorkflow(t)
	if problem := ciRequiredAggregateProblem(workflow); problem != "" {
		t.Fatal(problem)
	}
	script := requiredAggregateShell(t, jobs["required-ci"])
	for _, tc := range []struct {
		name       string
		results    string
		wantStatus int
		wantOutput string
	}{
		{name: "all success", results: `{"site":{"result":"success"},"test":{"result":"success"}}`},
		{name: "failure", results: `{"site":{"result":"success"},"test":{"result":"failure"}}`, wantStatus: 1, wantOutput: "test=failure"},
		{name: "cancelled", results: `{"site":{"result":"cancelled"}}`, wantStatus: 1, wantOutput: "site=cancelled"},
		{name: "skipped", results: `{"site":{"result":"skipped"}}`, wantStatus: 1, wantOutput: "site=skipped"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "REQUIRED_RESULTS="+tc.results)
			out, err := cmd.CombinedOutput()
			gotStatus := 0
			if err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("aggregate shell failed to run: %v\n%s", err, out)
				}
				gotStatus = exitErr.ExitCode()
			}
			if gotStatus != tc.wantStatus || !strings.Contains(string(out), tc.wantOutput) {
				t.Fatalf("aggregate status/output = %d/%q, want %d containing %q", gotStatus, out, tc.wantStatus, tc.wantOutput)
			}
		})
	}
}

func nodeIntegrationExpectedPairs() [][2]string {
	var pairs [][2]string
	for pkg, names := range nodeIntegrationTests {
		for _, name := range names {
			pairs = append(pairs, [2]string{pkg, name})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}
		return pairs[i][1] < pairs[j][1]
	})
	return pairs
}

func nodeIntegrationEvents(action string) string {
	var out strings.Builder
	for _, pair := range nodeIntegrationExpectedPairs() {
		fmt.Fprintf(&out, "{\"Action\":\"run\",\"Package\":%q,\"Test\":%q}\n", pair[0], pair[1])
		fmt.Fprintf(&out, "{\"Action\":%q,\"Package\":%q,\"Test\":%q,\"Elapsed\":0.01}\n", action, pair[0], pair[1])
	}
	return out.String()
}

func runNodeIntegrationFixture(t *testing.T, events string, nativeRC int, mutatePath func(string) error) (string, error, string, string) {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	artifactDir := filepath.Join(dir, "artifacts")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	eventsPath := filepath.Join(dir, "events.json")
	callsPath := filepath.Join(dir, "calls.txt")
	if err := os.WriteFile(eventsPath, []byte(events), 0o600); err != nil {
		t.Fatal(err)
	}
	goScript := `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FIXTURE_CALLS"
cat "$FIXTURE_EVENTS"
exit "$FIXTURE_NATIVE_RC"
`
	rendererScript := "#!/usr/bin/env bash\ncat >/dev/null\n"
	for name, contents := range map[string]string{"go": goScript, "testlog": rendererScript} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(contents), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if mutatePath != nil {
		if err := mutatePath(binDir); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", "ci-go-test.sh", "node-integration")
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"FIXTURE_CALLS="+callsPath,
		"FIXTURE_EVENTS="+eventsPath,
		fmt.Sprintf("FIXTURE_NATIVE_RC=%d", nativeRC),
		"CI_TESTLOG_RENDERER="+filepath.Join(binDir, "testlog"),
		"CI_NODE_INTEGRATION_ARTIFACT_DIR="+artifactDir,
		"CI_GO_TEST_HEARTBEAT_SECONDS=1",
	)
	out, err := cmd.CombinedOutput()
	calls, readErr := os.ReadFile(callsPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	return string(out), err, artifactDir, string(calls)
}

func TestCINodeIntegrationCommandExactSelectionAndEvidence(t *testing.T) {
	out, err, artifacts, calls := runNodeIntegrationFixture(t, nodeIntegrationEvents("pass"), 0, nil)
	if err != nil {
		t.Fatalf("node integration fixture failed: %v\n%s", err, out)
	}
	callLines := strings.Split(strings.TrimSpace(calls), "\n")
	if len(callLines) != 1 {
		t.Fatalf("native executions = %d, want 1; calls:\n%s", len(callLines), calls)
	}
	call := callLines[0]
	for _, marker := range []string{"test -json -vet=off", "-count=1", "-timeout=30m", "./internal/gladecli", "./internal/gladehome", "./internal/lwc/compile", "./internal/lwcbrowser", "./internal/server"} {
		if !strings.Contains(call, marker) {
			t.Errorf("native command missing %q: %s", marker, call)
		}
	}
	if strings.Contains(call, "TestBrowserRuntimeSuite") || strings.Contains(call, "TestGeneratedPhase3BaseComponentsRunInBrowser") {
		t.Errorf("native command includes forbidden browser selector: %s", call)
	}
	wantRun := "-run ^(?:" + strings.Join(nodeIntegrationRunNames, "|") + ")$"
	if !strings.Contains(call, wantRun) {
		t.Errorf("native command run selector mismatch\n got: %s\nwant substring: %s", call, wantRun)
	}
	for _, name := range []string{"test-node-integration.json", "expected.txt", "discovery.txt", "validation-summary.json"} {
		if _, err := os.Stat(filepath.Join(artifacts, name)); err != nil {
			t.Errorf("evidence %s missing: %v", name, err)
		}
	}
	summary, err := os.ReadFile(filepath.Join(artifacts, "validation-summary.json"))
	if err != nil || !strings.Contains(string(summary), `"tests": 30`) || !strings.Contains(string(summary), `"valid": true`) {
		t.Errorf("validation summary invalid: err=%v data=%s", err, summary)
	}
}

func nodeIntegrationCountProblem(script string) string {
	if !strings.Contains(script, `go test -json -vet=off -count=1 -timeout=30m -run "${node_integration_run_regex}"`) {
		return "authoritative node integration command lacks -count=1"
	}
	return ""
}

func TestCINodeIntegrationCommandRequiresFreshExecution(t *testing.T) {
	data, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	if problem := nodeIntegrationCountProblem(script); problem != "" {
		t.Fatal(problem)
	}
	mutated := strings.Replace(script,
		`go test -json -vet=off -count=1 -timeout=30m -run "${node_integration_run_regex}"`,
		`go test -json -vet=off -timeout=30m -run "${node_integration_run_regex}"`, 1)
	if mutated == script {
		t.Fatal("fixture did not remove -count=1")
	}
	if problem := nodeIntegrationCountProblem(mutated); problem == "" {
		t.Fatal("node integration command contract accepted removal of -count=1")
	}
}

func TestCINodeIntegrationValidatorRejectsInvalidEvidence(t *testing.T) {
	valid := nodeIntegrationEvents("pass")
	pairs := nodeIntegrationExpectedPairs()
	first := pairs[0]
	terminal := fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":0.01}\n", first[0], first[1])
	cases := map[string]struct {
		events   string
		nativeRC int
		mutate   func(string) error
	}{
		"skip":          {events: strings.Replace(valid, `"Action":"pass"`, `"Action":"skip"`, 1)},
		"fail":          {events: strings.Replace(valid, `"Action":"pass"`, `"Action":"fail"`, 1)},
		"missing":       {events: strings.Replace(valid, terminal, "", 1)},
		"extra":         {events: valid + `{"Action":"pass","Package":"github.com/glade-sh/glade/internal/server","Test":"TestUnexpected"}` + "\n"},
		"duplicate":     {events: valid + terminal},
		"malformed":     {events: valid + "{not-json}\n"},
		"wrong package": {events: strings.Replace(valid, first[0], "example.invalid/wrong", 1)},
		"nested skip":   {events: valid + fmt.Sprintf("{\"Action\":\"skip\",\"Package\":%q,\"Test\":%q}\n", first[0], first[1]+"/nested")},
		"native":        {events: valid, nativeRC: 23},
		"tee": {events: valid, mutate: func(binDir string) error {
			return os.WriteFile(filepath.Join(binDir, "tee"), []byte("#!/usr/bin/env bash\ncat >/dev/null\nexit 17\n"), 0o700)
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out, err, artifacts, _ := runNodeIntegrationFixture(t, tc.events, tc.nativeRC, tc.mutate)
			if err == nil {
				t.Fatalf("invalid evidence accepted\n%s", out)
			}
			if _, statErr := os.Stat(filepath.Join(artifacts, "validation-summary.json")); !os.IsNotExist(statErr) {
				t.Fatalf("invalid run left success summary: %v", statErr)
			}
		})
	}
}

func TestCINodeIntegrationWorkflowAndPurePartition(t *testing.T) {
	workflow, jobs := readCIWorkflow(t)
	node := jobs["node-integration"]
	for _, marker := range []string{
		"runs-on: ubuntu-latest", "timeout-minutes: 30", "GOMAXPROCS: \"2\"",
		"actions/setup-go@v6", "go-version: \"1.26.5\"", "cache: false",
		"actions/setup-node@v6", "node-version: \"22\"", "cache: npm",
		"cache-dependency-path: third_party/lwc/package-lock.json", "npm ci --prefix third_party/lwc",
		"scripts/ci-go-test.sh node-integration", "ci-node-integration",
		"actions/cache/restore@v4", "actions/cache/save@v4", "continue-on-error: true", "if: success()",
		"go-test-node-integration", "if: always()", "ci-artifacts/go-test-node-integration/", "if-no-files-found: error",
	} {
		if !strings.Contains(node, marker) {
			t.Errorf("node-integration job missing %q", marker)
		}
	}
	if strings.Count(node, "npm ci --prefix third_party/lwc") != 1 || strings.Count(workflow, "npm ci --prefix third_party/lwc") != 2 {
		t.Errorf("third_party/lwc npm install ownership count workflow/node = %d/%d, want 2/1 (distribution smoke plus node lane)", strings.Count(workflow, "npm ci --prefix third_party/lwc"), strings.Count(node, "npm ci --prefix third_party/lwc"))
	}
	for _, name := range []string{"gladecli", "server-and-playground"} {
		if strings.Contains(jobs[name], "actions/setup-node") || strings.Contains(jobs[name], "npm ci") {
			t.Errorf("pure job %s mutates Node dependencies", name)
		}
	}
	testJob := jobs["test"]
	if !strings.Contains(testJob, "actions/setup-node@v6") || !strings.Contains(testJob, `node-version: "22"`) {
		t.Error("pure test job must retain Node 22 runtime")
	}
	for _, forbidden := range []string{"cache: npm", "cache-dependency-path:", "npm ci --prefix third_party/lwc"} {
		if strings.Contains(testJob, forbidden) {
			t.Errorf("pure test job contains forbidden Node mutation %q", forbidden)
		}
	}
}

func TestCINodeIntegrationPureLaneSkipSelectors(t *testing.T) {
	wantSkip := map[string]string{
		"gladecli":              "^(?:TestRunDoctorReportsParser|TestRunDoctorJSON|TestRunDoctorShortFlags|TestRunDoctorReportsProjectLocalDataEnvironment)$",
		"server-and-playground": "^(?:TestVFPageBootstrapsLightningOut|TestVFPageBootstrapsMultiWidgetLightningOut|TestLightningModulesServesCompiledJS|TestLightningModulesServesSiblingModuleWithoutJSExtension|TestLWCShellComponentRouteServesHTML|TestLWCShellRootRendersHomeWithFormalTabsAndBuilderLink|TestLWCShellBuilderRouteRendersBuilderNavigationLayoutAndSampleRecord|TestLWCShellTabRouteIncludesPreviewRouteCatalog|TestServerRootRendersLWCHomeWhenProjectHasLWCs|TestLWCShellRendersApplicationNavAndConsoleMode|TestLWCShellAppRouteFallsBackToApplicationDefaultTab|TestLWCShellUnsupportedCustomTabReturnsDiagnostic|TestLWCShellMixedPageDiagnosticsStillRendersValidComponents)$",
		"repoguard":             "",
		"remaining-go":          "^(?:TestCompileProjectLWCBundles|TestCompileRewritesTemplateStylesheetImports|TestCompileEmitsSiblingJSModules|TestCompileEmitsUtilityOnlyLWCModules|TestCompileEmitsAdditionalHTMLTemplateModules|TestCompileTransformsCustomRenderComponentWithoutSameNameTemplate|TestCompileEnablesLwcOnDirective|TestSetupBundleIncludesLabelsSibling|TestSetupImportMapIncludesLocalComponents|TestValidateRootFindsRepoCheckout|TestInstallFromCWDSkipsGlobalShareAsSource|TestInstallFromCopiesToolchain|TestEnsureRootHonorsExplicitGladeHomeBeforeUserShare|TestBrowserRuntimeSuite|TestGeneratedPhase3BaseComponentsRunInBrowser)$",
	}
	for lane, skip := range wantSkip {
		t.Run(lane, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			goScript := "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >\"$FIXTURE_CALLS\"\nexit 0\n"
			rendererScript := "#!/usr/bin/env bash\ncat >/dev/null\n"
			for name, contents := range map[string]string{"go": goScript, "testlog": rendererScript} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", "ci-go-test.sh", "lane", lane)
			cmd.Env = append(os.Environ(),
				"PATH="+dir+":"+os.Getenv("PATH"), "FIXTURE_CALLS="+calls,
				"CI_GO_COMMAND="+realGoCommand(t), "CI_TESTLOG_RENDERER="+filepath.Join(dir, "testlog"),
				"CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "artifacts"),
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("lane failed: %v\n%s", err, out)
			}
			data, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			call := strings.TrimSpace(string(data))
			if skip == "" {
				if strings.Contains(call, "-skip") {
					t.Fatalf("repoguard unexpectedly skips tests: %s", call)
				}
			} else if !strings.Contains(call, "-skip "+skip) {
				t.Fatalf("lane skip mismatch\n got: %s\nwant: %s", call, skip)
			}
			for _, keep := range []string{
				"TestUIRecordAPIRecordInputHelpersFilterAndUnwrapFields", "TestUILayoutAPIModuleJSMapsGetLayoutRequest",
				"TestUIObjectInfoAPIModuleJSMapsObjectAndPicklistRequests", "TestUIRelatedListAPIModuleJSMapsRecordRequests",
				"TestGeneratedSystemStubsReproduceFromGenerator",
			} {
				if strings.Contains(skip, keep) {
					t.Errorf("pure lane skip excludes Node-executable test %s", keep)
				}
			}
		})
	}
}

func TestCIRequiredAggregateContractRejectsWeakening(t *testing.T) {
	workflow, _ := readCIWorkflow(t)
	mutations := map[string]string{
		"remove history":          strings.Replace(workflow, "      - apextest-history\n", "", 1),
		"remove node integration": strings.Replace(workflow, "      - node-integration\n", "", 1),
		"success-only condition":  strings.Replace(workflow, "    if: always()\n    runs-on: ubuntu-latest\n    timeout-minutes: 5", "    if: success()\n    runs-on: ubuntu-latest\n    timeout-minutes: 5", 1),
		"failure-only predicate":  strings.Replace(workflow, `select(.value.result != "success")`, `select(.value.result == "failure")`, 1),
		"remove exit":             strings.Replace(workflow, "            exit 1\n", "", 1),
		"rename job":              strings.Replace(workflow, "  required-ci:\n", "  required-checks:\n", 1),
		"rename display":          strings.Replace(workflow, "    name: Required CI\n    needs:\n", "    name: Required Checks\n    needs:\n", 1),
		"job continue on error":   strings.Replace(workflow, "    if: always()\n    runs-on: ubuntu-latest\n    timeout-minutes: 5\n    steps:\n", "    if: always()\n    runs-on: ubuntu-latest\n    timeout-minutes: 5\n    continue-on-error: true\n    steps:\n", 1),
		"step continue on error":  strings.Replace(workflow, "      - name: Require all CI jobs\n        env:\n", "      - name: Require all CI jobs\n        continue-on-error: true\n        env:\n", 1),
		"step skip condition":     strings.Replace(workflow, "      - name: Require all CI jobs\n        env:\n", "      - name: Require all CI jobs\n        if: false\n        env:\n", 1),
	}
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if mutated == workflow {
				t.Fatal("test fixture did not mutate workflow")
			}
			if problem := ciRequiredAggregateProblem(mutated); problem == "" {
				t.Fatal("required aggregate contract accepted weakening mutation")
			}
		})
	}
}

func TestCIParallelDAGTopology(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	wantJobs := []string{
		"apextest", "apextest-history", "gladecli", "node-integration", "required-ci", "required-scheduled-ci", "sema", "sema-equivalence", "sema-full", "sema-history",
		"server-and-playground", "site", "smoke-distribution", "smoke-runtime", "test", "vet",
	}
	gotJobs := make([]string, 0, len(jobs))
	for name := range jobs {
		gotJobs = append(gotJobs, name)
	}
	sort.Strings(gotJobs)
	if !reflect.DeepEqual(gotJobs, wantJobs) {
		t.Fatalf("ci jobs = %v, want %v", gotJobs, wantJobs)
	}
	for _, name := range wantJobs {
		job := jobs[name]
		if !strings.Contains(job, "runs-on: ubuntu-latest") {
			t.Errorf("%s is not an ubuntu-latest job", name)
		}
		if !map[string]bool{"apextest-history": true, "required-ci": true, "required-scheduled-ci": true, "sema-history": true, "sema-equivalence": true}[name] && strings.Contains(job, "needs:") {
			t.Errorf("root job %s must not be serialized with needs", name)
		}
	}
	for _, name := range []string{"site", "vet", "gladecli", "node-integration", "sema", "sema-full", "server-and-playground", "test", "smoke-runtime", "smoke-distribution"} {
		if !strings.Contains(jobs[name], "timeout-minutes: 30") {
			t.Errorf("%s timeout is not 30 minutes", name)
		}
	}
	if !strings.Contains(jobs["apextest"], "timeout-minutes: 35") || !strings.Contains(jobs["apextest-history"], "timeout-minutes: 5") {
		t.Error("Apex job timeouts changed")
	}
}

func TestCIParallelDAGLaneCommandsAndArtifacts(t *testing.T) {
	workflow, jobs := readCIWorkflow(t)
	wantLaneCommands := map[string][]string{
		"gladecli":              {"scripts/ci-go-test.sh lane gladecli"},
		"node-integration":      {"scripts/ci-go-test.sh node-integration"},
		"server-and-playground": {"scripts/ci-go-test.sh lane server-and-playground"},
		"test":                  {"scripts/ci-go-test.sh lane repoguard", "scripts/ci-go-test.sh lane remaining-go"},
	}
	for jobName, commands := range wantLaneCommands {
		for _, command := range commands {
			if count := strings.Count(jobs[jobName], command); count != 1 {
				t.Errorf("%s command %q count = %d, want 1", jobName, command, count)
			}
		}
	}
	if strings.Contains(workflow, "scripts/ci-go-test.sh core") || strings.Contains(workflow, "run: go test") {
		t.Error("CI workflow bypasses the individual tested lane wrapper commands")
	}

	wantArtifacts := map[string]string{
		"gladecli":              "go-test-gladecli",
		"node-integration":      "go-test-node-integration",
		"server-and-playground": "go-test-server-and-playground",
		"test":                  "go-test-remaining-go",
	}
	for jobName, artifact := range wantArtifacts {
		job := jobs[jobName]
		for _, marker := range []string{"if: always()", "actions/upload-artifact@v6", "name: " + artifact} {
			if !strings.Contains(job, marker) {
				t.Errorf("%s raw artifact missing %q", jobName, marker)
			}
		}
	}
	for _, path := range []string{
		"ci-artifacts/go-test/test-gladecli.json",
		"ci-artifacts/go-test-node-integration/test-node-integration.json",
		"ci-artifacts/go-test/test-server-and-playground.json",
		"ci-artifacts/go-test/test-repoguard.json",
		"ci-artifacts/go-test/test-remaining-go.json",
	} {
		if count := strings.Count(workflow, path); count != 1 {
			t.Errorf("raw event path %q count = %d, want 1", path, count)
		}
	}
	nodeJob := jobs["node-integration"]
	setupIndex := strings.Index(nodeJob, "actions/setup-node@v6")
	installIndex := strings.Index(nodeJob, "npm ci --prefix third_party/lwc")
	laneIndex := strings.Index(nodeJob, "scripts/ci-go-test.sh node-integration")
	if setupIndex < 0 || installIndex < setupIndex || laneIndex < installIndex {
		t.Errorf("node integration prerequisite order setup=%d install=%d lane=%d", setupIndex, installIndex, laneIndex)
	}
	for _, marker := range []string{
		"npm ci --prefix site", "npm test --prefix site", "npm run build --prefix site",
		"CGO_ENABLED=1 go build -o \"$RUNNER_TEMP/glade\" ./cmd/glade", "scripts/smoke-runtime.sh \"$RUNNER_TEMP/glade\"",
		"scripts/smoke-distribution.sh",
	} {
		if count := strings.Count(workflow, marker); count != 1 {
			t.Errorf("CI coverage marker %q count = %d, want 1", marker, count)
		}
	}
}

func TestCIParallelDAGCacheOwnership(t *testing.T) {
	workflow, jobs := readCIWorkflow(t)
	owners := map[string]string{
		"vet": "ci-vet", "gladecli": "ci-gladecli",
		"node-integration":      "ci-node-integration",
		"server-and-playground": "ci-server-playground", "test": "ci-test",
		"smoke-runtime": "ci-smoke-runtime", "smoke-distribution": "ci-smoke-distribution",
	}
	primaryKeys := make(map[string]string)
	for jobName, namespace := range owners {
		job := jobs[jobName]
		sumPath := "go.sum"
		for _, marker := range []string{
			"GOMAXPROCS: \"2\"", "actions/setup-go@v6", "go-version: \"1.26.5\"", "cache: false",
			"actions/cache/restore@v4", "actions/cache/save@v4", "continue-on-error: true", "if: success()",
			"${{ runner.os }}-${{ runner.arch }}-1.26.5-" + namespace,
			"${{ hashFiles('" + sumPath + "') }}-${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
		} {
			if !strings.Contains(job, marker) {
				t.Errorf("%s cache contract missing %q", jobName, marker)
			}
		}
		if strings.Count(job, "actions/cache/restore@v4") != 2 || strings.Count(job, "actions/cache/save@v4") != 2 {
			t.Errorf("%s does not own one module/build restore-save pair", jobName)
		}
		for _, line := range strings.Split(job, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "key: ") && strings.Contains(trimmed, namespace) {
				if other, exists := primaryKeys[trimmed]; exists {
					t.Errorf("%s and %s share primary key %q", other, jobName, trimmed)
				}
				primaryKeys[trimmed] = jobName
			}
		}
		if namespace != "ci-test" && namespace != "ci-node-integration" && !strings.Contains(job, "-ci-test-${{ hashFiles('go.sum') }}-") {
			t.Errorf("%s lacks the ci-test digest seed fallback", jobName)
		}
	}
	if strings.Count(workflow, "actions/cache/save@v4") != 16 {
		t.Errorf("cache save count = %d, want existing DAG and two history writers", strings.Count(workflow, "actions/cache/save@v4"))
	}
}

func TestCIParallelDAGPreservesApexAndUsesRootCheckout(t *testing.T) {
	_, jobs := readCIWorkflow(t)
	apex := jobs["apextest"]
	history := jobs["apextest-history"]
	for _, marker := range []string{
		"shard: [0, 1]", "timeout-minutes: 35", "scripts/ci-go-test.sh apex-shard \"${{ matrix.shard }}\"",
		"CI_APEXTEST_HISTORY_PATH", "actions/cache@v5", "name: apex-shard-${{ matrix.shard }}",
	} {
		if !strings.Contains(apex, marker) {
			t.Errorf("Apex matrix contract missing %q", marker)
		}
	}
	for _, marker := range []string{
		"needs: apextest", "if: ${{ success() }}", "timeout-minutes: 5", "actions/download-artifact@v7",
		"scripts/ci-go-test.sh apex-history-refresh", "actions/cache/save@v4",
	} {
		if !strings.Contains(history, marker) {
			t.Errorf("Apex history contract missing %q", marker)
		}
	}
	testJob := jobs["test"]
	for _, marker := range []string{
		"actions/checkout@v6", "hashFiles('go.sum')",
		"ci-artifacts/go-test/test-repoguard.json", "ci-artifacts/go-test/test-remaining-go.json",
	} {
		if !strings.Contains(testJob, marker) {
			t.Errorf("root test checkout contract missing %q", marker)
		}
	}
	if strings.Count(testJob, "actions/checkout@v6") != 1 {
		t.Errorf("test checkout count = %d, want 1", strings.Count(testJob, "actions/checkout@v6"))
	}
	for _, forbidden := range []string{
		"actions/create-github-app-token", "app-token", "Resolve glade-tools ref", "GLADE_TOOLS_REMOTE",
		"repository: glade-sh/glade-tools", "path: glade-tools", "working-directory: glade",
		"path: glade\n", "hashFiles('glade/go.sum')", "glade/ci-artifacts/",
	} {
		if strings.Contains(testJob, forbidden) {
			t.Errorf("root test checkout retains %q", forbidden)
		}
	}
}

func TestCIGoTestLogWrapperIsWired(t *testing.T) {
	script, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatalf("read ci-go-test.sh: %v", err)
	}
	scriptText := string(script)
	for _, want := range []string{
		`export GOMAXPROCS="${GOMAXPROCS:-2}"`,
		"run_with_heartbeat",
		"run_core_tests",
		"run_full_tests",
		"run_apextest_matrix_shard",
		"heartbeat_pid",
		`kill -0 "${pid}"`,
		"./internal/apextest",
		`go test -list '^Test' "${apex_package}"`,
		`run_json_with_heartbeat "go test Apex shard ${index}" "${events}" -timeout=30m -run "${regex}" "${apex_package}"`,
		"CI_SHARD_PLANNER",
		"validation-summary.json",
		`run_package_lane "gladecli"`,
		`run_package_lane "sema"`,
		"go test -json",
		"run_json_with_heartbeat",
		"CI_VERBOSE",
		"testlog_status_files",
		"owned_child_roots",
		"terminate_owned_tree",
		`trap 'handle_wrapper_signal 143' TERM`,
		`if [[ "${BASH_SOURCE[0]}" == "$0" ]]`,
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("ci-go-test.sh missing %q", want)
		}
	}
	if strings.Contains(scriptText, `grep '^Test' || true`) {
		t.Fatal("ci-go-test.sh must not suppress Apex test discovery failures")
	}
	for _, forbidden := range []string{"apextest.test", "go test -c", "-test.run", "-test.list"} {
		if strings.Contains(scriptText, forbidden) {
			t.Fatalf("ci-go-test.sh retains forbidden direct-binary path %q", forbidden)
		}
	}

	workflowPath := filepath.Join("..", ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"timeout-minutes: 30",
		"GOMAXPROCS: \"2\"",
		"go-version: \"1.26.5\"",
		"actions/checkout@v6",
		"actions/setup-go@v6",
		"actions/setup-node@v6",
		"scripts/ci-go-test.sh lane remaining-go",
		"apextest:",
		"matrix:",
		"shard: [0, 1]",
		"cache: false",
		"ci-apextest-${{ matrix.shard }}",
		"go-mod-v1-1.26.5-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.sum') }}-ci-apextest-${{ matrix.shard }}-${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
		"go-build-v1-1.26.5-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.sum') }}-ci-apextest-${{ matrix.shard }}-${{ github.sha }}-${{ github.run_id }}-${{ github.run_attempt }}",
		"scripts/ci-go-test.sh apex-shard \"${{ matrix.shard }}\"",
		"if: always()",
		"actions/upload-artifact@v6",
		"CI_APEXTEST_ARTIFACT_DIR: ci-artifacts/apextest-${{ matrix.shard }}",
		"ci-artifacts/apextest-${{ matrix.shard }}/discovery-stderr.txt",
		"ci-artifacts/apextest-${{ matrix.shard }}/discovery-command.txt",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("ci.yml missing %q", want)
		}
	}
	for _, forbidden := range []string{"GLADE_APP_CLIENT_ID", "GLADE_APP_PRIVATE_KEY", "actions/create-github-app-token"} {
		if strings.Contains(workflowText, forbidden) {
			t.Fatalf("ci.yml retains unused app authentication marker %q", forbidden)
		}
	}
	if strings.Contains(workflowText, "scripts/ci-go-test.sh race") {
		t.Fatalf("ci.yml should not run the full race suite on GitHub-hosted runners")
	}
	if strings.Count(workflowText, "go test -json") != 0 {
		t.Fatal("ci.yml must delegate the native Apex run to the tested wrapper")
	}
	if strings.Count(workflowText, "shard: [0, 1]") != 2 {
		t.Fatal("ci.yml must define two-cell Apex and sema matrices")
	}
	if strings.Contains(workflowText, ".ci-artifacts") || strings.Contains(scriptText, ".ci-artifacts") {
		t.Fatal("Apex evidence must use a non-hidden path so upload-artifact includes it by default")
	}
}

func TestCIPackageLanesRouteThroughCheckedManifest(t *testing.T) {
	manifest, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Version int                 `json:"version"`
		Lanes   map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(manifest, &document); err != nil {
		t.Fatal(err)
	}
	if document.Version != 1 || len(document.Lanes) != 6 {
		t.Fatalf("manifest version/lane count = %d/%d", document.Version, len(document.Lanes))
	}
	totalPackages := 0
	for _, packages := range document.Lanes {
		totalPackages += len(packages)
	}
	if totalPackages != 61 {
		t.Fatalf("manifest package union = %d, want 61", totalPackages)
	}
	remaining := document.Lanes["remaining-go"]
	if len(remaining) == 0 {
		t.Fatal("remaining-go must be explicitly enumerated")
	}
	for _, pkg := range remaining {
		if strings.Contains(pkg, "...") || strings.ContainsAny(pkg, "*?") {
			t.Fatalf("remaining-go contains catch-all %q", pkg)
		}
	}
	script, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, lane := range []string{"gladecli", "sema", "server-and-playground", "repoguard", "remaining-go"} {
		if !strings.Contains(string(script), `run_named_package_lane "`+lane+`"`) {
			t.Errorf("core routing missing lane %q", lane)
		}
	}
}

func TestCIPackageLaneCommandRunsOnlyRequestedManifestPackagesAndPreservesStatus(t *testing.T) {
	manifestData, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Lanes map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	toArgument := func(pkg string) string {
		return "./" + strings.TrimPrefix(pkg, "github.com/glade-sh/glade/")
	}
	allPackages := make(map[string]string)
	for lane, packages := range manifest.Lanes {
		for _, pkg := range packages {
			allPackages[toArgument(pkg)] = lane
		}
	}

	cases := []struct {
		lane        string
		wantTimeout string
		wantP       string
	}{
		{lane: "gladecli", wantTimeout: "-timeout=30m"},
		{lane: "sema", wantTimeout: "-timeout=30m"},
		{lane: "server-and-playground", wantTimeout: "-timeout=30m"},
		{lane: "repoguard", wantTimeout: "-timeout=30m"},
		{lane: "remaining-go", wantTimeout: "-timeout=20m", wantP: "-p=2"},
	}
	for _, tc := range cases {
		t.Run(tc.lane, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			fakeGo := filepath.Join(dir, "go")
			fakeRenderer := filepath.Join(dir, "testlog")
			goScript := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
printf '%s\n' '{"Action":"output","Package":"example.test/pkg","Output":"failure\n"}'
exit 23
`
			rendererScript := "#!/usr/bin/env bash\ncat >/dev/null\n"
			if err := os.WriteFile(fakeGo, []byte(goScript), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fakeRenderer, []byte(rendererScript), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "ci-go-test.sh", "lane", tc.lane)
			cmd.Env = append(os.Environ(),
				"PATH="+dir+":"+os.Getenv("PATH"),
				"FIXTURE_CALLS="+calls,
				"CI_GO_COMMAND="+realGoCommand(t),
				"CI_TESTLOG_RENDERER="+fakeRenderer,
				"CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "artifacts"),
				"CI_GO_TEST_HEARTBEAT_SECONDS=1",
			)
			out, err := cmd.CombinedOutput()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != 23 {
				t.Fatalf("lane status = %v, want native 23\n%s", err, out)
			}
			callData, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(callData)), "\n")
			if len(lines) != 1 {
				t.Fatalf("native executions = %d, want 1; calls:\n%s", len(lines), callData)
			}
			fields := strings.Fields(lines[0])
			var gotPackages []string
			for _, field := range fields {
				if strings.HasPrefix(field, "./") {
					gotPackages = append(gotPackages, field)
				}
			}
			var wantPackages []string
			for _, pkg := range manifest.Lanes[tc.lane] {
				wantPackages = append(wantPackages, toArgument(pkg))
			}
			sort.Strings(gotPackages)
			sort.Strings(wantPackages)
			if !reflect.DeepEqual(gotPackages, wantPackages) {
				t.Fatalf("lane packages = %v, want %v", gotPackages, wantPackages)
			}
			for _, field := range fields {
				if owner, exists := allPackages[field]; exists && owner != tc.lane {
					t.Errorf("lane executed package %s owned by %s", field, owner)
				}
			}
			if !strings.Contains(lines[0], tc.wantTimeout) {
				t.Errorf("lane call missing timeout %s: %s", tc.wantTimeout, lines[0])
			}
			if tc.wantP != "" {
				if !strings.Contains(lines[0], tc.wantP) {
					t.Errorf("lane call missing parallelism %s: %s", tc.wantP, lines[0])
				}
			} else if strings.Contains(lines[0], "-p=") {
				t.Errorf("lane call has unexpected parallelism: %s", lines[0])
			}
		})
	}
}

func TestCIPackageLaneCommandRejectsInvalidArguments(t *testing.T) {
	cases := [][]string{
		{"lane"},
		{"lane", "unknown"},
		{"lane", "apextest"},
		{"lane", "gladecli", "extra"},
		{"node-integration", "extra"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			cmd := exec.Command("bash", append([]string{"ci-go-test.sh"}, args...)...)
			out, err := cmd.CombinedOutput()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != 2 {
				t.Fatalf("invalid args %v status = %v, want 2\n%s", args, err, out)
			}
		})
	}
}

func TestCIVetHasOneAuthoritativeGateAndNoImplicitLaneWork(t *testing.T) {
	workflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(workflow), "go vet ./..."); got != 1 {
		t.Fatalf("standalone full vet gates = %d, want 1", got)
	}
	script, err := os.ReadFile("ci-go-test.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`go test -json -vet=off "$@"`,
		`GOFLAGS="${GOFLAGS:+${GOFLAGS} }-vet=off" go test -list '^Test' "${apex_package}"`,
	} {
		if !strings.Contains(string(script), want) {
			t.Errorf("wrapper missing implicit-vet suppression %q", want)
		}
	}
}

func TestCIGoTestLogModesPreserveFullDefaultAndCoreExcludesApex(t *testing.T) {
	for _, tc := range []struct {
		name          string
		args          []string
		wantApex      bool
		wantRace      bool
		wantTestCalls int
	}{
		{name: "default", wantApex: true, wantTestCalls: 6},
		{name: "test", args: []string{"test"}, wantApex: true, wantTestCalls: 6},
		{name: "core", args: []string{"core"}, wantTestCalls: 5},
		{name: "race", args: []string{"race"}, wantApex: true, wantRace: true, wantTestCalls: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			fakeGo := filepath.Join(dir, "go")
			fakeRenderer := filepath.Join(dir, "testlog")
			script := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
if [[ "$1" == "list" ]]; then
  printf '%s\n' github.com/glade-sh/glade/internal/apextest github.com/glade-sh/glade/internal/other
fi
`
			rendererScript := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "-output" ]]; then
    output="$2"
    shift 2
  else
    shift
  fi
done
tee "$output"
`
			if err := os.WriteFile(fakeGo, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fakeRenderer, []byte(rendererScript), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", append([]string{"ci-go-test.sh"}, tc.args...)...)
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "FIXTURE_CALLS="+calls, "CI_GO_COMMAND="+realGoCommand(t), "CI_TESTLOG_RENDERER="+fakeRenderer, "CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "artifacts"), "CI_GO_TEST_HEARTBEAT_SECONDS=1")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("mode failed: %v\n%s", err, out)
			}
			b, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			callText := string(b)
			gotApex := strings.Contains(callText, "./internal/apextest")
			if gotApex != tc.wantApex {
				t.Fatalf("Apex invocation = %v, want %v; calls:\n%s", gotApex, tc.wantApex, b)
			}
			lines := strings.Split(strings.TrimSpace(callText), "\n")
			var testCalls []string
			for _, line := range lines {
				if strings.HasPrefix(line, "test ") {
					testCalls = append(testCalls, line)
				}
			}
			if len(testCalls) != tc.wantTestCalls {
				t.Fatalf("test calls = %d, want %d; calls:\n%s", len(testCalls), tc.wantTestCalls, b)
			}
			for _, call := range testCalls {
				if !strings.Contains(call, " -json ") {
					t.Errorf("test lane bypassed JSON renderer: %s", call)
				}
				if got := strings.Contains(call, " -race "); got != tc.wantRace {
					t.Errorf("race marker = %v, want %v: %s", got, tc.wantRace, call)
				}
				if strings.Contains(call, " -skip ") || strings.Contains(call, " -skip=") {
					t.Errorf("aggregate mode filtered package coverage: %s", call)
				}
			}
			for _, pkg := range []string{"./internal/gladecli", "./internal/playground", "./internal/sema", "./internal/server", "./cmd/glade"} {
				if got := strings.Count(callText, pkg); got != 1 {
					t.Errorf("package lane %s executions = %d, want 1; calls:\n%s", pkg, got, b)
				}
			}
		})
	}
}

func TestCIGoTestLogAlwaysPreservesNativeStatus(t *testing.T) {
	for _, tc := range []struct {
		name       string
		nativeRC   int
		rendererRC int
		wantRC     int
		wantCalls  int
	}{
		{name: "both success", wantCalls: 5},
		{name: "native success renderer fail", rendererRC: 7, wantCalls: 5},
		{name: "native fail renderer success", nativeRC: 23, wantRC: 23, wantCalls: 1},
		{name: "both fail", nativeRC: 23, rendererRC: 7, wantRC: 23, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			fakeGo := filepath.Join(dir, "go")
			fakeRenderer := filepath.Join(dir, "testlog")
			goScript := `#!/usr/bin/env bash
if [[ "$1" == "list" ]]; then
  exit 0
fi
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
printf '%s\n' '{"Action":"output","Package":"example.test/pkg","Output":"ok  example.test/pkg 0.001s\n"}'
exit "$FIXTURE_NATIVE_RC"
`
			rendererScript := `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
  case "$1" in
    -output) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
tee "$output"
exit "$FIXTURE_RENDERER_RC"
`
			if err := os.WriteFile(fakeGo, []byte(goScript), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fakeRenderer, []byte(rendererScript), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "ci-go-test.sh", "core")
			cmd.Env = append(os.Environ(),
				"PATH="+dir+":"+os.Getenv("PATH"),
				"FIXTURE_CALLS="+calls,
				fmt.Sprintf("FIXTURE_NATIVE_RC=%d", tc.nativeRC),
				fmt.Sprintf("FIXTURE_RENDERER_RC=%d", tc.rendererRC),
				"CI_TESTLOG_RENDERER="+fakeRenderer,
				"CI_GO_COMMAND="+realGoCommand(t),
				"CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "artifacts"),
				"CI_GO_TEST_HEARTBEAT_SECONDS=1",
			)
			out, err := cmd.CombinedOutput()
			gotRC := 0
			if err != nil {
				exitErr, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatalf("wrapper error = %v\n%s", err, out)
				}
				gotRC = exitErr.ExitCode()
			}
			if gotRC != tc.wantRC {
				t.Fatalf("wrapper status = %d, want native status %d\n%s", gotRC, tc.wantRC, out)
			}
			callData, readErr := os.ReadFile(calls)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got := strings.Count(string(callData), "test -json"); got != tc.wantCalls {
				t.Fatalf("native test calls = %d, want %d; calls:\n%s", got, tc.wantCalls, callData)
			}
			if tc.rendererRC != 0 && !strings.Contains(string(out), "renderer failed with status 7") {
				t.Fatalf("renderer failure was not logged:\n%s", out)
			}
		})
	}
}

func TestCIApexShardLogUsesQuietRendererOnce(t *testing.T) {
	dir := t.TempDir()
	renderer := filepath.Join(dir, "testlog")
	if err := os.WriteFile(renderer, []byte("#!/usr/bin/env bash\ncat >/dev/null\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CI_TESTLOG_RENDERER", renderer)
	events := `{"Action":"output","Package":"` + fixturePackage + `","Test":"TestAlpha","Output":"=== RUN   TestAlpha\n"}` + "\n" +
		`{"Action":"output","Package":"` + fixturePackage + `","Test":"TestAlpha","Output":"    alpha_test.go:9: successful detail\n"}` + "\n" +
		passEvent("TestAlpha")
	out, err, artifacts := runApexShardFixture(t, "0", "TestAlpha\nTestBeta\nok  \t"+fixturePackage+"\n", 0, validFixturePlan(), 0, events, 0)
	if err != nil {
		t.Fatalf("apex shard failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "successful detail") || strings.Contains(out, `"Action"`) {
		t.Fatalf("Apex shard bypassed quiet renderer:\n%s", out)
	}
	calls, readErr := os.ReadFile(filepath.Join(filepath.Dir(artifacts), "calls"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got := strings.Count(string(calls), "test -json"); got != 1 {
		t.Fatalf("Apex native JSON executions = %d, want 1; calls:\n%s", got, calls)
	}
}

func TestCIGoTestLogPreservesOriginalExitCodeAndRunsOnce(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	rendererArgs := filepath.Join(dir, "renderer-args")
	fakeGo := filepath.Join(dir, "go")
	fakeRenderer := filepath.Join(dir, "testlog")
	artifactDir := filepath.Join(dir, "artifacts")
	goScript := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
if [[ "$1" == "test" ]]; then
  printf '%s\n' '{"Action":"output","Package":"example.test/pkg","Test":"TestFailure","Output":"    failure_test.go:17: deliberate failure\n"}'
  exit 23
fi
`
	rendererScript := `#!/usr/bin/env bash
printf '%s\n' "$*" > "$FIXTURE_RENDERER_ARGS"
output=
while [[ $# -gt 0 ]]; do
  case "$1" in
    -output) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
tee "$output"
exit "${FIXTURE_RENDERER_RC:-0}"
`
	if err := os.WriteFile(fakeGo, []byte(goScript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeRenderer, []byte(rendererScript), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "ci-go-test.sh", "core")
	cmd.Env = append(os.Environ(),
		"PATH="+dir+":"+os.Getenv("PATH"),
		"FIXTURE_CALLS="+calls,
		"FIXTURE_RENDERER_ARGS="+rendererArgs,
		"FIXTURE_RENDERER_RC=7",
		"CI_TESTLOG_RENDERER="+fakeRenderer,
		"CI_GO_COMMAND="+realGoCommand(t),
		"CI_GO_TEST_ARTIFACT_DIR="+artifactDir,
		"CI_GO_TEST_HEARTBEAT_SECONDS=1",
		"CI_VERBOSE=1",
	)
	out, err := cmd.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 23 {
		t.Fatalf("wrapper exit = %v, want 23\n%s", err, out)
	}
	callData, readErr := os.ReadFile(calls)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got := strings.Count(string(callData), "test -json"); got != 1 {
		t.Fatalf("underlying JSON test executions = %d, want 1; calls:\n%s", got, callData)
	}
	entries, readErr := os.ReadDir(artifactDir)
	if readErr != nil {
		t.Fatalf("read artifact directory: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("raw artifacts = %d, want 1", len(entries))
	}
	raw, readErr := os.ReadFile(filepath.Join(artifactDir, entries[0].Name()))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(raw), "deliberate failure") {
		t.Fatalf("raw artifact omitted failure output:\n%s", raw)
	}
	args, readErr := os.ReadFile(rendererArgs)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(args), "-verbose") {
		t.Fatalf("CI_VERBOSE=1 did not reach renderer: %s", args)
	}
	if !strings.Contains(string(out), "renderer failed with status 7") {
		t.Fatalf("renderer failure was not explicit:\n%s", out)
	}
}

func TestCIGoTestLogSignalsTerminateOwnedChildren(t *testing.T) {
	for _, tc := range []struct {
		name       string
		signal     syscall.Signal
		wantRC     int
		jsonRunner bool
	}{
		{name: "ordinary heartbeat TERM", signal: syscall.SIGTERM, wantRC: 143},
		{name: "ordinary heartbeat INT", signal: syscall.SIGINT, wantRC: 130},
		{name: "JSON pipeline TERM", signal: syscall.SIGTERM, wantRC: 143, jsonRunner: true},
		{name: "JSON pipeline INT", signal: syscall.SIGINT, wantRC: 130, jsonRunner: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			unrelated := exec.Command("sleep", "30")
			if err := unrelated.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = unrelated.Process.Kill()
				_ = unrelated.Wait()
			})
			worker := filepath.Join(dir, "worker")
			fakeGo := filepath.Join(dir, "go")
			fakeRenderer := filepath.Join(dir, "renderer")
			workerScript := "#!/usr/bin/env bash\nsleep 30 &\nwait\n"
			goScript := `#!/usr/bin/env bash
if [[ "$1" == "list" ]]; then exit 0; fi
sleep 30 &
wait
`
			rendererScript := "#!/usr/bin/env bash\nsleep 30 &\nwait\n"
			for path, body := range map[string]string{worker: workerScript, fakeGo: goScript, fakeRenderer: rendererScript} {
				if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
					t.Fatal(err)
				}
			}

			var cmd *exec.Cmd
			if tc.jsonRunner {
				cmd = exec.Command("bash", "ci-go-test.sh", "core")
			} else {
				cmd = exec.Command("bash", "-c", `source ./ci-go-test.sh; run_with_heartbeat "fixture heartbeat" "$1"`, "signal-fixture", worker)
			}
			cmd.Env = append(os.Environ(),
				"PATH="+dir+":"+os.Getenv("PATH"),
				"CI_GO_COMMAND="+realGoCommand(t),
				"CI_TESTLOG_RENDERER="+fakeRenderer,
				"CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "artifacts"),
				"CI_GO_TEST_HEARTBEAT_SECONDS=30",
			)
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			descendants := waitForDescendants(t, cmd.Process.Pid, 4)
			if err := cmd.Process.Signal(tc.signal); err != nil {
				t.Fatal(err)
			}
			err := cmd.Wait()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != tc.wantRC {
				t.Fatalf("wrapper exit = %v, want %d\n%s", err, tc.wantRC, output.String())
			}
			assertProcessesExit(t, descendants)
			if err := unrelated.Process.Signal(syscall.Signal(0)); err != nil {
				t.Fatalf("unrelated process was terminated: %v", err)
			}
		})
	}
}

func waitForDescendants(t *testing.T, root, minimum int) []int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pids := descendantPIDs(t, root)
		if len(pids) >= minimum {
			return pids
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("process %d did not start %d descendants", root, minimum)
	return nil
}

func descendantPIDs(t *testing.T, root int) []int {
	t.Helper()
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(root)).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		t.Fatalf("pgrep children of %d: %v", root, err)
	}
	var descendants []int
	for _, field := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(field)
		if err != nil {
			t.Fatalf("parse child pid %q: %v", field, err)
		}
		descendants = append(descendants, pid)
		descendants = append(descendants, descendantPIDs(t, pid)...)
	}
	return descendants
}

func assertProcessesExit(t *testing.T, pids []int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var alive []int
		for _, pid := range pids {
			if err := syscall.Kill(pid, 0); err == nil || err == syscall.EPERM {
				alive = append(alive, pid)
			}
		}
		if len(alive) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	var alive []int
	for _, pid := range pids {
		if err := syscall.Kill(pid, 0); err == nil || err == syscall.EPERM {
			alive = append(alive, pid)
		}
	}
	t.Fatalf("owned processes survived wrapper signal: %v", alive)
}
