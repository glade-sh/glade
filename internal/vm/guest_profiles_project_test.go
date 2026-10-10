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

// SF194 admits this API62/project40 contract in a Sites-enabled org.
func TestSitesGuestProfileRunAsAPI62Project40(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"40.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"glade.yml":         "org:\n  features: [Sites]\n",
		"force-app/main/default/classes/GladeGuestProfile62Assertions.cls": `@IsTest private class GladeGuestProfile62Assertions {
 private static List<Profile> getProfiles(Set<String> profileNames) {
  return [SELECT Id FROM Profile WHERE Name IN :profileNames];
 }
 private static User createUser(String email, Profile profileRecord) {
  return new User(Alias='jdoe',Email=email,EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LastName='Doe',LocaleSidKey='en_US',ProfileId=profileRecord.Id,TimeZoneSidKey='America/Los_Angeles',Username=email+Datetime.now().getTime());
 }
 private static User createGuestUser() {
  List<Profile> profiles=getProfiles(new Set<String>{'Standard Guest','Guest License User'});
  if(profiles.isEmpty()) return null;
  return createUser('ownedguest@testorg.com',profiles[0]);
 }
 @IsTest static void originalGuestHelperSupportsUninsertedRunAs() {
  User guest=createGuestUser();
  System.assertNotEquals(null,guest,'Sites org satisfies original guest profile predicate');
  System.assertEquals(null,guest.Id,'original helper constructs an uninserted User');
  Boolean entered=false;
  System.runAs(guest) {
   entered=true;
   System.assertEquals('Guest',UserInfo.getUserType(),'guest runtime context');
  }
  System.assertEquals(true,entered,'runAs entered and returned successfully');
 }
}
`,
		"force-app/main/default/classes/GladeGuestProfile62Assertions.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`,
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
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	report := apextest.Run(typesys.Build(p, sch), apextest.Options{NoDiskCache: true})
	if got := report.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
		for _, suite := range report.Suites {
			for _, c := range suite.Cases {
				if c.Problem != nil {
					t.Logf("%s: %s", c.MethodName, c.Problem.Message)
				}
			}
		}
		t.Fatalf("guest contract: %+v", got)
	}
}
