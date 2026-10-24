package vm

import (
	"fmt"
	"maps"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native scheduled_detail rows cover both enqueue methods and the
// queued/recorded abort routes at source APIs 62 and 67.
func TestScheduledDetailAbortQueuedAndRecordedRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, batch := range []bool{false, true} {
			for _, recorded := range []bool{false, true} {
				route := fmt.Sprintf("api%s/batch=%v/recorded=%v", api, batch, recorded)
				t.Run(route, func(t *testing.T) {
					machine := New(nil)
					org := storage.NewOrgState()
					machine.SetOrg(&org)
					machine.EnableTestContext()
					iface := "Schedulable"
					enqueue := "System.schedule('willow detail', '0 0 0 1 1 ? 2099', new WillowWorker())"
					kind := "ScheduledApex"
					if batch {
						iface = "Database.Batchable<SObject>"
						enqueue = "System.scheduleBatch(new WillowWorker(), 'willow detail', 60)"
						kind = "ScheduledBatch"
					}
					if err := machine.RegisterClass(Class{Name: "WillowWorker", Interfaces: []string{iface}}); err != nil {
						t.Fatal(err)
					}
					program, err := CompileAnonymousWithOptions("String ticket = "+enqueue+";", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					job := machine.testContext.AsyncJobs[0]
					cronID := storage.ID(cronTriggerID(job.ID))
					detailID := machine.Org.Objects["CronTrigger"].Records[cronID].Fields["CronJobDetailId"].ID
					if detailID == "" || machine.Org.Objects["CronJobDetail"].Records[detailID].Fields["Name"].String != "willow detail" {
						t.Fatal("missing initial detail")
					}
					if recorded {
						machine.recordAsyncJob(AsyncJob{ID: job.ID, Kind: kind, Object: job.Object}, "Completed", "")
						machine.testContext.AsyncJobs = nil
					}
					// Queried IDs are 18 characters, while enqueue returns the 15-character form.
					if _, err := machine.abortJob([]Value{String(apexIDTo18(string(cronID)))}); err != nil {
						t.Fatal(err)
					}
					if !machine.Org.Objects["CronTrigger"].Records[cronID].System.IsDeleted {
						t.Error("aborted trigger is still visible")
					}
					if record, exists := machine.Org.Objects["CronJobDetail"].Records[detailID]; exists && !record.System.IsDeleted {
						t.Error("native abortedDetails=0; Glade retained the CronJobDetail")
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					replacement := machine.testContext.AsyncJobs[len(machine.testContext.AsyncJobs)-1]
					replacementCron := machine.Org.Objects["CronTrigger"].Records[storage.ID(cronTriggerID(replacement.ID))]
					replacementID := replacementCron.Fields["CronJobDetailId"].ID
					if storage.IDsEqual(detailID, replacementID) {
						t.Error("native freshDetail=true; Glade reused the aborted detail identity")
					}
					if replacementDetail, exists := machine.Org.Objects["CronJobDetail"].Records[replacementID]; !exists || replacementDetail.System.IsDeleted {
						t.Error("replacement detail is missing")
					}
				})
			}
		}
	}
}

func TestScheduledDetailReleaseRetainsOtherLiveReferences(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	first := AsyncJob{ID: "707000000000001", Kind: "ScheduledApex", Name: "willow shared"}
	machine.recordCronTrigger(first, "WAITING")
	firstID := storage.ID(cronTriggerID(first.ID))
	object := machine.Org.Objects["CronTrigger"]
	firstRecord := object.Records[firstID]
	secondRecord := firstRecord
	secondRecord.Fields = maps.Clone(firstRecord.Fields)
	secondRecord.ID = "08e000000000002"
	object.Records[secondRecord.ID] = secondRecord
	machine.Org.Objects["CronTrigger"] = object
	detailID := firstRecord.Fields["CronJobDetailId"].ID
	machine.releaseScheduledCron(string(firstID))
	if machine.Org.Objects["CronJobDetail"].Records[detailID].System.IsDeleted {
		t.Fatal("detail was deleted while another live trigger referenced it")
	}
	machine.releaseScheduledCron(string(secondRecord.ID))
	if record, exists := machine.Org.Objects["CronJobDetail"].Records[detailID]; exists && !record.System.IsDeleted {
		t.Fatal("detail remained visible after its final trigger was released")
	}
}

func TestScheduledDetailRepeatedReleasePreservesReplacement(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	first := AsyncJob{ID: "707000000000001", Kind: "ScheduledApex", Name: "willow replacement"}
	machine.recordCronTrigger(first, "WAITING")
	firstID := cronTriggerID(first.ID)
	machine.releaseScheduledCron(firstID)
	replacement := AsyncJob{ID: "707000000000002", Kind: "ScheduledApex", Name: first.Name}
	machine.recordCronTrigger(replacement, "WAITING")
	machine.releaseScheduledCron(apexIDTo18(firstID))
	cron := machine.Org.Objects["CronTrigger"].Records[storage.ID(cronTriggerID(replacement.ID))]
	detailID := cron.Fields["CronJobDetailId"].ID
	detail, exists := machine.Org.Objects["CronJobDetail"].Records[detailID]
	if !exists || detail.System.IsDeleted || cron.System.IsDeleted {
		t.Fatal("releasing an old trigger hid the replacement job")
	}
}

func TestScheduledDetailReleaseJournalRollbackRestoresBothRows(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	job := AsyncJob{ID: "707000000000001", Kind: "ScheduledApex", Name: "willow journal", Cron: "0 0 0 1 1 ? 2099"}
	machine.recordCronTrigger(job, "WAITING")
	cronID := storage.ID(cronTriggerID(job.ID))
	detailID := org.Objects["CronTrigger"].Records[cronID].Fields["CronJobDetailId"].ID
	journal := storage.NewIsolationJournal(&org)
	mark := journal.Mark()
	machine.SetIsolationJournal(journal)
	machine.releaseScheduledCron(string(cronID))
	if !org.Objects["CronJobDetail"].Records[detailID].System.IsDeleted {
		t.Fatal("detail cleanup did not run")
	}
	if err := journal.Rollback(mark); err != nil {
		t.Fatal(err)
	}
	cron := org.Objects["CronTrigger"].Records[cronID]
	if cron.System.IsDeleted || cron.Fields["State"].String != "WAITING" || cron.Fields["NextFireTime"].Kind == storage.ValueNull {
		t.Fatal("journal rollback did not restore the scheduled trigger")
	}
	if detail, exists := org.Objects["CronJobDetail"].Records[detailID]; !exists || detail.System.IsDeleted {
		t.Fatal("journal rollback did not restore the job detail")
	}
}
