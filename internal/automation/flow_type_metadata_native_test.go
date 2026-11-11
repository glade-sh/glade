package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A field metadata declaration supplies a known type even without a complete
// snapshot. The equivalent Number source/target cases are captured in
// testdata/native-capture/flow_type_matrix/org.tsv at API 62 and 67.
func TestFlowTypeMatrixFieldMetadataAdmission(t *testing.T) {
	const capture = "../apextest/testdata/native-capture/flow_type_matrix"
	for _, api := range []string{"62", "67"} {
		for _, tc := range []struct {
			flow    string
			reject  bool
			unknown bool
		}{
			{"InNumberString", true, false},
			{"NotInNumberString", true, false},
			{"InNumberNumber", false, false},
			{"NumberString", true, false},
			{"NumberNumber", false, false},
			{"NumberString", false, true},
		} {
			prefix := "F"
			if !strings.HasPrefix(tc.flow, "In") && !strings.HasPrefix(tc.flow, "NotIn") {
				prefix = "T"
			}
			name := prefix + api + tc.flow
			testName := name
			if tc.unknown {
				testName += "_unknown_field"
			}
			t.Run(testName, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(capture, "api"+api, name+".flow-meta.xml"))
				if err != nil {
					t.Fatal(err)
				}
				target := "Measure__c"
				if tc.unknown {
					target = "Unspecified__c"
				}
				text := strings.ReplaceAll(string(data), "NumberOfEmployees", target)
				if text == string(data) {
					t.Fatal("expected captured target field")
				}
				root := t.TempDir()
				flow := filepath.Join(root, "flow.flow-meta.xml")
				field := filepath.Join(root, "objects", "Account", "fields", "Measure__c.field-meta.xml")
				writeWorkflowTestFile(t, flow, text)
				writeWorkflowTestFile(t, field, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Measure__c</fullName><label>Measure</label><type>Number</type><precision>9</precision><scale>0</scale></CustomField>`)
				idx, err := LoadProject(project.Project{FlowFiles: []string{flow}, FieldFiles: []string{field}})
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, d := range idx.Diagnostics {
					found = found || d.Code == "GLADEAUTO004"
				}
				if found != tc.reject {
					t.Fatalf("reject=%t, want %t: diagnostics=%+v", found, tc.reject, idx.Diagnostics)
				}
				if tc.reject && (len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 0) {
					t.Fatalf("rejected flow retained rules: %+v", idx.Flows)
				}
			})
		}
	}
}
