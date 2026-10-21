package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// These are the two independently observed API61 named rows. The observation
// transport intentionally fails; only its JSON payload supplies native values.
func TestRecordComparatorObjectViewNativeConformance(t *testing.T) {
	var data struct {
		SourceAPI, TransportAPI, Route, DeploymentID, DeploymentStatus string
		CheckOnly                                                      bool
		Classes, ClassAPIs, SourceSHA256                               map[string]string
		RawAnswerSHA256, OrgTSVSHA256, OrgRunSHA256                    string
		Rows                                                           []struct {
			ID, Method, RawPayload, RawPayloadSHA256 string
			Native                                   map[string]any
		}
	}
	raw, err := os.ReadFile("testdata/conformance/record_comparator_object_view_api61.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.SourceAPI != "61.0" || data.TransportAPI != "61.0" || data.Route != "istest" || !data.CheckOnly || data.DeploymentID == "" || data.DeploymentStatus != "Failed" || len(data.Rows) != 2 || len(data.Classes) != 2 {
		t.Fatal("incomplete native API61 scope")
	}
	checkHash := func(t *testing.T, text, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(text))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("native source/payload bytes changed")
		}
	}
	for _, digest := range []string{data.RawAnswerSHA256, data.OrgTSVSHA256, data.OrgRunSHA256} {
		if b, err := hex.DecodeString(digest); err != nil || len(b) != sha256.Size {
			t.Fatal("missing native receipt digest")
		}
	}
	for name, source := range data.Classes {
		if data.ClassAPIs[name] != "61.0" {
			t.Fatal("helper/caller version changed")
		}
		checkHash(t, source, data.SourceSHA256[name])
	}
	helper, err := os.ReadFile("testdata/record_comparator_object_view/RecordOrderingFixture.cls")
	if err != nil {
		t.Fatal(err)
	}
	if string(helper) != data.Classes["RecordOrderingFixture"] {
		t.Fatal("regression helper differs from native fixture")
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if seen[row.ID] {
			t.Fatal("duplicate native row")
		}
		seen[row.ID] = true
		t.Run(row.ID, func(t *testing.T) {
			checkHash(t, row.RawPayload, row.RawPayloadSHA256)
			var captured any
			if err := json.Unmarshal([]byte(row.RawPayload), &captured); err != nil || !reflect.DeepEqual(captured, row.Native) {
				t.Fatal("native projection differs from captured payload")
			}
			index := nativeBoundaryIndex(t, data.SourceAPI, data.Classes, nil)
			result := Run(index, Options{SelectedClasses: []string{"RecordOrderingCandidate"}, SelectedMethod: row.Method, NoDiskCache: true, Parallelism: 1})
			if s := result.Summary(); s.Total != 1 || s.Failed != 1 || s.CompileErrors != 0 {
				t.Fatalf("missing observation transport: %+v", s)
			}
			count := 0
			for _, suite := range result.Suites {
				for _, c := range suite.Cases {
					count++
					if c.Problem == nil || !strings.HasSuffix(c.Problem.Type, "AssertException") || !strings.Contains(c.Problem.Message, "NATIVE_OBSERVATION") {
						t.Fatalf("unexpected observation failure: %+v", c)
					}
					_, payload, ok := strings.Cut(c.Problem.Message, "Actual: ")
					var got any
					if !ok || json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(got, row.Native) {
						t.Fatalf("native=%s; local=%s", row.RawPayload, payload)
					}
				}
			}
			if count != 1 {
				t.Fatal("wrong observation count", count)
			}
		})
	}
	if !seen["RECORD_OBJECT_VIEW"] || !seen["SOBJECT_OBJECT_VIEW"] {
		t.Fatal("missing native pair")
	}
}
