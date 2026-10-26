package apextest

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type conformanceAPICase struct {
	ID         string `json:"id"`
	APIVersion string `json:"apiVersion"`
	ClassName  string `json:"className"`
	Anonymous  string `json:"anonymous"`
	Expected   string `json:"expected"`
}

const conformanceSubtestParallelism = 2

var conformanceSubtestSlots = make(chan struct{}, conformanceSubtestParallelism)

func TestConformanceAPIVersionsRunConcurrently(t *testing.T) {
	var data struct {
		APIVersions []string             `json:"apiVersions"`
		Cases       []conformanceAPICase `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/conformance/api_parallel_cases/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || len(data.Cases) != len(data.APIVersions) {
		t.Fatal("incomplete native API case table")
	}
	cases := make(map[string]conformanceAPICase, len(data.Cases))
	for _, tc := range data.Cases {
		if tc.ID == "" || tc.APIVersion == "" || tc.ClassName == "" || tc.Anonymous == "" || tc.Expected == "" {
			t.Fatalf("incomplete case: %#v", tc)
		}
		if _, exists := cases[tc.APIVersion]; exists {
			t.Fatalf("repeated API case: %s", tc.APIVersion)
		}
		cases[tc.APIVersion] = tc
		major := strings.Split(tc.APIVersion, ".")[0]
		f, err := os.Open(filepath.Join("testdata/conformance/api_parallel_cases/native", "api"+major, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(f)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if len(rows) != 2 || strings.Join(rows[0], "\t") != "api_version\tcase_id\troute\toutcome\tvalue" ||
			len(rows[1]) != 5 || rows[1][0] != tc.APIVersion || rows[1][1] != tc.ID || rows[1][2] != "anonymous" || rows[1][3] != "runtime" || rows[1][4] != tc.Expected {
			t.Fatalf("native row mismatch for API %s", tc.APIVersion)
		}
	}

	var active, peak, started atomic.Int32
	parallelLimit := conformanceTestParallelLimit()
	t.Cleanup(func() {
		runCount := int(started.Load())
		if runCount == 0 {
			return
		}
		wantParallel := min(conformanceSubtestParallelism, min(parallelLimit, runCount))
		if got := peak.Load(); got != int32(wantParallel) {
			t.Errorf("parallel API cases reached %d simultaneous runs; want %d", got, wantParallel)
		}
	})
	runConformanceAPIVersions(t, data.APIVersions, func(t *testing.T, api string) {
		started.Add(1)
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		tc, ok := cases[api]
		if !ok || tc.APIVersion != api {
			t.Fatalf("missing case for API %s", api)
		}
		conformanceAPIRunCase(t, tc)
	})
}

func conformanceTestParallelLimit() int {
	if testParallel := flag.Lookup("test.parallel"); testParallel != nil {
		if limit, err := strconv.Atoi(testParallel.Value.String()); err == nil && limit > 0 {
			return limit
		}
	}
	return 1
}

func runConformanceAPIVersions(t *testing.T, apis []string, run func(*testing.T, string)) {
	t.Helper()
	for _, api := range apis {
		api := api
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			run(t, api)
		})
	}
}

func runConformanceAPIVersion(t *testing.T, api string, run func(*testing.T)) {
	runConformanceSubtestParallel(t, api, run)
}

func runConformanceSubtestParallel(t *testing.T, name string, run func(*testing.T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		conformanceSubtestSlots <- struct{}{}
		defer func() { <-conformanceSubtestSlots }()
		run(t)
	})
}

func conformanceAPIRunCase(t *testing.T, tc conformanceAPICase) {
	t.Helper()
	fixture := filepath.Join("testdata/conformance/api_parallel_cases/native-project/force-app/main/default/classes", tc.ClassName+".cls")
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, tc.APIVersion))
	classPath := filepath.Join(root, "force-app/main/default/classes", tc.ClassName+".cls")
	writeFile(t, classPath, string(source))
	writeFile(t, classPath+"-meta.xml", "<ApexClass><apiVersion>"+tc.APIVersion+"</apiVersion></ApexClass>")
	index := loadTestIndex(t, root)
	if index.HasErrors() {
		t.Fatalf("fixture parse: %v", index.Diagnostics)
	}
	if analysis := sema.Analyze(index); analysis.HasErrors() {
		t.Fatalf("fixture analysis: %v", analysis.Diagnostics)
	}
	program, err := vm.CompileAnonymousWithOptions(tc.Anonymous, vm.CompileOptions{APIVersion: tc.APIVersion})
	if err != nil {
		t.Fatal(err)
	}
	org, err := orgFromIndexWithError(index)
	if err != nil {
		t.Fatal(err)
	}
	org.APIVersion = tc.APIVersion
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	result, err := runner.execute(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range result.Debug {
		if observed, ok := strings.CutPrefix(line, "NATIVE|"); ok {
			if observed == tc.Expected {
				return
			}
			t.Fatalf("API %s native=%q Glade=%q", tc.APIVersion, tc.Expected, observed)
		}
	}
	t.Fatalf("API %s emitted no observation", tc.APIVersion)
}
