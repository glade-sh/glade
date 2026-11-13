package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

const (
	boundaryStandardProfile  storage.ID = "00e000000000001"
	boundaryCommunityProfile storage.ID = "00e000000000002"
	boundaryGuestProfile     storage.ID = "00e000000000003"
	boundaryCustomProfile    storage.ID = "00e000000000004"
)

// boundaryProfiles adds the profiles the named-user and Lead-write guards read.
func boundaryProfiles(org *storage.OrgState) {
	storage.EnsureStandardObject(org, "Profile")
	profiles := org.Objects["Profile"]
	if profiles.Records == nil {
		profiles.Records = map[storage.ID]storage.Record{}
	}
	add := func(id storage.ID, name, userType string) {
		fields := map[string]storage.Value{"Name": storage.StringValue(name)}
		if userType != "" {
			fields["UserType"] = storage.StringValue(userType)
		}
		profiles.Records[id] = storage.Record{ID: id, Object: "Profile", Fields: fields}
	}
	add(boundaryStandardProfile, "Standard User", "")
	add(boundaryCommunityProfile, "Customer Community Login User", "")
	add(boundaryGuestProfile, "Site Guest User", "Guest")
	add(boundaryCustomProfile, "Custom Profile", "")
	org.Objects["Profile"] = profiles
}

func boundaryUserFields(userType string, profile storage.ID) map[string]storage.Value {
	fields := map[string]storage.Value{"IsActive": storage.BooleanValue(true)}
	if userType != "" {
		fields["UserType"] = storage.StringValue(userType)
	}
	if profile != "" {
		fields["ProfileId"] = storage.IDValue(profile)
	}
	return fields
}

// A UserRecordAccess query naming another user synthesizes rows for a named
// Standard User only on a Standard User profile with a compatible stored
// UserType, only for Case and Lead, and only in a declared shape. Each case
// isolates one guard: the other inputs stay qualifying.
func TestUserRecordAccessNamedUserBoundaries(t *testing.T) {
	cases := []struct {
		name        string
		userType    string
		profile     storage.ID
		undeclared  bool
		caseRows    int
		leadRows    int
		accountRows int
	}{
		{name: "standard-type", userType: "Standard", profile: boundaryStandardProfile, caseRows: 1, leadRows: 1},
		{name: "blank-type-standard-profile", profile: boundaryStandardProfile, caseRows: 1, leadRows: 1},
		// The object restriction: a qualifying user gets nothing on Account.
		{name: "undeclared-shape", userType: "Standard", profile: boundaryStandardProfile, undeclared: true},
		// The UserType guard: the profile qualifies, the stored type does not.
		{name: "automated-process-type", userType: "AutomatedProcess", profile: boundaryStandardProfile},
		// The profile guard, resolved from the profile record: a community
		// profile with no stored UserType is not a Standard user.
		{name: "blank-type-community-profile", profile: boundaryCommunityProfile},
		{name: "standard-type-custom-profile", userType: "Standard", profile: boundaryCustomProfile},
		{name: "blank-type-no-profile"},
		// A Guest-type profile is the captured Guest user: Case and Lead only.
		{name: "guest-profile", profile: boundaryGuestProfile, caseRows: 1, leadRows: 1},
	}
	const named = "005000000000017"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(`
Account a = new Account(Name = 'One');
insert a;
Lead l = new Lead(LastName = 'L', Company = 'C');
insert l;
Case c = new Case(Subject = 'S');
insert c;
Integer accountRows = Database.query('SELECT RecordId FROM UserRecordAccess WHERE UserId = \'` + named + `\' AND RecordId = \'' + a.Id + '\'').size();
Integer leadRows = Database.query('SELECT RecordId FROM UserRecordAccess WHERE UserId = \'` + named + `\' AND RecordId = \'' + l.Id + '\'').size();
Integer caseRows = Database.query('SELECT RecordId FROM UserRecordAccess WHERE UserId = \'` + named + `\' AND RecordId = \'' + c.Id + '\'').size();
System.assertEquals(` + itoa(tc.accountRows) + `, accountRows, 'account rows ');
System.assertEquals(` + itoa(tc.leadRows) + `, leadRows, 'lead rows');
System.assertEquals(` + itoa(tc.caseRows) + `, caseRows, 'case rows');
`)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			if tc.undeclared {
				storage.ApplyProjectOrgShape(&org, []string{project.UnspecifiedStandardFieldShape})
			}
			for _, object := range []string{"Account", "Lead", "Case", "UserRecordAccess", "User"} {
				storage.EnsureStandardObject(&org, object)
			}
			boundaryProfiles(&org)
			users := org.Objects["User"]
			if users.Records == nil {
				users.Records = map[storage.ID]storage.Record{}
			}
			users.Records[named] = storage.Record{ID: named, Object: "User", Fields: boundaryUserFields(tc.userType, tc.profile)}
			const actor storage.ID = "005000000000001"
			users.Records[actor] = storage.Record{ID: actor, Object: "User", Fields: boundaryUserFields("Standard", boundaryCustomProfile)}
			org.Objects["User"] = users
			if tc.undeclared {
				if storage.StandardObjectShapeDeclared(org.Objects["Lead"].Definition) {
					t.Fatal("fixture shape is declared")
				}
			} else if !storage.StandardObjectShapeDeclared(org.Objects["Lead"].Definition) || !storage.StandardObjectShapeDeclared(org.Objects["Case"].Definition) {
				t.Fatal("fixture shape is not declared")
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(users.Records[actor])
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func itoa(n int) string { return string(rune(0x30 + n)) }

// The scratch-default Lead edit is a Standard User profile user, in a declared
// shape with no declared model, on a Lead a user owns. Each case changes one
// input from the qualifying fixture.
func TestLeadScratchDefaultEditBoundaries(t *testing.T) {
	const (
		actor storage.ID = "005000000000002"
		other storage.ID = "005000000000001"
		queue storage.ID = "00G000000000001"
	)
	cases := []struct {
		name       string
		userType   string
		profile    storage.ID
		model      string
		undeclared bool
		owner      storage.ID
		want       bool
	}{
		{name: "qualifying", userType: "Standard", profile: boundaryStandardProfile, owner: other, want: true},
		{name: "other-profile", userType: "Standard", profile: boundaryCustomProfile, owner: other},
		{name: "community-profile", userType: "Standard", profile: boundaryCommunityProfile, owner: other},
		{name: "no-profile", userType: "Standard", owner: other},
		{name: "undeclared-shape", userType: "Standard", profile: boundaryStandardProfile, undeclared: true, owner: other},
		{name: "explicit-private-model", userType: "Standard", profile: boundaryStandardProfile, model: "Private", owner: other},
		{name: "explicit-read-model", userType: "Standard", profile: boundaryStandardProfile, model: "Read", owner: other},
		{name: "queue-owned", userType: "Standard", profile: boundaryStandardProfile, owner: queue},
		{name: "other-user-type", userType: "AutomatedProcess", profile: boundaryStandardProfile, owner: other},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				if tc.undeclared {
					storage.ApplyProjectOrgShape(&org, []string{project.UnspecifiedStandardFieldShape})
				}
				storage.EnsureStandardObject(&org, "Lead")
				boundaryProfiles(&org)
				state := org.Objects["Lead"]
				state.Definition.SharingModel = tc.model
				org.Objects["Lead"] = state
				if got := storage.StandardObjectShapeDeclared(state.Definition); got == tc.undeclared {
					t.Fatalf("shape declared=%t, want %t", got, !tc.undeclared)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				machine.SetCurrentUser(storage.Record{ID: actor, Object: "User", Fields: boundaryUserFields(tc.userType, tc.profile)})
				if machine.currentUserBypassesRecordSharing() || machine.currentUserObjectRecordGrant("Lead", "PermissionsViewAllRecords", "PermissionsModifyAllRecords") {
					t.Fatal("fixture actor bypasses record sharing")
				}
				lead := storage.Record{ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: tc.owner, CreatedByID: other}}
				// The edit rule itself, apart from the caller's model switch.
				if got := machine.leadScratchDefaultEdit("Lead", lead); got != tc.want {
					t.Errorf("scratch default edit=%t, want %t", got, tc.want)
				}
				for _, operation := range []string{"update", "upsert"} {
					if got := machine.currentUserCanWriteRecord("Lead", lead, string(actor), operation); got != tc.want {
						t.Errorf("%s allowed=%t, want %t", operation, got, tc.want)
					}
				}
			})
		}
	}
}

// An anonymous execution has no test context. A UserRecordAccess query naming
// a Standard User still returns that user's rows, computed as that user
// (case_phase anonymous rows, APIs 62.0 and 67.0: an explicit false row for a
// Case the reader cannot read), and the actor's own rows are unchanged.
func TestUserRecordAccessNamedUserWithoutTestContext(t *testing.T) {
	const named = "005000000000017"
	program, err := CompileAnonymous(`
Case c = new Case(Subject = 'S');
insert c;
List<UserRecordAccess> other = Database.query('SELECT RecordId, UserId, HasReadAccess FROM UserRecordAccess WHERE UserId = \'` + named + `\' AND RecordId = \'' + c.Id + '\'');
System.assertEquals(1, other.size(), 'named user rows');
System.assertEquals(false, (Boolean) other[0].get('HasReadAccess'), 'named user read');
System.assertEquals('` + named + `', String.valueOf(other[0].get('UserId')).substring(0, 15), 'named user id');
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	for _, object := range []string{"Case", "UserRecordAccess", "User"} {
		storage.EnsureStandardObject(&org, object)
	}
	caseState := org.Objects["Case"]
	caseState.Definition.SharingModel = "Private"
	org.Objects["Case"] = caseState
	boundaryProfiles(&org)
	users := org.Objects["User"]
	if users.Records == nil {
		users.Records = map[storage.ID]storage.Record{}
	}
	users.Records[named] = storage.Record{ID: named, Object: "User", Fields: boundaryUserFields("Standard", boundaryStandardProfile)}
	const actor storage.ID = "005000000000001"
	users.Records[actor] = storage.Record{ID: actor, Object: "User", Fields: boundaryUserFields("Standard", boundaryCustomProfile)}
	org.Objects["User"] = users
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(users.Records[actor])
	if machine.testContext != nil {
		t.Fatal("fixture has a test context")
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if machine.executionUser.Kind == "" || stringField(machine.executionUser, "Id") != string(actor) {
		t.Fatal("the execution user was not restored")
	}
}
