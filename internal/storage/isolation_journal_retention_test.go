package storage

import "testing"

func TestIsolationJournalRollbackReleasesBeforeImages(t *testing.T) {
	for _, nested := range []bool{false, true} {
		org := NewOrgState()
		EnsureStandardObject(&org, "Account")
		table := org.Objects["Account"]
		table.Records["001000000000001"] = Record{ID: "001000000000001", Object: "Account", Fields: map[string]Value{"Name": StringValue("outer")}}
		table.Records["001000000000002"] = Record{ID: "001000000000002", Object: "Account", Fields: map[string]Value{"Name": StringValue("inner")}}
		org.Objects["Account"] = table
		journal := NewIsolationJournal(&org)
		var mark IsolationMark
		if nested {
			journal.RecordUpdate("Account", "001000000000001", table.Records["001000000000001"])
			mark = journal.Mark()
		}
		journal.RecordUpdate("Account", "001000000000002", table.Records["001000000000002"])
		journal.RecordInsert("Account", "001000000000003")
		journal.RecordSequence("Account")
		if err := journal.Rollback(mark); err != nil {
			t.Fatal(err)
		}
		// Inspect the retained capacity: truncated entries must release records,
		// object names and IDs while an outer rollback point stays intact.
		for _, before := range journal.updated[len(journal.updated):cap(journal.updated)] {
			if before.record.Fields != nil || before.object != "" || before.id != "" {
				t.Fatal("rollback retained a before-image")
			}
		}
		for _, key := range journal.inserted[len(journal.inserted):cap(journal.inserted)] {
			if key.object != "" || key.id != "" {
				t.Fatal("rollback retained an inserted record key")
			}
		}
		for _, before := range journal.sequences[len(journal.sequences):cap(journal.sequences)] {
			if before.object != "" {
				t.Fatal("rollback retained a sequence key")
			}
		}
		if nested && (len(journal.updated) != 1 || journal.updated[0].record.Fields["Name"].String != "outer") {
			t.Fatal("rollback cleared an outer before-image")
		}
	}
}
