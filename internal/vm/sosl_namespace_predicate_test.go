package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The Apex Developer Guide's Namespace Prefix section permits local schema
// names in SOQL/SOSL within their owning namespace. Its SOSL testing section
// applies RETURNING WHERE filters to Test.setFixedSearchResults candidates.
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_classes_namespace_prefix.htm
func TestSOSLReturningWhereResolvesOwnedLookupNames(t *testing.T) {
	for _, tc := range []struct {
		name, predicate, expected string
	}{
		{"local", "ContactLink__c != null", "Alpha"},
		{"qualified", "parcel__ContactLink__c != null", "Alpha"},
		{"case", "contactlink__c != null", "Alpha"},
		{"null", "ContactLink__c = null", "Beta"},
		{"foreign", "other__ContactLink__c != null", "Beta"},
		{"and", "ContactLink__c != null AND Name = 'Alpha'", "Alpha"},
		{"or", "ContactLink__c != null OR Name = 'Alpha'", "Alpha"},
		{"not", "NOT (ContactLink__c = null)", "Alpha"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Namespace = "parcel"
			storage.EnsureStandardObject(&org, "Account")
			state := org.Objects["Account"]
			for _, field := range []string{"parcel__ContactLink__c", "other__ContactLink__c"} {
				state.Definition.Fields[field] = storage.Field{APIName: field, Type: storage.FieldReference, ReferenceTo: []string{"Contact"}}
			}
			for i, name := range []string{"Alpha", "Beta"} {
				id := storage.ID(fmt.Sprintf("001%012d", i+1))
				fields := map[string]storage.Value{"Name": storage.StringValue(name)}
				field := "parcel__ContactLink__c"
				if i == 1 {
					field = "other__ContactLink__c"
				}
				fields[field] = storage.IDValue("003000000000001")
				state.Records[id] = storage.Record{Object: "Account", ID: id, Fields: fields}
			}
			org.Objects["Account"] = state
			predicate := tc.predicate
			query := "FIND {candidate} IN NAME FIELDS RETURNING Account(Name WHERE " + predicate + " ORDER BY Name)"
			program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Test.setFixedSearchResults(new List<Id>{'001000000000001', '001000000000002'});
List<Account> rows = (List<Account>)Search.query('%s')[0];
System.assertEquals(1, rows.size());
System.assertEquals('%s', rows[0].Name);
String term = 'candidate';
List<List<SObject>> inlineRows = [FIND :term IN NAME FIELDS RETURNING Account(Name WHERE %s ORDER BY Name)];
System.assertEquals(1, inlineRows[0].size());
System.assertEquals('%s', inlineRows[0][0].get('Name'));
`, strings.ReplaceAll(query, "'", "\\'"), tc.expected, predicate, tc.expected), CompileOptions{APIVersion: "46.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
