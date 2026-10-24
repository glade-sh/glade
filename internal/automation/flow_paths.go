package automation

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/storage"
)

type flowScheduledPathXML struct {
	Name         string           `xml:"name"`
	PathType     string           `xml:"pathType"`
	Connector    flowConnectorXML `xml:"connector"`
	OffsetNumber float64          `xml:"offsetNumber"`
	OffsetUnit   string           `xml:"offsetUnit"`
	TimeSource   string           `xml:"timeSource"`
	RecordField  string           `xml:"recordField"`
	MaxBatchSize int              `xml:"maxBatchSize"`
}

func populateFlowPaths(rule *storage.FlowRule, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML) []diagnostic.Diagnostic {
	steps, targets, diagnostics := flowPathGraph(raw.Start.Connector.TargetReference, raw, formulas, variables)
	rule.Steps, rule.StepTargets = steps, targets
	paths := append([]flowScheduledPathXML(nil), raw.Start.ScheduledPaths...)
	paths = append(paths, raw.ScheduledPaths...)
	for _, source := range paths {
		kind := "scheduled"
		if strings.EqualFold(strings.TrimSpace(source.PathType), "AsyncAfterCommit") {
			kind = "async"
		}
		steps, targets, pathDiagnostics := flowPathGraph(source.Connector.TargetReference, raw, formulas, variables)
		diagnostics = append(diagnostics, pathDiagnostics...)
		rule.Paths = append(rule.Paths, storage.FlowPath{
			Name:         strings.TrimSpace(source.Name),
			Kind:         kind,
			Steps:        steps,
			StepTargets:  targets,
			OffsetNumber: source.OffsetNumber,
			OffsetUnit:   strings.TrimSpace(source.OffsetUnit),
			TimeSource:   strings.TrimSpace(source.TimeSource),
			RecordField:  strings.TrimSpace(source.RecordField),
			MaxBatchSize: source.MaxBatchSize,
		})
	}
	for i, d := range diagnostics {
		diagnostics[i] = flowUnsupported(rule.File, rule.Name, d.Message)
	}
	return diagnostics
}

func flowPathGraph(target string, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML) ([]storage.FlowStep, map[string][]storage.FlowStep, []diagnostic.Diagnostic) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, nil, nil
	}
	// Graph selection uses reachability from Start. Compile each deferred
	// connector as its own root, including when there is no immediate entry.
	raw.Start.Connector.TargetReference = target
	raw.graph = newFlowTargetGraph(raw)
	if raw.graph == nil {
		raw.graph = newLegacyPathFlowTargetGraph(raw)
	}
	branch, diagnostics := flowPopulateBranchFromTarget(storage.FlowBranch{}, target, raw, formulas, variables)
	var targets map[string][]storage.FlowStep
	if raw.graph != nil {
		targets = raw.graph.targets
	}
	return branch.Steps, targets, diagnostics
}
