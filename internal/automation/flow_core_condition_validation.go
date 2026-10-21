package automation

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

var flowCoreFormulaRecordFields = regexp.MustCompile(`\{!\$Record\.([[:alnum:]_]+)\}`)

func flowCoreValidateProjectConditions(p project.Project, idx *Index) error {
	if len(p.ObjectFiles) == 0 && len(p.FieldFiles) == 0 {
		return nil
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		return nil
	}
	org := storage.NewOrgState()
	org.Namespace = p.Namespace
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
		if xml.Unmarshal(data, &raw) != nil || raw.Start.DoesRequireRecordChangedToMeetCriteria || !flowActive(raw.Status) || !strings.EqualFold(raw.Start.TriggerType, "RecordAfterSave") || !strings.EqualFold(raw.Start.RecordTriggerType, "CreateAndUpdate") {
			continue
		}
		invalid := false
		reject := func(message string) {
			invalid = true
			idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: flow.File, Message: message})
		}
		knownMissing := func(objectName, fieldName string) bool {
			name, ok := storage.ResolveObjectName(org, objectName)
			if !ok || strings.Contains(fieldName, ".") {
				return false
			}
			definition := org.Objects[name].Definition
			_, ok = storage.ResolveFieldName(definition, org.Namespace, fieldName)
			return !ok && !flowRefsStandardMemberGap(definition, fieldName)
		}
		for _, lookup := range raw.RecordLookups {
			for _, filter := range lookup.Filters {
				if strings.EqualFold(filter.Operator, "EqualTo") && knownMissing(lookup.Object, filter.Field) {
					reject(fmt.Sprintf("record lookup %q filter %q references an unknown field", lookup.Name, filter.Field))
				}
			}
		}
		if flowCoreLiteralEntryFormula(raw.Start.FilterFormula) != "" {
			for _, match := range flowCoreFormulaRecordFields.FindAllStringSubmatch(raw.Start.FilterFormula, -1) {
				if knownMissing(raw.Start.Object, match[1]) {
					reject(fmt.Sprintf("start formula references an unknown triggering-record field %q", match[1]))
				}
			}
		}
		if invalid {
			flow.Rules = nil
		}
	}
	return nil
}
