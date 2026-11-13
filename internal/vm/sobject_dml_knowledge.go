package vm

import (
	"fmt"
	"strings"
	"time"

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
	if vm.knowledgeIssuedParents == nil {
		vm.knowledgeIssuedParents = make(map[storage.ID]bool)
	}
	vm.knowledgeIssuedParents[parent.ID] = true
	state, _ := storage.EnsureMutableObjectRecords(vm.Org, parent.Object)
	vm.recordIsolationJournalMutation(parent.Object, parent.ID, storage.Record{}, false)
	state.PutRecord(parent.ID, parent)
	state.Indexes = nil
	vm.Org.Objects[parent.Object] = *state
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["KnowledgeArticleId"] = storage.IDValue(parent.ID)
	record.Fields["PublishStatus"] = storage.StringValue("Draft")
	record.Fields["VersionNumber"] = storage.IntegerValue(0)
	record.Fields["IsLatestVersion"] = storage.BooleanValue(true)
	if user := vm.currentUserID(); user != "" {
		record.Fields["AssignedToId"] = storage.IDValue(storage.ID(user))
	}
	return nil
}

func (vm *VM) publishKnowledgeArticle(args []Value) (Value, error) {
	if vm != nil && vm.Org != nil && !vm.currentUserHasKnowledgePermission() {
		return Null, newExceptionError("System.NoAccessException", "Knowledge user permission is required")
	}
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
	for _, record := range versions.Records {
		if storage.IDsEqual(storage.ID(storageValueIDText(record.Fields["KnowledgeArticleId"])), storage.ID(id)) && strings.EqualFold(storageRecordStringField(record, "PublishStatus"), "Draft") {
			found = true
			break
		}
	}
	if !found {
		return missing()
	}
	mutable, _ := storage.EnsureMutableObjectRecords(vm.Org, "Knowledge__kav")
	versions = *mutable
	var latestVersion int64
	for recordID, record := range versions.Records {
		if record.System.IsDeleted || !storage.IDsEqual(storage.ID(storageValueIDText(record.Fields["KnowledgeArticleId"])), storage.ID(id)) {
			continue
		}
		status := storageRecordStringField(record, "PublishStatus")
		if strings.EqualFold(status, "Online") || strings.EqualFold(status, "Archived") {
			if number := record.Fields["VersionNumber"].Integer; number > latestVersion {
				latestVersion = number
			}
		}
		if strings.EqualFold(status, "Online") {
			vm.recordIsolationJournalMutation("Knowledge__kav", recordID, record, true)
			record = cloneRuntimeSnapshotRecord(record)
			if args[1].Bool {
				record.Fields["PublishStatus"] = storage.StringValue("Archived")
				record.Fields["IsLatestVersion"] = storage.BooleanValue(false)
			} else {
				record.System.IsDeleted = true
			}
			versions.PutRecord(recordID, record)
		}
	}
	if args[1].Bool || latestVersion == 0 {
		latestVersion++
	}
	for recordID, record := range versions.Records {
		if !storage.IDsEqual(storage.ID(storageValueIDText(record.Fields["KnowledgeArticleId"])), storage.ID(id)) ||
			!strings.EqualFold(storageRecordStringField(record, "PublishStatus"), "Draft") {
			continue
		}
		vm.recordIsolationJournalMutation("Knowledge__kav", recordID, record, true)
		record = cloneRuntimeSnapshotRecord(record)
		record.Fields["PublishStatus"] = storage.StringValue("Online")
		record.Fields["VersionNumber"] = storage.IntegerValue(latestVersion)
		versions.PutRecord(recordID, record)
	}
	versions.Indexes = nil
	vm.Org.Objects["Knowledge__kav"] = versions
	return Null, nil
}

func (vm *VM) currentUserHasKnowledgePermission() bool {
	var user Value
	if vm.testContext != nil {
		user = vm.testContext.CurrentUser
	} else if vm.executionUser.Kind != "" && vm.executionUser.Kind != ValueNull {
		user = vm.executionUser
	} else {
		user = vm.defaultOrgUser()
	}
	if user.Kind == ValueObject {
		if _, value, ok := objectFieldValue(user, "UserPermissionsKnowledgeUser"); ok && value.Kind == ValueBool && value.Bool {
			return true
		}
	}
	if record, ok := vm.findOrgRecord("User", storage.ID(vm.currentUserID())); ok {
		if value, ok := record.GetField("UserPermissionsKnowledgeUser"); ok && value.Kind == storage.ValueBoolean {
			return value.Boolean
		}
	}
	return false
}

// transitionKnowledgeOnline implements the observed immediate archive and edit
// paths for Lightning Knowledge records.
func (vm *VM) transitionKnowledgeOnline(args []Value, edit bool) (Value, error) {
	id, ok := valueIDString(args[0])
	if !ok || storage.ValidateID(storage.ID(id)) != nil {
		return Null, newExceptionError("System.InvalidParameterValueException", "Invalid article ID")
	}
	missing := func() (Value, error) {
		return Null, newExceptionError("System.HandledException", "No online article found")
	}
	if vm == nil || vm.Org == nil {
		return Null, newExceptionError("System.InvalidParameterValueException", "Invalid article ID")
	}
	parents := vm.Org.Objects["Knowledge__ka"]
	if _, _, exists := storage.LookupRecordByID(parents.Records, storage.ID(id)); !exists {
		if !edit {
			for issued := range vm.knowledgeIssuedParents {
				if storage.IDsEqual(issued, storage.ID(id)) {
					return missing()
				}
			}
		}
		return Null, newExceptionError("System.InvalidParameterValueException", "Invalid article ID")
	}
	versions := vm.Org.Objects["Knowledge__kav"]
	var online storage.Record
	var onlineID storage.ID
	draft := false
	for key, row := range versions.Records {
		if row.System.IsDeleted || !storage.IDsEqual(storage.ID(storageValueIDText(row.Fields["KnowledgeArticleId"])), storage.ID(id)) {
			continue
		}
		switch strings.ToLower(storageRecordStringField(row, "PublishStatus")) {
		case "online":
			online, onlineID = row, key
		case "draft":
			draft = true
		}
	}
	if onlineID == "" || draft {
		return missing()
	}
	mutable, _ := storage.EnsureMutableObjectRecords(vm.Org, "Knowledge__kav")
	versions = *mutable
	if !edit || args[1].Bool {
		vm.recordIsolationJournalMutation("Knowledge__kav", onlineID, online, true)
		archived := cloneRuntimeSnapshotRecord(online)
		archived.Fields["PublishStatus"] = storage.StringValue("Archived")
		if edit {
			archived.Fields["IsLatestVersion"] = storage.BooleanValue(false)
		}
		versions.PutRecord(onlineID, archived)
	}
	if !edit {
		versions.Indexes = nil
		vm.Org.Objects["Knowledge__kav"] = versions
		return Null, nil
	}
	copy := cloneRuntimeSnapshotRecord(online)
	copy.ID = ""
	delete(copy.Fields, "Id")
	copy.Fields["PublishStatus"] = storage.StringValue("Draft")
	copy.Fields["VersionNumber"] = storage.IntegerValue(0)
	vm.recordIsolationJournalSequence(copy.Object)
	if _, err := vm.assignPendingInsertID(&copy); err != nil {
		return Null, err
	}
	vm.recordIsolationJournalMutation("Knowledge__kav", copy.ID, storage.Record{}, false)
	versions.PutRecord(copy.ID, copy)
	versions.Indexes = nil
	vm.Org.Objects["Knowledge__kav"] = versions
	return String(string(copy.ID)), nil
}

func (vm *VM) assignKnowledgeDraft(args []Value) (Value, error) {
	id, ok := valueIDString(args[0])
	if !ok || storage.ValidateID(storage.ID(id)) != nil {
		return Null, newExceptionError("System.InvalidParameterValueException", "Invalid article ID")
	}
	if vm != nil && vm.Org != nil {
		for recordID, row := range vm.Org.Objects["Knowledge__kav"].Records {
			if !storage.IDsEqual(storage.ID(storageValueIDText(row.Fields["KnowledgeArticleId"])), storage.ID(id)) || !strings.EqualFold(storageRecordStringField(row, "PublishStatus"), "Draft") {
				continue
			}
			user, ok := valueIDString(args[1])
			if !ok || storage.ValidateID(storage.ID(user)) != nil || !vm.knowledgeAssigneeHasPermission(storage.ID(user)) {
				return Null, newExceptionError("System.InvalidParameterValueException", "Invalid assignee")
			}
			due := storage.NullValue()
			if args[3].Kind != ValueNull {
				value, err := parsePlatformDatetime(args[3])
				if err != nil {
					return Null, err
				}
				due = storage.DateTimeValue(formatPlatformDatetime(value.Truncate(time.Second)))
			}
			mutable, _ := storage.EnsureMutableObjectRecords(vm.Org, "Knowledge__kav")
			vm.recordIsolationJournalMutation("Knowledge__kav", recordID, row, true)
			row = cloneRuntimeSnapshotRecord(row)
			row.Fields["AssignedToId"] = storage.IDValue(storage.ID(user))
			row.Fields["AssignedById"] = storage.IDValue(storage.ID(vm.currentUserID()))
			row.Fields["AssignmentDate"] = storage.DateTimeValue(formatPlatformDatetime(vm.fakeNow.Truncate(time.Second)))
			row.Fields["AssignmentDueDate"] = due
			row.Fields["AssignmentNote"] = storage.StringValue(args[2].Text)
			mutable.PutRecord(recordID, row)
			mutable.Indexes = nil
			vm.Org.Objects["Knowledge__kav"] = *mutable
			return Null, nil
		}
	}
	return Null, newExceptionError("System.HandledException", "No draft article found")
}

func (vm *VM) knowledgeAssigneeHasPermission(id storage.ID) bool {
	if storage.IDsEqual(id, storage.ID(vm.currentUserID())) {
		return vm.currentUserHasKnowledgePermission()
	}
	user, ok := vm.findOrgRecord("User", id)
	if !ok {
		return false
	}
	permission, ok := user.GetField("UserPermissionsKnowledgeUser")
	return ok && permission.Kind == storage.ValueBoolean && permission.Boolean
}
