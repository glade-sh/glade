package dml

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/storage"
)

type Engine struct {
	Org               *storage.OrgState
	IDs               storage.IDGenerator
	Now               func() time.Time
	UserID            storage.ID
	FlowActionInvoker func(storage.FlowAction, storage.Record) error
	WorkflowEmailer   func(storage.WorkflowEmailAlert, storage.Record) error
	AutomationTracer  func(name string, args map[string]any)
	DeferAutomation   bool

	workflowDepth  int
	flowDepth      int
	summaryOrder   []string
	summaryUpdates map[string]SummaryUpdate
}

type SummaryUpdate struct {
	Object string
	Before storage.Record
	After  storage.Record
}

type Result struct {
	ID                storage.ID   `json:"id,omitempty"`
	Success           bool         `json:"success"`
	Error             string       `json:"error,omitempty"`
	StatusCode        string       `json:"statusCode,omitempty"`
	Fields            []string     `json:"fields,omitempty"`
	Errors            []Error      `json:"errors,omitempty"`
	Created           bool         `json:"created,omitempty"`
	MergedRecordIDs   []storage.ID `json:"mergedRecordIds,omitempty"`
	UpdatedRelatedIDs []storage.ID `json:"updatedRelatedIds,omitempty"`
}

type Error struct {
	Message    string   `json:"message"`
	StatusCode string   `json:"statusCode,omitempty"`
	Fields     []string `json:"fields,omitempty"`
}

func NewEngine(org *storage.OrgState) Engine {
	prefixes := make(map[string]string, len(org.Objects))
	for name, object := range org.Objects {
		if object.Definition.KeyPrefix != "" {
			prefixes[name] = object.Definition.KeyPrefix
		}
	}
	ids := storage.NewRuntimeIDGenerator(prefixes)
	ids.Sequences = copySequences(org.IDSequences)
	return Engine{Org: org, IDs: ids, Now: func() time.Time { return time.Now().UTC() }, UserID: "005000000000001"}
}

func (e Engine) systemTimestamp() string {
	now := time.Now().UTC()
	if e.Now != nil {
		now = e.Now().UTC()
	}
	return now.Format(time.RFC3339)
}

func (e Engine) systemUserID() storage.ID {
	if e.UserID != "" {
		return e.UserID
	}
	return "005000000000001"
}

func (e *Engine) Insert(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		id, err := e.insertOne(record)
		if err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: id, Success: true}
	}
	e.Org.IDSequences = copySequences(e.IDs.Sequences)
	return results
}

func (e *Engine) Update(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if err := e.updateOne(record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) Delete(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if err := e.deleteOne(record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) Upsert(records []storage.Record) []Result {
	return e.upsert(records, "")
}

func (e *Engine) UpsertWithExternalID(records []storage.Record, externalIDField string) []Result {
	return e.upsert(records, externalIDField)
}

func (e *Engine) upsert(records []storage.Record, externalIDField string) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if record.ID == "" {
			id, created, err := e.upsertByExternalID(record, externalIDField)
			if err != nil {
				results[i] = resultFromError(record.ID, err)
				continue
			}
			results[i] = Result{ID: id, Success: true, Created: created}
			continue
		}
		if err := e.validateUpsertIDReference(record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		if err := e.updateOne(record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: record.ID, Success: true, Created: false}
	}
	e.Org.IDSequences = copySequences(e.IDs.Sequences)
	return results
}

func (e *Engine) Undelete(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if record.ID == "" {
			results[i] = Result{Success: false, Error: "dml: undelete requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
			continue
		}
		object, objectName, err := e.object(record.Object)
		if err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		if err := e.validateObjectID(object.Definition, record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		storedID, stored, ok := storage.LookupRecordByID(object.Records, record.ID)
		if !ok {
			results[i] = resultFromError(record.ID, fmt.Errorf("dml: record %s does not exist", record.ID))
			continue
		}
		if !stored.System.IsDeleted {
			results[i] = failedResult(record.ID, fmt.Sprintf("dml: record %s is not deleted", record.ID), "ENTITY_IS_NOT_DELETED", nil)
			continue
		}
		stamp := e.systemTimestamp()
		stored.System.IsDeleted = false
		stored.System.LastModifiedDate = stamp
		stored.System.SystemModstamp = stamp
		stored.System.LastModifiedByID = e.systemUserID()
		object.Records[storedID] = stored
		e.Org.Objects[objectName] = object
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) EmptyRecycleBin(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if record.ID == "" {
			results[i] = Result{Success: false, Error: "dml: emptyRecycleBin requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
			continue
		}
		object, objectName, err := e.object(record.Object)
		if err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		if err := e.validateObjectID(object.Definition, record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		storedID, stored, ok := storage.LookupRecordByID(object.Records, record.ID)
		if !ok {
			results[i] = resultFromError(record.ID, fmt.Errorf("dml: record %s does not exist", record.ID))
			continue
		}
		if !stored.System.IsDeleted {
			results[i] = failedResult(record.ID, fmt.Sprintf("dml: record %s is not in the recycle bin", record.ID), "ENTITY_IS_NOT_IN_RECYCLE_BIN", nil)
			continue
		}
		delete(object.Records, storedID)
		e.Org.Objects[objectName] = object
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) Lock(records []storage.Record) []Result {
	return e.setLock(records, true)
}

func (e *Engine) Unlock(records []storage.Record) []Result {
	return e.setLock(records, false)
}

func (e *Engine) Merge(master storage.Record, duplicates []storage.Record) []Result {
	results := make([]Result, len(duplicates))
	object, objectName, err := e.object(master.Object)
	if err != nil {
		for i := range results {
			results[i] = resultFromError(master.ID, err)
		}
		return results
	}
	if master.ID == "" {
		for i := range results {
			results[i] = Result{Success: false, Error: "dml: merge master requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
		}
		return results
	}
	storedMaster, ok := object.Records[master.ID]
	if !ok || storedMaster.System.IsDeleted {
		for i := range results {
			results[i] = resultFromError(master.ID, fmt.Errorf("dml: merge master %s does not exist", master.ID))
		}
		return results
	}
	if len(master.Fields) > 0 || len(master.ExplicitNulls) > 0 {
		if err := e.updateOne(master); err != nil {
			for i := range results {
				results[i] = resultFromError(master.ID, err)
			}
			return results
		}
		object = e.Org.Objects[objectName]
	}
	for i, duplicate := range duplicates {
		if duplicate.ID == "" {
			results[i] = Result{Success: false, Error: "dml: merge duplicate requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
			continue
		}
		if duplicate.Object != "" && duplicate.Object != objectName {
			results[i] = resultFromError(duplicate.ID, fmt.Errorf("dml: merge duplicate object %s does not match master %s", duplicate.Object, objectName))
			continue
		}
		if duplicate.ID == master.ID {
			results[i] = resultFromError(duplicate.ID, fmt.Errorf("dml: merge duplicate cannot be master"))
			continue
		}
		storedDuplicate, ok := object.Records[duplicate.ID]
		if !ok || storedDuplicate.System.IsDeleted {
			results[i] = resultFromError(duplicate.ID, fmt.Errorf("dml: merge duplicate %s does not exist", duplicate.ID))
			continue
		}
		updatedRelatedIDs := e.reparentLookups(objectName, duplicate.ID, master.ID)
		stamp := e.systemTimestamp()
		storedDuplicate.System.IsDeleted = true
		storedDuplicate.System.LastModifiedDate = stamp
		storedDuplicate.System.SystemModstamp = stamp
		storedDuplicate.System.LastModifiedByID = e.systemUserID()
		object.Records[duplicate.ID] = storedDuplicate
		e.Org.Objects[objectName] = object
		results[i] = Result{ID: master.ID, Success: true, MergedRecordIDs: []storage.ID{duplicate.ID}, UpdatedRelatedIDs: updatedRelatedIDs}
	}
	return results
}

func (e *Engine) setLock(records []storage.Record, locked bool) []Result {
	results := make([]Result, len(records))
	op := "lock"
	if !locked {
		op = "unlock"
	}
	for i, record := range records {
		if record.ID == "" {
			results[i] = Result{Success: false, Error: "dml: " + op + " requires id", StatusCode: "MISSING_ARGUMENT", Fields: []string{"Id"}}
			continue
		}
		object, objectName, err := e.object(record.Object)
		if err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		if err := e.validateObjectID(object.Definition, record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		storedID, stored, ok := storage.LookupRecordByID(object.Records, record.ID)
		if !ok || stored.System.IsDeleted {
			results[i] = resultFromError(record.ID, fmt.Errorf("dml: record %s does not exist", record.ID))
			continue
		}
		stored.System.Locked = locked
		object.Records[storedID] = stored
		e.Org.Objects[objectName] = object
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) reparentLookups(parentObject string, oldID, newID storage.ID) []storage.ID {
	seen := make(map[storage.ID]struct{})
	for childObjectName, childObject := range e.Org.Objects {
		changed := false
		for _, relation := range childObject.Definition.Relations {
			if !containsString(relation.ParentObjects, parentObject) {
				continue
			}
			for id, record := range childObject.Records {
				value, ok := record.Fields[relation.Field]
				if !ok || idFromStorageValue(value) != oldID {
					continue
				}
				record.Fields[relation.Field] = storage.IDValue(newID)
				childObject.Records[id] = record
				seen[id] = struct{}{}
				changed = true
			}
		}
		if changed {
			e.Org.Objects[childObjectName] = childObject
		}
	}
	ids := make([]storage.ID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (e *Engine) WithTransaction(fn func(*Engine) error) error {
	before := e.Org.CloneRuntime()
	if err := fn(e); err != nil {
		*e.Org = before
		e.IDs.Sequences = copySequences(before.IDSequences)
		return err
	}
	return nil
}

func (e *Engine) TakeSummaryUpdates() []SummaryUpdate {
	if e == nil || len(e.summaryOrder) == 0 || len(e.summaryUpdates) == 0 {
		return nil
	}
	updates := make([]SummaryUpdate, 0, len(e.summaryOrder))
	for _, key := range e.summaryOrder {
		update, ok := e.summaryUpdates[key]
		if !ok || update.Object == "" || update.Before.ID == "" || update.After.ID == "" {
			continue
		}
		updates = append(updates, update)
	}
	e.summaryOrder = nil
	e.summaryUpdates = nil
	return updates
}

func (e *Engine) insertOne(record storage.Record) (storage.ID, error) {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return "", err
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		return "", customMetadataReadOnlyError(objectName)
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return "", err
	}
	normalizeNameFields(objectName, object.Definition, &record)
	stripImplicitReadOnlyDefaultFields(object.Definition, e.Org.Namespace, &record, true)
	if err := validateFieldWriteability(object.Definition, e.Org.Namespace, record, true); err != nil {
		return "", err
	}
	createPersonContact := objectName == "Account" && isPersonAccountRecord(record)
	applyDefaultRecordTypeID(object.Definition, &record)
	applyFieldDefaults(e.Org, object.Definition, &record)
	applyAutoNumberName(object.Definition, e.IDs.Sequences[objectName]+1, &record)
	applyCustomSettingInsertDefaults(e.Org, object.Definition, &record)
	applySetupInsertDefaults(objectName, object.Definition, &record)
	e.applyFileInsertDefaults(objectName, object.Definition, &record)
	stripMissingGeneratedRecordTypeID(e.Org, &record)
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return "", err
	}
	if err := validateRequired(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateValidationRules(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateUnique(objectName, object.Definition, record, ""); err != nil {
		return "", err
	}
	needsRollback := strings.EqualFold(objectName, "ContentVersion") || createPersonContact || (!e.DeferAutomation && hasObjectAutomation(object.Definition))
	var rollbackOrg storage.OrgState
	var rollbackSequences map[string]uint64
	if needsRollback {
		rollbackOrg = e.Org.CloneRuntime()
		rollbackSequences = copySequences(e.IDs.Sequences)
	}
	if record.ID == "" {
		id, err := e.IDs.Next(objectName)
		if err != nil {
			return "", err
		}
		record.ID = id
	}
	if err := storage.ValidateID(record.ID); err != nil {
		return "", err
	}
	if _, exists := object.Records[record.ID]; exists {
		return "", dmlErrorf("DUPLICATE_VALUE", []string{"Id"}, "dml: duplicate id %s", record.ID)
	}
	stamp := e.systemTimestamp()
	userID := e.systemUserID()
	if record.System.CreatedDate == "" {
		record.System.CreatedDate = stamp
	}
	if record.System.LastModifiedDate == "" {
		record.System.LastModifiedDate = stamp
	}
	if record.System.SystemModstamp == "" {
		record.System.SystemModstamp = stamp
	}
	if record.System.CreatedByID == "" {
		record.System.CreatedByID = userID
	}
	if record.System.LastModifiedByID == "" {
		record.System.LastModifiedByID = userID
	}
	if record.System.OwnerID == "" {
		record.System.OwnerID = userID
	}
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	object.Records[record.ID] = record.Clone()
	e.Org.Objects[objectName] = object
	e.recalculateSummaryFieldsForChildren(objectName, record)
	if objectName == "ContentVersion" {
		if err := e.afterInsertContentVersion(record); err != nil {
			*e.Org = rollbackOrg
			e.IDs.Sequences = rollbackSequences
			return "", err
		}
	}
	if objectName == "ContentDistribution" {
		e.afterInsertContentDistribution(record.ID)
	}
	if objectName == "EmailMessage" {
		if err := e.afterInsertEmailMessage(record); err != nil {
			*e.Org = rollbackOrg
			e.IDs.Sequences = rollbackSequences
			return "", err
		}
	}
	if createPersonContact {
		if err := e.afterInsertPersonAccount(record); err != nil {
			*e.Org = rollbackOrg
			e.IDs.Sequences = rollbackSequences
			return "", err
		}
	}
	if !e.DeferAutomation {
		if err := e.ApplyAutomation(objectName, record.ID); err != nil {
			*e.Org = rollbackOrg
			e.IDs.Sequences = rollbackSequences
			return "", err
		}
	}
	return record.ID, nil
}

func normalizeNameFields(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil {
		return
	}
	if strings.EqualFold(objectName, "Contact") || strings.EqualFold(objectName, "Lead") {
		normalizeFirstLastName(definition, record)
		return
	}
	normalizePersonAccountFields(objectName, definition, record)
}

func normalizeFirstLastName(definition storage.ObjectDefinition, record *storage.Record) {
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if _, ok := definition.Fields["Name"]; !ok {
		return
	}
	if _, ok := record.Fields["Name"]; ok && !hasNameComponentField(record.Fields) {
		return
	}
	firstName := stringField(record.Fields, "FirstName")
	lastName := stringField(record.Fields, "LastName")
	switch {
	case firstName != "" && lastName != "":
		record.Fields["Name"] = storage.StringValue(firstName + " " + lastName)
	case lastName != "":
		record.Fields["Name"] = storage.StringValue(lastName)
	}
}

func hasNameComponentField(fields map[string]storage.Value) bool {
	_, hasFirst := fields["FirstName"]
	_, hasLast := fields["LastName"]
	return hasFirst || hasLast
}

func normalizePersonAccountFields(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if !strings.EqualFold(objectName, "Account") || record == nil {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if !hasPersonAccountSignal(*record) {
		return
	}
	record.Fields["IsPersonAccount"] = storage.BooleanValue(true)
	normalizeFirstLastName(definition, record)
}

func hasPersonAccountSignal(record storage.Record) bool {
	return hasPersonAccountFieldSignal(record)
}

func hasPersonAccountFieldSignal(record storage.Record) bool {
	for field, value := range record.Fields {
		if (strings.HasPrefix(field, "Person") || field == "FirstName" || field == "LastName") && nonDefaultPersonValue(value) {
			return true
		}
	}
	return false
}

func nonDefaultPersonValue(value storage.Value) bool {
	switch value.Kind {
	case storage.ValueNull, "":
		return false
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return value.String != ""
	case storage.ValueID:
		return value.ID != ""
	case storage.ValueInteger:
		return value.Integer != 0
	default:
		return true
	}
}

func isPersonAccountRecord(record storage.Record) bool {
	if value, ok := record.Fields["IsPersonAccount"]; ok && value.Kind == storage.ValueBoolean {
		if !value.Boolean {
			return false
		}
		return hasPersonAccountFieldSignal(record) || idFromStorageValue(record.Fields["PersonContactId"]) != ""
	}
	return false
}

func stringField(fields map[string]storage.Value, name string) string {
	value, ok := fields[name]
	if !ok || value.Kind != storage.ValueString {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func applyCustomSettingInsertDefaults(org *storage.OrgState, definition storage.ObjectDefinition, record *storage.Record) {
	if org == nil || record == nil || !storage.IsCustomSettingDefinition(definition) {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	orgID := strings.TrimSpace(org.OrgID)
	if orgID == "" {
		orgID = "00D000000000001"
	}
	if value, ok := record.Fields["Name"]; !ok || value.Kind == storage.ValueNull || (value.Kind == storage.ValueString && strings.TrimSpace(value.String) == "") {
		record.Fields["Name"] = storage.StringValue(orgID)
	}
	if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
		if _, fieldOK := definition.Fields["SetupOwnerId"]; fieldOK {
			if value, ok := record.Fields["SetupOwnerId"]; !ok || value.Kind == storage.ValueNull || (value.Kind == storage.ValueString && strings.TrimSpace(value.String) == "") {
				record.Fields["SetupOwnerId"] = storage.StringValue(orgID)
			}
		}
	}
}

func applySetupInsertDefaults(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil {
		return
	}
	defaultRequiredBoolean := func(fieldName string) bool {
		switch {
		case (strings.EqualFold(objectName, "PermissionSet") || strings.EqualFold(objectName, "Profile")) && strings.HasPrefix(fieldName, "Permissions"):
			return true
		case strings.EqualFold(objectName, "User"):
			return true
		default:
			return false
		}
	}
	if !strings.EqualFold(objectName, "PermissionSet") && !strings.EqualFold(objectName, "Profile") && !strings.EqualFold(objectName, "User") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	for name, field := range definition.Fields {
		if field.Type != storage.FieldBoolean || !field.Required || !defaultRequiredBoolean(name) {
			continue
		}
		if _, ok := record.GetField(name); ok {
			continue
		}
		if record.HasExplicitNull(name) {
			continue
		}
		record.Fields[name] = storage.BooleanValue(false)
	}
	if strings.EqualFold(objectName, "User") {
		if _, ok := definition.Fields["CommunityNickname"]; !ok {
			return
		}
		defaultUserCommunityNickname(record)
	}
}

func (e *Engine) applyFileInsertDefaults(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil || !strings.EqualFold(objectName, "Document") {
		return
	}
	field, ok := definition.Fields["FolderId"]
	if !ok || !fieldReferencesObject(field, "User") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if _, ok := record.Fields["FolderId"]; ok {
		return
	}
	if record.ExplicitNulls != nil && record.ExplicitNulls["FolderId"] {
		return
	}
	record.Fields["FolderId"] = storage.IDValue(e.systemUserID())
}

func fieldReferencesObject(field storage.Field, objectName string) bool {
	for _, target := range field.ReferenceTo {
		if strings.EqualFold(target, objectName) {
			return true
		}
	}
	return false
}

func defaultUserCommunityNickname(record *storage.Record) {
	if record == nil || record.Fields == nil {
		return
	}
	if _, ok := record.Fields["CommunityNickname"]; ok {
		return
	}
	if record.ExplicitNulls != nil && record.ExplicitNulls["CommunityNickname"] {
		return
	}
	for _, field := range []string{"Alias", "Username", "LastName"} {
		value, ok := record.Fields[field]
		if !ok || value.Kind != storage.ValueString || strings.TrimSpace(value.String) == "" {
			continue
		}
		record.Fields["CommunityNickname"] = storage.StringValue(strings.TrimSpace(value.String))
		return
	}
}

func applyFieldDefaults(org *storage.OrgState, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	for name, field := range definition.Fields {
		if _, ok := record.Fields[name]; ok {
			continue
		}
		if strings.EqualFold(name, "RecordTypeId") {
			continue
		}
		if record.ExplicitNulls != nil && record.ExplicitNulls[name] {
			continue
		}
		if value, ok := defaultValueForRecordField(org, definition, *record, field); ok {
			record.Fields[name] = value
		}
	}
}

func applyDefaultRecordTypeID(definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil || len(definition.RecordTypes) == 0 {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if _, ok := record.GetField("RecordTypeId"); ok {
		return
	}
	if record.ExplicitNulls != nil && record.ExplicitNulls["RecordTypeId"] {
		return
	}
	recordType, ok := defaultRecordType(definition.RecordTypes)
	if !ok || recordType.ID == "" {
		return
	}
	record.Fields["RecordTypeId"] = storage.IDValue(recordType.ID)
}

func defaultRecordType(recordTypes []storage.RecordTypeInfo) (storage.RecordTypeInfo, bool) {
	for _, recordType := range recordTypes {
		if recordType.Default && recordType.Active && recordType.Available {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Default && recordType.Active {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Default {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if len(recordType.PicklistDefaults) > 0 && recordType.Active && recordType.Available {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if len(recordType.PicklistDefaults) > 0 && recordType.Active {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Active && recordType.Available {
			return recordType, true
		}
	}
	for _, recordType := range recordTypes {
		if recordType.Active {
			return recordType, true
		}
	}
	if len(recordTypes) == 0 {
		return storage.RecordTypeInfo{}, false
	}
	return recordTypes[0], true
}

func defaultValueForRecordField(org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, field storage.Field) (storage.Value, bool) {
	rawDefault := strings.TrimSpace(field.DefaultValue)
	if org != nil && strings.Contains(rawDefault, "$RecordType") {
		if value, _, ok := EvaluateRecordFormulaValueInOrg(rawDefault, field, org, definition, record); ok {
			return value, true
		}
	}
	if value, ok := storage.DefaultValueForRecordField(definition, record, field); ok {
		return value, true
	}
	if org == nil || rawDefault == "" {
		return storage.Value{}, false
	}
	value, _, ok := EvaluateRecordFormulaValueInOrg(rawDefault, field, org, definition, record)
	return value, ok
}

func applyAutoNumberName(definition storage.ObjectDefinition, sequence uint64, record *storage.Record) {
	if record == nil {
		return
	}
	nameField, ok := definition.Fields["Name"]
	if !ok || !nameField.AutoNumber {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if value, ok := record.Fields["Name"]; ok && value.Kind == storage.ValueString && strings.TrimSpace(value.String) != "" {
		return
	}
	record.Fields["Name"] = storage.StringValue(formatAutoNumber(nameField.DisplayFormat, sequence))
}

func formatAutoNumber(format string, sequence uint64) string {
	format = strings.TrimSpace(format)
	if format == "" {
		return fmt.Sprintf("%d", sequence)
	}
	start := strings.Index(format, "{")
	end := strings.Index(format, "}")
	if start < 0 || end <= start {
		return format
	}
	token := format[start+1 : end]
	width := 0
	for _, r := range token {
		if r != '0' {
			width = 0
			break
		}
		width++
	}
	number := fmt.Sprintf("%d", sequence)
	if width > 0 {
		number = fmt.Sprintf("%0*d", width, sequence)
	}
	return format[:start] + number + format[end+1:]
}

func (e *Engine) afterInsertContentVersion(version storage.Record) error {
	contentDocumentID := idFromStorageValue(version.Fields["ContentDocumentId"])
	if contentDocumentID == "" {
		document := storage.Record{
			Object: "ContentDocument",
			Fields: map[string]storage.Value{
				"Title":                    version.Fields["Title"].Clone(),
				"LatestPublishedVersionId": storage.IDValue(version.ID),
			},
		}
		if path, ok := version.Fields["PathOnClient"]; ok {
			extension := fileExtension(path.String)
			document.Fields["FileExtension"] = storage.StringValue(extension)
			if fileType := contentDocumentFileType(extension); fileType != "" {
				document.Fields["FileType"] = storage.StringValue(fileType)
			}
		}
		id, err := e.insertPlatformRecord(document)
		if err != nil {
			return err
		}
		contentDocumentID = id
		contentVersionObject := e.Org.Objects["ContentVersion"]
		stored := contentVersionObject.Records[version.ID]
		if stored.Fields == nil {
			stored.Fields = make(map[string]storage.Value)
		}
		stored.Fields["ContentDocumentId"] = storage.IDValue(contentDocumentID)
		contentVersionObject.Records[version.ID] = stored
		e.Org.Objects["ContentVersion"] = contentVersionObject
	} else {
		contentDocumentObject := e.Org.Objects["ContentDocument"]
		document, exists := contentDocumentObject.Records[contentDocumentID]
		if !exists {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{"ContentDocumentId"}, "dml: ContentDocument %s does not exist", contentDocumentID)
		}
		if document.Fields == nil {
			document.Fields = make(map[string]storage.Value)
		}
		document.Fields["LatestPublishedVersionId"] = storage.IDValue(version.ID)
		if title, ok := version.Fields["Title"]; ok {
			document.Fields["Title"] = title.Clone()
		}
		if path, ok := version.Fields["PathOnClient"]; ok {
			extension := fileExtension(path.String)
			document.Fields["FileExtension"] = storage.StringValue(extension)
			if fileType := contentDocumentFileType(extension); fileType != "" {
				document.Fields["FileType"] = storage.StringValue(fileType)
			}
		}
		contentDocumentObject.Records[contentDocumentID] = document
		e.Org.Objects["ContentDocument"] = contentDocumentObject
	}
	e.markLatestContentVersion(contentDocumentID, version.ID)
	if locationID := idFromStorageValue(version.Fields["FirstPublishLocationId"]); locationID != "" {
		link := storage.Record{
			Object: "ContentDocumentLink",
			Fields: map[string]storage.Value{
				"ContentDocumentId": storage.IDValue(contentDocumentID),
				"LinkedEntityId":    storage.IDValue(locationID),
				"ShareType":         storage.StringValue("V"),
				"Visibility":        storage.StringValue("AllUsers"),
			},
		}
		if _, err := e.insertOne(link); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) afterInsertContentDistribution(id storage.ID) {
	object := e.Org.Objects["ContentDistribution"]
	record, ok := object.Records[id]
	if !ok {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	base := "https://oaer.local/content/" + string(id)
	if _, ok := record.Fields["ContentDownloadUrl"]; !ok {
		record.Fields["ContentDownloadUrl"] = storage.StringValue(base + "/download")
	}
	if _, ok := record.Fields["DistributionPublicUrl"]; !ok {
		record.Fields["DistributionPublicUrl"] = storage.StringValue(base)
	}
	object.Records[id] = record
	e.Org.Objects["ContentDistribution"] = object
}

func (e *Engine) afterInsertEmailMessage(message storage.Record) error {
	toIDs, ok := message.GetField("ToIds")
	if !ok || toIDs.Kind != storage.ValueList {
		return nil
	}
	for _, toID := range toIDs.List {
		relationID := valueAsIDString(toID)
		if relationID == "" {
			continue
		}
		if relationID == "system" {
			relationID = string(e.systemUserID())
		}
		if err := storage.ValidateID(storage.ID(relationID)); err != nil {
			continue
		}
		storage.EnsureStandardObject(e.Org, "EmailMessageRelation")
		relation := storage.Record{
			Object: "EmailMessageRelation",
			Fields: map[string]storage.Value{
				"EmailMessageId": storage.IDValue(message.ID),
				"RelationId":     storage.IDValue(storage.ID(relationID)),
				"RelationType":   storage.StringValue("ToAddress"),
			},
		}
		if toAddress, ok := message.GetField("ToAddress"); ok && toAddress.String != "" {
			relation.Fields["RelationAddress"] = storage.StringValue(toAddress.String)
		}
		if _, err := e.insertPlatformRecord(relation); err != nil {
			return err
		}
	}
	return nil
}

func valueAsIDString(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return strings.TrimSpace(value.String)
	default:
		return ""
	}
}

func (e *Engine) insertPlatformRecord(record storage.Record) (storage.ID, error) {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return "", err
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return "", err
	}
	applyFieldDefaults(e.Org, object.Definition, &record)
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return "", err
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return "", err
	}
	if record.ID == "" {
		id, err := e.IDs.Next(objectName)
		if err != nil {
			return "", err
		}
		record.ID = id
	}
	if err := storage.ValidateID(record.ID); err != nil {
		return "", err
	}
	if _, exists := object.Records[record.ID]; exists {
		return "", dmlErrorf("DUPLICATE_VALUE", []string{"Id"}, "dml: duplicate id %s", record.ID)
	}
	stamp := e.systemTimestamp()
	userID := e.systemUserID()
	if record.System.CreatedDate == "" {
		record.System.CreatedDate = stamp
	}
	if record.System.LastModifiedDate == "" {
		record.System.LastModifiedDate = stamp
	}
	if record.System.SystemModstamp == "" {
		record.System.SystemModstamp = stamp
	}
	if record.System.CreatedByID == "" {
		record.System.CreatedByID = userID
	}
	if record.System.LastModifiedByID == "" {
		record.System.LastModifiedByID = userID
	}
	if record.System.OwnerID == "" {
		record.System.OwnerID = userID
	}
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	object.Records[record.ID] = record.Clone()
	e.Org.Objects[objectName] = object
	return record.ID, nil
}

func (e *Engine) markLatestContentVersion(contentDocumentID storage.ID, latestVersionID storage.ID) {
	contentVersionObject := e.Org.Objects["ContentVersion"]
	changed := false
	for id, stored := range contentVersionObject.Records {
		if idFromStorageValue(stored.Fields["ContentDocumentId"]) != contentDocumentID {
			continue
		}
		if stored.Fields == nil {
			stored.Fields = make(map[string]storage.Value)
		}
		stored.Fields["IsLatest"] = storage.BooleanValue(id == latestVersionID)
		contentVersionObject.Records[id] = stored
		changed = true
	}
	if changed {
		e.Org.Objects["ContentVersion"] = contentVersionObject
	}
}

func (e *Engine) afterInsertPersonAccount(account storage.Record) error {
	if _, ok := e.Org.Objects["Contact"]; !ok {
		return nil
	}
	contact := storage.Record{
		Object: "Contact",
		Fields: personContactFields(account),
	}
	contactID, err := e.insertOne(contact)
	if err != nil {
		return err
	}
	accountObject := e.Org.Objects["Account"]
	stored := accountObject.Records[account.ID]
	if stored.Fields == nil {
		stored.Fields = make(map[string]storage.Value)
	}
	stored.Fields["PersonContactId"] = storage.IDValue(contactID)
	accountObject.Records[account.ID] = stored
	e.Org.Objects["Account"] = accountObject
	return nil
}

func personContactFields(account storage.Record) map[string]storage.Value {
	fields := map[string]storage.Value{
		"AccountId": storage.IDValue(account.ID),
	}
	copyPersonContactField(fields, account, "FirstName", "FirstName")
	copyPersonContactField(fields, account, "LastName", "LastName")
	copyPersonContactField(fields, account, "PersonEmail", "Email")
	copyPersonContactField(fields, account, "PersonHomePhone", "HomePhone")
	copyPersonContactField(fields, account, "PersonMobilePhone", "MobilePhone")
	copyPersonContactField(fields, account, "PersonTitle", "Title")
	copyPersonContactField(fields, account, "PersonDepartment", "Department")
	copyPersonContactField(fields, account, "PersonBirthdate", "Birthdate")
	copyPersonContactField(fields, account, "PersonDoNotCall", "DoNotCall")
	copyPersonContactField(fields, account, "PersonHasOptedOutOfEmail", "HasOptedOutOfEmail")
	copyPersonContactField(fields, account, "PersonHasOptedOutOfFax", "HasOptedOutOfFax")
	copyPersonContactField(fields, account, "PersonEmailBouncedReason", "EmailBouncedReason")
	copyPersonContactField(fields, account, "PersonEmailBouncedDate", "EmailBouncedDate")
	for _, suffix := range []string{"Street", "City", "State", "StateCode", "PostalCode", "Country", "CountryCode"} {
		copyPersonContactField(fields, account, "PersonMailing"+suffix, "Mailing"+suffix)
		copyPersonContactField(fields, account, "PersonOther"+suffix, "Other"+suffix)
	}
	if _, ok := fields["LastName"]; !ok || fields["LastName"].Kind == storage.ValueNull {
		fields["LastName"] = storage.StringValue(stringField(account.Fields, "Name"))
	}
	for field, value := range fields {
		if value.Kind == "" {
			delete(fields, field)
		}
	}
	return fields
}

func copyPersonContactField(fields map[string]storage.Value, account storage.Record, source, target string) {
	if value, ok := account.Fields[source]; ok {
		fields[target] = value.Clone()
	}
}

func (e *Engine) syncPersonContact(account storage.Record) error {
	if !isPersonAccountRecord(account) {
		return nil
	}
	contactID := idFromStorageValue(account.Fields["PersonContactId"])
	if contactID == "" {
		return nil
	}
	contactObject, ok := e.Org.Objects["Contact"]
	if !ok {
		return nil
	}
	contact, ok := contactObject.Records[contactID]
	if !ok || contact.System.IsDeleted {
		return nil
	}
	if contact.Fields == nil {
		contact.Fields = make(map[string]storage.Value)
	}
	for field, value := range personContactFields(account) {
		contact.Fields[field] = value.Clone()
	}
	contactObject.Records[contactID] = contact
	e.Org.Objects["Contact"] = contactObject
	return nil
}

func deleteCaseInsensitiveField(fields map[string]storage.Value, field string) {
	if fields == nil || field == "" {
		return
	}
	for existing := range fields {
		if existing != field && strings.EqualFold(existing, field) {
			delete(fields, existing)
		}
	}
}

func fileExtension(path string) string {
	lastSlash := strings.LastIndexAny(path, `/\`)
	lastDot := strings.LastIndex(path, ".")
	if lastDot <= lastSlash || lastDot == len(path)-1 {
		return ""
	}
	return path[lastDot+1:]
}

func contentDocumentFileType(extension string) string {
	switch strings.ToLower(strings.TrimPrefix(extension, ".")) {
	case "docx":
		return "WORD_X"
	case "xlsx":
		return "EXCEL_X"
	case "pptx":
		return "POWER_POINT_X"
	case "pdf":
		return "PDF"
	case "jpg", "jpeg", "gif", "png":
		return strings.ToUpper(extension)
	case "m4a":
		return "M4A"
	default:
		return strings.ToUpper(extension)
	}
}

func (e *Engine) updateOne(record storage.Record) error {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return err
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		return customMetadataReadOnlyError(objectName)
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return err
	}
	normalizeNameFields(objectName, object.Definition, &record)
	if record.ID == "" {
		return fmt.Errorf("dml: update requires id")
	}
	stripImplicitReadOnlyDefaultFields(object.Definition, e.Org.Namespace, &record, false)
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return err
	}
	storedID, existing, ok := storage.LookupRecordByID(object.Records, record.ID)
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", record.ID)
	}
	if existing.System.IsDeleted {
		return fmt.Errorf("dml: record %s is deleted", record.ID)
	}
	stripUnchangedNonUpdateableFields(object.Definition, e.Org.Namespace, &record, existing)
	if err := validateFieldWriteability(object.Definition, e.Org.Namespace, record, false); err != nil {
		return err
	}
	stripReadOnlyUpdateFields(object.Definition, e.Org.Namespace, &record)
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateUnique(objectName, object.Definition, record, storedID); err != nil {
		return err
	}
	finalRecord := existing.Clone()
	if finalRecord.Fields == nil {
		finalRecord.Fields = make(map[string]storage.Value)
	}
	if finalRecord.ExplicitNulls == nil {
		finalRecord.ExplicitNulls = make(map[string]bool)
	}
	for field, value := range record.Fields {
		deleteCaseInsensitiveField(finalRecord.Fields, field)
		finalRecord.Fields[field] = value.Clone()
		delete(finalRecord.ExplicitNulls, field)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			deleteCaseInsensitiveField(finalRecord.Fields, field)
			delete(finalRecord.Fields, field)
			finalRecord.ExplicitNulls[field] = true
		}
	}
	if err := validateRequired(object.Definition, finalRecord); err != nil {
		return err
	}
	if err := e.validateValidationRules(object.Definition, finalRecord); err != nil {
		return err
	}
	if existing.Fields == nil {
		existing.Fields = make(map[string]storage.Value)
	}
	if existing.ExplicitNulls == nil {
		existing.ExplicitNulls = make(map[string]bool)
	}
	needsRollback := !e.DeferAutomation && hasObjectAutomation(object.Definition)
	var rollbackOrg storage.OrgState
	var rollbackSequences map[string]uint64
	if needsRollback {
		rollbackOrg = e.Org.CloneRuntime()
		rollbackSequences = copySequences(e.IDs.Sequences)
	}
	stamp := e.systemTimestamp()
	oldRecord := existing.Clone()
	for field, value := range record.Fields {
		existing.Fields[field] = value.Clone()
		delete(existing.ExplicitNulls, field)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			delete(existing.Fields, field)
			existing.ExplicitNulls[field] = true
		}
	}
	existing.System.LastModifiedDate = stamp
	existing.System.SystemModstamp = stamp
	existing.System.LastModifiedByID = e.systemUserID()
	object.Records[storedID] = existing
	e.Org.Objects[objectName] = object
	e.recalculateSummaryFieldsForChildren(objectName, oldRecord, finalRecord)
	if strings.EqualFold(objectName, "Account") {
		if err := e.syncPersonContact(existing); err != nil {
			*e.Org = rollbackOrg
			e.IDs.Sequences = rollbackSequences
			return err
		}
	}
	if !e.DeferAutomation {
		if err := e.ApplyAutomation(objectName, storedID); err != nil {
			*e.Org = rollbackOrg
			e.IDs.Sequences = rollbackSequences
			return err
		}
	}
	return nil
}

func (e *Engine) deleteOne(record storage.Record) error {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return err
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		return customMetadataReadOnlyError(objectName)
	}
	if record.ID == "" {
		return fmt.Errorf("dml: delete requires id")
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return err
	}
	storedID, stored, ok := storage.LookupRecordByID(object.Records, record.ID)
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", record.ID)
	}
	if stored.System.IsDeleted {
		return fmt.Errorf("dml: record %s is deleted", record.ID)
	}
	if err := e.validateDeleteReferences(objectName, storedID); err != nil {
		return err
	}
	return e.deleteRecord(objectName, storedID, make(map[string]bool))
}

func (e *Engine) deleteRecord(objectName string, id storage.ID, seen map[string]bool) error {
	key := objectName + ":" + string(id)
	if seen[key] {
		return nil
	}
	seen[key] = true
	object := e.Org.Objects[objectName]
	storedID, stored, ok := storage.LookupRecordByID(object.Records, id)
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", id)
	}
	if stored.System.IsDeleted {
		return fmt.Errorf("dml: record %s is deleted", id)
	}
	if err := e.validateDeleteReferences(objectName, id); err != nil {
		return err
	}
	stamp := e.systemTimestamp()
	stored.System.IsDeleted = true
	stored.System.LastModifiedDate = stamp
	stored.System.SystemModstamp = stamp
	stored.System.LastModifiedByID = e.systemUserID()
	object.Records[storedID] = stored
	e.Org.Objects[objectName] = object
	e.recalculateSummaryFieldsForChildren(objectName, stored)
	return e.cascadeDeleteChildren(objectName, storedID, seen)
}

func (e *Engine) upsertByExternalID(record storage.Record, externalIDField string) (storage.ID, bool, error) {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return "", false, err
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		return "", false, customMetadataReadOnlyError(objectName)
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return "", false, err
	}
	field, value, ok, err := upsertExternalID(object.Definition, e.Org.Namespace, record, externalIDField)
	if err != nil {
		return "", false, err
	}
	if !ok {
		id, err := e.insertOne(record)
		return id, true, err
	}
	matches := make([]storage.ID, 0, 1)
	for id, stored := range object.Records {
		if stored.System.IsDeleted {
			continue
		}
		storedValue, storedOK := stored.Fields[field]
		if !storedOK || !storageValuesEqual(object.Definition.Fields[field], storedValue, value) {
			continue
		}
		matches = append(matches, id)
	}
	if len(matches) > 1 {
		return "", false, dmlErrorf("DUPLICATE_VALUE", []string{field}, "dml: external id %s.%s matched multiple records", objectName, field)
	}
	if len(matches) == 0 {
		id, err := e.insertOne(record)
		return id, true, err
	}
	record.ID = matches[0]
	if err := e.updateOne(record); err != nil {
		return "", false, err
	}
	return record.ID, false, nil
}

func (e *Engine) validateUpsertIDReference(record storage.Record) error {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return err
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return err
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return err
	}
	storedID, existing, ok := storage.LookupRecordByID(object.Records, record.ID)
	if !ok || existing.System.IsDeleted || storedID == "" {
		return dmlErrorf("INVALID_CROSS_REFERENCE_KEY", []string{"Id"}, "invalid cross reference id")
	}
	return nil
}

func (e *Engine) object(name string) (storage.ObjectState, string, error) {
	objectName, ok := storage.ResolveObjectName(*e.Org, name)
	if !ok {
		return storage.ObjectState{}, "", fmt.Errorf("dml: unknown object %s", name)
	}
	object := e.Org.Objects[objectName]
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	return object, objectName, nil
}

func canonicalizeRecord(namespace string, definition storage.ObjectDefinition, objectName string, record storage.Record) (storage.Record, error) {
	record.Object = objectName
	fields := make(map[string]storage.Value, len(record.Fields))
	for field, value := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			fields[field] = value
			continue
		}
		if _, exists := fields[canonical]; exists && canonical != field {
			return storage.Record{}, fmt.Errorf("dml: duplicate field alias %s.%s", objectName, field)
		}
		fields[canonical] = normalizeStoredFieldValue(definition.Fields[canonical], value)
	}
	record.Fields = fields
	nulls := make(map[string]bool, len(record.ExplicitNulls))
	for field, value := range record.ExplicitNulls {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			nulls[field] = value
			continue
		}
		nulls[canonical] = value
	}
	record.ExplicitNulls = nulls
	return record, nil
}

func normalizeStoredFieldValue(field storage.Field, value storage.Value) storage.Value {
	if value.Kind != storage.ValueString || !isSingleLineTextField(field) {
		return value
	}
	value.String = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(value.String)
	return value
}

func isSingleLineTextField(field storage.Field) bool {
	if field.Type != storage.FieldString && field.Type != storage.FieldPicklist {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
	case "TEXTAREA", "LONGTEXTAREA", "RICHTEXTAREA":
		return false
	default:
		return true
	}
}

func validateFields(definition storage.ObjectDefinition, namespace string, record storage.Record) error {
	for field := range record.Fields {
		if field == "Id" {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			return fmt.Errorf("dml: unknown field %s.%s", record.Object, field)
		}
		if definition.Fields[canonical].Type == storage.FieldCalculated || definition.Fields[canonical].Type == storage.FieldSummary {
			return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{canonical}, "dml: field %s.%s is not writeable", record.Object, canonical)
		}
	}
	for field := range record.ExplicitNulls {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			return fmt.Errorf("dml: unknown field %s.%s", record.Object, field)
		}
		if definition.Fields[canonical].Type == storage.FieldCalculated || definition.Fields[canonical].Type == storage.FieldSummary {
			return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{canonical}, "dml: field %s.%s is not writeable", record.Object, canonical)
		}
	}
	return nil
}

func validateFieldWriteability(definition storage.ObjectDefinition, namespace string, record storage.Record, create bool) error {
	for field := range record.Fields {
		if err := validateFieldWriteabilityName(definition, namespace, record.Object, field, create); err != nil {
			return err
		}
	}
	for field := range record.ExplicitNulls {
		if err := validateFieldWriteabilityName(definition, namespace, record.Object, field, create); err != nil {
			return err
		}
	}
	return nil
}

func stripImplicitReadOnlyDefaultFields(definition storage.ObjectDefinition, namespace string, record *storage.Record, create bool) {
	if record == nil {
		return
	}
	for field, value := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			continue
		}
		fieldDef := definition.Fields[canonical]
		writeable := storage.FieldFlagValue(fieldDef.Updateable, true)
		if create {
			writeable = storage.FieldFlagValue(fieldDef.Createable, true)
		}
		if writeable || !storageFieldValueLooksImplicit(fieldDef, value) {
			continue
		}
		delete(record.Fields, field)
	}
}

func storageFieldValueLooksImplicit(field storage.Field, value storage.Value) bool {
	if storageValueIsDefaultZero(value) {
		return true
	}
	return storageValueMatchesDefault(field, value)
}

func storageValueMatchesDefault(field storage.Field, value storage.Value) bool {
	defaultValue := strings.TrimSpace(field.DefaultValue)
	if defaultValue == "" {
		return false
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueID:
		actual := value.String
		if value.Kind == storage.ValueID {
			actual = string(value.ID)
		}
		return strings.EqualFold(actual, strings.Trim(defaultValue, `'"`))
	case storage.ValueBoolean:
		switch strings.ToLower(defaultValue) {
		case "true":
			return value.Boolean
		case "false":
			return !value.Boolean
		default:
			return false
		}
	case storage.ValueInteger:
		return strconv.FormatInt(value.Integer, 10) == defaultValue
	case storage.ValueDecimal:
		return value.Decimal == defaultValue
	default:
		return false
	}
}

func storageValueIsDefaultZero(value storage.Value) bool {
	switch value.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueBoolean:
		return !value.Boolean
	case storage.ValueInteger:
		return value.Integer == 0
	case storage.ValueDecimal:
		return value.Decimal == "" || value.Decimal == "0" || value.Decimal == "0.0"
	case storage.ValueString:
		return value.String == ""
	default:
		return false
	}
}

func stripMissingGeneratedRecordTypeID(org *storage.OrgState, record *storage.Record) {
	if org == nil || record == nil || record.Fields == nil {
		return
	}
	value, ok := record.Fields["RecordTypeId"]
	if !ok {
		return
	}
	recordTypeID := ""
	switch value.Kind {
	case storage.ValueID:
		recordTypeID = string(value.ID)
	case storage.ValueString:
		recordTypeID = value.String
	default:
		return
	}
	if recordTypeID != "012000000000000AAA" {
		return
	}
	recordTypes, ok := org.Objects["RecordType"]
	if ok {
		if _, exists := recordTypes.Records[storage.ID(recordTypeID)]; exists {
			return
		}
	}
	delete(record.Fields, "RecordTypeId")
}

func validateFieldWriteabilityName(definition storage.ObjectDefinition, namespace, objectName, field string, create bool) error {
	if field == "Id" {
		return nil
	}
	canonical, ok := storage.ResolveFieldName(definition, namespace, field)
	if !ok {
		return nil
	}
	fieldDef := definition.Fields[canonical]
	if fieldDef.Type == storage.FieldCalculated || fieldDef.Type == storage.FieldSummary {
		return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{canonical}, "dml: field %s.%s is not writeable", objectName, canonical)
	}
	writeable := storage.FieldFlagValue(fieldDef.Updateable, true)
	if create {
		writeable = storage.FieldFlagValue(fieldDef.Createable, true)
	}
	if !writeable {
		if allowLocalWriteabilityOverride(definition, objectName, canonical, fieldDef, create) {
			return nil
		}
		return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{canonical}, "dml: field %s.%s is not writeable", objectName, canonical)
	}
	return nil
}

func allowLocalWriteabilityOverride(definition storage.ObjectDefinition, objectName, field string, fieldDef storage.Field, create bool) bool {
	if strings.EqualFold(objectName, "Account") && strings.EqualFold(field, "IsPersonAccount") {
		return true
	}
	if strings.EqualFold(field, "Name") && (strings.EqualFold(objectName, "Contact") || strings.EqualFold(objectName, "Lead")) {
		return true
	}
	if !create {
		return false
	}
	if allowLocalCreateRelationshipField(definition, field, fieldDef) {
		return true
	}
	if allowLocalCreateConfigurationField(definition, field, fieldDef) {
		return true
	}
	return false
}

func allowLocalCreateRelationshipField(definition storage.ObjectDefinition, field string, fieldDef storage.Field) bool {
	if fieldDef.Type != storage.FieldReference || isSystemManagedReadonlyField(field) {
		return false
	}
	if fieldDef.DefaultedOnCreate != nil && *fieldDef.DefaultedOnCreate {
		return false
	}
	if fieldDef.Required && isLocalCreateIdentityObject(definition) {
		return true
	}
	if isLocalSetupConfigurationObject(definition) && strings.HasSuffix(strings.ToLower(field), "id") {
		return true
	}
	return false
}

func isLocalCreateIdentityObject(definition storage.ObjectDefinition) bool {
	if isLocalSetupConfigurationObject(definition) {
		return true
	}
	requiredReferences := 0
	for _, field := range definition.Fields {
		if field.Required && field.Type == storage.FieldReference && field.RelationshipName != "" {
			requiredReferences++
		}
	}
	return requiredReferences >= 2
}

func allowLocalCreateConfigurationField(definition storage.ObjectDefinition, field string, fieldDef storage.Field) bool {
	if fieldDef.Type != storage.FieldString && fieldDef.Type != storage.FieldPicklist {
		return false
	}
	if fieldDef.Required {
		return true
	}
	if strings.EqualFold(field, "Type") && isLocalDeveloperNamedSetupObject(definition) {
		return true
	}
	return isLocalSetupConfigurationObject(definition) && strings.HasSuffix(strings.ToLower(field), "type")
}

func isLocalDeveloperNamedSetupObject(definition storage.ObjectDefinition) bool {
	if _, ok := fieldByName(definition, "DeveloperName"); !ok {
		return false
	}
	if _, ok := fieldByName(definition, "RelatedId"); ok {
		return true
	}
	if _, ok := fieldByName(definition, "SetupEntityId"); ok {
		return true
	}
	return false
}

func isLocalSetupConfigurationObject(definition storage.ObjectDefinition) bool {
	if _, ok := fieldByName(definition, "ParentId"); !ok {
		return false
	}
	for _, name := range []string{"SObjectType", "Field", "SetupEntityId", "SetupEntityType"} {
		if _, ok := fieldByName(definition, name); ok {
			return true
		}
	}
	return false
}

func fieldByName(definition storage.ObjectDefinition, name string) (storage.Field, bool) {
	for fieldName, field := range definition.Fields {
		if strings.EqualFold(fieldName, name) {
			return field, true
		}
	}
	return storage.Field{}, false
}

func isSystemManagedReadonlyField(field string) bool {
	switch strings.ToLower(field) {
	case "id", "isdeleted", "createddate", "createdbyid", "lastmodifieddate", "lastmodifiedbyid", "systemmodstamp", "lastvieweddate", "lastreferenceddate":
		return true
	default:
		return false
	}
}

func validateRequired(definition storage.ObjectDefinition, record storage.Record) error {
	for name, field := range definition.Fields {
		if !isDMLRequiredField(field) {
			continue
		}
		if value, ok := record.GetField(name); ok {
			if field.Type == storage.FieldString && strings.TrimSpace(value.String) == "" {
				return dmlErrorf("REQUIRED_FIELD_MISSING", []string{name}, "dml: missing required field %s.%s", record.Object, name)
			}
			continue
		}
		if record.HasExplicitNull(name) {
			return dmlErrorf("REQUIRED_FIELD_MISSING", []string{name}, "dml: required field %s.%s is null", record.Object, name)
		}
		return dmlErrorf("REQUIRED_FIELD_MISSING", []string{name}, "dml: missing required field %s.%s", record.Object, name)
	}
	return nil
}

func isDMLRequiredField(field storage.Field) bool {
	return field.Required
}

func stripReadOnlyUpdateFields(definition storage.ObjectDefinition, namespace string, record *storage.Record) {
	if record == nil {
		return
	}
	for field := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			continue
		}
		fieldDef := definition.Fields[canonical]
		if fieldDef.Type != storage.FieldCalculated && fieldDef.Type != storage.FieldSummary {
			continue
		}
		delete(record.Fields, field)
	}
}

func stripUnchangedNonUpdateableFields(definition storage.ObjectDefinition, namespace string, record *storage.Record, existing storage.Record) {
	if record == nil {
		return
	}
	for field, value := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			continue
		}
		fieldDef := definition.Fields[canonical]
		if fieldDef.Type != storage.FieldCalculated && fieldDef.Type != storage.FieldSummary && storage.FieldFlagValue(fieldDef.Updateable, true) {
			continue
		}
		existingValue, ok := existing.GetField(canonical)
		if ok && storageValuesEqual(fieldDef, value, existingValue) {
			delete(record.Fields, field)
		}
	}
	for field := range record.ExplicitNulls {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			continue
		}
		fieldDef := definition.Fields[canonical]
		if fieldDef.Type != storage.FieldCalculated && fieldDef.Type != storage.FieldSummary && storage.FieldFlagValue(fieldDef.Updateable, true) {
			continue
		}
		existingValue, ok := existing.GetField(canonical)
		if ok && existingValue.Kind == storage.ValueNull {
			delete(record.ExplicitNulls, field)
		}
	}
}

func (e *Engine) validateObjectID(definition storage.ObjectDefinition, record storage.Record) error {
	if record.ID == "" || definition.KeyPrefix == "" {
		return nil
	}
	if !strings.HasPrefix(string(record.ID), definition.KeyPrefix) {
		return dmlErrorf("INVALID_FIELD", []string{"Id"}, "dml: id %s does not belong to %s", record.ID, record.Object)
	}
	return nil
}

func (e *Engine) validateReferences(definition storage.ObjectDefinition, record storage.Record) error {
	for name, field := range definition.Fields {
		if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		value, ok := record.GetField(name)
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		id := idFromStorageValue(value)
		if id == "" {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{name}, "dml: invalid reference %s.%s", record.Object, name)
		}
		found := false
		for _, targetName := range field.ReferenceTo {
			canonical, ok := storage.ResolveObjectName(*e.Org, targetName)
			if !ok {
				continue
			}
			target := e.Org.Objects[canonical]
			_, parent, ok := storage.LookupRecordByID(target.Records, id)
			if ok && !parent.System.IsDeleted {
				found = true
				break
			}
		}
		if !found && isPolymorphicReference(definition, name) {
			found = e.referenceExistsInAnyObject(id)
		}
		if !found && allowMissingLocalReference(definition, name) {
			continue
		}
		if !found {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{name}, "dml: reference %s.%s points to missing record %s", record.Object, name, id)
		}
	}
	return nil
}

func allowMissingLocalReference(definition storage.ObjectDefinition, fieldName string) bool {
	field, ok := fieldByName(definition, fieldName)
	return ok && field.Type == storage.FieldReference && isLocalSetupConfigurationObject(definition) && strings.EqualFold(fieldName, "SetupEntityId")
}

func isPolymorphicReference(definition storage.ObjectDefinition, fieldName string) bool {
	for _, relationship := range definition.Relations {
		if strings.EqualFold(relationship.Field, fieldName) && relationship.Polymorphic {
			return true
		}
	}
	return strings.EqualFold(fieldName, "WhatId") || strings.EqualFold(fieldName, "WhoId")
}

func (e *Engine) referenceExistsInAnyObject(id storage.ID) bool {
	for _, object := range e.Org.Objects {
		record, ok := object.Records[id]
		if ok && !record.System.IsDeleted {
			return true
		}
	}
	return false
}

func (e *Engine) validateUnique(objectName string, definition storage.ObjectDefinition, record storage.Record, currentID storage.ID) error {
	for fieldName, field := range definition.Fields {
		if !field.Unique {
			continue
		}
		value, ok := record.Fields[fieldName]
		if !ok || value.Kind == storage.ValueNull {
			continue
		}
		object := e.Org.Objects[objectName]
		for id, stored := range object.Records {
			if id == currentID || stored.System.IsDeleted {
				continue
			}
			storedValue, ok := stored.Fields[fieldName]
			if !ok {
				continue
			}
			if storageValuesEqual(field, storedValue, value) {
				return dmlErrorf("DUPLICATE_VALUE", []string{fieldName}, "dml: duplicate value %s.%s", objectName, fieldName)
			}
		}
	}
	return nil
}

func (e *Engine) validateValidationRules(definition storage.ObjectDefinition, record storage.Record) error {
	for _, rule := range definition.ValidationRules {
		if !rule.Active {
			continue
		}
		matches, ok := evaluateValidationFormulaInOrg(rule.ErrorConditionFormula, e.Org, definition, record)
		if !ok || !matches {
			continue
		}
		message := rule.ErrorMessage
		if message == "" {
			message = fmt.Sprintf("dml: validation rule %s failed", rule.Name)
		}
		fields := []string(nil)
		if rule.ErrorDisplayField != "" {
			fields = []string{rule.ErrorDisplayField}
		}
		return dmlErrorf("FIELD_CUSTOM_VALIDATION_EXCEPTION", fields, "%s", message)
	}
	return nil
}

func evaluateValidationFormula(formula string, record storage.Record) (bool, bool) {
	return evaluateRecordFormula(formula, record)
}

func evaluateValidationFormulaInOrg(formula string, org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record) (bool, bool) {
	return evaluateRecordFormulaInOrg(formula, org, definition, record)
}

func trimFormulaLiteral(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		quote := value[0]
		if (quote == '\'' || quote == '"') && value[len(value)-1] == quote {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func (e *Engine) applyWorkflowFieldUpdates(objectName string, id storage.ID) error {
	if e.workflowDepth > 8 {
		return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: workflow field update recursion limit exceeded")
	}
	e.workflowDepth++
	defer func() {
		e.workflowDepth--
	}()

	object := e.Org.Objects[objectName]
	if len(object.Definition.WorkflowRules) == 0 {
		return nil
	}
	record, ok := object.Records[id]
	if !ok || record.System.IsDeleted {
		return nil
	}
	changed := false
	record = record.Clone()
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if record.ExplicitNulls == nil {
		record.ExplicitNulls = make(map[string]bool)
	}
	for _, rule := range object.Definition.WorkflowRules {
		if !rule.Active {
			continue
		}
		matches, ok := evaluateWorkflowRule(rule, record, object.Definition, e.Org.Namespace)
		if !ok || !matches {
			continue
		}
		for _, update := range rule.FieldUpdates {
			fieldName, ok := storage.ResolveFieldName(object.Definition, e.Org.Namespace, update.Field)
			if !ok || fieldName == "Id" {
				return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{update.Field}, "dml: workflow field update %s targets unknown or read-only field %s.%s", update.Name, objectName, update.Field)
			}
			if object.Definition.Fields[fieldName].Type == storage.FieldCalculated {
				return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: workflow field update %s targets calculated field %s.%s", update.Name, objectName, fieldName)
			}
			value, explicitNull, ok := workflowUpdateValue(object.Definition.Fields[fieldName], record, update, object.Definition, e.Org.Namespace)
			if !ok {
				return dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: workflow field update %s has unsupported value expression", update.Name)
			}
			if explicitNull {
				delete(record.Fields, fieldName)
				record.ExplicitNulls[fieldName] = true
			} else {
				record.Fields[fieldName] = value
				delete(record.ExplicitNulls, fieldName)
			}
			changed = true
		}
		for _, alert := range rule.EmailAlerts {
			if e.WorkflowEmailer == nil {
				continue
			}
			if err := e.WorkflowEmailer(alert, record); err != nil {
				return err
			}
		}
	}
	if !changed {
		return nil
	}
	if err := validateRequired(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateValidationRules(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateUnique(objectName, object.Definition, record, record.ID); err != nil {
		return err
	}
	stamp := e.systemTimestamp()
	record.System.LastModifiedDate = stamp
	record.System.SystemModstamp = stamp
	record.System.LastModifiedByID = e.systemUserID()
	object.Records[id] = record
	e.Org.Objects[objectName] = object
	return nil
}

func (e *Engine) ApplyAutomation(objectName string, id storage.ID) error {
	if err := e.applyWorkflowFieldUpdates(objectName, id); err != nil {
		return err
	}
	if err := e.applyFlowFieldUpdates(objectName, id); err != nil {
		return err
	}
	return nil
}

func hasObjectAutomation(definition storage.ObjectDefinition) bool {
	return len(definition.WorkflowRules) > 0 || len(definition.FlowRules) > 0
}

func (e *Engine) applyFlowFieldUpdates(objectName string, id storage.ID) error {
	if e.flowDepth > 8 {
		return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow field update recursion limit exceeded")
	}
	e.flowDepth++
	defer func() {
		e.flowDepth--
	}()

	object := e.Org.Objects[objectName]
	if len(object.Definition.FlowRules) == 0 {
		return nil
	}
	record, ok := object.Records[id]
	if !ok || record.System.IsDeleted {
		return nil
	}
	changed := false
	record = record.Clone()
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if record.ExplicitNulls == nil {
		record.ExplicitNulls = make(map[string]bool)
	}
	for _, rule := range object.Definition.FlowRules {
		if !rule.Active {
			continue
		}
		matches, ok := evaluateFlowRule(rule, record, object.Definition, e.Org.Namespace)
		e.traceAutomation("apex.flow.rule", map[string]any{
			"flow":    rule.Name,
			"object":  objectName,
			"record":  string(record.ID),
			"matched": ok && matches,
			"modeled": ok,
		})
		if !ok || !matches {
			continue
		}
		if len(rule.Branches) > 0 {
			branch, matched := e.selectFlowBranch(rule, record, object.Definition)
			e.traceAutomation("apex.flow.decision", map[string]any{
				"flow":    rule.Name,
				"object":  objectName,
				"record":  string(record.ID),
				"branch":  branch.Name,
				"default": branch.Default,
				"matched": matched,
			})
			if !matched {
				continue
			}
			branchLookups := append([]storage.FlowRecordLookup(nil), rule.RecordLookups...)
			branchLookups = append(branchLookups, branch.RecordLookups...)
			branchChanged, err := e.applyFlowEffects(rule.Name, objectName, &record, object.Definition, branch.FieldUpdates, branch.Actions, branchLookups, branch.RecordCreates)
			if err != nil {
				return err
			}
			changed = changed || branchChanged
			continue
		}
		ruleChanged, err := e.applyFlowEffects(rule.Name, objectName, &record, object.Definition, rule.FieldUpdates, rule.Actions, rule.RecordLookups, rule.RecordCreates)
		if err != nil {
			return err
		}
		changed = changed || ruleChanged
	}
	if !changed {
		return nil
	}
	if err := validateRequired(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateValidationRules(object.Definition, record); err != nil {
		return err
	}
	if err := e.validateUnique(objectName, object.Definition, record, record.ID); err != nil {
		return err
	}
	stamp := e.systemTimestamp()
	record.System.LastModifiedDate = stamp
	record.System.SystemModstamp = stamp
	record.System.LastModifiedByID = e.systemUserID()
	object.Records[id] = record
	e.Org.Objects[objectName] = object
	return nil
}

func (e *Engine) selectFlowBranch(rule storage.FlowRule, record storage.Record, definition storage.ObjectDefinition) (storage.FlowBranch, bool) {
	var defaultBranch storage.FlowBranch
	hasDefault := false
	for _, branch := range rule.Branches {
		if branch.Default {
			if !hasDefault {
				defaultBranch = branch
				hasDefault = true
			}
			continue
		}
		matches, ok := evaluateFlowBranch(branch, record, definition, e.Org.Namespace)
		if ok && matches {
			return branch, true
		}
	}
	if hasDefault {
		return defaultBranch, true
	}
	return storage.FlowBranch{}, false
}

func evaluateFlowBranch(branch storage.FlowBranch, record storage.Record, definition storage.ObjectDefinition, namespace string) (bool, bool) {
	if len(branch.Criteria) == 0 {
		return true, true
	}
	for _, item := range branch.Criteria {
		matches, ok := evaluateWorkflowCriteria(item, record, definition, namespace)
		if !ok || !matches {
			return matches, ok
		}
	}
	return true, true
}

func (e *Engine) applyFlowEffects(flowName, objectName string, record *storage.Record, definition storage.ObjectDefinition, updates []storage.WorkflowFieldUpdate, actions []storage.FlowAction, lookups []storage.FlowRecordLookup, creates []storage.FlowRecordCreate) (bool, error) {
	changed := false
	for _, update := range updates {
		fieldName, ok := storage.ResolveFieldName(definition, e.Org.Namespace, update.Field)
		if !ok || fieldName == "Id" {
			return false, dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{update.Field}, "dml: flow field update %s targets unknown or read-only field %s.%s", update.Name, objectName, update.Field)
		}
		if definition.Fields[fieldName].Type == storage.FieldCalculated {
			return false, dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: flow field update %s targets calculated field %s.%s", update.Name, objectName, fieldName)
		}
		value, explicitNull, ok := workflowUpdateValue(definition.Fields[fieldName], *record, update, definition, e.Org.Namespace)
		if !ok {
			return false, dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: flow field update %s has unsupported value expression", update.Name)
		}
		if explicitNull {
			delete(record.Fields, fieldName)
			record.ExplicitNulls[fieldName] = true
		} else {
			record.Fields[fieldName] = value
			delete(record.ExplicitNulls, fieldName)
		}
		e.traceAutomation("apex.flow.field_update", map[string]any{
			"flow":         flowName,
			"update":       update.Name,
			"object":       objectName,
			"record":       string(record.ID),
			"field":        fieldName,
			"value":        workflowValueString(value),
			"explicitNull": explicitNull,
		})
		changed = true
	}
	for _, action := range actions {
		e.traceAutomation("apex.flow.action", map[string]any{
			"flow":   flowName,
			"action": action.Name,
			"object": objectName,
			"record": string(record.ID),
		})
		if e.FlowActionInvoker == nil {
			return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s requires Apex action execution support", action.Name)
		}
		if err := e.FlowActionInvoker(action, record.Clone()); err != nil {
			return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s failed: %v", action.Name, err)
		}
	}
	for _, create := range creates {
		suppressed, err := e.flowRecordCreateSuppressedByLookup(create, lookups, *record, definition)
		if err != nil {
			return changed, err
		}
		if suppressed {
			e.traceAutomation("apex.flow.record_create_suppressed", map[string]any{
				"flow":   flowName,
				"create": create.Name,
				"object": create.ObjectName,
				"record": string(record.ID),
			})
			continue
		}
		createdID, err := e.executeFlowRecordCreate(create, *record, definition)
		if err != nil {
			return changed, err
		}
		e.traceAutomation("apex.flow.record_create", map[string]any{
			"flow":      flowName,
			"create":    create.Name,
			"object":    create.ObjectName,
			"sourceId":  string(record.ID),
			"createdId": string(createdID),
		})
	}
	return changed, nil
}

func (e *Engine) executeFlowRecordCreate(create storage.FlowRecordCreate, source storage.Record, sourceDefinition storage.ObjectDefinition) (storage.ID, error) {
	target, targetName, err := e.object(create.ObjectName)
	if err != nil {
		return "", dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record create %s targets unknown object %s", create.Name, create.ObjectName)
	}
	record := storage.Record{
		Object:        targetName,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
	for _, assignment := range create.InputAssignments {
		fieldName, ok := storage.ResolveFieldName(target.Definition, e.Org.Namespace, assignment.Field)
		if !ok || fieldName == "Id" {
			return "", dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{assignment.Field}, "dml: flow record create %s targets unknown or read-only field %s.%s", create.Name, targetName, assignment.Field)
		}
		field := target.Definition.Fields[fieldName]
		if field.Type == storage.FieldCalculated {
			return "", dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: flow record create %s targets calculated field %s.%s", create.Name, targetName, fieldName)
		}
		value, explicitNull, ok := flowRecordCreateAssignmentValue(field, source, assignment, sourceDefinition, e.Org.Namespace)
		if !ok {
			return "", dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: flow record create %s has unsupported value expression for %s.%s", create.Name, targetName, fieldName)
		}
		if explicitNull {
			record.ExplicitNulls[fieldName] = true
			delete(record.Fields, fieldName)
		} else {
			record.Fields[fieldName] = value
			delete(record.ExplicitNulls, fieldName)
		}
	}
	createdID, err := e.insertOne(record)
	if err != nil {
		return "", dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record create %s failed: %v", create.Name, err)
	}
	return createdID, nil
}

func flowRecordCreateAssignmentValue(field storage.Field, source storage.Record, assignment storage.WorkflowFieldUpdate, sourceDefinition storage.ObjectDefinition, namespace string) (storage.Value, bool, bool) {
	if assignment.SourceField != "" {
		sourceField, ok := storage.ResolveFieldName(sourceDefinition, namespace, assignment.SourceField)
		if !ok {
			return storage.Value{}, false, false
		}
		value, ok := sourceRecordFieldValue(source, sourceField)
		if !ok {
			return storage.NullValue(), true, true
		}
		return value.Clone(), false, true
	}
	return workflowUpdateValue(field, source, assignment, sourceDefinition, namespace)
}

func (e *Engine) flowRecordCreateSuppressedByLookup(create storage.FlowRecordCreate, lookups []storage.FlowRecordLookup, source storage.Record, sourceDefinition storage.ObjectDefinition) (bool, error) {
	createObject, ok := storage.ResolveObjectName(*e.Org, create.ObjectName)
	if !ok {
		return false, nil
	}
	for _, lookup := range lookups {
		lookupObject, ok := storage.ResolveObjectName(*e.Org, lookup.ObjectName)
		if !ok {
			return false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record lookup %s targets unknown object %s", lookup.Name, lookup.ObjectName)
		}
		if lookupObject != createObject {
			continue
		}
		matches, err := e.flowRecordLookupMatches(lookup, source, sourceDefinition)
		if err != nil {
			return false, err
		}
		e.traceAutomation("apex.flow.record_lookup", map[string]any{
			"lookup":  lookup.Name,
			"object":  lookup.ObjectName,
			"source":  string(source.ID),
			"matched": matches,
		})
		if matches {
			return true, nil
		}
	}
	return false, nil
}

func (e *Engine) traceAutomation(name string, args map[string]any) {
	if e.AutomationTracer != nil {
		e.AutomationTracer(name, args)
	}
}

func (e *Engine) flowRecordLookupMatches(lookup storage.FlowRecordLookup, source storage.Record, sourceDefinition storage.ObjectDefinition) (bool, error) {
	target, targetName, err := e.object(lookup.ObjectName)
	if err != nil {
		return false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record lookup %s targets unknown object %s", lookup.Name, lookup.ObjectName)
	}
	for _, candidate := range target.Records {
		if candidate.System.IsDeleted {
			continue
		}
		matches := true
		for _, item := range lookup.Criteria {
			match, ok := e.evaluateFlowLookupCriteria(item, candidate, target.Definition, source, sourceDefinition)
			if !ok {
				return false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", []string{item.Field}, "dml: flow record lookup %s has unsupported criteria for %s.%s", lookup.Name, targetName, item.Field)
			}
			if !match {
				matches = false
				break
			}
		}
		if matches {
			return true, nil
		}
		if lookup.GetFirstRecordOnly {
			continue
		}
	}
	return false, nil
}

func (e *Engine) evaluateFlowLookupCriteria(item storage.WorkflowCriteriaItem, target storage.Record, targetDefinition storage.ObjectDefinition, source storage.Record, sourceDefinition storage.ObjectDefinition) (bool, bool) {
	if strings.TrimSpace(item.SourceField) == "" {
		return evaluateWorkflowCriteria(item, target, targetDefinition, e.Org.Namespace)
	}
	targetField, ok := storage.ResolveFieldName(targetDefinition, e.Org.Namespace, strings.TrimSpace(item.Field))
	if !ok || targetField == "" {
		return false, false
	}
	sourceField, ok := storage.ResolveFieldName(sourceDefinition, e.Org.Namespace, strings.TrimSpace(item.SourceField))
	if !ok || sourceField == "" {
		return false, false
	}
	targetValue, targetOK := target.Fields[targetField]
	sourceValue, sourceOK := sourceRecordFieldValue(source, sourceField)
	if !targetOK {
		targetValue = storage.NullValue()
	}
	if !sourceOK {
		sourceValue = storage.NullValue()
	}
	field := targetDefinition.Fields[targetField]
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "", "equals", "equal", "eq":
		return storageValuesEqual(field, targetValue, sourceValue), true
	case "notequal", "not equal", "notequals", "not equals", "ne":
		return !storageValuesEqual(field, targetValue, sourceValue), true
	case "contains":
		return strings.Contains(workflowValueString(targetValue), workflowValueString(sourceValue)), true
	case "notcontain", "doesnotcontain":
		return !strings.Contains(workflowValueString(targetValue), workflowValueString(sourceValue)), true
	case "isnull", "isblank":
		return targetValue.Kind == storage.ValueNull, true
	default:
		return false, false
	}
}

func sourceRecordFieldValue(record storage.Record, field string) (storage.Value, bool) {
	if strings.EqualFold(field, "Id") {
		if record.ID == "" {
			return storage.Value{}, false
		}
		return storage.IDValue(record.ID), true
	}
	value, ok := record.Fields[field]
	return value, ok
}

func evaluateWorkflowRule(rule storage.WorkflowRule, record storage.Record, definition storage.ObjectDefinition, namespace string) (bool, bool) {
	if strings.TrimSpace(rule.Formula) != "" {
		return evaluateValidationFormula(rule.Formula, record)
	}
	if len(rule.Criteria) == 0 {
		return true, true
	}
	for _, item := range rule.Criteria {
		matches, ok := evaluateWorkflowCriteria(item, record, definition, namespace)
		if !ok || !matches {
			return matches, ok
		}
	}
	return true, true
}

func evaluateFlowRule(rule storage.FlowRule, record storage.Record, definition storage.ObjectDefinition, namespace string) (bool, bool) {
	if strings.TrimSpace(rule.Formula) != "" {
		return evaluateValidationFormula(rule.Formula, record)
	}
	if len(rule.Criteria) == 0 {
		return true, true
	}
	for _, item := range rule.Criteria {
		matches, ok := evaluateWorkflowCriteria(item, record, definition, namespace)
		if !ok || !matches {
			return matches, ok
		}
	}
	return true, true
}

func evaluateWorkflowCriteria(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition, namespace string) (bool, bool) {
	field, ok := storage.ResolveFieldName(definition, namespace, strings.TrimSpace(item.Field))
	if !ok || field == "" {
		return false, false
	}
	want := trimFormulaLiteral(item.Value)
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "", "equals", "equal", "eq":
		return validationFieldEquals(record, field, want), true
	case "notequal", "not equal", "notequals", "not equals", "ne":
		return !validationFieldEquals(record, field, want), true
	case "greaterthan", "greater than", "gt":
		return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, ">"), true
	case "greaterthanorequalto", "greater than or equal", "greater than or equal to", "gte", "ge":
		return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, ">="), true
	case "lessthan", "less than", "lt":
		return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, "<"), true
	case "lessthanorequalto", "less than or equal", "less than or equal to", "lte", "le":
		return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, "<="), true
	case "contains":
		value, ok := record.Fields[field]
		return ok && strings.Contains(workflowValueString(value), want), true
	case "notcontain", "doesnotcontain":
		value, ok := record.Fields[field]
		return !ok || !strings.Contains(workflowValueString(value), want), true
	case "isnull", "isblank":
		return validationFieldBlank(record, field), true
	default:
		return false, false
	}
}

func workflowUpdateValue(field storage.Field, record storage.Record, update storage.WorkflowFieldUpdate, definition storage.ObjectDefinition, namespace string) (storage.Value, bool, bool) {
	switch {
	case update.SourceField != "":
		sourceField, ok := storage.ResolveFieldName(definition, namespace, update.SourceField)
		if !ok {
			return storage.Value{}, false, false
		}
		value, ok := record.Fields[sourceField]
		if !ok {
			return storage.NullValue(), true, true
		}
		return value.Clone(), false, true
	case update.Formula != "":
		return workflowExpressionValue(field, record, update.Formula, definition, namespace)
	default:
		return workflowLiteralValue(field, update.LiteralValue)
	}
}

func workflowExpressionValue(field storage.Field, record storage.Record, expression string, definition storage.ObjectDefinition, namespace string) (storage.Value, bool, bool) {
	expression = strings.TrimSpace(expression)
	if expression == "" || strings.EqualFold(expression, "NULL") {
		return storage.NullValue(), true, true
	}
	if fieldName, ok := storage.ResolveFieldName(definition, namespace, expression); ok {
		if value, ok := record.Fields[fieldName]; ok {
			return value.Clone(), false, true
		}
		return storage.NullValue(), true, true
	}
	if value, explicitNull, ok := evaluateRecordFormulaValue(expression, field, record); ok {
		return value, explicitNull, true
	}
	return workflowLiteralValue(field, trimFormulaLiteral(expression))
}

func workflowLiteralValue(field storage.Field, literal string) (storage.Value, bool, bool) {
	literal = strings.TrimSpace(literal)
	if strings.EqualFold(literal, "NULL") {
		return storage.NullValue(), true, true
	}
	switch field.Type {
	case storage.FieldCalculated:
		switch strings.ToUpper(field.DisplayType) {
		case "INTEGER":
			var value int64
			if _, err := fmt.Sscanf(literal, "%d", &value); err != nil {
				return storage.Value{}, false, false
			}
			return storage.IntegerValue(value), false, true
		case "DECIMAL", "DOUBLE", "CURRENCY", "PERCENT":
			return storage.DecimalValue(literal), false, true
		case "BOOLEAN":
			if strings.EqualFold(literal, "true") {
				return storage.BooleanValue(true), false, true
			}
			if strings.EqualFold(literal, "false") {
				return storage.BooleanValue(false), false, true
			}
			return storage.Value{}, false, false
		default:
			return storage.StringValue(literal), false, true
		}
	case storage.FieldBoolean:
		if strings.EqualFold(literal, "true") {
			return storage.BooleanValue(true), false, true
		}
		if strings.EqualFold(literal, "false") {
			return storage.BooleanValue(false), false, true
		}
		return storage.Value{}, false, false
	case storage.FieldInteger:
		var value int64
		if _, err := fmt.Sscanf(literal, "%d", &value); err != nil {
			return storage.Value{}, false, false
		}
		return storage.IntegerValue(value), false, true
	case storage.FieldDecimal:
		return storage.DecimalValue(literal), false, true
	case storage.FieldID, storage.FieldReference:
		return storage.IDValue(storage.ID(literal)), false, true
	case storage.FieldDate:
		return storage.DateValue(literal), false, true
	case storage.FieldDateTime:
		return storage.DateTimeValue(literal), false, true
	default:
		return storage.StringValue(literal), false, true
	}
}

func workflowValueString(value storage.Value) string {
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return value.String
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueInteger:
		return fmt.Sprintf("%d", value.Integer)
	case storage.ValueBoolean:
		return fmt.Sprintf("%t", value.Boolean)
	default:
		return ""
	}
}

func (e *Engine) validateDeleteReferences(objectName string, id storage.ID) error {
	for childObjectName, childObject := range e.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if !relation.RestrictedDelete || !containsString(relation.ParentObjects, objectName) {
				continue
			}
			for _, child := range childObject.Records {
				if child.System.IsDeleted {
					continue
				}
				value, ok := child.Fields[relation.Field]
				if ok && idFromStorageValue(value) == id {
					return dmlErrorf("DELETE_FAILED", []string{relation.Field}, "dml: cannot delete %s %s because %s records reference it", objectName, id, childObjectName)
				}
			}
		}
	}
	return nil
}

func (e *Engine) cascadeDeleteChildren(objectName string, id storage.ID, seen map[string]bool) error {
	for childObjectName, childObject := range e.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if !relation.CascadeDelete || !containsString(relation.ParentObjects, objectName) {
				continue
			}
			for childID, child := range childObject.Records {
				if child.System.IsDeleted {
					continue
				}
				value, ok := child.Fields[relation.Field]
				if ok && idFromStorageValue(value) == id {
					if err := e.deleteRecord(childObjectName, childID, seen); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func upsertExternalID(definition storage.ObjectDefinition, namespace string, record storage.Record, externalIDField string) (string, storage.Value, bool, error) {
	if externalIDField != "" {
		fieldName := externalIDField
		if canonical, ok := storage.ResolveFieldName(definition, namespace, fieldName); ok {
			fieldName = canonical
		}
		field, ok := definition.Fields[fieldName]
		if !ok {
			return "", storage.Value{}, false, dmlErrorf("INVALID_FIELD", []string{externalIDField}, "dml: unknown external id field %s.%s", record.Object, externalIDField)
		}
		if !field.ExternalID {
			return "", storage.Value{}, false, dmlErrorf("INVALID_FIELD", []string{fieldName}, "dml: field %s.%s is not an external id", record.Object, fieldName)
		}
		value, ok := record.Fields[fieldName]
		if !ok || value.Kind == storage.ValueNull {
			return fieldName, storage.Value{}, false, dmlErrorf("MISSING_ARGUMENT", []string{fieldName}, "dml: external id field %s.%s is missing", record.Object, fieldName)
		}
		return fieldName, value, true, nil
	}
	for name, field := range definition.Fields {
		if !field.ExternalID {
			continue
		}
		value, ok := record.Fields[name]
		if ok && value.Kind != storage.ValueNull {
			return name, value, true, nil
		}
	}
	return "", storage.Value{}, false, nil
}

func idFromStorageValue(value storage.Value) storage.ID {
	switch value.Kind {
	case storage.ValueID:
		return value.ID
	case storage.ValueString:
		return storage.ID(value.String)
	default:
		return ""
	}
}

func (e *Engine) recalculateSummaryFieldsForChildren(childObjectName string, childRecords ...storage.Record) {
	if e == nil || e.Org == nil || len(childRecords) == 0 {
		return
	}
	recordTriggerUpdates := len(childRecords) > 1
	canonicalChild, ok := storage.ResolveObjectName(*e.Org, childObjectName)
	if !ok {
		canonicalChild = childObjectName
	}
	for parentObjectName, parentObject := range e.Org.Objects {
		changed := false
		for parentFieldName, field := range parentObject.Definition.Fields {
			if field.Type != storage.FieldSummary {
				continue
			}
			operation := strings.ToLower(strings.TrimSpace(field.SummaryOperation))
			summaryChild, _ := splitSummaryQualifiedField(field.SummarizedField)
			fkChild, fkField := splitSummaryQualifiedField(field.SummaryForeignKey)
			if summaryChild == "" && operation == "count" {
				summaryChild = fkChild
			}
			if summaryChild == "" || fkChild == "" || fkField == "" || !strings.EqualFold(summaryChild, fkChild) {
				continue
			}
			resolvedSummaryChild, ok := storage.ResolveObjectName(*e.Org, summaryChild)
			if !ok {
				resolvedSummaryChild = summaryChild
			}
			if !strings.EqualFold(resolvedSummaryChild, canonicalChild) {
				continue
			}
			childObject := e.Org.Objects[canonicalChild]
			fkFieldName, ok := storage.ResolveFieldName(childObject.Definition, e.Org.Namespace, fkField)
			if !ok {
				continue
			}
			parentIDs := summaryParentIDs(childRecords, fkFieldName)
			for parentID := range parentIDs {
				storedParentID, parentRecord, ok := storage.LookupRecordByID(parentObject.Records, parentID)
				if !ok || parentRecord.System.IsDeleted {
					continue
				}
				value, ok := e.evaluateSummaryField(parentRecord, field)
				if !ok {
					continue
				}
				oldValue, ok := parentRecord.Fields[parentFieldName]
				if !ok {
					oldValue = storage.NullValue()
				}
				if storageValuesEqual(field, oldValue, value) {
					continue
				}
				beforeParent := parentRecord.Clone()
				if parentRecord.Fields == nil {
					parentRecord.Fields = make(map[string]storage.Value)
				}
				parentRecord.Fields[parentFieldName] = value
				parentObject.Records[storedParentID] = parentRecord
				if recordTriggerUpdates {
					e.recordSummaryUpdate(parentObjectName, beforeParent, parentRecord)
				}
				changed = true
			}
		}
		if changed {
			e.Org.Objects[parentObjectName] = parentObject
		}
	}
}

func (e *Engine) recordSummaryUpdate(objectName string, before, after storage.Record) {
	if e == nil || before.ID == "" || after.ID == "" {
		return
	}
	key := strings.ToLower(objectName) + "\x00" + strings.ToLower(string(after.ID))
	if e.summaryUpdates == nil {
		e.summaryUpdates = make(map[string]SummaryUpdate)
	}
	update, exists := e.summaryUpdates[key]
	if !exists {
		e.summaryOrder = append(e.summaryOrder, key)
		update = SummaryUpdate{Object: objectName, Before: before.Clone()}
	}
	update.After = after.Clone()
	e.summaryUpdates[key] = update
}

func summaryParentIDs(records []storage.Record, fkFieldName string) map[storage.ID]bool {
	ids := make(map[storage.ID]bool)
	for _, record := range records {
		if record.Fields == nil {
			continue
		}
		id := idFromStorageValue(record.Fields[fkFieldName])
		if id != "" {
			ids[id] = true
		}
	}
	return ids
}

func (e *Engine) evaluateSummaryField(parent storage.Record, field storage.Field) (storage.Value, bool) {
	childObject, childField := splitSummaryQualifiedField(field.SummarizedField)
	fkObject, fkField := splitSummaryQualifiedField(field.SummaryForeignKey)
	operation := strings.ToLower(strings.TrimSpace(field.SummaryOperation))
	if childObject == "" && operation == "count" {
		childObject = fkObject
	}
	if e == nil || e.Org == nil || parent.ID == "" || childObject == "" || fkObject == "" || fkField == "" || !strings.EqualFold(childObject, fkObject) {
		return storage.Value{}, false
	}
	canonicalChild, ok := storage.ResolveObjectName(*e.Org, childObject)
	if !ok {
		return storage.Value{}, false
	}
	childState := e.Org.Objects[canonicalChild]
	childFieldName := ""
	if childField != "" {
		var ok bool
		childFieldName, ok = storage.ResolveFieldName(childState.Definition, e.Org.Namespace, childField)
		if !ok {
			return storage.Value{}, false
		}
	}
	fkFieldName, ok := storage.ResolveFieldName(childState.Definition, e.Org.Namespace, fkField)
	if !ok {
		return storage.Value{}, false
	}
	values := make([]float64, 0)
	count := int64(0)
	for _, child := range childState.Records {
		if child.System.IsDeleted || !storage.IDsEqual(idFromStorageValue(child.Fields[fkFieldName]), parent.ID) {
			continue
		}
		if !summaryFiltersMatch(e.Org, childState.Definition, child, field.SummaryFilterItems) {
			continue
		}
		if operation == "count" && childFieldName == "" {
			count++
			continue
		}
		value, ok := summaryRecordFieldValue(e.Org, childState.Definition, child, childFieldName)
		if !ok {
			continue
		}
		number, ok := summaryNumericValue(value)
		if !ok {
			continue
		}
		values = append(values, number)
	}
	switch operation {
	case "count":
		if childFieldName == "" {
			return storage.IntegerValue(count), true
		}
		return storage.IntegerValue(int64(len(values))), true
	case "", "sum":
		total := 0.0
		for _, value := range values {
			total += value
		}
		return storage.DecimalValue(strconv.FormatFloat(total, 'f', -1, 64)), true
	case "max":
		if len(values) == 0 {
			return storage.NullValue(), true
		}
		max := values[0]
		for _, value := range values[1:] {
			if value > max {
				max = value
			}
		}
		return storage.DecimalValue(strconv.FormatFloat(max, 'f', -1, 64)), true
	case "min":
		if len(values) == 0 {
			return storage.NullValue(), true
		}
		min := values[0]
		for _, value := range values[1:] {
			if value < min {
				min = value
			}
		}
		return storage.DecimalValue(strconv.FormatFloat(min, 'f', -1, 64)), true
	default:
		return storage.Value{}, false
	}
}

func splitSummaryQualifiedField(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return "", name
	}
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func summaryFiltersMatch(org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, filters []storage.SummaryFilterItem) bool {
	for _, filter := range filters {
		_, fieldName := splitSummaryQualifiedField(filter.Field)
		if fieldName == "" {
			fieldName = filter.Field
		}
		canonical, ok := storage.ResolveFieldName(definition, org.Namespace, fieldName)
		if !ok {
			return false
		}
		value, ok := summaryRecordFieldValue(org, definition, record, canonical)
		if !ok {
			value = storage.NullValue()
		}
		if !summaryFilterMatches(value, filter) {
			return false
		}
	}
	return true
}

func summaryRecordFieldValue(org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, fieldName string) (storage.Value, bool) {
	if value, ok := record.Fields[fieldName]; ok {
		return value, true
	}
	field, ok := definition.Fields[fieldName]
	if !ok || field.Type != storage.FieldCalculated || strings.TrimSpace(field.Formula) == "" {
		return storage.Value{}, false
	}
	value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, org, definition, record)
	return value, ok
}

func summaryFilterMatches(value storage.Value, filter storage.SummaryFilterItem) bool {
	switch strings.ToLower(strings.TrimSpace(filter.Operation)) {
	case "", "equals":
		return summaryValueMatchesText(value, filter.Value)
	default:
		return false
	}
}

func summaryValueMatchesText(value storage.Value, text string) bool {
	text = strings.TrimSpace(text)
	switch value.Kind {
	case storage.ValueBoolean:
		return strings.EqualFold(strconv.FormatBool(value.Boolean), text)
	case storage.ValueString:
		return strings.EqualFold(value.String, text)
	case storage.ValueID:
		return strings.EqualFold(string(value.ID), text)
	case storage.ValueInteger:
		parsed, err := strconv.ParseInt(text, 10, 64)
		return err == nil && value.Integer == parsed
	case storage.ValueDecimal:
		return strings.TrimRight(strings.TrimRight(value.Decimal, "0"), ".") == strings.TrimRight(strings.TrimRight(text, "0"), ".")
	case storage.ValueNull:
		return strings.EqualFold(text, "null") || text == ""
	default:
		return false
	}
}

func summaryNumericValue(value storage.Value) (float64, bool) {
	switch value.Kind {
	case storage.ValueInteger:
		return float64(value.Integer), true
	case storage.ValueDecimal:
		parsed, err := strconv.ParseFloat(value.Decimal, 64)
		return parsed, err == nil
	case storage.ValueString:
		parsed, err := strconv.ParseFloat(value.String, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func storageValuesEqual(field storage.Field, left, right storage.Value) bool {
	if left.Kind == storage.ValueString && right.Kind == storage.ValueString && !field.CaseSensitive {
		return strings.EqualFold(left.String, right.String)
	}
	if left.Kind != right.Kind {
		if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
			return string(left.ID) == right.String
		}
		if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
			return left.String == string(right.ID)
		}
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return left.String == right.String
	case storage.ValueDecimal:
		return left.Decimal == right.Decimal
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueID:
		return left.ID == right.ID
	default:
		return false
	}
}

type dmlError struct {
	code    string
	fields  []string
	message string
}

func (e dmlError) Error() string {
	return e.message
}

func dmlErrorf(code string, fields []string, format string, args ...any) error {
	return dmlError{code: code, fields: append([]string(nil), fields...), message: fmt.Sprintf(format, args...)}
}

func customMetadataReadOnlyError(objectName string) error {
	return dmlErrorf("INVALID_TYPE", nil, "dml: custom metadata object %s is read-only", objectName)
}

func resultFromError(id storage.ID, err error) Result {
	var typed dmlError
	if errors.As(err, &typed) {
		return failedResult(id, typed.message, typed.code, typed.fields)
	}
	msg := err.Error()
	code := "FIELD_CUSTOM_VALIDATION_EXCEPTION"
	var fields []string
	switch {
	case contains(msg, "unknown object"):
		code = "INVALID_TYPE"
	case contains(msg, "unknown field"):
		code = "INVALID_FIELD_FOR_INSERT_UPDATE"
		fields = extractField(msg)
	case contains(msg, "required field"):
		code = "REQUIRED_FIELD_MISSING"
		fields = extractField(msg)
	case contains(msg, "duplicate id"):
		code = "DUPLICATE_VALUE"
		fields = []string{"Id"}
	case contains(msg, "deleted"):
		code = "ENTITY_IS_DELETED"
	case contains(msg, "update requires id") || contains(msg, "delete requires id") || contains(msg, "undelete requires id"):
		code = "MISSING_ARGUMENT"
		fields = []string{"Id"}
		if contains(msg, "update requires id") {
			msg = "Id not specified in an update call:"
		}
	case contains(msg, "does not exist"):
		code = "ENTITY_IS_DELETED"
	}
	return failedResult(id, msg, code, fields)
}

func failedResult(id storage.ID, message, statusCode string, fields []string) Result {
	copiedFields := append([]string(nil), fields...)
	return Result{
		ID:         id,
		Success:    false,
		Error:      message,
		StatusCode: statusCode,
		Fields:     copiedFields,
		Errors: []Error{{
			Message:    message,
			StatusCode: statusCode,
			Fields:     append([]string(nil), copiedFields...),
		}},
	}
}

func extractField(msg string) []string {
	// Extract field name from "dml: ... field Object.Field" or "dml: ... field Object.Field is null"
	parts := strings.Split(msg, ".")
	if len(parts) >= 2 {
		field := strings.TrimSpace(parts[len(parts)-1])
		field = strings.TrimSuffix(field, " is null")
		if field != "" {
			return []string{field}
		}
	}
	return nil
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), substr)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func copySequences(in map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
