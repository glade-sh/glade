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
	assertSOSLUnsupported(t, machine,
		`Search.query('FIND {Needle} IN ALL FIELDS WITH DATA CATEGORY Products ABOVE Hardware RETURNING Contact(Id)');`,
		"SOSL WITH DATA CATEGORY")
	assertSOSLUnsupported(t, machine,
		`Search.query('FIND {Needle} IN ALL FIELDS RETURNING Remote_Record__x(Id, Name)');`,
		"SOSL external indexes")
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
