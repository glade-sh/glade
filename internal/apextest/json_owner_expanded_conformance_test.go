package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The captured source API is independent of the org REST API used in generated
// SObject URLs. Preserve both, and compare the transported JSON structurally.
func TestJSONExpandedOwnerOrgConformance(t *testing.T) {
	runJSONExpandedOwnerOrgConformance(t, "json_owner_expanded.json", 7)
}

func TestJSONOwnerRESTURLOrgConformance(t *testing.T) {
	runJSONExpandedOwnerOrgConformance(t, "json_owner_rest_url.json", 4)
}

func TestJSONGeneratedOwnerEnvelopeOrgConformance(t *testing.T) {
	runJSONExpandedOwnerOrgConformance(t, "json_owner_generated_envelope.json", 6)
}

func TestJSONOwnerCanonicalURL65OrgConformance(t *testing.T) {
	runJSONExpandedOwnerOrgConformance(t, "json_owner_url65_canonical.json", 6)
}

func runJSONExpandedOwnerOrgConformance(t *testing.T, file string, shapeCount int) {
	t.Helper()
	var data struct {
		APIVersion             string              `json:"apiVersion"`
		RuntimeAPIVersion      string              `json:"runtimeAPIVersion"`
		ShapeCount             int                 `json:"shapeCount"`
		NativeObservationCount int                 `json:"nativeObservationCount"`
		Source                 nativeBoundaryRoute `json:"source"`
		NativeFilesSHA256      map[string]string   `json:"nativeFilesSha256"`
		Rows                   []struct {
			ID                    string                               `json:"id"`
			TestMethod            string                               `json:"testMethod"`
			Anonymous             string                               `json:"anonymous"`
			SourceSHA256          string                               `json:"sourceSha256"`
			ExpectedByRoute       map[string]nativeObservationExpected `json:"expectedByRoute"`
			EvidenceSHA256ByRoute map[string]string                    `json:"evidenceSha256ByRoute"`
		} `json:"rows"`
		Carries []struct {
			Scope       string          `json:"scope"`
			Reason      string          `json:"reason"`
			NativeValue json.RawMessage `json:"nativeValue"`
		} `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/" + file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.APIVersion != "65.0" || data.RuntimeAPIVersion != "68.0" || data.ShapeCount != shapeCount || len(data.Rows) != shapeCount || data.NativeObservationCount != 2*shapeCount || len(data.Carries) == 0 {
		t.Fatal("incomplete expanded-owner native matrix")
	}
	if data.Source.RuntimeAPIVersion != data.RuntimeAPIVersion {
		t.Fatal("source/runtime API distinction missing")
	}
	nativeBoundaryCheckSources(t, data.Source)
	for name, sum := range data.NativeFilesSHA256 {
		if name == "" || len(sum) != 64 {
			t.Fatal("incomplete raw-evidence provenance")
		}
	}
	for _, carry := range data.Carries {
		if carry.Scope == "" || carry.Reason == "" || len(carry.NativeValue) == 0 || !json.Valid(carry.NativeValue) {
			t.Fatal("incomplete unanswered-cell carry")
		}
		t.Logf("carried %s: %s; native value=%s", carry.Scope, carry.Reason, carry.NativeValue)
	}
	counts := newConformanceCountLog(t, "JSONExpandedOwner", data.APIVersion)
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || row.TestMethod == "" || len(row.ExpectedByRoute) != 2 || len(row.EvidenceSHA256ByRoute) != 2 {
			t.Fatal("invalid or repeated expanded-owner row")
		}
		seen[row.ID] = true
		sum := sha256.Sum256([]byte(row.Anonymous))
		if hex.EncodeToString(sum[:]) != row.SourceSHA256 {
			t.Fatal("captured anonymous source changed")
		}
		for _, route := range []string{"anonymous", "isTest"} {
			if len(row.EvidenceSHA256ByRoute[route]) != 64 || row.ExpectedByRoute[route].Outcome != "runtime" {
				t.Fatal("missing native runtime cell")
			}
		}
		t.Run(row.ID, func(t *testing.T) {
			t.Run("anonymous", func(t *testing.T) {
				counts.TrackKind(t, "anonymous", "exact", 1)
				index := nativeBoundaryIndex(t, data.APIVersion, data.Source.Classes, nil)
				analysis := sema.Analyze(index)
				anonymous := sema.AnalyzeAnonymous(index, row.Anonymous, data.APIVersion)
				if index.HasErrors() || analysis.HasErrors() || anonymous.HasErrors() {
					t.Fatalf("native executes; Glade diagnostics=%v %v %v", index.Diagnostics, analysis.Diagnostics, anonymous.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: data.APIVersion})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = data.RuntimeAPIVersion
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				marker := "NATIVE_RESULT|" + row.ID + "|"
				var observation string
				for _, line := range result.Debug {
					if strings.HasPrefix(line, marker) {
						if observation != "" {
							t.Fatal("repeated observation")
						}
						observation = strings.TrimPrefix(line, marker)
					}
				}
				var expected, actual map[string]any
				if observation == "" || json.Unmarshal(row.ExpectedByRoute["anonymous"].Value, &expected) != nil || json.Unmarshal([]byte(observation), &actual) != nil {
					t.Fatal("missing or invalid native/local observation")
				}
				if !reflect.DeepEqual(expected, actual) {
					t.Fatalf("native=%s; Glade=%s", row.ExpectedByRoute["anonymous"].Value, observation)
				}
			})
			t.Run("isTest", func(t *testing.T) {
				counts.TrackKind(t, "isTest", "exact", 1)
				source := data.Source
				source.ID, source.TestMethod = row.ID, row.TestMethod
				residualObservationObserve(t, data.APIVersion, "isTest", source, row.ExpectedByRoute["isTest"])
			})
		})
	}
}
