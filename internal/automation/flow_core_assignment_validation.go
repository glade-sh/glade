package automation

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
)

func flowCoreValidateProjectAssignments(_ project.Project, idx *Index) error {
	for i := range idx.Flows {
		flow := &idx.Flows[i]
		data, err := os.ReadFile(flow.File)
		if err != nil {
			continue
		}
		var raw flowXML
		if xml.Unmarshal(data, &raw) != nil || !flowActive(raw.Status) || !strings.EqualFold(raw.Start.TriggerType, "RecordAfterSave") || !strings.EqualFold(raw.Start.RecordTriggerType, "CreateAndUpdate") {
			continue
		}
		variables := flowVariableMap(raw.Variables)
		constants := flowConstantMap(raw.Constants)
		formulas := flowFormulaMapWithConstants(raw.Formulas, raw.Constants)
		invalid := false
		for _, assignment := range raw.Assignments {
			for _, item := range assignment.Items {
				target := strings.TrimSpace(item.AssignToReference)
				variable, declared := variables[strings.ToLower(target)]
				if !declared || strings.Contains(target, ".") {
					continue
				}
				operator := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(item.Operator), " ", ""))
				primitive := strings.EqualFold(variable.DataType, "String") || strings.EqualFold(variable.DataType, "Number")
				numeric := strings.EqualFold(variable.DataType, "Number") || strings.EqualFold(variable.DataType, "Currency")
				problem := ""
				switch {
				case !variable.IsCollection && strings.EqualFold(variable.DataType, "String") && operator == "subtract":
					problem = "Subtract requires a numeric scalar target"
				case variable.IsCollection && primitive && operator == "additem":
					problem = "AddItem is invalid for a primitive collection target"
				case !variable.IsCollection && strings.EqualFold(variable.DataType, "String") && !flowAssignmentOperatorSupported(item.Operator):
					problem = "assignment operator is not a valid Flow operator"
				case !variable.IsCollection && numeric && (operator == "add" || operator == "subtract"):
					reference := strings.TrimSpace(item.Value.ElementReference)
					if reference != "" && !strings.Contains(reference, ".") && !strings.HasPrefix(reference, "$") {
						key := strings.ToLower(reference)
						_, knownVariable := variables[key]
						_, knownConstant := constants[key]
						_, knownFormula := formulas[key]
						if !knownVariable && !knownConstant && !knownFormula {
							problem = "assignment operand references an unknown scalar resource"
						}
					}
				}
				if problem == "" {
					continue
				}
				invalid = true
				idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: flow.File,
					Message: fmt.Sprintf("assignment %q target %q: %s", firstNonBlank(assignment.Name, assignment.Label), target, problem)})
			}
		}
		if invalid {
			flow.Rules = nil
		}
	}
	return nil
}
