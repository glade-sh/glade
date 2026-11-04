package dml

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// BeforeSaveCheckboxNullFailure checks the captured custom-checkbox insert
// boundary before a record-triggered Flow can act on an explicit null.
func BeforeSaveCheckboxNullFailure(org *storage.OrgState, record storage.Record) (Result, bool) {
	if org == nil {
		return Result{}, false
	}
	var candidates []string
	for name, explicit := range record.ExplicitNulls {
		if explicit {
			candidates = append(candidates, name)
		}
	}
	for name, value := range record.Fields {
		if value.Kind == storage.ValueNull {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) == 0 {
		return Result{}, false
	}
	objectName, ok := storage.ResolveObjectName(*org, record.Object)
	if !ok {
		return Result{}, false
	}
	object := org.Objects[objectName]
	hasFlow := false
	for _, rule := range object.Definition.FlowRules {
		if rule.Active && strings.EqualFold(rule.TriggerType, "RecordBeforeSave") && (rule.RecordTriggerType == "" || strings.EqualFold(rule.RecordTriggerType, "Create") || strings.EqualFold(rule.RecordTriggerType, "CreateAndUpdate")) {
			hasFlow = true
			break
		}
	}
	if !hasFlow {
		return Result{}, false
	}
	names := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		name, ok := storage.ResolveFieldName(object.Definition, org.Namespace, candidate)
		if ok && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	sort.Strings(names)
	for _, name := range names {
		field := object.Definition.Fields[name]
		def := strings.TrimSpace(field.DefaultValue)
		if field.Type != storage.FieldBoolean || !strings.HasSuffix(strings.ToLower(name), "__c") || !(strings.EqualFold(def, "false") || strings.EqualFold(def, "true")) {
			continue
		}
		label := field.Label
		if label == "" {
			label = name
		}
		err := dmlErrorf("INVALID_TYPE_ON_FIELD_IN_RECORD", []string{name}, "%s: value not of required type: ", label)
		return resultFromError(record.ID, err), true
	}
	return Result{}, false
}
