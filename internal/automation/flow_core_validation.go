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

func flowCoreValidateProjectEntries(p project.Project, idx *Index) error {
	org := storage.NewOrgState()
	if len(p.ObjectFiles) != 0 || len(p.FieldFiles) != 0 {
		if s, err := schema.LoadProject(p); err == nil {
			org.Namespace = p.Namespace
			for name, describe := range sobject.BuildDescribeRegistry(s).Objects {
				org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
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
			variables := flowVariableMap(raw.Variables)
			for _, assignment := range raw.Assignments {
				for _, item := range assignment.Items {
					if flowCoreScalarVariableReference(item.AssignToReference, variables) {
						continue
					}
					field := flowRecordFieldReference(item.AssignToReference)
					if strings.EqualFold(item.Operator, "Assign") && knownMissing(field) {
						reject(fmt.Sprintf("assignment %q references an unknown triggering-record field %q", assignment.Name, item.AssignToReference))
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
