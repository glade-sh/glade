package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type invocableFlowProjectAnswer struct {
	invocableFlowActionAnswer
	Primary    string                     `json:"primary"`
	Native     json.RawMessage            `json:"native"`
	ModernTwin *invocableFlowActionAnswer `json:"modernTwin"`
}

type invocableFlowProjectSource struct {
	Code                string                     `json:"code"`
	OperationSHA256     string                     `json:"operationSHA256"`
	CaptureSourceSHA256 map[string]string          `json:"captureSourceSHA256"`
	Expected            invocableFlowProjectAnswer `json:"expected"`
	Admission           string                     `json:"admission"`
	Note                string                     `json:"note"`
}

type invocableFlowProjectAPI struct {
	Routes              map[string]invocableFlowProjectSource `json:"routes"`
	RejectedFile        string                                `json:"rejectedFile"`
	RejectedDestination string                                `json:"rejectedDestination"`
}

type invocableFlowProjectRow struct {
	ID    string                             `json:"id"`
	ByAPI map[string]invocableFlowProjectAPI `json:"byAPI"`
}

func runInvocableFlowProjectConformance(t *testing.T, group string, shapes, cells, recovered int) {
	t.Helper()
	var data struct {
		Group                     string                    `json:"group"`
		APIVersions               []string                  `json:"apiVersions"`
		ShapeCount                int                       `json:"shapeCount"`
		NativeObservationCount    int                       `json:"nativeObservationCount"`
		RecoveredObservationCount int                       `json:"recoveredObservationCount"`
		Rows                      []invocableFlowProjectRow `json:"rows"`
	}
	fixtureDir := filepath.Join("testdata", "invocable_flow_project", "captured")
	raw, err := os.ReadFile(filepath.Join(fixtureDir, group+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Group != group || data.ShapeCount != shapes || len(data.Rows) != shapes || data.NativeObservationCount != cells || data.RecoveredObservationCount != recovered || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatal("incomplete native project matrix")
	}
	seen := map[string]bool{}
	observations, repairs := 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.ByAPI) != 3 {
			t.Fatal("invalid native shape binding")
		}
		seen[row.ID] = true
		for _, api := range data.APIVersions {
			fixture, ok := row.ByAPI[api]
			if !ok || len(fixture.Routes) != 2 {
				t.Fatal("missing API or route")
			}
			for _, route := range []string{"anonymous", "istest"} {
				source, ok := fixture.Routes[route]
				sum := sha256.Sum256([]byte(source.Code))
				if !ok || source.Code == "" || hex.EncodeToString(sum[:]) != source.OperationSHA256 || len(source.CaptureSourceSHA256) != 1 {
					t.Fatal("changed synthetic capture operation")
				}
				switch source.Admission {
				case "observed":
				case "recovered":
					if source.Note == "" {
						t.Fatal("missing verified recovery note")
					}
					repairs++
				default:
					t.Fatal("unobserved answer admitted")
				}
				switch source.Expected.Outcome {
				case "runtime", "uncaught_anonymous", "uncaught_test_failure":
				case "flow-deploy-rejected":
					if source.Expected.Primary == "" || !json.Valid(source.Expected.Native) || source.Expected.ModernTwin == nil || fixture.RejectedFile == "" || fixture.RejectedDestination == "" {
						t.Fatal("incomplete rejected metadata and modern twin")
					}
				default:
					t.Fatal("unsupported native outcome")
				}
				observations++
			}
		}
	}
	if observations != cells || repairs != recovered {
		t.Fatal("native matrix count changed")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "InvocableFlowProject-"+group, api)
			root := invocableFlowObservedProject(t, filepath.Join(fixtureDir, "api"+strings.TrimSuffix(api, ".0")))
			for n, row := range data.Rows {
				name := "FlowProjectObserved" + strconv.Itoa(n)
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, "@IsTest private class "+name+" { @IsTest static void observe(){"+row.ByAPI[api].Routes["istest"].Code+"} }")
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			invocableFlowActionCompile(t, append(index.Diagnostics, analysis.Diagnostics...))
			org := orgFromIndex(index)
			org.APIVersion = api
			if len(org.Metadata.Flows) != 9 {
				t.Fatalf("project loaded %d accepted autolaunched flows; want 9", len(org.Metadata.Flows))
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(runtime.Triggers) != 12 {
				t.Fatalf("project requires two six-event trigger declarations: %v", runtime.Triggers)
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			for n, row := range data.Rows {
				shape := "shape" + strconv.Itoa(n)
				t.Run(shape, func(t *testing.T) {
					fixture := row.ByAPI[api]
					for _, route := range []string{"anonymous", "istest"} {
						source := fixture.Routes[route]
						kind := "exact"
						if source.Expected.Outcome == "flow-deploy-rejected" {
							kind = "category"
						} else if api != "62.0" && strings.Contains(string(source.Expected.Value), "<a href='[host]'>SOAP API Developer Guide") {
							kind = "partial"
						}
						counts.RunKind(t, route, shape, route, kind, func(t *testing.T) {
							want := source.Expected.invocableFlowActionAnswer
							if source.Expected.Outcome == "flow-deploy-rejected" {
								invocableFlowProjectRejectedMetadata(t, root, fixtureDir, api, fixture, source.Expected.Primary)
								want = *source.Expected.ModernTwin
							}
							var execErr error
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(index, source.Code, api)
								invocableFlowActionCompile(t, analysis.Diagnostics)
								program, err := vm.CompileAnonymousWithOptions(source.Code, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, execErr = base.execute(program)
							} else {
								name := "FlowProjectObserved" + strconv.Itoa(n)
								cases := Discover(index, Options{SelectedClasses: []string{name}, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatal("missing independent named route")
								}
								methods, methodErrors := compileTestMethodsForIndex(&index, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								machine := base.newMachine()
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								_, execErr = machine.ExecuteInClass(programs[key], name)
							}
							invocableFlowProjectCompare(t, row.ID, api, want, execErr)
						})
					}
				})
			}
		})
	}
}

func invocableFlowObservedProject(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if relative == "rejected" {
				return filepath.SkipDir
			}
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, relative), string(raw))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return root
}

func invocableFlowProjectRejectedMetadata(t *testing.T, root, fixtureDir, api string, fixture invocableFlowProjectAPI, primary string) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	rejectedRoot := t.TempDir()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, "api"+strings.TrimSuffix(api, ".0"), fixture.RejectedFile))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(rejectedRoot, fixture.RejectedDestination)
	writeFile(t, path, string(raw))
	p.FlowFiles = append(p.FlowFiles, path)
	automations, err := automation.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range automations.Diagnostics {
		if d.Severity == diagnostic.Error && d.File == path && d.Message == primary {
			return
		}
	}
	t.Fatalf("native rejects the null-global assignment; project loader accepted it: %v", automations.Diagnostics)
}

func invocableFlowProjectCompare(t *testing.T, id, api string, want invocableFlowActionAnswer, execErr error) {
	t.Helper()
	if want.Outcome != "runtime" {
		invocableFlowActionCompare(t, id, api, want, execErr)
		return
	}
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("missing native payload outcome: %v", execErr)
	}
	_, payload, ok := strings.Cut(runtimeErr.ExceptionMessage(), "C126_RESULT|"+id+"|")
	if !ok {
		t.Fatalf("missing whole-payload marker: %v", execErr)
	}
	var got, expected any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want.Value, &expected); err != nil {
		t.Fatal(err)
	}
	got, expected = invocableFlowActionNormalize(got), invocableFlowActionNormalize(expected)
	if api != "62.0" {
		expected = invocableFlowProjectGuide(t, expected)
	}
	if !reflect.DeepEqual(got, expected) {
		g, _ := json.Marshal(got)
		w, _ := json.Marshal(expected)
		t.Fatalf("native %s; Glade %s", w, g)
	}
}

// Native uses the org-specific guide anchor at newer source APIs. Compare its
// stable surrounding text and every other payload field, retaining plain text
// locally. This also preserves exact caller DML wrappers around Flow errors.
func invocableFlowProjectGuide(t *testing.T, value any) any {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			v[key] = invocableFlowProjectGuide(t, item)
		}
	case []any:
		for n, item := range v {
			v[n] = invocableFlowProjectGuide(t, item)
		}
	case string:
		const anchorStart = "<a href='[host]'>SOAP API Developer Guide"
		if strings.Contains(v, anchorStart) {
			anchor := anchorStart + "</a>"
			// Anonymous observations retain the reader's exact closing-tag scrub;
			// recovered named observations retain the original closing anchor.
			if strings.Contains(v, anchorStart+"<[redacted-path]>") {
				anchor = anchorStart + "<[redacted-path]>"
			}
			if strings.Count(v, anchor) != 1 || !strings.Contains(v, "This error occurred when the flow tried to create records: ") || !strings.Contains(v, ". You can look up ExceptionCode values in the "+anchor+".") {
				t.Fatal("native Flow fault guide anchor changed")
			}
			return strings.Replace(v, anchor, "SOAP API Developer Guide", 1)
		}
	}
	return value
}

func TestInvocableFlowProjectControlsConformance(t *testing.T) {
	runInvocableFlowProjectConformance(t, "controls", 3, 18, 0)
}
func TestInvocableFlowProjectExecutionConformance(t *testing.T) {
	runInvocableFlowProjectConformance(t, "execution", 1, 6, 0)
}
func TestInvocableFlowProjectTriggersConformance(t *testing.T) {
	runInvocableFlowProjectConformance(t, "triggers", 10, 60, 0)
}
func TestInvocableFlowProjectMissingConformance(t *testing.T) {
	runInvocableFlowProjectConformance(t, "missing", 4, 24, 0)
}
func TestInvocableFlowProjectFaultsConformance(t *testing.T) {
	runInvocableFlowProjectConformance(t, "faults", 4, 24, 8)
}
func TestInvocableFlowProjectMetadataConformance(t *testing.T) {
	runInvocableFlowProjectConformance(t, "metadata", 3, 18, 0)
}
func TestInvocableFlowProjectCreationConformance(t *testing.T) {
	runInvocableFlowProjectConformance(t, "creation", 3, 18, 4)
}
