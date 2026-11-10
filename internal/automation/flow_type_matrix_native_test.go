package automation

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The deployment rows record the source/target combinations accepted by
// Salesforce. A compatible type does not imply successful interview execution.
func TestFlowTypeMatrixNativeAdmission(t *testing.T) {
	const root = "../apextest/testdata/native-capture/flow_type_matrix"
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := []byte(`{"objects":[{"name":"Account","fieldsComplete":true,"fields":[{"name":"Name","type":"Text"},{"name":"NumberOfEmployees","type":"Number"},{"name":"AnnualRevenue","type":"Currency"},{"name":"OwnerId","type":"Lookup","referenceTo":"User"},{"name":"Type","type":"Picklist"},{"name":"CreatedDate","type":"DateTime"}]},{"name":"Contact","fieldsComplete":true,"fields":[{"name":"LastName","type":"Text"},{"name":"Birthdate","type":"Date"},{"name":"IsEmailBounced","type":"Checkbox"}]}]}`)
	dir := t.TempDir()
	snapshotPath := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(snapshotPath, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(snapshot)
	var accepted, rejected int
	for _, row := range rows[1:] {
		if len(row) != 4 || row[1] != "deploy" {
			continue
		}
		api := strings.TrimSuffix(row[0], ".0")
		if api != "62" && api != "67" {
			t.Fatalf("unexpected API version %q", row[0])
		}
		name, result := row[2], row[3]
		wantReject := strings.HasPrefix(result, "rejected:")
		if !wantReject && result != "accepted" {
			t.Fatalf("unexpected deployment result for %s: %q", name, result)
		}
		if wantReject {
			rejected++
		} else {
			accepted++
		}
		t.Run(name, func(t *testing.T) {
			flow := filepath.Join(root, "api"+api, name+".flow-meta.xml")
			p := project.Project{FlowFiles: []string{flow}, SchemaSnapshot: snapshotPath, SchemaSnapshotSHA256: fmt.Sprintf("%x", sum)}
			idx, err := LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, d := range idx.Diagnostics {
				found = found || d.Code == "GLADEAUTO004"
			}
			if found != wantReject {
				t.Fatalf("native deployment result %q: reject=%t, diagnostics=%+v", result, found, idx.Diagnostics)
			}
			if wantReject && (len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 0) {
				t.Fatalf("rejected flow retained rules: %+v", idx.Flows)
			}
		})
	}
	if accepted == 0 || rejected == 0 {
		t.Fatalf("missing native controls: accepted=%d rejected=%d", accepted, rejected)
	}
}

// When the schema or source type cannot be established, a native mismatch in a
// known shape must not cause an inference about this different shape.
func TestFlowTypeMatrixUnknownSchemaAndSource(t *testing.T) {
	root := "../apextest/testdata/native-capture/flow_type_matrix"
	for _, tc := range []struct {
		name, file, before, after string
	}{
		{"unknown_lookup_object", "F62InNumberString", "<object>Account</object>", "<object>Missing_Synthetic__c</object>"},
		{"unknown_transform_object", "T62NumberString", "<objectType>Account</objectType>", "<objectType>Missing_Synthetic__c</objectType>"},
		{"unknown_transform_source", "T62NumberString", "<elementReference>inputValue</elementReference>", "<elementReference>MissingInput</elementReference>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "api62", tc.file+".flow-meta.xml"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tc.before) {
				t.Fatalf("fixture does not contain %q", tc.before)
			}
			flow := filepath.Join(t.TempDir(), tc.file+".flow-meta.xml")
			if err := os.WriteFile(flow, []byte(strings.Replace(string(data), tc.before, tc.after, 1)), 0600); err != nil {
				t.Fatal(err)
			}
			idx, err := LoadProject(project.Project{FlowFiles: []string{flow}})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range idx.Diagnostics {
				if d.Code == "GLADEAUTO004" {
					t.Fatalf("unproven type mismatch rejected: %+v", idx.Diagnostics)
				}
			}
		})
	}
}
