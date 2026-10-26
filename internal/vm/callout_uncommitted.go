package vm

// Callout eligibility is transaction state, independent of governor counters.
// Rollback restores pending work, but the active savepoint still blocks callouts.
type calloutPendingWork struct {
	pending    bool
	savepoints map[string]bool
}

func (vm *VM) checkCalloutAllowed() error {
	if vm.calloutPendingWork.pending {
		return newExceptionError("CalloutException", "You have uncommitted work pending. Please commit or rollback before calling out")
	}
	if len(vm.calloutPendingWork.savepoints) > 0 {
		return newExceptionError("CalloutException", "All active Savepoints must be released before making callouts.")
	}
	return nil
}

// Savepoint operations consume DML statements without creating pending writes.
func (vm *VM) incrementSavepointStatement() error {
	pending := vm.calloutPendingWork.pending
	defer func() { vm.calloutPendingWork.pending = pending }()
	return vm.incrementLimit("dmlStatements", 1)
}

func (vm *VM) saveCalloutPendingWork(id string) {
	if vm.calloutPendingWork.savepoints == nil {
		vm.calloutPendingWork.savepoints = make(map[string]bool)
	}
	vm.calloutPendingWork.savepoints[id] = vm.calloutPendingWork.pending
}

// Native UW09 and UW11 capture System.enqueueJob(job) and the one-argument
// Messaging.sendEmail(List<SingleEmailMessage>) as pending work. Other
// enqueue and email overloads have no native row and keep base eligibility.
func (vm *VM) markCalloutPendingWork() {
	vm.calloutPendingWork.pending = true
}

func singleEmailMessageList(messages Value) bool {
	for _, message := range messages.List {
		if message.Type != "Messaging.SingleEmailMessage" {
			return false
		}
	}
	return true
}
