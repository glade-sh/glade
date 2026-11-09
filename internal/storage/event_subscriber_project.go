package storage

import (
	"math"
	"strings"
)

// ProjectEventSubscriberSource marks local rows synthesized from active Apex
// event triggers. It uses existing record provenance, not a new saved shape.
const ProjectEventSubscriberSource = "glade:event-subscriber:"

// EventSubscriberVisibleID is the captured query projection for these virtual
// rows. Their distinct internal IDs remain the keys used by local storage.
const EventSubscriberVisibleID ID = "000000000000000AAA"

func IsProjectEventSubscriber(record Record) bool {
	return strings.HasPrefix(record.MetadataSourceUnit, ProjectEventSubscriberSource) && strings.EqualFold(record.Object, "EventBusSubscriber")
}

// ProjectEventSubscriberTriggerID is the synthetic trigger identity used in
// the captured REST URL, separate from the subscriber's exposed zero ID.
func ProjectEventSubscriberTriggerID(record Record) ID {
	if !IsProjectEventSubscriber(record) {
		return ""
	}
	id := strings.TrimPrefix(record.MetadataSourceUnit, ProjectEventSubscriberSource)
	if (len(id) != 15 && len(id) != 18) || !strings.EqualFold(id[:3], "01q") {
		return ""
	}
	return ID(id)
}

// SetProjectEventSubscriberTestPositions resets only synthesized rows in the
// private test org. Native tests observe zero while anonymous execution sees a
// positive stream position.
func SetProjectEventSubscriberTestPositions(org *OrgState) {
	if org == nil {
		return
	}
	state, ok := org.Objects["EventBusSubscriber"]
	if !ok {
		return
	}
	need := false
	for _, record := range state.Records {
		if IsProjectEventSubscriber(record) {
			need = true
			break
		}
	}
	if !need {
		return
	}
	mutable, _ := EnsureMutableObjectRecords(org, "EventBusSubscriber")
	if mutable == nil {
		return
	}
	for id, record := range mutable.Records {
		if !IsProjectEventSubscriber(record) {
			continue
		}
		record = record.Clone()
		record.Fields["Position"] = IntegerValue(0)
		mutable.Records[id] = record
	}
	org.Objects["EventBusSubscriber"] = *mutable
	RebuildObjectIndexes(org, "EventBusSubscriber")
}

// AdvanceProjectEventSubscriberTestPositions advances only source-backed
// subscribers for the event topics delivered successfully in a test context.
func AdvanceProjectEventSubscriberTestPositions(org *OrgState, events []Record) {
	if org == nil || len(events) == 0 {
		return
	}
	state, ok := org.Objects["EventBusSubscriber"]
	if !ok {
		return
	}
	counts := make(map[string]int64)
	for _, event := range events {
		counts[strings.ToLower(event.Object)]++
	}
	needsWrite := false
	for _, record := range state.Records {
		if IsProjectEventSubscriber(record) && counts[strings.ToLower(record.Fields["Topic"].String)] > 0 {
			needsWrite = true
			break
		}
	}
	if !needsWrite {
		return
	}
	mutable, _ := EnsureMutableObjectRecords(org, "EventBusSubscriber")
	if mutable == nil {
		return
	}
	for id, record := range mutable.Records {
		count := counts[strings.ToLower(record.Fields["Topic"].String)]
		position, present := record.GetField("Position")
		if !IsProjectEventSubscriber(record) || count == 0 || !present || position.Kind != ValueInteger || position.Integer > math.MaxInt64-count {
			continue
		}
		record = record.Clone()
		record.Fields["Position"] = IntegerValue(position.Integer + count)
		mutable.Records[id] = record
	}
	org.Objects["EventBusSubscriber"] = *mutable
	RebuildObjectIndexes(org, "EventBusSubscriber")
}
