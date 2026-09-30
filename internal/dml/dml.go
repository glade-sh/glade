package dml

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

type Engine struct {
	Org               *storage.OrgState
	IDs               storage.IDGenerator
	Now               func() time.Time
	UserID            storage.ID
	Options           Options
	FlowActionInvoker func(storage.FlowAction, storage.Record) error
	// FlowActionInvokerWithContext receives the values materialized by Flow
	// assignments and lookups for an Apex action. The legacy invoker remains
	// available for callers that only model the triggering record.
	FlowActionInvokerWithContext func(storage.FlowAction, storage.Record, FlowActionContext) error
	WorkflowEmailer              func(storage.WorkflowEmailAlert, storage.Record) error
	AutomationTracer             func(name string, args map[string]any)
	DeferAutomation              bool
	PriorRecords                 map[storage.ID]storage.Record
	IsolationJournal             *storage.IsolationJournal

	workflowDepth  int
	flowDepth      int
	summaryOrder   []string
	summaryUpdates map[string]SummaryUpdate
	automationRoll map[string]bool
	uniqueBatch    *uniqueBatchContext
	uniqueFieldMap map[string]bool
	activeValRules map[string]bool
	uniqueFields   map[string][]string
	uniqueIndexes  map[string]map[string]map[storage.ID]bool
	SummaryByChild *SummaryRelationCache
	subflowCache   map[string]cachedSubflow
}

// FlowActionContext contains the runtime references visible to a Flow Apex
// action. Collections and records are copied from the active Flow frame so an
// invoker can build the typed @InvocableMethod input without mutating Flow
// state.
type FlowActionContext struct {
	Records           map[string]storage.Record
	Collections       map[string][]storage.Record
	PriorCollections  map[string]bool
	Scalars           map[string]storage.Value
	LookupOutputs     map[string]storage.Record
	LookupCollections map[string][]storage.Record
}

// FlowInterviewInput contains the values supplied to an autolaunched Flow
// interview. The engine reuses the same Flow frame and Apex-action path as
// record-triggered flows.
type FlowInterviewInput struct {
	Scalars     map[string]storage.Value
	Records     map[string]storage.Record
	Collections map[string][]storage.Record
}

// FlowInterviewOutput contains the values materialized by an autolaunched
// Flow. It intentionally uses the same storage-backed shapes as the input so
// the VM can expose output variables through Flow.Interview.getVariableValue
// without creating a second Flow execution path.
type FlowInterviewOutput struct {
	Scalars     map[string]storage.Value
	Records     map[string]storage.Record
	Collections map[string][]storage.Record
}

type cachedSubflow struct {
	rule storage.FlowRule
	def  storage.ObjectDefinition
}

type SummaryRelationCache struct {
	mu      sync.RWMutex
	entries map[string][]summaryRelation
}

func NewSummaryRelationCache() *SummaryRelationCache {
	return &SummaryRelationCache{entries: make(map[string][]summaryRelation)}
}

func (c *SummaryRelationCache) load(childObjectName string) ([]summaryRelation, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	relations, ok := c.entries[childObjectName]
	return relations, ok
}

func (c *SummaryRelationCache) store(childObjectName string, relations []summaryRelation) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries[childObjectName] = relations
	c.mu.Unlock()
}

type Options struct {
	// Consumed by the VM after successful Task DML; it records a notification
	// request, not delivery or a Messaging.sendEmail invocation.
	CaptureTaskAssignmentEmail bool
	AllowFieldTruncation       bool
	AllowUpdateDeleted         bool
	AllowBatchUniqueValueSwap  bool
	// Test-context emptyRecycleBin releases membership while retaining deleted rows.
	RetainEmptiedRecycleBinRecords bool
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

type rollbackPoint struct {
	enabled   bool
	journal   bool
	mark      storage.IsolationMark
	org       storage.OrgState
	sequences map[string]uint64
}

type Error struct {
	Message    string   `json:"message"`
	StatusCode string   `json:"statusCode,omitempty"`
	Fields     []string `json:"fields,omitempty"`
}

type deleteRelation struct {
	childObject string
	field       string
}

type deleteContext struct {
	restrictedByParent map[string][]deleteRelation
	cascadeByParent    map[string][]deleteRelation
	setNullByParent    map[string][]deleteRelation
	referenceIndex     map[string]map[storage.ID][]storage.ID
}

type summaryRelation struct {
	parentObject string
	parentField  string
	field        storage.Field
	fkFieldName  string
}

func NewEngine(org *storage.OrgState) Engine {
	storage.EnsureUniqueKeyPrefixes(org)
	ids := storage.NewRuntimeIDGeneratorForOrg(org)
	ids.Sequences = copySequences(org.IDSequences)
	now := func() time.Time { return time.Now().UTC() }
	if org.Now != nil {
		now = org.Now
	}
	return Engine{
		Org:            org,
		IDs:            ids,
		Now:            now,
		UserID:         "005000000000001",
		automationRoll: make(map[string]bool),
		uniqueFieldMap: make(map[string]bool),
		activeValRules: make(map[string]bool),
		uniqueFields:   make(map[string][]string),
		uniqueIndexes:  make(map[string]map[string]map[storage.ID]bool),
	}
}

// RunAutolaunchedFlow executes an autolaunched Flow from an Apex interview in
// the current transaction. It deliberately shares the record-triggered Flow
// step interpreter so assignments, collection inputs, and Apex actions keep
// one observable implementation.
func (e *Engine) RunAutolaunchedFlow(rule storage.FlowRule, input FlowInterviewInput) error {
	_, err := e.RunAutolaunchedFlowWithOutput(rule, input)
	return err
}

// RunAutolaunchedFlowWithOutput executes an autolaunched Flow and returns the
// final frame values. Callers that do not need outputs can use
// RunAutolaunchedFlow, which preserves the original error-only contract.
func (e *Engine) RunAutolaunchedFlowWithOutput(rule storage.FlowRule, input FlowInterviewInput) (FlowInterviewOutput, error) {
	output := FlowInterviewOutput{
		Scalars:     make(map[string]storage.Value),
		Records:     make(map[string]storage.Record),
		Collections: make(map[string][]storage.Record),
	}
	if e == nil {
		return output, fmt.Errorf("autolaunched flow %s requires an engine", rule.Name)
	}
	frame := newFlowFrame()
	frame.textTemplates = cloneFlowTextTemplates(rule.TextTemplates)
	for name, value := range input.Scalars {
		frame.scalars[flowFrameKey(name)] = value.Clone()
	}
	for name, record := range input.Records {
		frame.records[flowFrameKey(name)] = record.Clone()
	}
	for name, records := range input.Collections {
		frame.collections[flowFrameKey(name)] = cloneFlowRecords(records)
	}
	// Salesforce exposes declared interview variables even when the caller
	// omits an optional input. Seed those variables as null or empty
	// collections so assignments and decisions can observe a real null value
	// instead of being treated as an unsupported reference.
	for _, variable := range rule.Variables {
		key := flowFrameKey(variable.Name)
		if key == "" {
			continue
		}
		if variable.IsCollection && flowVariableRecordCollection(variable) {
			if _, ok := frame.collections[key]; !ok {
				frame.collections[key] = []storage.Record{}
			}
			continue
		}
		if !variable.IsCollection && flowVariableRecordCollection(variable) && strings.TrimSpace(variable.ObjectType) != "" {
			if _, ok := frame.records[key]; !ok {
				frame.records[key] = storage.Record{Object: variable.ObjectType, Fields: make(map[string]storage.Value)}
			}
			continue
		}
		if variable.IsCollection {
			if _, ok := frame.scalars[key]; !ok {
				frame.scalars[key] = storage.ListValue()
			}
			continue
		}
		if _, ok := frame.scalars[key]; !ok {
			frame.scalars[key] = storage.NullValue()
		}
	}
	record := storage.Record{}
	_, err := e.applyFlowStepsWithFrame(rule.Name, "", &record, storage.ObjectDefinition{}, rule.Steps, frame)
	for name, value := range frame.scalars {
		output.Scalars[name] = value.Clone()
	}
	for name, value := range frame.records {
		output.Records[name] = value.Clone()
	}
	for name, records := range frame.collections {
		output.Collections[name] = cloneFlowRecords(records)
	}
	return output, err
}

func flowVariableRecordCollection(variable storage.FlowVariable) bool {
	return strings.EqualFold(strings.TrimSpace(variable.DataType), "SObject") ||
		strings.EqualFold(strings.TrimSpace(variable.DataType), "Apex")
}

func (e *Engine) syncOrgClock() {
	if e == nil || e.Org == nil || e.Now == nil {
		return
	}
	e.Org.Now = e.Now
}

func (e Engine) systemTimestamp() string {
	now := time.Now().UTC()
	if e.Now != nil {
		now = e.Now().UTC()
	}
	if e.Org != nil {
		var previous time.Time
		if e.Org.SystemTimestampBase != "" && e.Org.SystemTimestampSequence > 0 {
			if parsed, err := time.Parse(time.RFC3339, e.Org.SystemTimestampBase); err == nil {
				previous = parsed.Add(time.Duration(e.Org.SystemTimestampSequence-1) * time.Second)
			}
		}
		base := now.Format(time.RFC3339)
		if e.Org.SystemTimestampBase != base {
			e.Org.SystemTimestampBase = base
			e.Org.SystemTimestampSequence = 0
		}
		offset := e.Org.SystemTimestampSequence
		candidate := now.Add(time.Duration(offset) * time.Second)
		nextSequence := offset + 1
		if !previous.IsZero() && !candidate.After(previous) {
			candidate = previous.Add(time.Second)
			if delta := candidate.Sub(now); delta >= 0 {
				nextSequence = int64(delta/time.Second) + 1
			}
		}
		e.Org.SystemTimestampSequence = nextSequence
		now = candidate
	}
	return now.Format(time.RFC3339)
}

func (e Engine) statementTimestamp(stamp *string) string {
	if stamp == nil {
		return e.systemTimestamp()
	}
	if *stamp == "" {
		*stamp = e.systemTimestamp()
	}
	return *stamp
}

func (e Engine) systemUserID() storage.ID {
	if e.UserID != "" {
		return e.UserID
	}
	return "005000000000001"
}

func (e *Engine) Insert(records []storage.Record) []Result {
	e.syncOrgClock()
	results := make([]Result, len(records))
	statementStamp := ""
	for i, record := range records {
		id, err := e.insertOne(record, &statementStamp)
		if err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: id, Success: true}
	}
	e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
	return results
}

func (e *Engine) recordJournalSequence(objectName string) {
	if e == nil || e.IsolationJournal == nil || e.Org == nil || objectName == "" {
		return
	}
	if canonical, ok := storage.ResolveObjectName(*e.Org, objectName); ok {
		objectName = canonical
	}
	e.IsolationJournal.RecordSequence(objectName)
}

func (e *Engine) Update(records []storage.Record) []Result {
	e.syncOrgClock()
	results := make([]Result, len(records))
	restoreUniqueBatch := e.beginUniqueBatchContext(records)
	defer restoreUniqueBatch()
	for i, record := range records {
		if err := e.updateOne(record); err != nil {
			results[i] = resultFromError(record.ID, err)
			continue
		}
		results[i] = Result{ID: record.ID, Success: true}
	}
	e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
	return results
}

func (e *Engine) Delete(records []storage.Record) []Result {
	e.syncOrgClock()
	results := make([]Result, len(records))
	ctx := e.buildDeleteContext()
	for i, record := range records {
		if err := e.deleteOneWithContext(record, ctx); err != nil {
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
	e.syncOrgClock()
	results := make([]Result, len(records))
	restoreUniqueBatch := e.beginUniqueBatchContext(records)
	defer restoreUniqueBatch()
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
	e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
	return results
}

func (e *Engine) Undelete(records []storage.Record) []Result {
	e.syncOrgClock()
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
		if !stored.System.IsDeleted || stored.System.RecycleBinEmptied {
			results[i] = failedResult(record.ID, "Entity is not in the recycle bin", "UNDELETE_FAILED", nil)
			continue
		}
		if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
			object = e.Org.Objects[objectName]
			storedID, stored, _ = storage.LookupRecordByID(object.Records, record.ID)
		}
		stamp := e.systemTimestamp()
		if e.IsolationJournal != nil {
			e.IsolationJournal.RecordUpdate(objectName, storedID, stored)
		}
		stored.System.IsDeleted = false
		stored.System.RecycleBinEmptied = false
		stored.System.CascadeDeletedByObject = ""
		stored.System.CascadeDeletedByID = ""
		stored.System.LastModifiedDate = stamp
		stored.System.SystemModstamp = stamp
		stored.System.LastModifiedByID = e.systemUserID()
		object.Records[storedID] = stored
		e.Org.Objects[objectName] = object
		e.addUniqueIndexRecord(objectName, object.Definition, stored)
		e.restoreCascadeDeletedChildren(objectName, storedID, make(map[string]bool))
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) EmptyRecycleBin(records []storage.Record) []Result {
	e.syncOrgClock()
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
		if stored.System.RecycleBinEmptied {
			results[i] = failedResult(record.ID, "invalid record id; no recycle bin entry found", "INVALID_ID_FIELD", nil)
			continue
		}
		if !stored.System.IsDeleted {
			results[i] = failedResult(record.ID, fmt.Sprintf("dml: record %s is not in the recycle bin", record.ID), "ENTITY_IS_NOT_IN_RECYCLE_BIN", nil)
			continue
		}
		if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
			object = e.Org.Objects[objectName]
		}
		if e.IsolationJournal != nil {
			e.IsolationJournal.RecordUpdate(objectName, storedID, stored)
		}
		if e.Options.RetainEmptiedRecycleBinRecords {
			stored.System.RecycleBinEmptied = true
			object.Records[storedID] = stored
		} else {
			delete(object.Records, storedID)
		}
		e.Org.Objects[objectName] = object
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) Lock(records []storage.Record) []Result {
	e.syncOrgClock()
	return e.setLock(records, true)
}

func (e *Engine) Unlock(records []storage.Record) []Result {
	e.syncOrgClock()
	return e.setLock(records, false)
}

func (e *Engine) Merge(master storage.Record, duplicates []storage.Record) []Result {
	e.syncOrgClock()
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
	storedMasterID, storedMaster, ok := storage.LookupRecordByID(object.Records, master.ID)
	if !ok || storedMaster.System.IsDeleted {
		for i := range results {
			results[i] = resultFromError(master.ID, fmt.Errorf("dml: merge master %s does not exist", master.ID))
		}
		return results
	}
	master.ID = storedMasterID
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
		storedDuplicateID, storedDuplicate, ok := storage.LookupRecordByID(object.Records, duplicate.ID)
		if !ok || storedDuplicate.System.IsDeleted {
			results[i] = resultFromError(duplicate.ID, fmt.Errorf("dml: merge duplicate %s does not exist", duplicate.ID))
			continue
		}
		if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
			object = e.Org.Objects[objectName]
			storedDuplicateID, storedDuplicate, _ = storage.LookupRecordByID(object.Records, duplicate.ID)
		}
		duplicate.ID = storedDuplicateID
		updatedRelatedIDs := e.reparentLookups(objectName, duplicate.ID, master.ID)
		stamp := e.systemTimestamp()
		if e.IsolationJournal != nil {
			e.IsolationJournal.RecordUpdate(objectName, storedDuplicateID, storedDuplicate)
		}
		if storedDuplicate.Fields == nil {
			storedDuplicate.Fields = make(map[string]storage.Value)
		}
		storedDuplicate.Fields["MasterRecordId"] = storage.IDValue(master.ID)
		deleteExplicitNull(storedDuplicate.ExplicitNulls, "MasterRecordId")
		storedDuplicate.System.IsDeleted = true
		storedDuplicate.System.LastModifiedDate = stamp
		storedDuplicate.System.SystemModstamp = stamp
		storedDuplicate.System.LastModifiedByID = e.systemUserID()
		object.Records[storedDuplicateID] = storedDuplicate
		e.Org.Objects[objectName] = object
		e.removeUniqueIndexRecord(objectName, object.Definition, storedDuplicate)
		results[i] = Result{ID: master.ID, Success: true, MergedRecordIDs: []storage.ID{duplicate.ID}, UpdatedRelatedIDs: updatedRelatedIDs}
	}
	return results
}

func deleteExplicitNull(nulls map[string]bool, field string) {
	if nulls == nil {
		return
	}
	delete(nulls, field)
	for name := range nulls {
		if strings.EqualFold(name, field) {
			delete(nulls, name)
		}
	}
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
		if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
			object = e.Org.Objects[objectName]
			storedID, stored, _ = storage.LookupRecordByID(object.Records, record.ID)
		}
		if e.IsolationJournal != nil {
			e.IsolationJournal.RecordUpdate(objectName, storedID, stored)
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
				if _, cloned := storage.EnsureMutableObjectRecords(e.Org, childObjectName); cloned {
					childObject = e.Org.Objects[childObjectName]
					record = childObject.Records[id]
				}
				if e.IsolationJournal != nil {
					e.IsolationJournal.RecordUpdate(childObjectName, id, record)
				}
				record.Fields[relation.Field] = storage.IDValue(newID)
				childObject.Records[id] = record
				seen[id] = struct{}{}
				changed = true
			}
		}
		if changed {
			e.Org.Objects[childObjectName] = childObject
			e.clearUniqueIndexes()
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
	before := storage.SnapshotRuntimeOrg(e.Org)
	if err := fn(e); err != nil {
		*e.Org = before
		e.IDs.Sequences = copySequences(before.IDSequences)
		return err
	}
	return nil
}

func (e *Engine) beginRollbackPoint(enabled bool) rollbackPoint {
	if !enabled {
		return rollbackPoint{}
	}
	if e != nil && e.IsolationJournal != nil {
		return rollbackPoint{enabled: true, journal: true, mark: e.IsolationJournal.Mark()}
	}
	return rollbackPoint{
		enabled:   true,
		org:       storage.SnapshotRuntimeOrg(e.Org),
		sequences: copySequences(e.IDs.Sequences),
	}
}

func (e *Engine) restoreRollbackPoint(point rollbackPoint) error {
	if e == nil || !point.enabled {
		return nil
	}
	if point.journal && e.IsolationJournal != nil {
		if err := e.IsolationJournal.Rollback(point.mark); err != nil {
			return err
		}
		if e.Org != nil {
			e.IDs.Sequences = copySequences(e.Org.IDSequences)
		}
		e.clearUniqueIndexes()
		return nil
	}
	*e.Org = point.org
	e.IDs.Sequences = point.sequences
	e.clearUniqueIndexes()
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

func (e *Engine) insertOne(record storage.Record, statementStamp *string) (storage.ID, error) {
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
	normalizeUserNameFields(objectName, object.Definition, &record)
	createPersonContact := createsPersonContactOnInsert(objectName, record)
	applyDefaultRecordTypeID(objectName, object.Definition, &record)
	applyFieldDefaults(e.Org, object.Definition, &record)
	e.applyCurrencyInsertDefaults(object.Definition, &record)
	applyAutoNumberName(object.Definition, e.IDs.Sequences[objectName]+1, &record)
	e.applyContractNumberDefault(objectName, &record)
	applyCustomSettingInsertDefaults(e.Org, object.Definition, &record)
	applySetupInsertDefaults(objectName, object.Definition, &record)
	e.applyUserContactAccountDefault(objectName, object.Definition, &record)
	e.applyFileInsertDefaults(objectName, object.Definition, &record)
	if err := e.applyStringLengthRules(object.Definition, &record); err != nil {
		return "", err
	}
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return "", err
	}
	if err := validatePersonAccountRequiredFields(objectName, record); err != nil {
		return "", err
	}
	if err := validateAccountNameRequiredFields(objectName, object.Definition, record); err != nil {
		return "", err
	}
	if err := validateRequired(object.Definition, record); err != nil {
		return "", err
	}
	if isGeneratedPlaceholderInsertID(record.ID) {
		record.ID = ""
	}
	if err := e.validateObjectID(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateValidationRules(objectName, object.Definition, record, nil, true); err != nil {
		return "", err
	}
	if err := e.validateLookupFilters(object.Definition, record); err != nil {
		return "", err
	}
	if err := e.validateUnique(objectName, object.Definition, record, ""); err != nil {
		return "", err
	}
	needsFullRollback := sobjectInsertNeedsFullRollback(objectName) || (!e.DeferAutomation && hasObjectAutomation(object.Definition))
	needsPersonRollback := createPersonContact && !needsFullRollback
	rollback := e.beginRollbackPoint(needsFullRollback)
	var rollbackSequences map[string]uint64
	if needsPersonRollback {
		rollbackSequences = copySequences(e.IDs.Sequences)
	}
	if record.ID == "" {
		e.recordJournalSequence(objectName)
		id, err := e.IDs.Next(objectName)
		if err != nil {
			return "", err
		}
		record.ID = id
	}
	if err := storage.ValidateID(record.ID); err != nil {
		return "", err
	}
	if explicitNullCustomTextNameUsesRecordID(object.Definition, record) {
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		record.Fields["Name"] = storage.StringValue(string(record.ID))
		delete(record.ExplicitNulls, "Name")
	}
	if _, exists := object.Records[record.ID]; exists {
		return "", dmlErrorf("DUPLICATE_VALUE", []string{"Id"}, "dml: duplicate id %s", record.ID)
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
		object = e.Org.Objects[objectName]
	}
	stamp := e.statementTimestamp(statementStamp)
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
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordInsert(objectName, record.ID)
	}
	storedRecord := record.Clone()
	storedRecord.ParentRelationships = nil
	storedRecord.LoadedReferences = nil
	object.Records[record.ID] = storedRecord
	e.Org.Objects[objectName] = object
	e.addUniqueIndexRecord(objectName, object.Definition, record)
	e.recalculateSummaryFieldsForChildren(objectName, record)
	if err := e.afterInsertSObject(objectName, record); err != nil {
		if rollbackErr := e.restoreRollbackPoint(rollback); rollbackErr != nil {
			return "", rollbackErr
		}
		return "", err
	}
	if createPersonContact {
		if err := e.afterInsertPersonAccount(record); err != nil {
			if needsFullRollback {
				if rollbackErr := e.restoreRollbackPoint(rollback); rollbackErr != nil {
					return "", rollbackErr
				}
			} else {
				e.rollbackInsertedRecord(objectName, object.Definition, record, rollbackSequences)
			}
			return "", err
		}
	}
	if !e.DeferAutomation {
		if _, err := e.ApplyAutomation(objectName, record.ID); err != nil {
			if rollbackErr := e.restoreRollbackPoint(rollback); rollbackErr != nil {
				return "", rollbackErr
			}
			return "", err
		}
	}
	return record.ID, nil
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
