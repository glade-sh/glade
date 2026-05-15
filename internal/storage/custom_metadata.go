package storage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/open-aer/oaer/internal/schema"
)

type UnsupportedMetadataError struct {
	File    string
	Feature string
	Message string
}

func (e UnsupportedMetadataError) Error() string {
	message := e.Message
	if message == "" {
		message = "unsupported metadata"
	}
	if e.Feature != "" {
		message = e.Feature + ": " + message
	}
	if e.File != "" {
		return fmt.Sprintf("metadata %s: %s", e.File, message)
	}
	return "metadata: " + message
}

func ApplyCustomMetadataRecords(org *OrgState, records []schema.CustomMetadataRecord) error {
	if org == nil || len(records) == 0 {
		return nil
	}
	if org.Objects == nil {
		org.Objects = make(map[string]ObjectState)
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	ordered := append([]schema.CustomMetadataRecord(nil), records...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].ObjectName != ordered[j].ObjectName {
			return ordered[i].ObjectName < ordered[j].ObjectName
		}
		if ordered[i].DeveloperName != ordered[j].DeveloperName {
			return ordered[i].DeveloperName < ordered[j].DeveloperName
		}
		return ordered[i].File < ordered[j].File
	})
	ensureCustomMetadataPrefixes(org, ordered)
	generator := NewIDGenerator(prefixesForOrg(*org))
	generator.Sequences = copySequences(org.IDSequences)
	pending := make([]customMetadataPendingRecord, 0, len(ordered))
	for _, source := range ordered {
		objectName, state, err := customMetadataState(org, source)
		if err != nil {
			return err
		}
		id, err := generator.Next(objectName)
		if err != nil {
			return UnsupportedMetadataError{File: source.File, Feature: "custom metadata record", Message: err.Error()}
		}
		fields, refs, err := customMetadataFields(*org, state.Definition, source)
		if err != nil {
			return err
		}
		pending = append(pending, customMetadataPendingRecord{
			id:         id,
			objectName: objectName,
			source:     source,
			fields:     fields,
			refs:       refs,
		})
	}
	for _, pendingRecord := range pending {
		state := org.Objects[pendingRecord.objectName]
		if state.Records == nil {
			state.Records = make(map[ID]Record)
		}
		state.Records[pendingRecord.id] = Record{
			ID:     pendingRecord.id,
			Object: pendingRecord.objectName,
			Fields: cloneValues(pendingRecord.fields),
		}
		org.Objects[pendingRecord.objectName] = state
	}
	for _, pendingRecord := range pending {
		if len(pendingRecord.refs) == 0 {
			continue
		}
		state := org.Objects[pendingRecord.objectName]
		record := state.Records[pendingRecord.id]
		for field, target := range pendingRecord.refs {
			id, ok := resolveCustomMetadataRecordID(*org, target)
			if !ok {
				return UnsupportedMetadataError{
					File:    pendingRecord.source.File,
					Feature: "custom metadata relationship",
					Message: fmt.Sprintf("cannot resolve %s value %q", field, target),
				}
			}
			record.Fields[field] = IDValue(id)
		}
		state.Records[pendingRecord.id] = record
		org.Objects[pendingRecord.objectName] = state
	}
	for object, sequence := range generator.Sequences {
		if sequence > org.IDSequences[object] {
			org.IDSequences[object] = sequence
		}
	}
	return nil
}

func ensureCustomMetadataPrefixes(org *OrgState, records []schema.CustomMetadataRecord) {
	if org == nil {
		return
	}
	names := make([]string, 0, len(org.Objects)+len(records))
	seen := make(map[string]bool, len(org.Objects)+len(records))
	for name := range org.Objects {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	for _, record := range records {
		objectName := record.ObjectName
		if objectName == "" || seen[objectName] {
			continue
		}
		names = append(names, objectName)
		seen[objectName] = true
	}
	prefixes := AssignDeterministicPrefixes(names, nil)
	for _, name := range names {
		state := org.Objects[name]
		if state.Definition.KeyPrefix == "" && prefixes[name] != "" {
			state.Definition.KeyPrefix = prefixes[name]
			org.Objects[name] = state
		}
	}
}

type customMetadataPendingRecord struct {
	id         ID
	objectName string
	source     schema.CustomMetadataRecord
	fields     map[string]Value
	refs       map[string]string
}

func customMetadataState(org *OrgState, source schema.CustomMetadataRecord) (string, ObjectState, error) {
	if source.ObjectName == "" {
		return "", ObjectState{}, UnsupportedMetadataError{File: source.File, Feature: "custom metadata record", Message: "record filename must be Type.Record.md or Type.Record.md-meta.xml"}
	}
	objectName, ok := ResolveObjectName(*org, source.ObjectName)
	if !ok {
		objectName = source.ObjectName
	}
	state := org.Objects[objectName]
	if state.Definition.APIName == "" {
		state.Definition.APIName = objectName
	}
	if state.Definition.Metadata == nil {
		state.Definition.Metadata = map[string]string{"kind": "customMetadata"}
	} else if !IsCustomMetadataDefinition(state.Definition) {
		return "", ObjectState{}, UnsupportedMetadataError{File: source.File, Feature: "custom metadata record", Message: objectName + " is not a custom metadata type"}
	}
	if state.Definition.KeyPrefix == "" {
		state.Definition.KeyPrefix = AssignDeterministicPrefixes([]string{objectName}, nil)[objectName]
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]Field)
	}
	EnsureStandardObjectFields(&state.Definition)
	if state.Records == nil {
		state.Records = make(map[ID]Record)
	}
	org.Objects[objectName] = state
	return objectName, state, nil
}

func customMetadataFields(org OrgState, definition ObjectDefinition, source schema.CustomMetadataRecord) (map[string]Value, map[string]string, error) {
	fields := map[string]Value{
		"DeveloperName":    StringValue(source.DeveloperName),
		"MasterLabel":      StringValue(labelOrDeveloperName(source.Label, source.DeveloperName)),
		"Label":            StringValue(labelOrDeveloperName(source.Label, source.DeveloperName)),
		"NamespacePrefix":  StringValue(customMetadataNamespace(org.Namespace, definition.APIName)),
		"QualifiedApiName": StringValue(customMetadataQualifiedName(org.Namespace, definition.APIName, source.DeveloperName)),
	}
	refs := make(map[string]string)
	for _, value := range source.Values {
		fieldName, ok := ResolveFieldName(definition, org.Namespace, value.Field)
		if !ok {
			return nil, nil, UnsupportedMetadataError{File: source.File, Feature: "custom metadata field", Message: fmt.Sprintf("unknown field %s.%s", definition.APIName, value.Field)}
		}
		field := definition.Fields[fieldName]
		converted, isRef, err := customMetadataValue(field, value.Value)
		if err != nil {
			return nil, nil, UnsupportedMetadataError{File: source.File, Feature: "custom metadata field", Message: fmt.Sprintf("%s.%s %v", definition.APIName, fieldName, err)}
		}
		if isRef {
			refs[fieldName] = value.Value
			continue
		}
		fields[fieldName] = converted
	}
	return fields, refs, nil
}

func customMetadataValue(field Field, raw string) (Value, bool, error) {
	raw = strings.TrimSpace(raw)
	switch field.Type {
	case FieldBoolean:
		switch strings.ToLower(raw) {
		case "true":
			return BooleanValue(true), false, nil
		case "false":
			return BooleanValue(false), false, nil
		default:
			return Value{}, false, fmt.Errorf("expects boolean value, got %q", raw)
		}
	case FieldInteger:
		value, err := parseInt64(raw)
		if err != nil {
			return Value{}, false, fmt.Errorf("expects integer value, got %q", raw)
		}
		return IntegerValue(value), false, nil
	case FieldDecimal:
		if raw == "" {
			return NullValue(), false, nil
		}
		return DecimalValue(raw), false, nil
	case FieldDate:
		return DateValue(raw), false, nil
	case FieldDateTime:
		return DateTimeValue(raw), false, nil
	case FieldReference:
		if fieldReferencesEntityDefinition(field) {
			return StringValue(raw), false, nil
		}
		return Value{}, true, nil
	case FieldID:
		return IDValue(ID(raw)), false, nil
	case FieldString, FieldPicklist, FieldAny:
		return StringValue(raw), false, nil
	default:
		return Value{}, false, fmt.Errorf("uses unsupported field type %s", field.Type)
	}
}

func fieldReferencesEntityDefinition(field Field) bool {
	for _, target := range field.ReferenceTo {
		if strings.EqualFold(target, "EntityDefinition") || strings.EqualFold(target, "FieldDefinition") {
			return true
		}
	}
	return false
}

func resolveCustomMetadataRecordID(org OrgState, raw string) (ID, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if err := ValidateID(ID(raw)); err == nil {
		return ID(raw), true
	}
	for _, state := range org.Objects {
		if !IsCustomMetadataDefinition(state.Definition) {
			continue
		}
		for _, record := range state.Records {
			for _, candidate := range customMetadataRecordNames(org.Namespace, state.Definition, record) {
				if strings.EqualFold(candidate, raw) {
					return record.ID, true
				}
			}
		}
	}
	return "", false
}

func customMetadataRecordNames(namespace string, definition ObjectDefinition, record Record) []string {
	var out []string
	developerName := firstString(record, "DeveloperName")
	qualifiedName := firstString(record, "QualifiedApiName")
	if developerName != "" {
		out = append(out, developerName)
		out = append(out, strings.TrimSuffix(definition.APIName, "__mdt")+"."+developerName)
	}
	if qualifiedName != "" {
		out = append(out, qualifiedName)
		out = append(out, strings.TrimSuffix(definition.APIName, "__mdt")+"."+qualifiedName)
	}
	if namespace != "" && developerName != "" {
		out = append(out, namespace+"__"+developerName)
	}
	return out
}

func labelOrDeveloperName(label, developerName string) string {
	if label != "" {
		return label
	}
	return developerName
}

func customMetadataNamespace(namespace, objectName string) string {
	if namespace != "" && strings.HasPrefix(objectName, namespace+"__") {
		return namespace
	}
	return ""
}

func customMetadataQualifiedName(namespace, objectName, developerName string) string {
	if customMetadataNamespace(namespace, objectName) != "" {
		return namespace + "__" + developerName
	}
	return developerName
}

func firstString(record Record, fields ...string) string {
	for _, field := range fields {
		value, ok := record.GetField(field)
		if ok && value.Kind == ValueString {
			return value.String
		}
	}
	return ""
}

func parseInt64(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}
