package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Source 62 at transport 67 completed seven intentional observation assertions.
// Compare their observations; do not call the native tests passing tests.
// The source-67 access failures remain an open gate and are not retargeted here.
func TestMultiSelectEmptySelectionSource62Conformance(t *testing.T) {
	var data struct {
		SourceAPI                       string            `json:"sourceApi"`
		TransportAPI                    string            `json:"transportApi"`
		Route                           string            `json:"route"`
		NativeProcessExit               int               `json:"nativeProcessExit"`
		NativeIntentionalAssertionCount int               `json:"nativeIntentionalAssertionCount"`
		NativeComponentErrors           int               `json:"nativeComponentErrors"`
		Source67TimingEstablished       bool              `json:"source67TimingEstablished"`
		Files                           map[string]string `json:"files"`
		FileSHA256                      map[string]string `json:"fileSHA256"`
		Cells                           []map[string]any  `json:"cells"`
	}
	raw, err := os.ReadFile("testdata/conformance/multiselect_empty_selection/source62.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.SourceAPI != "62.0" || data.TransportAPI != "67.0" || data.Route != "isTest" ||
		data.NativeProcessExit != 1 || data.NativeIntentionalAssertionCount != 7 || data.NativeComponentErrors != 0 ||
		data.Source67TimingEstablished || len(data.Cells) != 7 || len(data.Files) != len(data.FileSHA256) {
		t.Fatal("incomplete or mislabelled source-62 native observation fixture")
	}
	root := t.TempDir()
	for path, source := range data.Files {
		digest := sha256.Sum256([]byte(source))
		if hex.EncodeToString(digest[:]) != data.FileSHA256[path] {
			t.Fatalf("captured owned fixture changed: %s", path)
		}
		writeFile(t, filepath.Join(root, path), source)
	}
	index := loadTestIndex(t, root)
	for _, want := range data.Cells {
		shape, ok := want["shape"].(string)
		if !ok || want["succeeded"] != true {
			t.Fatal("missing successful DML observation identity")
		}
		t.Run(shape, func(t *testing.T) {
			run := Run(index, Options{NoDiskCache: true, RuntimeRESTAPIVersion: data.TransportAPI,
				SelectedClasses: []string{"OwnedMultiTimingProbe"}, SelectedMethod: shape})
			if summary := run.Summary(); summary.Total != 1 || summary.Failed != 1 || summary.Errors != 0 ||
				len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				encoded, _ := json.Marshal(run)
				t.Fatalf("expected one deliberate observation assertion: %s", encoded)
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || result.Problem.Type != "System.AssertException" {
				t.Fatalf("unexpected observation failure: %#v", result.Problem)
			}
			const marker = "OWNED_MULTI_TIMING="
			start := strings.Index(result.Problem.Message, marker)
			if start < 0 {
				t.Fatalf("observation marker missing: %s", result.Problem.Message)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(result.Problem.Message[start+len(marker):]), &got); err != nil {
				t.Fatalf("decode local observation: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				actual, _ := json.Marshal(got)
				expected, _ := json.Marshal(want)
				t.Fatalf("source62/transport67 observation mismatch:\ngot  %s\nwant %s", actual, expected)
			}
		})
	}
}
