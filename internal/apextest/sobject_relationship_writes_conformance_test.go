package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay the exact owned sources and metadata captured on Salesforce. Both
// routes observe the exception and the relationship state after the attempted
// write, including matching-target, polymorphic, null and metadata controls.
func TestSObjectRelationshipWritesOrgConformance(t *testing.T) {
	var data struct {
		APIVersions            []string          `json:"apiVersions"`
		Routes                 []string          `json:"routes"`
		ShapeCount             int               `json:"shapeCount"`
		NativeObservationCount int               `json:"nativeObservationCount"`
		Metadata               map[string]string `json:"metadata"`
		MetadataSHA256         map[string]string `json:"metadataSHA256"`
		Fixtures               map[string]struct {
			Files        map[string]string `json:"files"`
			SourceSHA256 map[string]string `json:"sourceSHA256"`
		} `json:"fixtures"`
		Rows  []string `json:"rows"`
		Cells []struct {
			ID               string         `json:"id"`
			API              string         `json:"api"`
			Route            string         `json:"route"`
			Native           map[string]any `json:"native"`
			NativeSHA256     string         `json:"nativeSHA256"`
			RawReceiptSHA256 string         `json:"rawReceiptSHA256"`
		} `json:"cells"`
	}
	raw, err := os.ReadFile("testdata/conformance/sobject_relationship_writes.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" ||
		strings.Join(data.Rows, ",") != "S01,S02,S03,S04,S05,S06,S07,S08,S09,S10,M01,M02,M03,M04,M05,M06,M07,M08" ||
		data.ShapeCount != 18 || data.NativeObservationCount != 108 || len(data.Cells) != 108 || len(data.Fixtures) != 3 ||
		len(data.Metadata) != 9 || len(data.MetadataSHA256) != len(data.Metadata) {
		t.Fatal("incomplete relationship-write native matrix")
	}
	checkHash := func(label, source, want string) {
		t.Helper()
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(source))); got != want {
			t.Fatalf("%s hash: got %s, native %s", label, got, want)
		}
	}
	for path, source := range data.Metadata {
		if source == "" || !strings.HasPrefix(path, "objects/") || filepath.ToSlash(filepath.Clean(path)) != path {
			t.Fatalf("invalid owned metadata path: %q", path)
		}
		checkHash(path, source, data.MetadataSHA256[path])
	}
	wantCells := map[string]map[string]any{}
	for _, api := range data.APIVersions {
		for _, route := range data.Routes {
			for _, row := range data.Rows {
				wantCells[api+"/"+route+"/"+row] = nil
			}
		}
	}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Route + "/" + cell.ID
		if previous, ok := wantCells[key]; !ok || previous != nil || cell.Native["row"] != cell.ID {
			t.Fatalf("unknown, duplicate or misbound native cell: %s", key)
		}
		canonical, err := json.Marshal(cell.Native)
		if err != nil {
			t.Fatal(err)
		}
		checkHash(key, string(canonical), cell.NativeSHA256)
		if digest, err := hex.DecodeString(cell.RawReceiptSHA256); err != nil || len(digest) != sha256.Size {
			t.Fatalf("invalid raw receipt digest: %s", key)
		}
		wantCells[key] = cell.Native
	}
	for key, native := range wantCells {
		if native == nil {
			t.Fatalf("missing native cell: %s", key)
		}
	}
	parseObservation := func(t *testing.T, text string) (string, map[string]any) {
		t.Helper()
		_, frame, found := strings.Cut(text, "SOP|")
		if !found {
			t.Fatalf("missing native observation marker: %s", text)
		}
		row, payload, found := strings.Cut(frame, "|")
		if !found || !strings.Contains(","+strings.Join(data.Rows, ",")+",", ","+row+",") {
			t.Fatalf("unknown observation row: %s", text)
		}
		var observed map[string]any
		decoder := json.NewDecoder(strings.NewReader(payload))
		if err := decoder.Decode(&observed); err != nil {
			t.Fatalf("observation JSON: %v: %s", err, text)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF || observed["row"] != row {
			t.Fatalf("incomplete or misbound observation: %s", text)
		}
		return row, observed
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			fixture, ok := data.Fixtures[api]
			if !ok || len(fixture.Files) != 5 || len(fixture.SourceSHA256) != len(fixture.Files) {
				t.Fatal("missing captured source bundle")
			}
			suffix := strings.TrimSuffix(api, ".0")
			spec := "RelationshipWriteSpec" + suffix
			helper := "RelationshipWriteProbe" + suffix
			for _, path := range []string{"classes/" + helper + ".cls", "classes/" + helper + ".cls-meta.xml", "classes/" + spec + ".cls", "classes/" + spec + ".cls-meta.xml", "anonymous.apex"} {
				source := fixture.Files[path]
				if source == "" {
					t.Fatalf("missing captured source: %s", path)
				}
				checkHash(api+"/"+path, source, fixture.SourceSHA256[path])
				if strings.HasSuffix(path, "-meta.xml") && !strings.Contains(source, "<apiVersion>"+api+"</apiVersion>") {
					t.Fatalf("captured class API mismatch: %s", path)
				}
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for path, source := range data.Metadata {
				writeFile(t, filepath.Join(root, "force-app/main/default", path), source)
			}
			for path, source := range fixture.Files {
				if path != "anonymous.apex" {
					writeFile(t, filepath.Join(root, "force-app/main/default", path), source)
				}
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("captured source parsing: %v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("captured source semantics: %v", analysis.Diagnostics)
			}
			compare := func(t *testing.T, route string, observed map[string]map[string]any) {
				t.Helper()
				if len(observed) != data.ShapeCount {
					t.Fatalf("%s observations: got %d, native %d", route, len(observed), data.ShapeCount)
				}
				for _, row := range data.Rows {
					t.Run(row, func(t *testing.T) {
						want, got := wantCells[api+"/"+route+"/"+row], observed[row]
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("native: %#v; Glade: %#v", want, got)
						}
					})
				}
			}
			t.Run("anonymous", func(t *testing.T) {
				source := fixture.Files["anonymous.apex"]
				if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
					t.Fatalf("captured anonymous semantics: %v", analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runtime := CompileProjectRuntimeForRequest(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatalf("captured anonymous execution: %v", err)
				}
				observed := map[string]map[string]any{}
				for _, line := range result.Debug {
					if !strings.Contains(line, "SOP|") {
						continue
					}
					row, payload := parseObservation(t, line)
					if observed[row] != nil {
						t.Fatalf("duplicate anonymous observation: %s", row)
					}
					observed[row] = payload
				}
				compare(t, "anonymous", observed)
			})
			t.Run("istest", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{spec}, NoDiskCache: true, Parallelism: 1})
				observed := map[string]map[string]any{}
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						if result.ClassName != spec || result.Status != testreport.StatusFail || result.Problem == nil || result.Problem.Type != "System.AssertException" {
							t.Fatalf("missing intentional named observation: %#v; problem %#v", result, result.Problem)
						}
						row, payload := parseObservation(t, result.Problem.Message)
						if result.MethodName != "row"+row || observed[row] != nil {
							t.Fatalf("duplicate or misbound named observation: %#v", result)
						}
						observed[row] = payload
					}
				}
				if run.Summary().Total != data.ShapeCount {
					t.Fatalf("named selection: %#v: %s", run.Summary(), firstRunProblem(run))
				}
				compare(t, "istest", observed)
			})
		})
	}
}
