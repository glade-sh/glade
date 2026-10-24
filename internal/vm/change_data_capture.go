package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type changeDataCaptureEntry struct {
	operation     string
	before, after storage.Record
	user          string
}

func (vm *VM) hasChangeDataCaptureSubscribers() bool {
	for object := range vm.Triggers {
		if hasSuffixFold(object, "ChangeEvent") {
			return true
		}
	}
	return false
}

func (vm *VM) observeChangeDataCapture(operation string, before, after storage.Record) {
	if vm.testContext == nil || vm.Org == nil {
		return
	}
	record := after
	if operation == "DELETE" {
		record = before
	}
	event := changeDataCaptureObject(record.Object)
	if len(vm.triggersForOperation(event, triggerTimingAfter, "insert")) == 0 {
		return
	}
	// Enabling CDC applies to changes already made in this test transaction.
	vm.testContext.changeJournal = append(vm.testContext.changeJournal, changeDataCaptureEntry{
		operation: operation, before: before.Clone(), after: after.Clone(), user: vm.currentUserID(),
	})
}

func changeDataCaptureObject(object string) string {
	if hasSuffixFold(object, "__c") {
		return object[:len(object)-3] + "__ChangeEvent"
	}
	return object + "ChangeEvent"
}

func (vm *VM) snapshotChangeDataCapture() []changeDataCaptureEntry {
	if vm.testContext == nil {
		return nil
	}
	return append([]changeDataCaptureEntry(nil), vm.testContext.changeJournal...)
}

// CaptureChangeDataCaptureTestSetup retains only mutations observed during
// setup, without treating unrelated rows in the org as new test changes.
func (vm *VM) CaptureChangeDataCaptureTestSetup() {
	if vm.Org == nil || vm.testContext == nil {
		return
	}
	entries := make([]storage.ChangeDataCaptureEntry, len(vm.testContext.changeJournal))
	for i, entry := range vm.testContext.changeJournal {
		entries[i] = storage.ChangeDataCaptureEntry{
			Operation: entry.operation, Before: entry.before, After: entry.after, User: entry.user,
		}
	}
	vm.Org.ChangeDataCaptureTestSetup = storage.NewChangeDataCaptureSnapshot(entries)
}

func (vm *VM) restoreChangeDataCaptureTestSetup() {
	if vm.Org == nil || vm.testContext == nil {
		return
	}
	for _, entry := range vm.Org.ChangeDataCaptureTestSetup.Entries() {
		vm.testContext.changeJournal = append(vm.testContext.changeJournal, changeDataCaptureEntry{
			operation: entry.Operation, before: entry.Before, after: entry.After, user: entry.User,
		})
	}
}

func (vm *VM) restoreChangeDataCapture(entries []changeDataCaptureEntry) {
	if vm.testContext != nil {
		vm.testContext.changeJournal = append([]changeDataCaptureEntry(nil), entries...)
	}
}

func (vm *VM) changeDataCaptureCheckpoint() func() {
	entries := vm.snapshotChangeDataCapture()
	return func() { vm.restoreChangeDataCapture(entries) }
}

func (vm *VM) saveChangeDataCapture(id string) {
	if vm.testContext == nil {
		return
	}
	if vm.testContext.changeSavepoints == nil {
		vm.testContext.changeSavepoints = map[string][]changeDataCaptureEntry{}
	}
	vm.testContext.changeSavepoints[id] = vm.snapshotChangeDataCapture()
}

func (vm *VM) rollbackChangeDataCapture(id string) {
	if vm.testContext != nil {
		vm.restoreChangeDataCapture(vm.testContext.changeSavepoints[id])
	}
}

func (vm *VM) releaseChangeDataCapture(id string) {
	if vm.testContext != nil {
		delete(vm.testContext.changeSavepoints, id)
	}
}

func (vm *VM) materializeChangeDataCapture(entries []changeDataCaptureEntry) []storage.Record {
	// Multiple saves of a newly created record produce one CREATE with final
	// values. Existing records retain their first before-image for UPDATE.
	var changes []changeDataCaptureEntry
	byID := map[string]int{}
	for _, entry := range entries {
		record := entry.after
		if entry.operation == "DELETE" {
			record = entry.before
		}
		key := strings.ToLower(record.Object) + "/" + canonicalIDMapKey(string(record.ID))
		if i, ok := byID[key]; ok && entry.operation == "UPDATE" && (changes[i].operation == "CREATE" || changes[i].operation == "UPDATE") {
			changes[i].after = entry.after
			continue
		}
		byID[key] = len(changes)
		changes = append(changes, entry)
	}
	var events []storage.Record
	for _, entry := range changes {
		record := entry.after
		if entry.operation == "DELETE" {
			record = entry.before
		}
		// Workflow and after-save Flow may update the stored row after the
		// initial write. CREATE/UPDATE bodies expose those committed values.
		if entry.operation == "CREATE" || entry.operation == "UPDATE" {
			if _, final, ok := storage.LookupRecordByID(vm.Org.Objects[record.Object].Records, record.ID); ok && !final.System.IsDeleted {
				record = final
			}
		}
		eventName := changeDataCaptureObject(record.Object)
		storage.EnsureChangeEventObject(vm.Org, eventName)
		fields := map[string]storage.Value{}
		var changed, nulled []storage.Value
		for name := range vm.Org.Objects[eventName].Definition.Fields {
			if name == "ChangeEventHeader" || name == "ReplayId" {
				continue
			}
			fields[name] = storage.NullValue()
			if name == "Id" || entry.operation == "DELETE" {
				continue
			}
			value, ok := record.GetField(name)
			if !ok {
				value = storage.NullValue()
			}
			if entry.operation == "UPDATE" {
				prior, ok := entry.before.GetField(name)
				if !ok {
					prior = storage.NullValue()
				}
				if storage.ValuesEqual(prior, value) {
					continue
				}
				changed = append(changed, storage.StringValue(name))
				if value.Kind == storage.ValueNull {
					nulled = append(nulled, storage.StringValue(name))
				}
			}
			fields[name] = value.Clone()
		}
		sort.Slice(changed, func(i, j int) bool { return changed[i].String < changed[j].String })
		sort.Slice(nulled, func(i, j int) bool { return nulled[i].String < nulled[j].String })
		fields["ChangeEventHeader.entityName"] = storage.StringValue(record.Object)
		fields["ChangeEventHeader.changeType"] = storage.StringValue(entry.operation)
		fields["ChangeEventHeader.changedFields"] = storage.ListValue(changed...)
		fields["ChangeEventHeader.nulledFields"] = storage.ListValue(nulled...)
		fields["ChangeEventHeader.diffFields"] = storage.ListValue()
		fields["ChangeEventHeader.commitUser"] = storage.StringValue(entry.user)
		// Identical payloads share an event and list all affected record IDs.
		merged := false
		for i := range events {
			if events[i].Object != eventName || !sameChangeDataCaptureBody(events[i].Fields, fields) {
				continue
			}
			ids := events[i].Fields["ChangeEventHeader.recordIds"]
			ids.List = append(ids.List, storage.StringValue(apexIDTo18(string(record.ID))))
			events[i].Fields["ChangeEventHeader.recordIds"] = ids
			merged = true
			break
		}
		if merged {
			continue
		}
		fields["ChangeEventHeader.recordIds"] = storage.ListValue(storage.StringValue(apexIDTo18(string(record.ID))))
		events = append(events, storage.Record{Object: eventName, Fields: fields})
	}
	stamp := vm.fakeNow.UnixMilli()
	if vm.testContext.changeTransaction == "" {
		vm.testContext.changeTransaction = fmt.Sprintf("local-%d", stamp)
	}
	for i := range events {
		vm.testContext.changeSequence++
		fields := events[i].Fields
		fields["ChangeEventHeader.commitNumber"] = storage.IntegerValue(stamp)
		fields["ChangeEventHeader.commitTimestamp"] = storage.IntegerValue(stamp)
		fields["ChangeEventHeader.sequenceNumber"] = storage.IntegerValue(vm.testContext.changeSequence)
		fields["ChangeEventHeader.transactionKey"] = storage.StringValue(vm.testContext.changeTransaction)
		fields["ChangeEventHeader.changeOrigin"] = storage.StringValue("glade")
		fields["ReplayId"] = storage.StringValue(fmt.Sprintf("cdc-%d", vm.testContext.changeSequence))
	}
	return events
}

func sameChangeDataCaptureBody(a, b map[string]storage.Value) bool {
	if len(a) != len(b)+1 {
		return false
	} // a also has recordIds
	for field, value := range b {
		if other, ok := a[field]; !ok || !storage.ValuesEqual(value, other) {
			return false
		}
	}
	return true
}

func (vm *VM) drainChangeDataCapture(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	for iteration := 0; iteration < maxLoopIterations; iteration++ {
		entries := vm.testContext.changeJournal
		vm.testContext.changeJournal = nil
		if !vm.testContext.ChangeDataCaptureEnabled {
			// Deployed ChangeEvents members already select their entities;
			// explicit test enablement also includes unconfigured entities.
			var selected []changeDataCaptureEntry
			if vm.Org != nil {
				for _, entry := range entries {
					object := entry.after.Object
					if object == "" {
						object = entry.before.Object
					}
					for _, entity := range vm.Org.Metadata.ChangeEventEntities {
						if strings.EqualFold(entity, changeDataCaptureObject(object)) {
							selected = append(selected, entry)
							break
						}
					}
				}
			}
			entries = selected
		}
		if len(entries) == 0 {
			return nil
		}
		start := len(vm.testContext.PlatformEvents)
		vm.testContext.PlatformEvents = append(vm.testContext.PlatformEvents, vm.materializeChangeDataCapture(entries)...)
		// Both explicit and stopTest CDC delivery use Automated Process, while
		// the header retains the user who made the original mutation.
		if err := vm.drainTestPlatformEventsFrom(result, start, true); err != nil {
			return err
		}
	}
	return fmt.Errorf("test change event delivery exceeded %d iterations", maxLoopIterations)
}
