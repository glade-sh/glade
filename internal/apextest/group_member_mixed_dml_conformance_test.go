package apextest

import (
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

func TestGroupMemberMixedDMLNativeConformance(t *testing.T) {
	var data struct {
		RowCount, MatchedCount, CarriedCount, ScalarCells int
		OrgContext                                        struct {
			RequestedEdition, OtherFeatureContext string
			FeaturesRequested                     []string
			SettingsChanged                       *bool
		}
		OrgIDSHA256, OrgTSVSHA256, OrgRunSHA256           string
		AnonymousSources, AnonymousSourceSHA256           map[string]string
		TestClass, TestClassSource, TestClassSourceSHA256 string
		MetadataByAPI, MetadataSHA256ByAPI                map[string]string
		Deployments                                       map[string]struct {
			ID, Status, TestOutcome, ReceiptSHA256 string
			CheckOnly                              bool
		}
		Observations []struct {
			ID, API, TransportAPI, Route, ReceiptSHA256, CarryReason string
			Native                                                   map[string]any
		}
		Uncaptured []string
	}
	raw, err := os.ReadFile("testdata/conformance/group_member_mixed_dml.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	checkDigest := func(value string) {
		t.Helper()
		if bytes, err := hex.DecodeString(value); err != nil || len(bytes) != sha256.Size {
			t.Fatal("missing native artifact binding")
		}
	}
	checkHash := func(source, want string) {
		t.Helper()
		digest := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(digest[:]) != want {
			t.Fatal("captured source hash changed")
		}
	}
	if data.RowCount != 27 || data.MatchedCount != 21 || data.CarriedCount != 6 || data.ScalarCells != 81 || len(data.Observations) != 27 || len(data.AnonymousSources) != 4 || len(data.Uncaptured) != 2 {
		t.Fatal("incomplete native matrix")
	}
	if data.OrgContext.RequestedEdition != "Developer" || data.OrgContext.OtherFeatureContext != "UNCAPTURED" || data.OrgContext.FeaturesRequested == nil || len(data.OrgContext.FeaturesRequested) != 0 || data.OrgContext.SettingsChanged == nil || *data.OrgContext.SettingsChanged {
		t.Fatal("native org context changed")
	}
	for _, digest := range []string{data.OrgIDSHA256, data.OrgTSVSHA256, data.OrgRunSHA256} {
		checkDigest(digest)
	}
	checkHash(data.TestClassSource, data.TestClassSourceSHA256)
	for _, id := range []string{"GM01", "GM02", "GM03", "GM04"} {
		checkHash(data.AnonymousSources[id], data.AnonymousSourceSHA256[id])
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		checkHash(data.MetadataByAPI[api], data.MetadataSHA256ByAPI[api])
		if !strings.Contains(data.MetadataByAPI[api], "<apiVersion>"+api+"</apiVersion>") {
			t.Fatal("native API pin changed")
		}
		deployment, ok := data.Deployments[api]
		if !ok || deployment.ID == "" || deployment.Status != "Succeeded" || deployment.CheckOnly || deployment.TestOutcome != "Failed" {
			t.Fatal("missing successful named deployment/assertion transport", api)
		}
		checkDigest(deployment.ReceiptSHA256)
	}
	seen := map[string]bool{}
	matched, carried, scalars := 0, 0, 0
	for _, row := range data.Observations {
		key := row.API + "/" + row.Route + "/" + row.ID
		if seen[key] || row.API != row.TransportAPI || len(row.Native) != 3 || data.MetadataByAPI[row.API] == "" {
			t.Fatal("invalid capture", key)
		}
		seen[key] = true
		checkDigest(row.ReceiptSHA256)
		scalars += len(row.Native)
		if row.CarryReason != "" {
			if row.Route != "anonymous" || (row.ID != "GM03" && row.ID != "GM04") || row.Native["status"] != "MIXED_DML_OPERATION" || row.Native["outcome"] != "dml" {
				t.Fatal("carry scope changed", key)
			}
			carried++
			t.Logf("CARRY %s native=%v: %s", key, row.Native, row.CarryReason)
			continue
		}
		matched++
		t.Run(key, func(t *testing.T) {
			index := nativeBoundaryIndex(t, row.API, map[string]string{data.TestClass: data.TestClassSource}, nil)
			org := orgFromIndex(index)
			org.APIVersion = row.TransportAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var payload string
			marker := "MEMBER_DML|" + row.ID + "|"
			switch row.Route {
			case "anonymous":
				program, err := vm.CompileAnonymousWithOptions(data.AnonymousSources[row.ID], vm.CompileOptions{APIVersion: row.API})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				found := 0
				for _, debug := range result.Debug {
					if _, value, ok := strings.Cut(debug, marker); ok {
						payload = value
						found++
					}
				}
				if found != 1 {
					t.Fatalf("observation frames=%d", found)
				}
			case "istest":
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
				}
				cases := Discover(index, Options{SelectedClasses: []string{data.TestClass}, SelectedMethod: strings.ToLower(row.ID)})
				if len(cases) != 1 {
					t.Fatal("named selection changed")
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				var observed *vm.RuntimeError
				if !errors.As(execErr, &observed) || observed.Type != "System.AssertException" {
					t.Fatalf("missing intentional observation assertion: %v", execErr)
				}
				var ok bool
				_, payload, ok = strings.Cut(observed.ExceptionMessage(), marker)
				if !ok {
					t.Fatal("wrong named observation frame")
				}
			default:
				t.Fatal("uncaptured route")
			}
			var actual map[string]any
			if err := json.Unmarshal([]byte(payload), &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(row.Native, actual) {
				t.Fatalf("native=%v; local=%v", row.Native, actual)
			}
		})
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, id := range []string{"GM01", "GM02", "GM03", "GM04", "GM05"} {
			if !seen[api+"/istest/"+id] || (id != "GM05" && !seen[api+"/anonymous/"+id]) {
				t.Fatal("missing native context", api, id)
			}
		}
	}
	if matched != data.MatchedCount || carried != data.CarriedCount || scalars != data.ScalarCells {
		t.Fatalf("matched=%d carried=%d scalar cells=%d", matched, carried, scalars)
	}
	for _, scope := range data.Uncaptured {
		t.Log("UNCAPTURED", scope)
	}
}
