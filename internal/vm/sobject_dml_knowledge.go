package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

var knowledgeArticleDMLPolicy = objectDMLPolicy{
	storedInsertDerivedFields: initializeKnowledgeArticleVersion,
}

// API 62/67 named observations generate a parent only in the stored row;
// insertion populates the caller's Id but leaves KnowledgeArticleId null.
func initializeKnowledgeArticleVersion(vm *VM, record *storage.Record) error {
	storage.EnsureStandardObject(vm.Org, "Knowledge__ka")
	parent := storage.Record{Object: "Knowledge__ka", Fields: make(map[string]storage.Value)}
	vm.recordIsolationJournalSequence(parent.Object)
	if _, err := vm.assignPendingInsertID(&parent); err != nil {
		return err
	}
	state, _ := storage.EnsureMutableObjectRecords(vm.Org, parent.Object)
	vm.recordIsolationJournalMutation(parent.Object, parent.ID, storage.Record{}, false)
	state.Records[parent.ID] = parent
	state.Indexes = nil
	vm.Org.Objects[parent.Object] = *state
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["KnowledgeArticleId"] = storage.IDValue(parent.ID)
	record.Fields["PublishStatus"] = storage.StringValue("Draft")
	record.Fields["VersionNumber"] = storage.IntegerValue(0)
	record.Fields["IsLatestVersion"] = storage.BooleanValue(true)
	return nil
}

func (vm *VM) publishKnowledgeArticle(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("KbManagement.PublishingService.publishArticle expects 2 arguments")
	}
	if args[1].Kind != ValueBool {
		return Null, fmt.Errorf("KbManagement.PublishingService.publishArticle argument 2 has wrong type")
	}
	id, ok := valueIDString(args[0])
	if !ok || storage.ValidateID(storage.ID(id)) != nil {
		return Null, newExceptionError("System.InvalidParameterValueException", "Invalid article ID")
	}
	missing := func() (Value, error) {
		return Null, newExceptionError("System.HandledException", "No draft article found")
	}
	if vm == nil || vm.Org == nil {
		return missing()
	}
	parents := vm.Org.Objects["Knowledge__ka"]
	if _, _, ok := storage.LookupRecordByID(parents.Records, storage.ID(id)); !ok {
		return missing()
	}
	versions := vm.Org.Objects["Knowledge__kav"]
	found := false
	for recordID, record := range versions.Records {
		if !storage.IDsEqual(storage.ID(storageValueIDText(record.Fields["KnowledgeArticleId"])), storage.ID(id)) ||
			!strings.EqualFold(storageRecordStringField(record, "PublishStatus"), "Draft") {
			continue
		}
		if !found {
			mutable, _ := storage.EnsureMutableObjectRecords(vm.Org, "Knowledge__kav")
			versions = *mutable
		}
		vm.recordIsolationJournalMutation("Knowledge__kav", recordID, record, true)
		record = cloneRuntimeSnapshotRecord(record)
		record.Fields["PublishStatus"] = storage.StringValue("Online")
		versions.Records[recordID] = record
		found = true
	}
	if !found {
		return missing()
	}
	versions.Indexes = nil
	vm.Org.Objects["Knowledge__kav"] = versions
	return Null, nil
}
