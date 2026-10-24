package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// A literal assignment can initialize an omitted Boolean before any node reads
// it. Keep the existing boundary when entry branches, reads inputs, or cycles.
func invocableFlowBooleanAssignedBeforeUse(rule storage.FlowRule, variable storage.FlowVariable) bool {
	if variable.IsCollection || !strings.EqualFold(variable.DataType, "Boolean") {
		return false
	}
	visited := make(map[string]bool)
	var walk func([]storage.FlowStep) int
	walk = func(steps []storage.FlowStep) int {
		for _, step := range steps {
			switch step.Kind {
			case "reference":
				key := strings.ToLower(strings.TrimSpace(step.Target))
				body, found := rule.StepTargets[key]
				if !found || visited[key] {
					return -1
				}
				visited[key] = true
				if result := walk(body); result != 0 {
					return result
				}
			case "assignment":
				assignment := step.Assignment
				if !strings.EqualFold(assignment.Operator, "Assign") || assignment.SourceField != "" {
					return -1
				}
				formula := strings.TrimSpace(assignment.Formula)
				if formula != "" && formula != `""` && !strings.EqualFold(formula, "TRUE") && !strings.EqualFold(formula, "FALSE") {
					return -1
				}
				if strings.EqualFold(assignment.Target, variable.Name) {
					if strings.EqualFold(formula, "TRUE") || strings.EqualFold(formula, "FALSE") {
						return 1
					}
					return -1
				}
			default:
				return -1
			}
		}
		return 0
	}
	return walk(rule.Steps) == 1
}
