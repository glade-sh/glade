package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
)

// Replay captured helper and test metadata independently. Async insert59/60
// differs within the same named route. Publication success was observed only
// in these active named tests; independent anonymous rejections establish no
// publication source-API boundary for the unmeasured named helper versions.
func TestLegacyDMLBoundaryNativeConformance(t *testing.T) {
	var data struct {
		NativeObservationCount int `json:"nativeObservationCount"`
		Sources                map[string]struct {
			TestAPI string `json:"testApi"`
			Files   map[string]struct {
				Source string `json:"source"`
				SHA256 string `json:"sha256"`
			} `json:"files"`
		} `json:"sources"`
		NativeFilesSHA256 map[string]string `json:"nativeFilesSha256"`
		Rows              []struct {
			ID             string          `json:"id"`
			SourceGroup    string          `json:"sourceGroup"`
			HelperClass    string          `json:"helperClass"`
			HelperAPI      string          `json:"helperApi"`
			ClassName      string          `json:"className"`
			MethodName     string          `json:"methodName"`
			RuntimeAPI     string          `json:"runtimeApi"`
			EvidenceSHA256 string          `json:"evidenceSha256"`
			NativeMessage  string          `json:"nativeMessage"`
			Native         json.RawMessage `json:"native"`
		} `json:"rows"`
		Carries []struct {
			Scope       string          `json:"scope"`
			Reason      string          `json:"reason"`
			NativeValue json.RawMessage `json:"nativeValue"`
		} `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/legacy_dml_boundary_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.NativeObservationCount != 6 || len(data.Rows) != 6 || len(data.Sources) != 3 || len(data.Carries) < 3 {
		t.Fatal("incomplete mixed-API native packet")
	}
	for name, hash := range data.NativeFilesSHA256 {
		if decoded, err := hex.DecodeString(hash); name == "" || err != nil || len(decoded) != sha256.Size {
			t.Fatal("invalid native provenance")
		}
	}
	for _, carry := range data.Carries {
		if carry.Scope == "" || carry.Reason == "" || !json.Valid(carry.NativeValue) {
			t.Fatal("incomplete native carry")
		}
		t.Logf("carried %s: %s; native=%s", carry.Scope, carry.Reason, carry.NativeValue)
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		t.Run(row.ID, func(t *testing.T) {
			if row.ID == "" || seen[row.ID] || row.RuntimeAPI != "68.0" || len(row.EvidenceSHA256) != 64 || (row.HelperAPI != "59.0" && row.HelperAPI != "60.0") {
				t.Fatal("invalid or duplicate native cell")
			}
			seen[row.ID] = true
			source, found := data.Sources[row.SourceGroup]
			if !found || source.TestAPI != "64.0" {
				t.Fatal("missing mixed-API source group")
			}
			root := t.TempDir()
			for name, file := range source.Files {
				if name == "" || filepath.IsAbs(name) || strings.Contains(name, "..") {
					t.Fatal("invalid owned source path")
				}
				sum := sha256.Sum256([]byte(file.Source))
				if hex.EncodeToString(sum[:]) != file.SHA256 {
					t.Fatalf("owned source changed: %s", name)
				}
				writeFile(t, filepath.Join(root, name), file.Source)
			}
			for className, want := range map[string]string{row.HelperClass: row.HelperAPI, row.ClassName: source.TestAPI} {
				path := "force-app/main/default/classes/" + className + ".cls"
				var metadata struct {
					API string `xml:"apiVersion"`
				}
				if source.Files[path].Source == "" {
					t.Fatal("missing captured class")
				}
				if err := xml.Unmarshal([]byte(source.Files[path+"-meta.xml"].Source), &metadata); err != nil || metadata.API != want {
					t.Fatalf("mixed-API metadata differs: %s", className)
				}
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native fixture compiles; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: row.MethodName, RuntimeRESTAPIVersion: row.RuntimeAPI, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected one selected observation, got %v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.ClassName != row.ClassName || result.MethodName != row.MethodName || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %+v", result)
			}
			want := legacyDMLBoundaryPayload(t, row.ID, row.NativeMessage)
			var retained map[string]any
			if err := json.Unmarshal(row.Native, &retained); err != nil || !reflect.DeepEqual(want, retained) {
				t.Fatal("native value differs from retained assertion transport")
			}
			got := legacyDMLBoundaryPayload(t, row.ID, result.Problem.Message)
			if !reflect.DeepEqual(legacyDMLBoundaryProjection(t, want), legacyDMLBoundaryProjection(t, got)) {
				t.Fatalf("native=%s; Glade=%s", row.Native, result.Problem.Message)
			}
		})
	}
}

func legacyDMLBoundaryPayload(t *testing.T, row, message string) map[string]any {
	t.Helper()
	marker := "NATIVE_RESULT|" + row + "|"
	_, payload, found := strings.Cut(message, marker)
	var observed map[string]any
	if !found || strings.Count(message, marker) != 1 || json.Unmarshal([]byte(payload), &observed) != nil || observed["row"] != row {
		t.Fatalf("invalid owned observation: %s", message)
	}
	return observed
}

// Only generated publication identities vary. Preserve key presence, validate
// their formats, and retain identity equality across immediate/after-stop/raw
// snapshots. Raw SaveResult JSON is compared structurally, including absent Id.
func legacyDMLBoundaryProjection(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	row, _ := value["row"].(string)
	publication := strings.HasPrefix(row, "PUBLISH")
	ids, uuids := map[string]string{}, map[string]string{}
	uuidPattern := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	var visit func(any) any
	visit = func(value any) any {
		switch value := value.(type) {
		case []any:
			out := make([]any, len(value))
			for i, item := range value {
				out[i] = visit(item)
			}
			return out
		case map[string]any:
			out := make(map[string]any, len(value))
			keys := make([]string, 0, len(value))
			for key := range value {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				item := value[key]
				if key == "raw" {
					text, ok := item.(string)
					var raw map[string]any
					if !ok || json.Unmarshal([]byte(text), &raw) != nil {
						t.Fatal("raw SaveResult is not a JSON object")
					}
					out[key] = visit(raw)
					continue
				}
				if publication && key == "id" && item != nil {
					id, ok := item.(string)
					if !ok || len(id) != 18 || !strings.HasPrefix(id, "1QQ") || storage.ValidateID(storage.ID(id)) != nil {
						t.Fatalf("invalid native/local publication identity: %v", item)
					}
					if ids[id] == "" {
						ids[id] = fmt.Sprintf("publication-id-%d", len(ids)+1)
					}
					out[key] = ids[id]
					continue
				}
				if publication && key == "message" && value["statusCode"] == "OPERATION_ENQUEUED" {
					message, ok := item.(string)
					if !ok || !uuidPattern.MatchString(message) {
						t.Fatalf("enqueue message is not a UUID: %v", item)
					}
					if uuids[message] == "" {
						uuids[message] = fmt.Sprintf("enqueue-uuid-%d", len(uuids)+1)
					}
					out[key] = uuids[message]
					continue
				}
				out[key] = visit(item)
			}
			return out
		default:
			return value
		}
	}
	return visit(value).(map[string]any)
}
