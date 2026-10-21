package apextest

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func applyApexTriggerRecords(org *storage.OrgState, index typesys.Index, caches ...*sourceCache) {
	if org == nil {
		return
	}
	storage.EnsureStandardObject(org, "ApexTrigger")
	state := org.Objects["ApexTrigger"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	generator := storage.NewIDGenerator(map[string]string{"ApexTrigger": "01q"})
	sources := sourceCacheFor(caches)
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		id, err := generator.Next("ApexTrigger")
		if err != nil {
			continue
		}
		matched := false
		for existingID, record := range state.Records {
			if strings.EqualFold(record.Fields["Name"].String, trigger.Name) &&
				strings.EqualFold(record.Fields["NamespacePrefix"].String, trigger.Namespace) {
				id = existingID
				matched = true
				break
			}
		}
		for !matched && state.Records[id].ID != "" {
			id, err = generator.Next("ApexTrigger")
			if err != nil {
				break
			}
		}
		if err != nil {
			continue
		}
		source := ""
		if trigger.File != "" {
			source, _ = sources.read(trigger.File)
		}
		if trigger.Dependency && !strings.EqualFold(index.Project.Namespace, trigger.Namespace) {
			source = ""
		}
		version := trigger.EffectiveAPIVersion
		if version == "" {
			version = index.Project.SourceAPIVersion
		}
		if version == "" {
			version = "65.0"
		}
		state.Records[id] = storage.Record{ID: id, Object: "ApexTrigger", Fields: map[string]storage.Value{
			"Name": storage.StringValue(trigger.Name), "NamespacePrefix": storage.StringValue(trigger.Namespace),
			"Body": storage.StringValue(source), "ApiVersion": storage.DecimalValue(version),
			"TableEnumOrId": storage.StringValue(trigger.ObjectName), "Status": storage.StringValue("Active"),
			"IsValid": storage.BooleanValue(true), "LengthWithoutComments": storage.IntegerValue(int64(len(source))),
		}}
	}
	org.Objects["ApexTrigger"] = state
}
