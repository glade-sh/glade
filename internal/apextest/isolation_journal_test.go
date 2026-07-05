package apextest

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
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

func TestRunSequentialMethodsIsolatesSetupOrgWithClassJournal(t *testing.T) {
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
	stats := SnapshotPerfCounters()
	if stats.CloneRuntimeOrgCalls != 1 {
		t.Fatalf("cloneRuntimeOrg calls = %d, want setup clone only", stats.CloneRuntimeOrgCalls)
	}
	if stats.CloneFallbacks != 0 {
		t.Fatalf("clone fallbacks = %d, want class journal path", stats.CloneFallbacks)
	}
	if stats.JournalRollbacks != 2 {
		t.Fatalf("journal rollbacks = %d, want one per method", stats.JournalRollbacks)
	}
}

func TestRunNoSetupSingleMethodClassesUseSharedJournal(t *testing.T) {
	ResetPerfCounters()
	storage.ResetCloneStats()
	t.Cleanup(ResetPerfCounters)
	t.Cleanup(storage.ResetCloneStats)

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		writeFile(t, filepath.Join(root, "force-app/main/classes/JournalNoSetup"+name+"Test.cls"), `
@isTest
private class JournalNoSetup`+name+`Test {
  @isTest static void mutatesOrg() {
    System.assertEquals(0, [SELECT count() FROM Account]);
    insert new Account(Name = '`+name+`');
  }
}
`)
	}

	run := Run(loadTestIndex(t, root), Options{Parallelism: 1})
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
	stats := SnapshotPerfCounters()
	if stats.CloneRuntimeOrgCalls != 0 {
		t.Fatalf("cloneRuntimeOrg calls = %d, want shared no-setup journal path", stats.CloneRuntimeOrgCalls)
	}
	if stats.JournalRollbacks != 3 {
		t.Fatalf("journal rollbacks = %d, want one per method", stats.JournalRollbacks)
	}
}

func TestRunNoSetupJournalKeepsMethodsInSameClassSerial(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JournalNoSetupSerialTest.cls"), `
@isTest
private class JournalNoSetupSerialTest {
  private static void burn() {
    Integer total = 0;
    for (Integer i = 0; i < 50000; i++) {
      total += i;
    }
    System.assert(total > 0);
  }
  @isTest static void one() { burn(); }
  @isTest static void two() { burn(); }
  @isTest static void three() { burn(); }
  @isTest static void four() { burn(); }
}
`)

	var mu sync.Mutex
	activeByClass := map[string]int{}
	overlapped := false
	run := Run(loadTestIndex(t, root), Options{
		Parallelism: 4,
		Progress: func(progress TestProgress) {
			switch progress.Event {
			case "test_start":
				mu.Lock()
				activeByClass[progress.ClassName]++
				if activeByClass[progress.ClassName] > 1 {
					overlapped = true
				}
				mu.Unlock()
			case "test_done":
				mu.Lock()
				activeByClass[progress.ClassName]--
				mu.Unlock()
			}
		},
	})
	if got := run.Summary(); got.Total != 4 || got.Passed != 4 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
	if overlapped {
		t.Fatal("no-setup journal path ran methods from the same class concurrently")
	}
}

func TestJournalIsolationProbeRejectsTestSetMock(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "MockedCalloutTest.cls")
	writeFile(t, file, `
@isTest
private class MockedCalloutTest {
  @isTest static void usesMock() {
    System.Test.setMock(HttpCalloutMock.class, new Mock());
  }
}
`)

	if newClassIsolationProbeCache(nil).fileSupportsJournalIsolation(file) {
		t.Fatal("Test.setMock class should use full clone isolation")
	}
}

func TestRunNoSetupTriggerSuitesUseFullCloneIsolation(t *testing.T) {
	ResetPerfCounters()
	storage.ResetCloneStats()
	t.Cleanup(ResetPerfCounters)
	t.Cleanup(storage.ResetCloneStats)

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/triggers/AccountBeforeInsert.trigger"), `
trigger AccountBeforeInsert on Account (before insert) {
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JournalNoSetupTriggerTest.cls"), `
@isTest
private class JournalNoSetupTriggerTest {
  @isTest static void one() {
    insert new Account(Name = 'One');
    System.assertEquals(1, [SELECT count() FROM Account]);
  }
  @isTest static void two() {
    insert new Account(Name = 'Two');
    System.assertEquals(1, [SELECT count() FROM Account]);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{Parallelism: 1})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
	stats := SnapshotPerfCounters()
	if stats.CloneRuntimeOrgCalls == 0 {
		t.Fatal("trigger suite used shared no-setup journal path, want full clone isolation")
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

func TestJournalRollbackCoversSetCreatedDateOnSetupRecord(t *testing.T) {
	ResetPerfCounters()
	storage.ResetCloneStats()
	t.Cleanup(ResetPerfCounters)
	t.Cleanup(storage.ResetCloneStats)

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JournalSetupCreatedDateTest.cls"), `
@isTest
private class JournalSetupCreatedDateTest {
  @TestSetup static void setup() {
    insert new Account(Name = 'Fixture');
  }
  @isTest static void firstMethodChangesSetupCreatedDate() {
    Account row = [SELECT Id FROM Account WHERE Name = 'Fixture' LIMIT 1];
    Test.setCreatedDate(row.Id, Datetime.newInstanceGmt(2026, 1, 2, 3, 4, 5));
  }
  @isTest static void secondMethodSeesOriginalSetupRecord() {
    Account row = [SELECT Id, CreatedDate FROM Account WHERE Name = 'Fixture' LIMIT 1];
    System.assertNotEquals(Datetime.newInstanceGmt(2026, 1, 2, 3, 4, 5), row.CreatedDate);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
	stats := SnapshotPerfCounters()
	if stats.CloneFallbacks != 0 {
		t.Fatalf("clone fallbacks = %d, want class journal path", stats.CloneFallbacks)
	}
	if stats.JournalRollbacks != 2 {
		t.Fatalf("journal rollbacks = %d, want one per method", stats.JournalRollbacks)
	}
}

func TestJournalRollbackCoversRunAsMaterializedUser(t *testing.T) {
	ResetPerfCounters()
	storage.ResetCloneStats()
	t.Cleanup(ResetPerfCounters)
	t.Cleanup(storage.ResetCloneStats)

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JournalRunAsUserTest.cls"), `
@isTest
private class JournalRunAsUserTest {
  static Id runAsId = '00500000000JRN1';
  @isTest static void firstMethodMaterializesRunAsUser() {
    Profile p = [SELECT Id FROM Profile WHERE Name = 'System Administrator' LIMIT 1];
    System.runAs(new User(Id = runAsId, ProfileId = p.Id, Username = 'journal-runas@example.test')) {
      System.assertEquals(runAsId, UserInfo.getUserId());
      System.assertEquals(1, [
        SELECT count()
        FROM PermissionSetAssignment
        WHERE AssigneeId = :runAsId
      ]);
    }
  }
  @isTest static void secondMethodDoesNotSeeRunAsUser() {
    System.assertEquals(0, [SELECT count() FROM User WHERE Id = :runAsId]);
    System.assertEquals(0, [
      SELECT count()
      FROM PermissionSetAssignment
      WHERE AssigneeId = :runAsId
    ]);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
	stats := SnapshotPerfCounters()
	if stats.CloneFallbacks != 0 {
		t.Fatalf("clone fallbacks = %d, want class journal path", stats.CloneFallbacks)
	}
	if stats.JournalRollbacks != 2 {
		t.Fatalf("journal rollbacks = %d, want one per method", stats.JournalRollbacks)
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
