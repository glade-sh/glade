package project

import (
	"path/filepath"
	"testing"
)

// unpackaged/pre and unpackaged/post hold metadata that the project's setup
// deploys around its package before tests run. The directory convention is
// the declaration; a missing cumulusci.yml does not make that metadata absent.
// Other unpackaged bundles (config, test metadata) stay outside the project.
func TestLoadCollectsSetupMetadataStagesWithoutCumulusCIConfig(t *testing.T) {
	for _, configured := range []bool{false, true} {
		name := "unconfigured"
		if configured {
			name = "cumulusci"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/Main.cls"), "public class Main {}")
			if configured {
				writeFile(t, filepath.Join(root, "cumulusci.yml"), "project:\n  name: Owned\n")
			}
			object := `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><recordTypes><fullName>Owned</fullName><label>Owned</label><active>true</active></recordTypes></CustomObject>`
			pre := filepath.Join(root, "unpackaged/pre/owned/objects/Account.object")
			post := filepath.Join(root, "unpackaged/post/owned/objects/Contact.object")
			writeFile(t, pre, object)
			writeFile(t, post, object)
			writeFile(t, filepath.Join(root, "unpackaged/config/owned/objects/Opportunity.object"), object)

			p, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]bool{}
			for _, path := range p.ObjectFiles {
				got[filepath.Base(path)] = true
			}
			if !got["Account.object"] || !got["Contact.object"] || got["Opportunity.object"] || len(p.ObjectFiles) != 2 {
				t.Fatalf("object files = %#v, want pre and post setup metadata only", p.ObjectFiles)
			}
			if len(p.ApexFiles) != 1 || filepath.Base(p.ApexFiles[0]) != "Main.cls" {
				t.Fatalf("apex files = %#v", p.ApexFiles)
			}
		})
	}
}
