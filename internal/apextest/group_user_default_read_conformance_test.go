package apextest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type groupDefaultReadFixture struct {
	RowCount, ScalarCells, EnforcedRows, EnforcedScalarCells, ContextRows, ContextScalarCells int
	OrgIDSHA256, OrgTSVSHA256, OrgRunSHA256, TestClass, ContextReason                         string
	Sources, SourceSHA256                                                                     map[string]string
	OrgContext                                                                                struct {
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

func loadGroupDefaultReadFixture(t *testing.T) groupDefaultReadFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/conformance/group_user_default_read.json")
	if err != nil {
		t.Fatal(err)
	}
	var data groupDefaultReadFixture
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestGroupUserDefaultReadNativeFixture(t *testing.T) {
	data := loadGroupDefaultReadFixture(t)
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
	if data.RowCount != 12 || data.ScalarCells != 87 || data.EnforcedRows != 9 || data.EnforcedScalarCells != 63 || data.ContextRows != 3 || data.ContextScalarCells != 24 || len(data.Rows) != 12 || len(data.Captures) != 3 || len(data.Sources) != 4 || data.ContextReason == "" {
		t.Fatal("incomplete native read matrix")
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
	contextCells, enforcedCells := 0, 0
	for _, row := range data.Rows {
		key := row.API + "/" + row.Route + "/" + row.ID
		if seenRow[key] || !seenAPI[row.API] || row.API != row.TransportAPI || row.Route != "istest" || row.ID < "GU01" || row.ID > "GU04" {
			t.Fatal("invalid native row", key)
		}
		seenRow[key] = true
		digest(row.ReceiptSHA256)
		if row.ID != "GU04" {
			if len(row.Native) != 7 {
				t.Fatal("query observation fields changed")
			}
			enforcedCells += roleGroupObservationLeaves(row.Native)
			continue
		}
		want := map[string]any{
			"viewAllUsers": false, "viewAllData": false, "groupOwnerField": true,
			"memberOwnerField": false, "userOwnerField": false,
			"sharingModels": map[string]any{"User": "Read", "Group": "ReadWrite", "GroupMember": "ReadWrite"},
		}
		if !reflect.DeepEqual(row.Native, want) {
			t.Fatal("captured configuration evidence changed", key)
		}
		contextCells += roleGroupObservationLeaves(row.Native)
		t.Logf("NATIVE CONTEXT %s = %v: %s", key, row.Native, data.ContextReason)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		if !seenAPI[api] {
			t.Fatal("missing native API", api)
		}
	}
	if enforcedCells != data.EnforcedScalarCells || contextCells != data.ContextScalarCells || enforcedCells+contextCells != data.ScalarCells {
		t.Fatal("native scalar counts changed")
	}
	for _, scope := range data.Uncaptured {
		t.Log("UNCAPTURED", scope)
	}
}

func TestGroupUserDefaultReadNativeConformance(t *testing.T) {
	data := loadGroupDefaultReadFixture(t)
	for _, capture := range data.Captures {
		t.Run(capture.API, func(t *testing.T) {
			index := nativeBoundaryIndex(t, capture.API, data.Sources, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = capture.TransportAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			setups, setupErrors, setupPrograms, setupProgramErrors := compileTestSetupMethods(index)
			seed, _, err, _ := prepareTestSetupOrg(context.Background(), data.TestClass, runner.base, nil, setups[data.TestClass], setupErrors[data.TestClass], setupPrograms[data.TestClass], setupProgramErrors[data.TestClass], nil, runner.newOrg(), Options{Parallelism: 1}, nil)
			if err != nil {
				t.Fatal("native setup completed; local setup:", err)
			}
			cases := Discover(index, Options{SelectedClasses: []string{data.TestClass}})
			if len(cases) != 4 {
				t.Fatal("named observation selection changed")
			}
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			for _, row := range data.Rows {
				if row.API != capture.API || row.ID == "GU04" {
					continue // GU04 is org configuration evidence, validated above.
				}
				t.Run(row.ID, func(t *testing.T) {
					for _, testCase := range cases {
						if !strings.EqualFold(testCase.MethodName, row.ID) {
							continue
						}
						key := testCaseKey(testCase)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						state := seed.Clone()
						machine.SetOrg(&state)
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, execErr := machine.ExecuteInClass(programs[key], testCase.ClassName)
						var observed *vm.RuntimeError
						if !errors.As(execErr, &observed) || observed.Type != "System.AssertException" {
							t.Fatalf("missing intentional observation assertion: %v", execErr)
						}
						_, payload, ok := strings.Cut(observed.ExceptionMessage(), "GROUP_LOOKUP|"+row.ID+"|")
						var actual map[string]any
						if !ok || json.Unmarshal([]byte(payload), &actual) != nil {
							t.Fatal("invalid named observation frame")
						}
						if !reflect.DeepEqual(row.Native, actual) {
							t.Fatalf("native=%v local=%v", row.Native, actual)
						}
						return
					}
					t.Fatal("missing native method", row.ID)
				})
			}
		})
	}
}
