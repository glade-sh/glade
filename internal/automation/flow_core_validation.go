package automation

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

func flowCoreValidateProjectEntries(p project.Project, idx *Index, schemas *projectSchemaLoader) error {
	org := storage.NewOrgState()
	updateOrg := storage.NewOrgState()
	completeObjects := make(map[string]bool)
	if len(p.ObjectFiles) != 0 || len(p.FieldFiles) != 0 || p.SchemaSnapshot != "" {
		if s, err := schemas.load(); err == nil {
			for _, object := range s.Objects {
				if object.CompleteSnapshot {
					completeObjects[strings.ToLower(object.Name)] = true
				}
			}
			org.Namespace = p.Namespace
			updateOrg.Namespace = p.Namespace
			for name, describe := range sobject.BuildDescribeRegistry(s).Objects {
				state := storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
				updateOrg.Objects[name] = state
				if len(p.ObjectFiles) != 0 || len(p.FieldFiles) != 0 {
					org.Objects[name] = state
				}
			}
		}
	}
	for i := range idx.Flows {
		flow := &idx.Flows[i]
		data, err := os.ReadFile(flow.File)
		if err != nil {
			continue
		}
		var raw flowXML
		if xml.Unmarshal(data, &raw) != nil || raw.Start.Object == "" || !flowActive(raw.Status) {
			continue
		}
		invalid := false
		reject := func(message string) {
			invalid = true
			idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: flow.File, Message: message})
		}
		switch strings.ToLower(strings.TrimSpace(raw.Start.RecordTriggerType)) {
		case "", "create", "update", "createandupdate", "delete":
		default:
			reject(fmt.Sprintf("record trigger type %q is not supported", raw.Start.RecordTriggerType))
		}
		objectName, supplied := storage.ResolveObjectName(org, raw.Start.Object)
		if supplied && flowTriggerTypeDML(raw.Start.TriggerType) {
			definition := org.Objects[objectName].Definition
			knownMissing := func(field string) bool {
				field = trimObjectPrefix(strings.TrimSpace(field))
				if field == "" || strings.Contains(field, ".") {
					return false
				}
				_, ok := storage.ResolveFieldName(definition, org.Namespace, field)
				return !ok && !flowRefsStandardMemberGap(definition, field)
			}
			for _, filter := range raw.Start.Filters {
				if strings.EqualFold(filter.Operator, "EqualTo") && knownMissing(filter.Field) {
					reject(fmt.Sprintf("start filter %q references an unknown field", filter.Field))
				}
			}
			if strings.TrimSpace(raw.Start.FilterFormula) == "" {
				logic := strings.TrimSpace(raw.Start.FilterLogic)
				if logic != "" && !strings.EqualFold(logic, "and") {
					for _, filter := range raw.Start.Filters {
						// Reject only the enum aliases captured as invalid. Other
						// operators retain the base loader's warning/projection.
						switch strings.ToLower(strings.TrimSpace(filter.Operator)) {
						case "", "equals", "notequals", "isblank":
							reject(fmt.Sprintf("start filter operator %q is not supported for custom logic", filter.Operator))
						}
					}
				}
			}
			variables := flowVariableMap(raw.Variables)
			for _, assignment := range raw.Assignments {
				for _, item := range assignment.Items {
					if flowCoreScalarVariableReference(item.AssignToReference, variables) || invocableFlowActionBooleanAssignment(item, variables) != "" {
						continue
					}
					if variable, declared := variables[strings.ToLower(strings.TrimSpace(item.AssignToReference))]; declared && !variable.IsCollection && strings.EqualFold(variable.DataType, "SObject") {
						continue
					}
					field := flowRecordFieldReference(item.AssignToReference)
					if strings.EqualFold(item.Operator, "Assign") && knownMissing(field) {
						reject(fmt.Sprintf("assignment %q references an unknown triggering-record field %q", assignment.Name, item.AssignToReference))
					}
				}
			}
		}
		if strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordBeforeSave") {
			// Object metadata can be a partial overlay. Only a verified snapshot
			// with an explicit completeness assertion proves a field absent.
			updateObject, updateSupplied := storage.ResolveObjectName(updateOrg, raw.Start.Object)
			if updateSupplied && completeObjects[strings.ToLower(updateObject)] {
				definition := updateOrg.Objects[updateObject].Definition
				for _, update := range raw.RecordUpdates {
					if !strings.EqualFold(strings.TrimSpace(update.InputReference), "$Record") {
						continue
					}
					for _, field := range update.Fields {
						name := strings.TrimSpace(field.Field)
						_, known := storage.ResolveFieldName(definition, updateOrg.Namespace, name)
						if name != "" && !strings.Contains(name, ".") && !known && !flowRefsStandardMemberGap(definition, name) {
							reject(fmt.Sprintf("record update %q references an unknown triggering-record field %q", update.Name, name))
						}
					}
				}
			}
		}
		if invalid {
			flow.Rules = nil
		}
	}
	return nil
}
