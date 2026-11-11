package automation

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Salesforce rejects duplicate output targets and unknown target fields in
// the captured API 62 and 67 Transform deployments.
func TestFlowTransformTargetValidationNative(t *testing.T) {
	for _, api := range []string{"api62", "api67"} {
		t.Run(api, func(t *testing.T) {
			fixture := filepath.Join("..", "apextest", "testdata", "native-capture")
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

func TestFlowTransformValidationSnapshotBoundaries(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "native-capture", "flow_transform_record_map", "api62")
	valid, err := os.ReadFile(filepath.Join(fixture, "TransformProbe.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := os.ReadFile(filepath.Join(fixture, "TransformReject.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, metadata, snapshot, namespace string
		badHash, reject                     bool
	}{
		{
			name: "partial_snapshot_cannot_prove_missing", metadata: string(invalid),
			snapshot: `{"objects":[{"name":"Contact","partial":true,"fieldsComplete":true,"fields":[{"name":"LastName","type":"Text"}]}]}`,
		},
		{
			name: "invalid_hash_cannot_prove_missing", metadata: string(invalid), badHash: true,
			snapshot: `{"objects":[{"name":"Contact","fieldsComplete":true,"fields":[{"name":"LastName","type":"Text"}]}]}`,
		},
		{
			name: "namespace_and_case", namespace: "probe",
			metadata: strings.Replace(string(valid), "<outputFieldApiName>Title</outputFieldApiName>", "<outputFieldApiName>kNoWn__c</outputFieldApiName>", 1),
			snapshot: `{"objects":[{"name":"Contact","fieldsComplete":true,"fields":[{"name":"LastName","type":"Text"},{"name":"probe__Known__c","type":"Text"}]}]}`,
		},
		{
			name: "namespace_missing_control", namespace: "probe", reject: true,
			metadata: string(invalid),
			snapshot: `{"objects":[{"name":"Contact","fieldsComplete":true,"fields":[{"name":"LastName","type":"Text"},{"name":"probe__Known__c","type":"Text"}]}]}`,
		},
		{
			name: "duplicate_in_second_transform", reject: true,
			metadata: strings.Replace(string(valid), "</Flow>", `<transforms><name>OtherMap</name><dataType>SObject</dataType><isCollection>false</isCollection><objectType>Contact</objectType><transformValues><transformValueActions><outputFieldApiName>Title</outputFieldApiName><transformType>Map</transformType><value><elementReference>InputText</elementReference></value></transformValueActions><transformValueActions><outputFieldApiName>Title</outputFieldApiName><transformType>Map</transformType><value><elementReference>NullText</elementReference></value></transformValueActions></transformValues></transforms></Flow>`, 1),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			flow := filepath.Join(root, "probe.flow-meta.xml")
			if err := os.WriteFile(flow, []byte(tc.metadata), 0600); err != nil {
				t.Fatal(err)
			}
			p := project.Project{FlowFiles: []string{flow}, Namespace: tc.namespace}
			if tc.snapshot != "" {
				data := []byte(tc.snapshot)
				p.SchemaSnapshot = filepath.Join(root, "schema.json")
				if err := os.WriteFile(p.SchemaSnapshot, data, 0600); err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(data)
				p.SchemaSnapshotSHA256 = fmt.Sprintf("%x", sum)
				if tc.badHash {
					p.SchemaSnapshotSHA256 = strings.Repeat("0", 64)
				}
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
		})
	}
}
