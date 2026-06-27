package debuglog

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestBuildEditorAnalysisFoldsNestedMethods(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|EXECUTION_STARTED",
		"00:00:00.002 (2000000)|CODE_UNIT_STARTED|[EXTERNAL]|ns.TestProcessor.run",
		"00:00:00.003 (3000000)|METHOD_ENTRY|[2]|01p000000000001|ns.TestProcessor.run()",
		"00:00:00.004 (4000000)|METHOD_ENTRY|[10]|01p000000000001|ns.TestProcessor.fail()",
		"00:00:00.005 (5000000)|METHOD_EXIT|[10]|ns.TestProcessor.fail()",
		"00:00:00.006 (6000000)|METHOD_EXIT|[2]|ns.TestProcessor.run()",
		"00:00:00.007 (7000000)|CODE_UNIT_FINISHED|ns.TestProcessor.run",
		"00:00:00.008 (8000000)|EXECUTION_FINISHED",
		"",
	}, "\n"))

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())

	if fold := findEditorFold(analysis, "codeUnit"); fold.Range.StartLine != 1 || fold.Range.EndLine != 6 {
		t.Fatalf("codeUnit fold = %#v, want lines 1-6", fold)
	}
	if fold := findEditorFoldByText(analysis, "method", "fail"); fold.Range.StartLine != 3 || fold.Range.EndLine != 4 || fold.Depth != 2 {
		t.Fatalf("nested method fold = %#v, want lines 3-4 depth 2", fold)
	}
	if symbol := findEditorSymbol(analysis.Symbols, "TestProcessor.run"); symbol.Name == "" || len(symbol.Children) == 0 {
		t.Fatalf("run symbol = %#v, want children", symbol)
	}
}

func TestBuildEditorAnalysisUsesDocumentLinesForFoldsAfterHeader(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"64.0 APEX_CODE,FINEST;APEX_PROFILING,FINE;DB,FINE;SYSTEM,DEBUG",
		"00:00:00.001 (1000000)|CODE_UNIT_STARTED|[EXTERNAL]|ns.TestProcessor.run",
		"00:00:00.002 (2000000)|METHOD_ENTRY|[2]|01p000000000001|ns.TestProcessor.run()",
		"00:00:00.003 (3000000)|METHOD_EXIT|[2]|ns.TestProcessor.run()",
		"00:00:00.004 (4000000)|CODE_UNIT_FINISHED|ns.TestProcessor.run",
		"",
	}, "\n"))

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())

	if fold := findEditorFold(analysis, "codeUnit"); fold.Range.StartLine != 1 || fold.Range.EndLine != 4 {
		t.Fatalf("codeUnit fold = %#v, want document lines 1-4", fold)
	}
	if frame := findReplayFrame(analysis, 0); frame.Range.StartLine != 1 || frame.EntryIndex != 0 {
		t.Fatalf("replay frame = %#v, want range line 1 and entry index 0", frame)
	}
}

func TestBuildEditorAnalysisLinksMethodEntriesToSource(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, "00:00:00.001 (1000000)|METHOD_ENTRY|[2]|01p000000000001|ns.TestProcessor.run()\n")

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())
	link := findEditorLink(analysis, "method")

	if !strings.HasSuffix(link.Target.File, filepath.Join("classes", "TestProcessor.cls")) {
		t.Fatalf("method target = %#v, want TestProcessor.cls", link.Target)
	}
	if link.Target.Line != 2 {
		t.Fatalf("method line = %d, want 2", link.Target.Line)
	}
}

func TestBuildEditorAnalysisLinksVariablesToSourceDeclaration(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|CODE_UNIT_STARTED|[EXTERNAL]|ns.TestProcessor.run",
		"00:00:00.002 (2000000)|METHOD_ENTRY|[2]|01p000000000001|ns.TestProcessor.run()",
		"00:00:00.003 (3000000)|VARIABLE_SCOPE_BEGIN|[4]|a|Account|false|false",
		"00:00:00.004 (4000000)|VARIABLE_ASSIGNMENT|[4]|a|{\"Name\":\"Acme\"}",
		"00:00:00.005 (5000000)|METHOD_EXIT|[2]|ns.TestProcessor.run()",
		"00:00:00.006 (6000000)|CODE_UNIT_FINISHED|ns.TestProcessor.run",
		"",
	}, "\n"))

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())
	variable := findEditorVariable(analysis, "a")

	if !strings.HasSuffix(variable.SourceDef.File, filepath.Join("classes", "TestProcessor.cls")) {
		t.Fatalf("source definition = %#v, want TestProcessor.cls", variable.SourceDef)
	}
	if variable.SourceDef.Line != 4 {
		t.Fatalf("source definition line = %d, want 4", variable.SourceDef.Line)
	}
	if link := findEditorLink(analysis, "variableSource"); link.Target.File == "" {
		t.Fatalf("missing variable source link: %#v", analysis.Links)
	}
}

func TestBuildEditorAnalysisLinksVariablesToSourceParameter(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|METHOD_ENTRY|[14]|01p000000000001|ns.TestProcessor.withParam(String)",
		"00:00:00.002 (2000000)|VARIABLE_SCOPE_BEGIN|[14]|inputName|String|false|false",
		"00:00:00.003 (3000000)|VARIABLE_ASSIGNMENT|[14]|inputName|\"Acme\"",
		"00:00:00.004 (4000000)|METHOD_EXIT|[14]|ns.TestProcessor.withParam(String)",
		"",
	}, "\n"))

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())
	variable := findEditorVariable(analysis, "inputName")

	if !strings.HasSuffix(variable.SourceDef.File, filepath.Join("classes", "TestProcessor.cls")) {
		t.Fatalf("source definition = %#v, want TestProcessor.cls", variable.SourceDef)
	}
	if variable.SourceDef.Line != 14 {
		t.Fatalf("source definition line = %d, want 14", variable.SourceDef.Line)
	}
	if frame := findReplayFrame(analysis, 0); frame.CanReplay {
		t.Fatalf("parameterized method should not be replayable: %#v", frame)
	}
}

func TestBuildEditorAnalysisFallsBackToLogVariableScope(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|METHOD_ENTRY|[2]|01p000000000001|ns.TestProcessor.run()",
		"00:00:00.002 (2000000)|VARIABLE_SCOPE_BEGIN|[4]|missing|String|false|false",
		"00:00:00.003 (3000000)|VARIABLE_ASSIGNMENT|[4]|missing|\"value\"",
		"00:00:00.004 (4000000)|METHOD_EXIT|[2]|ns.TestProcessor.run()",
		"",
	}, "\n"))

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())
	variable := findEditorVariable(analysis, "missing")

	if variable.SourceDef.File != "" {
		t.Fatalf("source definition = %#v, want empty", variable.SourceDef)
	}
	if variable.LogDef.Line != 2 {
		t.Fatalf("log definition line = %d, want 2", variable.LogDef.Line)
	}
	if link := findEditorLink(analysis, "variableLog"); link.Target.Line != 2 {
		t.Fatalf("variable log link = %#v, want line 2", link)
	}
}

func TestBuildEditorAnalysisExpiresVariablesWhenFrameCloses(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|METHOD_ENTRY|[2]|01p000000000001|ns.TestProcessor.run()",
		"00:00:00.002 (2000000)|VARIABLE_SCOPE_BEGIN|[4]|a|Account|false|false",
		"00:00:00.003 (3000000)|METHOD_EXIT|[2]|ns.TestProcessor.run()",
		"00:00:00.004 (4000000)|METHOD_ENTRY|[10]|01p000000000001|ns.TestProcessor.fail()",
		"00:00:00.005 (5000000)|VARIABLE_ASSIGNMENT|[10]|a|{\"Name\":\"Late\"}",
		"00:00:00.006 (6000000)|METHOD_EXIT|[10]|ns.TestProcessor.fail()",
		"",
	}, "\n"))

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())

	if diagnostic := findEditorDiagnostic(analysis, "apexlog.unscopedVariable"); diagnostic.Message == "" {
		t.Fatalf("missing unscoped variable diagnostic after frame close: %#v", analysis.Diagnostics)
	}
}

func TestBuildEditorAnalysisAppliesMinConfidence(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|METHOD_ENTRY|[2]|01p000000000001|ns.TestProcessor.run()",
		"00:00:00.002 (2000000)|METHOD_EXIT|[2]|ns.TestProcessor.run()",
		"",
	}, "\n"))
	opts := testEditorOptions()
	opts.MinConfidence = 0.99

	analysis := BuildEditorAnalysis(log, index, opts)

	if analysis.Coverage.ResolvedSources != 0 {
		t.Fatalf("resolved sources = %d, want 0", analysis.Coverage.ResolvedSources)
	}
	if link := findEditorLink(analysis, "method"); link.Target.File != "" {
		t.Fatalf("method link = %#v, want none above threshold", link)
	}
	if frame := findReplayFrame(analysis, 0); frame.CanReplay {
		t.Fatalf("replay frame should not pass confidence threshold: %#v", frame)
	}
}

func TestBuildEditorAnalysisDoesNotReplayMissingSource(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|METHOD_ENTRY|[2]|01p000000000001|ns.MissingClass.run()",
		"00:00:00.002 (2000000)|METHOD_EXIT|[2]|ns.MissingClass.run()",
		"",
	}, "\n"))

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())

	if frame := findReplayFrame(analysis, 0); frame.CanReplay {
		t.Fatalf("missing source should not be replayable: %#v", frame)
	}
}

func TestBuildEditorAnalysisUsesUTF16Columns(t *testing.T) {
	log := mustParseEditorLog(t, "00:00:00.001 (1000000)|USER_DEBUG|[1]|DEBUG|hello 😀\n")

	analysis := BuildEditorAnalysis(log, typesys.Index{}, testEditorOptions())
	got := analysis.Entries[0].Range.EndColumn
	want := len(utf16.Encode([]rune(log.Entries[0].Raw)))

	if got != want {
		t.Fatalf("end column = %d, want UTF-16 length %d", got, want)
	}
}

func TestBuildEditorAnalysisLinksSOQLObjectAndFields(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, "00:00:00.001 (1000000)|SOQL_EXECUTE_BEGIN|[6]|Aggregations:0|SELECT Id, Name FROM Account WHERE Name = 'Acme'\n")

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())
	objectLink := findEditorLink(analysis, "schemaObject")
	fieldLink := findEditorLink(analysis, "schemaField")

	if !strings.HasSuffix(objectLink.Target.File, filepath.Join("objects", "Account", "Account.object-meta.xml")) {
		t.Fatalf("object target = %#v", objectLink.Target)
	}
	if !strings.HasSuffix(fieldLink.Target.File, filepath.Join("objects", "Account", "fields", "Name.field-meta.xml")) {
		t.Fatalf("field target = %#v", fieldLink.Target)
	}
}

func TestBuildEditorAnalysisDoesNotMarkSOQLOrDMLFramesReplayable(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|CODE_UNIT_STARTED|[EXTERNAL]|ns.TestProcessor.run",
		"00:00:00.002 (2000000)|SOQL_EXECUTE_BEGIN|[6]|Aggregations:0|SELECT Id, Name FROM Account WHERE Name = 'Acme'",
		"00:00:00.003 (3000000)|SOQL_EXECUTE_END|[6]|Rows:1",
		"00:00:00.004 (4000000)|DML_BEGIN|[5]|Op:Insert|Type:Account|Rows:1",
		"00:00:00.005 (5000000)|DML_END|[5]",
		"00:00:00.006 (6000000)|CODE_UNIT_FINISHED|ns.TestProcessor.run",
		"",
	}, "\n"))

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())

	for _, frame := range analysis.ReplayFrames {
		if frame.EntryIndex == 1 || frame.EntryIndex == 3 {
			if frame.CanReplay {
				t.Fatalf("frame %d should not be replayable: %#v", frame.EntryIndex, frame)
			}
		}
	}
}

func TestBuildEditorAnalysisReportsUnmatchedExit(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, "00:00:00.001 (1000000)|METHOD_EXIT|[2]|ns.TestProcessor.run()\n")

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())
	diagnostic := findEditorDiagnostic(analysis, "apexlog.unmatchedExit")

	if diagnostic.Message == "" {
		t.Fatalf("missing unmatched-exit diagnostic: %#v", analysis.Diagnostics)
	}
}

func TestBuildEditorAnalysisKeepsPartialResultsOnUnknownEvents(t *testing.T) {
	index := mustLoadDebugIndex(t, filepath.Join("testdata", "project"))
	log := mustParseEditorLog(t, strings.Join([]string{
		"00:00:00.001 (1000000)|METHOD_ENTRY|[2]|01p000000000001|ns.TestProcessor.run()",
		"00:00:00.002 (2000000)|UNKNOWN_EVENT|payload",
		"00:00:00.003 (3000000)|METHOD_EXIT|[2]|ns.TestProcessor.run()",
		"",
	}, "\n"))

	analysis := BuildEditorAnalysis(log, index, testEditorOptions())

	if len(analysis.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(analysis.Entries))
	}
	if findEditorFold(analysis, "method").Range.EndLine != 2 {
		t.Fatalf("folds = %#v, want method fold through unknown event", analysis.Folds)
	}
}

func testEditorOptions() EditorOptions {
	return EditorOptions{
		LogFile:       "apex.log",
		ProjectRoot:   filepath.Join("testdata", "project"),
		MaxCandidates: 5,
		MinConfidence: 0.35,
		Now: func() time.Time {
			return time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
		},
	}
}

func mustParseEditorLog(t *testing.T, input string) apexlog.Log {
	t.Helper()
	log, err := apexlog.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func findEditorFold(analysis EditorAnalysis, kind string) EditorFold {
	for _, fold := range analysis.Folds {
		if fold.Kind == kind {
			return fold
		}
	}
	return EditorFold{}
}

func findEditorFoldByText(analysis EditorAnalysis, kind, text string) EditorFold {
	for _, fold := range analysis.Folds {
		if fold.Kind == kind && strings.Contains(fold.Collapsed, text) {
			return fold
		}
	}
	return EditorFold{}
}

func findEditorSymbol(symbols []EditorSymbol, name string) EditorSymbol {
	for _, symbol := range symbols {
		if symbol.Name == name {
			return symbol
		}
		if child := findEditorSymbol(symbol.Children, name); child.Name != "" {
			return child
		}
	}
	return EditorSymbol{}
}

func findEditorLink(analysis EditorAnalysis, kind string) EditorLink {
	for _, link := range analysis.Links {
		if link.Kind == kind {
			return link
		}
	}
	return EditorLink{}
}

func findEditorVariable(analysis EditorAnalysis, name string) EditorVariable {
	for _, variable := range analysis.Variables {
		if variable.Name == name {
			return variable
		}
	}
	return EditorVariable{}
}

func findEditorDiagnostic(analysis EditorAnalysis, code string) EditorDiagnostic {
	for _, diagnostic := range analysis.Diagnostics {
		if diagnostic.Code == code {
			return diagnostic
		}
	}
	return EditorDiagnostic{}
}

func findReplayFrame(analysis EditorAnalysis, entryIndex int) EditorReplayFrame {
	for _, frame := range analysis.ReplayFrames {
		if frame.EntryIndex == entryIndex {
			return frame
		}
	}
	return EditorReplayFrame{}
}
