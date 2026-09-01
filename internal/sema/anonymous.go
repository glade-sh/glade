package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// AnalyzeAnonymous applies the project semantic model to an execute-anonymous
// body. The synthetic method keeps diagnostics in the caller's original source
// offsets, so consumers do not need to compensate for a wrapper class.
func AnalyzeAnonymous(index typesys.Index, source, apiVersion string) Result {
	rng := diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1, Offset: 0}, End: diagnostic.Position{Line: 1, Column: 1 + len(source), Offset: len(source)}}
	apiVersion, err := apexversion.ResolveSource(apiVersion)
	if err != nil {
		item := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_VERSION", Message: err.Error(), Range: &rng}
		return Result{Project: index.Project, Summary: Summary{Diagnostics: 1}, Diagnostics: []diagnostic.Diagnostic{item}}
	}
	if _, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: apiVersion}); err != nil {
		item := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_ANONYMOUS_PARSE", Message: err.Error(), Range: &rng}
		return Result{Project: index.Project, Summary: Summary{Diagnostics: 1}, Diagnostics: []diagnostic.Diagnostic{item}}
	}
	analyzer := NewAnalyzer()
	analyzer.prepareAnalysisContext(index, AnalyzeOptions{}, nil)
	index = prepareAnalysisIndexWithSources(index, analyzer.sources)
	analyzer.prepareAnalysisModel(index)
	state := buildSemaTypeMemberState(index, nil, analyzer.sources)
	model := state.view()
	typ := typesys.TypeSymbol{Kind: apexast.DeclarationClass, Name: "__GladeAnonymous", LocalName: "__GladeAnonymous", EffectiveAPIVersion: apiVersion, Range: rng}
	member := typesys.MemberSymbol{Kind: apexast.DeclarationMethod, Name: "execute", Type: "void", Modifiers: []string{"static"}, HasBody: true, Range: rng}
	queryChecker := newQuerySemanticsChecker(index, analyzer.queryDeclaredObjects...)
	queryChecker.apiVersion, _ = apexversion.Major(apiVersion)
	diagnostics := queryChecker.checkFile(typ.File, source)
	bodyDiagnostics, compiled := analyzer.checkBodyIRWithCompileStatus(typ, member, source, 0, source, map[string]string{}, model, map[string]typesys.TypeSymbol{})
	diagnostics = append(diagnostics, bodyDiagnostics...)
	if !compiled {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_ANONYMOUS_PARSE", Message: "anonymous Apex could not be compiled", Range: &rng})
	}
	return Result{Project: index.Project, Summary: Summary{Diagnostics: len(diagnostics)}, Diagnostics: diagnostics}
}

func anonymousDiagnosticMessage(result Result) string {
	if len(result.Diagnostics) == 0 {
		return ""
	}
	var messages []string
	for _, item := range result.Diagnostics {
		messages = append(messages, item.Message)
	}
	return strings.Join(messages, "; ")
}
