package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// P0 REST describe (plain Developer org, API 62.0 and 67.0): EmailMessage.Status
// is a restricted picklist with values 0..5. An Apex picklist is a String, so
// String access, switch, SOQL and DML with a listed value stay legal.
func TestStubRestrictedPicklistStringAccess(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "EmailMessage")
			source := `EmailMessage m = new EmailMessage(Subject='S', Status='3');
insert m;
String s = m.Status;
System.assertEquals('3', s);
EmailMessage q = [SELECT Status FROM EmailMessage WHERE Status = '3' AND Id = :m.Id];
System.assertEquals('3', q.Status);
switch on q.Status { when '3' { } when else { System.assert(false, q.Status); } }
Schema.DescribeFieldResult d = EmailMessage.Status.getDescribe();
System.assertEquals(Schema.DisplayType.PICKLIST, d.getType());
System.assert(d.isRestrictedPicklist());
System.assertEquals(6, d.getPicklistValues().size());
System.assertEquals('Sent', d.getPicklistValues()[3].getLabel());
Schema.DescribeFieldResult j = AsyncApexJob.Status.getDescribe();
System.assertEquals(Schema.DisplayType.PICKLIST, j.getType());
System.assert(j.isRestrictedPicklist());`
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
