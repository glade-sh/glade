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

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestDescribeSourceShadowConstructorNativeConformance(t *testing.T) {
	runDescribeSourceShadowNativeConformance(t, "constructor")
}

func TestDescribeSourceShadowJSONNativeConformance(t *testing.T) {
	runDescribeSourceShadowNativeConformance(t, "json")
}

// The lexical Schema class and its nested type are captured Apex source, not
// platform descriptors. Constructor and JSON observations remain separate.
func runDescribeSourceShadowNativeConformance(t *testing.T, phase string) {
	t.Helper()
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		NativeObservationCount int      `json:"nativeObservationCount"`
		Sources                map[string]map[string]struct {
			Source string `json:"source"`
			SHA256 string `json:"sha256"`
		} `json:"sources"`
		NativeFilesSHA256 map[string]string `json:"nativeFilesSha256"`
		Rows              []struct {
			Phase               string          `json:"phase"`
			API                 string          `json:"api"`
			Route               string          `json:"route"`
			Object              string          `json:"object"`
			ID                  string          `json:"id"`
			SourceGroup         string          `json:"sourceGroup"`
			Anonymous           string          `json:"anonymous,omitempty"`
			AnonymousSHA256     string          `json:"anonymousSha256,omitempty"`
			ClassName           string          `json:"className,omitempty"`
			NamedSource         string          `json:"namedSource,omitempty"`
			NamedSourceSHA256   string          `json:"namedSourceSha256,omitempty"`
			NamedMetadata       string          `json:"namedMetadata,omitempty"`
			NamedMetadataSHA256 string          `json:"namedMetadataSha256,omitempty"`
			EvidenceSHA256      string          `json:"evidenceSha256"`
			Native              json.RawMessage `json:"native"`
		} `json:"rows"`
		Carries []struct {
			Scope       string          `json:"scope"`
			Reason      string          `json:"reason"`
			NativeValue json.RawMessage `json:"nativeValue"`
		} `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/describe_source_shadow_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.NativeObservationCount != 24 || len(data.Rows) != 24 || len(data.Carries) < 2 {
		t.Fatal("incomplete source-shadow native packet")
	}
	for name, sum := range data.NativeFilesSHA256 {
		decoded, err := hex.DecodeString(sum)
		if name == "" || err != nil || len(decoded) != sha256.Size {
			t.Fatal("invalid native provenance")
		}
	}
	for _, carry := range data.Carries {
		if carry.Scope == "" || carry.Reason == "" || !json.Valid(carry.NativeValue) {
			t.Fatal("incomplete source-shadow carry")
		}
		t.Logf("carried %s: %s; native=%s", carry.Scope, carry.Reason, carry.NativeValue)
	}
	seen := map[string]bool{}
	selected := 0
	for _, row := range data.Rows {
		if row.Phase != phase {
			continue
		}
		selected++
		key := row.API + "/" + row.Route + "/" + row.Object
		t.Run(key, func(t *testing.T) {
			if seen[key] || (row.Object != "User" && row.Object != "Account") || (row.API != "62.0" && row.API != "67.0" && row.API != "68.0") || len(row.EvidenceSHA256) != 64 {
				t.Fatal("invalid or duplicate native cell")
			}
			seen[key] = true
			root := t.TempDir()
			sources, found := data.Sources[row.SourceGroup]
			if !found {
				t.Fatal("missing source group")
			}
			for name, source := range sources {
				if name == "" || filepath.IsAbs(name) || strings.Contains(name, "..") {
					t.Fatal("invalid captured source path")
				}
				describeSourceShadowCheckHash(t, source.Source, source.SHA256)
				writeFile(t, filepath.Join(root, name), source.Source)
			}
			checkAPI := func(text string) {
				var metadata struct {
					API string `xml:"apiVersion"`
				}
				if err := xml.Unmarshal([]byte(text), &metadata); err != nil || metadata.API != row.API {
					t.Fatal("captured source API differs")
				}
			}
			checkAPI(sources["force-app/main/default/classes/Schema.cls-meta.xml"].Source)
			if row.Route == "isTest" {
				describeSourceShadowCheckHash(t, row.NamedSource, row.NamedSourceSHA256)
				describeSourceShadowCheckHash(t, row.NamedMetadata, row.NamedMetadataSHA256)
				checkAPI(row.NamedMetadata)
				path := filepath.Join(root, "force-app/main/default/classes", row.ClassName+".cls")
				writeFile(t, path, row.NamedSource)
				writeFile(t, path+"-meta.xml", row.NamedMetadata)
			} else if row.Route != "anonymous" {
				t.Fatal("uncaptured route")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source accepted; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			var message string
			if row.Route == "anonymous" {
				describeSourceShadowCheckHash(t, row.Anonymous, row.AnonymousSHA256)
				analysis := sema.AnalyzeAnonymous(index, row.Anonymous, row.API)
				if analysis.HasErrors() {
					t.Fatalf("native anonymous accepted; Glade diagnostics=%v", analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: row.API})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				marker := "NATIVE_RESULT|" + row.ID + "|"
				for _, line := range result.Debug {
					if strings.HasPrefix(line, marker) {
						if message != "" {
							t.Fatal("repeated native observation")
						}
						message = line
					}
				}
			} else {
				run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("expected one named observation, got %v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
					t.Fatalf("missing assertion transport: %+v", result)
				}
				message = result.Problem.Message
			}
			marker := "NATIVE_RESULT|" + row.ID + "|"
			_, payload, found := strings.Cut(message, marker)
			var want, got map[string]any
			if !found || strings.Count(message, marker) != 1 || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal(row.Native, &want) != nil {
				t.Fatalf("invalid observation: %s", message)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("native=%s; Glade=%s", row.Native, payload)
			}
		})
	}
	if selected != 12 {
		t.Fatalf("phase %s has %d native cells, want 12", phase, selected)
	}
}

func describeSourceShadowCheckHash(t *testing.T, source, expected string) {
	t.Helper()
	sum := sha256.Sum256([]byte(source))
	if source == "" || hex.EncodeToString(sum[:]) != expected {
		t.Fatal("captured source changed")
	}
}
