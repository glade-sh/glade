package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: scratch org, classes at API 59 through 67. Type.forName('Schema',
// sObjectName) returns the SObject type below API 60 and null from API 60.
func TestTypeForNameSchemaNamespaceByClassAPIVersion(t *testing.T) {
	cls := `@isTest
public class TfnProbe {
  @isTest static void run() {
    String o = '';
    for (String n : new List<String>{'Task','Case','Account','Probe__c'}) {
      Type t = Type.forName('Schema', n);
      o += '\nTFN|Schema,' + n + '|' + (t == null ? 'null' : t.getName());
      Type t2 = Type.forName('', n);
      o += '\nTFN|empty,' + n + '|' + (t2 == null ? 'null' : t2.getName());
      Type t3 = Type.forName(n);
      o += '\nTFN|single,' + n + '|' + (t3 == null ? 'null' : t3.getName());
      try { SObject s = (SObject)Type.forName('Schema', n).newInstance(); o += '\nTFN|newInstance,' + n + '|ok ' + s.getSObjectType(); } catch (Exception e) { o += '\nTFN|newInstance,' + n + '|' + e.getTypeName(); }
    }
    System.assert(false, o);
  }
}
`
	want := map[string]string{
		"59": `TFN|Schema,Task|Task
TFN|empty,Task|Task
TFN|single,Task|Task
TFN|newInstance,Task|ok Task
TFN|Schema,Case|Case
TFN|empty,Case|Case
TFN|single,Case|Case
TFN|newInstance,Case|ok Case
TFN|Schema,Account|Account
TFN|empty,Account|Account
TFN|single,Account|Account
TFN|newInstance,Account|ok Account
TFN|Schema,Probe__c|Probe__c
TFN|empty,Probe__c|Probe__c
TFN|single,Probe__c|Probe__c
TFN|newInstance,Probe__c|ok Probe__c`,
		"60": `TFN|Schema,Task|null
TFN|empty,Task|Task
TFN|single,Task|Task
TFN|newInstance,Task|System.NullPointerException
TFN|Schema,Case|null
TFN|empty,Case|Case
TFN|single,Case|Case
TFN|newInstance,Case|System.NullPointerException
TFN|Schema,Account|null
TFN|empty,Account|Account
TFN|single,Account|Account
TFN|newInstance,Account|System.NullPointerException
TFN|Schema,Probe__c|null
TFN|empty,Probe__c|Probe__c
TFN|single,Probe__c|Probe__c
TFN|newInstance,Probe__c|System.NullPointerException`,
		"61": `TFN|Schema,Task|null
TFN|empty,Task|Task
TFN|single,Task|Task
TFN|newInstance,Task|System.NullPointerException
TFN|Schema,Case|null
TFN|empty,Case|Case
TFN|single,Case|Case
TFN|newInstance,Case|System.NullPointerException
TFN|Schema,Account|null
TFN|empty,Account|Account
TFN|single,Account|Account
TFN|newInstance,Account|System.NullPointerException
TFN|Schema,Probe__c|null
TFN|empty,Probe__c|Probe__c
TFN|single,Probe__c|Probe__c
TFN|newInstance,Probe__c|System.NullPointerException`,
		"62": `TFN|Schema,Task|null
TFN|empty,Task|Task
TFN|single,Task|Task
TFN|newInstance,Task|System.NullPointerException
TFN|Schema,Case|null
TFN|empty,Case|Case
TFN|single,Case|Case
TFN|newInstance,Case|System.NullPointerException
TFN|Schema,Account|null
TFN|empty,Account|Account
TFN|single,Account|Account
TFN|newInstance,Account|System.NullPointerException
TFN|Schema,Probe__c|null
TFN|empty,Probe__c|Probe__c
TFN|single,Probe__c|Probe__c
TFN|newInstance,Probe__c|System.NullPointerException`,
		"67": `TFN|Schema,Task|null
TFN|empty,Task|Task
TFN|single,Task|Task
TFN|newInstance,Task|System.NullPointerException
TFN|Schema,Case|null
TFN|empty,Case|Case
TFN|single,Case|Case
TFN|newInstance,Case|System.NullPointerException
TFN|Schema,Account|null
TFN|empty,Account|Account
TFN|single,Account|Account
TFN|newInstance,Account|System.NullPointerException
TFN|Schema,Probe__c|null
TFN|empty,Probe__c|Probe__c
TFN|single,Probe__c|Probe__c
TFN|newInstance,Probe__c|System.NullPointerException`,
	}
	for version, expected := range want {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			const base = "force-app/main/default/"
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+version+`.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, base, "objects/Probe__c/Probe__c.object-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Probe</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Probes</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			writeFile(t, filepath.Join(root, base, "classes/TfnProbe.cls"), cls)
			writeFile(t, filepath.Join(root, base, "classes/TfnProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+version+`.0</apiVersion><status>Active</status></ApexClass>`)
			run := Run(loadTestIndex(t, root), Options{})
			var got string
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if c.Problem != nil {
						got = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: ")
					}
				}
			}
			if strings.TrimSpace(got) != strings.TrimSpace(expected) {
				t.Fatalf("native Type.forName rows differ at API %s\ngot:\n%s\nwant:\n%s", version, got, expected)
			}
		})
	}
}
