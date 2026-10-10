package vm_test

import (
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"testing"
)

// SF137 accepted all three original-version contracts: public read/write edit,
// public read-only rejection, and no delete grant from public read/write.
func TestPublicSharingWriteOperationProject(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	files := map[string]string{
		"force-app/main/default/classes/GladeSharedWrite55.cls":                                  "public with sharing class GladeSharedWrite55 { public static Database.SaveResult change(SObject row){return Database.update(row,false);} public static Database.DeleteResult remove(SObject row){return Database.delete(row,false);} }",
		"force-app/main/default/classes/GladeSharedWrite55.cls-meta.xml":                         "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>",
		"force-app/main/default/objects/GladeSharedWrite__c/GladeSharedWrite__c.object-meta.xml": "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>GladeSharedWrite__c</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>GladeSharedWrite__c Records</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>",
		"force-app/main/default/objects/GladeSharedRead__c/GladeSharedRead__c.object-meta.xml":   "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>GladeSharedRead__c</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>GladeSharedRead__c Records</pluralLabel><sharingModel>Read</sharingModel></CustomObject>",
		"force-app/main/default/classes/GladePublicWrite59Proof.cls":                             "@IsTest private with sharing class GladePublicWrite59Proof {\n private static User actor(){Profile p=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce'];User u=new User(Username='glade137.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='reader',Email='reader@example.invalid',LastName='Reader',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');insert u;PermissionSet ps=new PermissionSet(Name='Glade137',Label='Glade137');insert ps;for(String obj:new List<String>{'GladeSharedWrite__c','GladeSharedRead__c'}){insert new ObjectPermissions(ParentId=ps.Id,SObjectType=obj,PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true,PermissionsDelete=true,PermissionsViewAllRecords=false,PermissionsModifyAllRecords=false);}insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id);return u;}\n private static Id row(Boolean writable){Id id;System.runAs(new User(Id=UserInfo.getUserId())){SObject s=writable?(SObject)new GladeSharedWrite__c(Name='before'):(SObject)new GladeSharedRead__c(Name='before');insert s;id=(Id)s.get('Id');}return id;}\n @IsTest static void publicReadWriteAllowsOtherOwnerUpdate(){User u=actor();Id id=row(true);System.runAs(u){Database.SaveResult r=GladeSharedWrite55.change(new GladeSharedWrite__c(Id=id,Name='after'));System.assertEquals(true,r.isSuccess(),String.valueOf(r.getErrors()));System.assertEquals('after',[SELECT Name FROM GladeSharedWrite__c WHERE Id=:id].Name);}System.assertEquals('after',[SELECT Name FROM GladeSharedWrite__c WHERE Id=:id].Name);}\n @IsTest static void publicReadOnlyRejectsOtherOwnerUpdate(){User u=actor();Id id=row(false);System.runAs(u){System.assertEquals(1,[SELECT COUNT() FROM GladeSharedRead__c WHERE Id=:id]);Database.SaveResult r=GladeSharedWrite55.change(new GladeSharedRead__c(Id=id,Name='after'));System.assertEquals(false,r.isSuccess());System.assertEquals(StatusCode.INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY,r.getErrors()[0].getStatusCode());}System.assertEquals('before',[SELECT Name FROM GladeSharedRead__c WHERE Id=:id].Name);}\n @IsTest static void publicReadWriteDoesNotGrantDelete(){User u=actor();Id id=row(true);System.runAs(u){Database.DeleteResult r=GladeSharedWrite55.remove(new GladeSharedWrite__c(Id=id));System.assertEquals(false,r.isSuccess());System.assertEquals(StatusCode.INSUFFICIENT_ACCESS_OR_READONLY,r.getErrors()[0].getStatusCode());}System.assertEquals(1,[SELECT COUNT() FROM GladeSharedWrite__c WHERE Id=:id]);}\n}",
		"force-app/main/default/classes/GladePublicWrite59Proof.cls-meta.xml":                    "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>59.0</apiVersion><status>Active</status></ApexClass>",
		"sfdx-project.json": "{\"sourceApiVersion\": \"64.0\", \"namespace\": \"proofpkg\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}",
	}
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	r := apextest.Run(typesys.Build(p, s), apextest.Options{NoDiskCache: true})
	if got := r.Summary(); got.Total != 3 || got.Passed != 3 {
		t.Fatalf("public sharing operation: %+v; %+v", got, r)
	}
}
