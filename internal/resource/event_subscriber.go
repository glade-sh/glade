package resource

import (
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// ApplyProjectEventSubscribers derives rows only for active source-backed,
// unnamespaced platform-event triggers. The native rows cover this metadata
// shape; an absent or inactive trigger does not create a subscriber.
func ApplyProjectEventSubscribers(org *storage.OrgState, index typesys.Index, readFile func(string) ([]byte, error)) error {
	if org == nil {
		return nil
	}
	if readFile == nil {
		readFile = os.ReadFile
	}
	hasEventTrigger := false
	for _, trigger := range index.Triggers {
		if trigger.HasSourceSnapshot() && trigger.File != "" && trigger.Namespace == "" && strings.HasSuffix(strings.ToLower(trigger.ObjectName), "__e") {
			hasEventTrigger = true
			break
		}
	}
	if !hasEventTrigger {
		syncProjectEventSubscribers(org, nil)
		return nil
	}
	var desired []storage.Record
	seen := make(map[string]bool)
	triggerIDs := eventSubscriberTriggerIDs(org, index.Triggers)
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() || trigger.File == "" || trigger.Namespace != "" || !strings.HasSuffix(strings.ToLower(trigger.ObjectName), "__e") {
			continue
		}
		topic, ok := storage.ResolveObjectName(*org, trigger.ObjectName)
		if !ok {
			continue
		}
		path := trigger.File + "-meta.xml"
		input, captured := index.ApexMetadataForTrigger(trigger)
		body, err := readFile(path)
		if err != nil {
			if os.IsNotExist(err) && (!captured || !input.Present) {
				continue
			}
			return err
		}
		if captured && (!input.Present || sha256.Sum256(body) != input.Digest) {
			return fmt.Errorf("trigger metadata changed during project loading: %s", path)
		}
		var metadata struct {
			XMLName xml.Name `xml:"ApexTrigger"`
			Status  string   `xml:"status"`
		}
		if err := xml.Unmarshal(body, &metadata); err != nil {
			return fmt.Errorf("invalid trigger metadata %s: %w", path, err)
		}
		if !strings.EqualFold(strings.TrimSpace(metadata.Status), "Active") {
			continue
		}
		key := strings.ToLower(trigger.Name + "\x00" + topic)
		if seen[key] {
			continue
		}
		triggerID := triggerIDs[eventTriggerIdentityKey(trigger)]
		if triggerID == "" {
			continue
		}
		seen[key] = true
		desired = append(desired, storage.Record{Object: "EventBusSubscriber", MetadataSourceUnit: storage.ProjectEventSubscriberSource + string(triggerID), Fields: map[string]storage.Value{
			"Name": storage.StringValue(trigger.Name), "Topic": storage.StringValue(topic),
			"Type": storage.StringValue("ApexTrigger"), "Status": storage.StringValue("Running"),
			"Position": storage.IntegerValue(1), "LastError": storage.NullValue(),
		}})
	}
	sort.Slice(desired, func(i, j int) bool { return eventSubscriberKey(desired[i]) < eventSubscriberKey(desired[j]) })
	syncProjectEventSubscribers(org, desired)
	return nil
}

func eventTriggerIdentityKey(trigger typesys.TriggerSymbol) string {
	return strings.ToLower(trigger.File + "\x00" + trigger.Namespace + "\x00" + trigger.Name)
}

// Match the existing project ApexTrigger identity order; a saved or already
// materialized trigger takes precedence over a fresh deterministic identity.
func eventSubscriberTriggerIDs(org *storage.OrgState, triggers []typesys.TriggerSymbol) map[string]storage.ID {
	ids := make(map[string]storage.ID)
	used := make(map[storage.ID]bool)
	state := org.Objects["ApexTrigger"]
	existing := make(map[string]storage.ID, len(state.Records))
	for id, row := range state.Records {
		key := strings.ToLower(row.Fields["NamespacePrefix"].String + "\x00" + row.Fields["Name"].String)
		existing[key] = id
	}
	generator := storage.NewIDGenerator(map[string]string{"ApexTrigger": "01q"})
	for _, trigger := range triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		id, err := generator.Next("ApexTrigger")
		if err != nil {
			continue
		}
		existingID, matched := existing[strings.ToLower(trigger.Namespace+"\x00"+trigger.Name)]
		if matched {
			id = existingID
		}
		for !matched && (used[id] || state.Records[id].ID != "") {
			id, err = generator.Next("ApexTrigger")
			if err != nil {
				break
			}
		}
		if err == nil {
			used[id] = true
			ids[eventTriggerIdentityKey(trigger)] = id
		}
	}
	return ids
}

// SyncProjectEventSubscribers refreshes only locally synthesized subscriber
// rows on a saved org, preserving unrelated rows and existing stream state.
func SyncProjectEventSubscribers(org, projectOrg *storage.OrgState) {
	if org == nil || projectOrg == nil {
		return
	}
	var desired []storage.Record
	for _, record := range projectOrg.Objects["EventBusSubscriber"].Records {
		if storage.IsProjectEventSubscriber(record) {
			desired = append(desired, record.Clone())
		}
	}
	sort.Slice(desired, func(i, j int) bool { return eventSubscriberKey(desired[i]) < eventSubscriberKey(desired[j]) })
	syncProjectEventSubscribers(org, desired)
}

func eventSubscriberKey(record storage.Record) string {
	return strings.ToLower(record.Fields["Name"].String + "\x00" + record.Fields["Topic"].String + "\x00" + record.Fields["Type"].String)
}

func syncProjectEventSubscribers(org *storage.OrgState, desired []storage.Record) {
	state, exists := org.Objects["EventBusSubscriber"]
	if !exists && len(desired) == 0 {
		return
	}
	if exists && len(desired) == 0 {
		owned := false
		for _, record := range state.Records {
			if storage.IsProjectEventSubscriber(record) {
				owned = true
				break
			}
		}
		if !owned {
			return
		}
	}
	if !exists {
		storage.EnsureStandardObject(org, "EventBusSubscriber")
	}
	mutable, _ := storage.EnsureMutableObjectRecords(org, "EventBusSubscriber")
	if mutable == nil {
		return
	}
	state = *mutable
	if state.Records == nil {
		state.SetRecords(make(map[storage.ID]storage.Record))
	}
	old := make(map[string]storage.Record)
	unowned := make(map[string]bool)
	for id, record := range state.Records {
		if storage.IsProjectEventSubscriber(record) {
			old[eventSubscriberKey(record)] = record
			state.DeleteRecord(id)
		} else {
			unowned[eventSubscriberKey(record)] = true
		}
	}
	generator := storage.NewIDGenerator(map[string]string{"EventBusSubscriber": state.Definition.KeyPrefix})
	for _, record := range desired {
		if unowned[eventSubscriberKey(record)] {
			continue
		}
		if previous, ok := old[eventSubscriberKey(record)]; ok && state.Records[previous.ID].ID == "" {
			record.ID = previous.ID
			if position, ok := previous.GetField("Position"); ok {
				record.Fields["Position"] = position
			}
		} else {
			record.ID = ""
			for {
				id, err := generator.Next("EventBusSubscriber")
				if err != nil {
					break
				}
				if state.Records[id].ID == "" {
					record.ID = id
					break
				}
			}
		}
		if record.ID != "" {
			state.PutRecord(record.ID, record)
		}
	}
	org.Objects["EventBusSubscriber"] = state
	storage.RebuildObjectIndexes(org, "EventBusSubscriber")
}
