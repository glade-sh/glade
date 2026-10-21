package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Replay the exact named sources with their distinct class API versions. The
// exception rows observe an uncaught FinalException, not a partial map result.
// Native stacks are retained as evidence without asserting instruction timing.
func TestMapListAliasNativeConformance(t *testing.T) {
	var data struct {
		Routes                             []string
		ShapeCount, NativeObservationCount int
		Fixtures                           map[string]struct {
			Project, ProjectSHA256 string
			Classes                map[string]struct {
				Source, SourceSHA256, Metadata, MetadataSHA256, SourceAPI string
			}
			Capture struct {
				DeploymentID, TransportAPI, Route string
				CheckOnly, Done                   bool
				ObservedRows                      int
			}
		}
		Cells []struct {
			Fixture, Route, Method, RawRow, RawRowSHA256 string
			RawFailure, RawFailureSHA256                 string
			NativeType, NativeMessage, NativeStackTrace  string
			Native                                       map[string]any
		}
		Uncaptured []struct{ Scope, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/map_list_alias_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	methods := []string{"helperBreak", "helperEnd", "nestedEnd", "valuesSnapshot"}
	variants := []string{"helper48-test24", "helper67-test67"}
	if strings.Join(data.Routes, ",") != "istest" || data.ShapeCount != 4 || data.NativeObservationCount != 8 || len(data.Fixtures) != 2 || len(data.Cells) != 8 || len(data.Uncaptured) != 2 {
		t.Fatal("incomplete native map/list alias matrix")
	}
	for _, boundary := range data.Uncaptured {
		if boundary.Scope == "" || boundary.Reason == "" {
			t.Fatal("uncaptured scope requires a reason")
		}
		t.Logf("UNCAPTURED %s: %s", boundary.Scope, boundary.Reason)
	}
	checkHash := func(value, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(value))
		if value == "" || hex.EncodeToString(sum[:]) != want {
			t.Fatal("changed owned source or raw native observation")
		}
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.Fixture + "/" + cell.Method
		if seen[key] || cell.Route != "istest" || cell.NativeStackTrace == "" {
			t.Fatalf("duplicate or incomplete native cell: %s", key)
		}
		seen[key] = true
		checkHash(cell.RawRow, cell.RawRowSHA256)
		checkHash(cell.RawFailure, cell.RawFailureSHA256)
		method, payload, found := strings.Cut(strings.TrimSuffix(cell.RawRow, "\n"), "\t")
		var fromRaw map[string]any
		if !found || method != cell.Method || json.Unmarshal([]byte(payload), &fromRaw) != nil || !reflect.DeepEqual(fromRaw, cell.Native) {
			t.Fatalf("native cell differs from raw row: %s", key)
		}
		var failure struct{ Message, StackTrace string }
		if json.Unmarshal([]byte(cell.RawFailure), &failure) != nil || failure.Message != cell.NativeType+": "+cell.NativeMessage || failure.StackTrace != cell.NativeStackTrace {
			t.Fatalf("native cell differs from raw failure: %s", key)
		}
		if observation, observed := cell.Native["observation"]; observed {
			_, payload, found := strings.Cut(cell.NativeMessage, "Actual: ")
			var actual map[string]any
			if cell.NativeType != "System.AssertException" || !found || json.Unmarshal([]byte(payload), &actual) != nil || !reflect.DeepEqual(actual, observation) {
				t.Fatalf("invalid native assertion observation: %s", key)
			}
		} else if cell.NativeType != "System.FinalException" || cell.Native["message"] != failure.Message || cell.Native["stackTrace"] != failure.StackTrace {
			t.Fatalf("invalid native uncaught exception: %s", key)
		}
	}
	for _, variant := range variants {
		for _, method := range methods {
			if !seen[variant+"/"+method] {
				t.Fatalf("missing native cell: %s/%s", variant, method)
			}
		}
		t.Run(variant, func(t *testing.T) {
			fixture := data.Fixtures[variant]
			if fixture.Capture.DeploymentID == "" || fixture.Capture.TransportAPI != "67.0" || fixture.Capture.Route != "istest" || !fixture.Capture.CheckOnly || !fixture.Capture.Done || fixture.Capture.ObservedRows != 4 || len(fixture.Classes) != 2 {
				t.Fatal("incomplete native capture provenance")
			}
			checkHash(fixture.Project, fixture.ProjectSHA256)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fixture.Project)
			for name, class := range fixture.Classes {
				checkHash(class.Source, class.SourceSHA256)
				checkHash(class.Metadata, class.MetadataSHA256)
				var declaration struct {
					APIVersion string `xml:"apiVersion"`
				}
				wantAPI := "67.0"
				if variant == "helper48-test24" {
					wantAPI = "48.0"
					if name == "AliasObservation" {
						wantAPI = "24.0"
					}
				}
				if (name != "AliasStore" && name != "AliasObservation") || class.SourceAPI != wantAPI || xml.Unmarshal([]byte(class.Metadata), &declaration) != nil || declaration.APIVersion != wantAPI {
					t.Fatalf("unexpected class API: %s", name)
				}
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, class.Source)
				writeFile(t, path+"-meta.xml", class.Metadata)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"AliasObservation"}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 4 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 4 {
				t.Fatalf("observation selection: %#v", run.Summary())
			}
			for _, cell := range data.Cells {
				if cell.Fixture != variant {
					continue
				}
				t.Run(cell.Method, func(t *testing.T) {
					matched := 0
					for _, result := range run.Suites[0].Cases {
						if result.MethodName != cell.Method {
							continue
						}
						matched++
						if result.Problem == nil || result.Problem.Type != cell.NativeType {
							t.Fatalf("native type=%s; local=%#v", cell.NativeType, result.Problem)
						}
						// The native test API includes the exception type in its message;
						// the local report exposes that same type separately.
						message := strings.TrimPrefix(result.Problem.Message, cell.NativeType+": ")
						if observation, observed := cell.Native["observation"]; observed {
							prefix, payload, found := strings.Cut(message, "Actual: ")
							wantPrefix, _, _ := strings.Cut(cell.NativeMessage, "Actual: ")
							var actual map[string]any
							if !found || prefix != wantPrefix || json.Unmarshal([]byte(payload), &actual) != nil || !reflect.DeepEqual(actual, observation) {
								t.Fatalf("native=%s; local=%s", cell.NativeMessage, message)
							}
						} else if message != cell.NativeMessage {
							t.Fatalf("native=%q; local=%q", cell.NativeMessage, message)
						}
					}
					if matched != 1 {
						t.Fatalf("expected one named observation, got %d", matched)
					}
				})
			}
		})
	}
}
