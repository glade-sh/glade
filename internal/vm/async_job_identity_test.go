package vm

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Every caller of nextAsyncJobID is covered, including the test-only routes.
func TestAsyncJobIdentitySharedOrgRoutes(t *testing.T) {
	routes := []struct {
		name, source        string
		testOnly, scheduled bool
	}{
		{"System.schedule", "System.debug(System.schedule('%s', '0 0 0 1 1 ? 2099', new CedarWorker()));", false, true},
		{"System.scheduleBatch", "System.debug(System.scheduleBatch(new CedarWorker(), '%s', 60));", false, true},
		{"System.enqueueJob", "System.debug(System.enqueueJob(new CedarWorker()));", false, false},
		{"Database.executeBatch", "System.debug(Database.executeBatch(new CedarWorker()));", false, false},
		{"future", "CedarWorker.later();", true, false},
		{"Test.enqueueBatchJobs", "System.debug(Test.enqueueBatchJobs(1).get(0));", true, false},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, mode := range []string{"anonymous", "istest"} {
			for _, route := range routes {
				if route.testOnly && mode == "anonymous" {
					continue
				}
				for _, clone := range []bool{false, true} {
					t.Run(fmt.Sprintf("api%s/%s/%s/clone=%v", api, mode, route.name, clone), func(t *testing.T) {
						org := storage.NewOrgState()
						base := New(nil)
						if err := base.RegisterClass(Class{Name: "CedarWorker", Interfaces: []string{"Schedulable", "Queueable", "Database.Batchable<SObject>"}}); err != nil {
							t.Fatal(err)
						}
						if err := base.RegisterMethod(Method{Name: "CedarWorker.later", ClassName: "CedarWorker", IsStatic: true, Modifiers: []string{"future"}}); err != nil {
							t.Fatal(err)
						}
						var firstID string
						var firstRecords map[storage.ID]storage.Record
						for i, name := range []string{"cedar first transaction", "cedar second transaction"} {
							machine := base.CloneRuntime(nil)
							if !clone {
								machine = New(nil)
								if err := machine.RegisterClass(Class{Name: "CedarWorker", Interfaces: []string{"Schedulable", "Queueable", "Database.Batchable<SObject>"}}); err != nil {
									t.Fatal(err)
								}
								if err := machine.RegisterMethod(Method{Name: "CedarWorker.later", ClassName: "CedarWorker", IsStatic: true, Modifiers: []string{"future"}}); err != nil {
									t.Fatal(err)
								}
							}
							machine.SetOrg(&org)
							if mode == "istest" {
								machine.EnableTestContext()
							}
							source := route.source
							if route.scheduled {
								source = fmt.Sprintf(source, name)
							}
							program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := machine.Execute(program)
							if err != nil {
								t.Fatal(err)
							}
							id := ""
							if route.name == "future" {
								id = machine.testContext.AsyncJobs[0].ID
							} else {
								if len(result.Debug) != 1 {
									t.Fatalf("missing ID: %v", result.Debug)
								}
								id = result.Debug[0]
							}
							if i == 0 {
								firstID = id
								if route.scheduled {
									firstRecords = org.Clone().Objects["CronJobDetail"].Records
								}
							} else {
								if asyncJobIDTextEqual(id, firstID) {
									t.Errorf("fresh VM reused job identity %s", id)
								}
								if route.scheduled {
									detailID := storage.ID(cronJobDetailID("707" + id[3:15]))
									if got := org.Objects["CronJobDetail"].Records[detailID].Fields["Name"].String; got != name {
										t.Errorf("detail name = %q, want %q", got, name)
									}
									for id, before := range firstRecords {
										if !reflect.DeepEqual(org.Objects["CronJobDetail"].Records[id], before) {
											t.Errorf("first transaction detail %s changed", id)
										}
									}
								}
							}
						}
					})
				}
			}
		}
	}
}

func TestAsyncJobIdentityRetainedRowsRoutes(t *testing.T) {
	for _, object := range []string{"AsyncApexJob", "CronTrigger", "CronJobDetail"} {
		for _, deleted := range []bool{false, true} {
			for _, id18 := range []bool{false, true} {
				for _, testContext := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/deleted=%v/id18=%v/istest=%v", object, deleted, id18, testContext), func(t *testing.T) {
						org := storage.NewOrgState()
						id := "707000000000001"
						if object == "CronTrigger" {
							id = cronTriggerID(id)
						}
						if object == "CronJobDetail" {
							id = cronJobDetailID(id)
						}
						if id18 {
							id = apexIDTo18(id)
						}
						record := storage.Record{ID: storage.ID(id), Object: object}
						record.System.IsDeleted = deleted
						org.Objects[object] = storage.ObjectState{Records: map[storage.ID]storage.Record{record.ID: record}}
						machine := New(nil)
						machine.SetOrg(&org)
						if testContext {
							machine.EnableTestContext()
						}
						if got := machine.nextAsyncJobID(); got == "707000000000001" {
							t.Fatalf("reused identity reserved by %s", object)
						}
					})
				}
			}
		}
	}
}

func TestAsyncJobIdentityAnonymousToTestContextRoute(t *testing.T) {
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	first := machine.nextAsyncJobID()
	machine.EnableTestContext()
	if second := machine.nextAsyncJobID(); first == second {
		t.Fatal("test context reused anonymous allocation")
	}
}

func TestAsyncJobIdentitySequenceJournalRollbackRoute(t *testing.T) {
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	journal := storage.NewIsolationJournal(&org)
	machine.SetIsolationJournal(journal)
	mark := journal.Mark()
	first := machine.nextAsyncJobID()
	next := New(nil)
	next.SetOrg(&org)
	if second := next.nextAsyncJobID(); first == second {
		t.Error("fresh VM reused an allocation before any job was recorded")
	}
	if err := journal.Rollback(mark); err != nil {
		t.Fatal(err)
	}
	if org.IDSequences["AsyncApexJob"] != 0 {
		t.Fatal("journal did not restore async ID sequence")
	}
}

func TestAsyncJobIdentityClonedOrgAndUnstoredRoutes(t *testing.T) {
	for _, testContext := range []bool{false, true} {
		t.Run(fmt.Sprintf("istest=%v", testContext), func(t *testing.T) {
			machine := New(nil)
			if testContext {
				machine.EnableTestContext()
			}
			if first, second := machine.nextAsyncJobID(), machine.nextAsyncJobID(); first == second {
				t.Fatal("VM without org reused an identity")
			}
			org := storage.NewOrgState()
			machine.SetOrg(&org)
			first := machine.nextAsyncJobID()
			snapshot := org.Clone()
			for _, state := range []*storage.OrgState{&org, &snapshot} {
				fresh := New(nil)
				fresh.SetOrg(state)
				if testContext {
					fresh.EnableTestContext()
				}
				if next := fresh.nextAsyncJobID(); first == next {
					t.Fatal("org snapshot lost allocation sequence")
				}
			}
		})
	}
}
