package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type setupRenderingCapture struct {
	family, probe, testClass string
	configurations, cases    []string
}

var setupRenderingCaptures = []setupRenderingCapture{
	{"standard_profile_inventory", "ProfileInventoryProbe", "ProfileInventoryTest", []string{"plain", "communities", "communities_feature"}, []string{"inventory", "missing", "limited", "readOnly"}},
	{"email_services_metadata", "EmailServicesProbe", "EmailServicesTest", []string{"plain"}, []string{"active", "inactive", "missing", "limited"}},
	{"pdf_rendering", "PdfRenderingProbe", "PdfRenderingTest", []string{"plain"}, []string{"text", "nullInput", "emptyInput", "megabyte", "megabytePlus", "page", "missingPage"}},
}

// Capture validation stays separate from parity: an exported native observation
// is useful even while the corresponding local implementation remains incomplete.
func TestSetupRenderingNativeCaptureMatrices(t *testing.T) {
	for _, capture := range setupRenderingCaptures {
		t.Run(capture.family, func(t *testing.T) {
			rows := readSetupRenderingCaptures(t, capture)
			readSetupRenderingCaptureCarries(t, capture, rows)
		})
	}
}

func TestStandardProfileInventoryNativeConformance(t *testing.T) {
	runSetupRenderingCaptureConformance(t, setupRenderingCaptures[0])
}

func TestSetupRenderingCaptureEmailServicesNativeConformance(t *testing.T) {
	runSetupRenderingCaptureConformance(t, setupRenderingCaptures[1])
}

func TestPdfRenderingNativeConformance(t *testing.T) {
	runSetupRenderingCaptureConformance(t, setupRenderingCaptures[2])
}

func readSetupRenderingCaptures(t *testing.T, capture setupRenderingCapture) [][]string {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", capture.family)
	run, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil || len(run) == 0 {
		t.Fatalf("missing native invocation record: %v", err)
	}
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+4*len(capture.configurations)*len(capture.cases) || strings.Join(rows[0], ",") != "source_api,configuration,route,case_id,observation" {
		t.Fatalf("incomplete native matrix: %d rows", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 5 || !slices.Contains([]string{"62.0", "67.0"}, row[0]) || !slices.Contains(capture.configurations, row[1]) || !slices.Contains([]string{"anonymous", "isTest"}, row[2]) || !slices.Contains(capture.cases, row[3]) || !json.Valid([]byte(row[4])) {
			t.Fatalf("invalid native cell: %v", row)
		}
		key := strings.Join(row[:4], "/")
		if seen[key] {
			t.Fatalf("duplicate native cell: %s", key)
		}
		seen[key] = true
	}
	return rows[1:]
}

func readSetupRenderingCaptureCarries(t *testing.T, capture setupRenderingCapture, rows [][]string) map[string]string {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", "conformance", capture.family, "carried.tsv"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	carried, err := reader.ReadAll()
	if err != nil || len(carried) == 0 || strings.Join(carried[0], ",") != "source_api,configuration,route,case_id,reason" {
		t.Fatalf("invalid native carries: %v", err)
	}
	native := map[string]bool{}
	for _, row := range rows {
		native[strings.Join(row[:4], "/")] = true
	}
	reasons := map[string]string{}
	for _, row := range carried[1:] {
		if len(row) != 5 {
			t.Fatal("incomplete native carry")
		}
		key := strings.Join(row[:4], "/")
		if !native[key] || reasons[key] != "" || strings.TrimSpace(row[4]) == "" {
			t.Fatalf("invalid or duplicate native carry: %s", key)
		}
		reasons[key] = row[4]
	}
	return reasons
}

func runSetupRenderingCaptureConformance(t *testing.T, capture setupRenderingCapture) {
	t.Helper()
	rows := readSetupRenderingCaptures(t, capture)
	carries := readSetupRenderingCaptureCarries(t, capture, rows)
	// Full parity remains runnable while capture-only exports retain explicit
	// implementation gaps. Matching controls always execute in ordinary CI.
	assertCarried := os.Getenv("GLADE_ASSERT_NATIVE_CAPTURE_PARITY") == "1"
	for _, api := range []string{"62.0", "67.0"} {
		for _, configuration := range capture.configurations {
			t.Run(api+"/"+configuration, func(t *testing.T) {
				root := t.TempDir()
				project := filepath.Join("testdata", "conformance", capture.family, "project")
				if err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					name, err := filepath.Rel(project, path)
					if err != nil {
						return err
					}
					source := strings.ReplaceAll(string(body), "62.0", api)
					source = strings.ReplaceAll(source, "capture.user@example.invalid", "system@example.invalid")
					writeFile(t, filepath.Join(root, name), source)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if configuration == "communities" {
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`)
				}
				if configuration == "communities_feature" {
					definition, err := os.ReadFile(filepath.Join("testdata", "conformance", capture.family, "configurations", configuration, "project-scratch-def.json"))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				for _, row := range rows {
					if row[0] != api || row[1] != configuration {
						continue
					}
					t.Run(row[2]+"/"+row[3], func(t *testing.T) {
						if reason := carries[strings.Join(row[:4], "/")]; reason != "" && !assertCarried {
							t.Logf("CARRY: %s; native cell retained in org.tsv", reason)
							return
						}
						var payload string
						if row[2] == "isTest" {
							run := Run(index, Options{SelectedClasses: []string{capture.testClass}, SelectedMethod: row[3], NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("expected one project test observation: %+v", run.Summary())
							}
							result := run.Suites[0].Cases[0]
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("native=%s; missing named observation transport: %+v", row[4], result.Problem)
							}
							_, payload, _ = strings.Cut(result.Problem.Message, "CAPTURE|")
						} else {
							body := "System.debug('CAPTURE|'+JSON.serialize(" + capture.probe + ".observe('" + row[3] + "')));"
							if checked := sema.AnalyzeAnonymous(index, body, api); checked.HasErrors() {
								t.Fatal(checked.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							org := orgFromIndex(index)
							org.APIVersion = api
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
							result, err := runner.execute(program)
							if err != nil {
								var runtimeErr *vm.RuntimeError
								if !errors.As(err, &runtimeErr) {
									t.Fatal(err)
								}
								terminal, marshalErr := json.Marshal(map[string]string{"outcome": "uncaught-runtime", "message": runtimeErr.ExceptionMessage()})
								if marshalErr != nil {
									t.Fatal(marshalErr)
								}
								payload = string(terminal)
							}
							for _, line := range result.Debug {
								if strings.HasPrefix(line, "CAPTURE|") {
									payload = strings.TrimPrefix(line, "CAPTURE|")
								}
							}
						}
						var want, got any
						if json.Unmarshal([]byte(row[4]), &want) != nil || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("invalid observation: native=%s; Glade=%q", row[4], payload)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%s; Glade=%s", row[4], payload)
						}
					})
				}
			})
		}
	}
}
