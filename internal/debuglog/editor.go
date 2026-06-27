package debuglog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/typesys"
)

type EditorOptions struct {
	LogFile       string
	ProjectRoot   string
	ObjectFiles   []string
	FieldFiles    []string
	MaxCandidates int
	MinConfidence float64
	Now           func() time.Time
}

type editorFrame struct {
	ID       string
	Kind     string
	Name     string
	Entry    int
	Range    EditorRange
	Depth    int
	ParentID string
	Source   EditorLocation
	Children []EditorSymbol
}

type editorVarScope struct {
	Name      string
	Type      string
	FrameID   string
	Range     EditorRange
	LogDef    EditorLocation
	SourceDef EditorLocation
}

// BuildEditorAnalysis converts a parsed Apex debug log into an editor-friendly
// map. It keeps source matching in Annotate and adds ranges, hierarchy, and
// runtime scopes for editor providers.
func BuildEditorAnalysis(log apexlog.Log, index typesys.Index, opts EditorOptions) EditorAnalysis {
	if opts.MaxCandidates <= 0 {
		opts.MaxCandidates = 5
	}
	if opts.MinConfidence <= 0 {
		opts.MinConfidence = 0.35
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	annotated, _ := Annotate(log, index, opts.MaxCandidates)

	analysis := EditorAnalysis{
		Version:     1,
		LogFile:     opts.LogFile,
		ProjectRoot: opts.ProjectRoot,
		Language:    "apexlog",
		GeneratedAt: now().UTC().Format(time.RFC3339),
		Entries:     make([]EditorEntry, 0, len(log.Entries)),
		Coverage: EditorCoverage{
			TotalEntries: len(log.Entries),
		},
	}

	sourceIndex := BuildSourceIndex(index)
	replayable := replayableEntryIndexes(annotated, sourceIndex, opts.MinConfidence)
	schemaIndex := buildEditorSchemaIndex(opts.ProjectRoot, opts.ObjectFiles, opts.FieldFiles)
	var stack []editorFrame
	activeVars := map[string]editorVarScope{}
	variablesByKey := map[string]*EditorVariable{}
	symbolsByFrame := map[string]*EditorSymbol{}
	parentByFrame := map[string]string{}

	for i, entry := range log.Entries {
		rng := rangeForEntry(entry)
		depth := editorDepth(len(stack))
		parentID := ""
		if len(stack) > 0 {
			parentID = stack[len(stack)-1].ID
		}
		source := EditorLocation{}
		if i < len(annotated.Entries) {
			source = locationFromCandidate(annotated.Entries[i].Best)
			if !locationMeetsConfidence(source, opts.MinConfidence) {
				source = EditorLocation{}
			}
			if source.File != "" {
				analysis.Coverage.ResolvedSources++
			}
		}

		editorEntry := EditorEntry{
			Index:    i,
			Kind:     string(entry.Kind),
			Raw:      entry.Raw,
			Range:    rng,
			Depth:    depth,
			ParentID: parentID,
			Fields:   fieldsForEditorEntry(entry),
			Source:   source,
		}

		switch entry.Kind {
		case apexlog.EntryExecutionStarted:
			frame := editorFrame{ID: frameID(i, "execution"), Kind: "execution", Name: "Execution", Entry: i, Range: rng, Depth: editorDepth(len(stack)), ParentID: parentID}
			stack = append(stack, frame)
			editorEntry.FrameID = frame.ID
		case apexlog.EntryCodeUnitStarted:
			name := displayCodeUnit(entry.Data.CodeUnit)
			frame := editorFrame{ID: frameID(i, name), Kind: "codeUnit", Name: name, Entry: i, Range: rng, Depth: editorDepth(len(stack)), ParentID: parentID, Source: source}
			stack = append(stack, frame)
			editorEntry.FrameID = frame.ID
			addSourceLink(&analysis, "source", rng, source)
		case apexlog.EntryMethodEntry, apexlog.EntryConstructorEntry, apexlog.EntrySystemMethodEntry:
			name := displayMethodSymbol(entry.Data.MethodSymbol)
			kind := "method"
			if entry.Kind == apexlog.EntryConstructorEntry {
				kind = "constructor"
			}
			if entry.Kind == apexlog.EntrySystemMethodEntry {
				kind = "systemMethod"
			}
			if source.File == "" {
				source = methodEntryLocation(sourceIndex, entry.Data.MethodSymbol)
			} else if methodSource := methodEntryLocation(sourceIndex, entry.Data.MethodSymbol); methodSource.File != "" {
				source = methodSource
			}
			if !locationMeetsConfidence(source, opts.MinConfidence) {
				source = EditorLocation{}
			}
			frame := editorFrame{ID: frameID(i, name), Kind: kind, Name: name, Entry: i, Range: rng, Depth: editorDepth(len(stack)), ParentID: parentID, Source: source}
			stack = append(stack, frame)
			editorEntry.FrameID = frame.ID
			addSourceLink(&analysis, "method", rng, source)
		case apexlog.EntrySOQLExecuteBegin:
			addSourceLink(&analysis, "source", rng, source)
			addSOQLSchemaLinks(&analysis, entry, rng, schemaIndex)
			frame := editorFrame{ID: frameID(i, "soql"), Kind: "soql", Name: "SOQL", Entry: i, Range: rng, Depth: editorDepth(len(stack)), ParentID: parentID, Source: source}
			stack = append(stack, frame)
			editorEntry.FrameID = frame.ID
		case apexlog.EntryDMLBegin:
			addSourceLink(&analysis, "source", rng, source)
			addDMLSchemaLink(&analysis, entry, rng, schemaIndex)
			frame := editorFrame{ID: frameID(i, "dml"), Kind: "dml", Name: strings.TrimSpace(entry.Data.DMLOperation + " " + entry.Data.DMLType), Entry: i, Range: rng, Depth: editorDepth(len(stack)), ParentID: parentID, Source: source}
			stack = append(stack, frame)
			editorEntry.FrameID = frame.ID
		case apexlog.EntryCumulativeLimitUsage, apexlog.EntryLimitUsageForNamespace:
			frame := editorFrame{ID: frameID(i, "limits"), Kind: "limits", Name: "Limits", Entry: i, Range: rng, Depth: editorDepth(len(stack)), ParentID: parentID}
			stack = append(stack, frame)
			editorEntry.FrameID = frame.ID
		case apexlog.EntryVariableScopeBegin:
			scope := editorVarScope{
				Name:    entry.Data.VariableName,
				Type:    entry.Data.VariableType,
				FrameID: parentID,
				Range:   rng,
				LogDef:  EditorLocation{File: opts.LogFile, Line: rng.StartLine + 1, Column: rng.StartColumn, Symbol: entry.Data.VariableName, Reason: "variable scope", Confidence: 1},
			}
			scope.SourceDef = variableSourceLocation(sourceIndex, parentFrame(stack), entry.Data.VariableName)
			if !locationMeetsConfidence(scope.SourceDef, opts.MinConfidence) {
				scope.SourceDef = EditorLocation{}
			}
			activeVars[varKey(parentID, entry.Data.VariableName)] = scope
			addVariableLinks(&analysis, scope, rng, false)
		case apexlog.EntryVariableAssignment:
			scope, ok := activeVars[varKey(parentID, entry.Data.VariableName)]
			if !ok {
				scope, ok = findActiveVar(activeVars, entry.Data.VariableName)
			}
			if !ok {
				analysis.Diagnostics = append(analysis.Diagnostics, EditorDiagnostic{Range: rng, Severity: "warning", Code: "apexlog.unscopedVariable", Message: "Variable assignment has no matching VARIABLE_SCOPE_BEGIN."})
				break
			}
			key := varKey(scope.FrameID, scope.Name)
			variable := variablesByKey[key]
			if variable == nil {
				variable = &EditorVariable{Name: scope.Name, Type: scope.Type, ScopeID: scope.FrameID, Range: scope.Range, LogDef: scope.LogDef, SourceDef: scope.SourceDef}
				variablesByKey[key] = variable
			}
			variable.Value = entry.Data.VariableValue
			variable.Assignment = EditorLocation{File: opts.LogFile, Line: rng.StartLine + 1, Column: rng.StartColumn, Symbol: scope.Name, Reason: "variable assignment", Confidence: 1}
			addVariableLinks(&analysis, scope, rng, true)
			if scope.SourceDef.File != "" {
				analysis.Coverage.ResolvedVariables++
			}
		case apexlog.EntrySOQLExecuteEnd:
			closeFrameKind(&analysis, &stack, "soql", i, rng, symbolsByFrame, parentByFrame, replayable, activeVars)
		case apexlog.EntryDMLEnd:
			closeFrameKind(&analysis, &stack, "dml", i, rng, symbolsByFrame, parentByFrame, replayable, activeVars)
		case apexlog.EntryCumulativeLimitUsageEnd:
			closeFrameKind(&analysis, &stack, "limits", i, rng, symbolsByFrame, parentByFrame, replayable, activeVars)
		case apexlog.EntryCodeUnitFinished:
			closeFrameKind(&analysis, &stack, "codeUnit", i, rng, symbolsByFrame, parentByFrame, replayable, activeVars)
		case apexlog.EntryMethodExit, apexlog.EntryConstructorExit, apexlog.EntrySystemMethodExit:
			kind := "method"
			if entry.Kind == apexlog.EntryConstructorExit {
				kind = "constructor"
			}
			if entry.Kind == apexlog.EntrySystemMethodExit {
				kind = "systemMethod"
			}
			if !closeFrameKind(&analysis, &stack, kind, i, rng, symbolsByFrame, parentByFrame, replayable, activeVars) {
				analysis.Diagnostics = append(analysis.Diagnostics, EditorDiagnostic{Range: rng, Severity: "warning", Code: "apexlog.unmatchedExit", Message: "Exit event has no matching entry event."})
			}
		case apexlog.EntryExecutionFinished:
			closeFrameKind(&analysis, &stack, "execution", i, rng, symbolsByFrame, parentByFrame, replayable, activeVars)
		case apexlog.EntryExceptionThrown, apexlog.EntryFatalError:
			analysis.Diagnostics = append(analysis.Diagnostics, EditorDiagnostic{Range: rng, Severity: "error", Code: "apexlog.exception", Message: strings.TrimSpace(entry.Data.ExceptionType + " " + entry.Data.ExceptionText)})
		default:
			if entry.Kind == apexlog.EntryOther {
				analysis.Coverage.ParserWarnings++
			}
		}

		analysis.Entries = append(analysis.Entries, editorEntry)
		addEntryHoverAndTokens(&analysis, entry, rng, source)
	}

	lastLine := 0
	if len(log.Entries) > 0 {
		lastLine = log.Entries[len(log.Entries)-1].Line - 1
	}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		end := EditorRange{StartLine: lastLine, EndLine: lastLine}
		addFoldAndSymbol(&analysis, frame, len(log.Entries)-1, end, symbolsByFrame, parentByFrame, replayable)
		analysis.Diagnostics = append(analysis.Diagnostics, EditorDiagnostic{
			Range:    end,
			Severity: "warning",
			Code:     "apexlog.unclosedFrame",
			Message:  "Entry event was still open at end of log.",
		})
	}

	for _, variable := range variablesByKey {
		analysis.Variables = append(analysis.Variables, *variable)
	}
	sort.SliceStable(analysis.Variables, func(i, j int) bool {
		if analysis.Variables[i].Range.StartLine != analysis.Variables[j].Range.StartLine {
			return analysis.Variables[i].Range.StartLine < analysis.Variables[j].Range.StartLine
		}
		return analysis.Variables[i].Name < analysis.Variables[j].Name
	})
	analysis.Symbols = buildEditorSymbolTree(symbolsByFrame, parentByFrame)
	sort.SliceStable(analysis.Links, func(i, j int) bool {
		if analysis.Links[i].Range.StartLine != analysis.Links[j].Range.StartLine {
			return analysis.Links[i].Range.StartLine < analysis.Links[j].Range.StartLine
		}
		return analysis.Links[i].Range.StartColumn < analysis.Links[j].Range.StartColumn
	})
	return analysis
}

func rangeForEntry(entry apexlog.Entry) EditorRange {
	line := entry.Line - 1
	if line < 0 {
		line = 0
	}
	return EditorRange{StartLine: line, StartColumn: 0, EndLine: line, EndColumn: utf16ColumnLen(entry.Raw)}
}

func fieldsForEditorEntry(entry apexlog.Entry) map[string]any {
	fields := map[string]any{}
	if entry.Data.SourceLine > 0 {
		fields["sourceLine"] = entry.Data.SourceLine
	}
	if entry.Data.MethodSymbol != "" {
		fields["methodSymbol"] = entry.Data.MethodSymbol
	}
	if entry.Data.VariableName != "" {
		fields["variableName"] = entry.Data.VariableName
	}
	if entry.Data.VariableType != "" {
		fields["variableType"] = entry.Data.VariableType
	}
	if entry.Data.VariableValue != "" {
		fields["variableValue"] = entry.Data.VariableValue
	}
	if entry.Data.SOQLQuery != "" {
		fields["soqlQuery"] = entry.Data.SOQLQuery
	}
	if entry.Data.SOQLRows > 0 {
		fields["soqlRows"] = entry.Data.SOQLRows
	}
	if entry.Data.DMLType != "" {
		fields["dmlType"] = entry.Data.DMLType
	}
	if entry.Data.HeapBytes > 0 {
		fields["heapBytes"] = entry.Data.HeapBytes
	}
	return fields
}

func locationFromCandidate(candidate SourceCandidate) EditorLocation {
	if candidate.File == "" {
		return EditorLocation{}
	}
	return EditorLocation{
		File:       candidate.File,
		Line:       candidate.Line,
		Symbol:     candidate.Symbol,
		Reason:     candidate.Reason,
		Confidence: candidate.Confidence,
	}
}

func locationMeetsConfidence(location EditorLocation, minConfidence float64) bool {
	if location.File == "" {
		return false
	}
	if minConfidence <= 0 {
		return true
	}
	return location.Confidence >= minConfidence
}

func methodEntryLocation(index SourceIndex, symbol string) EditorLocation {
	ns, typ, method := parseMethodLikeSymbol(symbol)
	for _, sourceMethod := range methodBySymbol(index, ns, typ, method) {
		return EditorLocation{File: sourceMethod.File, Line: sourceMethod.StartLine, Symbol: methodSymbol(sourceMethod.Namespace, sourceMethod.TypeName, sourceMethod.Name), Reason: "method entry", Confidence: 0.9}
	}
	return EditorLocation{}
}

func variableSourceLocation(index SourceIndex, frame editorFrame, name string) EditorLocation {
	if frame.Name == "" || name == "" {
		return EditorLocation{}
	}
	ns, typ, method := parseMethodLikeSymbol(frame.Name)
	if typ == "" || method == "" {
		ns, typ, method = parseMethodLikeSymbol(frame.Source.Symbol)
	}
	for _, sourceMethod := range methodBySymbol(index, ns, typ, method) {
		line := findVariableDeclarationLine(sourceMethod.File, name, sourceMethod.StartLine, sourceMethod.EndLine)
		if line > 0 {
			return EditorLocation{File: sourceMethod.File, Line: line, Symbol: name, Reason: "variable declaration", Confidence: 0.8}
		}
	}
	return EditorLocation{}
}

func findVariableDeclarationLine(file, name string, startLine, endLine int) int {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	name = strings.TrimSpace(name)
	for i, line := range lines {
		lineNo := i + 1
		if lineNo < startLine || lineNo > endLine {
			continue
		}
		if lineNo <= startLine+8 && lineHasParameterDeclaration(line, name) {
			return lineNo
		}
		matches := varDeclarationRe.FindStringSubmatch(line)
		if len(matches) >= 3 && strings.EqualFold(matches[2], name) {
			return lineNo
		}
	}
	return 0
}

func lineHasParameterDeclaration(line, name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	line = strings.Split(line, "//")[0]
	paramRe := regexp.MustCompile(`(?i)(?:^|[,(]\s*)(?:final\s+)?[A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)?(?:<[^>]+>)?(?:\[\])?\s+` + regexp.QuoteMeta(name) + `\b`)
	return paramRe.FindStringIndex(line) != nil
}

func addVariableLinks(analysis *EditorAnalysis, scope editorVarScope, rng EditorRange, assignment bool) {
	linkRange := rng
	kind := "variableLog"
	target := scope.LogDef
	if scope.SourceDef.File != "" {
		kind = "variableSource"
		target = scope.SourceDef
	}
	analysis.Links = append(analysis.Links, EditorLink{Kind: kind, Range: linkRange, Target: target, Title: "Open variable definition"})
	if assignment && scope.LogDef.File != "" && scope.SourceDef.File != "" {
		analysis.Links = append(analysis.Links, EditorLink{Kind: "variableLog", Range: linkRange, Target: scope.LogDef, Title: "Open variable scope"})
	}
}

func addSourceLink(analysis *EditorAnalysis, kind string, rng EditorRange, target EditorLocation) {
	if target.File == "" {
		return
	}
	analysis.Links = append(analysis.Links, EditorLink{Kind: kind, Range: rng, Target: target, Title: "Open Apex source"})
}

func addSOQLSchemaLinks(analysis *EditorAnalysis, entry apexlog.Entry, rng EditorRange, schema editorSchemaIndex) {
	objectName := parseFromObjectPreserveCase(entry.Data.SOQLQuery)
	if objectName == "" {
		return
	}
	if target, ok := schema.objectLocation(objectName); ok {
		analysis.Links = append(analysis.Links, EditorLink{Kind: "schemaObject", Range: rng, Target: target, Title: "Open schema object"})
		analysis.Coverage.ResolvedSchemaRefs++
	}
	for _, field := range parseSelectFields(entry.Data.SOQLQuery) {
		if target, ok := schema.fieldLocation(objectName, field); ok {
			analysis.Links = append(analysis.Links, EditorLink{Kind: "schemaField", Range: rng, Target: target, Title: "Open schema field"})
			analysis.Coverage.ResolvedSchemaRefs++
		}
	}
}

func addDMLSchemaLink(analysis *EditorAnalysis, entry apexlog.Entry, rng EditorRange, schema editorSchemaIndex) {
	if target, ok := schema.objectLocation(entry.Data.DMLType); ok {
		analysis.Links = append(analysis.Links, EditorLink{Kind: "schemaObject", Range: rng, Target: target, Title: "Open schema object"})
		analysis.Coverage.ResolvedSchemaRefs++
	}
}

func closeFrameKind(analysis *EditorAnalysis, stack *[]editorFrame, kind string, endIndex int, rng EditorRange, symbolsByFrame map[string]*EditorSymbol, parentByFrame map[string]string, replayable map[int]bool, activeVars map[string]editorVarScope) bool {
	for i := len(*stack) - 1; i >= 0; i-- {
		if (*stack)[i].Kind != kind {
			continue
		}
		frame := (*stack)[i]
		*stack = append((*stack)[:i], (*stack)[i+1:]...)
		expireActiveVars(activeVars, frame.ID)
		addFoldAndSymbol(analysis, frame, endIndex, rng, symbolsByFrame, parentByFrame, replayable)
		return true
	}
	return false
}

func expireActiveVars(activeVars map[string]editorVarScope, frameID string) {
	for key, scope := range activeVars {
		if scope.FrameID == frameID {
			delete(activeVars, key)
		}
	}
}

func addFoldAndSymbol(analysis *EditorAnalysis, frame editorFrame, endIndex int, endRange EditorRange, symbolsByFrame map[string]*EditorSymbol, parentByFrame map[string]string, replayable map[int]bool) {
	startLine := frame.Range.StartLine
	if endRange.EndLine < startLine {
		return
	}
	rng := EditorRange{StartLine: startLine, StartColumn: 0, EndLine: endRange.EndLine, EndColumn: endRange.EndColumn}
	if rng.EndLine > rng.StartLine {
		analysis.Folds = append(analysis.Folds, EditorFold{Kind: frame.Kind, Range: rng, Collapsed: frame.Name, Depth: frame.Depth})
	}
	symbol := &EditorSymbol{Name: frame.Name, Kind: frame.Kind, Range: rng, Select: EditorRange{StartLine: startLine, EndLine: startLine}, Detail: frame.ID, Source: frame.Source}
	symbolsByFrame[frame.ID] = symbol
	parentByFrame[frame.ID] = frame.ParentID
	canReplay := replayable[frame.Entry]
	analysis.ReplayFrames = append(analysis.ReplayFrames, EditorReplayFrame{FrameID: frame.ID, EntryIndex: frame.Entry, Range: rng, CanReplay: canReplay, Reason: replayFrameReason(canReplay)})
	_ = endIndex
}

func addEntryHoverAndTokens(analysis *EditorAnalysis, entry apexlog.Entry, rng EditorRange, source EditorLocation) {
	hover := "**" + string(entry.Kind) + "**"
	if entry.Data.MethodSymbol != "" {
		hover += " `" + entry.Data.MethodSymbol + "`"
	}
	if entry.Data.VariableName != "" {
		hover += " `" + entry.Data.VariableName + "`"
	}
	if source.File != "" {
		hover += fmt.Sprintf("\n\nSource: `%s:%d`\nConfidence: `%.2f`", source.File, source.Line, source.Confidence)
	}
	analysis.Hovers = append(analysis.Hovers, EditorHover{Range: rng, Markdown: hover})
	if entry.Timestamp != "" {
		analysis.Semantic = append(analysis.Semantic, EditorToken{Range: EditorRange{StartLine: rng.StartLine, EndLine: rng.StartLine, EndColumn: minInt(utf16ColumnLen(entry.Timestamp), rng.EndColumn)}, TokenType: "timestamp"})
	}
	analysis.Semantic = append(analysis.Semantic, EditorToken{Range: rng, TokenType: "event"})
	if entry.Data.MethodSymbol != "" {
		analysis.Semantic = append(analysis.Semantic, EditorToken{Range: rng, TokenType: "method"})
	}
	if entry.Data.VariableName != "" {
		analysis.Semantic = append(analysis.Semantic, EditorToken{Range: rng, TokenType: "variable"})
	}
}

func parentFrame(stack []editorFrame) editorFrame {
	if len(stack) == 0 {
		return editorFrame{}
	}
	return stack[len(stack)-1]
}

func editorDepth(stackLen int) int {
	if stackLen <= 1 {
		return stackLen
	}
	return stackLen - 1
}

func buildEditorSymbolTree(symbolsByFrame map[string]*EditorSymbol, parentByFrame map[string]string) []EditorSymbol {
	ids := make([]string, 0, len(symbolsByFrame))
	for id := range symbolsByFrame {
		ids = append(ids, id)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		left := symbolsByFrame[ids[i]].Range.StartLine
		right := symbolsByFrame[ids[j]].Range.StartLine
		if left != right {
			return left < right
		}
		return ids[i] < ids[j]
	})
	childrenByParent := map[string][]string{}
	for _, id := range ids {
		parentID := parentByFrame[id]
		childrenByParent[parentID] = append(childrenByParent[parentID], id)
	}
	var clone func(string) EditorSymbol
	clone = func(id string) EditorSymbol {
		base := *symbolsByFrame[id]
		base.Children = nil
		for _, childID := range childrenByParent[id] {
			base.Children = append(base.Children, clone(childID))
		}
		return base
	}
	var roots []EditorSymbol
	for _, id := range ids {
		parentID := parentByFrame[id]
		if parentID == "" || symbolsByFrame[parentID] == nil {
			roots = append(roots, clone(id))
			continue
		}
	}
	return roots
}

func findActiveVar(vars map[string]editorVarScope, name string) (editorVarScope, bool) {
	for _, variable := range vars {
		if strings.EqualFold(variable.Name, name) {
			return variable, true
		}
	}
	return editorVarScope{}, false
}

func varKey(frameID, name string) string {
	return strings.ToLower(strings.TrimSpace(frameID)) + "|" + strings.ToLower(strings.TrimSpace(name))
}

func frameID(index int, name string) string {
	return fmt.Sprintf("frame:%d:%s", index, normalizeFrameName(name))
}

func normalizeFrameName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.NewReplacer(" ", "-", "(", "", ")", "", ".", "-").Replace(name)
	return name
}

func displayCodeUnit(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Code Unit"
	}
	return displayMethodSymbol(value)
}

func displayMethodSymbol(value string) string {
	value = strings.TrimSpace(value)
	if open := strings.Index(value, "("); open >= 0 {
		value = strings.TrimSpace(value[:open])
	}
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return value
	}
	return strings.TrimSpace(parts[len(parts)-2]) + "." + strings.TrimSpace(parts[len(parts)-1])
}

func parseMethodLikeSymbol(value string) (namespace, typeName, methodName string) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "Class.")
	if open := strings.Index(value, "("); open >= 0 {
		value = strings.TrimSpace(value[:open])
	}
	return parseCodeUnitSymbol(value)
}

func replayableEntryIndexes(annotated AnnotatedLog, index SourceIndex, minConfidence float64) map[int]bool {
	out := map[int]bool{}
	for i := range annotated.Log.Entries {
		entryPoint, ok := inferEntryPointAtEntry(annotated, i)
		if ok && replayEntryPointLocation(index, entryPoint, minConfidence).File != "" {
			out[i] = true
		}
	}
	return out
}

func replayFrameReason(canReplay bool) string {
	if !canReplay {
		return "No source-backed entry point."
	}
	return ""
}

type editorSchemaIndex struct {
	root    string
	objects map[string]string
	fields  map[string]string
}

func buildEditorSchemaIndex(projectRoot string, objectFiles, fieldFiles []string) editorSchemaIndex {
	idx := editorSchemaIndex{root: projectRoot, objects: map[string]string{}, fields: map[string]string{}}
	for _, path := range objectFiles {
		objectName := objectNameFromObjectPath(path)
		if objectName != "" {
			idx.objects[strings.ToLower(objectName)] = filepath.Clean(path)
		}
	}
	for _, path := range fieldFiles {
		objectName, fieldName := objectFieldFromFieldPath(path)
		if objectName != "" && fieldName != "" {
			idx.fields[strings.ToLower(objectName)+"."+strings.ToLower(fieldName)] = filepath.Clean(path)
		}
	}
	return idx
}

func (idx editorSchemaIndex) objectLocation(objectName string) (EditorLocation, bool) {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return EditorLocation{}, false
	}
	path := idx.objects[strings.ToLower(objectName)]
	if path == "" && idx.root != "" {
		path = filepath.Join(idx.root, "force-app", "main", "default", "objects", objectName, objectName+".object-meta.xml")
	}
	if _, err := os.Stat(path); err != nil {
		return EditorLocation{}, false
	}
	return EditorLocation{File: filepath.Clean(path), Line: 1, Symbol: objectName, Reason: "schema object", Confidence: 1}, true
}

func (idx editorSchemaIndex) fieldLocation(objectName, fieldName string) (EditorLocation, bool) {
	objectName = strings.TrimSpace(objectName)
	fieldName = strings.TrimSpace(fieldName)
	if objectName == "" || fieldName == "" {
		return EditorLocation{}, false
	}
	path := idx.fields[strings.ToLower(objectName)+"."+strings.ToLower(fieldName)]
	if path == "" && idx.root != "" {
		path = filepath.Join(idx.root, "force-app", "main", "default", "objects", objectName, "fields", fieldName+".field-meta.xml")
	}
	if _, err := os.Stat(path); err != nil {
		return EditorLocation{}, false
	}
	return EditorLocation{File: filepath.Clean(path), Line: 1, Symbol: objectName + "." + fieldName, Reason: "schema field", Confidence: 1}, true
}

func objectNameFromObjectPath(path string) string {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	for _, suffix := range []string{".object-meta.xml", ".object"} {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSpace(base[:len(base)-len(suffix)])
		}
	}
	return strings.TrimSpace(filepath.Base(filepath.Dir(path)))
}

func objectFieldFromFieldPath(path string) (string, string) {
	fieldName := filepath.Base(path)
	if strings.HasSuffix(strings.ToLower(fieldName), ".field-meta.xml") {
		fieldName = fieldName[:len(fieldName)-len(".field-meta.xml")]
	}
	objectName := filepath.Base(filepath.Dir(filepath.Dir(path)))
	return strings.TrimSpace(objectName), strings.TrimSpace(fieldName)
}

var selectFieldsRe = regexp.MustCompile(`(?is)\bselect\s+(.+?)\s+from\s+([a-zA-Z_][a-zA-Z0-9_]*)`)

func parseFromObjectPreserveCase(query string) string {
	if parsed, err := soql.Parse(query); err == nil && strings.TrimSpace(parsed.Object) != "" {
		return strings.TrimSpace(parsed.Object)
	}
	match := selectFieldsRe.FindStringSubmatch(query)
	if len(match) < 3 {
		return ""
	}
	return strings.TrimSpace(match[2])
}

func parseSelectFields(query string) []string {
	if parsed, err := soql.Parse(query); err == nil {
		return normalizeSOQLFieldNames(parsed.Fields)
	}
	match := selectFieldsRe.FindStringSubmatch(query)
	if len(match) < 2 {
		return nil
	}
	return normalizeSOQLFieldNames(strings.Split(match[1], ","))
}

func normalizeSOQLFieldNames(fields []string) []string {
	var out []string
	for _, raw := range fields {
		field := strings.TrimSpace(raw)
		if field == "" || strings.Contains(field, "(") {
			continue
		}
		field = strings.Fields(field)[0]
		if strings.Contains(field, ".") {
			parts := strings.Split(field, ".")
			field = parts[len(parts)-1]
		}
		out = append(out, field)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func utf16ColumnLen(value string) int {
	return len(utf16.Encode([]rune(value)))
}
