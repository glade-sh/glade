package automation

import "strings"

// Lower a single negated scalar text comparison before building either decision
// representation. Other NOT expressions retain their unsupported diagnostics.
// The returned flag requires framed decision execution: the legacy record
// predicate evaluator cannot resolve the declared scalar used by this rewrite.
func flowNormalizeTextDecisionNegation(raw *flowXML, variables map[string]flowVariableXML) bool {
	normalized := false
	for i := range raw.Decisions {
		decision := &raw.Decisions[i]
		if len(decision.Rules) != 1 {
			continue
		}
		rule := &decision.Rules[0]
		if len(rule.Conditions) != 1 || !strings.EqualFold(strings.Join(strings.Fields(rule.ConditionLogic), " "), "NOT 1") ||
			strings.TrimSpace(rule.Connector.TargetReference) == "" || strings.TrimSpace(decision.DefaultConnector.TargetReference) == "" {
			continue
		}
		condition := &rule.Conditions[0]
		variable, declared := variables[strings.ToLower(strings.TrimSpace(condition.LeftValueReference))]
		if !declared || variable.IsCollection || !strings.EqualFold(variable.DataType, "String") ||
			!strings.EqualFold(strings.TrimSpace(condition.Operator), "Contains") ||
			condition.RightValue.ElementReference != "" || condition.RightValue.Formula != "" || condition.RightValue.Expression != "" {
			continue
		}
		rule.ConditionLogic = "and"
		// Keep Contains as the predicate and exchange its true/false paths.
		// DoesNotContain is not a valid native Flow metadata operator.
		rule.Connector, decision.DefaultConnector = decision.DefaultConnector, rule.Connector
		normalized = true
	}
	return normalized
}
