package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type roleGroupNativeFixture struct {
	RowCount, ScalarObservationCount        int
	OrgIDSHA256, OrgTSVSHA256, OrgRunSHA256 string
	OrgContext                              struct {
		RequestedEdition, SettingsTransportAPI, SettingsSHA256 string
		EnableNetworksEnabled                                  *bool
	}
	AnonymousSources, AnonymousSourceSHA256           map[string]string
	TestClass, TestClassSource, TestClassSourceSHA256 string
	MetadataByAPI, MetadataSHA256ByAPI                map[string]string
	Captures                                          []struct {
		API, TransportAPI, Route, DeploymentID, NativeOutcome string
		CheckOnly                                             bool
		Observations                                          map[string]map[string]any
		ReceiptSHA256                                         map[string]string
	}
	Uncaptured []struct{ Scope, Reason string }
}

func loadRoleGroupNativeFixture(t *testing.T) roleGroupNativeFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/conformance/user_role_generated_groups.json")
	if err != nil {
		t.Fatal(err)
	}
	var data roleGroupNativeFixture
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestUserRoleGeneratedGroupsNativeFixture(t *testing.T) {
	data := loadRoleGroupNativeFixture(t)
	checkHash := func(value, want string) {
		t.Helper()
		digest := sha256.Sum256([]byte(value))
		if hex.EncodeToString(digest[:]) != want {
			t.Fatal("captured source hash mismatch")
		}
	}
	checkDigest := func(value string) {
		t.Helper()
		if decoded, err := hex.DecodeString(value); err != nil || len(decoded) != sha256.Size {
			t.Fatal("missing native artifact binding")
		}
	}
	if data.RowCount != 18 || data.ScalarObservationCount != 228 || len(data.Captures) != 6 || len(data.Uncaptured) != 3 || len(data.AnonymousSources) != 3 {
		t.Fatal("incomplete role-group native matrix")
	}
	if data.OrgContext.RequestedEdition != "Developer" || data.OrgContext.SettingsTransportAPI != "68.0" || data.OrgContext.EnableNetworksEnabled == nil || *data.OrgContext.EnableNetworksEnabled {
		t.Fatal("native org context changed")
	}
	for _, value := range []string{data.OrgIDSHA256, data.OrgTSVSHA256, data.OrgRunSHA256, data.OrgContext.SettingsSHA256} {
		checkDigest(value)
	}
	checkHash(data.TestClassSource, data.TestClassSourceSHA256)
	for _, id := range []string{"RG01", "RG02", "RG03"} {
		if data.AnonymousSources[id] == "" {
			t.Fatal("missing anonymous row", id)
		}
		checkHash(data.AnonymousSources[id], data.AnonymousSourceSHA256[id])
	}
	seen := map[string]bool{}
	rows, scalars := 0, 0
	for _, capture := range data.Captures {
		key := capture.API + "/" + capture.Route
		if seen[key] || capture.TransportAPI != capture.API || len(capture.Observations) != 3 || len(capture.ReceiptSHA256) != 3 {
			t.Fatal("invalid native capture", key)
		}
		seen[key] = true
		if capture.Route == "istest" {
			if capture.CheckOnly || capture.DeploymentID == "" || capture.NativeOutcome != "Failed" {
				t.Fatal("missing named assertion transport provenance")
			}
		} else if capture.Route != "anonymous" || capture.NativeOutcome != "success" {
			t.Fatal("missing anonymous runtime answer")
		}
		for _, id := range []string{"RG01", "RG02", "RG03"} {
			value, ok := capture.Observations[id]
			if !ok {
				t.Fatal("missing native row", key, id)
			}
			checkDigest(capture.ReceiptSHA256[id])
			rows++
			scalars += roleGroupObservationLeaves(value)
		}
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		metadata := data.MetadataByAPI[api]
		checkHash(metadata, data.MetadataSHA256ByAPI[api])
		if !strings.Contains(metadata, "<apiVersion>"+api+"</apiVersion>") {
			t.Fatal("native source API pin changed")
		}
		for _, route := range []string{"anonymous", "istest"} {
			if !seen[api+"/"+route] {
				t.Fatal("missing native context", api, route)
			}
		}
	}
	if rows != data.RowCount || scalars != data.ScalarObservationCount {
		t.Fatalf("native rows=%d scalar cells=%d", rows, scalars)
	}
	for _, carry := range data.Uncaptured {
		if carry.Scope == "" || carry.Reason == "" {
			t.Fatal("unobserved scope requires a reason")
		}
		t.Logf("UNCAPTURED %s: %s", carry.Scope, carry.Reason)
	}
}

func TestUserRoleGeneratedGroupsNativeConformance(t *testing.T) {
	data := loadRoleGroupNativeFixture(t)
	rows := 0
	for _, capture := range data.Captures {
		t.Run(capture.API+"/"+capture.Route, func(t *testing.T) {
			index := nativeBoundaryIndex(t, capture.API, map[string]string{data.TestClass: data.TestClassSource}, nil)
			check := func(t *testing.T, id, payload string) {
				t.Helper()
				want, ok := capture.Observations[id]
				var actual map[string]any
				if !ok || json.Unmarshal([]byte(payload), &actual) != nil {
					t.Fatalf("invalid native observation frame: %s", payload)
				}
				if !reflect.DeepEqual(roleGroupUnorderedObservation(want), roleGroupUnorderedObservation(actual)) {
					t.Errorf("native=%v; local=%v", want, actual)
				}
			}
			if capture.Route == "anonymous" {
				org := orgFromIndex(index)
				org.APIVersion = capture.API
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, id := range []string{"RG01", "RG02", "RG03"} {
					rows++
					t.Run(id, func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions(data.AnonymousSources[id], vm.CompileOptions{APIVersion: capture.API})
						if err != nil {
							t.Fatal(err)
						}
						result, err := runner.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						found := 0
						for _, debug := range result.Debug {
							if _, payload, ok := strings.Cut(debug, id+"|"); ok {
								found++
								check(t, id, payload)
							}
						}
						if found != 1 {
							t.Fatalf("observation frames=%d", found)
						}
					})
				}
				return
			}
			if capture.Route != "istest" {
				t.Fatal("uncaptured route")
			}
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native named source accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{data.TestClass}})
			if len(cases) != 3 {
				t.Fatal("native named selection changed")
			}
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			seen := map[string]bool{}
			for _, testCase := range cases {
				rows++
				key := testCaseKey(testCase)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
				}
				// Match the native-boundary harness: pin the org API directly,
				// including API62, independently of supported REST server routes.
				org := orgFromIndex(index)
				org.APIVersion = capture.TransportAPI
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr := machine.ExecuteInClass(programs[key], testCase.ClassName)
				var runtimeErr *vm.RuntimeError
				if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
					t.Fatalf("missing intentional observation failure: %v", execErr)
				}
				_, frame, ok := strings.Cut(runtimeErr.ExceptionMessage(), "ROLE_GROUP_RESULT|")
				id, payload, framed := strings.Cut(frame, "|")
				if !ok || !framed || seen[id] {
					t.Fatal("invalid named observation frame")
				}
				seen[id] = true
				t.Run(id, func(t *testing.T) { check(t, id, payload) })
			}
		})
	}
	if rows != 18 {
		t.Fatalf("replayed native rows=%d", rows)
	}
}

func roleGroupObservationLeaves(value any) int {
	switch value := value.(type) {
	case map[string]any:
		n := 0
		for _, item := range value {
			n += roleGroupObservationLeaves(item)
		}
		return n
	case []any:
		n := 0
		for _, item := range value {
			n += roleGroupObservationLeaves(item)
		}
		return n
	default:
		return 1
	}
}

// Captured arrays are SOQL result collections without ORDER BY. Preserve their
// multiplicity while comparing group association and identity without row order.
func roleGroupUnorderedObservation(value any) any {
	switch value := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, item := range value {
			out[key] = roleGroupUnorderedObservation(item)
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = roleGroupUnorderedObservation(item)
		}
		sort.Slice(out, func(i, j int) bool {
			left, _ := json.Marshal(out[i])
			right, _ := json.Marshal(out[j])
			return string(left) < string(right)
		})
		return out
	default:
		return value
	}
}
