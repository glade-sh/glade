package automation

import "strings"

// flowRoutedDecisionNeedsGraph reports a routed Decision that the flat branch
// projection cannot order: an element runs before the first Decision, or more
// than one Decision routes. The flat projection keeps only the chosen
// outcome's path and treats every Decision as a sibling alternative. Native
// rows (flow_assign_routes, API 62.0 and 67.0) apply an earlier Assignment
// together with the chosen outcome.
//
// Orchestration flows are unsupported and keep their earlier projection, so
// their unsupported-target diagnostics do not change.
func flowRoutedDecisionNeedsGraph(raw flowXML) bool {
	start := strings.TrimSpace(raw.Start.Connector.TargetReference)
	if start == "" || flowIsOrchestration(raw) {
		return false
	}
	routed := 0
	startsAtDecision := false
	for _, decision := range raw.Decisions {
		if len(decision.Rules) == 0 || !flowDecisionHasRoutedBranch(decision) {
			continue
		}
		routed++
		if strings.EqualFold(start, strings.TrimSpace(decision.Name)) {
			startsAtDecision = true
		}
	}
	return routed > 1 || routed == 1 && !startsAtDecision
}

func flowIsOrchestration(raw flowXML) bool {
	processType := strings.ToLower(strings.TrimSpace(raw.ProcessType))
	return len(raw.OrchestratedStages) > 0 || len(raw.Stages) > 0 || strings.Contains(processType, "orchestrat")
}
