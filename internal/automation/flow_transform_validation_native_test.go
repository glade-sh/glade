package automation

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Salesforce rejects duplicate output targets and unknown target fields in
// the captured API 62 and 67 Transform deployments.
func TestFlowTransformTargetValidationNative(t *testing.T) {
	for _, api := range []string{"api62", "api67"} {
		t.Run(api, func(t *testing.T) {
			fixture := filepath.Join("..", "apextest", "testdata", "conformance")
			for _, tc := range []struct {
				name, family, file string
				complete, reject   bool
			}{
				{"accepted", "flow_transform_record_map", "TransformProbe", true, false},
				{"missing_complete", "flow_transform_record_map", "TransformReject", true, true},
				{"missing_incomplete", "flow_transform_record_map", "TransformReject", false, false},
				{"duplicate_null_first", "flow_transform_collection_null", "DuplicateNullFirst", false, true},
				{"duplicate_null_last", "flow_transform_collection_null", "DuplicateNullLast", false, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					file := filepath.Join(fixture, tc.family, api, tc.file+".flow-meta.xml")
					p := project.Project{FlowFiles: []string{file}}
					if tc.complete {
						data := []byte(`{"objects":[{"name":"Contact","fieldsComplete":true,"fields":[{"name":"LastName","type":"Text"},{"name":"Title","type":"Text"}]}]}`)
						snapshot := filepath.Join(t.TempDir(), "schema.json")
						if err := os.WriteFile(snapshot, data, 0600); err != nil {
							t.Fatal(err)
						}
						sum := sha256.Sum256(data)
						p.SchemaSnapshot, p.SchemaSnapshotSHA256 = snapshot, fmt.Sprintf("%x", sum)
					}
					idx, err := LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, d := range idx.Diagnostics {
						found = found || d.Code == "GLADEAUTO004"
					}
					if found != tc.reject {
						t.Fatalf("reject=%t diagnostics=%+v", tc.reject, idx.Diagnostics)
					}
					if tc.reject && (len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 0) {
						t.Fatalf("rejected Flow retained rules: %+v", idx.Flows)
					}
				})
			}
		})
	}
}
