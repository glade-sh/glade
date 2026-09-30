package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Regression controls preserve the pre-enrichment record shapes; they do not
// establish Salesforce contracts for uncaptured length/default behavior.
func TestStubPermissionEnrichmentPreservesExistingDMLShapes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"63.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/StubScopeProof.cls"), `@IsTest private class StubScopeProof {
 @IsTest static void longGroupName(){String name='Total Opportunities greater than 200 into Annual Revenue on Account';Group row=new Group(Name=name,Type='Regular');insert row;System.assertNotEquals(null,row.Id);}
 @IsTest static void sparseObjectPermissions(){PermissionSet ps=new PermissionSet(Label='testPermSet',Name='testPermSet');insert ps;ObjectPermissions op=new ObjectPermissions(ParentId=ps.Id,SobjectType='Account');op.PermissionsRead=true;op.PermissionsCreate=true;op.PermissionsEdit=false;insert op;System.assertNotEquals(null,op.Id);}
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/StubScopeProof.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Passed != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("preservation: %s", data)
	}
}
