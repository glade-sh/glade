package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce's secure-coding USER_MODE guide documents the full set of FLS
// errors on QueryException.getInaccessibleFields. Exact messages are not asserted.
func TestExecUserModeExpandedProjectionDetails(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Profile base = [SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1];
User limited = new User(ProfileId=base.Id, Alias='capuser', LastName='Synthetic',
    Email='capture@example.invalid', EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US',
    LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles',
    Username='synthetic@example.invalid');
insert limited;
PermissionSet grant = new PermissionSet(Name='SyntheticContactRead', Label='Synthetic Contact Read');
insert grant;
insert new ObjectPermissions(ParentId=grant.Id, SObjectType='Contact', PermissionsRead=true,
    PermissionsCreate=false, PermissionsEdit=false, PermissionsDelete=false,
    PermissionsViewAllRecords=false, PermissionsModifyAllRecords=false);
insert new PermissionSetAssignment(AssigneeId=limited.Id, PermissionSetId=grant.Id);
System.runAs(limited) {
    Assert.isFalse(Case.SObjectType.getDescribe().isAccessible());
    Assert.isTrue(Contact.SObjectType.getDescribe().isAccessible());
    Assert.isFalse(Contact.Birthdate.getDescribe().isAccessible());
    Set<Id> keys = new Set<Id>{'500000000000001'};
    Boolean denied = false;
    try {
        Database.query('SELECT Subject,Id,Status,Priority FROM Case WHERE Id IN :keys WITH USER_MODE ORDER BY Subject ASC NULLS FIRST');
    } catch (QueryException issue) {
        denied = true;
        Map<String,Set<String>> inaccessible = issue.getInaccessibleFields();
        Assert.isTrue(inaccessible.containsKey('Case'));
        Assert.isTrue(inaccessible.get('Case').contains('Subject'));
        Assert.isTrue(inaccessible.get('Case').contains('Status'));
        Assert.isTrue(inaccessible.get('Case').contains('Priority'));
    }
    Assert.isTrue(denied);
    keys = new Set<Id>{'003000000000001'};
    denied = false;
    try {
        Database.query('SELECT Id,Name,Birthdate FROM Contact WHERE Id IN :keys WITH USER_MODE ORDER BY Name');
    } catch (QueryException issue) {
        denied = true;
        Map<String,Set<String>> inaccessible = issue.getInaccessibleFields();
        Assert.isTrue(inaccessible.containsKey('Contact'));
        Assert.isTrue(inaccessible.get('Contact').contains('Birthdate'));
        Assert.isFalse(inaccessible.get('Contact').contains('Name'));
    }
    Assert.isTrue(denied);
    Assert.areEqual(0, Database.query('SELECT Id,Name FROM Contact WHERE Id IN :keys WITH USER_MODE ORDER BY Name').size());
}

`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "Case")
			storage.EnsureStandardObject(&org, "Contact")
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUserModeExpandedProjectionShapeBoundaries(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Case")
	storage.EnsureStandardObject(&org, "Contact")
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"SELECT Subject,Id,Status,Priority FROM Case WHERE Id IN :keys WITH USER_MODE ORDER BY Subject ASC NULLS FIRST", true},
		{"SELECT Id,Name,Birthdate FROM Contact WHERE Id IN :keys WITH USER_MODE ORDER BY Name", true},
		{"SELECT Id,Name FROM Contact WHERE Id IN :keys WITH USER_MODE", true},
		{"SELECT Id,Subject FROM Case WHERE Id = :key WITH USER_MODE", false},
		{"SELECT Id,Subject FROM Case WHERE Subject IN :keys WITH USER_MODE", false},
		{"SELECT Id,Subject,Status,Priority,Origin FROM Case WHERE Id IN :keys WITH USER_MODE", false},
		{"SELECT Id,Owner.Name FROM Case WHERE Id IN :keys WITH USER_MODE", false},
		{"SELECT Id,Subject FROM Case WHERE Id IN :keys WITH USER_MODE LIMIT 1", false},
		{"SELECT Id,Subject FROM Case WHERE Id IN :keys WITH USER_MODE ORDER BY Status", false},
		{"SELECT Id,Subject,Status FROM Case WHERE Id IN :keys WITH USER_MODE ORDER BY Subject,Status", false},
	} {
		query, err := machine.parseSOQLAt(tc.query)
		if err != nil {
			t.Fatal(err)
		}
		if got := machine.userModeInaccessibleExpandedIDQuery(query); got != tc.want {
			t.Errorf("expanded details for %q = %t, want %t", tc.query, got, tc.want)
		}
	}
}
