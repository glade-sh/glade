package automation

import (
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

// Validation uses supplied object metadata. Schema-free Flow projections keep
// their existing diagnostics; absence of a definition is not native rejection.
func flowRefsValidateProjectDecisions(p project.Project, idx *Index) error {
	if len(p.FlowFiles) == 0 || len(p.ObjectFiles) == 0 && len(p.FieldFiles) == 0 {
		return nil
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		// Schema problems are reported by the schema load; decision checks
		// need a schema and are skipped without failing the project load.
		return nil
	}
	org := storage.NewOrgState()
	org.Namespace = p.Namespace
	storage.EnsureStandardObject(&org, "RecordType")
	for name, describe := range sobject.BuildDescribeRegistry(s).Objects {
		org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
	}
	for i := range idx.Flows {
		flow := &idx.Flows[i]
		data, err := os.ReadFile(flow.File)
		if err != nil {
			continue
		}
		var raw flowXML
		if err := xml.Unmarshal(data, &raw); err != nil {
			continue
		}
		if !flowActive(raw.Status) || raw.Start.Object == "" || !flowTriggerTypeDML(raw.Start.TriggerType) {
			continue
		}
		invalid := false
		for _, decision := range raw.Decisions {
			for _, rule := range decision.Rules {
				for _, condition := range rule.Conditions {
					priorCondition := flowPriorDecisionReference(condition.LeftValueReference)
					if priorCondition && !flowPriorDecisionConditionSupported(condition) {
						continue
					}
					field, known, valid := flowRefsDecisionField(raw, condition.LeftValueReference, org)
					if !known {
						continue
					}
					problem := ""
					triggeringField := flowRecordSourceFieldReference(condition.LeftValueReference)
					directTriggeringField := triggeringField != "" && !strings.Contains(triggeringField, ".")
					switch {
					case !valid:
						problem = "references an unknown field path"
					case priorCondition && strings.EqualFold(strings.TrimSpace(raw.Start.RecordTriggerType), "Create") && (field.Type == storage.FieldBoolean || strings.TrimSpace(condition.Operator) == "IsNull" && field.APIName == "Id"):
						problem = "requires an update-capable save flow"
					case directTriggeringField && strings.EqualFold(condition.Operator, "IsEmpty") && (field.Type == storage.FieldPicklist || strings.TrimSpace(field.Formula) != ""):
						problem = "uses IsEmpty on a scalar field"
					case directTriggeringField && strings.EqualFold(raw.Start.TriggerType, "RecordBeforeSave") && strings.TrimSpace(field.Formula) != "":
						problem = "references a calculated triggering-record field before save"
					}
					if problem == "" {
						continue
					}
					invalid = true
					idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     "GLADEAUTO004",
						File:     flow.File,
						Message:  fmt.Sprintf("decision %q condition %q %s", decision.Name, condition.LeftValueReference, problem),
					})
				}
			}
		}
		if invalid {
			// A natively invalid definition cannot supply executable effects.
			flow.Rules = nil
		}
	}
	return nil
}

func flowRefsDecisionField(raw flowXML, reference string, org storage.OrgState) (storage.Field, bool, bool) {
	objectName := raw.Start.Object
	path := flowRecordSourceFieldReference(reference)
	mode := strings.TrimSpace(raw.Start.RecordTriggerType)
	if path == "" && flowHasSaveTrigger(raw) && flowPriorDecisionReference(reference) && (strings.EqualFold(mode, "Create") || strings.EqualFold(mode, "Update") || strings.EqualFold(mode, "CreateAndUpdate")) {
		path = strings.TrimPrefix(strings.TrimSpace(reference), "$Record__Prior.")
	}
	if path == "" {
		resource, remainder, found := strings.Cut(strings.TrimSpace(reference), ".")
		if !found {
			return storage.Field{}, false, false
		}
		objectName = ""
		for _, lookup := range raw.RecordLookups {
			if lookup.StoreOutputAutomatically && strings.EqualFold(resource, firstNonBlank(lookup.Name, lookup.Label)) {
				objectName, path = lookup.Object, remainder
				break
			}
		}
	}
	name, ok := storage.ResolveObjectName(org, objectName)
	if !ok || path == "" {
		return storage.Field{}, false, false
	}
	definition := org.Objects[name].Definition
	parts := strings.Split(path, ".")
	for i, part := range parts {
		if i == len(parts)-1 {
			fieldName, ok := storage.ResolveFieldName(definition, org.Namespace, part)
			if !ok && flowRefsStandardMemberGap(definition, part) {
				return storage.Field{}, false, false
			}
			return definition.Fields[fieldName], true, ok
		}
		found := false
		for _, field := range definition.Fields {
			if len(field.ReferenceTo) != 1 || !strings.EqualFold(storage.ParentRelationshipName(field), part) {
				continue
			}
			parent, ok := storage.ResolveObjectName(org, field.ReferenceTo[0])
			if !ok {
				return storage.Field{}, false, false
			}
			definition, found = org.Objects[parent].Definition, true
			break
		}
		if !found {
			// A path cannot continue through a field the object has but that
			// is not a relationship: that is invalid. Only a name the
			// definition does not know at all may be a platform member the
			// project's partial definition lacks.
			if _, scalar := storage.ResolveFieldName(definition, org.Namespace, part); !scalar && flowRefsStandardMemberGap(definition, part) {
				return storage.Field{}, false, false
			}
			return storage.Field{}, true, false
		}
	}
	return storage.Field{}, true, false
}

// flowRefsStandardMemberGap reports a platform member that a standard
// object's project definition may not list. Only custom members are decided
// by the project's own metadata; anything else is unknown, never invalid.
func flowRefsStandardMemberGap(definition storage.ObjectDefinition, member string) bool {
	if _, known := storage.ResolveKnownStandardObjectName(definition.APIName); !known {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(member))
	return !strings.HasSuffix(lower, "__c") && !strings.HasSuffix(lower, "__r")
}
