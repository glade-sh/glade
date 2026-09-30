package soql

import (
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Mirrors the four-set query matrix in the owned API 53 Salesforce proof.
// Apex SObject-list bind expansion is tested by the integrated fixture, not here.
func TestDuplicateRecordPolymorphicTypeQueries(t *testing.T) {
	org := storage.NewOrgState()
	for _, name := range []string{"Account", "Contact", "Lead", "DuplicateRecordSet", "DuplicateRecordItem"} {
		storage.EnsureStandardObject(&org, name)
	}
	put := func(object string, id storage.ID, fields map[string]storage.Value) {
		state := org.Objects[object]
		state.Records[id] = storage.Record{ID: id, Object: object, Fields: fields}
		org.Objects[object] = state
	}
	account := storage.ID("001000000000001")
	put("Account", account, map[string]storage.Value{"Type": storage.StringValue("Prospect")})
	contacts := []storage.ID{}
	leads := []storage.ID{}
	sets := []storage.ID{}
	for i := 1; i <= 5; i++ {
		id := storage.ID(fmt.Sprintf("003%012d", i))
		contacts = append(contacts, id)
		put("Contact", id, map[string]storage.Value{"AccountId": storage.IDValue(account)})
	}
	for i := 1; i <= 4; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i))
		leads = append(leads, id)
		put("Lead", id, nil)
	}
	groups := [][]storage.ID{{contacts[0], contacts[1]}, {contacts[2], contacts[3], leads[0]}, {contacts[4], leads[1]}, {leads[2], leads[3]}}
	for i, group := range groups {
		id := storage.ID(fmt.Sprintf("0Ff%012d", i+1))
		sets = append(sets, id)
		put("DuplicateRecordSet", id, map[string]storage.Value{"RecordCount": storage.IntegerValue(int64(len(group)))})
		for j, recordID := range group {
			put("DuplicateRecordItem", storage.ID(fmt.Sprintf("0Fh%012d", i*10+j+1)), map[string]storage.Value{"DuplicateRecordSetId": storage.IDValue(id), "RecordId": storage.IDValue(recordID)})
		}
	}
	run := func(t *testing.T, query string) Result {
		t.Helper()
		result, err := ParseAndExecute(org, query)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	t.Run("typeProjectionAndFilters", func(t *testing.T) {
		for _, tc := range []struct {
			predicate, kind string
			want            int
		}{{"=", "Contact", 5}, {"!=", "Lead", 4}} {
			result := run(t, "SELECT RecordId,Record.Type FROM DuplicateRecordItem WHERE Record.Type"+tc.predicate+"'Contact'")
			if result.Rows != tc.want {
				t.Fatalf("%s rows=%d; want %d", tc.predicate, result.Rows, tc.want)
			}
			for _, row := range result.Records {
				if row.Fields["Record.Type"].String != tc.kind {
					t.Fatalf("Record.Type=%#v", row.Fields["Record.Type"])
				}
			}
		}
	})
	mixed := "SELECT Id FROM DuplicateRecordSet WHERE Id IN (SELECT DuplicateRecordSetId FROM DuplicateRecordItem WHERE Record.Type='Contact') AND Id IN (SELECT DuplicateRecordSetId FROM DuplicateRecordItem WHERE Record.Type!='Contact')"
	t.Run("mixedSemijoins", func(t *testing.T) {
		result := run(t, mixed)
		if result.Rows != 2 {
			t.Fatalf("mixed rows=%d", result.Rows)
		}
		for _, row := range result.Records {
			if !storage.IDsEqual(row.ID, sets[1]) && !storage.IDsEqual(row.ID, sets[2]) {
				t.Fatalf("unexpected mixed set %s", row.ID)
			}
		}
	})
	t.Run("groupedContactCounts", func(t *testing.T) {
		result := run(t, fmt.Sprintf("SELECT DuplicateRecordSetId DRSId,COUNT(Id) ContactTotal FROM DuplicateRecordItem WHERE Record.Type='Contact' AND DuplicateRecordSetId IN ('%s','%s') GROUP BY DuplicateRecordSetId,DuplicateRecordSet.RecordCount", sets[1], sets[2]))
		if result.Rows != 2 {
			t.Fatalf("aggregate rows=%d", result.Rows)
		}
		for _, row := range result.Records {
			want := int64(1)
			if storage.IDsEqual(row.Fields["DRSId"].ID, sets[1]) {
				want = 2
			}
			if row.Fields["ContactTotal"].Integer != want {
				t.Fatalf("aggregate=%#v", row.Fields)
			}
		}
	})
	t.Run("parentChildren", func(t *testing.T) {
		result := run(t, fmt.Sprintf("SELECT Id,RecordCount,(SELECT RecordId FROM DuplicateRecordItems) FROM DuplicateRecordSet WHERE RecordCount>1 AND Id IN (SELECT DuplicateRecordSetId FROM DuplicateRecordItem WHERE Record.Type='Contact') AND Id NOT IN ('%s')", sets[2]))
		if result.Rows != 2 {
			t.Fatalf("parent rows=%d", result.Rows)
		}
		for _, row := range result.Records {
			children := row.Children["DuplicateRecordItems"]
			if int64(len(children)) != row.Fields["RecordCount"].Integer {
				t.Fatalf("parent/children=%#v", row.Fields)
			}
			index := 0
			if storage.IDsEqual(row.ID, sets[1]) {
				index = 1
			} else if !storage.IDsEqual(row.ID, sets[0]) {
				t.Fatalf("unexpected parent %s", row.ID)
			}
			expected := map[storage.ID]bool{}
			for _, id := range groups[index] {
				expected[id] = true
			}
			for _, child := range children {
				id := child.Fields["RecordId"].ID
				if !expected[id] {
					t.Fatalf("unexpected child RecordId %s", id)
				}
				delete(expected, id)
			}
			if len(expected) != 0 {
				t.Fatalf("missing children %v", expected)
			}
		}
	})
	t.Run("ordinaryAccountType", func(t *testing.T) {
		result := run(t, "SELECT Account.Type FROM Contact WHERE Account.Type='Prospect'")
		if result.Rows != 5 {
			t.Fatalf("ordinary Type rows=%d", result.Rows)
		}
		for _, row := range result.Records {
			if row.Fields["Account.Type"].String != "Prospect" {
				t.Fatalf("ordinary Type=%#v", row.Fields)
			}
		}
	})
}
