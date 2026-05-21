package apextest

import (
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/dml"
	"github.com/open-aer/oaer/internal/storage"
)

func TestIsolationJournalRestoresInsertUpdateDeleteAndSequences(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{"Name": storage.StringValue("Base")},
			},
			"001000000000002": {
				ID:     "001000000000002",
				Object: "Account",
				Fields: map[string]storage.Value{"Name": storage.StringValue("DeleteMe")},
			},
		},
	}
	org.IDSequences["Account"] = 2
	journal := storage.NewIsolationJournal(&org)
	mark := journal.Mark()
	engine := dml.NewEngine(&org)
	engine.IsolationJournal = journal

	inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Inserted")}}})
	if !inserted[0].Success {
		t.Fatalf("insert = %#v", inserted)
	}
	updated := engine.Update([]storage.Record{{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("Changed")}}})
	if !updated[0].Success {
		t.Fatalf("update = %#v", updated)
	}
	updatedInserted := engine.Update([]storage.Record{{Object: "Account", ID: inserted[0].ID, Fields: map[string]storage.Value{"Name": storage.StringValue("Inserted changed")}}})
	if !updatedInserted[0].Success {
		t.Fatalf("update inserted = %#v", updatedInserted)
	}
	deleted := engine.Delete([]storage.Record{{Object: "Account", ID: "001000000000002"}})
	if !deleted[0].Success {
		t.Fatalf("delete = %#v", deleted)
	}
	if err := journal.Rollback(mark); err != nil {
		t.Fatal(err)
	}

	account := org.Objects["Account"]
	if _, ok := account.Records[inserted[0].ID]; ok {
		t.Fatalf("inserted record survived rollback")
	}
	if got := account.Records["001000000000001"].Fields["Name"].String; got != "Base" {
		t.Fatalf("updated record name = %q, want Base", got)
	}
	if account.Records["001000000000002"].System.IsDeleted {
		t.Fatalf("deleted record stayed deleted")
	}
	if got := org.IDSequences["Account"]; got != 2 {
		t.Fatalf("sequence = %d, want 2", got)
	}
}

func TestRunSequentialMethodsIsolatesSetupOrgWithClones(t *testing.T) {
	ResetPerfCounters()
	storage.ResetCloneStats()
	t.Cleanup(ResetPerfCounters)
	t.Cleanup(storage.ResetCloneStats)

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JournalReuseTest.cls"), `
@isTest
private class JournalReuseTest {
  @TestSetup static void setup() {
    insert new Account(Name = 'Fixture');
  }
  @isTest static void firstMethod() {
    Account row = [SELECT Id, Name FROM Account LIMIT 1];
    row.Name = 'Changed';
    update row;
    insert new Account(Name = 'First');
  }
  @isTest static void secondMethod() {
    System.assertEquals(1, [SELECT count() FROM Account]);
    Account row = [SELECT Id, Name FROM Account LIMIT 1];
    System.assertEquals('Fixture', row.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
	if got := SnapshotPerfCounters().CloneRuntimeOrgCalls; got != 3 {
		t.Fatalf("cloneRuntimeOrg calls = %d, want setup plus method clones", got)
	}
}

func TestJournalRollbackCoversSetCreatedDateAfterInsert(t *testing.T) {
	ResetPerfCounters()
	storage.ResetCloneStats()
	t.Cleanup(ResetPerfCounters)
	t.Cleanup(storage.ResetCloneStats)

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JournalSetCreatedDateTest.cls"), `
@isTest
private class JournalSetCreatedDateTest {
  @TestSetup static void setup() {
    insert new Account(Name = 'Fixture');
  }
  @isTest static void firstMethod() {
    Contact row = new Contact(LastName = 'Inserted');
    insert row;
    Test.setCreatedDate(row.Id, Datetime.now());
  }
  @isTest static void secondMethod() {
    Contact row = new Contact(LastName = 'Inserted');
    insert row;
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestJournalRollbackCoversInsertAfterAllOrNoneSnapshot(t *testing.T) {
	ResetPerfCounters()
	storage.ResetCloneStats()
	t.Cleanup(ResetPerfCounters)
	t.Cleanup(storage.ResetCloneStats)

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JournalAllOrNoneSnapshotTest.cls"), `
@isTest
private class JournalAllOrNoneSnapshotTest {
  @TestSetup static void setup() {
    insert new Account(Name = 'Fixture');
  }
  @isTest static void firstMethod() {
    Contact row = new Contact(LastName = 'Inserted');
    insert row;
    try {
      Database.insert(new List<Account>{ new Account(), new Account(Name = 'Good') }, true);
    } catch (Exception e) {
    }
  }
  @isTest static void secondMethod() {
    Contact row = new Contact(LastName = 'Inserted');
    insert row;
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}
