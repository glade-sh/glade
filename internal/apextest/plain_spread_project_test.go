package apextest

import (
	"path/filepath"
	"testing"
)

func TestPlainDeveloperSpreadKeepsExplicitProjectObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":[],"settings":{}}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/CareProgram/CareProgram.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"></CustomObject>`)
	index := loadTestIndex(t, root)
	if len(index.Objects) == 0 {
		t.Fatal("project object not loaded")
	}
	org := orgFromIndex(index)
	if _, ok := org.Objects["CareProgram"]; !ok {
		t.Fatal("explicit project object removed")
	}
}

func TestPlainDeveloperSpreadKeepsExplicitCapturedProjectObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":[],"settings":{}}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/AIFeatureExtractor/AIFeatureExtractor.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"></CustomObject>`)
	index := loadTestIndex(t, root)
	if len(index.Objects) == 0 {
		t.Fatal("project object not loaded")
	}
	org := orgFromIndex(index)
	if _, ok := org.Objects["AIFeatureExtractor"]; !ok {
		t.Fatal("explicit captured project object removed")
	}
}
