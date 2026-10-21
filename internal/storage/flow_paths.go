package storage

// FlowStepsHaveReachableEffects checks compiled branch viability without
// expanding references. An excluded source key represents an ancestor already
// visited by the legacy path compiler. Loops remain emitted effects even when
// their bodies are empty; decisions and references only route to other steps.
func FlowStepsHaveReachableEffects(steps []FlowStep, targets map[string][]FlowStep, excluded map[string]bool) bool {
	queue := [][]FlowStep{steps}
	seen := make(map[string]bool)
	for head := 0; head < len(queue); head++ {
		for _, step := range queue[head] {
			switch step.Kind {
			case "reference":
				if step.VisitKey != "" && excluded[step.VisitKey] || seen[step.Target] {
					continue
				}
				seen[step.Target] = true
				if target, ok := targets[step.Target]; ok {
					queue = append(queue, target)
				}
			case "decision":
				for _, branch := range step.Branches {
					queue = append(queue, branch.Steps)
				}
			default:
				return true
			}
		}
	}
	return false
}
