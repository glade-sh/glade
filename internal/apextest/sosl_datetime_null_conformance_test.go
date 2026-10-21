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
)

// Replay the exact owned fixed-search sources. Anonymous execution cannot use
// fixed search results, so its native rejection is retained as context evidence.
// Bound-null RHS and SOQL observations are explicit carries, not SOSL oracles.
func TestSOSLDatetimeNullOrgConformance(t *testing.T) {
	type observation struct {
		ID               string         `json:"id"`
		API              string         `json:"api"`
		Route            string         `json:"route"`
		Native           map[string]any `json:"native"`
		NativeSHA256     string         `json:"nativeSHA256"`
		RawReceiptSHA256 string         `json:"rawReceiptSHA256"`
	}
	var data struct {
		APIVersions              []string          `json:"apiVersions"`
		Routes                   []string          `json:"routes"`
		ShapeCount               int               `json:"shapeCount"`
		NativeObservationCount   int               `json:"nativeObservationCount"`
		EnforcedObservationCount int               `json:"enforcedObservationCount"`
		Metadata                 map[string]string `json:"metadata"`
		MetadataSHA256           map[string]string `json:"metadataSHA256"`
		Fixtures                 map[string]struct {
			Files        map[string]string `json:"files"`
			SourceSHA256 map[string]string `json:"sourceSHA256"`
		} `json:"fixtures"`
		Rows         []string `json:"rows"`
		EnforcedRows []string `json:"enforcedRows"`
		Carries      []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"carries"`
		Cells             []observation `json:"cells"`
		AnonymousContexts []struct {
			API                    string          `json:"api"`
			Native                 map[string]any  `json:"native"`
			NativeSHA256           string          `json:"nativeSHA256"`
			RawReceiptSHA256       string          `json:"rawReceiptSHA256"`
			ExpectedSemanticNative json.RawMessage `json:"expectedSemanticNative"`
			Reason                 string          `json:"reason"`
		} `json:"anonymousContexts"`
	}
	raw, err := os.ReadFile("testdata/conformance/sosl_datetime_null.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	rows := make([]string, 0, 19)
	for i := 1; i <= 18; i++ {
		rows = append(rows, fmt.Sprintf("D%02d", i))
	}
	rows = append(rows, "Q01")
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "istest" || !reflect.DeepEqual(data.Rows, rows) || !reflect.DeepEqual(data.EnforcedRows, rows[:17]) || data.ShapeCount != 19 || data.NativeObservationCount != 57 || data.EnforcedObservationCount != 51 || len(data.Cells) != 57 || len(data.Fixtures) != 3 || len(data.Metadata) != 2 || len(data.MetadataSHA256) != 2 || len(data.AnonymousContexts) != 3 || len(data.Carries) != 2 {
		t.Fatal("incomplete SOSL Datetime-null native matrix")
	}
	checkHash := func(label, source, want string) {
		t.Helper()
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(source))); got != want {
			t.Fatalf("%s hash: got %s, native %s", label, got, want)
		}
	}
	checkObservation := func(label string, native map[string]any, want, receipt string) {
		t.Helper()
		canonical, err := json.Marshal(native)
		if err != nil {
			t.Fatal(err)
		}
		checkHash(label, string(canonical), want)
		if digest, err := hex.DecodeString(receipt); err != nil || len(digest) != sha256.Size {
			t.Fatalf("invalid raw receipt digest: %s", label)
		}
	}
	for path, source := range data.Metadata {
		if source == "" || !strings.HasPrefix(path, "objects/") || filepath.ToSlash(filepath.Clean(path)) != path {
			t.Fatalf("invalid owned metadata path: %q", path)
		}
		checkHash(path, source, data.MetadataSHA256[path])
	}
	carries := map[string]string{}
	for _, carry := range data.Carries {
		if (carry.ID != "D18" && carry.ID != "Q01") || carry.Reason == "" || carries[carry.ID] != "" {
			t.Fatalf("invalid carry: %#v", carry)
		}
		carries[carry.ID] = carry.Reason
	}
	wantCells := map[string]map[string]any{}
	for _, api := range data.APIVersions {
		for _, row := range rows {
			wantCells[api+"/istest/"+row] = nil
		}
	}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Route + "/" + cell.ID
		if previous, ok := wantCells[key]; !ok || previous != nil || cell.Native["row"] != cell.ID {
			t.Fatalf("unknown, duplicate or misbound native cell: %s", key)
		}
		checkObservation(key, cell.Native, cell.NativeSHA256, cell.RawReceiptSHA256)
		wantCells[key] = cell.Native
	}
	for key, native := range wantCells {
		if native == nil {
			t.Fatalf("missing native cell: %s", key)
		}
	}
	anonymous := map[string]bool{}
	for _, context := range data.AnonymousContexts {
		if _, ok := data.Fixtures[context.API]; !ok || anonymous[context.API] || context.Reason == "" || string(context.ExpectedSemanticNative) != "null" || context.Native == nil {
			t.Fatalf("invalid anonymous context: %#v", context)
		}
		checkObservation(context.API+"/anonymous", context.Native, context.NativeSHA256, context.RawReceiptSHA256)
		anonymous[context.API] = true
		t.Logf("anonymous API%s carry: %s", context.API, context.Reason)
	}
	parseObservation := func(t *testing.T, text string) (string, map[string]any) {
		t.Helper()
		_, frame, found := strings.Cut(text, "SDN|")
		if !found {
			t.Fatalf("missing native observation marker: %s", text)
		}
		row, payload, found := strings.Cut(frame, "|")
		if _, ok := wantCells[data.APIVersions[0]+"/istest/"+row]; !found || !ok {
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
		t.Run(api, func(t *testing.T) {
			fixture := data.Fixtures[api]
			if len(fixture.Files) != 5 || len(fixture.SourceSHA256) != 5 {
				t.Fatal("missing captured source bundle")
			}
			suffix := strings.TrimSuffix(api, ".0")
			helper := "DatetimeNullSearchProbe" + suffix
			spec := "DatetimeNullSearchSpec" + suffix
			for _, path := range []string{"classes/" + helper + ".cls", "classes/" + helper + ".cls-meta.xml", "classes/" + spec + ".cls", "classes/" + spec + ".cls-meta.xml", "anonymous-context.apex"} {
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
				if path != "anonymous-context.apex" {
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
			if run.Summary().Total != data.ShapeCount || len(observed) != data.ShapeCount {
				t.Fatalf("named selection: %#v: %s", run.Summary(), firstRunProblem(run))
			}
			for _, row := range rows {
				t.Run(row, func(t *testing.T) {
					want, got := wantCells[api+"/istest/"+row], observed[row]
					if reason, carry := carries[row]; carry {
						t.Logf("native carry: %s; native=%#v; local=%#v", reason, want, got)
						return
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native: %#v; Glade: %#v", want, got)
					}
				})
			}
		})
	}
}
