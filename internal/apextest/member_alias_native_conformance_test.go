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

type memberAliasNativeSource struct {
	Classes         map[string]string `json:"classes"`
	ClassMetadata   map[string]string `json:"classMetadata"`
	Triggers        map[string]string `json:"triggers,omitempty"`
	TriggerMetadata map[string]string `json:"triggerMetadata,omitempty"`
	Anonymous       string            `json:"anonymous"`
	TestClass       string            `json:"testClass"`
	TestMethod      string            `json:"testMethod"`
	SourceSHA256    map[string]string `json:"sourceSha256"`
}

type memberAliasNativeCell struct {
	APIVersion     string          `json:"apiVersion"`
	Route          string          `json:"route"`
	Observation    json.RawMessage `json:"observation"`
	EvidenceSHA256 string          `json:"evidenceSha256"`
	Command        string          `json:"command"`
}

// The owned source is replayed unchanged, including the debug/assertion transport.
// Expected values come from each captured API/context cell, not from the runtime.
func TestMemberAliasNativeConformance(t *testing.T) {
	runNativeSnapshotConformance(t, "member_alias_native.json", "MemberAliasNative")
}

func runNativeSnapshotConformance(t *testing.T, filename, family string) {
	t.Helper()
	var data struct {
		APIVersions            []string                           `json:"apiVersions"`
		Routes                 []string                           `json:"routes"`
		NativeObservationCount int                                `json:"nativeObservationCount"`
		OrgIDSHA256            string                             `json:"orgIdSha256"`
		SourcesByAPI           map[string]memberAliasNativeSource `json:"sourcesByAPI"`
		Cells                  []memberAliasNativeCell            `json:"cells"`
		D7Evidence             struct {
			OrgTSV          string `json:"observationsTsv"`
			OrgTSVSHA256    string `json:"orgTsvSha256"`
			OrgRun          string `json:"runMetadata"`
			OrgRunSHA256    string `json:"orgRunSha256"`
			RawOrgRunSHA256 string `json:"rawOrgRunSha256"`
			Normalization   string `json:"normalization"`
		} `json:"d7Evidence"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", filename))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" || data.NativeObservationCount != 6 || len(data.Cells) != 6 || len(data.SourcesByAPI) != 3 {
		t.Fatal("incomplete native matrix")
	}
	memberAliasNativeDigest(t, "org", data.OrgIDSHA256)
	cells := map[string]memberAliasNativeCell{}
	for _, cell := range data.Cells {
		key := cell.APIVersion + "/" + cell.Route
		if _, exists := cells[key]; exists || cell.Command == "" || !json.Valid(cell.Observation) {
			t.Fatalf("invalid/repeated native cell %s", key)
		}
		memberAliasNativeDigest(t, key, cell.EvidenceSHA256)
		cells[key] = cell
	}
	for _, document := range []struct{ name, text, digest string }{
		{"org.tsv", data.D7Evidence.OrgTSV, data.D7Evidence.OrgTSVSHA256},
		{"ORG-RUN.txt", data.D7Evidence.OrgRun, data.D7Evidence.OrgRunSHA256},
	} {
		sum := sha256.Sum256([]byte(document.text))
		if document.text == "" || hex.EncodeToString(sum[:]) != document.digest {
			t.Fatalf("captured D7 evidence changed: %s", document.name)
		}
	}
	memberAliasNativeDigest(t, "raw ORG-RUN.txt", data.D7Evidence.RawOrgRunSHA256)
	if data.D7Evidence.RawOrgRunSHA256 == data.D7Evidence.OrgRunSHA256 || data.D7Evidence.Normalization == "" || !strings.Contains(data.D7Evidence.OrgRun, "HOME=<scratch-auth-home>") {
		t.Fatal("missing explicit D7 authentication-path normalization")
	}
	lines := strings.Split(strings.TrimSuffix(data.D7Evidence.OrgTSV, "\n"), "\n")
	if len(lines) != 7 || lines[0] != "row\tapi\tcontext\tvalue" {
		t.Fatal("incomplete D7 observation table")
	}
	d7Seen := map[string]bool{}
	for _, line := range lines[1:] {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			t.Fatal("invalid D7 observation row")
		}
		route := fields[2]
		if route == "istest" {
			route = "isTest"
		}
		key := fields[1] + ".0/" + route
		cell, exists := cells[key]
		var exported, native any
		if !exists || d7Seen[key] || json.Unmarshal([]byte(fields[3]), &exported) != nil || json.Unmarshal(cell.Observation, &native) != nil || !reflect.DeepEqual(exported, native) {
			t.Fatalf("D7 row does not match exported cell: %s", key)
		}
		d7Seen[key] = true
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			source, ok := data.SourcesByAPI[api]
			if !ok || len(source.Classes) == 0 || len(source.ClassMetadata) != len(source.Classes) || source.TestClass == "" || source.TestMethod == "" {
				t.Fatal("incomplete captured source")
			}
			files := map[string]string{"anonymous": source.Anonymous}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for name, text := range source.Classes {
				meta, ok := source.ClassMetadata[name]
				if !ok || !strings.Contains(meta, "<apiVersion>"+api+"</apiVersion>") {
					t.Fatalf("missing captured API metadata: %s", name)
				}
				files["class:"+name] = text
				files["classMetadata:"+name] = meta
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, text)
				writeFile(t, path+"-meta.xml", meta)
			}
			if len(source.Triggers) != len(source.TriggerMetadata) {
				t.Fatal("incomplete captured trigger metadata")
			}
			for name, text := range source.Triggers {
				meta, ok := source.TriggerMetadata[name]
				if !ok || !strings.Contains(meta, "<apiVersion>"+api+"</apiVersion>") {
					t.Fatalf("missing captured trigger API metadata: %s", name)
				}
				files["trigger:"+name] = text
				files["triggerMetadata:"+name] = meta
				path := filepath.Join(root, "force-app/main/default/triggers", name+".trigger")
				writeFile(t, path, text)
				writeFile(t, path+"-meta.xml", meta)
			}
			if len(files) != len(source.SourceSHA256) {
				t.Fatal("incomplete captured source digest matrix")
			}
			for name, text := range files {
				sum := sha256.Sum256([]byte(text))
				if hex.EncodeToString(sum[:]) != source.SourceSHA256[name] {
					t.Fatalf("captured source changed: %s", name)
				}
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepts sources; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if len(source.Triggers) > 0 {
				triggers, failures := compileProjectTriggers(index)
				if len(failures) > 0 {
					t.Fatal(failures)
				}
				for _, trigger := range triggers {
					if err := runner.base.RegisterTrigger(trigger); err != nil {
						t.Fatal(err)
					}
				}
			}
			counts := newConformanceCountLog(t, family, api)
			for _, route := range data.Routes {
				t.Run(route, func(t *testing.T) {
					counts.TrackKind(t, route, "exact", 1)
					cell, ok := cells[api+"/"+route]
					if !ok {
						t.Fatal("missing native answer")
					}
					var observed string
					if route == "anonymous" {
						analysis := sema.AnalyzeAnonymous(index, source.Anonymous, api)
						program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
						if analysis.HasErrors() || err != nil {
							t.Fatalf("native executes anonymous; diagnostics=%v compile=%v", analysis.Diagnostics, err)
						}
						result, err := runner.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						for _, line := range result.Debug {
							if at := strings.Index(line, "OBS|"); at >= 0 {
								if observed != "" {
									t.Fatal("duplicate anonymous observation")
								}
								observed = line[at+len("OBS|"):]
							}
						}
					} else {
						cases := Discover(index, Options{SelectedClasses: []string{source.TestClass}, SelectedMethod: source.TestMethod})
						if len(cases) != 1 {
							t.Fatalf("captured test discovery: %d", len(cases))
						}
						methods, methodErrors := compileTestMethods(cases)
						programs, programErrors := compileTestInvokePrograms(cases)
						key := testCaseKey(cases[0])
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("captured test compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
							t.Fatalf("missing captured assertion transport: %v", err)
						}
						message := runtimeErr.ExceptionMessage()
						if at := strings.Index(message, "OBS|"); at >= 0 {
							observed = message[at+len("OBS|"):]
						}
					}
					var got, want any
					if err := json.Unmarshal([]byte(observed), &got); err != nil {
						t.Fatalf("missing/invalid captured payload: %q: %v", observed, err)
					}
					if err := json.Unmarshal(cell.Observation, &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%s local=%s", cell.Observation, observed)
					}
				})
			}
		})
	}
}

func memberAliasNativeDigest(t *testing.T, name, digest string) {
	t.Helper()
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256.Size {
		t.Fatalf("invalid native digest: %s", name)
	}
}
