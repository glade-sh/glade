package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type describeJSONVersionFixture struct {
	ProjectAPI, ProjectSource, ProjectSourceSHA256                 string
	RowCount, ScalarCells                                          int
	OrgIDSHA256, OrgTSVSHA256, OrgRunSHA256, TestClass, Projection string
	Sources, SourceSHA256                                          map[string]string
	AnonymousSource, AnonymousSourceSHA256                         string
	OrgContext                                                     struct {
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

func loadDescribeJSONVersionFixture(t *testing.T) describeJSONVersionFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/conformance/describe_json_version_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var data describeJSONVersionFixture
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDescribeJSONVersionNativeFixture(t *testing.T) {
	data := loadDescribeJSONVersionFixture(t)
	digest := func(value string) {
		t.Helper()
		if bytes, err := hex.DecodeString(value); err != nil || len(bytes) != sha256.Size {
			t.Fatal("missing native binding")
		}
	}
	checkHash := func(source, want string) {
		t.Helper()
		hash := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(hash[:]) != want {
			t.Fatal("native source binding changed")
		}
	}
	if data.RowCount != 36 || data.ScalarCells != 312 || len(data.Rows) != 36 || len(data.Captures) != 3 || len(data.Sources) != 2 || data.Projection == "" {
		t.Fatal("incomplete describe URL matrix")
	}
	if data.OrgContext.RequestedEdition != "Developer" || data.OrgContext.OtherFeatureContext != "UNCAPTURED" || data.OrgContext.FeaturesRequested == nil || len(data.OrgContext.FeaturesRequested) != 0 || data.OrgContext.SettingsChanged == nil || *data.OrgContext.SettingsChanged {
		t.Fatal("native org context changed")
	}
	for _, value := range []string{data.OrgIDSHA256, data.OrgTSVSHA256, data.OrgRunSHA256} {
		digest(value)
	}
	for name, source := range data.Sources {
		checkHash(source, data.SourceSHA256[name])
	}
	checkHash(data.AnonymousSource, data.AnonymousSourceSHA256)
	checkHash(data.ProjectSource, data.ProjectSourceSHA256)
	var project struct {
		API string `json:"sourceApiVersion"`
	}
	if json.Unmarshal([]byte(data.ProjectSource), &project) != nil || data.ProjectAPI != "62.0" || project.API != data.ProjectAPI {
		t.Fatal("native project API pin changed")
	}
	seenAPI, seenRow := map[string]bool{}, map[string]bool{}
	for _, capture := range data.Captures {
		if seenAPI[capture.API] || capture.API != capture.TransportAPI || capture.CheckOnly || capture.DeploymentID == "" || capture.DeploymentStatus != "Succeeded" {
			t.Fatal("invalid native capture")
		}
		seenAPI[capture.API] = true
		checkHash(capture.Metadata, capture.MetadataSHA256)
		digest(capture.DeploymentReceiptSHA256)
		if !strings.Contains(capture.Metadata, "<apiVersion>"+capture.API+"</apiVersion>") {
			t.Fatal("native component API changed")
		}
	}
	cells := 0
	for _, row := range data.Rows {
		key := row.API + "/" + row.Route + "/" + row.ID
		if seenRow[key] || !seenAPI[row.API] || row.API != row.TransportAPI || (row.Route != "anonymous" && row.Route != "istest") || row.ID < "DJ01" || row.ID > "DJ06" {
			t.Fatal("invalid native row", key)
		}
		seenRow[key] = true
		digest(row.ReceiptSHA256)
		cells += describeJSONObservationLeaves(row.Native)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		if !seenAPI[api] {
			t.Fatal("missing native API", api)
		}
	}
	if cells != data.ScalarCells {
		t.Fatal("native cell count changed", cells)
	}
	for _, scope := range data.Uncaptured {
		t.Log("UNCAPTURED", scope)
	}
}

func describeJSONObservationLeaves(value any) int {
	switch value := value.(type) {
	case map[string]any:
		n := 0
		for _, item := range value {
			n += describeJSONObservationLeaves(item)
		}
		return n
	case []any:
		n := 0
		for _, item := range value {
			n += describeJSONObservationLeaves(item)
		}
		return n
	default:
		return 1
	}
}

func TestDescribeJSONVersionNativeConformance(t *testing.T) {
	data := loadDescribeJSONVersionFixture(t)
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
		if len(cases) != 6 {
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
				payload := ""
				marker := "DESCRIBE_VERSION|" + row.ID + "|"
				if row.Route == "anonymous" {
					source := ""
					for _, line := range strings.Split(data.AnonymousSource, "\n") {
						if strings.Contains(line, marker) {
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
					if err != nil {
						t.Fatal(err)
					}
					for _, message := range result.Debug {
						if _, value, ok := strings.Cut(message, marker); ok {
							payload = value
						}
					}
				} else {
					for _, testCase := range cases {
						if !strings.EqualFold(testCase.MethodName, row.ID) {
							continue
						}
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
							t.Fatalf("missing observation assertion: %v", execErr)
						}
						_, payload, _ = strings.Cut(observed.ExceptionMessage(), marker)
					}
				}
				var actual map[string]any
				if payload == "" || json.Unmarshal([]byte(payload), &actual) != nil {
					t.Fatal("invalid native observation frame")
				}
				if !reflect.DeepEqual(row.Native, actual) {
					t.Fatalf("native=%v local=%v", row.Native, actual)
				}
			})
		}
	}
}
