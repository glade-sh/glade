package automation

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
)

// Delete filters with a single unknown scalar reference are invalid metadata.
// Dotted record and lookup paths, formulas, and other operators keep their
// existing validation paths.
func flowCoreValidateProjectDeleteFilters(_ project.Project, idx *Index) error {
	for i := range idx.Flows {
		flow := &idx.Flows[i]
		data, err := os.ReadFile(flow.File)
		if err != nil {
			continue
		}
		var raw flowXML
		if xml.Unmarshal(data, &raw) != nil || !flowActive(raw.Status) || !strings.EqualFold(raw.Start.TriggerType, "RecordAfterSave") || raw.Start.Object == "" {
			continue
		}
		variables := flowVariableMap(raw.Variables)
		constants := flowConstantMap(raw.Constants)
		formulas := flowFormulaMapWithConstants(raw.Formulas, raw.Constants)
		invalid := false
		for _, del := range raw.RecordDeletes {
			if del.InputReference != "" || len(del.Filters) != 3 || !strings.EqualFold(strings.TrimSpace(del.FilterLogic), "and") {
				continue
			}
			equalsOnly := true
			for _, filter := range del.Filters {
				equalsOnly = equalsOnly && strings.EqualFold(strings.TrimSpace(filter.Operator), "EqualTo")
			}
			if !equalsOnly {
				continue
			}
			for _, filter := range del.Filters {
				reference := strings.TrimSpace(filter.Value.ElementReference)
				if !strings.EqualFold(strings.TrimSpace(filter.Operator), "EqualTo") || reference == "" || strings.Contains(reference, ".") || strings.HasPrefix(reference, "$") {
					continue
				}
				key := strings.ToLower(reference)
				if _, ok := variables[key]; ok {
					continue
				}
				if _, ok := constants[key]; ok {
					continue
				}
				if _, ok := formulas[key]; ok {
					continue
				}
				known := false
				for _, lookup := range raw.RecordLookups {
					if strings.EqualFold(reference, firstNonBlank(lookup.Name, lookup.Label)) {
						known = true
						break
					}
				}
				if known {
					continue
				}
				invalid = true
				idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: flow.File,
					Message: fmt.Sprintf("record delete %q filter references unknown scalar resource %q", firstNonBlank(del.Name, del.Label), reference)})
			}
		}
		if invalid {
			flow.Rules = nil
		}
	}
	return nil
}
