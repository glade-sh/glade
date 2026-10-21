package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type setCustomIdentityCell struct {
	APIVersion         string `json:"apiVersion"`
	Context            string `json:"context"`
	HelperAPIVersion   string `json:"helperAPIVersion"`
	Anonymous          string `json:"anonymous"`
	TestClass          string `json:"testClass"`
	TestMethod         string `json:"testMethod"`
	ClassSource        string `json:"classSource"`
	SourceProvenance   string `json:"sourceProvenance"`
	SourceSHA256       string `json:"sourceSha256"`
	HelperSourceSHA256 string `json:"helperSourceSha256"`
	EvidenceSHA256     string `json:"evidenceSha256"`
	StdoutSHA256       string `json:"stdoutSha256"`
	StderrSHA256       string `json:"stderrSha256"`
	NativeEnvelope     struct {
		API             string          `json:"api"`
		CaseTableRowID  string          `json:"case_table_row_id"`
		InvocationRoute string          `json:"invocation_route"`
		Outcome         string          `json:"outcome"`
		Value           json.RawMessage `json:"value"`
	} `json:"nativeEnvelope"`
}

// Execute retained anonymous source and explicitly reconstructed named wrappers
// against the complete native payload. Named full-request bytes were not
// retained; the source provenance distinguishes that from captured source.
func TestSetCustomIdentityConformance(t *testing.T) {
	var data struct {
		APIVersions               []string `json:"apiVersions"`
		ShapeCount                int      `json:"shapeCount"`
		NativeObservationCount    int      `json:"nativeObservationCount"`
		RequestedObservationCount int      `json:"requestedObservationCount"`
		CarriedObservationCount   int      `json:"carriedObservationCount"`
		PendingObservationCount   int      `json:"pendingObservationCount"`
		Marker                    string   `json:"marker"`
		Helper                    struct {
			Name               string `json:"name"`
			Source             string `json:"source"`
			SourceSHA256       string `json:"sourceSha256"`
			APIVersionRule     string `json:"apiVersionRule"`
			MetadataProvenance string `json:"metadataProvenance"`
		} `json:"helper"`
		Rows []struct {
			ID         string                  `json:"id"`
			SourceRow  string                  `json:"sourceRow"`
			Body       string                  `json:"body"`
			BodySHA256 string                  `json:"bodySha256"`
			Cells      []setCustomIdentityCell `json:"cells"`
		} `json:"rows"`
		Carries []struct {
			ID        string `json:"id"`
			SourceRow string `json:"sourceRow"`
			Reason    string `json:"reason"`
			Evidence  struct {
				Document       string `json:"document"`
				DocumentSHA256 string `json:"documentSha256"`
				Quote          string `json:"quote"`
			} `json:"evidence"`
			Cells []struct {
				APIVersion   string          `json:"apiVersion"`
				Context      string          `json:"context"`
				NativeStatus string          `json:"nativeStatus"`
				Expected     json.RawMessage `json:"expected"`
			} `json:"cells"`
		} `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/set_custom_identity.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.ShapeCount != 17 || len(data.Rows) != 17 || data.NativeObservationCount != 102 || data.RequestedObservationCount != 108 || data.CarriedObservationCount != 6 || data.PendingObservationCount != 6 || data.Marker == "" || len(data.Carries) != 1 {
		t.Fatal("incomplete native/carry matrix")
	}
	checkDigest := func(name, digest string) {
		t.Helper()
		bytes, err := hex.DecodeString(digest)
		if err != nil || len(bytes) != sha256.Size {
			t.Fatalf("invalid evidence digest: %s", name)
		}
	}
	checkSource := func(name, source, digest string) {
		t.Helper()
		checkDigest(name, digest)
		sum := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(sum[:]) != digest {
			t.Fatalf("fixture source changed: %s", name)
		}
	}
	checkSource(data.Helper.Name, data.Helper.Source, data.Helper.SourceSHA256)
	if data.Helper.APIVersionRule == "" || data.Helper.MetadataProvenance == "" {
		t.Fatal("missing helper metadata provenance")
	}
	seenRows, nativeCells := map[string]bool{}, 0
	for _, row := range data.Rows {
		if row.ID == "" || seenRows[row.ID] || len(row.Cells) != 6 || row.SourceRow == "" {
			t.Fatal("missing/repeated native row")
		}
		seenRows[row.ID] = true
		checkSource(row.ID, row.Body, row.BodySHA256)
		seenCells := map[string]bool{}
		for _, cell := range row.Cells {
			key := cell.APIVersion + "/" + cell.Context
			if seenCells[key] || !strings.Contains(",62.0,67.0,68.0,", ","+cell.APIVersion+",") || (cell.Context != "anonymous" && cell.Context != "isTest") {
				t.Fatal("missing/repeated API route")
			}
			seenCells[key] = true
			if cell.HelperAPIVersion != cell.APIVersion || cell.HelperSourceSHA256 != data.Helper.SourceSHA256 || cell.SourceProvenance == "" {
				t.Fatal("helper/source metadata mismatch")
			}
			checkDigest(row.ID, cell.EvidenceSHA256)
			checkDigest(row.ID, cell.StdoutSHA256)
			checkDigest(row.ID, cell.StderrSHA256)
			if cell.Context == "anonymous" {
				checkSource(row.ID, cell.Anonymous, cell.SourceSHA256)
				if !strings.Contains(cell.SourceProvenance, "Exact executeAnonymous") || cell.NativeEnvelope.InvocationRoute != "executeAnonymous" || !strings.Contains(cell.Anonymous, row.Body) {
					t.Fatal("missing retained anonymous source")
				}
			} else {
				checkSource(row.ID, cell.ClassSource, cell.SourceSHA256)
				if !strings.Contains(cell.SourceProvenance, "Reconstructed named wrapper") || cell.NativeEnvelope.InvocationRoute != "metadata-check-only" || cell.TestClass == "" || cell.TestMethod == "" || !strings.Contains(cell.ClassSource, row.Body) {
					t.Fatal("missing reconstructed named source provenance")
				}
			}
			if cell.NativeEnvelope.API != cell.APIVersion || cell.NativeEnvelope.CaseTableRowID != row.ID || cell.NativeEnvelope.Outcome != "runtime" || !json.Valid(cell.NativeEnvelope.Value) {
				t.Fatal("missing independent native envelope")
			}
			nativeCells++
		}
	}
	if nativeCells != data.NativeObservationCount {
		t.Fatal("native cell count differs")
	}
	pendingCells := 0
	for _, carry := range data.Carries {
		checkDigest(carry.ID, carry.Evidence.DocumentSHA256)
		if carry.ID != "namespace-twin" || carry.SourceRow != "F.6" || carry.Reason == "" || carry.Evidence.Document != "README.md" || carry.Evidence.Quote != "Namespaced F.6 is not run under explicit no-package/no-namespace scope." || len(carry.Cells) != 6 {
			t.Fatal("missing explicit namespace carry evidence")
		}
		seenCells := map[string]bool{}
		for _, cell := range carry.Cells {
			key := cell.APIVersion + "/" + cell.Context
			if seenCells[key] || !strings.Contains(",62.0,67.0,68.0,", ","+cell.APIVersion+",") || (cell.Context != "anonymous" && cell.Context != "isTest") || cell.NativeStatus != "NOT_RUN" || string(cell.Expected) != "null" {
				t.Fatal("namespace carry has a fabricated native answer")
			}
			seenCells[key] = true
			pendingCells++
		}
	}
	if pendingCells != data.PendingObservationCount || pendingCells != data.CarriedObservationCount || nativeCells+pendingCells != data.RequestedObservationCount {
		t.Fatal("pending/carry count differs")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "SetCustomIdentity", api)
			baseIndex := listSObjectCastIndex(t, api, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, cell := range row.Cells {
						if cell.APIVersion != api {
							continue
						}
						counts.Run(t, cell.Context, row.ID, cell.Context, func(t *testing.T) {
							classes := map[string]string{data.Helper.Name: data.Helper.Source}
							if cell.Context == "isTest" {
								classes[cell.TestClass] = cell.ClassSource
							}
							index := listSObjectCastIndex(t, api, classes)
							analysis := sema.Analyze(index)
							diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
							want := listSObjectCastExpected{Outcome: "runtime", Value: string(cell.NativeEnvelope.Value)}
							var execErr error
							if cell.Context == "anonymous" {
								anonymous := sema.AnalyzeAnonymous(index, cell.Anonymous, api)
								diagnostics = append(diagnostics, anonymous.Diagnostics...)
								program, compileErr := vm.CompileAnonymousWithOptions(cell.Anonymous, vm.CompileOptions{APIVersion: api})
								listSObjectCastCompile(t, want, diagnostics, compileErr)
								runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
								if err != nil {
									t.Fatal(err)
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: &org})
								_, execErr = runner.execute(program)
							} else {
								listSObjectCastCompile(t, want, diagnostics, nil)
								cases := Discover(index, Options{SelectedClasses: []string{cell.TestClass}, SelectedMethod: cell.TestMethod})
								if len(cases) != 1 {
									t.Fatalf("observation discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethodsForIndex(&index, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							collectionCastReflectionValue(t, data.Marker+row.ID+"|", want, execErr)
						})
					}
				})
			}
			for _, carry := range data.Carries {
				for _, cell := range carry.Cells {
					if cell.APIVersion != api {
						continue
					}
					counts.RunKind(t, cell.Context, carry.ID, carry.ID+"/"+cell.Context, "carried", func(t *testing.T) {
						t.Logf("native NOT_RUN; still pending: %s; evidence %s: %s", carry.Reason, carry.Evidence.Document, carry.Evidence.Quote)
					})
				}
			}
			t.Log("namespaced twin: carried 2 requested cells; pending 2; native observations 0")
		})
	}
}
