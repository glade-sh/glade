package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecSOSLFieldScopesUseLocalFieldCategories(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Contact(LastName = 'Needle Name', Email = 'plain@example.test', Phone = '555-0100', Description = 'plain body');
insert new Contact(LastName = 'Plain Name', Email = 'needle@example.test', Phone = '555-0200', Description = 'plain body');
insert new Contact(LastName = 'Plain Other', Email = 'plain2@example.test', Phone = '555-NEEDLE', Description = 'plain body');
insert new Contact(LastName = 'Plain Body', Email = 'plain3@example.test', Phone = '555-0300', Description = 'needle body');

List<List<SObject>> nameRows = Search.query('FIND {Needle} IN NAME FIELDS RETURNING Contact(Id, LastName ORDER BY LastName)');
System.assertEquals(1, nameRows[0].size());
System.assertEquals('Needle Name', ((Contact)nameRows[0][0]).LastName);

List<List<SObject>> emailRows = Search.query('FIND {needle} IN EMAIL FIELDS RETURNING Contact(Id, Email ORDER BY Email)');
System.assertEquals(1, emailRows[0].size());
System.assertEquals('needle@example.test', ((Contact)emailRows[0][0]).Email);

List<List<SObject>> phoneRows = Search.query('FIND {NEEDLE} IN PHONE FIELDS RETURNING Contact(Id, Phone ORDER BY Phone)');
System.assertEquals(1, phoneRows[0].size());
System.assertEquals('555-NEEDLE', ((Contact)phoneRows[0][0]).Phone);

List<List<SObject>> allRows = Search.query('FIND {needle} IN ALL FIELDS RETURNING Contact(Id, LastName ORDER BY LastName LIMIT 4)');
System.assertEquals(4, allRows[0].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	contact := org.Objects["Contact"]
	contact.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
	org.Objects["Contact"] = contact
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOSLUnaliasedToLabelProjectsSourceField(t *testing.T) {
	program, err := CompileAnonymous(`
Account row = new Account(Name = 'Label row', Type = 'Prospect');
insert row;
Test.setFixedSearchResults(new List<Id>{row.Id});
List<List<SObject>> results = Search.query('FIND {Label row} RETURNING Account(Id, toLabel(Type))');
System.assertEquals('Prospect', results[0][0].get('Type'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOSLBareWildcardSearchOperand(t *testing.T) {
	program, err := CompileAnonymous(`
Contact row = new Contact(LastName = 'Unrelated');
insert row;
Test.setFixedSearchResults(new List<Id>{row.Id});
String searchTerm = '* @group';
List<List<SObject>> rows = [FIND :searchTerm IN ALL FIELDS RETURNING Contact(Id, LastName)];
System.assertEquals(1, rows.size());
System.assertEquals(1, rows[0].size());
System.assertEquals(row.Id, rows[0][0].Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOSLHostedSearchServicesStayUnsupported(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	org.Objects["Remote_Record__x"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Remote_Record__x",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	// A35 L016: this clause order is rejected before the hosted boundary.
	_, err := machine.searchQuery([]Value{String("FIND {Needle} IN ALL FIELDS WITH DATA CATEGORY Products ABOVE Hardware RETURNING Contact(Id)")})
	if err == nil || !strings.Contains(err.Error(), "unexpected token: RETURNING") {
		t.Fatalf("native clause-order rejection: %v", err)
	}
	assertSOSLUnsupported(t, machine,
		`Search.query('FIND {Needle} IN ALL FIELDS RETURNING Remote_Record__x(Id, Name)');`,
		"SOSL external indexes")
}

func TestExecSOSLUnimplementedOptionsFailClosed(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	// R138 disables correction; R151 rejects a numeric division value.
	if _, err := machine.searchQuery([]Value{String("FIND {Needle} IN ALL FIELDS RETURNING Contact(Id) WITH SPELL_CORRECTION = false")}); err != nil {
		t.Fatal(err)
	}
	_, err := machine.searchQuery([]Value{String("FIND {Needle} IN ALL FIELDS RETURNING Contact(Id) WITH DIVISION = 0")})
	if err == nil || !strings.Contains(err.Error(), "unexpected token: '0'") {
		t.Fatalf("native numeric-division rejection: %v", err)
	}
}

func TestExecSearchFindAppliesGlobalLimit(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Nook One');
insert new Account(Name = 'Nook Two');
insert new Contact(LastName = 'Nook Contact');
Search.SearchResults results = Search.find('FIND {Nook*} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id, LastName) LIMIT 1');
System.assertEquals(1, results.get('Account').size());
System.assertEquals(0, results.get('Contact').size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func assertSOSLUnsupported(t *testing.T, machine *VM, source, want string) {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(runtimeErr.Message, want) {
		t.Fatalf("err = %#v, want UnsupportedFeature containing %q", err, want)
	}
}

func TestExecSOQLQualifiedCurrentObjectIdIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
User userRecord = [SELECT Id, LastName FROM User LIMIT 1];
String query = 'SELECT Id FROM User WHERE user.id = \'' + userRecord.Id + '\'';
List<User> rows = Database.query(query);
System.assertEquals(1, rows.size());
System.assertEquals(userRecord.Id, rows[0].Id);
String projectionQuery = 'SELECT user.lastname FROM User WHERE Id = \'' + userRecord.Id + '\'';
List<User> projected = Database.query(projectionQuery);
System.assertEquals(1, projected.size());
System.assertEquals(userRecord.LastName, projected[0].LastName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
