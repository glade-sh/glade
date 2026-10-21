package automation

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/storage"
)

// flowTargetGraph stores each compiled source node once. References remain edges
// in the public model, including through clone and serialization operations.
// Structured loop bodies keep their stop boundary in the target key. The
// path-scoped fallback retains source visitation when compiling other cycles.
type flowTargetGraph struct {
	targets      map[string][]storage.FlowStep
	segments     map[string]storage.FlowBranch
	pathScoped   bool
	compileDepth int
}

func (g *flowTargetGraph) populate(branch storage.FlowBranch, target string, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML) (storage.FlowBranch, []diagnostic.Diagnostic) {
	target = strings.TrimSpace(target)
	stop := strings.TrimSpace(raw.graphStop)
	if target == "" || (stop != "" && strings.EqualFold(target, stop)) {
		return branch, nil
	}
	key := strings.ToLower(target)
	if stop != "" {
		key += "/" + strings.ToLower(stop)
	}
	segment, ok := g.segments[key]
	var diagnostics []diagnostic.Diagnostic
	if !ok {
		// A pending target is already a valid back-reference. Do not recursively
		// materialize another copy when a source cycle reaches it.
		if g.pathScoped {
			g.segments[key] = storage.FlowBranch{}
			g.targets[key] = nil
		}
		g.compileDepth++
		// Diagnose a source target once, rather than once per incoming path.
		segment, diagnostics = flowPopulateBranchBody(storage.FlowBranch{}, target, stop, raw, formulas, variables, make(map[string]bool))
		g.compileDepth--
		g.segments[key] = segment
		g.targets[key] = segment.Steps
	}
	if g.pathScoped {
		reference := storage.FlowStep{Kind: "reference", Target: key, VisitKey: strings.ToLower(target)}
		// Pending and empty targets remain references inside compilation. Only
		// the completed outermost result may retain legacy empty-root fallback.
		if g.compileDepth == 0 && !storage.FlowStepsHaveReachableEffects([]storage.FlowStep{reference}, g.targets, nil) {
			return branch, diagnostics
		}
		branch.Steps = append(branch.Steps, reference)
		return branch, diagnostics
	}
	if len(segment.Steps) == 0 {
		return branch, diagnostics
	}
	branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "reference", Target: key})
	// These summaries describe local effects only; execution always uses Steps.
	branch.FieldUpdates = append(branch.FieldUpdates, segment.FieldUpdates...)
	branch.Actions = append(branch.Actions, segment.Actions...)
	branch.RecordLookups = append(branch.RecordLookups, segment.RecordLookups...)
	branch.RecordCreates = append(branch.RecordCreates, segment.RecordCreates...)
	branch.RecordDeletes = append(branch.RecordDeletes, segment.RecordDeletes...)
	return branch, diagnostics
}

type flowGraphLoopEdges struct{ body, after string }

type flowGraphSources struct {
	nodes   map[string][]string
	weights map[string]map[string]int
	loops   map[string]flowGraphLoopEdges
	start   string
}

func collectFlowGraphSources(raw flowXML) *flowGraphSources {
	if strings.TrimSpace(raw.Start.FaultConnector.TargetReference) != "" {
		return nil
	}
	nodes := make(map[string][]string)
	weights := make(map[string]map[string]int)
	loops := make(map[string]flowGraphLoopEdges)
	normalize := func(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
	addEdge := func(source, target string, weight int) {
		target = normalize(target)
		if target == "" {
			return
		}
		if weights[source] == nil {
			weights[source] = make(map[string]int)
		}
		if weights[source][target] == 0 {
			nodes[source] = append(nodes[source], target)
		}
		weights[source][target] += weight
	}
	valid := true
	add := func(name, next, fault string) {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || strings.Contains(key, "/") || strings.TrimSpace(fault) != "" {
			valid = false
			return
		}
		if _, exists := nodes[key]; exists {
			valid = false
		}
		nodes[key] = nil
		addEdge(key, next, 1)
	}
	for _, n := range raw.Assignments {
		add(firstNonBlank(n.Name, n.Label), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.RecordUpdates {
		add(firstNonBlank(n.Name, n.Label), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.RecordLookups {
		add(firstNonBlank(n.Name, n.Label), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.RecordCreates {
		add(firstNonBlank(n.Name, n.Label), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.RecordDeletes {
		add(firstNonBlank(n.Name, n.Label), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.Subflows {
		add(firstNonBlank(n.Name, n.Label), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.ApexPluginCalls {
		add(firstNonBlank(n.Name, n.Label), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.Transforms {
		add(firstNonBlank(n.Name, n.Label), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.CollectionProcessors {
		add(firstNonBlank(n.Name, n.Label), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.ActionCalls {
		add(firstNonBlank(n.Name, n.Label, n.ActionName), n.Connector.TargetReference, n.FaultConnector.TargetReference)
	}
	for _, n := range raw.Loops {
		if strings.TrimSpace(n.CollectionReference) == "" || strings.TrimSpace(n.NextValueConnector.TargetReference) == "" {
			return nil
		}
		add(n.Name, n.NoMoreValuesConnector.TargetReference, n.FaultConnector.TargetReference)
		key := normalize(n.Name)
		loops[key] = flowGraphLoopEdges{body: normalize(n.NextValueConnector.TargetReference), after: normalize(n.NoMoreValuesConnector.TargetReference)}
		addEdge(key, n.NextValueConnector.TargetReference, 1)
	}
	for _, n := range raw.CustomErrors {
		add(n.Name, "", "")
	}
	for _, n := range raw.Decisions {
		add(n.Name, n.DefaultConnector.TargetReference, "")
		key := strings.ToLower(strings.TrimSpace(n.Name))
		for _, rule := range n.Rules {
			groups, ok := flowConditionLogicGroups(rule.ConditionLogic, len(rule.Conditions))
			if !ok {
				// Compilation diagnoses and omits this branch. An explicit default
				// also fixes the fallback route if no compiled branch has effects.
				// Without it, the legacy fallback can follow the rejected connector.
				if strings.TrimSpace(n.DefaultConnector.TargetReference) == "" {
					return nil
				}
				continue
			}
			// Each DNF group was previously a separate subtree copy.
			addEdge(key, rule.Connector.TargetReference, len(groups))
		}
	}
	if !valid {
		return nil
	}
	for _, edges := range nodes {
		for _, target := range edges {
			if _, known := nodes[target]; !known {
				return nil
			}
		}
	}
	return &flowGraphSources{nodes: nodes, weights: weights, loops: loops, start: normalize(raw.Start.Connector.TargetReference)}
}

func newFlowTargetGraph(raw flowXML) *flowTargetGraph {
	sources := collectFlowGraphSources(raw)
	if sources == nil {
		return nil
	}
	nodes, weights, loops, start := sources.nodes, sources.weights, sources.loops, sources.start
	normalize := func(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
	if len(loops) != 0 {
		if _, known := nodes[start]; start == "" || !known {
			return nil
		}
	}
	// Nested bodies must be strictly contained and entered through their own
	// headers. Collect complete bodies before assigning each node to its
	// innermost loop so an inner return never becomes an outer continuation.
	bodies := make(map[string]map[string]bool)
	for owner, loop := range loops {
		body := make(map[string]bool)
		var collect func(string) bool
		collect = func(key string) bool {
			if key == "" || key == owner {
				return true
			}
			if _, known := nodes[key]; !known {
				return false
			}
			if body[key] {
				return true
			}
			body[key] = true
			for _, next := range nodes[key] {
				if !collect(next) {
					return false
				}
			}
			return true
		}
		if !collect(loop.body) || body[start] || body[loop.after] {
			return nil
		}
		bodies[owner] = body
	}
	for first, firstBody := range bodies {
		for second, secondBody := range bodies {
			if first == second {
				continue
			}
			shared := 0
			for key := range firstBody {
				if secondBody[key] {
					shared++
				}
			}
			if shared == 0 {
				continue
			}
			// Overlapping sibling bodies and equal bodies depend on legacy
			// ancestor visitation. Only a proper subset with its header inside
			// the containing body is a structured nested loop.
			if shared == len(firstBody) && len(firstBody) < len(secondBody) && secondBody[first] {
				continue
			}
			if shared == len(secondBody) && len(secondBody) < len(firstBody) && firstBody[second] {
				continue
			}
			return nil
		}
	}
	bodyOwner := make(map[string]string)
	for owner, body := range bodies {
		for key := range body {
			previous := bodyOwner[key]
			if previous == "" || len(body) < len(bodies[previous]) {
				bodyOwner[key] = owner
			}
		}
	}
	for source, edges := range nodes {
		for _, target := range edges {
			sourceOwner := bodyOwner[source]
			if sourceOwner != "" && target == sourceOwner {
				continue
			}
			targetOwner := bodyOwner[target]
			if sourceOwner == targetOwner {
				continue
			}
			if targetOwner != "" && source == targetOwner && target == loops[targetOwner].body {
				continue
			}
			return nil
		}
	}
	// Removing only returns to a body's own loop must leave a DAG. This
	// excludes graphs whose meaning depends on a legacy ancestor's visited set.
	state := make(map[string]uint8)
	var acyclic func(string) bool
	acyclic = func(key string) bool {
		if state[key] != 0 {
			return state[key] == 2
		}
		edges, exists := nodes[key]
		if !exists {
			return false
		}
		state[key] = 1
		for _, target := range edges {
			if bodyOwner[key] == target {
				continue
			}
			if !acyclic(target) {
				return false
			}
		}
		state[key] = 2
		return true
	}
	for key := range nodes {
		if !acyclic(key) {
			return nil
		}
	}
	type context struct{ target, stop string }
	children := func(c context) map[context]int {
		out := make(map[context]int)
		if loop, ok := loops[c.target]; ok {
			if loop.body != "" && loop.body != c.target {
				out[context{loop.body, c.target}]++
			}
			if loop.after != "" && loop.after != c.stop {
				out[context{loop.after, c.stop}]++
			}
			return out
		}
		for _, target := range nodes[c.target] {
			if target != c.stop {
				out[context{target, c.stop}] += weights[c.target][target]
			}
		}
		return out
	}
	decisions := make(map[string]bool)
	for _, n := range raw.Decisions {
		decisions[normalize(n.Name)] = true
	}
	incoming := make(map[context]int)
	containsDecision := make(map[context]bool)
	checked := make(map[context]bool)
	var decisionSuffix func(context) bool
	decisionSuffix = func(c context) bool {
		if checked[c] {
			return containsDecision[c]
		}
		checked[c] = true
		containsDecision[c] = decisions[c.target]
		for next, weight := range children(c) {
			incoming[next] += weight
			if decisionSuffix(next) {
				containsDecision[c] = true
			}
		}
		return containsDecision[c]
	}
	if len(loops) != 0 {
		decisionSuffix(context{target: start})
	} else {
		for key := range nodes {
			decisionSuffix(context{target: key})
		}
	}
	// Shared decision suffixes multiply the old materialized path count.
	// Loop returns do not count as sharing: they terminate their body context.
	sharedDecisionSuffix := false
	for c, count := range incoming {
		if count > 1 && containsDecision[c] {
			sharedDecisionSuffix = true
			break
		}
	}
	if !sharedDecisionSuffix {
		return nil
	}
	return &flowTargetGraph{targets: make(map[string][]storage.FlowStep), segments: make(map[string]storage.FlowBranch)}
}

// newLegacyPathFlowTargetGraph bounds source paths that cannot use the strict
// structured graph. References carry raw source visitation keys so execution
// preserves the existing compiler's ancestor cutoffs and empty-branch pruning.
func newLegacyPathFlowTargetGraph(raw flowXML) *flowTargetGraph {
	sources := collectFlowGraphSources(raw)
	if sources == nil || sources.start == "" {
		return nil
	}
	if _, known := sources.nodes[sources.start]; !known {
		return nil
	}
	reachable := map[string]bool{sources.start: true}
	queue := []string{sources.start}
	for head := 0; head < len(queue); head++ {
		for _, target := range sources.nodes[queue[head]] {
			if !reachable[target] {
				reachable[target] = true
				queue = append(queue, target)
			}
		}
	}
	// Decision reachability is a reverse worklist, rather than recursive suffix
	// expansion, because this fallback intentionally permits source cycles.
	reverse := make(map[string][]string)
	incoming := make(map[string]int)
	for source := range reachable {
		for _, target := range sources.nodes[source] {
			reverse[target] = append(reverse[target], source)
			if _, loopHeader := sources.loops[target]; !loopHeader {
				incoming[target] += sources.weights[source][target]
			}
		}
	}
	containsDecision := make(map[string]bool)
	queue = nil
	for _, decision := range raw.Decisions {
		key := strings.ToLower(strings.TrimSpace(decision.Name))
		if reachable[key] {
			containsDecision[key] = true
			queue = append(queue, key)
		}
	}
	for head := 0; head < len(queue); head++ {
		for _, predecessor := range reverse[queue[head]] {
			if !containsDecision[predecessor] {
				containsDecision[predecessor] = true
				queue = append(queue, predecessor)
			}
		}
	}
	for target, count := range incoming {
		if count > 1 && containsDecision[target] {
			return &flowTargetGraph{targets: make(map[string][]storage.FlowStep), segments: make(map[string]storage.FlowBranch), pathScoped: true}
		}
	}
	return nil
}
