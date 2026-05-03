package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSFDXProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true}],
  "namespace": "NU",
  "sourceApiVersion": "61.0"
}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello {}")
	writeFile(t, filepath.Join(root, "force-app/main/triggers/Hello.trigger"), "trigger Hello on Account (before insert) {}")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), "<CustomObject/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/fields/Name__c.field-meta.xml"), "<CustomField/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/recordTypes/Business.recordType-meta.xml"), "<RecordType/>")
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/validationRules/Block.validationRule-meta.xml"), "<ValidationRule/>")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Namespace != "NU" || p.SourceAPIVersion != "61.0" {
		t.Fatalf("project metadata = %#v", p)
	}
	if len(p.ApexFiles) != 2 || len(p.ObjectFiles) != 1 || len(p.FieldFiles) != 1 || len(p.RecordTypeFiles) != 1 || len(p.ValidationRuleFiles) != 1 {
		t.Fatalf("unexpected file counts: %#v", p)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
