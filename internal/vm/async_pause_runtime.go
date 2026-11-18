package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// pauseResumeJob implements System.pauseJobById, pauseJobByName,
// resumeJobById and resumeJobByName. A scheduled job keeps its CronTrigger row,
// so pausing flips State between WAITING and PAUSED and counts one DML
// statement.
// NATIVE pause and resume scheduled jobs apex_release_gates/org.tsv anonymous 62.0 67.0
func (vm *VM) pauseResumeJob(callee string, args []Value) (Value, error) {
	pause := strings.Contains(callee, ".pauseJob")
	byID := strings.HasSuffix(callee, "ById")
	if len(args) != 1 {
		return Null, fmt.Errorf("%s expects 1 argument", callee)
	}
	param := "jobName"
	if byID {
		param = "cronTriggerId"
	}
	var key string
	switch {
	case args[0].Kind == ValueNull:
	case byID && args[0].Kind == ValueString:
		key = args[0].Text
	case byID:
		id, ok := valueIDString(args[0])
		if !ok {
			return Null, fmt.Errorf("%s expects job Id", callee)
		}
		key = id
	case args[0].Kind == ValueString:
		key = args[0].Text
	default:
		return Null, fmt.Errorf("%s expects job name String", callee)
	}
	if key == "" {
		return Null, newExceptionError("System.NullPointerException", "Parameter "+param+" cannot be empty or null")
	}
	missing := "No Apex job named " + key + " exists"
	if byID {
		missing = "No Apex job with CronTriggerId " + key + " exists"
	}
	if vm.Org == nil {
		return Null, newExceptionError("System.AsyncException", missing)
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["CronTrigger"]
	var found storage.Record
	for _, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		state := record.Fields["State"].String
		if strings.EqualFold(state, "Deleted") || strings.EqualFold(state, "Aborted") {
			continue
		}
		if byID {
			if !asyncJobIDTextEqual(string(record.ID), key) {
				continue
			}
		} else {
			detail := vm.Org.Objects["CronJobDetail"].Records[record.Fields["CronJobDetailId"].ID]
			if detail.System.IsDeleted || detail.Fields["Name"].String != key {
				continue
			}
		}
		found = record
		break
	}
	if found.ID == "" {
		return Null, newExceptionError("System.AsyncException", missing)
	}
	paused := strings.EqualFold(found.Fields["State"].String, "PAUSED")
	if pause && paused {
		return Null, newExceptionError("System.AsyncException", "Job ID is already paused")
	}
	if !pause && !paused {
		return Null, newExceptionError("System.AsyncException", "Job ID is not paused")
	}
	if err := vm.incrementLimit("dmlStatements", 1); err != nil {
		return Null, err
	}
	vm.recordIsolationJournalMutation("CronTrigger", found.ID, found, true)
	if pause {
		found.Fields["State"] = storage.StringValue("PAUSED")
	} else {
		found.Fields["State"] = storage.StringValue("WAITING")
	}
	object.PutRecord(found.ID, found)
	vm.Org.Objects["CronTrigger"] = object
	return Null, nil
}
