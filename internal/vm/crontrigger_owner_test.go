package vm

import (
	"reflect"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func cronTriggerOwnerMachine(t *testing.T) *VM {
	t.Helper()
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	machine.fakeNow = time.Date(2026, 10, 13, 18, 30, 17, 500000000, time.UTC)
	machine.SetCurrentUser(storage.Record{
		ID: "005000000000101", Object: "User",
		Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue("America/Los_Angeles")},
	})
	for _, class := range []Class{
		{Name: "MeadowScheduledWorker", Interfaces: []string{"Schedulable"}},
		{Name: "MeadowScheduledBatch", Interfaces: []string{"Database.Batchable<SObject>"}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

func TestExecCronTriggerCreationOwner(t *testing.T) {
	for _, tc := range []struct {
		name, schedule string
	}{
		{"schedule", `System.schedule('Meadow owner schedule', '0 30 1 7 11 ? 2027', new MeadowScheduledWorker())`},
		{"schedule_batch", `System.scheduleBatch(new MeadowScheduledBatch(), 'Meadow owner batch', 5, 1)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := cronTriggerOwnerMachine(t)
			program, err := CompileAnonymous(`
Id jobId = ` + tc.schedule + `;
CronTrigger scheduled = [SELECT OwnerId FROM CronTrigger WHERE Id = :jobId];
System.assertEquals(UserInfo.getUserId(), scheduled.OwnerId);
System.assertEquals(1, [SELECT COUNT() FROM CronTrigger WHERE Id = :jobId AND OwnerId = :UserInfo.getUserId()]);
`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecCronTriggerOwnerMetadataControls(t *testing.T) {
	for _, tc := range []struct {
		name, schedule, assertions string
	}{
		{
			name:     "schedule",
			schedule: `System.schedule('Meadow metadata schedule', '0 30 1 7 11 ? 2027', new MeadowScheduledWorker())`,
			assertions: `
System.assertEquals('0 30 1 7 11 ? 2027', scheduled.CronExpression);
System.assertEquals('America/Los_Angeles', scheduled.TimeZoneSidKey);
System.assertEquals('2027-11-07 09:30:00', scheduled.NextFireTime.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals('2026-10-13 18:30:17', scheduled.StartTime.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals('Meadow metadata schedule', scheduled.CronJobDetail.Name);
`,
		},
		{
			name:     "schedule_batch",
			schedule: `System.scheduleBatch(new MeadowScheduledBatch(), 'Meadow metadata batch', 5, 1)`,
			assertions: `
System.assertEquals('SimpleTrigger', scheduled.CronExpression);
System.assertEquals('GMT', scheduled.TimeZoneSidKey);
System.assertEquals(scheduled.NextFireTime, scheduled.StartTime);
System.assertEquals('2026-10-13 18:35:17', scheduled.NextFireTime.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals('Meadow metadata batch', scheduled.CronJobDetail.Name);
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := cronTriggerOwnerMachine(t)
			program, err := CompileAnonymous(`
Id jobId = ` + tc.schedule + `;
CronTrigger scheduled = [SELECT CronExpression, TimeZoneSidKey, NextFireTime, StartTime, PreviousFireTime, EndTime, State, TimesTriggered, CronJobDetail.Name, CronJobDetail.JobType FROM CronTrigger WHERE Id = :jobId];
System.assertEquals('WAITING', scheduled.State);
System.assertEquals(0, scheduled.TimesTriggered);
System.assertEquals(null, scheduled.PreviousFireTime);
System.assertEquals(null, scheduled.EndTime);
System.assertEquals('7', scheduled.CronJobDetail.JobType);
` + tc.assertions)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRecordCronTriggerUpdatePreservesOwner(t *testing.T) {
	// Missing and explicit-null owners preserve the existing local update
	// boundary; they do not describe a scheduler-created native row.
	for _, tc := range []struct {
		name    string
		owner   storage.Value
		present bool
	}{
		{"existing_owner", storage.IDValue("005000000000101"), true},
		{"missing_owner", storage.Value{}, false},
		{"explicit_null_owner", storage.NullValue(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := cronTriggerOwnerMachine(t)
			machine.ensureAsyncObjects()
			job := AsyncJob{ID: "707000000000101", Kind: "ScheduledApex", Name: "Meadow retained owner", Cron: "0 30 1 7 11 ? 2027"}
			id := storage.ID(cronTriggerID(job.ID))
			fields := map[string]storage.Value{"State": storage.StringValue("WAITING")}
			if tc.present {
				fields["OwnerId"] = tc.owner
			}
			state := machine.Org.Objects["CronTrigger"]
			state.Records[id] = storage.Record{ID: id, Object: "CronTrigger", Fields: fields}
			machine.Org.Objects["CronTrigger"] = state
			machine.SetCurrentUser(storage.Record{ID: "005000000000202", Object: "User"})
			machine.recordCronTrigger(job, "ACQUIRED")
			stored := machine.Org.Objects["CronTrigger"].Records[id]
			owner, present := stored.Fields["OwnerId"]
			if present != tc.present || !reflect.DeepEqual(owner, tc.owner) {
				t.Fatalf("owner after another user's update = %#v (%v), want %#v (%v)", owner, present, tc.owner, tc.present)
			}
			if got := stored.Fields["State"].String; got != "ACQUIRED" {
				t.Fatalf("state = %q, want ACQUIRED", got)
			}
		})
	}
}
