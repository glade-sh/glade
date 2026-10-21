package dml

import "github.com/glade-sh/glade/internal/storage"

// Path references represent the legacy compiler's per-path ancestor set. Empty
// branches were omitted before evaluating their predicates; retain that behavior
// without unfolding the stored connector graph. Earlier references carry no
// VisitKey and retain their existing selection and cycle-error behavior.
func flowPathBranchViable(steps []storage.FlowStep, targets map[string][]storage.FlowStep, visited map[string]bool) bool {
	for _, step := range steps {
		if step.Kind == "reference" && step.VisitKey != "" {
			return storage.FlowStepsHaveReachableEffects(steps, targets, visited)
		}
	}
	return true
}

// Legacy loop bodies use a clone of the parent's ancestor set. Isolate each
// iteration so body visits cannot suppress later iterations or the no-more path.
func (e *Engine) applyFlowLoopBody(flowName, objectName string, record *storage.Record, definition storage.ObjectDefinition, steps []storage.FlowStep, frame *flowFrame) (bool, error) {
	parent := frame.pathVisited
	if parent != nil {
		frame.pathVisited = make(map[string]bool, len(parent))
		for key, seen := range parent {
			frame.pathVisited[key] = seen
		}
	}
	defer func() { frame.pathVisited = parent }()
	return e.applyFlowStepsWithFrame(flowName, objectName, record, definition, steps, frame)
}
