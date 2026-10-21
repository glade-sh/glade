package automation

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

const standardFieldDecisionFlow = `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start><object>Account</object><triggerType>RecordAfterSave</triggerType></start>
  <decisions>
    <name>Check</name>
    <rules>
      <name>Matches</name>
      <conditions><leftValueReference>%s</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>%s</stringValue></rightValue></conditions>
    </rules>
  </decisions>
  <assignments>
    <name>Assign_Tier</name>
    <assignmentItems><assignToReference>$Record.Tier__c</assignToReference><operator>Assign</operator><value><stringValue>Gold</stringValue></value></assignmentItems>
  </assignments>
</Flow>`

const accountObjectMetadata = `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>`

const accountTierField = `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>Tier__c</fullName>
  <label>Tier</label>
  <type>Text</type>
  <length>20</length>
</CustomField>`

// projectWithAccountTier writes a project that customizes the standard Account
// object with one custom field, with or without the object's own metadata file.
func projectWithAccountTier(t *testing.T, root string, withObjectFile bool) project.Project {
	t.Helper()
	field := filepath.Join(root, "force-app/main/default/objects/Account/fields/Tier__c.field-meta.xml")
	writeWorkflowTestFile(t, field, accountTierField)
	p := project.Project{FieldFiles: []string{field}}
	if withObjectFile {
		object := filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml")
		writeWorkflowTestFile(t, object, accountObjectMetadata)
		p.ObjectFiles = []string{object}
	}
	return p
}

// A project that declares only a custom field on a standard object still has
// that object's standard fields: a decision on one must keep the flow active.
func TestFlowDecisionsOnStandardFieldsKeepTheFlowActive(t *testing.T) {
	for _, withObjectFile := range []bool{false, true} {
		for _, tc := range []struct {
			name, reference, value string
			wantRule               bool
		}{
			{"standard field", "$Record.Industry", "Banking", true},
			{"owner id", "$Record.OwnerId", "005000000000001AAA", true},
			{"owner relationship field", "$Record.Owner.Name", "Pat", false},
			{"audit date", "$Record.CreatedDate", "2024-01-01", false},
		} {
			t.Run(fmt.Sprintf("objectFile=%v/%s", withObjectFile, tc.name), func(t *testing.T) {
				root := t.TempDir()
				p := projectWithAccountTier(t, root, withObjectFile)
				flow := filepath.Join(root, "force-app/main/default/flows/Tier_Flow.flow-meta.xml")
				writeWorkflowTestFile(t, flow, fmt.Sprintf(standardFieldDecisionFlow, tc.reference, tc.value))
				p.FlowFiles = []string{flow}
				idx, err := LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				for _, diag := range idx.Diagnostics {
					if diag.Code == "GLADEAUTO004" {
						t.Fatalf("valid standard-field decision rejected: %#v", diag)
					}
				}
				if len(idx.Flows) != 1 {
					t.Fatalf("flows = %#v", idx.Flows)
				}
				if tc.wantRule && len(idx.Flows[0].Rules) != 1 {
					t.Fatalf("rules = %#v; the flow lost its rule", idx.Flows[0].Rules)
				}
			})
		}
	}
}

// The captured rejection shape stays: a decision on a custom field that the
// project's own object definition lacks is still an invalid definition.
func TestFlowDecisionOnMissingCustomFieldStaysRejected(t *testing.T) {
	root := t.TempDir()
	p := projectWithAccountTier(t, root, true)
	flow := filepath.Join(root, "force-app/main/default/flows/Tier_Flow.flow-meta.xml")
	writeWorkflowTestFile(t, flow, fmt.Sprintf(standardFieldDecisionFlow, "$Record.Missing__c", "x"))
	p.FlowFiles = []string{flow}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, diag := range idx.Diagnostics {
		found = found || diag.Code == "GLADEAUTO004"
	}
	if !found || len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 0 {
		t.Fatalf("custom-field rejection lost: diagnostics=%#v flows=%#v", idx.Diagnostics, idx.Flows)
	}
}

// A project whose schema cannot be read still loads: decision checks need a
// schema and are skipped, the flow keeps its rules, and no error escapes.
func TestFlowDecisionCheckSkippedWhenTheSchemaCannotBeRead(t *testing.T) {
	root := t.TempDir()
	field := filepath.Join(root, "force-app/main/default/objects/Account/fields/Tier__c.field-meta.xml")
	flow := filepath.Join(root, "force-app/main/default/flows/Tier_Flow.flow-meta.xml")
	writeWorkflowTestFile(t, field, "<CustomField><fullName>Tier__c</fullName>")
	writeWorkflowTestFile(t, flow, fmt.Sprintf(standardFieldDecisionFlow, "$Record.Industry", "Banking"))
	idx, err := LoadProject(project.Project{FlowFiles: []string{flow}, FieldFiles: []string{field}})
	if err != nil {
		t.Fatalf("LoadProject() error = %v; an unreadable schema must not fail the whole load", err)
	}
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
		t.Fatalf("flows = %#v; the flow lost its rule", idx.Flows)
	}
}
