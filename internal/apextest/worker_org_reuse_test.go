package apextest

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Reuse is an allocation change on the existing journal route; the existing
// isolation_journal_test.go controls pin insert/update/delete/sequence reset.
func TestMethodWorkerOrgReuseAcrossClasses(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			for _, name := range []string{"Cedar", "Maple", "Pine"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/"+name+"Test.cls"), `
@isTest private class `+name+`Test {
 @isTest static void first() {
  System.assertEquals(0, [SELECT COUNT() FROM Account]);
  insert new Account(Name='first');
  System.assertEquals(1, [SELECT COUNT() FROM Account]);
 }
 @isTest static void second() {
  System.assertEquals(0, [SELECT COUNT() FROM Account]);
  insert new Account(Name='second');
  System.assertEquals(1, [SELECT COUNT() FROM Account]);
 }
}`)
			}
			// An actual setup routes the run through runTestPlansWithSetups,
			// while the three no-setup classes can reuse one journal worker.
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/FixtureTest.cls"), `
@isTest private class FixtureTest {
 @TestSetup static void seed() { insert new Account(Name='fixture'); }
 @isTest static void first() { System.assertEquals(1, [SELECT COUNT() FROM Account]); }
 @isTest static void second() { System.assertEquals('fixture', [SELECT Name FROM Account LIMIT 1].Name); }
}`)
			run := Run(loadTestIndex(t, root), Options{Parallelism: 1, ParallelMethods: true, PerfCounters: true})
			if summary := run.Summary(); summary.Total != 8 || summary.Passed != 8 {
				t.Fatalf("summary=%+v suites=%+v", summary, run.Suites)
			}
			stats := SnapshotPerfCounters()
			// One shared-seed worker, one setup clone and its method worker.
			if stats.CloneRuntimeOrgCalls > 3 {
				t.Fatalf("org clones=%d, want at most 3", stats.CloneRuntimeOrgCalls)
			}
			if stats.JournalRollbacks != 8 {
				t.Fatalf("rollbacks=%d, want 8", stats.JournalRollbacks)
			}
		})
	}
}

func TestMethodWorkerOrgPoolConcurrentReuse(t *testing.T) {
	seed := storage.NewOrgState()
	storage.EnsureStandardObject(&seed, "Account")
	pool := &testWorkerOrgPool{seed: seed, capacity: 4}
	for range 2 {
		ready := make(chan struct{}, 8)
		release := make(chan struct{})
		errors := make(chan string, 8)
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				worker := pool.take("Fixture", nil)
				defer pool.put(worker)
				state := worker.org.Objects["Account"]
				if len(state.Records) != 0 {
					errors <- "worker observed another worker's record"
				}
				id := storage.ID("001000000000001")
				worker.journal.RecordInsert("Account", id)
				state.Records[id] = storage.Record{ID: id, Object: "Account"}
				worker.org.Objects["Account"] = state
				ready <- struct{}{}
				<-release
			}()
		}
		for range 8 {
			<-ready
		}
		close(release)
		wg.Wait()
		close(errors)
		for message := range errors {
			t.Error(message)
		}
		if len(pool.idle) != 4 || len(seed.Objects["Account"].Records) != 0 {
			t.Fatal("pool retained excess workers or changed the seed")
		}
	}
}
