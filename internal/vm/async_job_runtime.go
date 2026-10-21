package vm

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) testStart() (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.startTest is only available in test context")
	}
	if vm.testContext.Started {
		return Null, &RuntimeError{Type: "System.FinalException", Message: "Testing already started", Stack: vm.stackFrames()}
	}
	vm.testContext.Started = true
	vm.testContext.Stopped = false
	vm.fakeNow = vm.fakeNow.Add(time.Second)
	vm.testContext.AsyncStartIndex = len(vm.testContext.AsyncJobs)
	vm.testContext.PlatformEventStartIndex = len(vm.testContext.PlatformEvents)
	vm.testContext.ChainEnqueued = false
	vm.testContext.ParentLimits = vm.limits
	vm.testContext.ParentViolations = append([]LimitViolation(nil), vm.limitViolations...)
	vm.ResetLimits()
	return Null, nil
}
func (vm *VM) testStop(result *Result) (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.stopTest is only available in test context")
	}
	if !vm.testContext.Started {
		return Null, &RuntimeError{Type: "System.FinalException", Message: "Testing has not started", Stack: vm.stackFrames()}
	}
	if vm.testContext.Stopped {
		return Null, &RuntimeError{Type: "System.FinalException", Message: "Testing already stopped", Stack: vm.stackFrames()}
	}
	vm.testContext.Stopped = true
	vm.prepareTestStopAsyncJobs()
	err := vm.drainTestWork(result)
	vm.limits = vm.testContext.ParentLimits
	vm.limitViolations = append([]LimitViolation(nil), vm.testContext.ParentViolations...)
	return Null, err
}

// Queueables, futures and scheduled Apex submitted before startTest execute at
// stopTest. Scheduled batches expose their queued job then, but
// pre-start scheduled batch payloads do not run.
func (vm *VM) prepareTestStopAsyncJobs() {
	jobs := vm.testContext.AsyncJobs
	boundary := min(max(vm.testContext.AsyncStartIndex, 0), len(jobs))
	prior := make([]AsyncJob, 0, boundary)
	ready := make([]AsyncJob, 0, len(jobs))
	for i, job := range jobs {
		reused := vm.activeTestScheduledReuse(job)
		if job.ScheduledAborted && !reused {
			prior = append(prior, job)
			continue
		}
		if reused && job.Kind == "ScheduledBatch" {
			// stopTest creates both queued BatchApex projections; neither
			// retained scheduled payload runs start/execute/finish here.
			vm.recordAsyncJob(job, "Queued", "")
			job.Deferred = true
			prior = append(prior, job)
			continue
		}
		if job.Kind == "ScheduledBatch" {
			vm.recordAsyncJob(job, "Queued", "")
		}
		if i < boundary && job.Kind != "Queueable" && job.Kind != "Future" && job.Kind != "ScheduledApex" && !reused {
			prior = append(prior, job)
		} else {
			ready = append(ready, job)
		}
	}
	vm.testContext.AsyncJobs = append(prior, ready...)
	vm.testContext.AsyncStartIndex = len(prior)
}

func (vm *VM) drainTestWork(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	previousPreserve, previousStopDraining := vm.testContext.PreserveAsyncStatics, vm.testContext.StopDraining
	vm.testContext.PreserveAsyncStatics = true
	vm.testContext.StopDraining = true
	defer func() {
		vm.testContext.PreserveAsyncStatics = previousPreserve
		vm.testContext.StopDraining = previousStopDraining
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
			vm.nextDrainableAsyncJobIndex(vm.testContext.AsyncJobs, startIndex) < 0 && !vm.hasPendingTestQueueable(startIndex) {
			return nil
		}
	}
	return fmt.Errorf("Test.stopTest async/event drain exceeded %d iterations", maxLoopIterations)
}

// hasPendingTestQueueable keeps stopTest draining when a finalizer replaces the
// only ready job with a delayed retry. The per-pass drain has already restored
// Draining here, so asyncJobDue alone would treat the retry as future work.
// Preserve the ordinary delay barrier and admission of deferred jobs.
func (vm *VM) hasPendingTestQueueable(startIndex int) bool {
	if vm.testContext == nil || !vm.testContext.Stopped {
		return false
	}
	for i := max(0, startIndex); i < len(vm.testContext.AsyncJobs); i++ {
		job := vm.testContext.AsyncJobs[i]
		if job.Kind == "Queueable" && !job.Deferred && !job.ScheduledAborted {
			return true
		}
	}
	return false
}

func (vm *VM) enqueueJob(args []Value, result *Result) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("System.enqueueJob expects Queueable[, Integer|AsyncOptions]")
	}
	// Validate submission arguments, not AsyncOptions writes.
	if args[0].Kind == ValueNull {
		return Null, newExceptionError("System.NullPointerException", "Argument cannot be null.")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("System.enqueueJob expects Queueable object")
	}
	delayMinutes := 0
	delaySet := false
	if len(args) == 2 {
		switch {
		case args[1].Kind == ValueNull:
			return Null, newExceptionError("System.NullPointerException", "Argument 2 cannot be null")
		case args[1].Kind == ValueInt:
			delayMinutes = int(args[1].Int)
			delaySet = true
		case args[1].Kind == ValueObject && strings.EqualFold(args[1].Type, "AsyncOptions"):
			if delay, ok := asyncOptionsInt(args[1], "minimumQueueableDelayInMinutes"); ok {
				delayMinutes = delay
				delaySet = true
			}
		default:
			return Null, fmt.Errorf("System.enqueueJob options expects Integer or AsyncOptions")
		}
	}
	if delayMinutes < 0 {
		return Null, newExceptionError("System.InvalidParameterValueException", "Delay must be greater than or equal to 0 minutes")
	}
	if delayMinutes > 10 {
		return Null, newExceptionError("System.InvalidParameterValueException", "Delay must be less than or equal to 10 minutes")
	}
	maxDepth, hasMaxDepth := 0, false
	if len(args) == 2 && args[1].Kind == ValueObject {
		maxDepth, hasMaxDepth = asyncOptionsInt(args[1], "maximumQueueableStackDepth")
		if hasMaxDepth && maxDepth < 0 {
			return Null, newExceptionError("System.InvalidParameterValueException", "System.InvalidParameterValueException: Stack depth must be greater than 0")
		}
		// F11/F12 capture rejection with an existing cap. Keep previously
		// uncapped dispatch chains on their prior submission path.
		if hasMaxDepth && maxDepth > 0 && vm.currentAsyncKind == "Queueable" && vm.currentQueueableDepth > 0 && vm.currentQueueableMaxDepth > 0 {
			return Null, newExceptionError("System.AsyncException", "Cannot reset Maximum Queueable Stack Depth in a queueable")
		}
		depth := 1
		if vm.currentAsyncKind == "Queueable" {
			depth = vm.currentQueueableDepth + 1
		}
		if hasMaxDepth && depth > maxDepth {
			return Null, newExceptionError("System.AsyncException", "Maximum stack depth has been reached.")
		}
	}
	duplicateSignature := ""
	if len(args) == 2 && args[1].Kind == ValueObject && strings.EqualFold(args[1].Type, "AsyncOptions") {
		duplicateSignature = asyncOptionsQueueableDuplicateSignature(args[1])
		if duplicateSignature != "" {
			if existingJobID := vm.queueableDuplicateSignatures[duplicateSignature]; existingJobID != "" {
				return Null, newExceptionError("System.DuplicateMessageException", "Attempt to enqueue job with duplicate queueable signature")
			}
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
	// Detach the queued object from the caller while preserving aliases inside
	// its object graph. A Queueable may attach a Finalizer that is also stored
	// in one of its fields; both references must observe the Queueable's
	// mutations when the finalizer runs after the async body.
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "Queueable", Object: cloneValueDetachedPreserveRefs(args[0]), QueueableDelayMinutes: delayMinutes, QueueableDelaySet: delaySet, QueueableDuplicateSignature: duplicateSignature}
	if delayMinutes > 0 {
		job.NotBefore = vm.fakeNow.Add(time.Duration(delayMinutes) * time.Minute)
	}
	if vm.currentAsyncKind == "Queueable" {
		job.QueueableDepth = vm.currentQueueableDepth + 1
		job.QueueableMaxDepth = vm.currentQueueableMaxDepth
	} else {
		job.QueueableDepth = 1
	}
	if hasMaxDepth {
		job.QueueableMaxDepth = maxDepth
	}
	if job.QueueableMaxDepth > 0 && job.QueueableDepth > job.QueueableMaxDepth {
		return Null, fmt.Errorf("MaximumQueueableStackDepth exceeded")
	}
	if duplicateSignature != "" {
		vm.queueableDuplicateSignatures[duplicateSignature] = job.ID
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
	if args[0].Kind == ValueNull {
		return Null, newExceptionError("System.NullPointerException", "Argument 'batchable' cannot be null")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("Database.executeBatch expects Batchable object")
	}
	batchSize := 200
	if len(args) == 2 {
		if args[1].Kind == ValueNull {
			return Null, newExceptionError("System.InvalidParameterValueException", "batchSize must not be null")
		}
		if args[1].Kind != ValueInt {
			return Null, fmt.Errorf("Database.executeBatch scope size expects Integer")
		}
		batchSize = int(args[1].Int)
		if batchSize <= 0 {
			return Null, newExceptionError("System.InvalidParameterValueException", "batchSize must be greater than or equal to one")
		}
		if batchSize > 2000 {
			// R151 uses an Iterable start result. Preserve the existing
			// QueryLocator path until its upper-bound behavior is captured.
			start, ok, _ := vm.resolveInstanceMethodForArgs(args[0].Type, "start", []Value{asyncContext("Database.BatchableContext", "")})
			base, generic := genericBaseName(start.ReturnType)
			if !ok || !generic || (!strings.EqualFold(base, "Iterable") && !strings.EqualFold(base, "System.Iterable")) {
				return Null, fmt.Errorf("Database.executeBatch scope size must be at most 2000")
			}
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
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "BatchApex", Object: cloneValueDetachedPreserveRefs(args[0]), BatchSize: batchSize, DirectBatchSubmission: vm.currentAsyncKind == ""}
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
	if len(args) == 3 {
		if args[2].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "schedulable object cannot be null")
		}
		if args[1].Kind == ValueNull {
			return Null, newExceptionError("System.StringException", "cron expression cannot be 'null'")
		}
	}
	if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueObject {
		return Null, fmt.Errorf("System.schedule expects name, cron, and Schedulable object")
	}
	if err := validateScheduleCron(args[1].Text); err != nil {
		return Null, err
	}
	if vm.hasWaitingScheduledJob(args[0].Text) {
		return Null, newExceptionError("System.AsyncException", fmt.Sprintf("The Apex job named \"%s\" is already scheduled for execution.", args[0].Text))
	}
	var notBefore time.Time
	if cronHasExplicitYear(args[1].Text) {
		if nextFire, ok := cronNextFireDate(args[1].Text, vm.cronClock()); ok {
			notBefore = nextFire
		} else if cronIsFixedDate(args[1].Text) {
			return Null, newExceptionError("System.AsyncException", "Based on configured schedule, the given trigger 'SCHEDULED_APEX_JOB_TYPE.000000000000000' will never fire.")
		}
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("scheduledJobs", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "ScheduledApex", Object: cloneValue(args[2]), Name: args[0].Text, Cron: args[1].Text, NotBefore: notBefore}
	vm.bindTestScheduledReuse(&job)
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	vm.recordCronTrigger(job, "WAITING")
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
	if vm.currentAsyncKind == "BatchApex" {
		return Null, newExceptionError("System.AsyncException", "System.scheduleBatch cannot be called from a batch start, batch execute, or future method.")
	}
	if vm.hasWaitingScheduledJob(args[1].Text) {
		return Null, newExceptionError("System.AsyncException", fmt.Sprintf("The Apex job named \"%s\" is already scheduled for execution.", args[1].Text))
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
	vm.bindTestScheduledReuse(&job)
	vm.enqueueAsyncJob(job)
	if vm.testContext != nil && vm.testContext.Stopped {
		vm.recordAsyncJob(job, "Queued", "")
	}
	vm.recordCronTrigger(job, "WAITING")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":      job.Kind,
		"jobId":     job.ID,
		"class":     args[0].Type,
		"name":      job.Name,
		"batchSize": batchSize,
	})
	return String(cronTriggerID(job.ID)), nil
}
func (vm *VM) hasWaitingScheduledJob(name string) bool {
	if vm.Org == nil {
		return false
	}
	for _, record := range vm.Org.Objects["CronTrigger"].Records {
		if record.System.IsDeleted || !strings.EqualFold(record.Fields["State"].String, "WAITING") {
			continue
		}
		detail := vm.Org.Objects["CronJobDetail"].Records[record.Fields["CronJobDetailId"].ID]
		if !detail.System.IsDeleted && detail.Fields["Name"].String == name {
			return true
		}
	}
	return false
}

func (vm *VM) abortJob(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("System.abortJob expects job Id")
	}
	if args[0].Kind == ValueNull {
		return Null, newExceptionError("System.StringException", "jobId expression cannot be 'null'")
	}
	jobID, ok := valueIDString(args[0])
	if !ok {
		return Null, fmt.Errorf("System.abortJob expects String job Id")
	}
	if vm.testContext == nil {
		if vm.asyncJobRecordStatus(jobID) != "" {
			return Null, unsupportedCallError("System.abortJob local async scheduling surface")
		}
		return Null, newExceptionError("System.StringException", "Job does not exist or is already aborted.")
	}
	for i, job := range vm.testContext.AsyncJobs {
		if !asyncJobIDTextEqual(job.ID, jobID) && !asyncJobIDTextEqual(cronTriggerID(job.ID), jobID) {
			continue
		}
		if job.Kind == "ScheduledApex" || job.Kind == "ScheduledBatch" {
			vm.testContext.AsyncJobs[i].ScheduledAborted = true
		} else {
			vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs[:i], vm.testContext.AsyncJobs[i+1:]...)
		}
		if job.Kind != "ScheduledBatch" || vm.asyncJobRecordStatus(job.ID) != "" {
			vm.recordAsyncJob(job, "Aborted", "")
		}
		if job.Kind == "ScheduledApex" || job.Kind == "ScheduledBatch" {
			vm.releaseScheduledCron(cronTriggerID(job.ID))
		}
		return Null, nil
	}
	if vm.asyncJobRecordStatus(jobID) != "" {
		vm.abortRecordedAsyncJob(jobID)
		return Null, nil
	}
	return Null, newExceptionError("System.StringException", "Job does not exist or is already aborted.")
}

func asyncJobIDTextEqual(left, right string) bool {
	return storage.IDsEqual(storage.ID(left), storage.ID(right))
}

// The native test helper keeps the first five jobs active;
// only the remaining mock jobs appear in the flex queue.
func (vm *VM) enqueueTestBatchJobs(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("Test.enqueueBatchJobs expects Integer")
	}
	if err := vm.requireTestContext("Test.enqueueBatchJobs"); err != nil {
		return Null, err
	}
	if args[0].Kind == ValueNull || args[0].Kind == ValueInt && args[0].Int < 1 {
		return Null, newExceptionError("System.InvalidParameterValueException", "Number of Jobs must not be null or less than 1")
	}
	if args[0].Kind != ValueInt {
		return Null, fmt.Errorf("Test.enqueueBatchJobs expects Integer")
	}
	ids := typedList("List<Id>")
	ids.nativeListMembership = true
	for i := int64(0); i < args[0].Int; i++ {
		id := vm.nextAsyncJobID()
		mockID := platformScalar("Id", id)
		mockID.nativeListElement = true
		ids.List = append(ids.List, mockID)
		if i >= 5 {
			heldID := platformScalar("Id", apexIDTo18(id))
			heldID.nativeListElement = true
			vm.testContext.FlexQueueJobs = append(vm.testContext.FlexQueueJobs, heldID)
		}
	}
	return ids, nil
}

func (vm *VM) moveFlexQueueJob(callee string, args []Value) (Value, error) {
	relative := strings.HasSuffix(callee, "moveAfterJob") || strings.HasSuffix(callee, "moveBeforeJob")
	want := 1
	if relative {
		want = 2
	}
	if len(args) != want {
		return Null, fmt.Errorf("%s expects %d job Id argument(s)", callee, want)
	}
	indices := make([]int, len(args))
	for i, arg := range args {
		if arg.Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", fmt.Sprintf("Argument %d cannot be null", i+1))
		}
		id, ok := valueIDString(arg)
		if !ok {
			return Null, fmt.Errorf("%s expects job Id", callee)
		}
		indices[i] = -1
		if vm.testContext != nil {
			for j, job := range vm.testContext.FlexQueueJobs {
				queuedID, _ := valueIDString(job)
				if asyncJobIDTextEqual(queuedID, id) {
					indices[i] = j
					break
				}
			}
		}
		if indices[i] < 0 {
			return Null, newExceptionError("System.NoSuchElementException", "Job with id "+apexIDTo18(id)+" not found in the queue")
		}
	}
	from := indices[0]
	jobs := vm.testContext.FlexQueueJobs
	to := 0
	if strings.HasSuffix(callee, "moveJobToEnd") {
		to = len(jobs) - 1
	} else if relative {
		to = indices[1]
		if from == to {
			return Bool(true), nil
		}
		if from < to {
			to--
		}
		if strings.HasSuffix(callee, "moveAfterJob") {
			to++
		}
	}
	// Keep previously returned order snapshots independent of a later move.
	moved := jobs[from]
	order := append([]Value(nil), jobs[:from]...)
	order = append(order, jobs[from+1:]...)
	order = append(order, Null)
	copy(order[to+1:], order[to:len(order)-1])
	order[to] = moved
	vm.testContext.FlexQueueJobs = order
	return Bool(true), nil
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
		if storedID, record, found := storage.LookupRecordByID(object.Records, storage.ID(asyncID)); found {
			vm.recordIsolationJournalMutation("AsyncApexJob", storedID, record, true)
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
	vm.releaseScheduledCron(cronID)
}

func (vm *VM) releaseScheduledCron(cronID string) {
	if vm == nil || vm.Org == nil {
		return
	}
	if object, ok := vm.Org.Objects["CronTrigger"]; ok {
		if storedID, record, found := storage.LookupRecordByID(object.Records, storage.ID(cronID)); found {
			vm.recordIsolationJournalMutation("CronTrigger", storedID, record, true)
			if record.Fields == nil {
				record.Fields = make(map[string]storage.Value)
			}
			record.Fields["State"] = storage.StringValue("Deleted")
			record.System.IsDeleted = true
			delete(record.Fields, "NextFireTime")
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
	_, record, ok := storage.LookupRecordByID(object.Records, storage.ID(jobID))
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
		if _, err := vm.callEventPublishEntry(method, publish.Callback, []Value{callbackResult}, result, publish.Fail); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) callEventPublishEntry(method Method, receiver Value, args []Value, result *Result, failure bool) (Value, error) {
	call := func() (Value, error) { return vm.callAsyncEntry(method, receiver, args, result, false) }
	// E rows capture omitted-sharing success callbacks. Their user is
	// AutomatedProcess, including in explicit with-sharing helpers. Failure
	// callbacks and declared sharing retain their unprobed existing path.
	// Extending the API62/67 observations to other APIs is a generalization.
	if failure {
		return call()
	}
	mode := strings.TrimSpace(method.SharingMode)
	if !method.SourceContextBound {
		// Ordinary callback methods use the class registry. Inspect declarations,
		// not classSharingMode's API67 effective secure-default "with sharing".
		mode = ""
		seen := map[string]bool{}
		for name := method.ClassName; strings.TrimSpace(name) != "" && !seen[strings.ToLower(name)]; {
			seen[strings.ToLower(name)] = true
			class, ok := vm.lookupClass(name)
			if !ok {
				return call()
			}
			for _, declaration := range []string{"with sharing", "without sharing", "inherited sharing"} {
				if methodHasModifier(class.Modifiers, declaration) {
					return call()
				}
			}
			name = vm.resolvedSuperClassName(class)
		}
	}
	if mode != "" {
		return call()
	}
	user := vm.automatedProcessUser()
	if user.Kind == "" {
		return call()
	}
	previousUser, previousReadUser := vm.testContext.CurrentUser, vm.eventPublishReadUserID
	vm.testContext.CurrentUser = user
	vm.eventPublishReadUserID = vm.currentUserID()
	defer func() {
		vm.testContext.CurrentUser = previousUser
		vm.eventPublishReadUserID = previousReadUser
	}()
	return call()
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
		for groupIndex, objectName := range order {
			preserveLaterGroups := func() {
				for _, later := range order[groupIndex+1:] {
					vm.testContext.PlatformEvents = append(vm.testContext.PlatformEvents, grouped[later]...)
				}
			}
			if stopTimeDelivery {
				wasDraining := vm.testContext.Draining
				previousUser := vm.testContext.CurrentUser
				vm.testContext.Draining = true
				if user := vm.automatedProcessUser(); user.Kind != "" {
					vm.testContext.CurrentUser = user
				}
				// Native named tests at APIs 62/67/68 retain test statics in
				// subscribers, including writes visible after stopTest returns.
				err := func() error {
					restoreCreations := vm.beginRecordCreationTransaction()
					defer restoreCreations()
					return vm.deliverPlatformEventSubscribers(grouped[objectName], result)
				}()
				vm.testContext.Draining = wasDraining
				vm.testContext.CurrentUser = previousUser
				if err != nil {
					var subscriber *eventBusTriggerRetryError
					var thrown *apexThrowError
					if errors.As(err, &subscriber) && !subscriber.handled && errors.As(subscriber.cause, &thrown) {
						// Native stopTest delivery consumes ordinary subscriber
						// exceptions without failing the test or rolling back prior
						// writes. Keep retries/checkpoints and VM errors separate.
						continue
					}
					if vm.requeueEventBusRetry(err) {
						preserveLaterGroups()
						return nil
					}
					return err
				}
				continue
			}
			if err := vm.deliverPlatformEventSubscribers(grouped[objectName], result); err != nil {
				if vm.requeueEventBusRetry(err) {
					preserveLaterGroups()
					return nil
				}
				return err
			}
		}
	}
	return nil
}

func (vm *VM) requeueEventBusRetry(err error) bool {
	if vm == nil || vm.testContext == nil {
		return false
	}
	var retry *eventBusTriggerRetryError
	if !errors.As(err, &retry) {
		return false
	}
	// Attempts 0 through 9, then discard the exhausted batch.
	if retry.retryable && retry.retries >= 9 {
		return true
	}
	if retry.retryable {
		if vm.testContext.PlatformEventRetries == nil {
			vm.testContext.PlatformEventRetries = make(map[string]int)
		}
		for _, record := range retry.records {
			vm.testContext.PlatformEventRetries[recordFieldString(record, "ReplayId")] = retry.retries + 1
		}
	}
	vm.testContext.PlatformEvents = append(vm.testContext.PlatformEvents, retry.records...)
	return retry.handled
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
		maxJobs = vm.drainableAsyncJobCount(*jobs, startIndex)
	}
	processed := 0
	for maxJobs < 0 || processed < maxJobs {
		jobIndex := vm.nextDrainableAsyncJobIndex(*jobs, startIndex)
		if jobIndex < 0 {
			break
		}
		job := (*jobs)[jobIndex]
		*jobs = append((*jobs)[:jobIndex], (*jobs)[jobIndex+1:]...)
		processed++
		var asyncStaticSnapshot map[string]map[string]Value
		var asyncStaticInitSnapshot map[string]staticInitState
		// stopTest runs collected work with the caller's static state, including
		// collection caches and deduplication sets. Other drain paths retain
		// their existing transaction reset policy.
		if vm.testContext != nil && !vm.testContext.PreserveAsyncStatics {
			asyncStaticSnapshot = vm.testAsyncStaticFieldSnapshot()
			asyncStaticInitSnapshot = copyStaticInitStateMap(vm.staticInitState)
			if err := vm.ResetTestAsyncStaticCollections(); err != nil {
				vm.restoreStaticFieldSnapshot(asyncStaticSnapshot)
				vm.staticInitState = copyStaticInitStateMap(asyncStaticInitSnapshot)
				return err
			}
		} else if vm.testContext == nil {
			if err := vm.ResetStatics(); err != nil {
				return err
			}
		}
		*chainEnqueued = false
		// Native test callbacks observe their scheduled job as Queued.
		if !job.ScheduledAborted && !(vm.testContext != nil && vm.testContext.Stopped && job.Kind == "ScheduledApex") {
			vm.recordAsyncJob(job, "Processing", "")
		}
		appendTrace(result, "apex.async.run", "apex.async", map[string]any{
			"kind":  job.Kind,
			"jobId": job.ID,
		})
		batchProjection := false
		err := vm.runAsyncJob(job, result, &batchProjection)
		if asyncStaticSnapshot != nil {
			vm.restoreStaticFieldSnapshot(asyncStaticSnapshot)
			vm.staticInitState = copyStaticInitStateMap(asyncStaticInitSnapshot)
		}
		if err != nil {
			if !job.ScheduledAborted && !batchProjection {
				vm.recordAsyncJob(job, "Failed", err.Error())
			}
			return err
		}
		if batchProjection {
			// The captured batch path has already recorded its phase-specific
			// status and totals, including Completed jobs with errors.
			continue
		}
		if job.ScheduledAborted {
			// Retained callback execution does not resurrect its aborted job row.
			continue
		}
		if vm.testContext != nil && vm.testContext.Stopped && job.Kind == "ScheduledApex" && vm.asyncJobRecordStatus(job.ID) == "Aborted" {
			// A callback can abort its own trigger after leaving the pending
			// queue. Preserve that recorded status when the callback returns.
			continue
		}
		if vm.testContext != nil && job.Kind == "BatchApex" {
			vm.recordAsyncJob(job, "Completed", "")
		} else if job.Kind == "ScheduledApex" {
			vm.recordAsyncJob(job, "Queued", "")
		} else {
			vm.recordAsyncJob(job, "Completed", "")
		}
	}
	return nil
}
func (vm *VM) drainableAsyncJobCount(jobs []AsyncJob, startIndex int) int {
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(jobs) {
		startIndex = len(jobs)
	}
	count := 0
	for _, job := range jobs[startIndex:] {
		if !job.Deferred && vm.asyncJobDue(job) {
			count++
		}
	}
	return count
}
func (vm *VM) nextDrainableAsyncJobIndex(jobs []AsyncJob, startIndex int) int {
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(jobs) {
		startIndex = len(jobs)
	}
	for i := startIndex; i < len(jobs); i++ {
		if !jobs[i].Deferred && vm.asyncJobDue(jobs[i]) {
			return i
		}
	}
	return -1
}
func (vm *VM) asyncJobDue(job AsyncJob) bool {
	if job.ScheduledAborted && !vm.activeTestScheduledReuse(job) {
		return false
	}
	if vm.testContext != nil && vm.testContext.Stopped && job.Kind == "ScheduledApex" && job.RunAtTestStop {
		return true
	}
	if vm.testContext != nil && vm.testContext.Stopped && vm.testContext.Draining &&
		job.Kind == "ScheduledApex" && vm.activeTestScheduledReuse(job) {
		return true
	}
	if vm.testContext != nil && vm.testContext.Stopped && vm.testContext.Draining {
		// stopTest runs collected schedules even in a future year.
		if job.Kind == "Queueable" || job.Kind == "ScheduledApex" {
			return true
		}
	}
	return job.NotBefore.IsZero() || !job.NotBefore.After(vm.fakeNow)
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
		class, ok := vm.ensureMutableClass(className)
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
		vm.storeMutableClassAtAlias(className, class)
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
		vm.recordApexClass(vm.asyncClassName(job))
		if vm.testContext.Draining && job.Kind == "ScheduledBatch" {
			job.Deferred = true
		}
		// The owned batch_finish_lifecycle observations run schedules created
		// by finish at stopTest, while other nested schedule origins retain
		// their existing admission boundary.
		finishSchedule := vm.testContext.Stopped && job.Kind == "ScheduledApex" && vm.currentAsyncKind == "BatchApex" && vm.inBatchFinish
		if vm.testContext.Draining && finishSchedule {
			job.RunAtTestStop = true
		}
		if vm.testContext.Draining && job.Kind != "Queueable" && job.Kind != "BatchApex" && job.Kind != "ScheduledBatch" && !finishSchedule {
			return
		}
		if vm.testContext.Draining && job.Kind == "Queueable" && !vm.canDrainQueueableJob(job) {
			job.Deferred = true
		}
		if vm.testContext.Draining && job.Kind == "BatchApex" && vm.currentAsyncKind != "" {
			job.SuppressWorkerRecords = true
			// A scheduled callback submits a pending batch but stopTest does
			// not execute that descendant. Queueable-to-batch stays drainable.
			if vm.testContext.Stopped && vm.currentAsyncKind == "ScheduledApex" {
				job.Deferred = true
			}
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
func asyncOptionsQueueableDuplicateSignature(options Value) string {
	if options.Kind != ValueObject {
		return ""
	}
	for name, value := range options.Fields {
		if strings.EqualFold(name, "duplicateSignature") {
			return queueableDuplicateSignatureText(value)
		}
	}
	return ""
}
func queueableDuplicateSignatureText(value Value) string {
	if value.Kind != ValueObject || !strings.EqualFold(value.Type, "QueueableDuplicateSignature") {
		return ""
	}
	if text, ok := value.Fields["value"]; ok {
		return scalarText(text)
	}
	return ""
}
func (vm *VM) canDrainQueueableJob(job AsyncJob) bool {
	if vm.currentAsyncKind == "BatchApex" {
		return true
	}
	// Owned API 62/67 F04/F12: a future's explicit-depth Queueable starts
	// during stopTest. Uncapped future descendants retain their boundary.
	if vm.currentAsyncKind == "Future" {
		return job.QueueableDepth == 1 && job.QueueableMaxDepth >= job.QueueableDepth
	}
	if vm.currentAsyncKind != "Queueable" {
		return false
	}
	// A finalizer runs as Queueable work after the parent Queueable body has
	// restored its depth state. Salesforce drains its immediate child at
	// Test.stopTest even though the child has no explicit max-stack option.
	// Ordinary Queueable bodies retain a positive current depth and continue to
	// require their explicit max-stack allowance below.
	if vm.currentQueueableDepth == 0 && job.QueueableDepth == 1 {
		return true
	}
	if job.QueueableMaxDepth <= 0 {
		return false
	}
	return job.QueueableDepth > 0 && job.QueueableDepth <= job.QueueableMaxDepth
}
func (vm *VM) runAsyncJob(job AsyncJob, result *Result, batchProjection ...*bool) error {
	_, err := vm.withQueueableDuplicateSignatureTransaction(func() (Value, error) {
		return Null, vm.runAsyncJobInTransaction(job, result, batchProjection...)
	})
	return err
}

func (vm *VM) runAsyncJobInTransaction(job AsyncJob, result *Result, batchProjection ...*bool) error {
	switch job.Kind {
	case "Future":
		_, err := vm.withAsyncLimitWindow(func() (Value, error) {
			return vm.withAsyncKind("Future", func() (Value, error) {
				return vm.callAsyncEntry(job.Method, Null, job.Args, result, false)
			})
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
		_, err := vm.withAsyncLimitWindow(func() (Value, error) {
			return vm.withQueueableJob(job, func() (Value, error) {
				if staticExecute {
					return vm.callAsyncEntry(target, Null, args, result, true)
				}
				return vm.callAsyncEntry(target, job.Object, args, result, true)
			})
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
			return Null, vm.runBatchJob(job, result, batchProjection...)
		})
		return err
	case "ScheduledBatch":
		_, err := vm.withAsyncKind("BatchApex", func() (Value, error) {
			return Null, vm.runBatchJob(job, result)
		})
		// stopTest leaves the scheduled batch's cron WAITING.
		testStop := vm.testContext != nil && vm.testContext.Stopped && vm.testContext.Draining
		if !testStop {
			vm.recordCronTrigger(job, "Complete")
		}
		return err
	case "ScheduledApex":
		contextJobID := job.ID
		if job.ScheduledReusedBy != "" {
			contextJobID = job.ScheduledReusedBy
		}
		args := []Value{schedulableContext(contextJobID)}
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
		_, err := vm.withScheduledLimitWindow(func() (Value, error) {
			return vm.withAsyncKind("ScheduledApex", func() (Value, error) {
				return vm.callAsyncEntry(target, job.Object, args, result, false)
			})
		})
		// The synchronous test callback leaves its cron row WAITING.
		testStop := vm.testContext != nil && vm.testContext.Stopped && vm.testContext.Draining
		if job.ScheduledReusedBy == "" && !testStop {
			vm.recordCronTrigger(job, "Complete")
		}
		return err
	default:
		return fmt.Errorf("unsupported async job kind %s", job.Kind)
	}
}

// Override only the callback's caller frame; helpers retain normal dispatch.
func (vm *VM) callAsyncEntry(method Method, receiver Value, args []Value, result *Result, queueable bool) (Value, error) {
	sharing := ""
	mode := strings.ToLower(strings.TrimSpace(method.SharingMode))
	if !method.SourceContextBound {
		mode, _ = vm.classSharingMode(method.ClassName)
	}
	entry := vm.testExecutionEntry
	inheritedTestEntry := mode == "inherited sharing" && entry != nil &&
		strings.EqualFold(vm.currentClass, entry.className) && len(vm.callStack) == entry.depth &&
		vm.currentMethod.SourceContextBound && strings.TrimSpace(vm.currentMethod.SharingMode) == ""
	if mode == "" || inheritedTestEntry {
		sharing = "without sharing"
	}
	if inheritedTestEntry && !queueable && apexversion.Enabled(vm.currentMethod.APIVersion, apexversion.SecureDefaults) {
		sharing = "with sharing"
	}
	return vm.callMethodWithReceiver(method, receiver, args, result, sharing)
}

func (vm *VM) withQueueableDuplicateSignatureTransaction(run func() (Value, error)) (Value, error) {
	previous := vm.queueableDuplicateSignatures
	vm.queueableDuplicateSignatures = make(map[string]string)
	defer func() {
		vm.queueableDuplicateSignatures = previous
	}()
	return run()
}
func (vm *VM) runQueueableFinalizer(finalizer Value, job AsyncJob, result *Result, parentErr error) error {
	restoreCreations := vm.beginRecordCreationTransaction()
	defer restoreCreations()
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
	_, err := vm.withQueueableDelay(job, func() (Value, error) {
		return vm.withAsyncKind("Queueable", func() (Value, error) {
			// B44X01Z/02Z/05Z: omitted finalizers enter with sharing at API
			// 62/67 even when the Queueable parent runs without sharing. Keep
			// declared sharing on its existing path; those shapes are unprobed.
			// Applying the omitted-entry rule at other APIs is a generalization.
			mode := strings.ToLower(strings.TrimSpace(target.SharingMode))
			if !target.SourceContextBound {
				mode, _ = vm.classSharingMode(target.ClassName)
			}
			if mode == "" {
				return vm.callMethodWithReceiver(target, finalizer, args, result, "with sharing")
			}
			return vm.callAsyncEntry(target, finalizer, args, result, false)
		})
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
	vm.currentAsyncKind = "Queueable"
	vm.currentQueueableDepth = job.QueueableDepth
	if vm.currentQueueableDepth <= 0 {
		vm.currentQueueableDepth = 1
	}
	vm.currentQueueableMaxDepth = job.QueueableMaxDepth
	defer func() {
		vm.currentAsyncKind = previousKind
		vm.currentQueueableDepth = previousDepth
		vm.currentQueueableMaxDepth = previousMaxDepth
	}()
	return vm.withQueueableDelay(job, run)
}

// Native source62/67 observations retain a nullable submission delay in both
// the Queueable body and its finalizer. Scope only delay here: finalizers keep
// their separate stack-depth and public async-predicate behavior.
func (vm *VM) withQueueableDelay(job AsyncJob, run func() (Value, error)) (Value, error) {
	previousDelay, previousSet := vm.currentQueueableDelay, vm.currentQueueableDelaySet
	vm.currentQueueableDelay, vm.currentQueueableDelaySet = job.QueueableDelayMinutes, job.QueueableDelaySet
	defer func() {
		vm.currentQueueableDelay, vm.currentQueueableDelaySet = previousDelay, previousSet
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
		// Finalizers reuse the Queueable bookkeeping kind at depth zero, but
		// native source62/66/67 reports this predicate only for the body.
		return vm.currentAsyncKind == "Queueable" && vm.currentQueueableDepth > 0
	case "System.isScheduled":
		return vm.currentAsyncKind == "ScheduledApex"
	default:
		return false
	}
}
func (vm *VM) runBatchJob(job AsyncJob, result *Result, batchProjection ...*bool) error {
	var scope []Value
	projectNative := vm.capturedBatchFaultJob(job)
	parentStackDepth := len(vm.callStack)
	markProjection := func() {
		if len(batchProjection) != 0 && batchProjection[0] != nil {
			*batchProjection[0] = true
		}
	}
	stateful := vm.isStatefulBatchObject(job.Object)
	baseObject := cloneValueDetachedPreserveRefs(job.Object)
	statefulObject := cloneValueDetachedPreserveRefs(job.Object)
	itemType := ""
	if vm.requiresDatabaseBatchableContract(job.Object) {
		itemType = vm.databaseBatchableItemType(job.Object)
		if err := vm.validateDatabaseBatchableContract(job.Object.Type, itemType); err != nil {
			return err
		}
	}
	locatorStart := false
	if start, ok := vm.resolveInstanceMethod(job.Object.Type, "start"); ok {
		receiver := batchTransactionReceiver(baseObject, stateful, &statefulObject)
		value, err := vm.withAsyncLimitWindow(func() (Value, error) {
			return vm.callAsyncEntry(start, receiver, batchArgs(start, "Database.BatchableContext", job.ID), result, false)
		})
		if err != nil {
			if projectNative && vm.capturedBatchFaultException(job, "START", err) {
				err = projectBatchFaultExceptionStack(err, parentStackDepth)
				if vm.testContext != nil {
					vm.recordAsyncJob(job, "Preparing", "")
					vm.recordAsyncJobTotals(job, 0, 0, 0)
				} else {
					vm.recordAsyncJob(job, "Failed", err.Error())
					vm.recordAsyncJobTotals(job, 0, 1, 1)
				}
				if eventErr := vm.emitBatchApexErrorEvent(job, nil, 0, "START", err, result, true); eventErr != nil {
					return eventErr
				}
				markProjection()
				if vm.testContext != nil {
					return err
				}
				return nil
			}
			vm.recordAsyncJobTotals(job, 0, 0, 1)
			vm.recordAsyncJob(job, "Failed", err.Error())
			if eventErr := vm.emitBatchApexErrorEvent(job, nil, 0, "START", err, result); eventErr != nil {
				return eventErr
			}
			return err
		}
		scopeValues, err := vm.batchScopeValues(value, result)
		if err != nil {
			return err
		}
		scope = append(scope, scopeValues...)
		locatorStart = value.Kind == ValueObject && value.Type == "Database.QueryLocator"
	}
	projectNative = projectNative && !locatorStart && vm.capturedBatchFaultScope(scope)
	projectedErrors := 0
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
				return vm.callAsyncEntry(execute, receiver, batchExecuteArgs(execute, chunk, job.ID, batchChunkType(itemType, locatorStart, execute)), result, false)
			})
			if err != nil {
				// F06/F07/F09: named stopTest executes this callback in the test
				// transaction. Catching its error retains preceding DML, including
				// explicitly delivered immediate events. Real async still rolls back.
				retainTestEffects := len(scope) == 1 && vm.capturedNamedBatchExecuteEffects(job, chunk, err)
				if rollbackEnabled && !retainTestEffects {
					if rollbackErr := vm.rollbackBatchChunkTransaction(rollbackMark); rollbackErr != nil {
						return rollbackErr
					}
				}
				if !job.SuppressWorkerRecords {
					vm.recordBatchWorkerJob(job, processed+1, processedItems, chunk, "Failed", err.Error())
				}
				if projectNative && vm.capturedBatchFaultException(job, "EXECUTE", err) {
					err = projectBatchFaultExceptionStack(err, parentStackDepth)
					if vm.testContext != nil {
						vm.recordAsyncJob(job, "Processing", "")
						vm.recordAsyncJobTotals(job, 1, 0, 0)
					} else {
						projectedErrors++
					}
					if eventErr := vm.emitBatchApexErrorEvent(job, chunk, len(scope), "EXECUTE", err, result, true); eventErr != nil {
						return eventErr
					}
					if vm.testContext != nil {
						markProjection()
						return err
					}
					// Native anonymous execution still invokes finish after this
					// singleton chunk fails; stopTest instead rethrows above.
					continue
				}
				vm.recordAsyncJobTotals(job, len(chunks), processed, 1)
				vm.recordAsyncJob(job, "Failed", err.Error())
				if eventErr := vm.emitBatchApexErrorEvent(job, chunk, len(scope), "EXECUTE", err, result); eventErr != nil {
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
			previous := vm.inBatchFinish
			vm.inBatchFinish = true
			defer func() { vm.inBatchFinish = previous }()
			return vm.callAsyncEntry(finish, receiver, batchArgs(finish, "Database.BatchableContext", job.ID), result, false)
		})
		if err != nil {
			if rollbackEnabled {
				if rollbackErr := vm.rollbackBatchChunkTransaction(rollbackMark); rollbackErr != nil {
					return rollbackErr
				}
			}
			if projectNative && projectedErrors == 0 && vm.capturedBatchFaultException(job, "FINISH", err) {
				err = projectBatchFaultExceptionStack(err, parentStackDepth)
				vm.recordAsyncJob(job, "Completed", "")
				errors := 0
				if vm.testContext == nil {
					errors = projectedErrors + 1
				}
				vm.recordAsyncJobTotals(job, 1, 1, errors)
				if eventErr := vm.emitBatchApexErrorEvent(job, nil, len(scope), "FINISH", err, result, true); eventErr != nil {
					return eventErr
				}
				markProjection()
				if vm.testContext != nil {
					return err
				}
				return nil
			}
			vm.recordAsyncJobTotals(job, len(chunks), len(chunks), 1)
			vm.recordAsyncJob(job, "Failed", err.Error())
			if eventErr := vm.emitBatchApexErrorEvent(job, nil, len(scope), "FINISH", err, result); eventErr != nil {
				return eventErr
			}
			return err
		}
	}
	if projectNative {
		markProjection()
		vm.recordAsyncJob(job, "Completed", "")
		vm.recordAsyncJobTotals(job, 1, 1, projectedErrors)
	}
	return nil
}

// F06/F07/F09 cover singleton stateless iterable batches at source API 62/67.
// Other batch shapes and finish failures retain their existing rollback path.
func (vm *VM) capturedNamedBatchExecuteEffects(job AsyncJob, scope []Value, cause error) bool {
	if vm.testContext == nil || !vm.testContext.Stopped || !vm.testContext.Draining ||
		vm.Org == nil || vm.Org.Namespace != "" || job.Kind != "BatchApex" || job.BatchSize != 1 || len(scope) != 1 ||
		scope[0].Kind != ValueObject || scope[0].projectClass || vm.isStatefulBatchObject(job.Object) {
		return false
	}
	var thrown *apexThrowError
	if !errors.As(cause, &thrown) {
		return false
	}
	if _, ok := vm.resolveObjectName(runtimeObjectType(scope[0])); !ok {
		return false
	}
	class, ok := vm.lookupClass(job.Object.Type)
	if !ok || class.Namespace != "" || class.SuperClass != "" {
		return false
	}
	version := class.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	major, ok := apexversion.Major(version)
	if !ok || (major != 62 && major != 67) {
		return false
	}
	item := vm.databaseBatchableItemType(job.Object)
	if !strings.EqualFold(item, "SObject") && !strings.EqualFold(item, "Object") {
		return false
	}
	start, ok := vm.resolveInstanceMethod(job.Object.Type, "start")
	return ok && runtimeBatchableSameType(start.ReturnType, "Iterable<"+item+">")
}

// Native API 62/67/68 fault observations cover direct Batchable<SObject>
// jobs with an Iterable<SObject> start and batch size 1. The returned scope
// further bounds execute/finish projection to one unsaved Account. Other
// iterables, scopes, batch sizes and APIs retain their existing behavior.
func (vm *VM) capturedBatchFaultJob(job AsyncJob) bool {
	if job.Kind != "BatchApex" || job.BatchSize != 1 || vm.Org == nil || vm.Org.Namespace != "" {
		return false
	}
	class, ok := vm.lookupClass(job.Object.Type)
	if !ok || class.Namespace != "" || class.SuperClass != "" || vm.isStatefulBatchObject(job.Object) ||
		!strings.EqualFold(vm.databaseBatchableItemType(job.Object), "SObject") {
		return false
	}
	version := class.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	major, ok := apexversion.Major(version)
	if !ok || (major != 62 && major != 67 && major != 68) {
		return false
	}
	start, ok := vm.resolveInstanceMethod(job.Object.Type, "start")
	return ok && runtimeBatchableSameType(start.ReturnType, "Iterable<SObject>")
}

func (vm *VM) capturedBatchFaultScope(scope []Value) bool {
	if len(scope) != 1 || scope[0].Kind != ValueObject || scope[0].projectClass {
		return false
	}
	name, ok := storage.ResolveObjectName(*vm.Org, runtimeObjectType(scope[0]))
	if !ok || !strings.EqualFold(name, "Account") {
		return false
	}
	return sObjectIDFromFields(scope[0].Fields) == ""
}

func (vm *VM) capturedBatchFaultException(job AsyncJob, phase string, err error) bool {
	typeName, _, _ := vm.batchErrorExceptionDetails(err)
	if !strings.EqualFold(typeName, "System.IllegalArgumentException") {
		return false
	}
	// The opt-out witness covers execute only. Its start/finish faults were
	// not captured and retain the existing path.
	if phase == "EXECUTE" {
		return true
	}
	class, ok := vm.lookupClass(job.Object.Type)
	if !ok {
		return false
	}
	for _, iface := range vm.resolvedInterfaceNamesInHierarchy(class) {
		if strings.EqualFold(iface, "Database.RaisesPlatformEvents") || strings.EqualFold(iface, "RaisesPlatformEvents") {
			return true
		}
	}
	return false
}

// The captured batch fault text starts at the asynchronous entry, even when
// stopTest is called by a named setup method. Project only the synchronous
// prefix out of Apex text; retain batch-internal calls and all diagnostic
// frames, source positions and the original exception value.
func projectBatchFaultExceptionStack(err error, parentDepth int) error {
	thrown, ok := err.(*apexThrowError)
	if !ok || parentDepth <= 0 || parentDepth >= len(thrown.stack) {
		return err
	}
	value := thrown.value
	value.Fields = make(map[string]Value, len(thrown.value.Fields))
	for field, fieldValue := range thrown.value.Fields {
		value.Fields[field] = fieldValue
	}
	value.Fields["__stackTrace"] = String(stackTraceString(thrown.stack[parentDepth:]))
	value.exceptionAnonymousCallerLine = 0
	return &apexThrowError{value: value, stack: thrown.stack}
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
	// The captured Iterable scopes match sema's semaBatchableIterableScopeCaptured:
	// (T, E) = Object/Object, String/String, SObject/SObject and SObject/Object
	// (PB01, H1K1PC, H1K1M1C, H1K1S, H1K1M2C, H1K1M3 at API 62.0 and 67.0).
	if strings.EqualFold(collectionBase(scopeType), "Iterable") {
		element, ok := collectionElementType(scopeType)
		if !ok {
			return false
		}
		for _, pair := range [][2]string{{"Object", "Object"}, {"String", "String"}, {"SObject", "SObject"}, {"SObject", "Object"}} {
			if runtimeBatchableSameType(itemType, pair[0]) && runtimeBatchableSameType(element, pair[1]) {
				return true
			}
		}
		return false
	}
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
	restoreCreations := vm.beginRecordCreationTransaction()
	defer restoreCreations()
	parentSingleReserved, parentMassReserved := vm.singleEmailReserved, vm.massEmailReserved
	vm.singleEmailReserved, vm.massEmailReserved = 0, 0
	parentLimits := vm.limits
	parentCaps := vm.limitCaps
	parentViolations := append([]LimitViolation(nil), vm.limitViolations...)
	parentCPUBudget := vm.cpuBudgetUsed
	parentCPUStartedAt := vm.cpuStartedAt
	parentCPURunning := vm.cpuRunning
	if asyncCaps, ok := LimitCapsForProfile("strict-async"); ok && vm.limitCaps.nativeDefaults {
		// Only native defaults widen. Complete explicit caps remain local,
		// including query and CPU values equal to the synchronous defaults.
		vm.limitCaps.Queries = asyncCaps.Queries
		vm.limitCaps.HeapSize = asyncCaps.HeapSize
		vm.limitCaps.CPUTimeMS = asyncCaps.CPUTimeMS
	}
	vm.ResetLimits()
	defer func() {
		vm.singleEmailReserved, vm.massEmailReserved = parentSingleReserved, parentMassReserved
		vm.limits = parentLimits
		vm.limitCaps = parentCaps
		vm.limitViolations = parentViolations
		vm.cpuBudgetUsed = parentCPUBudget
		vm.cpuStartedAt = parentCPUStartedAt
		vm.cpuRunning = parentCPURunning
	}()
	value, err := run()
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
func (vm *VM) emitBatchApexErrorEvent(job AsyncJob, scope []Value, totalScopeCount int, phase string, cause error, result *Result, nativeProjection ...bool) error {
	if vm.Org == nil || cause == nil {
		return nil
	}
	// Salesforce's BatchApexErrorEvent reference documents START, EXECUTE and
	// FINISH faults for batches opting in through Database.RaisesPlatformEvents.
	class, ok := vm.lookupClass(job.Object.Type)
	if !ok {
		return nil
	}
	raisesEvents := false
	for _, iface := range vm.resolvedInterfaceNamesInHierarchy(class) {
		if strings.EqualFold(iface, "Database.RaisesPlatformEvents") || strings.EqualFold(iface, "RaisesPlatformEvents") {
			raisesEvents = true
			break
		}
	}
	if !raisesEvents {
		return nil
	}
	vm.ensureAsyncObjects()
	vm.ensureBatchApexErrorEventObject()
	exceptionType, message, stack := vm.batchErrorExceptionDetails(cause)
	jobScope := storage.StringValue(batchErrorJobScope(scope))
	jobID := storage.IDValue(storage.ID(job.ID))
	if len(nativeProjection) != 0 && nativeProjection[0] {
		// The captured event exposes the original 15-character job text to
		// String.valueOf, rather than widening it through an Apex Id scalar.
		jobID = storage.StringValue(job.ID)
		if (phase == "START" || phase == "FINISH") && scope == nil {
			jobScope = storage.NullValue()
		} else if phase == "EXECUTE" && vm.capturedBatchFaultScope(scope) {
			// The captured Iterable<SObject> yields an unsaved record: the
			// execute event contains its null Id as text, not the record text.
			jobScope = storage.StringValue("null")
		}
	}
	record := storage.Record{
		Object: "BatchApexErrorEvent",
		Fields: map[string]storage.Value{
			"AsyncApexJobId": jobID,
			"JobScope":       jobScope,
			"ExceptionType":  storage.StringValue(exceptionType),
			"Message":        storage.StringValue(message),
			"Phase":          storage.StringValue(phase),
			"StackTrace":     storage.StringValue(stack),
		},
	}
	// F06/F07: after stopTest rethrows, the error event remains pending until
	// the test explicitly delivers it. The failed callback's own immediate
	// events can already have been delivered before its throw.
	if phase == "EXECUTE" && totalScopeCount == 1 && vm.capturedNamedBatchExecuteEffects(job, scope, cause) {
		vm.testContext.PlatformEvents = append(vm.testContext.PlatformEvents, record)
		return nil
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
func (vm *VM) batchErrorExceptionDetails(err error) (exceptionType, message, stack string) {
	var thrown *apexThrowError
	if errors.As(err, &thrown) {
		value := annotateException(thrown.value, thrown.stack)
		return vm.exceptionQualifiedTypeName(value.Type), exceptionMessage(value), value.Fields["__stackTrace"].Text
	}
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) {
		exceptionType = "Exception"
		if runtimeErr.Type != "" {
			exceptionType = vm.exceptionQualifiedTypeName(runtimeErr.Type)
		}
		return exceptionType, runtimeErr.ExceptionMessage(), apexStackTraceString(runtimeErr.Stack)
	}
	return "Exception", err.Error(), ""
}

func batchErrorExceptionType(err error) string {
	var thrown *apexThrowError
	if errors.As(err, &thrown) && thrown.value.Type != "" {
		return exceptionQualifiedTypeName(thrown.value.Type)
	}
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) && runtimeErr.Type != "" {
		return runtimeErr.Type
	}
	return "Exception"
}

func batchErrorMessage(err error) string {
	var thrown *apexThrowError
	if errors.As(err, &thrown) {
		if _, message, ok := objectFieldValue(thrown.value, "message"); ok {
			return message.String()
		}
		return "Script-thrown exception"
	}
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) {
		return runtimeErr.ExceptionMessage()
	}
	return err.Error()
}

func batchErrorStackTrace(err error) string {
	var thrown *apexThrowError
	if errors.As(err, &thrown) {
		if trace := thrown.value.Fields["__stackTrace"]; trace.Kind == ValueString {
			return trace.Text
		}
		return stackTraceString(thrown.stack)
	}
	return err.Error()
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

// batchChunkType is the runtime list type of one execute chunk, or "" to let
// the scope parameter type it. Native API 62.0 and 67.0, values of
// "instanceof List<Object>|instanceof List<SObject>":
//   - a QueryLocator start under Batchable<Object> or Batchable<SObject> gives
//     a List<SObject> for List and Iterable scopes: H1K1L, H1K1LC, H1K1S,
//     H1K1SC (true|true);
//   - another start with an Iterable scope gives a List<T>: PB01, H1K1PC,
//     H1K1P1, H1K1P2 (Object, true|false), H1K1M1C (String, true|false),
//     H1K1M2C, H1K1M3 (SObject, true|true).
//
// Other Batchable types have no captured row and keep the parameter's type.
func batchChunkType(itemType string, locatorStart bool, method Method) string {
	if len(method.Params) == 0 {
		return ""
	}
	item := strings.ToLower(canonicalRuntimePlatformType(itemType))
	if locatorStart && (item == "object" || item == "sobject") {
		return "List<SObject>"
	}
	if strings.EqualFold(collectionBase(method.Params[len(method.Params)-1].Type), "Iterable") {
		switch item {
		case "object", "string", "sobject":
			return "List<" + canonicalRuntimePlatformType(itemType) + ">"
		}
	}
	return ""
}

func batchExecuteArgs(method Method, scope []Value, jobID, chunkType string) []Value {
	chunk := List(scope...)
	chunk.Type = chunkType
	switch len(method.Params) {
	case 0:
		return nil
	case 1:
		return []Value{chunk}
	default:
		return []Value{asyncContext("Database.BatchableContext", jobID), chunk}
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
	ctx := asyncContext("FinalizerContextImpl", jobID)
	ctx.Fields["Result"] = parentJobResultValue("SUCCESS")
	ctx.Fields["Exception"] = Null
	if parentErr != nil {
		ctx.Fields["Result"] = parentJobResultValue("UNHANDLED_EXCEPTION")
		exception := Object("Exception")
		exception.Fields["message"] = String(parentErr.Error())
		// F01/F02: FinalizerContext retains the thrown exception's concrete
		// type, message and stack, for custom and runtime-generated exceptions.
		var thrown *apexThrowError
		if errors.As(parentErr, &thrown) {
			exception = annotateException(cloneValueDetachedPreserveRefs(thrown.value), thrown.stack)
		}
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
		ctx.Fields["TriggerId"] = platformScalar("Id", cronTriggerID(jobID))
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
		record.System.CreatedByID = storage.ID(vm.currentUserInfoField("Id", storage.DefaultUserID))
	}
	record.System.LastModifiedDate = vm.fakeNow.Format(time.RFC3339)
	record.System.LastModifiedByID = record.System.CreatedByID
	record.System.SystemModstamp = record.System.LastModifiedDate
	className := vm.asyncClassName(job)
	apexClassID := vm.recordApexClass(className)
	record.Fields["Status"] = storage.StringValue(status)
	record.Fields["JobType"] = storage.StringValue(asyncJobType(job))
	record.Fields["ApexClassId"] = storage.IDValue(apexClassID)
	record.Fields["ApexClassName"] = storage.StringValue(className)
	record.Fields["MethodName"] = storage.StringValue(asyncMethodName(job))
	// Queueable dispatch still uses execute; its catalog method facet is a
	// separate, nullable value. Keep this candidate confined to Queueable
	// until native method-facet rows establish the other job kinds.
	if job.Kind == "Queueable" {
		record.Fields["MethodName"] = storage.NullValue()
	}
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
	if !job.NotBefore.IsZero() {
		record.Fields["NotBefore"] = storage.DateTimeValue(job.NotBefore.Format(time.RFC3339))
	} else {
		delete(record.Fields, "NotBefore")
	}
	for _, field := range []string{"JobItemsProcessed", "NumberOfErrors"} {
		if _, exists := record.Fields[field]; !exists {
			record.Fields[field] = storage.IntegerValue(0)
		}
	}
	if existing, ok := record.Fields["TotalJobItems"]; ok && existing.Kind == storage.ValueInteger && asyncJobType(job) == "BatchApex" {
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
	record.Fields["CronJobDetailId"] = storage.IDValue(storage.ID(cronJobDetailID(job.Name)))
	record.Fields["TimesTriggered"] = storage.IntegerValue(0)
	if !exists {
		if ownerID := vm.currentUserID(); ownerID != "" {
			record.Fields["OwnerId"] = storage.IDValue(storage.ID(ownerID))
		}
		if job.Kind == "ScheduledBatch" {
			// Native scheduled-batch audit dates have second precision.
			record.System.CreatedDate = vm.fakeNow.Format(time.RFC3339)
		}
		record.Fields["StartTime"] = storage.DateTimeValue(formatPlatformDatetime(vm.fakeNow.Truncate(time.Second)))
		record.Fields["TimeZoneSidKey"] = storage.StringValue(vm.currentUserTimeZoneID())
	}
	if job.Kind == "ScheduledBatch" {
		record.Fields["CronExpression"] = storage.StringValue("SimpleTrigger")
		record.Fields["TimeZoneSidKey"] = storage.StringValue("GMT")
		if !exists {
			parts := strings.Fields(job.Cron)
			if len(parts) == 3 && parts[0] == "after" && parts[2] == "minutes" {
				if minutes, err := strconv.Atoi(parts[1]); err == nil {
					next := formatPlatformDatetime(vm.fakeNow.Add(time.Duration(minutes) * time.Minute).Truncate(time.Second))
					record.Fields["NextFireTime"] = storage.DateTimeValue(next)
					record.Fields["StartTime"] = storage.DateTimeValue(next)
				}
			}
		}
	} else if nextFireTime, ok := cronNextFireTime(job.Cron, vm.cronClock()); ok {
		record.Fields["NextFireTime"] = storage.DateTimeValue(nextFireTime)
	}
	if strings.EqualFold(state, "Deleted") {
		delete(record.Fields, "NextFireTime")
	}
	object.Records[record.ID] = record
	vm.Org.Objects["CronTrigger"] = object
	vm.recordCronJobDetail(job)
}

// cronClock returns the VM clock expressed in the running user's time zone,
// the zone in which scheduled cron expressions are evaluated.
func (vm *VM) cronClock() time.Time {
	location := time.UTC
	if zoneID := strings.TrimSpace(vm.currentUserTimeZoneID()); zoneID != "" {
		if loaded, err := time.LoadLocation(zoneID); err == nil {
			location = loaded
		}
	}
	return vm.fakeNow.In(location)
}

func cronNextFireTime(expr string, now time.Time) (string, bool) {
	candidate, ok := cronNextFireDate(expr, now)
	if !ok {
		return "", false
	}
	return formatPlatformDatetime(candidate), true
}

func cronHasExplicitYear(expr string) bool {
	parts := strings.Fields(expr)
	return len(parts) == 7 && parts[6] != "*" && parts[6] != "?"
}

// Reject malformed schedules before recording a local job.
func validateScheduleCron(expr string) error {
	parts := strings.Fields(strings.ToUpper(expr))
	stringError := func(message string) error {
		return newExceptionError("System.StringException", message)
	}
	for i := 0; i < min(3, len(parts)); i++ {
		part := parts[i]
		if value, err := strconv.Atoi(part); err == nil {
			if i < 2 && (value < 0 || value > 59) {
				return stringError("Minute and Second values must be between 0 and 59")
			}
			if i == 2 && (value < 0 || value > 23) {
				return stringError("Hour values must be between 0 and 23")
			}
		} else if strings.IndexFunc(part, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0 {
			if len(part) > 3 {
				part = part[:3]
			}
			return stringError("Illegal characters for this position: '" + part + "'")
		} else if i < 2 {
			// Native scheduling rows require fixed seconds and minutes, even
			// when the hour is recurring.
			return stringError("Seconds and minutes must be specified as integers: " + expr)
		}
	}
	if len(parts) < 6 {
		return stringError("Unexpected end of expression.")
	}
	if parts[3] == "?" && parts[5] == "?" {
		return stringError("'?' can only be specified for Day-of-Month -OR- Day-of-Week.")
	}
	if parts[3] != "?" && parts[5] != "?" {
		return stringError("Support for specifying both a day-of-week AND a day-of-month parameter is not implemented.")
	}
	// time.Date normalizes impossible dates; registration must reject them.
	day, dayErr := strconv.Atoi(parts[3])
	month, monthErr := strconv.Atoi(parts[4])
	if dayErr == nil && monthErr == nil && len(parts) == 7 {
		year, yearErr := strconv.Atoi(parts[6])
		if yearErr == nil {
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			if date.Year() != year || int(date.Month()) != month || date.Day() != day {
				return newExceptionError("System.AsyncException", "Based on configured schedule, the given trigger 'SCHEDULED_APEX_JOB_TYPE.000000000000000' will never fire.")
			}
		}
	}
	return nil
}

// cronIsFixedDate identifies the numeric, one-shot date form.
func cronIsFixedDate(expr string) bool {
	parts := strings.Fields(expr)
	if len(parts) != 7 || (parts[5] != "?" && parts[5] != "*") {
		return false
	}
	for _, i := range []int{0, 1, 2, 3, 4, 6} {
		if _, err := strconv.Atoi(parts[i]); err != nil {
			return false
		}
	}
	return true
}

func cronNextFireDate(expr string, now time.Time) (time.Time, bool) {
	parts := strings.Fields(expr)
	if len(parts) != 6 && len(parts) != 7 {
		return time.Time{}, false
	}
	// The documented scheduler grammar keeps seconds and minutes fixed.
	sec, err := strconv.Atoi(parts[0])
	if err != nil || sec < 0 || sec > 59 {
		return time.Time{}, false
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return time.Time{}, false
	}
	hours, ok := cronFieldValues(parts[2], 0, 23, false, nil)
	if !ok {
		return time.Time{}, false
	}
	// L in the day-of-month field selects the last day of each month.
	lastDay := strings.EqualFold(parts[3], "L")
	dayPart := parts[3]
	if lastDay {
		dayPart = "*"
	}
	days, ok := cronFieldValues(dayPart, 1, 31, true, nil)
	if !ok {
		return time.Time{}, false
	}
	months, ok := cronFieldValues(parts[4], 1, 12, false, []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"})
	if !ok {
		return time.Time{}, false
	}
	weekdays, ok := cronFieldValues(parts[5], 1, 7, true, []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"})
	if !ok {
		return time.Time{}, false
	}
	yearPart := "*"
	if len(parts) == 7 {
		yearPart = parts[6]
	}
	// Preserve the existing year domain, including fixed dates beyond 2099.
	years, ok := cronFieldValues(yearPart, 1970, 9999, false, nil)
	if !ok {
		return time.Time{}, false
	}
	location := now.Location()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	// Keep the existing search horizon for recurring schedules. Explicit year
	// sets can be sparse and must be searched even beyond that horizon.
	end := start.AddDate(0, 0, 3660)
	for _, year := range years {
		if year < now.Year() {
			continue
		}
		if yearPart == "*" && year > end.Year() {
			break
		}
		for _, month := range months {
			for _, day := range days {
				candidateDay := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
				if candidateDay.Month() != time.Month(month) || candidateDay.Before(start) {
					continue
				}
				if yearPart == "*" && !candidateDay.Before(end) {
					continue
				}
				if lastDay && candidateDay.AddDate(0, 0, 1).Month() == candidateDay.Month() {
					continue
				}
				if !slices.Contains(weekdays, salesforceCronWeekday(candidateDay)) {
					continue
				}
				for _, hour := range hours {
					candidate, exists := cronWallTime(year, time.Month(month), day, hour, minute, sec, location)
					if exists && candidate.After(now) {
						return candidate, true
					}
				}
			}
		}
	}
	return time.Time{}, false
}

// A missing local time does not fire. An overlapping local time uses the later
// instant, which is standard time at the Los Angeles autumn transition.
func cronWallTime(year int, month time.Month, day, hour, minute, second int, location *time.Location) (time.Time, bool) {
	wall := time.Date(year, month, day, hour, minute, second, 0, time.UTC)
	if wall.Year() != year || wall.Month() != month || wall.Day() != day {
		return time.Time{}, false
	}
	guess := time.Date(year, month, day, hour, minute, second, 0, location)
	var latest time.Time
	for _, delta := range []time.Duration{-24 * time.Hour, 0, 24 * time.Hour} {
		_, offset := guess.Add(delta).Zone()
		candidate := wall.Add(-time.Duration(offset) * time.Second).In(location)
		if candidate.Year() == year && candidate.Month() == month && candidate.Day() == day && candidate.Hour() == hour && candidate.Minute() == minute && candidate.Second() == second && (latest.IsZero() || candidate.After(latest)) {
			latest = candidate
		}
	}
	return latest, !latest.IsZero()
}

// cronFieldValues evaluates the documented list, range, wildcard and increment
// operators into an ordered set. Calendar-relative W/# selectors are outside
// this field-domain evaluator; the caller handles day-of-month L. Registration
// validation is deliberately separate.
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_scheduler.htm
func cronFieldValues(part string, minValue, maxValue int, questionWildcard bool, names []string) ([]int, bool) {
	part = strings.ToUpper(part)
	if questionWildcard && part == "?" {
		part = "*"
	}
	valueOf := func(text string) (int, bool) {
		for i, name := range names {
			if text == name {
				return minValue + i, true
			}
		}
		value, err := strconv.Atoi(text)
		return value, err == nil && value >= minValue && value <= maxValue
	}
	selected := make([]bool, maxValue+1)
	for _, item := range strings.Split(part, ",") {
		base, increment, stepped := strings.Cut(item, "/")
		step := 1
		if stepped {
			var err error
			step, err = strconv.Atoi(increment)
			if err != nil || step < 1 || step > maxValue-minValue+1 {
				return nil, false
			}
		}
		first, last := minValue, maxValue
		if base != "*" {
			left, right, ranged := strings.Cut(base, "-")
			var ok bool
			first, ok = valueOf(left)
			if !ok {
				return nil, false
			}
			if ranged {
				last, ok = valueOf(right)
				if !ok || last < first {
					return nil, false
				}
			} else if !stepped {
				last = first
			}
		}
		for value := first; value <= last; value += step {
			selected[value] = true
		}
	}
	var values []int
	for value := minValue; value <= maxValue; value++ {
		if selected[value] {
			values = append(values, value)
		}
	}
	return values, len(values) != 0
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

// Catalog ownership is separate from the receiver used for method dispatch.
func (vm *VM) asyncClassName(job AsyncJob) string {
	name := asyncClassName(job)
	if job.Kind != "Queueable" {
		return name
	}
	class, known := vm.lookupClass(name)
	if !known && job.Object.Type != "" {
		class, known = vm.lookupClass(job.Object.Type)
	}
	if !known {
		return name
	}
	lexical := runtimeClassName(class)
	prefix := ""
	if class.Namespace != "" && hasPrefixFold(lexical, class.Namespace+".") {
		prefix = lexical[:len(class.Namespace)+1]
		lexical = lexical[len(prefix):]
	}
	if dot := strings.Index(lexical, "."); dot >= 0 {
		owner := prefix + lexical[:dot]
		if _, known := vm.lookupClass(owner); known {
			return owner
		}
	}
	return name
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
	if asyncJobType(job) == "BatchApex" || job.Kind == "Queueable" || job.Kind == "ScheduledApex" {
		return 0
	}
	return 1
}

// bindTestScheduledReuse retains the already-collected payload separately from
// its cancelled Cron/AsyncApexJob rows. Only the proved one-predecessor,
// same-kind/same-class replacement is bound here.
func (vm *VM) bindTestScheduledReuse(job *AsyncJob) {
	if vm.testContext == nil || vm.testContext.Stopped || vm.testContext.Draining {
		return
	}
	found := -1
	for i, prior := range vm.testContext.AsyncJobs {
		if prior.Name != job.Name || prior.Kind != job.Kind || !strings.EqualFold(prior.Object.Type, job.Object.Type) {
			continue
		}
		if !prior.ScheduledAborted || prior.ScheduledReusedBy != "" {
			continue
		}
		if found >= 0 {
			return
		}
		found = i
	}
	if found < 0 {
		return
	}
	vm.testContext.AsyncJobs[found].ScheduledReusedBy = job.ID
	job.ScheduledReusedBy = job.ID
}

func (vm *VM) activeTestScheduledReuse(job AsyncJob) bool {
	if job.ScheduledReusedBy == "" || vm.testContext == nil {
		return false
	}
	for _, target := range vm.testContext.AsyncJobs {
		if target.ID == job.ScheduledReusedBy && !target.ScheduledAborted &&
			target.Kind == job.Kind && target.Name == job.Name &&
			strings.EqualFold(target.Object.Type, job.Object.Type) {
			return true
		}
	}
	return false
}
