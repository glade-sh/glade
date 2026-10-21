package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The derived-field lifecycle remains a candidate until the independent native
// observations agree. Metadata flags, rather than value labels, own this rule.
func TestTaskPriorityFlagCandidateInsertUpdate(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	task := org.Objects["Task"]
	task.Definition = task.Definition.Clone()
	priority := task.Definition.Fields["Priority"]
	priority.DefaultValue = "Pulse"
	priority.PicklistValues = append(priority.PicklistValues, storage.PicklistValue{Value: "Pulse", Label: "Immediate attention", Active: true})
	task.Definition.Fields["Priority"] = priority
	org.Objects["Task"] = task
	configured := org.Objects["TaskPriority"]
	configured.Records["01J000000000099"] = storage.Record{ID: "01J000000000099", Object: "TaskPriority", Fields: map[string]storage.Value{"ApiName": storage.StringValue("Pulse"), "IsHighPriority": storage.BooleanValue(true)}}
	org.Objects["TaskPriority"] = configured
	engine := NewEngine(&org)
	for _, input := range []struct {
		priority string
		high     bool
	}{{"High", true}, {"Normal", false}, {"Low", false}, {"Pulse", true}, {"", true}} {
		t.Run(input.priority, func(t *testing.T) {
			fields := map[string]storage.Value{"Subject": storage.StringValue("flag observation")}
			if input.priority != "" {
				fields["Priority"] = storage.StringValue(input.priority)
			}
			result := engine.Insert([]storage.Record{{Object: "Task", Fields: fields}})[0]
			if !result.Success {
				t.Fatal(result.Error)
			}
			stored := org.Objects["Task"].Records[result.ID]
			if got := stored.Fields["IsHighPriority"]; got.Kind != storage.ValueBoolean || got.Boolean != input.high {
				t.Fatalf("stored metadata-derived flag=%v; want %t", got, input.high)
			}
			if input.high {
				sparse := engine.Update([]storage.Record{{Object: "Task", ID: result.ID, Fields: map[string]storage.Value{"Subject": storage.StringValue("changed subject")}}})[0]
				if !sparse.Success {
					t.Fatal(sparse.Error)
				}
				stored = org.Objects["Task"].Records[result.ID]
				if got := stored.Fields["IsHighPriority"]; got.Kind != storage.ValueBoolean || !got.Boolean {
					t.Fatalf("sparse updated flag=%v; want true", got)
				}
				updated := engine.Update([]storage.Record{{Object: "Task", ID: result.ID, Fields: map[string]storage.Value{"Priority": storage.StringValue("Low")}}})[0]
				if !updated.Success {
					t.Fatal(updated.Error)
				}
				stored = org.Objects["Task"].Records[result.ID]
				if got := stored.Fields["IsHighPriority"]; got.Kind != storage.ValueBoolean || got.Boolean {
					t.Fatalf("updated flag=%v; want false", got)
				}
			}
		})
	}
}

func TestTaskPriorityFlagCandidateMetadataControls(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["TaskPriority"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"01J000000000001": {Fields: map[string]storage.Value{"ApiName": storage.StringValue("High"), "IsHighPriority": storage.BooleanValue(false)}},
		"01J000000000002": {Fields: map[string]storage.Value{"ApiName": storage.StringValue("Pulse"), "IsHighPriority": storage.BooleanValue(true)}},
	}}
	engine := NewEngine(&org)
	for _, input := range []struct {
		object, priority string
		before, want     bool
	}{{"Task", "High", true, false}, {"Task", "Pulse", false, true}, {"Task", "Missing", true, true}, {"Owned__c", "Pulse", false, false}} {
		row := storage.Record{Object: input.object, Fields: map[string]storage.Value{"Priority": storage.StringValue(input.priority), "IsHighPriority": storage.BooleanValue(input.before)}}
		engine.applyTaskPriorityFlag(&row)
		if got := row.Fields["IsHighPriority"].Boolean; got != input.want {
			t.Fatalf("%s/%s=%t; want %t", input.object, input.priority, got, input.want)
		}
	}
}
