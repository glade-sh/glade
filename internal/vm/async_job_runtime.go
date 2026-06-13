package vm

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) testStart() (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.startTest is only available in test context")
	}
	if vm.testContext.Started {
		return Null, fmt.Errorf("Test.startTest cannot be called more than once")
	}
	vm.testContext.Started = true
	vm.testContext.Stopped = false
	vm.fakeNow = vm.fakeNow.Add(time.Second)
	vm.testContext.AsyncStartIndex = len(vm.testContext.AsyncJobs)
	vm.testContext.PlatformEventStartIndex = len(vm.testContext.PlatformEvents)
	vm.deferPreStartAsyncJobRecords()
	vm.testContext.ChainEnqueued = false
	vm.testContext.ParentLimits = vm.limits
	vm.testContext.ParentViolations = append([]LimitViolation(nil), vm.limitViolations...)
	vm.ResetLimits()
	return Null, nil
}
func (vm *VM) deferPreStartAsyncJobRecords() {
	if vm.testContext == nil || vm.Org == nil || len(vm.testContext.AsyncJobs) == 0 {
		return
	}
	vm.ensureAsyncObjects()
	storage.EnsureMutableObjectRecords(vm.Org, "AsyncApexJob")
	object := vm.Org.Objects["AsyncApexJob"]
	for _, job := range vm.testContext.AsyncJobs {
		storedID, record, ok := storage.LookupRecordByID(object.Records, storage.ID(job.ID))
		if !ok {
			continue
		}
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		vm.recordIsolationJournalMutation("AsyncApexJob", storedID, record, true)
		record.Fields["Status"] = storage.StringValue("Deferred")
		record.System.HiddenFromSOQL = true
		object.Records[storedID] = record
	}
	vm.Org.Objects["AsyncApexJob"] = object
}
func (vm *VM) testStop(result *Result) (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.stopTest is only available in test context")
	}
	if !vm.testContext.Started {
		return Null, fmt.Errorf("Test.stopTest called before Test.startTest")
	}
	if vm.testContext.Stopped {
		return Null, fmt.Errorf("Test.stopTest cannot be called more than once")
	}
	vm.testContext.Stopped = true
	err := vm.drainTestWork(result)
	vm.limits = vm.testContext.ParentLimits
	vm.limitViolations = append([]LimitViolation(nil), vm.testContext.ParentViolations...)
	return Null, err
}
func (vm *VM) drainTestWork(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	previousPreserve := vm.testContext.PreserveAsyncStatics
	vm.testContext.PreserveAsyncStatics = true
	defer func() {
		vm.testContext.PreserveAsyncStatics = previousPreserve
	}()
	for iteration := 0; iteration < maxLoopIterations; iteration++ {
		beforeAsync := len(vm.testContext.AsyncJobs)
		beforeEvents := len(vm.testContext.PlatformEvents)
		beforePublishes := len(vm.testContext.EventPublishes)
		if err := vm.drainTestAsync(result); err != nil {
			return err
		}
		if err := vm.drainTestPlatformEventsFrom(result, vm.testContext.PlatformEventStartIndex, true); err != nil {
			return err
		}
		if err := vm.drainTestEventPublishes(result); err != nil {
			return err
		}
		startIndex := vm.testContext.AsyncStartIndex
		if startIndex < 0 {
			startIndex = 0
		}
		if startIndex > len(vm.testContext.AsyncJobs) {
			startIndex = len(vm.testContext.AsyncJobs)
		}
		eventStartIndex := vm.testContext.PlatformEventStartIndex
		if eventStartIndex < 0 {
			eventStartIndex = 0
		}
		if eventStartIndex > len(vm.testContext.PlatformEvents) {
			eventStartIndex = len(vm.testContext.PlatformEvents)
		}
		if len(vm.testContext.AsyncJobs) <= startIndex && len(vm.testContext.PlatformEvents) <= eventStartIndex && len(vm.testContext.EventPublishes) == 0 {
			return nil
		}
		if len(vm.testContext.AsyncJobs) == beforeAsync && len(vm.testContext.PlatformEvents) == beforeEvents && len(vm.testContext.EventPublishes) == beforePublishes &&
			nextDrainableAsyncJobIndex(vm.testContext.AsyncJobs, startIndex) < 0 {
			return nil
		}
	}
	return fmt.Errorf("Test.stopTest async/event drain exceeded %d iterations", maxLoopIterations)
}
func (vm *VM) enqueueJob(args []Value, result *Result) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("System.enqueueJob expects Queueable[, Integer|AsyncOptions]")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("System.enqueueJob expects Queueable object")
	}
	delayMinutes := 0
	if len(args) == 2 {
		switch {
		case args[1].Kind == ValueInt:
			delayMinutes = int(args[1].Int)
		case args[1].Kind == ValueObject && strings.EqualFold(args[1].Type, "AsyncOptions"):
			if delay, ok := asyncOptionsInt(args[1], "minimumQueueableDelayInMinutes"); ok {
				delayMinutes = delay
			}
		default:
			return Null, fmt.Errorf("System.enqueueJob options expects Integer or AsyncOptions")
		}
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("queueableJobs", 1); err != nil {
		return Null, err
	}
	draining, chainEnqueued := vm.asyncDrainState()
	if draining && vm.currentAsyncKind == "Queueable" && chainEnqueued {
		return Null, fmt.Errorf("Queueable chaining limit exceeded")
	}
	vm.markAsyncChainEnqueued()
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "Queueable", Object: cloneValue(args[0]), QueueableDelayMinutes: delayMinutes}
	if vm.currentAsyncKind == "Queueable" {
		job.QueueableDepth = vm.currentQueueableDepth + 1
		job.QueueableMaxDepth = vm.currentQueueableMaxDepth
	} else {
		job.QueueableDepth = 1
	}
	if len(args) == 2 && args[1].Kind == ValueObject {
		if maxDepth, ok := asyncOptionsInt(args[1], "maximumQueueableStackDepth"); ok {
			job.QueueableMaxDepth = maxDepth
		}
	}
	if job.QueueableMaxDepth > 0 && job.QueueableDepth > job.QueueableMaxDepth {
		return Null, fmt.Errorf("MaximumQueueableStackDepth exceeded")
	}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":  job.Kind,
		"jobId": job.ID,
		"class": args[0].Type,
	})
	return String(job.ID), nil
}
func (vm *VM) executeBatch(args []Value, result *Result) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("Database.executeBatch expects batch instance[, scopeSize]")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("Database.executeBatch expects Batchable object")
	}
	batchSize := 200
	if len(args) == 2 {
		if args[1].Kind != ValueInt {
			return Null, fmt.Errorf("Database.executeBatch scope size expects Integer")
		}
		batchSize = int(args[1].Int)
		if batchSize <= 0 {
			return Null, fmt.Errorf("Database.executeBatch scope size must be positive")
		}
		if batchSize > 2000 {
			return Null, fmt.Errorf("Database.executeBatch scope size must be at most 2000")
		}
	}
	if !vm.isBatchableObject(args[0]) {
		return Null, fmt.Errorf("Database.executeBatch expects Batchable object")
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("batchJobs", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "BatchApex", Object: cloneValueDetachedPreserveRefs(args[0]), BatchSize: batchSize}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":      job.Kind,
		"jobId":     job.ID,
		"class":     args[0].Type,
		"batchSize": batchSize,
	})
	return String(job.ID), nil
}

func (vm *VM) isBatchableObject(value Value) bool {
	if value.Kind != ValueObject || strings.TrimSpace(value.Type) == "" {
		return false
	}
	if userProvisioningBatchableType(value.Type) {
		return true
	}
	class, ok := vm.lookupClass(value.Type)
	if ok {
		for _, iface := range vm.resolvedInterfaceNamesInHierarchy(class) {
			if batchableInterfaceName(iface) {
				return true
			}
		}
	}
	return false
}

func batchableInterfaceName(name string) bool {
	name = strings.TrimSpace(name)
	if base, ok := genericBaseName(name); ok {
		name = base
	}
	return strings.EqualFold(name, "Database.Batchable") || strings.EqualFold(name, "Batchable")
}

func (vm *VM) scheduleJob(args []Value, result *Result) (Value, error) {
	if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueObject {
		return Null, fmt.Errorf("System.schedule expects name, cron, and Schedulable object")
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("scheduledJobs", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "ScheduledApex", Object: cloneValue(args[2]), Name: args[0].Text, Cron: args[1].Text}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	vm.recordCronTrigger(job, "Waiting")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":  job.Kind,
		"jobId": job.ID,
		"class": args[2].Type,
		"name":  job.Name,
	})
	return String(cronTriggerID(job.ID)), nil
}
func (vm *VM) scheduleBatch(args []Value, result *Result) (Value, error) {
	if len(args) != 3 && len(args) != 4 {
		return Null, fmt.Errorf("System.scheduleBatch expects batch instance, name, minutesFromNow[, scopeSize]")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("System.scheduleBatch expects Batchable object")
	}
	if !vm.isBatchableObject(args[0]) {
		return Null, fmt.Errorf("System.scheduleBatch expects Batchable object")
	}
	if args[1].Kind != ValueString {
		return Null, fmt.Errorf("System.scheduleBatch expects job name String")
	}
	if args[2].Kind != ValueInt {
		return Null, fmt.Errorf("System.scheduleBatch expects minutesFromNow Integer")
	}
	batchSize := 200
	if len(args) == 4 {
		if args[3].Kind != ValueInt {
			return Null, fmt.Errorf("System.scheduleBatch scope size expects Integer")
		}
		batchSize = int(args[3].Int)
		if batchSize <= 0 {
			return Null, fmt.Errorf("System.scheduleBatch scope size must be positive")
		}
		if batchSize > 2000 {
			return Null, fmt.Errorf("System.scheduleBatch scope size must be at most 2000")
		}
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("batchJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("scheduledJobs", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "ScheduledBatch", Object: cloneValue(args[0]), BatchSize: batchSize, Name: args[1].Text, Cron: fmt.Sprintf("after %d minutes", args[2].Int), SuppressWorkerRecords: true}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	vm.recordCronTrigger(job, "Waiting")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":      job.Kind,
		"jobId":     job.ID,
		"class":     args[0].Type,
		"name":      job.Name,
		"batchSize": batchSize,
	})
	return String(cronTriggerID(job.ID)), nil
}
func (vm *VM) abortJob(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("System.abortJob expects job Id")
	}
	if vm.testContext == nil {
		return Null, unsupportedCallError("System.abortJob local async scheduling surface")
	}
	jobID, ok := valueIDString(args[0])
	if !ok {
		return Null, fmt.Errorf("System.abortJob expects String job Id")
	}
	for i, job := range vm.testContext.AsyncJobs {
		if job.ID != jobID && cronTriggerID(job.ID) != jobID {
			continue
		}
		vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs[:i], vm.testContext.AsyncJobs[i+1:]...)
		vm.recordAsyncJob(job, "Aborted", "")
		if job.Kind == "ScheduledApex" {
			vm.recordCronTrigger(job, "Deleted")
		}
		return Null, nil
	}
	if vm.asyncJobRecordStatus(jobID) != "" {
		vm.abortRecordedAsyncJob(jobID)
		return Null, nil
	}
	return Null, unsupportedCallError("System.abortJob unknown local async records")
}
func (vm *VM) abortRecordedAsyncJob(jobID string) {
	if vm == nil || vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	asyncID := jobID
	if strings.HasPrefix(asyncID, "08e") {
		asyncID = strings.Replace(asyncID, "08e", "707", 1)
	}
	if object, ok := vm.Org.Objects["AsyncApexJob"]; ok {
		if record, found := object.Records[storage.ID(asyncID)]; found {
			vm.recordIsolationJournalMutation("AsyncApexJob", storage.ID(asyncID), record, true)
			if record.Fields == nil {
				record.Fields = make(map[string]storage.Value)
			}
			record.Fields["Status"] = storage.StringValue("Aborted")
			object.Records[record.ID] = record
			vm.Org.Objects["AsyncApexJob"] = object
		}
	}
	cronID := jobID
	if strings.HasPrefix(cronID, "707") {
		cronID = strings.Replace(cronID, "707", "08e", 1)
	}
	if object, ok := vm.Org.Objects["CronTrigger"]; ok {
		if record, found := object.Records[storage.ID(cronID)]; found {
			vm.recordIsolationJournalMutation("CronTrigger", storage.ID(cronID), record, true)
			if record.Fields == nil {
				record.Fields = make(map[string]storage.Value)
			}
			record.Fields["State"] = storage.StringValue("Deleted")
			object.Records[record.ID] = record
			vm.Org.Objects["CronTrigger"] = object
		}
	}
}
func (vm *VM) asyncJobRecordStatus(jobID string) string {
	if vm.Org == nil {
		return ""
	}
	vm.ensureAsyncObjects()
	if strings.HasPrefix(jobID, "08e") {
		jobID = strings.Replace(jobID, "08e", "707", 1)
	}
	object := vm.Org.Objects["AsyncApexJob"]
	record, ok := object.Records[storage.ID(jobID)]
	if !ok {
		return ""
	}
	if status, ok := record.GetField("Status"); ok && status.Kind == storage.ValueString {
		return status.String
	}
	return ""
}
func (vm *VM) drainTestAsync(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	startIndex := vm.testContext.AsyncStartIndex
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(vm.testContext.AsyncJobs) {
		startIndex = len(vm.testContext.AsyncJobs)
	}
	return vm.drainAsyncJobsFrom(result, &vm.testContext.AsyncJobs, startIndex, &vm.testContext.Draining, &vm.testContext.ChainEnqueued)
}
func (vm *VM) drainTestEventPublishes(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	for len(vm.testContext.EventPublishes) > 0 {
		publish := vm.testContext.EventPublishes[0]
		vm.testContext.EventPublishes = vm.testContext.EventPublishes[1:]
		methodName := "onSuccess"
		resultType := "eventbus.SuccessResult"
		if publish.Fail {
			methodName = "onFailure"
			resultType = "eventbus.FailureResult"
		}
		callbackResult := Object(resultType)
		uuidValues := make([]Value, 0, len(publish.EventUUIDs))
		for _, uuid := range publish.EventUUIDs {
			uuidValues = append(uuidValues, String(uuid))
		}
		callbackResult.Fields["eventUuids"] = List(uuidValues...)
		method, ok, ambiguous := vm.resolveInstanceMethodForArgs(publish.Callback.Type, methodName, []Value{callbackResult})
		if ambiguous {
			return vm.ambiguousOverloadError(publish.Callback.Type+"."+methodName, []Value{callbackResult})
		}
		if !ok {
			return fmt.Errorf("event publish callback %s has no %s method", publish.Callback.Type, methodName)
		}
		if _, err := vm.callMethodWithReceiver(method, publish.Callback, []Value{callbackResult}, result); err != nil {
			return err
		}
	}
	return nil
}
func (vm *VM) drainTestPlatformEvents(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	return vm.drainTestPlatformEventsFrom(result, 0, false)
}
func (vm *VM) drainTestPlatformEventsFrom(result *Result, startIndex int, stopTimeDelivery bool) error {
	if vm.testContext == nil {
		return nil
	}
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(vm.testContext.PlatformEvents) {
		startIndex = len(vm.testContext.PlatformEvents)
	}
	for len(vm.testContext.PlatformEvents) > startIndex {
		records := append([]storage.Record(nil), vm.testContext.PlatformEvents[startIndex:]...)
		vm.testContext.PlatformEvents = vm.testContext.PlatformEvents[:startIndex]
		grouped := make(map[string][]storage.Record)
		order := make([]string, 0)
		for _, record := range records {
			if _, ok := grouped[record.Object]; !ok {
				order = append(order, record.Object)
			}
			grouped[record.Object] = append(grouped[record.Object], record)
		}
		for _, objectName := range order {
			if stopTimeDelivery {
				wasDraining := vm.testContext.Draining
				previousUser := vm.testContext.CurrentUser
				vm.testContext.Draining = true
				if user := vm.automatedProcessUser(); user.Kind != "" {
					vm.testContext.CurrentUser = user
				}
				_, err := vm.runWithFreshStatics(func() (Value, error) {
					_, err := vm.runTriggers(triggerTimingAfter, "insert", grouped[objectName], nil, result)
					return Null, err
				})
				vm.testContext.Draining = wasDraining
				vm.testContext.CurrentUser = previousUser
				if err != nil {
					return err
				}
				continue
			}
			if _, err := vm.runTriggers(triggerTimingAfter, "insert", grouped[objectName], nil, result); err != nil {
				return err
			}
		}
	}
	return nil
}
func (vm *VM) automatedProcessUser() Value {
	if vm == nil || vm.Org == nil {
		return Value{}
	}
	users, ok := vm.Org.Objects["User"]
	if !ok {
		return Value{}
	}
	for _, record := range users.Records {
		if strings.EqualFold(recordFieldString(record, "UserType"), "AutomatedProcess") {
			return vmValueFromRecord(record)
		}
	}
	return Value{}
}
func (vm *VM) runWithFreshStatics(fn func() (Value, error)) (Value, error) {
	snapshot := vm.staticFieldSnapshot()
	staticInitSnapshot := copyStaticInitStateMap(vm.staticInitState)
	if err := vm.ResetStatics(); err != nil {
		return Null, err
	}
	value, err := fn()
	vm.restoreStaticFieldSnapshot(snapshot)
	vm.staticInitState = copyStaticInitStateMap(staticInitSnapshot)
	return value, err
}
func (vm *VM) drainLocalAsync(result *Result) error {
	return vm.drainAsyncJobs(result, &vm.localAsyncJobs, &vm.localAsyncDrain, &vm.localAsyncChain)
}
func (vm *VM) drainAsyncJobs(result *Result, jobs *[]AsyncJob, draining *bool, chainEnqueued *bool) error {
	return vm.drainAsyncJobsFrom(result, jobs, 0, draining, chainEnqueued)
}
func (vm *VM) drainAsyncJobsFrom(result *Result, jobs *[]AsyncJob, startIndex int, draining *bool, chainEnqueued *bool) error {
	if *draining {
		return nil
	}
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(*jobs) {
		startIndex = len(*jobs)
	}
	*draining = true
	restoreStatics := true
	if vm.testContext != nil && vm.testContext.PreserveAsyncStatics {
		restoreStatics = false
	}
	var snapshot map[string]map[string]Value
	var staticInitSnapshot map[string]staticInitState
	if restoreStatics {
		snapshot = vm.staticFieldSnapshot()
		staticInitSnapshot = copyStaticInitStateMap(vm.staticInitState)
	}
	defer func() {
		if restoreStatics {
			vm.restoreStaticFieldSnapshot(snapshot)
			vm.staticInitState = copyStaticInitStateMap(staticInitSnapshot)
		}
		*draining = false
	}()
	maxJobs := -1
	if vm.testContext != nil {
		maxJobs = drainableAsyncJobCount(*jobs, startIndex)
	}
	processed := 0
	for maxJobs < 0 || processed < maxJobs {
		jobIndex := nextDrainableAsyncJobIndex(*jobs, startIndex)
		if jobIndex < 0 {
			break
		}
		job := (*jobs)[jobIndex]
		*jobs = append((*jobs)[:jobIndex], (*jobs)[jobIndex+1:]...)
		processed++
		var asyncStaticSnapshot map[string]map[string]Value
		var asyncStaticInitSnapshot map[string]staticInitState
		if vm.testContext != nil {
			asyncStaticSnapshot = vm.testAsyncStaticFieldSnapshot()
			asyncStaticInitSnapshot = copyStaticInitStateMap(vm.staticInitState)
			if err := vm.ResetTestAsyncStaticCollections(); err != nil {
				vm.restoreStaticFieldSnapshot(asyncStaticSnapshot)
				vm.staticInitState = copyStaticInitStateMap(asyncStaticInitSnapshot)
				return err
			}
		} else {
			if err := vm.ResetStatics(); err != nil {
				return err
			}
		}
		*chainEnqueued = false
		vm.recordAsyncJob(job, "Processing", "")
		appendTrace(result, "apex.async.run", "apex.async", map[string]any{
			"kind":  job.Kind,
			"jobId": job.ID,
		})
		err := vm.runAsyncJob(job, result)
		if asyncStaticSnapshot != nil {
			vm.restoreStaticFieldSnapshot(asyncStaticSnapshot)
			vm.staticInitState = copyStaticInitStateMap(asyncStaticInitSnapshot)
		}
		if err != nil {
			vm.recordAsyncJob(job, "Failed", err.Error())
			return err
		}
		if vm.testContext != nil && job.Kind == "BatchApex" {
			vm.recordAsyncJob(job, "Completed", "")
			vm.markCompletedBatchJobVisiblePendingInTest(job)
		} else if job.Kind == "ScheduledApex" {
			vm.recordAsyncJob(job, "Queued", "")
		} else {
			vm.recordAsyncJob(job, "Completed", "")
		}
	}
	return nil
}
func drainableAsyncJobCount(jobs []AsyncJob, startIndex int) int {
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(jobs) {
		startIndex = len(jobs)
	}
	count := 0
	for _, job := range jobs[startIndex:] {
		if !job.Deferred {
			count++
		}
	}
	return count
}
func nextDrainableAsyncJobIndex(jobs []AsyncJob, startIndex int) int {
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(jobs) {
		startIndex = len(jobs)
	}
	for i := startIndex; i < len(jobs); i++ {
		if !jobs[i].Deferred {
			return i
		}
	}
	return -1
}
func (vm *VM) staticFieldSnapshot() map[string]map[string]Value {
	out := make(map[string]map[string]Value, len(vm.Classes))
	for className, class := range vm.Classes {
		fields := make(map[string]Value, len(class.StaticFields))
		for fieldName, field := range class.StaticFields {
			fields[fieldName] = field.Value
		}
		out[className] = fields
	}
	return out
}
func (vm *VM) testAsyncStaticFieldSnapshot() map[string]map[string]Value {
	out := make(map[string]map[string]Value, len(vm.Classes))
	for className, class := range vm.Classes {
		if !classHasStaticCollectionField(class) {
			continue
		}
		fields := make(map[string]Value)
		for fieldName, field := range class.StaticFields {
			if resetTestAsyncStaticField(field) || resetTestAsyncStaticFieldForReinitialization(class, field) {
				fields[fieldName] = cloneValue(field.Value)
			}
		}
		if len(fields) != 0 {
			out[className] = fields
		}
	}
	return out
}
func (vm *VM) restoreStaticFieldSnapshot(snapshot map[string]map[string]Value) {
	for className, fields := range snapshot {
		class, ok := vm.Classes[className]
		if !ok {
			continue
		}
		for fieldName, value := range fields {
			field, ok := class.StaticFields[fieldName]
			if !ok {
				continue
			}
			field.Value = value
			class.StaticFields[fieldName] = field
		}
		vm.Classes[className] = class
	}
	vm.invalidateStaticValueRefs()
}
func (vm *VM) asyncDrainState() (bool, bool) {
	if vm.testContext != nil {
		return vm.testContext.Draining, vm.testContext.ChainEnqueued
	}
	return vm.localAsyncDrain, vm.localAsyncChain
}
func (vm *VM) markAsyncChainEnqueued() {
	if vm.testContext != nil {
		if vm.testContext.Draining {
			vm.testContext.ChainEnqueued = true
		}
		return
	}
	if vm.localAsyncDrain {
		vm.localAsyncChain = true
	}
}
func (vm *VM) enqueueAsyncJob(job AsyncJob) {
	if vm.testContext != nil {
		vm.recordApexClass(asyncClassName(job))
		if vm.testContext.Draining && job.Kind != "Queueable" && job.Kind != "BatchApex" {
			return
		}
		if vm.testContext.Draining && job.Kind == "Queueable" && !vm.canDrainQueueableJob(job) {
			job.Deferred = true
		}
		if vm.testContext.Draining && job.Kind == "BatchApex" && vm.currentAsyncKind == "Queueable" {
			job.Deferred = true
		}
		if vm.testContext.Draining && job.Kind == "BatchApex" && vm.currentAsyncKind != "" {
			job.SuppressWorkerRecords = true
		}
		vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs, job)
		return
	}
	vm.localAsyncJobs = append(vm.localAsyncJobs, job)
}
func asyncOptionsInt(options Value, fieldName string) (int, bool) {
	if options.Kind != ValueObject {
		return 0, false
	}
	for name, value := range options.Fields {
		if !strings.EqualFold(name, fieldName) {
			continue
		}
		switch value.Kind {
		case ValueInt:
			return int(value.Int), true
		case ValueDecimal:
			return int(value.Decimal), true
		}
	}
	return 0, false
}
func (vm *VM) canDrainQueueableJob(job AsyncJob) bool {
	if vm.currentAsyncKind == "BatchApex" {
		return true
	}
	if vm.currentAsyncKind != "Queueable" {
		return false
	}
	if job.QueueableMaxDepth <= 0 {
		return false
	}
	return job.QueueableDepth > 0 && job.QueueableDepth <= job.QueueableMaxDepth
}
func (vm *VM) runAsyncJob(job AsyncJob, result *Result) error {
	switch job.Kind {
	case "Future":
		_, err := vm.withAsyncKind("Future", func() (Value, error) {
			return vm.callMethod(job.Method, job.Args, result)
		})
		return err
	case "Queueable":
		args := []Value{asyncContext("QueueableContext", job.ID)}
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", args)
		if !ok && !ambiguous {
			args = nil
			target, ok, ambiguous = vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", nil)
		}
		staticTarget := Method{}
		staticExecute := false
		if !ok && !ambiguous {
			args = []Value{asyncContext("QueueableContext", job.ID)}
			staticTarget, ok, ambiguous = vm.resolveStaticMethodForArgs(job.Object.Type, "execute", args)
			staticExecute = ok
		}
		if !ok && !ambiguous {
			args = nil
			staticTarget, ok, ambiguous = vm.resolveStaticMethodForArgs(job.Object.Type, "execute", nil)
			staticExecute = ok
		}
		if ambiguous {
			return fmt.Errorf("async job %s execute method is ambiguous", job.Object.Type)
		}
		if !ok {
			return fmt.Errorf("async job %s has no execute method", job.Object.Type)
		}
		if staticExecute {
			target = staticTarget
		}
		if len(target.Params) == 0 {
			args = nil
		}
		previousFinalizer := vm.currentFinalizer
		vm.currentFinalizer = Value{}
		_, err := vm.withQueueableJob(job, func() (Value, error) {
			if staticExecute {
				return vm.callMethod(target, args, result)
			}
			return vm.callMethodWithReceiver(target, job.Object, args, result)
		})
		finalizer := vm.currentFinalizer
		vm.currentFinalizer = previousFinalizer
		if finalizer.Kind == ValueObject {
			finalizerErr := vm.runQueueableFinalizer(finalizer, job, result, err)
			if err == nil {
				err = finalizerErr
			}
		}
		return err
	case "BatchApex":
		_, err := vm.withAsyncKind("BatchApex", func() (Value, error) {
			return Null, vm.runBatchJob(job, result)
		})
		return err
	case "ScheduledBatch":
		_, err := vm.withAsyncKind("BatchApex", func() (Value, error) {
			return Null, vm.runBatchJob(job, result)
		})
		vm.recordCronTrigger(job, "Complete")
		return err
	case "ScheduledApex":
		args := []Value{schedulableContext(job.ID)}
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", args)
		if ambiguous {
			return vm.ambiguousOverloadError(job.Object.Type+".execute", args)
		}
		if !ok {
			target, ok, ambiguous = vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", nil)
			if ambiguous {
				return vm.ambiguousOverloadError(job.Object.Type+".execute", nil)
			}
		}
		if !ok {
			return fmt.Errorf("scheduled job %s has no execute method", job.Object.Type)
		}
		if len(target.Params) == 0 {
			args = nil
		}
		_, err := vm.withAsyncKind("ScheduledApex", func() (Value, error) {
			return vm.callMethodWithReceiver(target, job.Object, args, result)
		})
		vm.recordCronTrigger(job, "Complete")
		return err
	default:
		return fmt.Errorf("unsupported async job kind %s", job.Kind)
	}
}
func (vm *VM) runQueueableFinalizer(finalizer Value, job AsyncJob, result *Result, parentErr error) error {
	args := []Value{finalizerContext(job.ID, parentErr)}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(finalizer.Type, "execute", args)
	if ambiguous {
		return fmt.Errorf("async finalizer %s execute method is ambiguous", finalizer.Type)
	}
	if !ok {
		return fmt.Errorf("async finalizer %s has no execute method", finalizer.Type)
	}
	if len(target.Params) == 0 {
		args = nil
	}
	_, err := vm.withAsyncKind("Queueable", func() (Value, error) {
		return vm.callMethodWithReceiver(target, finalizer, args, result)
	})
	return err
}
func (vm *VM) withAsyncKind(kind string, run func() (Value, error)) (Value, error) {
	previous := vm.currentAsyncKind
	vm.currentAsyncKind = kind
	defer func() {
		vm.currentAsyncKind = previous
	}()
	return run()
}
func (vm *VM) withQueueableJob(job AsyncJob, run func() (Value, error)) (Value, error) {
	previousKind := vm.currentAsyncKind
	previousDepth := vm.currentQueueableDepth
	previousMaxDepth := vm.currentQueueableMaxDepth
	previousDelay := vm.currentQueueableDelay
	vm.currentAsyncKind = "Queueable"
	vm.currentQueueableDepth = job.QueueableDepth
	if vm.currentQueueableDepth <= 0 {
		vm.currentQueueableDepth = 1
	}
	vm.currentQueueableMaxDepth = job.QueueableMaxDepth
	vm.currentQueueableDelay = job.QueueableDelayMinutes
	defer func() {
		vm.currentAsyncKind = previousKind
		vm.currentQueueableDepth = previousDepth
		vm.currentQueueableMaxDepth = previousMaxDepth
		vm.currentQueueableDelay = previousDelay
	}()
	return run()
}
func (vm *VM) isAsyncKind(callee string) bool {
	switch callee {
	case "System.isBatch":
		return vm.currentAsyncKind == "BatchApex"
	case "System.isFuture":
		return vm.currentAsyncKind == "Future"
	case "System.isQueueable":
		return vm.currentAsyncKind == "Queueable"
	case "System.isScheduled":
		return vm.currentAsyncKind == "ScheduledApex"
	default:
		return false
	}
}
func (vm *VM) runBatchJob(job AsyncJob, result *Result) error {
	var scope []Value
	stateful := vm.isStatefulBatchObject(job.Object)
	baseObject := cloneValueDetachedPreserveRefs(job.Object)
	statefulObject := cloneValueDetachedPreserveRefs(job.Object)
	if vm.requiresDatabaseBatchableContract(job.Object) {
		itemType := vm.databaseBatchableItemType(job.Object)
		if err := vm.validateDatabaseBatchableContract(job.Object.Type, itemType); err != nil {
			return err
		}
	}
	if start, ok := vm.resolveInstanceMethod(job.Object.Type, "start"); ok {
		receiver := batchTransactionReceiver(baseObject, stateful, &statefulObject)
		value, err := vm.withAsyncLimitWindow(func() (Value, error) {
			return vm.callMethodWithReceiver(start, receiver, batchArgs(start, "Database.BatchableContext", job.ID), result)
		})
		if err != nil {
			return err
		}
		scopeValues, err := vm.batchScopeValues(value, result)
		if err != nil {
			return err
		}
		scope = append(scope, scopeValues...)
	}
	chunks := batchChunks(scope, job.BatchSize)
	executeArgs := []Value{asyncContext("Database.BatchableContext", job.ID), List()}
	execute, ok, ambiguous := vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", executeArgs)
	if ambiguous {
		return vm.ambiguousOverloadError(job.Object.Type+".execute", executeArgs)
	}
	if !ok {
		executeArgs = []Value{List()}
		execute, ok, ambiguous = vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", executeArgs)
		if ambiguous {
			return vm.ambiguousOverloadError(job.Object.Type+".execute", executeArgs)
		}
	}
	if !ok {
		execute, ok, ambiguous = vm.resolveInstanceMethodForArgs(job.Object.Type, "execute", nil)
		if ambiguous {
			return vm.ambiguousOverloadError(job.Object.Type+".execute", nil)
		}
	}
	if ok {
		vm.recordAsyncJobTotals(job, len(chunks), 0, 0)
		processed := 0
		processedItems := 0
		for _, chunk := range chunks {
			rollbackMark, rollbackEnabled := vm.beginBatchChunkTransaction()
			receiver := batchTransactionReceiver(baseObject, stateful, &statefulObject)
			_, err := vm.withAsyncLimitWindow(func() (Value, error) {
				return vm.callMethodWithReceiver(execute, receiver, batchExecuteArgs(execute, chunk, job.ID), result)
			})
			if err != nil {
				if rollbackEnabled {
					if rollbackErr := vm.rollbackBatchChunkTransaction(rollbackMark); rollbackErr != nil {
						return rollbackErr
					}
				}
				if !job.SuppressWorkerRecords {
					vm.recordBatchWorkerJob(job, processed+1, processedItems, chunk, "Failed", err.Error())
				}
				vm.recordAsyncJobTotals(job, len(chunks), processed, 1)
				vm.recordAsyncJob(job, "Failed", err.Error())
				if eventErr := vm.emitBatchApexErrorEvent(job, chunk, "EXECUTE", err, result); eventErr != nil {
					return eventErr
				}
				return err
			}
			processed++
			processedItems += len(chunk)
			if !job.SuppressWorkerRecords {
				vm.recordBatchWorkerJob(job, processed, processedItems, chunk, "Completed", "")
			}
		}
	}
	if finish, ok := vm.resolveInstanceMethod(job.Object.Type, "finish"); ok {
		vm.recordAsyncJob(job, "Completed", "")
		receiver := batchTransactionReceiver(baseObject, stateful, &statefulObject)
		rollbackMark, rollbackEnabled := vm.beginBatchChunkTransaction()
		_, err := vm.withAsyncLimitWindow(func() (Value, error) {
			return vm.callMethodWithReceiver(finish, receiver, batchArgs(finish, "Database.BatchableContext", job.ID), result)
		})
		if err != nil {
			if rollbackEnabled {
				if rollbackErr := vm.rollbackBatchChunkTransaction(rollbackMark); rollbackErr != nil {
					return rollbackErr
				}
			}
			vm.recordAsyncJobTotals(job, len(chunks), len(chunks), 1)
			vm.recordAsyncJob(job, "Failed", err.Error())
			return err
		}
	}
	return nil
}

func batchTransactionReceiver(base Value, stateful bool, statefulObject *Value) Value {
	if stateful && statefulObject != nil {
		return *statefulObject
	}
	return cloneValueDetachedPreserveRefs(base)
}

func (vm *VM) isStatefulBatchObject(value Value) bool {
	if value.Kind != ValueObject || strings.TrimSpace(value.Type) == "" {
		return false
	}
	class, ok := vm.lookupClass(value.Type)
	if !ok {
		return false
	}
	for _, iface := range vm.resolvedInterfaceNamesInHierarchy(class) {
		if statefulInterfaceName(iface) {
			return true
		}
	}
	return false
}

func (vm *VM) requiresDatabaseBatchableContract(value Value) bool {
	if value.Kind != ValueObject || strings.TrimSpace(value.Type) == "" || userProvisioningBatchableType(value.Type) {
		return false
	}
	class, ok := vm.lookupClass(value.Type)
	if !ok {
		return false
	}
	for _, iface := range vm.resolvedInterfaceNamesInHierarchy(class) {
		if batchableInterfaceName(iface) {
			return true
		}
	}
	return false
}

func (vm *VM) databaseBatchableItemType(value Value) string {
	if value.Kind != ValueObject || strings.TrimSpace(value.Type) == "" {
		return "Object"
	}
	class, ok := vm.lookupClass(value.Type)
	if !ok {
		return "Object"
	}
	for _, iface := range vm.resolvedInterfaceNamesInHierarchy(class) {
		if !batchableInterfaceName(iface) {
			continue
		}
		args, ok := genericTypeArgs(iface)
		if ok && len(args) == 1 && strings.TrimSpace(args[0]) != "" {
			return strings.TrimSpace(args[0])
		}
	}
	return "Object"
}

func (vm *VM) validateDatabaseBatchableContract(typeName, itemType string) error {
	starts := vm.batchableConcreteMethodsByName(typeName, "start")
	if len(starts) == 0 {
		return fmt.Errorf("Database.Batchable %s missing start", typeName)
	}
	if !vm.runtimeBatchableMethodCompatible("start", itemType, starts) {
		return fmt.Errorf("Database.Batchable %s invalid start signature", typeName)
	}
	executes := vm.batchableConcreteMethodsByName(typeName, "execute")
	if len(executes) == 0 {
		return fmt.Errorf("Database.Batchable %s missing execute", typeName)
	}
	if !vm.runtimeBatchableMethodCompatible("execute", itemType, executes) {
		return fmt.Errorf("Database.Batchable %s invalid execute signature", typeName)
	}
	finishes := vm.batchableConcreteMethodsByName(typeName, "finish")
	if len(finishes) == 0 {
		return fmt.Errorf("Database.Batchable %s missing finish", typeName)
	}
	if !vm.runtimeBatchableMethodCompatible("finish", itemType, finishes) {
		return fmt.Errorf("Database.Batchable %s invalid finish signature", typeName)
	}
	return nil
}

func (vm *VM) batchableConcreteMethodsByName(typeName, methodName string) []Method {
	var out []Method
	seen := make(map[string]bool)
	for current := typeName; current != ""; {
		if seen[current] {
			break
		}
		seen[current] = true
		class, ok := vm.lookupClass(current)
		if !ok {
			break
		}
		for _, method := range vm.registeredMethodCandidates(current + "." + methodName) {
			if method.IsStatic || !strings.EqualFold(apexMethodMemberName(method.Name), methodName) {
				continue
			}
			if methodHasModifier(method.Modifiers, "abstract") {
				continue
			}
			out = append(out, method)
		}
		current = vm.resolvedSuperClassName(class)
	}
	return out
}

func (vm *VM) runtimeBatchableMethodCompatible(methodName, itemType string, methods []Method) bool {
	for _, method := range methods {
		switch strings.ToLower(methodName) {
		case "start":
			if len(method.Params) != 1 || !runtimeBatchableSameType(method.Params[0].Type, "Database.BatchableContext") {
				continue
			}
			if strings.EqualFold(method.ReturnType, "Database.QueryLocator") || vm.runtimeBatchableIterableReturn(itemType, method.ReturnType) {
				return true
			}
		case "execute":
			if len(method.Params) != 2 ||
				!runtimeBatchableSameType(method.Params[0].Type, "Database.BatchableContext") ||
				!vm.runtimeBatchableScopeTypeCompatible(itemType, method.Params[1].Type) {
				continue
			}
			if runtimeBatchableVoidReturn(method.ReturnType) {
				return true
			}
		case "finish":
			if len(method.Params) != 1 || !runtimeBatchableSameType(method.Params[0].Type, "Database.BatchableContext") {
				continue
			}
			if runtimeBatchableVoidReturn(method.ReturnType) {
				return true
			}
		}
	}
	return false
}

func (vm *VM) runtimeBatchableIterableReturn(itemType, returnType string) bool {
	if vm.typeAssignableTo(returnType, "Iterable<"+itemType+">") {
		return true
	}
	if !strings.EqualFold(collectionBase(returnType), "Iterable") {
		return vm.runtimeTypeImplementsIterableOf(returnType, itemType)
	}
	element, ok := collectionElementType(returnType)
	return ok && vm.runtimeBatchableItemAssignable(element, itemType)
}

func (vm *VM) runtimeTypeImplementsIterableOf(typeName, itemType string) bool {
	class, ok := vm.lookupClass(typeName)
	if !ok {
		if nested, nestedOK := vm.resolveOnlyNestedTypeName(typeName); nestedOK {
			class, ok = vm.lookupClass(nested)
		}
	}
	if !ok {
		return false
	}
	for _, iface := range vm.resolvedInterfaceNamesInHierarchy(class) {
		if !strings.EqualFold(collectionBase(iface), "Iterable") {
			continue
		}
		element, ok := collectionElementType(iface)
		if ok && vm.runtimeBatchableItemAssignable(element, itemType) {
			return true
		}
	}
	return false
}

func (vm *VM) runtimeBatchableScopeTypeCompatible(itemType, scopeType string) bool {
	if !strings.EqualFold(collectionBase(scopeType), "List") {
		return false
	}
	element, ok := collectionElementType(scopeType)
	if !ok {
		return false
	}
	if vm.runtimeBatchableItemAssignable(element, itemType) {
		return true
	}
	return runtimeBatchableSameType(itemType, "SObject") && runtimeBatchableSameType(element, "Object")
}

func (vm *VM) runtimeBatchableItemAssignable(from, to string) bool {
	return runtimeBatchableSameType(from, to) || vm.typeAssignableTo(from, to)
}

func runtimeBatchableVoidReturn(returnType string) bool {
	return strings.TrimSpace(returnType) == "" || strings.EqualFold(returnType, "void")
}

func runtimeBatchableSameType(left, right string) bool {
	return strings.EqualFold(canonicalRuntimePlatformType(left), canonicalRuntimePlatformType(right))
}

func (vm *VM) resolvedInterfaceNamesInHierarchy(class Class) []string {
	seenTypes := make(map[string]bool)
	seenInterfaces := make(map[string]bool)
	var out []string
	for {
		typeName := runtimeClassName(class)
		if strings.TrimSpace(typeName) == "" {
			typeName = class.Name
		}
		typeKey := strings.ToLower(strings.TrimSpace(typeName))
		if typeKey == "" || seenTypes[typeKey] {
			break
		}
		seenTypes[typeKey] = true
		for _, iface := range vm.resolvedInterfaceNames(class) {
			vm.appendResolvedInterfaceName(&out, seenInterfaces, iface)
		}
		superName := vm.resolvedSuperClassName(class)
		if strings.TrimSpace(superName) == "" {
			break
		}
		superClass, ok := vm.lookupClass(superName)
		if !ok {
			break
		}
		class = superClass
	}
	return out
}

func (vm *VM) appendResolvedInterfaceName(out *[]string, seen map[string]bool, interfaceName string) {
	interfaceName = strings.TrimSpace(interfaceName)
	if interfaceName == "" {
		return
	}
	key := strings.ToLower(interfaceName)
	if seen[key] {
		return
	}
	seen[key] = true
	*out = append(*out, interfaceName)
	lookupName := interfaceName
	if base, ok := genericBaseName(lookupName); ok {
		lookupName = base
	}
	interfaceClass, ok := vm.lookupClass(lookupName)
	if !ok {
		return
	}
	for _, parent := range vm.resolvedInterfaceNames(interfaceClass) {
		vm.appendResolvedInterfaceName(out, seen, parent)
	}
}

func statefulInterfaceName(name string) bool {
	name = strings.TrimSpace(name)
	if base, ok := genericBaseName(name); ok {
		name = base
	}
	return strings.EqualFold(name, "Database.Stateful") || strings.EqualFold(name, "Stateful")
}

func (vm *VM) beginBatchChunkTransaction() (storage.IsolationMark, bool) {
	if vm == nil || vm.Org == nil {
		return storage.IsolationMark{}, false
	}
	if vm.isolationJournal == nil || vm.isolationJournal.Org() != vm.Org {
		vm.isolationJournal = storage.NewIsolationJournal(vm.Org)
	}
	return vm.isolationJournal.Mark(), true
}

func (vm *VM) rollbackBatchChunkTransaction(mark storage.IsolationMark) error {
	if vm == nil || vm.Org == nil || vm.isolationJournal == nil {
		return nil
	}
	currentSequences := copyOrgIDSequences(vm.Org.IDSequences)
	if err := vm.isolationJournal.Rollback(mark); err != nil {
		return err
	}
	vm.applyMaxIDSequencesForJournalRollback(currentSequences)
	return nil
}

func (vm *VM) withAsyncLimitWindow(run func() (Value, error)) (Value, error) {
	parentLimits := vm.limits
	parentViolations := append([]LimitViolation(nil), vm.limitViolations...)
	vm.ResetLimits()
	value, err := run()
	vm.limits = parentLimits
	vm.limitViolations = parentViolations
	return value, err
}

func (vm *VM) batchScopeValues(value Value, result *Result) ([]Value, error) {
	switch {
	case value.Kind == ValueNull:
		return nil, nil
	case value.Kind == ValueList:
		return append([]Value(nil), value.List...), nil
	case value.Kind == ValueSet:
		return append([]Value(nil), value.Set...), nil
	case value.Kind == ValueObject && value.Type == "Database.QueryLocator":
		if records, ok := value.Fields["Records"]; ok && records.Kind == ValueList {
			scope := append([]Value(nil), records.List...)
			if query, ok := value.Fields["Query"]; ok && query.Kind == ValueString {
				vm.refreshQueryLocatorScopeQueriedFields(scope, query.Text)
			}
			return scope, nil
		}
		return nil, nil
	case value.Kind == ValueObject:
		iterator := value
		if !isIteratorValue(iterator) {
			var err error
			iterator, err = vm.iteratorForObject(value, result)
			if err != nil {
				return nil, err
			}
		}
		return vm.collectIteratorValues(iterator, result)
	default:
		return nil, fmt.Errorf("Database.Batchable.start returned unsupported scope %s", value.Kind)
	}
}
func (vm *VM) refreshQueryLocatorScopeQueriedFields(scope []Value, queryText string) {
	if vm == nil || strings.TrimSpace(queryText) == "" {
		return
	}
	queriedFields := vm.queriedSObjectFields(queryText)
	if len(queriedFields) == 0 {
		return
	}
	for i := range scope {
		if scope[i].Kind != ValueObject {
			continue
		}
		objectName := scope[i].Type
		if objectName == "" {
			objectName = "SObject"
		}
		if scope[i].Fields == nil {
			scope[i].Fields = make(map[string]Value)
		}
		scope[i].Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(objectName, queriedFields)
		vm.hydrateQueriedRecordTypeRelationships(scope[i])
		vm.applyQueriedParentRelationshipFieldMarkers(&scope[i], queryText)
	}
}
func (vm *VM) collectIteratorValues(iterator Value, result *Result) ([]Value, error) {
	const iteratorName = "__glade_batch_iterator"
	previousIterator, hadIterator := vm.Globals[iteratorName]
	previousIteratorType, hadIteratorType := vm.VarTypes[iteratorName]
	defer func() {
		if hadIterator {
			vm.Globals[iteratorName] = previousIterator
		} else {
			delete(vm.Globals, iteratorName)
		}
		if hadIteratorType {
			vm.VarTypes[iteratorName] = previousIteratorType
		} else {
			delete(vm.VarTypes, iteratorName)
		}
	}()
	vm.Globals[iteratorName] = iterator
	vm.VarTypes[iteratorName] = iterator.Type
	values := []Value{}
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return nil, fmt.Errorf("batch iterable exceeded %d iterations", maxLoopIterations)
		}
		hasNext, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "hasNext", nil, result)
		if err != nil {
			return nil, err
		}
		if !handled || hasNext.Kind != ValueBool {
			return nil, fmt.Errorf("batch iterable requires Boolean hasNext")
		}
		if !hasNext.Bool {
			return values, nil
		}
		value, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "next", nil, result)
		if err != nil {
			return nil, err
		}
		if !handled {
			return nil, fmt.Errorf("batch iterable requires next")
		}
		values = append(values, value)
	}
}
func (vm *VM) emitBatchApexErrorEvent(job AsyncJob, scope []Value, phase string, cause error, result *Result) error {
	if vm.Org == nil || cause == nil {
		return nil
	}
	vm.ensureAsyncObjects()
	vm.ensureBatchApexErrorEventObject()
	record := storage.Record{
		Object: "BatchApexErrorEvent",
		Fields: map[string]storage.Value{
			"AsyncApexJobId": storage.IDValue(storage.ID(job.ID)),
			"JobScope":       storage.StringValue(batchErrorJobScope(scope)),
			"ExceptionType":  storage.StringValue(batchErrorExceptionType(cause)),
			"Message":        storage.StringValue(cause.Error()),
			"Phase":          storage.StringValue(phase),
			"StackTrace":     storage.StringValue(cause.Error()),
		},
	}
	_, err := vm.runTriggers(triggerTimingAfter, "insert", []storage.Record{record}, nil, result)
	return err
}
func (vm *VM) ensureBatchApexErrorEventObject() {
	if vm.Org == nil {
		return
	}
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "BatchApexErrorEvent",
		Label:     "Batch Apex Error Event",
		KeyPrefix: "1Be",
		Fields: map[string]storage.Field{
			"AsyncApexJobId": {APIName: "AsyncApexJobId", Type: storage.FieldReference, ReferenceTo: []string{"AsyncApexJob"}, RelationshipName: "AsyncApexJob"},
			"JobScope":       {APIName: "JobScope", Type: storage.FieldString},
			"ExceptionType":  {APIName: "ExceptionType", Type: storage.FieldString},
			"Message":        {APIName: "Message", Type: storage.FieldString},
			"Phase":          {APIName: "Phase", Type: storage.FieldString},
			"StackTrace":     {APIName: "StackTrace", Type: storage.FieldString},
		},
		Relations: []storage.Relationship{{
			Field:              "AsyncApexJobId",
			ParentObjects:      []string{"AsyncApexJob"},
			ParentRelationship: "AsyncApexJob",
		}},
	})
}
func batchErrorJobScope(scope []Value) string {
	parts := make([]string, 0, len(scope))
	for _, item := range scope {
		if id := sObjectIDFromFields(item.Fields); id != "" {
			parts = append(parts, string(id))
			continue
		}
		if text, ok := idValueText(item); ok && text != "" {
			parts = append(parts, text)
			continue
		}
		if item.Kind != ValueNull {
			parts = append(parts, item.String())
		}
	}
	return strings.Join(parts, ",")
}
func batchErrorExceptionType(err error) string {
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) && runtimeErr.Type != "" {
		return runtimeErr.Type
	}
	return "Exception"
}
func batchChunks(values []Value, size int) [][]Value {
	if size <= 0 {
		size = 200
	}
	if len(values) == 0 {
		return nil
	}
	var chunks [][]Value
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}
func batchArgs(method Method, contextType, jobID string) []Value {
	if len(method.Params) == 0 {
		return nil
	}
	return []Value{asyncContext(contextType, jobID)}
}
func batchExecuteArgs(method Method, scope []Value, jobID string) []Value {
	switch len(method.Params) {
	case 0:
		return nil
	case 1:
		return []Value{List(scope...)}
	default:
		return []Value{asyncContext("Database.BatchableContext", jobID), List(scope...)}
	}
}
func asyncContext(typeName, jobID string) Value {
	ctx := Object(typeName)
	if jobID != "" {
		ctx.Fields["JobId"] = String(jobID)
	}
	return ctx
}
func finalizerContext(jobID string, parentErr error) Value {
	ctx := asyncContext("FinalizerContext", jobID)
	ctx.Fields["Result"] = parentJobResultValue("SUCCESS")
	ctx.Fields["Exception"] = Null
	if parentErr != nil {
		ctx.Fields["Result"] = parentJobResultValue("UNHANDLED_EXCEPTION")
		exception := Object("Exception")
		exception.Fields["message"] = String(parentErr.Error())
		ctx.Fields["Exception"] = exception
	}
	return ctx
}
func parentJobResultValue(name string) Value {
	value := Value{Kind: ValueObject, Type: "ParentJobResult", Text: name}
	value.Fields = map[string]Value{"ordinal": Int(0)}
	if name == "UNHANDLED_EXCEPTION" {
		value.Fields["ordinal"] = Int(1)
	}
	return value
}
func schedulableContext(jobID string) Value {
	ctx := Object("SchedulableContext")
	if jobID != "" {
		ctx.Fields["TriggerId"] = String(cronTriggerID(jobID))
	}
	return ctx
}
func cronTriggerID(jobID string) string {
	return strings.Replace(jobID, "707", "08e", 1)
}
func (vm *VM) nextAsyncJobID() string {
	if vm.testContext != nil {
		vm.testContext.JobSeq++
		return fmt.Sprintf("707%012d", vm.testContext.JobSeq)
	}
	vm.localAsyncSeq++
	return fmt.Sprintf("707%012d", vm.localAsyncSeq)
}
func (vm *VM) recordAsyncJob(job AsyncJob, status, detail string) {
	if vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["AsyncApexJob"]
	id := storage.ID(job.ID)
	record, exists := object.Records[id]
	if record.ID == "" {
		record = storage.Record{ID: storage.ID(job.ID), Object: "AsyncApexJob", Fields: make(map[string]storage.Value)}
	}
	vm.recordIsolationJournalMutation("AsyncApexJob", id, record, exists)
	if record.System.CreatedDate == "" {
		record.System.CreatedDate = vm.fakeNow.Format(time.RFC3339)
	}
	if record.System.CreatedByID == "" {
		record.System.CreatedByID = storage.ID(vm.currentUserInfoField("Id", "005000000000001"))
	}
	record.System.LastModifiedDate = vm.fakeNow.Format(time.RFC3339)
	record.System.LastModifiedByID = record.System.CreatedByID
	record.System.SystemModstamp = record.System.LastModifiedDate
	className := asyncClassName(job)
	apexClassID := vm.recordApexClass(className)
	record.Fields["Status"] = storage.StringValue(status)
	record.Fields["JobType"] = storage.StringValue(asyncJobType(job))
	record.Fields["ApexClassId"] = storage.IDValue(apexClassID)
	record.Fields["ApexClassName"] = storage.StringValue(className)
	record.Fields["MethodName"] = storage.StringValue(asyncMethodName(job))
	if job.Kind == "ScheduledApex" || job.Kind == "ScheduledBatch" {
		record.Fields["CronTriggerId"] = storage.IDValue(storage.ID(cronTriggerID(job.ID)))
	} else {
		delete(record.Fields, "CronTriggerId")
	}
	if job.ParentJobID != "" {
		record.Fields["ParentJobId"] = storage.IDValue(storage.ID(job.ParentJobID))
	} else {
		delete(record.Fields, "ParentJobId")
	}
	if job.LastProcessed != "" {
		record.Fields["LastProcessed"] = storage.StringValue(job.LastProcessed)
	} else {
		delete(record.Fields, "LastProcessed")
	}
	if job.LastProcessedOffset > 0 {
		record.Fields["LastProcessedOffset"] = storage.IntegerValue(int64(job.LastProcessedOffset))
	} else {
		delete(record.Fields, "LastProcessedOffset")
	}
	if existing, ok := record.Fields["TotalJobItems"]; ok && existing.Kind == storage.ValueInteger && existing.Integer > 0 && asyncJobType(job) == "BatchApex" {
		record.Fields["TotalJobItems"] = existing
	} else {
		record.Fields["TotalJobItems"] = storage.IntegerValue(int64(asyncTotalItems(job)))
	}
	if status == "Completed" {
		record.Fields["JobItemsProcessed"] = record.Fields["TotalJobItems"]
		record.Fields["NumberOfErrors"] = storage.IntegerValue(0)
		record.Fields["CompletedDate"] = storage.DateTimeValue(vm.fakeNow.Format(time.RFC3339))
	} else if status == "Failed" {
		record.Fields["NumberOfErrors"] = storage.IntegerValue(1)
		record.Fields["ExtendedStatus"] = storage.StringValue(detail)
		record.Fields["CompletedDate"] = storage.DateTimeValue(vm.fakeNow.Format(time.RFC3339))
	} else if status == "Aborted" {
		record.Fields["CompletedDate"] = storage.DateTimeValue(vm.fakeNow.Format(time.RFC3339))
	} else {
		delete(record.Fields, "CompletedDate")
	}
	object.Records[record.ID] = record
	vm.Org.Objects["AsyncApexJob"] = object
}

func (vm *VM) recordBatchWorkerJob(parent AsyncJob, chunkIndex, processedItems int, chunk []Value, status, detail string) {
	if vm.Org == nil || asyncJobType(parent) != "BatchApex" || len(chunk) == 0 {
		return
	}
	worker := parent
	worker.ID = batchWorkerJobID(parent.ID, chunkIndex)
	worker.Kind = "BatchApexWorker"
	worker.ParentJobID = parent.ID
	worker.LastProcessed = batchLastProcessedID(chunk)
	worker.LastProcessedOffset = processedItems
	vm.recordAsyncJob(worker, status, detail)
	vm.recordAsyncJobTotals(worker, len(chunk), batchWorkerProcessedItems(status, len(chunk)), batchWorkerErrorCount(status))
}

func batchWorkerJobID(parentID string, index int) string {
	sum := sha1.Sum([]byte(parentID + ":" + strconv.Itoa(index)))
	return "707" + hex.EncodeToString(sum[:])[:12]
}

func batchLastProcessedID(chunk []Value) string {
	if len(chunk) == 0 {
		return ""
	}
	last := chunk[len(chunk)-1]
	if last.Kind == ValueObject {
		if id := sObjectIDFromFields(last.Fields); id != "" {
			return string(id)
		}
	}
	if text, ok := idValueText(last); ok {
		return text
	}
	return ""
}

func batchWorkerProcessedItems(status string, chunkSize int) int {
	if status == "Completed" {
		return chunkSize
	}
	return 0
}

func batchWorkerErrorCount(status string) int {
	if status == "Failed" {
		return 1
	}
	return 0
}

func (vm *VM) markCompletedBatchJobVisiblePendingInTest(job AsyncJob) {
	if vm.Org == nil {
		return
	}
	object := vm.Org.Objects["AsyncApexJob"]
	id := storage.ID(job.ID)
	record, exists := object.Records[id]
	if record.ID == "" {
		return
	}
	vm.recordIsolationJournalMutation("AsyncApexJob", id, record, exists)
	record.Fields["__GLADETestPendingStatus"] = storage.StringValue("Queued")
	object.Records[record.ID] = record
	vm.Org.Objects["AsyncApexJob"] = object
}
func (vm *VM) recordAsyncJobTotals(job AsyncJob, total, processed, errors int) {
	if vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["AsyncApexJob"]
	id := storage.ID(job.ID)
	record, exists := object.Records[id]
	if record.ID == "" {
		record = storage.Record{ID: storage.ID(job.ID), Object: "AsyncApexJob", Fields: make(map[string]storage.Value)}
	}
	vm.recordIsolationJournalMutation("AsyncApexJob", id, record, exists)
	record.Fields["TotalJobItems"] = storage.IntegerValue(int64(total))
	record.Fields["JobItemsProcessed"] = storage.IntegerValue(int64(processed))
	record.Fields["NumberOfErrors"] = storage.IntegerValue(int64(errors))
	object.Records[record.ID] = record
	vm.Org.Objects["AsyncApexJob"] = object
}
func (vm *VM) recordCronTrigger(job AsyncJob, state string) {
	if vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["CronTrigger"]
	id := storage.ID(cronTriggerID(job.ID))
	record, exists := object.Records[id]
	if record.ID == "" {
		record = storage.Record{ID: id, Object: "CronTrigger", Fields: make(map[string]storage.Value)}
	}
	vm.recordIsolationJournalMutation("CronTrigger", id, record, exists)
	record.Fields["State"] = storage.StringValue(state)
	record.Fields["CronExpression"] = storage.StringValue(job.Cron)
	record.Fields["CronJobDetail"] = storage.StringValue(job.Name)
	record.Fields["CronJobDetailId"] = storage.IDValue(storage.ID(cronJobDetailID(job.Name)))
	record.Fields["TimesTriggered"] = storage.IntegerValue(0)
	if nextFireTime, ok := cronNextFireTime(job.Cron, vm.fakeNow); ok {
		record.Fields["NextFireTime"] = storage.DateTimeValue(nextFireTime)
	}
	object.Records[record.ID] = record
	vm.Org.Objects["CronTrigger"] = object
	vm.recordCronJobDetail(job)
}
func cronNextFireTime(expr string, now time.Time) (string, bool) {
	parts := strings.Fields(expr)
	if len(parts) != 6 && len(parts) != 7 {
		return "", false
	}
	sec, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", false
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", false
	}
	hour, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", false
	}
	if sec < 0 || sec > 59 || min < 0 || min > 59 || hour < 0 || hour > 23 {
		return "", false
	}
	day, anyDay, ok := cronField(parts[3], 1, 31, true)
	if !ok {
		return "", false
	}
	month, anyMonth, ok := cronField(parts[4], 1, 12, false)
	if !ok {
		return "", false
	}
	weekday, anyWeekday, ok := cronField(parts[5], 1, 7, true)
	if !ok {
		return "", false
	}
	year, anyYear := 0, true
	if len(parts) == 7 {
		year, anyYear, ok = cronField(parts[6], 1970, 9999, false)
		if !ok {
			return "", false
		}
	}
	if !anyYear && !anyMonth && !anyDay && anyWeekday {
		candidate := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
		if candidate.After(now.UTC()) {
			return formatPlatformDatetime(candidate), true
		}
		return "", false
	}
	start := now.UTC().Truncate(24 * time.Hour)
	for offset := 0; offset < 3660; offset++ {
		candidateDay := start.AddDate(0, 0, offset)
		if !anyYear && candidateDay.Year() != year {
			continue
		}
		if !anyMonth && int(candidateDay.Month()) != month {
			continue
		}
		if !anyDay && candidateDay.Day() != day {
			continue
		}
		if !anyWeekday && salesforceCronWeekday(candidateDay) != weekday {
			continue
		}
		candidate := time.Date(candidateDay.Year(), candidateDay.Month(), candidateDay.Day(), hour, min, sec, 0, time.UTC)
		if candidate.After(now.UTC()) {
			return formatPlatformDatetime(candidate), true
		}
	}
	return "", false
}
func cronField(part string, min, max int, questionWildcard bool) (int, bool, bool) {
	if part == "*" || (questionWildcard && part == "?") {
		return 0, true, true
	}
	value, err := strconv.Atoi(part)
	if err != nil || value < min || value > max {
		return 0, false, false
	}
	return value, false, true
}
func salesforceCronWeekday(value time.Time) int {
	weekday := int(value.Weekday()) + 1
	if weekday == 8 {
		return 1
	}
	return weekday
}
func (vm *VM) recordCronJobDetail(job AsyncJob) {
	if vm.Org == nil || job.Name == "" {
		return
	}
	object := vm.Org.Objects["CronJobDetail"]
	id := storage.ID(cronJobDetailID(job.Name))
	if _, ok := object.Records[id]; ok {
		return
	}
	vm.recordIsolationJournalMutation("CronJobDetail", id, storage.Record{}, false)
	object.Records[id] = storage.Record{
		ID:     id,
		Object: "CronJobDetail",
		Fields: map[string]storage.Value{
			"Name":    storage.StringValue(job.Name),
			"JobType": storage.StringValue(cronJobType(job)),
		},
	}
	vm.Org.Objects["CronJobDetail"] = object
}
func asyncClassName(job AsyncJob) string {
	if job.Method.ClassName != "" {
		return job.Method.ClassName
	}
	return job.Object.Type
}
func asyncJobType(job AsyncJob) string {
	if job.Kind == "ScheduledBatch" {
		return "BatchApex"
	}
	return job.Kind
}
func asyncApexClassID(className string) string {
	sum := sha1.Sum([]byte(className))
	return "01p" + hex.EncodeToString(sum[:])[:12]
}
func cronJobDetailID(name string) string {
	sum := sha1.Sum([]byte(name))
	return "08a" + hex.EncodeToString(sum[:])[:12]
}
func cronJobType(job AsyncJob) string {
	if job.Kind == "ScheduledApex" || job.Kind == "ScheduledBatch" {
		return "7"
	}
	return "0"
}
func asyncMethodName(job AsyncJob) string {
	if job.Method.Name == "" {
		return "execute"
	}
	name := job.Method.Name
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		return name[dot+1:]
	}
	return name
}
func asyncTotalItems(job AsyncJob) int {
	if asyncJobType(job) != "BatchApex" || job.BatchSize <= 0 {
		return 1
	}
	return 1
}
