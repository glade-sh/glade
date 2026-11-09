package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

// Native: testdata/conformance/csnpe_capture/org.tsv ASSERTIONS and CS_LIST_EXCEPTION.
func TestCustomSettingNullFieldProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "objects/ProbeList__c/ProbeList__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Probe List</label><customSettingsType>List</customSettingsType><visibility>Public</visibility></CustomObject>`)
			writeFile(t, filepath.Join(base, "objects/ProbeList__c/fields/Text__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Text__c</fullName><label>Text</label><length>80</length><type>Text</type></CustomField>`)
			path := filepath.Join(base, "classes/SettingNullTest.cls")
			writeFile(t, path, `@IsTest private class SettingNullTest { @IsTest static void checkNull() {
for (Integer i=0;i<2;i++) {
 ProbeList__c s=ProbeList__c.getValues('absent');
 System.assertEquals(null,s);
 Boolean caught=false; try { String v=s.Text__c; } catch (NullPointerException e) { caught=true; System.assertEquals('Attempt to de-reference a null object',e.getMessage()); }
 System.assert(caught);
}
} }`)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			for i := 0; i < 2; i++ {
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"SettingNullTest"}})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
					data, _ := json.Marshal(run)
					t.Fatalf("iteration %d: %s", i, data)
				}
			}
		})
	}
}
