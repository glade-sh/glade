package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// ApplyPlatformEventFlows runs subscribers on delivered event payloads, which
// are transient records rather than rows inserted into object storage.
func (e *Engine) ApplyPlatformEventFlows(records []storage.Record) error {
	if e == nil || e.Org == nil {
		return nil
	}
	for _, payload := range records {
		objectName, ok := storage.ResolveObjectName(*e.Org, payload.Object)
		if !ok {
			continue
		}
		behavior := e.Org.Objects[objectName].Definition.Metadata["publishBehavior"]
		if !strings.EqualFold(behavior, "PublishAfterCommit") && !strings.EqualFold(behavior, "PublishImmediately") {
			continue
		}
		record := payload.Clone()
		record.Object = objectName
		if err := e.applyTriggeredFlows(objectName, &record, "PlatformEvent", "platform-event"); err != nil {
			return err
		}
	}
	return nil
}

func flowTriggeredStepsSafe(rule storage.FlowRule, steps []storage.FlowStep, label string) bool {
	if label != "platform-event" {
		return flowCoreTriggeredStepsSafe(rule, steps, label)
	}
	// Named native observations cover sequential Apex actions receiving $Record.
	// Other platform-event Flow effects remain outside this dispatch boundary.
	if len(steps) == 0 {
		return false
	}
	for _, step := range steps {
		if step.Kind != "action" || !strings.EqualFold(step.Action.ActionType, "apex") {
			return false
		}
	}
	return true
}
