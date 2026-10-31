package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const asyncWebserviceCalloutFixture = "testdata/conformance/async_webservice_callout"

type asyncWebserviceCalloutCase struct {
	api, route, id, method, input, observation string
}

func TestAsyncWebserviceCalloutNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	cases := readAsyncWebserviceCalloutCases(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			rows := readAsyncWebserviceCalloutOracle(t, api)
			var apiCases []asyncWebserviceCalloutCase
			var selectedClasses []string
			selected := make(map[string]bool)
			for _, tc := range cases {
				if tc.api != api {
					continue
				}
				apiCases = append(apiCases, tc)
				if row, ok := rows[tc.id]; !ok || row.route != tc.route || row.observation != tc.observation {
					t.Fatalf("case %s: native table=%+v, case table=%+v", tc.id, row, tc)
				}
				if tc.route == "istest" {
					className, _ := asyncWebserviceCalloutMethod(tc.method)
					if !selected[className] {
						selected[className] = true
						selectedClasses = append(selectedClasses, className)
					}
				}
			}
			if len(apiCases) != len(rows) || len(selectedClasses) == 0 {
				t.Fatalf("API %s: cases=%d native rows=%d @IsTest classes=%d", api, len(apiCases), len(rows), len(selectedClasses))
			}

			projectRoot := copyAsyncWebserviceCalloutProject(t, api, "project")
			index := loadTestIndex(t, projectRoot)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			testCases := Discover(index, Options{SelectedClasses: selectedClasses})
			methodCases := make(map[string]TestCase, len(testCases))
			for _, testCase := range testCases {
				methodCases[testCase.ClassName+"."+testCase.MethodName] = testCase
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, testCases)
			programs, programErrors := compileTestInvokePrograms(testCases)
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			seenMethods := make(map[string]bool)
			for _, tc := range apiCases {
				t.Run(tc.route+"/"+tc.id, func(t *testing.T) {
					switch tc.route {
					case "istest":
						className, methodName := asyncWebserviceCalloutMethod(tc.method)
						qualified := className + "." + methodName
						if seenMethods[qualified] {
							t.Fatalf("duplicate @IsTest case %s", qualified)
						}
						seenMethods[qualified] = true
						testCase, ok := methodCases[qualified]
						if !ok {
							t.Fatalf("missing @IsTest method %s", qualified)
						}
						key := testCaseKey(testCase)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: method=%v invoke=%v", methodErrors[key], programErrors[key])
						}
						machine := base.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						if _, err := machine.ExecuteTestInClass(programs[key], testCase.ClassName); err != nil {
							t.Fatalf("native=%s; Glade: %v", tc.observation, err)
						}
					case "anonymous", "anonymous-reject", "anonymous_reject":
						source := readAsyncWebserviceCalloutSource(t, api, tc.input)
						analysis := sema.AnalyzeAnonymous(index, source, api)
						if tc.route != "anonymous" || strings.HasPrefix(tc.observation, "REJECT|") {
							assertAsyncWebserviceCalloutRejection(t, tc.observation, analysis.Diagnostics)
							return
						}
						if analysis.HasErrors() {
							t.Fatalf("native anonymous accepted; Glade: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := base.newMachine().Execute(program); err != nil {
							t.Fatalf("native=%s; Glade: %v", tc.observation, err)
						}
					case "deploy":
						rejectedRoot := copyAsyncWebserviceCalloutProject(t, api, "rejected/project")
						rejected := loadTestIndex(t, rejectedRoot)
						rejectedAnalysis := sema.Analyze(rejected)
						assertAsyncWebserviceCalloutRejection(t, tc.observation, rejected.Diagnostics, rejectedAnalysis.Diagnostics)
					default:
						t.Fatalf("unknown route %q", tc.route)
					}
				})
			}
			if len(seenMethods) != len(testCases) {
				t.Fatalf("@IsTest rows=%d discovered=%d", len(seenMethods), len(testCases))
			}

			for _, mode := range []string{"uncached", "cached_cold", "cached_memory_warm", "cached_disk_warm"} {
				t.Run("project_runner/"+mode, func(t *testing.T) {
					if mode != "cached_memory_warm" {
						InvalidateRuntimeCaches()
					}
					ResetPerfCounters()
					// Source API comes from the copied class metadata. The runner's
					// REST execution context is a separate, supported host version.
					run := Run(index, Options{SelectedClasses: selectedClasses, RuntimeRESTAPIVersion: "67.0",
						NoDiskCache: mode == "uncached", Parallelism: 1, PerfCounters: true})
					if got := run.Summary(); got.Total != len(testCases) || got.Passed != len(testCases) || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
						for _, suite := range run.Suites {
							for _, result := range suite.Cases {
								if result.Problem != nil {
									t.Logf("%s.%s: %s", result.ClassName, result.MethodName, result.Problem.Message)
								}
							}
						}
						t.Fatalf("project route failed: summary=%+v", got)
					}
					phases := SnapshotPerfCounters().Phases
					if mode == "cached_memory_warm" && phases.MemoryCacheHits == 0 {
						t.Fatalf("warm route did not use memory cache: %+v", phases)
					}
					if mode == "cached_disk_warm" && phases.DiskCacheHits != 1 {
						t.Fatalf("restored route did not use disk cache: %+v", phases)
					}
				})
			}
		})
	}
}

func asyncWebserviceCalloutMethod(method string) (string, string) {
	if className, methodName, qualified := strings.Cut(method, "."); qualified {
		return className, methodName
	}
	return "LongtailAsyncSoapTest", method
}

func assertAsyncWebserviceCalloutRejection(t *testing.T, observation string, groups ...[]diagnostic.Diagnostic) {
	t.Helper()
	want, ok := strings.CutPrefix(observation, "REJECT|")
	if !ok || want == "" {
		t.Fatalf("missing native rejection: %q", observation)
	}
	count := 0
	for _, diagnostics := range groups {
		for _, got := range diagnostics {
			if got.Severity != diagnostic.Error {
				continue
			}
			count++
			if got.Message != want {
				t.Errorf("native diagnostic=%q; Glade=%q (%s)", want, got.Message, got.Code)
			}
		}
	}
	if count == 0 {
		t.Fatalf("native rejects with %q, but Glade accepted", want)
	}
}

func readAsyncWebserviceCalloutSource(t *testing.T, api, path string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(asyncWebserviceCalloutFixture, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(string(body), "67.0", api)
	run, err := os.ReadFile(filepath.Join(asyncWebserviceCalloutFixture, "native", "api"+strings.TrimSuffix(api, ".0"), "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(run), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(path), sha256.Sum256([]byte(source)))) {
		t.Fatalf("native source changed: %s", path)
	}
	return source
}

func readAsyncWebserviceCalloutCases(t *testing.T) []asyncWebserviceCalloutCase {
	t.Helper()
	f, err := os.Open(filepath.Join(asyncWebserviceCalloutFixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], "\t") != "api_version\troute\tcase_id\tmethod\tinput_shape\tnative_observation" {
		t.Fatalf("case table has %d rows or an invalid header", len(rows))
	}
	out := make([]asyncWebserviceCalloutCase, 0, len(rows)-1)
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		if len(row) != 6 || (row[0] != "62.0" && row[0] != "67.0") || row[2] == "" || seen[row[0]+"/"+row[2]] {
			t.Fatalf("invalid async webservice callout case row: %v", row)
		}
		seen[row[0]+"/"+row[2]] = true
		out = append(out, asyncWebserviceCalloutCase{row[0], row[1], row[2], row[3], row[4], row[5]})
	}
	return out
}

func readAsyncWebserviceCalloutOracle(t *testing.T, api string) map[string]asyncWebserviceCalloutCase {
	t.Helper()
	apiName := strings.TrimSuffix(api, ".0")
	oraclePath := filepath.Join(asyncWebserviceCalloutFixture, "native", "api"+apiName)
	run, err := os.ReadFile(filepath.Join(oraclePath, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(run), "native_complete=true\n") || !strings.Contains(string(run), "api="+api+"\n") {
		t.Fatalf("incomplete native capture for API %s", api)
	}
	f, err := os.Open(filepath.Join(oraclePath, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], "\t") != "route\tcase_id\tobservation" {
		t.Fatalf("native API %s table has %d rows or invalid header", api, len(rows))
	}
	out := make(map[string]asyncWebserviceCalloutCase, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) != 3 || row[1] == "" || out[row[1]].id != "" {
			t.Fatalf("invalid native row for API %s: %v", api, row)
		}
		out[row[1]] = asyncWebserviceCalloutCase{api: api, route: row[0], id: row[1], observation: row[2]}
	}
	return out
}

func copyAsyncWebserviceCalloutProject(t *testing.T, api, subdir string) string {
	t.Helper()
	apiName := strings.TrimSuffix(api, ".0")
	run, err := os.ReadFile(filepath.Join(asyncWebserviceCalloutFixture, "native", "api"+apiName, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(run), "native_complete=true\n") || !strings.Contains(string(run), "api="+api+"\n") {
		t.Fatalf("incomplete native capture for API %s", api)
	}
	sourceRoot := filepath.Join(asyncWebserviceCalloutFixture, filepath.FromSlash(subdir))
	targetRoot := t.TempDir()
	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source = []byte(strings.ReplaceAll(string(source), "67.0", api))
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		captureKey := filepath.ToSlash(filepath.Join(filepath.ToSlash(subdir), rel))
		digest := fmt.Sprintf("source_sha256:%s=%x\n", captureKey, sha256.Sum256(source))
		if !strings.Contains(string(run), digest) {
			return fmt.Errorf("native source changed: %s", captureKey)
		}
		out := filepath.Join(targetRoot, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, source, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	return targetRoot
}
