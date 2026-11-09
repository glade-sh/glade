package dml

import (
	"math"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Before-save loop captures cover record collection variables assigned by a
// prior Flow element, and Text collections built by prior Add assignments
// whose body appends the current item to a Text variable (flow_assign_routes,
// API 62.0 and 67.0). Runtime also checks assignment actually occurred.
func flowBeforeSaveAssignedRecordLoopsSafe(rule storage.FlowRule, steps []storage.FlowStep) bool {
	assigned := map[string]bool{}
	added := map[string]bool{}
	if !flowStepsSatisfyWithLoopBodies(steps, rule.StepTargets, func(step storage.FlowStep, _ bool) bool {
		if step.Kind == "assignment" && strings.EqualFold(strings.TrimSpace(step.Assignment.Operator), "Assign") && strings.TrimSpace(step.Assignment.SourceField) != "" {
			assigned[flowFrameKey(step.Assignment.Target)] = true
		}
		if step.Kind == "assignment" && strings.EqualFold(strings.TrimSpace(step.Assignment.Operator), "Add") {
			added[flowFrameKey(step.Assignment.Target)] = true
		}
		return true
	}, true) {
		return false
	}
	declared := map[string]bool{}
	numbers := map[string]bool{}
	texts := map[string]bool{}
	textCollections := map[string]bool{}
	for _, v := range rule.Variables {
		if v.IsCollection && strings.EqualFold(v.DataType, "String") {
			textCollections[flowFrameKey(v.Name)] = true
		}
		if !v.IsCollection && strings.EqualFold(v.DataType, "String") {
			texts[flowFrameKey(v.Name)] = true
		}
		if v.IsCollection && strings.EqualFold(v.DataType, "SObject") {
			declared[flowFrameKey(v.Name)] = true
		}
		if !v.IsCollection && strings.EqualFold(v.DataType, "Number") {
			numbers[flowFrameKey(v.Name)] = true
		}
	}
	return flowStepsSatisfyWithLoopBodies(steps, rule.StepTargets, func(step storage.FlowStep, _ bool) bool {
		if step.Kind != "loop" {
			return true
		}
		key := flowFrameKey(step.Loop.CollectionReference)
		if textCollections[key] && added[key] {
			item := flowFrameKey(step.Loop.CurrentItemReference)
			return flowStepsSatisfyWithLoopBodies(step.Loop.Steps, rule.StepTargets, func(body storage.FlowStep, _ bool) bool {
				a := body.Assignment
				return body.Kind == "assignment" && texts[flowFrameKey(a.Target)] && strings.EqualFold(strings.TrimSpace(a.Operator), "Add") && strings.TrimSpace(a.Formula) == "" && flowFrameKey(a.SourceField) == item
			}, true)
		}
		if !declared[key] || !assigned[key] {
			return false
		}
		return flowStepsSatisfyWithLoopBodies(step.Loop.Steps, rule.StepTargets, func(body storage.FlowStep, _ bool) bool {
			switch body.Kind {
			case "customError":
				return true
			case "assignment":
				a := body.Assignment
				literal := strings.TrimSpace(a.LiteralValue)
				number, err := strconv.ParseFloat(literal, 64)
				return numbers[flowFrameKey(a.Target)] && strings.EqualFold(strings.TrimSpace(a.Operator), "Add") && strings.TrimSpace(a.SourceField) == "" && (strings.TrimSpace(a.Formula) == "" || strings.TrimSpace(a.Formula) == literal) && err == nil && !math.IsInf(number, 0) && !math.IsNaN(number)
			default:
				return false
			}
		}, true)
	}, true)
}
