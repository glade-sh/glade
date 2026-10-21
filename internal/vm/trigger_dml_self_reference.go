package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Before-update records cannot recursively update or delete themselves. Keep
// immutable input identities through helper calls and nested trigger frames.
type beforeUpdateTriggerFrame struct {
	name   string
	object string
	ids    map[string]bool
}

func (vm *VM) enterBeforeUpdateTrigger(trigger Trigger, records []storage.Record) func() {
	if trigger.Timing != triggerTimingBefore || trigger.Operation != "update" {
		return func() {}
	}
	frame := beforeUpdateTriggerFrame{name: trigger.Name, object: trigger.Object, ids: make(map[string]bool, len(records))}
	if name, ok := vm.resolveObjectName(frame.object); ok {
		frame.object = name
	}
	for _, record := range records {
		if record.ID != "" {
			frame.ids[canonicalIDMapKey(string(record.ID))] = true
		}
	}
	depth := len(vm.beforeUpdateFrames)
	vm.beforeUpdateFrames = append(vm.beforeUpdateFrames, frame)
	return func() {
		vm.beforeUpdateFrames[depth] = beforeUpdateTriggerFrame{}
		vm.beforeUpdateFrames = vm.beforeUpdateFrames[:depth]
	}
}

func (vm *VM) preflightTriggerSelfReferenceFailures(operation string, records []storage.Record, failures []dml.Result) []dml.Result {
	if (operation != "update" && operation != "delete") || len(vm.beforeUpdateFrames) == 0 {
		return failures
	}
	for i, record := range records {
		if len(failures) > i && !failures[i].Success && failures[i].Error != "" {
			continue
		}
		object := record.Object
		if name, ok := vm.resolveObjectName(object); ok {
			object = name
		}
		key := canonicalIDMapKey(string(record.ID))
		for j := len(vm.beforeUpdateFrames) - 1; j >= 0; j-- {
			frame := vm.beforeUpdateFrames[j]
			if !strings.EqualFold(frame.object, object) || !frame.ids[key] {
				continue
			}
			if failures == nil {
				failures = make([]dml.Result, len(records))
				for k, input := range records {
					failures[k] = dml.Result{ID: input.ID, Success: true}
				}
			}
			id := string(record.ID)
			if len(id) == 18 {
				id = id[:15]
			}
			message := fmt.Sprintf("Object (id = %s) is currently in trigger %s, therefore it cannot recursively %s itself", id, frame.name, operation)
			failures[i] = dmlFailure(record.ID, message, "SELF_REFERENCE_FROM_TRIGGER", nil)
			break
		}
	}
	return failures
}
