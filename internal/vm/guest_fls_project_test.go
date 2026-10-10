package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// SF200 admits this API62/project40 contract in a Sites-enabled org.
func TestSitesGuestContactFLSAPI62Project40(t *testing.T) {
	report := runGuestFLSProject(t, map[string]string{
		"GladeGuestFls62Assertions.cls": `@IsTest private class GladeGuestFls62Assertions {
 private static User userFor(Profile profileRecord,String suffix) {
  return new User(Alias='jdoe',Email='owned@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LastName='Doe',LocaleSidKey='en_US',ProfileId=profileRecord.Id,TimeZoneSidKey='America/Los_Angeles',Username='gladefls62assert.'+UserInfo.getOrganizationId()+'.'+Datetime.now().getTime()+'.'+suffix+'@example.invalid');
 }
 private static void assertContactFlags(Boolean expected) {
  Schema.DescribeSObjectResult obj=Schema.getGlobalDescribe().get('Contact').getDescribe();
  Schema.DescribeFieldResult field=obj.fields.getMap().get('LastName').getDescribe();
  System.assertEquals(expected,obj.isAccessible());
  System.assertEquals(expected,obj.isCreateable());
  System.assertEquals(expected,obj.isUpdateable());
  System.assertEquals(expected,field.isAccessible());
  System.assertEquals(expected,field.isCreateable());
  System.assertEquals(expected,field.isUpdateable());
  System.assertEquals(expected,Contact.LastName.getDescribe().isCreateable());
  System.assertEquals(expected,Contact.LastName.getDescribe().isUpdateable());
 }
 @IsTest static void originalGuestPredicateHasNoContactPermissions() {
  Set<String> profileNames=new Set<String>{'Standard Guest','Guest License User'};
  List<Profile> profiles=[SELECT Id FROM Profile WHERE Name IN :profileNames];
  System.assert(!profiles.isEmpty());
  User originalFirst=userFor(profiles[0],'first');
  System.runAs(originalFirst) {
   System.assertEquals('Guest',UserInfo.getUserType());
   assertContactFlags(false);
  }
  Integer i=0;
  for(Profile profileRecord:profiles) {
   System.runAs(userFor(profileRecord,'each'+i)) {
    System.assertEquals('Guest',UserInfo.getUserType());
    assertContactFlags(false);
   }
   i++;
  }
 }
 @IsTest static void administratorContactPermissionsRemainAvailable() {
  Profile admin=[SELECT Id FROM Profile WHERE PermissionsModifyAllData=TRUE AND UserType='Standard' LIMIT 1];
  System.runAs(userFor(admin,'admin')) {
   System.assertEquals('Standard',UserInfo.getUserType());
   assertContactFlags(true);
  }
 }
}
`,
	})
	if got := report.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
		logGuestFLSProblems(t, report)
		t.Fatalf("SF200 guest FLS contract: %+v", got)
	}
}

func TestSitesGuestFLSAllowsConfiguredGrantAndLeavesStandardDefault(t *testing.T) {
	report := runGuestFLSProject(t, map[string]string{
		"GladeGuestFlsConfigured62.cls": `@IsTest private class GladeGuestFlsConfigured62 {
 private static User userFor(Profile profileRecord,String suffix) {
  User u=new User(Alias='jdoe',Email='owned@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LastName='Doe',LocaleSidKey='en_US',ProfileId=profileRecord.Id,TimeZoneSidKey='America/Los_Angeles',Username='gladefls62configured.'+UserInfo.getOrganizationId()+'.'+Datetime.now().getTime()+'.'+suffix+'@example.invalid');
  insert u;
  return u;
 }
 private static void assertContactFlags(Boolean expected) {
  Schema.DescribeSObjectResult obj=Contact.SObjectType.getDescribe();
  Schema.DescribeFieldResult field=Contact.Description.getDescribe();
  System.assertEquals(expected,obj.isAccessible());
  System.assertEquals(expected,obj.isCreateable());
  System.assertEquals(expected,obj.isUpdateable());
  System.assertEquals(expected,field.isAccessible());
  System.assertEquals(expected,field.isCreateable());
  System.assertEquals(expected,field.isUpdateable());
 }
 @IsTest static void assignedPermissionSetCanGrantGuestContactAccess() {
  Profile guest=[SELECT Id FROM Profile WHERE UserType='Guest' LIMIT 1];
  User u=userFor(guest,'guest');
  PermissionSet ps=new PermissionSet(Name='GrantGuestContact',Label='Grant Guest Contact'); insert ps;
  insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Contact',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true);
  insert new FieldPermissions(ParentId=ps.Id,SObjectType='Contact',Field='Contact.Description',PermissionsRead=true,PermissionsEdit=true);
  insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id);
  System.runAs(u) { assertContactFlags(true); }
 }
 @IsTest static void standardProfileRetainsDefaultContactAccess() {
  Profile standard=[SELECT Id FROM Profile WHERE Name='Standard User'];
  System.runAs(userFor(standard,'standard')) { assertContactFlags(true); }
 }
}
`,
	})
	if got := report.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
		logGuestFLSProblems(t, report)
		t.Fatalf("guest explicit/non-guest controls: %+v", got)
	}
}

func logGuestFLSProblems(t *testing.T, report testreport.Run) {
	t.Helper()
	for _, suite := range report.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				t.Logf("%s: %s", c.MethodName, c.Problem.Message)
			}
		}
	}
}

func runGuestFLSProject(t *testing.T, classes map[string]string) testreport.Run {
	t.Helper()
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"40.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"glade.yml":         "org:\n  features: [Sites]\n",
	}
	for name, content := range classes {
		files[filepath.Join("force-app/main/default/classes", name)] = content
		files[filepath.Join("force-app/main/default/classes", name+"-meta.xml")] = `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return apextest.Run(typesys.Build(p, sch), apextest.Options{NoDiskCache: true})
}
