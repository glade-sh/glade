package automation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// Every Flow validation pass needs the project schema. One LoadProject call
// reads it once and hands the same value to each pass.
func TestLoadProjectReadsSchemaOnce(t *testing.T) {
	dir := t.TempDir()
	object := filepath.Join(dir, "objects", "Account")
	if err := os.MkdirAll(filepath.Join(object, "fields"), 0o750); err != nil {
		t.Fatal(err)
	}
	objectFile := filepath.Join(object, "Account.object-meta.xml")
	writeWorkflowTestFile(t, objectFile, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Account</label></CustomObject>`)
	fieldFile := filepath.Join(object, "fields", "Tier__c.field-meta.xml")
	writeWorkflowTestFile(t, fieldFile, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Tier__c</fullName><type>Text</type><length>20</length><label>Tier</label></CustomField>`)
	flow := filepath.Join(dir, "Route.flow-meta.xml")
	writeWorkflowTestFile(t, flow, `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType><status>Active</status>
  <start><object>Account</object><recordTriggerType>CreateAndUpdate</recordTriggerType><triggerType>RecordAfterSave</triggerType><connector><targetReference>Write</targetReference></connector>
    <filterLogic>and</filterLogic><filters><field>Tier__c</field><operator>EqualTo</operator><value><stringValue>Gold</stringValue></value></filters></start>
  <recordUpdates><name>Write</name><inputReference>$Record</inputReference><inputAssignments><field>Description</field><value><stringValue>Routed</stringValue></value></inputAssignments></recordUpdates>
</Flow>`)
	p := project.Project{Root: dir, FlowFiles: []string{flow}, ObjectFiles: []string{objectFile}, FieldFiles: []string{fieldFile}}

	loads := 0
	previous := loadProjectSchema
	loadProjectSchema = func(p project.Project) (schema.Schema, error) {
		loads++
		return previous(p)
	}
	t.Cleanup(func() { loadProjectSchema = previous })

	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if loads != 1 {
		t.Fatalf("schema loaded %d times, want 1", loads)
	}
	if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
		t.Fatalf("flow did not load: %#v", idx)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
}
