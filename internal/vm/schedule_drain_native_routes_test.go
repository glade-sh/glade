package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// schedule_drain/org.tsv:2-17 isolates origin and class nesting at API 62/67.
// Exercise both callers of asyncJobDue: the drain count and ready-job lookup.
func TestScheduleDrainAdmissionAndDueNativeRoutes(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/schedule_drain/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 17 {
		t.Fatalf("native rows: count=%d error=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			var native struct{ Scheduled int }
			if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			org.APIVersion = row[0]
			machine.SetOrg(&org)
			machine.EnableTestContext()
			machine.testContext.Started = true
			machine.testContext.Stopped = true
			machine.testContext.Draining = !strings.HasPrefix(row[1], "direct")
			switch {
			case strings.HasPrefix(row[1], "batchFinish"):
				machine.currentAsyncKind = "BatchApex"
				machine.inBatchFinish = true
			case strings.HasPrefix(row[1], "queueable"):
				machine.currentAsyncKind = "Queueable"
				machine.currentQueueableDepth = 1
			case strings.HasPrefix(row[1], "scheduledCallback"):
				machine.currentAsyncKind = "ScheduledApex"
			case strings.HasPrefix(row[1], "direct"):
			default:
				t.Fatal("unknown captured origin")
			}
			class := Class{Name: "ScheduleDrainTop", Interfaces: []string{"Schedulable"}}
			if strings.HasSuffix(row[1], "InnerClass") {
				class.Name = "ScheduleDrainProbe.InnerWork"
				class.Nested = true
			}
			if err := machine.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
			machine.enqueueAsyncJob(AsyncJob{Kind: "ScheduledApex", Object: Object(class.Name), NotBefore: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)})
			machine.currentAsyncKind = ""
			machine.testContext.Draining = true
			jobs := machine.testContext.AsyncJobs
			if count := machine.drainableAsyncJobCount(jobs, 0); count != native.Scheduled {
				t.Errorf("native scheduled=%d; drain count=%d", native.Scheduled, count)
			}
			if ready := machine.nextDrainableAsyncJobIndex(jobs, 0) >= 0; ready != (native.Scheduled == 1) {
				t.Errorf("native scheduled=%d; ready=%t", native.Scheduled, ready)
			}
			// The settle check runs outside Draining. Callback-created top-level
			// schedules must keep it running; skipped inner jobs must not.
			machine.testContext.Draining = false
			if !strings.HasPrefix(row[1], "direct") || native.Scheduled == 0 {
				if ready := machine.nextDrainableAsyncJobIndex(jobs, 0) >= 0; ready != (native.Scheduled == 1) {
					t.Errorf("native scheduled=%d; settle ready=%t", native.Scheduled, ready)
				}
			}
		})
	}
}

// The native schedule_drain rows 7/15 exercise a first-level queueable body.
// Preserve the existing boundaries for other origins without claiming native
// coverage of finalizer scheduling or deeper queueable chains.
func TestScheduleDrainOtherQueueableOriginsPreserveAdmission(t *testing.T) {
	for _, route := range []struct {
		name  string
		depth int
	}{
		{"finalizer", 0},
		{"chained_body", 2},
	} {
		t.Run(route.name, func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			machine.testContext.Stopped = true
			machine.testContext.Draining = true
			machine.currentAsyncKind = "Queueable"
			machine.currentQueueableDepth = route.depth
			machine.enqueueAsyncJob(AsyncJob{Kind: "ScheduledApex", Object: Object("ScheduleDrainTop")})
			if len(machine.testContext.AsyncJobs) != 0 {
				t.Fatal("schedule admission changed outside the first-level queueable body")
			}
		})
	}
}
