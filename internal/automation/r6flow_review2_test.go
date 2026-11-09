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

func TestR6FlowPartialSnapshotDoesNotRejectEntry(t *testing.T) {
	root := t.TempDir()
	snapshot := []byte(`{"objects":[{"name":"Opportunity","partial":true}]}`)
	path := filepath.Join(root, "snapshot.json")
	if err := os.WriteFile(path, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(snapshot)
	flow := filepath.Join(root, "force-app/main/default/flows/R6.flow-meta.xml")
	writeWorkflowTestFile(t, flow, `<Flow><processType>AutoLaunchedFlow</processType><status>Active</status><start><object>Opportunity</object><triggerType>RecordBeforeSave</triggerType><recordTriggerType>Create</recordTriggerType><filters><field>Unknown__c</field><operator>EqualTo</operator><value><stringValue>x</stringValue></value></filters></start></Flow>`)
	idx, err := LoadProject(project.Project{FlowFiles: []string{flow}, SchemaSnapshot: path, SchemaSnapshotSHA256: fmt.Sprintf("%x", sum)})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range idx.Diagnostics {
		if d.Code == "GLADEAUTO004" {
			t.Fatalf("partial snapshot rejected entry: %v", d)
		}
	}
}

func TestR6FlowNonRoutedGlobalRemainsUnsupported(t *testing.T) {
	root := t.TempDir()
	p := projectWithAccountTier(t, root, true)
	flow := filepath.Join(root, "force-app/main/default/flows/R6.flow-meta.xml")
	writeWorkflowTestFile(t, flow, fmt.Sprintf(standardFieldDecisionFlow, "$Profile.Name", "System Administrator"))
	p.FlowFiles = []string{flow}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range idx.Diagnostics {
		if d.Code == "GLADEAUTO002" && strings.Contains(d.Message, "$Profile.Name") {
			return
		}
	}
	t.Fatalf("unmodeled non-routed decision lost warning: %v", idx.Diagnostics)
}
