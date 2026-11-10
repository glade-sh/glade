package automation

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

// A schema overlay does not prove a field absent. Only an explicitly complete
// snapshot permits rejecting an unknown field; duplicate output targets need
// no schema. Both controls were rejected by native Flow deployment at 62/67.
func flowValidateProjectTransformTargets(p project.Project, idx *Index) {
	org := storage.NewOrgState()
	org.Namespace = p.Namespace
	complete := make(map[string]bool)
	loaded := false
	loadSchema := func() {
		if loaded {
			return
		}
		loaded = true
		if p.SchemaSnapshot != "" || len(p.ObjectFiles) > 0 || len(p.FieldFiles) > 0 {
			if s, err := schema.LoadProject(p); err == nil {
				for _, object := range s.Objects {
					if object.CompleteSnapshot {
						complete[strings.ToLower(object.Name)] = true
					}
				}
				for name, describe := range sobject.BuildDescribeRegistry(s).Objects {
					org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
				}
			}
		}
	}
	for i := range idx.Flows {
		flow := &idx.Flows[i]
		data, err := os.ReadFile(flow.File)
		if err != nil || (!bytes.Contains(data, []byte("transformValues")) && !bytes.Contains(data, []byte("recordLookups"))) {
			continue
		}
		var raw flowXML
		if xml.Unmarshal(data, &raw) != nil || !flowActive(raw.Status) || !strings.EqualFold(strings.TrimSpace(raw.ProcessType), "AutoLaunchedFlow") {
			continue
		}
		variables := make(map[string]flowVariableXML, len(raw.Variables))
		for _, variable := range raw.Variables {
			if name := strings.ToLower(strings.TrimSpace(variable.Name)); name != "" {
				variables[name] = variable
			}
		}
		rejectType := func(node, field string) {
			idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: flow.File, Message: fmt.Sprintf("flow %q has an incompatible source type for field %q", node, field)})
			flow.Rules = nil
		}
		for _, lookup := range raw.RecordLookups {
			loadSchema()
			objectName, found := storage.ResolveObjectName(org, lookup.Object)
			if !found {
				continue
			}
			definition := org.Objects[objectName].Definition
			for _, filter := range lookup.Filters {
				operator := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(filter.Operator), " ", ""))
				if operator != "in" && operator != "notin" {
					continue
				}
				field, found := storage.ResolveFieldName(definition, org.Namespace, strings.TrimSpace(filter.Field))
				variable, declared := variables[strings.ToLower(strings.TrimSpace(filter.Value.ElementReference))]
				if found && declared && variable.IsCollection && flowTypeDirectReference(filter.Value) && flowCapturedTypeMismatch(definition.Fields[field].Type, variable.DataType) {
					rejectType(lookup.Name, field)
					break
				}
			}
		}
		for _, transform := range raw.Transforms {
			if !strings.EqualFold(strings.TrimSpace(transform.DataType), "SObject") || strings.EqualFold(strings.TrimSpace(transform.IsCollection), "true") || len(transform.TransformValues) != 1 {
				continue
			}
			loadSchema()
			objectName, found := storage.ResolveObjectName(org, transform.ObjectType)
			definition := org.Objects[objectName].Definition
			canRejectMissing := found && complete[strings.ToLower(objectName)]
			seen := make(map[string]bool)
			for _, action := range transform.TransformValues[0].Actions {
				field := strings.TrimSpace(action.OutputFieldAPIName)
				if field == "" || !strings.EqualFold(strings.TrimSpace(action.TransformType), "Map") {
					continue
				}
				key := strings.ToLower(field)
				canonical, fieldKnown := storage.ResolveFieldName(definition, org.Namespace, field)
				if seen[key] || (canRejectMissing && !fieldKnown && !flowRefsStandardMemberGap(definition, field)) {
					idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: flow.File, Message: fmt.Sprintf("flow transform %q has an invalid output field %q", transform.Name, field)})
					flow.Rules = nil
					break
				}
				seen[key] = true
				if !fieldKnown {
					continue
				}
				sourceType := ""
				if variable, ok := variables[strings.ToLower(strings.TrimSpace(action.Value.ElementReference))]; ok && !variable.IsCollection && flowTypeDirectReference(action.Value) {
					sourceType = variable.DataType
				} else if action.Value.ElementReference == "" && (action.Value.StringValue != "" || action.Value.EmptyStringValue) && action.Value.Formula == "" && action.Value.Expression == "" && action.Value.BooleanValue == "" && action.Value.NumberValue == "" && action.Value.DateValue == "" && action.Value.DateTimeValue == "" {
					sourceType = "String"
				}
				if flowCapturedTypeMismatch(definition.Fields[canonical].Type, sourceType) {
					rejectType(transform.Name, field)
					break
				}
			}
		}
	}
}

// The type matrix under testdata/native-capture/flow_type_matrix/org.tsv
// records lookup and transform deployment controls at API 62 and 67. Its
// Number, Boolean, Date and DateTime targets restrict the source types.
// Unknown source or target kinds retain their existing path.
func flowCapturedTypeMismatch(target storage.FieldType, source string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case "string", "number", "boolean", "date", "datetime":
	default:
		return false
	}
	switch target {
	case storage.FieldInteger, storage.FieldDecimal:
		return source != "number"
	case storage.FieldBoolean:
		return source != "boolean"
	case storage.FieldDate, storage.FieldDateTime:
		return source != "date" && source != "datetime"
	default:
		return false
	}
}

// Captures contain a single direct reference, not a mixture of operands.
func flowTypeDirectReference(value flowValueXML) bool {
	return strings.TrimSpace(value.ElementReference) != "" && value.StringValue == "" && !value.EmptyStringValue && value.BooleanValue == "" && value.NumberValue == "" && value.DateValue == "" && value.DateTimeValue == "" && value.Formula == "" && value.Expression == ""
}
