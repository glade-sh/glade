package perfscan

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

var (
	invocableRe    = regexp.MustCompile(`(?i)@InvocableMethod\b`)
	batchableRe    = regexp.MustCompile(`(?i)\bimplements\b[^{};]*Database\.Batchable\b`)
	newClassTypeRe = regexp.MustCompile(`(?i)new\s+([A-Za-z_][A-Za-z0-9_\.]*)`)
)

const (
	selectivityRiskThreshold = 3
	bigSelectFieldCount      = 20
	maxAsyncChainDepth       = 3
)

var (
	loopNodeKinds = map[string]struct{}{
		"enhanced_for_statement": {},
		"for_statement":          {},
		"while_statement":        {},
		"do_statement":           {},
	}

	platformDmlMethods = map[string]struct{}{
		"insert":   {},
		"update":   {},
		"delete":   {},
		"upsert":   {},
		"undelete": {},
		"merge":    {},
	}

	platformAsyncMethods = map[string]struct{}{
		"enqueuejob":   {},
		"executebatch": {},
		"schedule":     {},
	}

	platformDescribeMethods = map[string]struct{}{
		"getglobaldescribe": {},
		"describesobjects":  {},
	}
)

func scanApex(report *Report, p project.Project, parsed apexast.Result, index typesys.Index) {
	asyncMethodsByFile := collectAsyncMethodsByFile(parsed, index)
	testMethodsByFile := collectTestMethodsByFile(parsed)
	asyncGraph := newAsyncCallGraph(index)

	for _, file := range parsed.Files {
		for _, decl := range file.Declarations {
			if decl.Kind == apexast.DeclarationTrigger {
				report.AddEntryPoint(EntryPoint{Kind: EntryTrigger, Name: decl.Name, File: file.Path, Line: decl.Range.Start.Line})
			}
		}
	}
	for _, path := range p.ApexFiles {
		if isTestSourcePath(path) || isVendoredSourcePath(path) {
			continue
		}
		sourceBytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		scanApexSource(report, path, string(sourceBytes), index, asyncMethodsByFile[path], testMethodsByFile[path], asyncGraph)
	}
	asyncGraph.emitFindings(report)
}

func scanApexSource(report *Report, path, source string, index typesys.Index, asyncMethods map[string]asyncMethodMetadata, testMethods map[string]struct{}, asyncGraph *asyncCallGraph) {
	if asyncMethods == nil {
		asyncMethods = make(map[string]asyncMethodMetadata)
	}
	if testMethods == nil {
		testMethods = make(map[string]struct{})
	}
	parser := apexast.NewParser()
	astFile := parser.ParseSourceAST(path, source)

	state := scanApexFileState{
		path:         path,
		source:       source,
		report:       report,
		resolver:     newPlatformCallResolver(index),
		asyncMethods: asyncMethods,
		testMethods:  testMethods,
		asyncGraph:   asyncGraph,
	}
	state.pushScope()
	for _, node := range astFile.Nodes {
		state.scanNode(node)
	}
	state.popScope()

	state.emitLoopFindings()

	if astFile.Diagnostics != nil {
		// Parse diagnostics are surfaced through declaration parsing and project analysis.
	}

	for _, match := range invocableRe.FindAllStringIndex(source, -1) {
		line := lineAt(source, match[0])
		report.AddEntryPoint(EntryPoint{Kind: EntryInvocable, Name: "InvocableMethod", File: path, Line: line})
	}

	if batchableRe.MatchString(source) {
		className := classNameFromSource(path, source)
		report.AddEntryPoint(EntryPoint{Kind: EntryBatch, Name: className, File: path, Line: 1})
	}

}

func (s *scanApexFileState) scanNode(node apexast.ASTNode) {
	if node.Kind == "if_statement" && s.isRunningTestCondition(node) {
		s.scanRunningTestIf(node)
		return
	}

	s.recordFormalParameter(node)
	s.recordSObjectTypeVariables(node)

	if _, ok := loopNodeKinds[node.Kind]; ok {
		s.enterLoop(node)
		return
	}
	if node.Kind == "method_declaration" {
		s.enterMethod(node)
		return
	}

	switch node.Kind {
	case "query_expression":
		s.recordSoqlQuery(node)
		s.markLoop(findingsCategorySOQL)
	case "dml_expression":
		s.markLoop(findingsCategoryDML)
		s.markFutureDml()
	case "method_invocation":
		s.processMethodInvocation(node)
	}

	if node.Kind == "block" {
		s.pushScope()
		for _, child := range node.Children {
			s.scanNode(child)
		}
		s.popScope()
		return
	}

	for _, child := range node.Children {
		s.scanNode(child)
	}
}

func (s *scanApexFileState) enterLoop(node apexast.ASTNode) {
	body := loopBody(node)
	loop := &loopFindings{line: node.Range.Start.Line, depth: len(s.loopStack) + 1}
	s.loopStack = append(s.loopStack, loop)
	s.loops = append(s.loops, loop)
	if body != nil {
		s.scanNode(*body)
	}
	s.loopStack = s.loopStack[:len(s.loopStack)-1]
}

func (s *scanApexFileState) markLoop(kind findingsCategory) {
	for _, loop := range s.loopStack {
		switch kind {
		case findingsCategorySOQL:
			loop.hasSoql = true
		case findingsCategoryDML:
			loop.hasDml = true
		case findingsCategoryDescribe:
			loop.hasDescribe = true
		case findingsCategoryAsync:
			loop.hasAsync = true
		}
	}
}

func (s *scanApexFileState) processMethodInvocation(node apexast.ASTNode) {
	methodName, receiver := methodInvocationParts(node, s.source)
	if methodName == "" || receiver == nil {
		return
	}

	segments := identifierSegments(*receiver, s.source)
	if len(segments) == 0 {
		return
	}

	method := strings.ToLower(strings.TrimSpace(methodName))
	line := node.Range.Start.Line
	if _, ok := platformDmlMethods[method]; ok {
		if s.isPlatformStaticMethod(segments, methodName) {
			s.markLoop(findingsCategoryDML)
			s.markFutureDml()
		}
		return
	}

	if _, ok := platformAsyncMethods[method]; ok {
		if s.isPlatformStaticMethod(segments, methodName) {
			s.markLoop(findingsCategoryAsync)
			s.recordAsyncInvocation(method, node, line)
		}
		return
	}

	if _, ok := platformDescribeMethods[method]; ok {
		if s.isPlatformStaticMethod(segments, methodName) {
			s.markLoop(findingsCategoryDescribe)
			s.recordDescribeLine(line)
		}
		return
	}

	if method == "getdescribe" {
		if s.isSObjectTypeDescribeReceiver(segments) {
			s.markLoop(findingsCategoryDescribe)
			s.recordDescribeLine(line)
		}
	}
}

func (s *scanApexFileState) recordAsyncInvocation(method string, node apexast.ASTNode, line int) {
	methodLower := strings.ToLower(method)
	switch methodLower {
	case "enqueuejob":
		target := s.classFromInvocationArg(node, 0)
		if target == "" {
			return
		}
		currentMethod := s.currentMethod()
		if currentMethod == nil {
			return
		}
		if currentMethod.hasMeta && currentMethod.meta.isQueueableExecute {
			s.asyncGraph.addQueueableEdge(currentMethod.meta.className, target, line, s.path)
		}
		if currentMethod.meta.isBatchExecute && s.asyncGraph.addBatchQueueableEdge(currentMethod.meta.className, target, line, s.path) {
			s.report.AddFinding(staticFinding(
				"perf.async.batch.execute-queueable",
				CategoryAsync,
				SeverityMedium,
				77,
				s.path,
				line,
				"Batch execute enqueues Queueable work, which can stack async work inside chunked transactions.",
				"Return a bounded payload from batch execute into one queue job per chunk or use an explicit work queue entry.",
			))
		}
	case "executebatch":
		target := s.classFromInvocationArg(node, 0)
		if target == "" {
			return
		}
		currentMethod := s.currentMethod()
		if currentMethod == nil {
			return
		}
		switch {
		case currentMethod.meta.isBatchExecute:
			s.report.AddFinding(staticFinding(
				"perf.async.batch.execute",
				CategoryAsync,
				SeverityMedium,
				79,
				s.path,
				line,
				"Batch execute calls executeBatch, which can spawn additional full-scan jobs from each execute scope.",
				"Ensure batching boundaries and terminal conditions exist so chained jobs cannot run indefinitely.",
			))
			s.asyncGraph.addBatchEdge(currentMethod.meta.className, target, line, s.path)
		case currentMethod.meta.isBatchFinish:
			s.report.AddFinding(staticFinding(
				"perf.async.batch.finish",
				CategoryAsync,
				SeverityMedium,
				80,
				s.path,
				line,
				"Batch finish calls executeBatch, which can continue chain creation after each scope completes.",
				"Evaluate finish-time batching and cap chaining through explicit run-state checks.",
			))
			s.asyncGraph.addBatchEdge(currentMethod.meta.className, target, line, s.path)
		}
	case "schedule":
		target := s.classFromInvocationArg(node, 2)
		if target == "" {
			return
		}
		currentMethod := s.currentMethod()
		if currentMethod == nil || !currentMethod.meta.isSchedulableExecute {
			return
		}
		s.asyncGraph.addScheduleEdge(currentMethod.meta.className, target, line, s.path)
	}
}

func (s *scanApexFileState) classFromInvocationArg(node apexast.ASTNode, argIndex int) string {
	arg := invocationArgAt(node, argIndex)
	if arg == nil {
		return ""
	}
	return classNameFromExpression(*arg, s.source)
}

func (s *scanApexFileState) markFutureDml() {
	current := s.currentMethod()
	if current == nil {
		return
	}
	current.sawFutureDml = true
}

func (s *scanApexFileState) enterMethod(node apexast.ASTNode) {
	if _, ok := s.testMethods[methodRangeKey(node.Range)]; ok {
		return
	}

	frame := asyncMethodFrame{}
	if meta, ok := s.asyncMethods[methodRangeKey(node.Range)]; ok {
		frame = asyncMethodFrame{meta: meta, hasMeta: true}
		if meta.isFuture {
			s.report.AddFinding(staticFinding(
				"perf.async.future",
				CategoryAsync,
				SeverityMedium,
				74,
				s.path,
				meta.line,
				"@future methods defer execution and consume async governor budgets.",
				"Use @future only for intentionally asynchronous work, or prefer Queueable/Batch for longer logic.",
			))
		}
	}
	s.methodStack = append(s.methodStack, frame)

	for _, child := range node.Children {
		s.scanNode(child)
	}

	top := &s.methodStack[len(s.methodStack)-1]
	if len(top.describeLines) > 1 && !isTestSourcePath(s.path) {
		s.report.AddFinding(staticFinding(
			"perf.describe.repeated",
			CategoryDescribe,
			SeverityMedium,
			55,
			s.path,
			top.describeLines[0],
			"Repeated describe calls in the same method can waste CPU and heap.",
			"Store describe results in a local variable or immutable per-transaction cache.",
		))
	}
	if top.hasMeta && top.meta.isFutureCallout && top.sawFutureDml {
		s.report.AddFinding(Finding{
			ID:         "perf.async.future.callout-dml",
			Category:   CategoryAsync,
			Severity:   SeverityHigh,
			Confidence: ConfidenceStatic,
			Score:      92,
			EntryPoint: EntryPoint{Kind: EntryUnknown},
			Message:    "@future(callout=true) mixed with DML in one method will throw runtime mixed-DML restrictions.",
			Location:   Location{File: s.path, Line: top.meta.line},
			Fix:        "Split callout and DML into separate async paths or use Continuation/queueable orchestration.",
		})
	}
	s.methodStack = s.methodStack[:len(s.methodStack)-1]
}

func (s *scanApexFileState) currentMethod() *asyncMethodFrame {
	if len(s.methodStack) == 0 {
		return nil
	}
	return &s.methodStack[len(s.methodStack)-1]
}

func (s *scanApexFileState) emitLoopFindings() {
	for _, loop := range s.loops {
		if loop.hasSoql {
			s.report.AddFinding(staticFinding(
				"perf.soql.loop",
				CategorySOQL,
				SeverityHigh,
				95,
				s.path,
				loop.line,
				"SOQL inside a loop can exceed query limits and repeats database work per record.",
				"Move the query outside the loop, query all needed rows once, and use a map keyed by Id or business key.",
			))
		}
		if loop.hasDml {
			s.report.AddFinding(staticFinding(
				"perf.dml.loop",
				CategoryDML,
				SeverityHigh,
				92,
				s.path,
				loop.line,
				"DML inside a loop can exceed statement limits and repeats save-order automation per record.",
				"Build a collection inside the loop and run one DML operation after the loop.",
			))
		}
		if loop.hasAsync {
			s.report.AddFinding(staticFinding(
				"perf.async.loop",
				CategoryAsync,
				SeverityHigh,
				88,
				s.path,
				loop.line,
				"Async enqueue inside a loop can exceed queueable, future, scheduled, or batch enqueue limits.",
				"Enqueue one job with the full work set or batch the records through a bounded async entry point.",
			))
		}
	}
}

func (s *scanApexFileState) recordSoqlQuery(node apexast.ASTNode) {
	queryText := strings.TrimSpace(nodeText(s.source, node))
	if queryText == "" {
		return
	}
	if strings.HasPrefix(queryText, "[") && strings.HasSuffix(queryText, "]") {
		queryText = strings.TrimSpace(queryText[1 : len(queryText)-1])
	}
	parsed, err := soql.Parse(queryText)
	if err != nil {
		return
	}
	line := node.Range.Start.Line
	s.recordSoqlSelectivity(parsed, line, queryText)
	s.recordSoqlProjection(parsed, line, queryText)
	s.recordSoqlChildQuery(parsed, line, queryText)
	s.recordSoqlOrderBy(parsed, line, queryText)
}

func (s *scanApexFileState) recordSoqlSelectivity(query soql.Query, line int, queryText string) {
	if query.Where == nil {
		return
	}
	score, reasons := soqlSelectivityScore(*query.Where)
	if score < selectivityRiskThreshold {
		return
	}
	evidence := []Evidence{
		{Kind: "soql", Message: "soql selectivity score", Value: strconv.Itoa(score)},
	}
	for _, reason := range reasons {
		evidence = append(evidence, Evidence{Kind: "soql", Message: reason})
	}
	s.report.AddFinding(Finding{
		ID:         "perf.soql.selectivity",
		Category:   CategorySOQL,
		Severity:   SeverityMedium,
		Confidence: ConfidenceStatic,
		Score:      68,
		EntryPoint: EntryPoint{Kind: EntryUnknown},
		Message:    "SOQL WHERE clauses may be non-selective for large subscriber-org tables.",
		Location:   Location{File: s.path, Line: line},
		Evidence:   evidence,
		Fix:        "Check the query plan with production-scale row counts, then refine filters to indexed fields or narrower predicates where needed.",
	})
	for _, reason := range reasons {
		if strings.HasPrefix(reason, "Formula field in WHERE") {
			s.recordSoqlFormulaWhere(reason, line, queryText)
			break
		}
	}
}

func (s *scanApexFileState) recordSoqlProjection(query soql.Query, line int, queryText string) {
	var reasons []string
	if len(query.Fields) >= bigSelectFieldCount {
		reasons = append(reasons, "query projects "+strconv.Itoa(len(query.Fields))+" fields")
	}
	for _, field := range query.Fields {
		upper := strings.ToUpper(strings.TrimSpace(field))
		if strings.HasPrefix(upper, "FIELDS(") && strings.Contains(upper, "ALL") {
			reasons = append(reasons, "query uses FIELDS(ALL)")
			break
		}
	}
	if len(reasons) == 0 {
		return
	}
	evidence := []Evidence{
		{Kind: "soql", Message: "query", Value: queryText},
	}
	for _, reason := range reasons {
		evidence = append(evidence, Evidence{Kind: "soql", Message: reason})
	}
	s.report.AddFinding(Finding{
		ID:         "perf.soql.overfetch",
		Category:   CategorySOQL,
		Severity:   SeverityMedium,
		Confidence: ConfidenceStatic,
		Score:      76,
		EntryPoint: EntryPoint{Kind: EntryUnknown},
		Message:    "SOQL returns more columns than needed and may increase row transfer and CPU.",
		Location:   Location{File: s.path, Line: line},
		Evidence:   evidence,
		Fix:        "Project only fields required by the business path. Prefer explicit minimal field lists over broad selectors.",
	})
}

func (s *scanApexFileState) recordSoqlChildQuery(query soql.Query, line int, queryText string) {
	relations := make([]string, 0, len(query.ChildQueries))
	for _, child := range query.ChildQueries {
		if child.Query.HasLimit {
			continue
		}
		relations = append(relations, child.Relationship)
	}
	if len(relations) == 0 {
		return
	}
	evidence := []Evidence{
		{Kind: "soql", Message: "query", Value: queryText},
	}
	for _, relation := range relations {
		evidence = append(evidence, Evidence{Kind: "soql", Message: "child relationship without LIMIT", Value: relation})
	}
	s.report.AddFinding(Finding{
		ID:         "perf.soql.subquery-no-limit",
		Category:   CategorySOQL,
		Severity:   SeverityMedium,
		Confidence: ConfidenceStatic,
		Score:      74,
		EntryPoint: EntryPoint{Kind: EntryUnknown},
		Message:    "Parent-child subqueries without LIMIT can amplify row processing in nested loops.",
		Location:   Location{File: s.path, Line: line},
		Evidence:   evidence,
		Fix:        "Cap subquery output with LIMIT and move filters to the child query root where possible.",
	})
}

func (s *scanApexFileState) recordSoqlOrderBy(query soql.Query, line int, queryText string) {
	var fields []string
	for _, order := range query.Order {
		if !isIndexedField(order.Field) {
			fields = append(fields, order.Field)
		}
	}
	if len(fields) == 0 {
		return
	}
	evidence := []Evidence{
		{Kind: "soql", Message: "query", Value: queryText},
	}
	for _, field := range fields {
		evidence = append(evidence, Evidence{Kind: "soql", Message: "ORDER BY field", Value: field})
	}
	s.report.AddFinding(Finding{
		ID:         "perf.soql.orderby-no-index",
		Category:   CategorySOQL,
		Severity:   SeverityMedium,
		Confidence: ConfidenceStatic,
		Score:      72,
		EntryPoint: EntryPoint{Kind: EntryUnknown},
		Message:    "ORDER BY on non-indexed fields may require a full sort and consume additional query resources.",
		Location:   Location{File: s.path, Line: line},
		Evidence:   evidence,
		Fix:        "Where possible, ORDER BY indexed fields or perform sorting in post-fetch application logic on small result sets.",
	})
}

func (s *scanApexFileState) recordSoqlFormulaWhere(reason string, line int, queryText string) {
	s.report.AddFinding(Finding{
		ID:         "perf.soql.where-formula",
		Category:   CategorySOQL,
		Severity:   SeverityMedium,
		Confidence: ConfidenceStatic,
		Score:      70,
		EntryPoint: EntryPoint{Kind: EntryUnknown},
		Message:    "Formula expressions in WHERE break index usage and can force full scans.",
		Location:   Location{File: s.path, Line: line},
		Evidence: []Evidence{
			{Kind: "soql", Message: queryText},
			{Kind: "soql", Message: reason},
		},
		Fix: "Move formula-derived comparisons to indexed fields or precompute values into helper columns.",
	})
}

func (s *scanApexFileState) isSObjectTypeDescribeReceiver(segments []string) bool {
	if len(segments) == 0 {
		return false
	}
	for _, seg := range segments {
		if strings.EqualFold(seg, "SObjectType") {
			return true
		}
	}
	if len(segments) == 1 {
		return s.isSObjectTypeVar(segments[0])
	}
	return false
}

func (s *scanApexFileState) isPlatformStaticMethod(segments []string, method string) bool {
	if len(segments) == 0 {
		return false
	}
	for end := len(segments); end > 0; end-- {
		typeName := strings.Join(segments[:end], ".")
		if s.resolver.isShadowedType(typeName) {
			return false
		}
		if s.resolver.hasStaticMethod(typeName, method) {
			return true
		}
	}
	return false
}

func (s *scanApexFileState) isRunningTestCondition(node apexast.ASTNode) bool {
	for _, child := range node.Children {
		if child.Kind == "block" || child.Kind == "else_clause" {
			continue
		}
		if containsRunningTestCondition(child, s.source) {
			return true
		}
	}
	return false
}

func (s *scanApexFileState) scanRunningTestIf(node apexast.ASTNode) {
	seenThen := false
	for _, child := range node.Children {
		if child.Kind == "block" {
			if !seenThen {
				seenThen = true
				continue
			}
			s.scanNode(child)
			continue
		}
		if child.Kind == "else_clause" {
			for _, elseNode := range child.Children {
				s.scanNode(elseNode)
			}
		}
	}
}

func (s *scanApexFileState) recordSObjectTypeVariables(node apexast.ASTNode) {
	typeName := ""
	for _, child := range node.Children {
		if isTypeNode(child.Kind) && typeName == "" {
			typeName = strings.TrimSpace(nodeText(s.source, child))
		}
	}
	if !isSObjectTypeType(typeName) {
		return
	}
	for _, child := range node.Children {
		if child.Kind == "variable_declarator" {
			name := variableIdentifier(child, s.source)
			if name != "" {
				s.declareSObjectTypeVar(name)
			}
		}
	}
}

func (s *scanApexFileState) recordFormalParameter(node apexast.ASTNode) {
	if node.Kind != "formal_parameter" {
		return
	}
	typeName := ""
	name := ""
	for _, child := range node.Children {
		if typeName == "" && isTypeNode(child.Kind) {
			typeName = strings.TrimSpace(nodeText(s.source, child))
		}
		if child.Kind == "identifier" {
			name = strings.TrimSpace(nodeText(s.source, child))
		}
	}
	if isSObjectTypeType(typeName) && name != "" {
		s.declareSObjectTypeVar(name)
	}
}

func (s *scanApexFileState) recordDescribeLine(line int) {
	if current := s.currentMethod(); current != nil {
		for _, existing := range current.describeLines {
			if existing == line {
				return
			}
		}
		current.describeLines = append(current.describeLines, line)
		return
	}
	for _, existing := range s.describeLines {
		if existing == line {
			return
		}
	}
	s.describeLines = append(s.describeLines, line)
}

func (s *scanApexFileState) isSObjectTypeVar(name string) bool {
	key := strings.ToLower(name)
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if s.scopes[i][key] {
			return true
		}
	}
	return false
}

func (s *scanApexFileState) declareSObjectTypeVar(name string) {
	if len(s.scopes) == 0 {
		return
	}
	top := len(s.scopes) - 1
	if s.scopes[top] == nil {
		s.scopes[top] = make(map[string]bool)
	}
	s.scopes[top][strings.ToLower(name)] = true
}

func (s *scanApexFileState) pushScope() {
	s.scopes = append(s.scopes, make(map[string]bool))
}

func (s *scanApexFileState) popScope() {
	if len(s.scopes) == 0 {
		return
	}
	s.scopes = s.scopes[:len(s.scopes)-1]
}

func loopBody(node apexast.ASTNode) *apexast.ASTNode {
	for _, child := range node.Children {
		if child.Kind == "block" {
			return &child
		}
	}
	return nil
}

func identifierSegments(node apexast.ASTNode, source string) []string {
	var out []string
	if node.Kind == "identifier" {
		value := strings.TrimSpace(nodeText(source, node))
		if value != "" {
			out = append(out, value)
		}
		return out
	}
	for _, child := range node.Children {
		switch child.Kind {
		case "identifier", "field_access", "scoped_type_identifier", "binary_expression", "array_access", "member_access", "type_identifier", "scoped_identifier":
			out = append(out, identifierSegments(child, source)...)
		case "method_invocation":
			continue
		default:
			out = append(out, identifierSegments(child, source)...)
		}
	}
	return out
}

func methodInvocationParts(node apexast.ASTNode, source string) (method string, receiver *apexast.ASTNode) {
	if node.Kind != "method_invocation" {
		return "", nil
	}

	methodIndex := -1
	for i := len(node.Children) - 1; i >= 0; i-- {
		if node.Children[i].Kind == "argument_list" {
			methodIndex = i - 1
			break
		}
	}
	if methodIndex < 0 {
		methodIndex = len(node.Children) - 2
	}
	if methodIndex < 0 || methodIndex >= len(node.Children) {
		return "", nil
	}

	methodNode := node.Children[methodIndex]
	if methodNode.Kind != "identifier" {
		return "", nil
	}
	method = strings.TrimSpace(nodeText(source, methodNode))
	if method == "" {
		return "", nil
	}
	if methodIndex == 0 {
		return method, nil
	}
	return method, &node.Children[methodIndex-1]
}

func methodRangeKey(r apexast.Range) string {
	return strconv.Itoa(r.Start.Offset) + ":" + strconv.Itoa(r.End.Offset)
}

func invocationArgAt(node apexast.ASTNode, index int) *apexast.ASTNode {
	for _, child := range node.Children {
		if child.Kind != "argument_list" {
			continue
		}
		if index < 0 || index >= len(child.Children) {
			return nil
		}
		return &child.Children[index]
	}
	return nil
}

func classNameFromExpression(node apexast.ASTNode, source string) string {
	text := strings.TrimSpace(nodeText(source, node))
	match := newClassTypeRe.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return normalizeAsyncTypeName(match[1])
}

func containsRunningTestCondition(node apexast.ASTNode, source string) bool {
	if node.Kind == "method_invocation" {
		method, receiver := methodInvocationParts(node, source)
		if !strings.EqualFold(method, "isRunningTest") || receiver == nil {
			return false
		}
		receiverText := strings.TrimSpace(nodeText(source, *receiver))
		if isTestRunningReceiverText(receiverText) {
			return true
		}
		segments := identifierSegments(*receiver, source)
		return isTestRunningReceiver(segments)
	}
	for _, child := range node.Children {
		if containsRunningTestCondition(child, source) {
			return true
		}
	}
	return false
}

func isTestRunningReceiverText(value string) bool {
	value = strings.ReplaceAll(value, " ", "")
	return strings.EqualFold(value, "Test") || strings.EqualFold(value, "System.Test")
}

func isTestRunningReceiver(segments []string) bool {
	if len(segments) == 0 {
		return false
	}
	if strings.EqualFold(segments[0], "Test") {
		return true
	}
	return len(segments) >= 2 && strings.EqualFold(segments[0], "System") && strings.EqualFold(segments[1], "Test")
}

func staticFinding(id string, category Category, severity Severity, score int, file string, line int, message, fix string) Finding {
	return Finding{
		ID:         id,
		Category:   category,
		Severity:   severity,
		Confidence: ConfidenceStatic,
		Score:      score,
		Message:    message,
		Location:   Location{File: file, Line: line},
		Evidence:   []Evidence{{Kind: "static", Message: message}},
		Fix:        fix,
	}
}

func lineAt(source string, offset int) int {
	line := 1
	for i := 0; i < offset && i < len(source); i++ {
		if source[i] == '\n' {
			line++
		}
	}
	return line
}

func nodeText(source string, node apexast.ASTNode) string {
	start := node.Range.Start.Offset
	end := node.Range.End.Offset
	if start < 0 || end < start || end > len(source) {
		return ""
	}
	return source[start:end]
}

func variableIdentifier(node apexast.ASTNode, source string) string {
	for _, child := range node.Children {
		if child.Kind == "identifier" {
			return strings.TrimSpace(nodeText(source, child))
		}
	}
	return strings.TrimSpace(nodeText(source, node))
}

type findingsCategory string

const (
	findingsCategorySOQL     findingsCategory = "soql"
	findingsCategoryDML      findingsCategory = "dml"
	findingsCategoryDescribe findingsCategory = "describe"
	findingsCategoryAsync    findingsCategory = "async"
)

type scanApexFileState struct {
	path          string
	source        string
	report        *Report
	resolver      *platformCallResolver
	asyncMethods  map[string]asyncMethodMetadata
	testMethods   map[string]struct{}
	methodStack   []asyncMethodFrame
	asyncGraph    *asyncCallGraph
	loops         []*loopFindings
	loopStack     []*loopFindings
	describeLines []int
	scopes        []map[string]bool
}

type asyncMethodMetadata struct {
	className            string
	methodName           string
	line                 int
	isFuture             bool
	isFutureCallout      bool
	isQueueableExecute   bool
	isBatchExecute       bool
	isBatchFinish        bool
	isSchedulableExecute bool
}

type asyncMethodFrame struct {
	meta          asyncMethodMetadata
	hasMeta       bool
	sawFutureDml  bool
	describeLines []int
}

type asyncTypeFlags struct {
	queueable   bool
	batchable   bool
	schedulable bool
}

type asyncCallEdge struct {
	line int
	file string
}

type asyncCallGraph struct {
	typeFlags      map[string]asyncTypeFlags
	queueableEdges map[string]map[string]asyncCallEdge
	batchEdges     map[string]map[string]asyncCallEdge
	batchQueueable map[string]map[string]asyncCallEdge
	scheduleEdges  map[string]map[string]asyncCallEdge
}

func collectAsyncMethodsByFile(parsed apexast.Result, index typesys.Index) map[string]map[string]asyncMethodMetadata {
	typeFlags := collectAsyncTypeFlags(index)
	out := make(map[string]map[string]asyncMethodMetadata, len(parsed.Files))
	for _, file := range parsed.Files {
		methodMap := make(map[string]asyncMethodMetadata)
		collectAsyncMethodsFromDeclarations(file.Declarations, nil, typeFlags, methodMap)
		out[file.Path] = methodMap
	}
	return out
}

func collectTestMethodsByFile(parsed apexast.Result) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{}, len(parsed.Files))
	for _, file := range parsed.Files {
		methodMap := make(map[string]struct{})
		collectTestMethodsFromDeclarations(file.Declarations, false, methodMap)
		out[file.Path] = methodMap
	}
	return out
}

func collectTestMethodsFromDeclarations(decls []apexast.Declaration, parentIsTestClass bool, out map[string]struct{}) {
	for _, decl := range decls {
		switch decl.Kind {
		case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			nextIsTestClass := parentIsTestClass || hasIsTestClassModifier(decl.Modifiers)
			collectTestMethodsFromDeclarations(decl.Members, nextIsTestClass, out)
		case apexast.DeclarationMethod:
			if nextIsTestMethod(parentIsTestClass, decl.Modifiers) {
				out[strconv.Itoa(decl.Range.Start.Offset)+":"+strconv.Itoa(decl.Range.End.Offset)] = struct{}{}
			}
		}
	}
}

func hasIsTestClassModifier(modifiers []string) bool {
	for _, modifier := range modifiers {
		normalized := strings.ToLower(strings.TrimSpace(modifier))
		normalized = strings.ReplaceAll(normalized, " ", "")
		normalized = strings.TrimPrefix(normalized, "@")
		if strings.EqualFold(normalized, "istest") {
			return true
		}
	}
	return false
}

func nextIsTestMethod(parentIsTestClass bool, modifiers []string) bool {
	if parentIsTestClass {
		return true
	}
	for _, modifier := range modifiers {
		normalized := strings.ToLower(strings.TrimSpace(modifier))
		normalized = strings.ReplaceAll(normalized, " ", "")
		normalized = strings.TrimPrefix(normalized, "@")
		if strings.EqualFold(normalized, "istest") || strings.EqualFold(normalized, "testmethod") {
			return true
		}
	}
	return false
}

func isTestSourcePath(path string) bool {
	lower := strings.ToLower(path)
	if strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/") {
		return true
	}
	return strings.HasSuffix(lower, "test.cls") || strings.HasSuffix(lower, "tests.cls")
}

func isVendoredSourcePath(path string) bool {
	lower := "/" + strings.Trim(strings.ToLower(filepathSlash(path)), "/") + "/"
	return strings.Contains(lower, "/fflib/") ||
		strings.Contains(lower, "/vendor/") ||
		strings.Contains(lower, "/vendors/") ||
		strings.Contains(lower, "/third-party/") ||
		strings.Contains(lower, "/third_party/")
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func collectAsyncTypeFlags(index typesys.Index) map[string]asyncTypeFlags {
	out := make(map[string]asyncTypeFlags)
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		key := normalizeAsyncTypeName(typ.Name)
		if key == "" {
			continue
		}
		flags := asyncTypeFlags{}
		for _, iface := range typ.Interfaces {
			base := baseTypeName(iface)
			switch strings.ToLower(base) {
			case "queueable":
				flags.queueable = true
			case "batchable":
				flags.batchable = true
			case "schedulable":
				flags.schedulable = true
			}
		}
		out[key] = flags
		short := strings.ToLower(shortTypeName(key))
		if short != "" && short != key {
			if _, ok := out[short]; !ok {
				out[short] = flags
			}
		}
	}
	return out
}

func collectAsyncMethodsFromDeclarations(decls []apexast.Declaration, classStack []string, typeFlags map[string]asyncTypeFlags, out map[string]asyncMethodMetadata) {
	for _, decl := range decls {
		switch decl.Kind {
		case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			nextStack := append(append([]string{}, classStack...), decl.Name)
			collectAsyncMethodsFromDeclarations(decl.Members, nextStack, typeFlags, out)
		case apexast.DeclarationMethod:
			meta := asyncMethodMetadata{
				className:       strings.Join(classStack, "."),
				methodName:      decl.Name,
				line:            decl.Range.Start.Line,
				isFuture:        hasFutureModifier(decl.Modifiers),
				isFutureCallout: hasFutureCalloutModifier(decl.Modifiers),
			}
			classFlags := asyncTypeFlagsFromName(meta.className, typeFlags)
			meta.isQueueableExecute = classFlags.queueable && strings.EqualFold(decl.Name, "execute") && hasQueueableContextParam(decl.Parameters)
			meta.isBatchExecute = classFlags.batchable && strings.EqualFold(decl.Name, "execute") && hasBatchExecuteParams(decl.Parameters)
			meta.isBatchFinish = classFlags.batchable && strings.EqualFold(decl.Name, "finish") && hasBatchFinishParams(decl.Parameters)
			meta.isSchedulableExecute = classFlags.schedulable && strings.EqualFold(decl.Name, "execute") && hasSchedulableContextParam(decl.Parameters)
			out[strconv.Itoa(decl.Range.Start.Offset)+":"+strconv.Itoa(decl.Range.End.Offset)] = meta
		}
	}
}

func asyncTypeFlagsFromName(name string, typeFlags map[string]asyncTypeFlags) asyncTypeFlags {
	key := strings.ToLower(normalizeAsyncTypeName(name))
	if flags, ok := typeFlags[key]; ok {
		return flags
	}
	short := strings.ToLower(shortTypeName(key))
	if flags, ok := typeFlags[short]; ok {
		return flags
	}
	return asyncTypeFlags{}
}

func normalizeAsyncTypeName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if i := strings.IndexRune(trimmed, '<'); i >= 0 {
		trimmed = trimmed[:i]
	}
	trimmed = strings.TrimSpace(trimmed)
	return trimmed
}

func baseTypeName(name string) string {
	base := normalizeAsyncTypeName(name)
	if dot := strings.LastIndex(base, "."); dot >= 0 {
		base = base[dot+1:]
	}
	return base
}

func shortTypeName(name string) string {
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

func newAsyncCallGraph(index typesys.Index) *asyncCallGraph {
	return &asyncCallGraph{
		typeFlags:      collectAsyncTypeFlags(index),
		queueableEdges: make(map[string]map[string]asyncCallEdge),
		batchEdges:     make(map[string]map[string]asyncCallEdge),
		batchQueueable: make(map[string]map[string]asyncCallEdge),
		scheduleEdges:  make(map[string]map[string]asyncCallEdge),
	}
}

func (g *asyncCallGraph) addQueueableEdge(from, to string, line int, file string) bool {
	if !g.isQueueableType(from) || !g.isQueueableType(to) || from == "" || to == "" {
		return false
	}
	addAsyncEdge(g.queueableEdges, from, to, line, file)
	return true
}

func (g *asyncCallGraph) addBatchEdge(from, to string, line int, file string) bool {
	if !g.isBatchableType(to) || from == "" || to == "" {
		return false
	}
	addAsyncEdge(g.batchEdges, from, to, line, file)
	return true
}

func (g *asyncCallGraph) addBatchQueueableEdge(from, to string, line int, file string) bool {
	if !g.isQueueableType(to) || from == "" || to == "" {
		return false
	}
	addAsyncEdge(g.batchQueueable, from, to, line, file)
	return true
}

func (g *asyncCallGraph) addScheduleEdge(from, to string, line int, file string) bool {
	if !g.isSchedulableType(to) || from == "" || to == "" {
		return false
	}
	addAsyncEdge(g.scheduleEdges, from, to, line, file)
	return true
}

func addAsyncEdge(graph map[string]map[string]asyncCallEdge, from, to string, line int, file string) {
	edges, ok := graph[from]
	if !ok {
		edges = make(map[string]asyncCallEdge)
		graph[from] = edges
	}
	if _, exists := edges[to]; !exists {
		edges[to] = asyncCallEdge{line: line, file: file}
	}
}

func (g *asyncCallGraph) isQueueableType(name string) bool {
	return asyncTypeFlagsFromName(name, g.typeFlags).queueable
}

func (g *asyncCallGraph) isBatchableType(name string) bool {
	return asyncTypeFlagsFromName(name, g.typeFlags).batchable
}

func (g *asyncCallGraph) isSchedulableType(name string) bool {
	return asyncTypeFlagsFromName(name, g.typeFlags).schedulable
}

func (g *asyncCallGraph) emitFindings(report *Report) {
	g.emitQueueableFindings(report)
	g.emitBatchFindings(report)
	g.emitScheduleFindings(report)
}

func (g *asyncCallGraph) emitQueueableFindings(report *Report) {
	for _, cycle := range findCycles(g.queueableEdges) {
		line, file := edgeLine(g.queueableEdges, cycle[0], cycle[1%len(cycle)])
		if isSelfContinuationCycle(cycle) {
			report.AddFinding(Finding{
				ID:         "perf.async.queueable.self-continuation",
				Category:   CategoryAsync,
				Severity:   SeverityMedium,
				Confidence: ConfidenceStatic,
				Score:      78,
				EntryPoint: EntryPoint{Kind: EntryUnknown},
				Message:    "Queueable execute re-enqueues the same class as a continuation while work remains.",
				Location:   Location{File: file, Line: line},
				Evidence:   []Evidence{{Kind: "async", Message: "queueable continuation", Value: strings.Join(cycle, " -> ")}},
				Fix:        "Verify the payload shrinks on every run, cap chain depth where possible, and prefer batch or scheduled work for large subscriber-org backlogs.",
			})
			continue
		}
		report.AddFinding(Finding{
			ID:         "perf.async.queueable.cycle",
			Category:   CategoryAsync,
			Severity:   SeverityHigh,
			Confidence: ConfidenceStatic,
			Score:      94,
			EntryPoint: EntryPoint{Kind: EntryUnknown},
			Message:    "Queueable chaining has a cycle and can recurse until async limits are reached.",
			Location:   Location{File: file, Line: line},
			Evidence:   []Evidence{{Kind: "async", Message: "queueable chain", Value: strings.Join(cycle, " -> ")}},
			Fix:        "Limit queueable handoff depth and include state checks to stop unbounded chaining.",
		})
	}
	for _, deep := range findLongChains(g.queueableEdges, maxAsyncChainDepth) {
		report.AddFinding(Finding{
			ID:         "perf.async.queueable.chain-depth",
			Category:   CategoryAsync,
			Severity:   SeverityHigh,
			Confidence: ConfidenceStatic,
			Score:      93,
			EntryPoint: EntryPoint{Kind: EntryUnknown},
			Message:    "Queueable chain depth exceeds the safe limit and can overflow chain safety boundaries.",
			Location:   Location{File: deep.edge.file, Line: deep.edge.line},
			Evidence:   []Evidence{{Kind: "async", Message: "queueable chain", Value: strings.Join(deep.chain, " -> ")}},
			Fix:        "Replace unbounded queueable handoff with explicit completion conditions and per-run limits.",
		})
	}
}

func isSelfContinuationCycle(cycle []string) bool {
	return len(cycle) == 2 && strings.EqualFold(cycle[0], cycle[1])
}

func (g *asyncCallGraph) emitBatchFindings(report *Report) {
	for _, cycle := range findCycles(g.batchEdges) {
		line, file := edgeLine(g.batchEdges, cycle[0], cycle[1%len(cycle)])
		report.AddFinding(Finding{
			ID:         "perf.async.batch.unbounded-cycle",
			Category:   CategoryAsync,
			Severity:   SeverityHigh,
			Confidence: ConfidenceStatic,
			Score:      92,
			EntryPoint: EntryPoint{Kind: EntryUnknown},
			Message:    "Batch chaining forms a cycle and can run indefinitely across execution contexts.",
			Location:   Location{File: file, Line: line},
			Evidence:   []Evidence{{Kind: "async", Message: "batch chain", Value: strings.Join(cycle, " -> ")}},
			Fix:        "Stop recursive batch chaining and move state checks to avoid uncontrolled fan-out.",
		})
	}
	for _, deep := range findLongChains(g.batchEdges, maxAsyncChainDepth) {
		report.AddFinding(Finding{
			ID:         "perf.async.batch.unbounded-chain",
			Category:   CategoryAsync,
			Severity:   SeverityHigh,
			Confidence: ConfidenceStatic,
			Score:      91,
			EntryPoint: EntryPoint{Kind: EntryUnknown},
			Message:    "Batch execute/finish chaining can grow beyond the safe chain depth.",
			Location:   Location{File: deep.edge.file, Line: deep.edge.line},
			Evidence:   []Evidence{{Kind: "async", Message: "batch chain", Value: strings.Join(deep.chain, " -> ")}},
			Fix:        "Break batch fan-out by re-queuing through bounded checkpoints and durable state.",
		})
	}
}

func (g *asyncCallGraph) emitScheduleFindings(report *Report) {
	for _, cycle := range findCycles(g.scheduleEdges) {
		line, file := edgeLine(g.scheduleEdges, cycle[0], cycle[1%len(cycle)])
		report.AddFinding(Finding{
			ID:         "perf.async.schedule.recursive",
			Category:   CategoryAsync,
			Severity:   SeverityHigh,
			Confidence: ConfidenceStatic,
			Score:      90,
			EntryPoint: EntryPoint{Kind: EntryUnknown},
			Message:    "Schedulable classes reschedule themselves and can create run-away job loops.",
			Location:   Location{File: file, Line: line},
			Evidence:   []Evidence{{Kind: "async", Message: "schedule chain", Value: strings.Join(cycle, " -> ")}},
			Fix:        "Use explicit termination checks and central schedule configuration to avoid recursive rescheduling.",
		})
	}
}

func (g *asyncCallGraph) allNodesFromGraph(graph map[string]map[string]asyncCallEdge) map[string]struct{} {
	nodes := make(map[string]struct{})
	for from, edges := range graph {
		nodes[from] = struct{}{}
		for to := range edges {
			nodes[to] = struct{}{}
		}
	}
	return nodes
}

type deepChainFinding struct {
	chain []string
	edge  asyncCallEdge
}

func findLongChains(graph map[string]map[string]asyncCallEdge, maxDepth int) []deepChainFinding {
	nodes := make(map[string]struct{})
	for from, edges := range graph {
		nodes[from] = struct{}{}
		for to := range edges {
			nodes[to] = struct{}{}
		}
	}
	var findings []deepChainFinding
	type chainState struct {
		chain []string
		edge  asyncCallEdge
	}
	for start := range nodes {
		visited := map[string]bool{start: true}
		state := []string{start}
		var dfs func(current string)
		dfs = func(current string) {
			for to, edge := range graph[current] {
				if visited[to] {
					continue
				}
				next := append(state, to)
				if len(next)-1 > maxDepth {
					findings = append(findings, deepChainFinding{chain: append([]string{}, next...), edge: edge})
					continue
				}
				visited[to] = true
				state = append(state, to)
				dfs(to)
				state = state[:len(state)-1]
				delete(visited, to)
			}
		}
		dfs(start)
	}
	return findings
}

func findCycles(graph map[string]map[string]asyncCallEdge) [][]string {
	nodes := make(map[string]struct{})
	for from, edges := range graph {
		nodes[from] = struct{}{}
		for to := range edges {
			nodes[to] = struct{}{}
		}
	}
	state := make(map[string]int)
	stack := []string{}
	stackIndex := make(map[string]int)
	seen := make(map[string]struct{})
	var cycles [][]string

	var dfs func(node string)
	dfs = func(node string) {
		state[node] = 1
		stackIndex[node] = len(stack)
		stack = append(stack, node)
		for to := range graph[node] {
			switch state[to] {
			case 0:
				dfs(to)
			case 1:
				start := stackIndex[to]
				cycle := append([]string{}, stack[start:]...)
				cycle = append(cycle, to)
				key := cycleKey(cycle)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				cycles = append(cycles, cycle)
			}
		}
		delete(stackIndex, node)
		stack = stack[:len(stack)-1]
		state[node] = 2
	}
	for node := range nodes {
		if state[node] == 0 {
			dfs(node)
		}
	}
	sort.Slice(cycles, func(i, j int) bool {
		return strings.Join(cycles[i], "->") < strings.Join(cycles[j], "->")
	})
	return cycles
}

func edgeLine(graph map[string]map[string]asyncCallEdge, from, to string) (int, string) {
	if edges, ok := graph[from]; ok {
		if edge, ok := edges[to]; ok {
			return edge.line, edge.file
		}
	}
	return 0, ""
}

func cycleKey(cycle []string) string {
	unique := make([]string, len(cycle))
	copy(unique, cycle)
	sort.Strings(unique)
	return strings.Join(unique, "|")
}

func hasFutureModifier(modifiers []string) bool {
	for _, modifier := range modifiers {
		text := strings.ToLower(strings.TrimSpace(modifier))
		text = strings.ReplaceAll(text, " ", "")
		if text == "future" || strings.HasPrefix(text, "future(") || strings.HasPrefix(text, "@future") {
			return true
		}
	}
	return false
}

func hasFutureCalloutModifier(modifiers []string) bool {
	for _, modifier := range modifiers {
		text := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(modifier), " ", ""))
		text = strings.ReplaceAll(text, "@", "")
		if !strings.HasPrefix(text, "future") {
			continue
		}
		return strings.Contains(text, "callout=true")
	}
	return false
}

func hasQueueableContextParam(params []apexast.Parameter) bool {
	if len(params) != 1 {
		return false
	}
	return isTypeNameMatch(params[0].Type, "QueueableContext")
}

func hasBatchExecuteParams(params []apexast.Parameter) bool {
	if len(params) != 2 {
		return false
	}
	return isTypeNameMatch(params[0].Type, "Database.BatchableContext") &&
		isListType(params[1].Type)
}

func hasBatchFinishParams(params []apexast.Parameter) bool {
	if len(params) != 1 {
		return false
	}
	return isTypeNameMatch(params[0].Type, "Database.BatchableContext")
}

func hasSchedulableContextParam(params []apexast.Parameter) bool {
	if len(params) != 1 {
		return false
	}
	return isTypeNameMatch(params[0].Type, "SchedulableContext")
}

func isTypeNameMatch(actualType, expected string) bool {
	base := baseTypeName(normalizeAsyncTypeName(actualType))
	expectedBase := baseTypeName(normalizeAsyncTypeName(expected))
	return strings.EqualFold(base, expectedBase)
}

func isListType(value string) bool {
	t := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(t, "list<") || strings.EqualFold(baseTypeName(value), "List")
}

type loopFindings struct {
	line        int
	depth       int
	hasSoql     bool
	hasDml      bool
	hasDescribe bool
	hasAsync    bool
}

func isTypeNode(kind string) bool {
	switch kind {
	case "type_identifier", "scoped_type_identifier", "generic_type", "type_name", "scoped_identifier":
		return true
	}
	return false
}

func isSObjectTypeType(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return false
	}
	if strings.EqualFold(typeName, "SObjectType") {
		return true
	}
	return strings.HasSuffix(typeName, ".SObjectType")
}

type platformCallResolver struct {
	typesByName map[string]typesys.TypeSymbol
}

func newPlatformCallResolver(index typesys.Index) *platformCallResolver {
	resolver := &platformCallResolver{typesByName: map[string]typesys.TypeSymbol{}}
	for _, typ := range index.Types {
		if typ.Name == "" {
			continue
		}
		resolver.addTypeSymbol(typ)
	}
	for _, typ := range typesys.StandardPlatformSymbols() {
		resolver.addTypeSymbol(typ)
	}
	return resolver
}

func (r *platformCallResolver) addTypeSymbol(typ typesys.TypeSymbol) {
	for _, key := range typeResolverKeys(typ.Name) {
		if _, seen := r.typesByName[key]; !seen {
			r.typesByName[key] = typ
		}
	}
}

func (r *platformCallResolver) isShadowedType(typeName string) bool {
	if _, ok := r.typesByName[strings.ToLower(typeName)]; !ok {
		return false
	}
	for _, typ := range typesys.StandardPlatformSymbols() {
		if strings.EqualFold(typ.Name, typeName) {
			return false
		}
	}
	return true
}

func (r *platformCallResolver) hasStaticMethod(typeName, methodName string) bool {
	typ, ok := r.typesByName[strings.ToLower(typeName)]
	if !ok {
		return false
	}
	for _, member := range typ.Members {
		if member.Kind != apexast.DeclarationMethod {
			continue
		}
		if strings.EqualFold(member.Name, methodName) && hasModifier(member.Modifiers, "static") {
			return true
		}
	}
	return false
}

func typeResolverKeys(name string) []string {
	parts := strings.Split(name, ".")
	if len(parts) == 0 {
		return nil
	}
	keys := make([]string, 0, len(parts)+1)
	keys = append(keys, strings.ToLower(name))
	keys = append(keys, strings.ToLower(parts[len(parts)-1]))
	return keys
}

func hasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(strings.TrimSpace(modifier), expected) {
			return true
		}
	}
	return false
}

func classNameFromSource(path, source string) string {
	classRe := regexp.MustCompile(`(?i)\bclass\s+([A-Za-z_][A-Za-z0-9_]*)`)
	match := classRe.FindStringSubmatch(source)
	if len(match) > 1 {
		return match[1]
	}
	return path
}

func soqlSelectivityScore(condition soql.Condition) (int, []string) {
	score := 0
	reasons := make([]string, 0)
	for _, andCond := range condition.And {
		subScore, subReasons := soqlSelectivityScore(andCond)
		score += subScore
		reasons = append(reasons, subReasons...)
	}
	for _, orCond := range condition.Or {
		subScore, subReasons := soqlSelectivityScore(orCond)
		score += subScore
		reasons = append(reasons, subReasons...)
	}
	if len(condition.And) == 0 && len(condition.Or) == 0 && condition.Field != "" {
		leafScore, leafReason := soqlPredicateScore(condition)
		score += leafScore
		if leafReason != "" {
			reasons = append(reasons, leafReason)
		}
	}
	return score, reasons
}

func soqlPredicateScore(cond soql.Condition) (int, string) {
	field := strings.TrimSpace(cond.Field)
	op := strings.ToUpper(strings.TrimSpace(cond.Op))
	base := 0
	reason := ""

	if isFormulaField(field) {
		base += 2
		reason = "Formula field in WHERE"
	}

	if cond.Not {
		base += 3
		if reason != "" {
			reason = reason + ", NOT predicate"
		} else {
			reason = "NOT predicate"
		}
	}

	switch op {
	case "LIKE":
		if hasLeadingWildcardValue(cond.Value, cond.Values) {
			base += 3
			if reason != "" {
				reason += ", LIKE with leading wildcard"
			} else {
				reason = "LIKE with leading wildcard"
			}
		}
	case "NOT LIKE":
		base += 3
		if reason != "" {
			reason += ", NOT LIKE"
		} else {
			reason = "NOT LIKE"
		}
	case "!=":
		base += 3
		if reason != "" {
			reason += ", " + op
		} else {
			reason = op
		}
	case "NOT IN":
		base += 3
		if reason != "" {
			reason += ", NOT IN"
		} else {
			reason = "NOT IN"
		}
	case "=":
		switch {
		case isHighlyIndexedField(field):
			base += 0
		case isMediumSelectivityField(field):
			base += 1
		default:
			base += 2
		}
		if reason == "" {
			reason = "equals comparison"
		}
	case "IN":
		base += 1
		if reason == "" {
			reason = "IN operator"
		}
	case ">", ">=", "<", "<=":
		base += 2
		if reason == "" {
			reason = op + " comparison"
		}
	default:
		base += 2
	}

	return base, reason
}

func isFormulaField(field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}
	if strings.Contains(field, "(") && strings.HasSuffix(field, ")") {
		return true
	}
	if strings.Contains(field, "Formula") || strings.Contains(strings.ToLower(field), "formula") {
		return true
	}
	segments := strings.Split(field, ".")
	last := strings.TrimSpace(segments[len(segments)-1])
	return last == "BillingAddress" || last == "MailingAddress" || last == "OtherAddress"
}

func isHighlyIndexedField(field string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "id", "name", "ownerid", "createddate", "systemmodstamp", "recordtypeid":
		return true
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(field)), "id")
}

func isMediumSelectivityField(field string) bool {
	field = strings.ToLower(strings.TrimSpace(field))
	if strings.Contains(field, "date") || strings.HasSuffix(field, "date") {
		return true
	}
	switch field {
	case "name":
		return true
	}
	return false
}

func hasLeadingWildcardValue(first storage.Value, others []storage.Value) bool {
	if hasWildcard(first) {
		return true
	}
	for _, value := range others {
		if hasWildcard(value) {
			return true
		}
	}
	return false
}

func hasWildcard(value storage.Value) bool {
	switch value.Kind {
	case storage.ValueString:
		return strings.HasPrefix(value.String, "%")
	case storage.ValueDate, storage.ValueDateTime:
		return false
	default:
		return false
	}
}

func isIndexedField(field string) bool {
	return isHighlyIndexedField(strings.TrimSpace(field))
}
