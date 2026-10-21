package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type catalogBoundaryNativeFixture struct {
	ProjectAPI, ProjectSource, ProjectSourceSHA256                   string
	RowCount, ScalarCells, ExpectedMethods, MatchedRows, CarriedRows int
	OrgIDSHA256, OrgTSVSHA256, OrgRunSHA256, TestClass, Projection   string
	Sources, SourceSHA256                                            map[string]string
	AnonymousSource, AnonymousSourceSHA256                           string
	CarriedIDs                                                       map[string]string
	OrgContext                                                       struct {
		RequestedEdition, OtherFeatureContext string
		FeaturesRequested                     []string
		SettingsChanged                       *bool
	}
	Captures []struct {
		API, TransportAPI, Metadata, MetadataSHA256, DeploymentID, DeploymentStatus, DeploymentReceiptSHA256 string
		CheckOnly                                                                                            bool
	}
	Rows []struct {
		API, TransportAPI, Route, ID, ReceiptSHA256 string
		Native                                      map[string]any
	}
	Uncaptured []string
}

type catalogBoundaryNativeSpec struct {
	file, observer, testClass                            string
	rows, cells, methods, matched, carried, matchedCells int
	routes                                               map[string][]string
	carriedIDs                                           []string
}

func builtinPricebookNativeSpec() catalogBoundaryNativeSpec {
	return catalogBoundaryNativeSpec{
		file: "builtin_pricebook_id_native.json", observer: "OwnedStoreBoundaryObserver", testClass: "OwnedStoreBoundaryTest",
		rows: 20, cells: 64, methods: 3, matched: 12, carried: 8, matchedCells: 56,
		routes:     map[string][]string{"BP01": {"istest"}, "BP02": {"anonymous", "istest"}, "WS01": {"anonymous", "istest"}},
		carriedIDs: []string{"WS01"},
	}
}

func loadCatalogBoundaryNativeFixture(t *testing.T, spec catalogBoundaryNativeSpec) catalogBoundaryNativeFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", spec.file))
	if err != nil {
		t.Fatal(err)
	}
	var data catalogBoundaryNativeFixture
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func validateCatalogBoundaryNativeFixture(t *testing.T, spec catalogBoundaryNativeSpec) {
	t.Helper()
	data := loadCatalogBoundaryNativeFixture(t, spec)
	digest := func(value string) {
		t.Helper()
		if decoded, err := hex.DecodeString(value); err != nil || len(decoded) != sha256.Size {
			t.Fatal("invalid native receipt binding")
		}
	}
	checkHash := func(source, want string) {
		t.Helper()
		hash := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(hash[:]) != want {
			t.Fatal("native source binding changed")
		}
	}
	if data.RowCount != spec.rows || len(data.Rows) != spec.rows || data.ScalarCells != spec.cells || data.ExpectedMethods != spec.methods || data.MatchedRows != spec.matched || data.CarriedRows != spec.carried || data.TestClass != spec.testClass || data.Projection == "" {
		t.Fatal("native matrix contract changed")
	}
	if len(data.Sources) != 2 || len(data.SourceSHA256) != 2 || len(data.Captures) != 4 {
		t.Fatal("native source or capture matrix incomplete")
	}
	for _, name := range []string{spec.observer, spec.testClass} {
		checkHash(data.Sources[name], data.SourceSHA256[name])
	}
	if data.OrgContext.RequestedEdition != "Developer" || data.OrgContext.OtherFeatureContext != "UNCAPTURED" || data.OrgContext.FeaturesRequested == nil || len(data.OrgContext.FeaturesRequested) != 0 || data.OrgContext.SettingsChanged == nil || *data.OrgContext.SettingsChanged {
		t.Fatal("native org context changed")
	}
	for _, value := range []string{data.OrgIDSHA256, data.OrgTSVSHA256, data.OrgRunSHA256} {
		digest(value)
	}
	checkHash(data.AnonymousSource, data.AnonymousSourceSHA256)
	checkHash(data.ProjectSource, data.ProjectSourceSHA256)
	var project struct {
		API string `json:"sourceApiVersion"`
	}
	if json.Unmarshal([]byte(data.ProjectSource), &project) != nil || data.ProjectAPI != "62.0" || project.API != data.ProjectAPI {
		t.Fatal("native project API pin changed")
	}
	apis := []string{"60.0", "62.0", "67.0", "68.0"}
	wantedAPIs := make(map[string]bool)
	for _, api := range apis {
		wantedAPIs[api] = true
	}
	seenAPI := make(map[string]bool)
	for _, capture := range data.Captures {
		if !wantedAPIs[capture.API] || seenAPI[capture.API] || capture.API != capture.TransportAPI || capture.CheckOnly || capture.DeploymentID == "" || capture.DeploymentStatus != "Succeeded" {
			t.Fatal("invalid native capture")
		}
		seenAPI[capture.API] = true
		checkHash(capture.Metadata, capture.MetadataSHA256)
		digest(capture.DeploymentReceiptSHA256)
		if !strings.Contains(capture.Metadata, "<apiVersion>"+capture.API+"</apiVersion>") {
			t.Fatal("native component API changed")
		}
	}
	if len(data.CarriedIDs) != len(spec.carriedIDs) {
		t.Fatal("native carries changed")
	}
	for _, id := range spec.carriedIDs {
		if strings.TrimSpace(data.CarriedIDs[id]) == "" {
			t.Fatal("missing native carry reason", id)
		}
	}
	wantedRows := make(map[string]bool)
	for _, api := range apis {
		for id, routes := range spec.routes {
			for _, route := range routes {
				wantedRows[api+"/"+route+"/"+id] = true
			}
		}
	}
	cells, matched, carried, matchedCells := 0, 0, 0, 0
	for _, row := range data.Rows {
		key := row.API + "/" + row.Route + "/" + row.ID
		if !wantedRows[key] || !seenAPI[row.API] || row.API != row.TransportAPI {
			t.Fatal("unexpected or duplicate native row", key)
		}
		delete(wantedRows, key)
		digest(row.ReceiptSHA256)
		leaves := roleGroupObservationLeaves(row.Native)
		cells += leaves
		if data.CarriedIDs[row.ID] != "" {
			carried++
			want := map[string]any{"objectPresent": false, "fields": map[string]any{}}
			if row.ID != "WS01" || !reflect.DeepEqual(row.Native, want) {
				t.Fatal("native unavailable-surface observation changed", key)
			}
		} else {
			matched++
			matchedCells += leaves
		}
	}
	if len(wantedRows) != 0 || cells != spec.cells || matched != spec.matched || carried != spec.carried || matchedCells != spec.matchedCells {
		t.Fatalf("native coverage changed: missing=%d cells=%d matched=%d carried=%d matchedCells=%d", len(wantedRows), cells, matched, carried, matchedCells)
	}
	for _, scope := range data.Uncaptured {
		t.Log("UNCAPTURED", scope)
	}
}

func TestBuiltinPricebookIDNativeFixture(t *testing.T) {
	validateCatalogBoundaryNativeFixture(t, builtinPricebookNativeSpec())
}

func TestBuiltinPricebookIDNativeConformance(t *testing.T) {
	runCatalogBoundaryNativeConformance(t, builtinPricebookNativeSpec())
}

func runCatalogBoundaryNativeConformance(t *testing.T, spec catalogBoundaryNativeSpec) {
	t.Helper()
	data := loadCatalogBoundaryNativeFixture(t, spec)
	for _, capture := range data.Captures {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), data.ProjectSource)
		for name, source := range data.Sources {
			path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
			writeFile(t, path, source)
			writeFile(t, path+"-meta.xml", capture.Metadata)
		}
		index := loadTestIndex(t, root)
		analysis := sema.Analyze(index)
		if index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
		}
		cases := Discover(index, Options{SelectedClasses: []string{data.TestClass}})
		if len(cases) != spec.methods {
			t.Fatal("native named selection changed")
		}
		methods, methodErrors := compileTestMethods(cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		for _, row := range data.Rows {
			if row.API != capture.API {
				continue
			}
			t.Run(row.API+"/"+row.Route+"/"+row.ID, func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = capture.TransportAPI
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				marker := "STORE_BOUNDARY|" + row.ID + "|"
				payload := ""
				var observationErr error
				if row.Route == "anonymous" {
					source := ""
					for _, line := range strings.Split(data.AnonymousSource, "\n") {
						if strings.Contains(line, marker) {
							if source != "" {
								t.Fatal("ambiguous captured anonymous line")
							}
							source = line
						}
					}
					if source == "" {
						t.Fatal("missing captured anonymous line")
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row.API})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					observationErr = err
					for _, message := range result.Debug {
						if _, value, ok := strings.Cut(message, marker); ok {
							payload = value
						}
					}
				} else {
					found := false
					for _, testCase := range cases {
						if !strings.EqualFold(testCase.MethodName, row.ID) {
							continue
						}
						found = true
						key := testCaseKey(testCase)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
						}
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, execErr := machine.ExecuteInClass(programs[key], testCase.ClassName)
						var observed *vm.RuntimeError
						if !errors.As(execErr, &observed) || observed.Type != "System.AssertException" {
							observationErr = fmt.Errorf("missing observation assertion: %v", execErr)
						} else if _, value, ok := strings.Cut(observed.ExceptionMessage(), marker); ok {
							payload = value
						}
					}
					if !found {
						t.Fatal("missing captured named method")
					}
				}
				var actual map[string]any
				if payload == "" || json.Unmarshal([]byte(payload), &actual) != nil {
					observationErr = fmt.Errorf("invalid observation frame (execution error: %v)", observationErr)
				}
				if reason := data.CarriedIDs[row.ID]; reason != "" {
					t.Logf("CARRIED native=%v local=%v error=%v reason=%s", row.Native, actual, observationErr, reason)
					return
				}
				if observationErr != nil {
					t.Fatal(observationErr)
				}
				if !reflect.DeepEqual(row.Native, actual) {
					t.Fatalf("native=%v local=%v", row.Native, actual)
				}
			})
		}
	}
}
