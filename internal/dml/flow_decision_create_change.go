package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// A creation has no previous value. Salesforce documents Is Changed as false
// in this case. An assigned Id requires explicit actual-insert event provenance;
// other existing-Id decisions retain their prior-value path.
func flowCreateIsChangedCriterion(item storage.WorkflowCriteriaItem, source storage.Record, definition storage.ObjectDefinition, namespace string, frame *flowFrame) (matches, supported, handled bool) {
	if frame == nil || !frame.saveDecisions || source.ID != "" && !frame.createDecision || !strings.EqualFold(strings.TrimSpace(item.Operation), "isChanged") {
		return false, false, false
	}
	field := strings.TrimSpace(item.Field)
	if strings.Contains(field, ".") || item.SourceField != "" {
		return false, false, true
	}
	if _, ok := storage.ResolveFieldName(definition, namespace, field); !ok {
		return false, false, true
	}
	switch strings.ToLower(trimFormulaLiteral(item.Value)) {
	case "true":
		return false, true, true
	case "false":
		return true, true, true
	default:
		return false, false, true
	}
}

// Record successful insert provenance before immediate or deferred automation.
// Do not infer creation from an absent prior-record snapshot.
func (e *Engine) markCreatedAutomation(id storage.ID) {
	if id == "" {
		return
	}
	if e.pendingCreatedAutomation == nil {
		e.pendingCreatedAutomation = make(map[storage.ID]bool)
	}
	e.pendingCreatedAutomation[id] = true
}

func (e *Engine) beginCreatedAutomation(id storage.ID) func() {
	previous := e.activeCreatedAutomationID
	e.activeCreatedAutomationID = ""
	if e.pendingCreatedAutomation[id] {
		delete(e.pendingCreatedAutomation, id)
		if len(e.pendingCreatedAutomation) == 0 {
			e.pendingCreatedAutomation = nil
		}
		e.activeCreatedAutomationID = id
	}
	return func() { e.activeCreatedAutomationID = previous }
}
