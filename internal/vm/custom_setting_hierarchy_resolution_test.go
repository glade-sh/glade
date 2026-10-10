package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecHierarchyCustomSettingGetInstancePrefersCurrentUser(t *testing.T) {
	program, err := CompileAnonymous(`
Hierarchy_Setting__c userSetting = new Hierarchy_Setting__c(SetupOwnerId = UserInfo.getUserId(), Enabled__c = false);
insert userSetting;
Hierarchy_Setting__c organizationSetting = new Hierarchy_Setting__c(SetupOwnerId = UserInfo.getOrganizationId(), Enabled__c = true);
insert organizationSetting;
Hierarchy_Setting__c resolved = Hierarchy_Setting__c.getInstance();
System.assertEquals(false, resolved.Enabled__c);
System.assertEquals(true, Hierarchy_Setting__c.getOrgDefaults().Enabled__c);
`)
	if err != nil {
		t.Fatal(err)
	}

	org := customDataOrg()
	hierarchy := org.Objects["Hierarchy_Setting__c"]
	hierarchy.Records = make(map[storage.ID]storage.Record)
	org.Objects["Hierarchy_Setting__c"] = hierarchy
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	if _, err := machine.Execute(program); err != nil {
		t.Logf("org records after execution: %#v", machine.Org.Objects["Hierarchy_Setting__c"].Records)
		t.Fatal(err)
	}
}
