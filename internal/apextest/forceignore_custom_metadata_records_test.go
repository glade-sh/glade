package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// A project whose .forceignore lists its customMetadata directory never
// deploys those records, so Type__mdt.getInstance returns null for them, as
// it does in an org built from that source. Without the ignore line the record
// is found. Local project behavior; no native capture.
func TestForceignoreCustomMetadataRecordsAreNotLoaded(t *testing.T) {
	for _, ignored := range []bool{false, true} {
		name := map[bool]string{false: "kept", true: "ignored"}[ignored]
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			if ignored {
				writeFile(t, filepath.Join(root, ".forceignore"), "force-app/main/default/customMetadata\n")
			}
			base := "force-app/main/default/"
			writeFile(t, filepath.Join(root, base, "objects/Handler__mdt/Handler__mdt.object-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Handler</label><pluralLabel>Handlers</pluralLabel></CustomObject>`)
			writeFile(t, filepath.Join(root, base, "objects/Handler__mdt/fields/Active__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Active__c</fullName><label>Active</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
			writeFile(t, filepath.Join(root, base, "customMetadata/Handler.Case.md-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>Case</label><protected>false</protected><values><field>Active__c</field><value xsi:type="xsd:boolean">true</value></values></CustomMetadata>`)
			writeFile(t, filepath.Join(root, base, "classes/Probe.cls"), `@IsTest private class Probe { @IsTest static void observe() { Handler__mdt h = Handler__mdt.getInstance('Case'); System.assert(false, 'R|' + (h == null ? 'null' : 'found')); } }`)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"Probe"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("unexpected test selection: %+v", run.Summary())
			}
			want := map[bool]string{false: "R|found", true: "R|null"}[ignored]
			if got := run.Suites[0].Cases[0].Problem.Message; !strings.Contains(got, want) {
				t.Fatalf("ignored=%v: want %q; got %q", ignored, want, got)
			}
		})
	}
}
