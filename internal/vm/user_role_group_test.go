package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// RG01-RG03 native API62/67/68 anonymous and named observations on the
// default Developer org (Communities disabled) preserve Role groups and expose
// an Internal group per role. Names, memberships and sharing are not asserted.
func TestExecUserRoleGeneratedGroups(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"existing Role control", `
UserRole role = new UserRole(Name='Owned control', DeveloperName='OwnedControl');
insert role;
List<Group> groups = [SELECT Id, RelatedId FROM Group WHERE RelatedId=:role.Id AND Type='Role'];
System.assertEquals(1, groups.size());
Id originalId = groups[0].Id;
role.Name = 'Owned changed'; update role;
System.assertEquals(originalId, [SELECT Id FROM Group WHERE RelatedId=:role.Id AND Type='Role'].Id);
`},
		{"fresh", `
UserRole role = new UserRole(Name='Owned north', DeveloperName='OwnedNorth');
insert role;
System.assertEquals(1, [SELECT COUNT() FROM Group WHERE RelatedId=:role.Id AND Type='Role'], 'existing Role control');
System.assertEquals(1, [SELECT COUNT() FROM Group WHERE RelatedId=:role.Id AND Type='RoleAndSubordinatesInternal'], 'observed Internal presence');
`},
		{"parent and child", `
UserRole parent = new UserRole(Name='Owned parent', DeveloperName='OwnedParent');
insert parent;
UserRole child = new UserRole(Name='Owned child', DeveloperName='OwnedChild', ParentRoleId=parent.Id);
insert child;
for (UserRole role : new List<UserRole>{parent, child}) {
    System.assertEquals(1, [SELECT COUNT() FROM Group WHERE RelatedId=:role.Id AND Type='Role'], 'existing Role control');
    System.assertEquals(1, [SELECT COUNT() FROM Group WHERE RelatedId=:role.Id AND Type='RoleAndSubordinatesInternal'], 'observed Internal identity');
}
`},
		{"updates and distinct roles", `
UserRole first = new UserRole(Name='Owned first', DeveloperName='OwnedFirst');
UserRole second = new UserRole(Name='Owned second', DeveloperName='OwnedSecond');
insert new List<UserRole>{first, second};
List<Group> beforeRows = [SELECT Id, Type, RelatedId FROM Group WHERE RelatedId=:first.Id AND Type='RoleAndSubordinatesInternal'];
System.assertEquals(1, beforeRows.size(), 'observed first Internal presence');
Id beforeId = beforeRows[0].Id;
first.Name = 'Owned first updated';
update first;
update first;
List<Group> afterRows = [SELECT Id, Type, RelatedId FROM Group WHERE RelatedId=:first.Id AND Type='RoleAndSubordinatesInternal'];
System.assertEquals(1, afterRows.size(), 'idempotent update');
System.assertEquals(beforeId, afterRows[0].Id, 'Internal identity remains');
System.assertEquals(1, [SELECT COUNT() FROM Group WHERE RelatedId=:first.Id AND Type='Role'], 'existing first Role control');
System.assertEquals(1, [SELECT COUNT() FROM Group WHERE RelatedId=:second.Id AND Type='Role'], 'existing second Role control');
System.assertEquals(1, [SELECT COUNT() FROM Group WHERE RelatedId=:second.Id AND Type='RoleAndSubordinatesInternal'], 'second role Internal presence');
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "UserRole")
			storage.EnsureStandardObject(&org, "Group")
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
