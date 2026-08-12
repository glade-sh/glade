package schema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestCB117CustomObjectEnableSearchMetadata(t *testing.T) {
	root := t.TempDir()
	defaultPath := filepath.Join(root, "force-app/main/objects/Default__c/Default__c.object-meta.xml")
	enabledPath := filepath.Join(root, "force-app/main/objects/Enabled__c/Enabled__c.object-meta.xml")
	writeFile(t, defaultPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Default</label></CustomObject>`)
	writeFile(t, enabledPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Enabled</label><enableSearch>true</enableSearch></CustomObject>`)

	loaded, err := LoadProject(project.Project{ObjectFiles: []string{defaultPath, enabledPath}})
	if err != nil {
		t.Fatal(err)
	}
	objects := objectsByName(loaded.Objects)
	if objects["Default__c"].EnableSearch {
		t.Fatal("custom object without enableSearch defaulted to searchable")
	}
	if !objects["Enabled__c"].EnableSearch {
		t.Fatal("custom object with enableSearch=true was not marked searchable")
	}
}
