package soql

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// nulls_order_standard_lookups rows (API 62.0 and 67.0): N is the null row, L
// and H point at the lower and higher parent Id. Contact.AccountId ignores the
// NULLS clause; Contact.ReportsToId and a custom lookup honour it.
func TestStandardLookupNullsClauseOrder(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
		"Id":          {APIName: "Id", Type: storage.FieldID},
		"LastName":    {APIName: "LastName", Type: storage.FieldString},
		"AccountId":   {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		"ReportsToId": {APIName: "ReportsToId", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}},
		"Owned__c":    {APIName: "Owned__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
	}}
	state := storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	for i, label := range []string{"N", "L", "H"} {
		id := storage.ID([]string{"003000000000001", "003000000000002", "003000000000003"}[i])
		fields := map[string]storage.Value{"LastName": storage.StringValue(label)}
		if label != "N" {
			parent := map[string]storage.Value{"L": storage.IDValue("001000000000001"), "H": storage.IDValue("001000000000002")}[label]
			contact := map[string]storage.Value{"L": storage.IDValue("003000000000011"), "H": storage.IDValue("003000000000012")}[label]
			fields["AccountId"], fields["Owned__c"], fields["ReportsToId"] = parent, parent, contact
		}
		state.Records[id] = storage.Record{ID: id, Object: "Contact", Fields: fields}
	}
	org.Objects["Contact"] = state
	ignore := map[string]string{"NULLS FIRST": "NLH", "NULLS LAST": "NLH", "ASC": "NLH", "DESC": "HLN", "DESC NULLS LAST": "HLN", "DESC NULLS FIRST": "HLN", "ASC NULLS LAST": "NLH"}
	honour := map[string]string{"NULLS FIRST": "NLH", "NULLS LAST": "LHN", "ASC": "NLH", "DESC": "NHL", "DESC NULLS LAST": "HLN", "DESC NULLS FIRST": "NHL", "ASC NULLS LAST": "LHN"}
	for field, want := range map[string]map[string]string{"AccountId": ignore, "Contact.AccountId": ignore, "ReportsToId": honour, "Owned__c": honour} {
		for tail, order := range want {
			t.Run(field+" "+tail, func(t *testing.T) {
				result, err := ParseAndExecute(org, "SELECT LastName FROM Contact ORDER BY "+field+" "+tail)
				if err != nil {
					t.Fatal(err)
				}
				var got strings.Builder
				for _, row := range result.Records {
					got.WriteString(row.Fields["LastName"].String)
				}
				if got.String() != order {
					t.Fatalf("order=%s; native=%s", got.String(), order)
				}
			})
		}
	}
	if StandardLookupIgnoresNullsClause("Contact", "ReportsToId") || !StandardLookupIgnoresNullsClause("task", "WHATID") {
		t.Fatal("captured nulls-clause classification changed")
	}
}
