package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact API63 fixture admitted by SF189.
func TestCustomPermissionAPI63Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "63.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCustomPermission63Proof.cls"), `@IsTest private class GladeCustomPermission63Proof {
 private static Id customPermissionId(String permissionName) {
  return [SELECT Id FROM CustomPermission WHERE DeveloperName = :permissionName WITH SYSTEM_MODE LIMIT 1].Id;
 }
 @IsTest static void originalMetadataPermissionIsQueryable() {
  System.assertNotEquals(null,customPermissionId('GladeCustomPermission63Owned'));
 }
 @IsTest static void originalSetupEntityAssignmentEnablesPermission() {
  PermissionSet permSet=new PermissionSet(Name='GladeCustomPermission63TestSet',Label='Test Permission Set');
  insert permSet;
  Id permissionId=customPermissionId('GladeCustomPermission63Owned');
  SetupEntityAccess permSetPermission=new SetupEntityAccess(ParentId=permSet.Id,SetupEntityId=permissionId);
  PermissionSetAssignment permSetAssignment=new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),PermissionSetId=permSet.Id);
  insert new List<SObject>{permSetPermission,permSetAssignment};
  System.runAs(new User(Id=UserInfo.getUserId())) {
   System.assertEquals(true,FeatureManagement.checkPermission('GladeCustomPermission63Owned'));
  }
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCustomPermission63Proof.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/customPermissions/GladeCustomPermission63Owned.customPermission-meta.xml"), `<?xml version="1.0" encoding="UTF-8" ?>
<CustomPermission xmlns="http://soap.sforce.com/2006/04/metadata">
    <isLicensed>false</isLicensed>
    <label>ApexKit Example</label>
</CustomPermission>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Passed != 2 || got.Total != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("exact proof: %s", data)
	}
}
