package sema

import (
	"strconv"
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
	apiVersion, err := apexversion.PreserveSource(apiVersion)
	if err != nil {
		item := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_VERSION", Message: err.Error(), Range: &rng}
		return Result{Project: index.Project, Summary: Summary{Diagnostics: 1}, Diagnostics: []diagnostic.Diagnostic{item}}
	}
	if diagnostics := apexast.SharingDeclarationDiagnostics("", source); len(diagnostics) != 0 {
		return Result{Project: index.Project, Summary: Summary{Diagnostics: len(diagnostics)}, Diagnostics: diagnostics}
	}
	if _, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: apiVersion, PrefixStatementCandidates: true}); err != nil {
		return anonymousCompileFailure(index, source, rng, err)
	}
	setup := anonymousSetupFor(index, apiVersion)
	analyzer := setup.analyzer()
	index = setup.index
	model := setup.typeMembers.view()
	typ, member := anonymousExecuteSymbols(apiVersion, rng)
	queryChecker := setup.newQueryChecker()
	queryChecker.apiVersion, _ = apexversion.Major(apiVersion)
	diagnostics := queryChecker.checkFile(typ.File, source)
	bodyDiagnostics, compiled := analyzer.checkBodyIRWithScopeOptions(typ, member, source, 0, source, map[string]string{}, model, buildConstructability(index), true, nil)
	if !compiled {
		// A body that does not compile with a ++/-- statement only the VM
		// prefix fallback lowers keeps the name-path compile error.
		if _, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: apiVersion}); err != nil {
			return anonymousCompileFailure(index, source, rng, err)
		}
	}
	diagnostics = append(diagnostics, bodyDiagnostics...)
	if !compiled {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_ANONYMOUS_PARSE", Message: "anonymous Apex could not be compiled", Range: &rng})
	}
	return Result{Project: index.Project, Summary: Summary{Diagnostics: len(diagnostics)}, Diagnostics: NativeLifecycleDiagnostics(index, diagnostics)}
}

func anonymousExecuteSymbols(apiVersion string, rng diagnostic.Range) (typesys.TypeSymbol, typesys.MemberSymbol) {
	typ := typesys.TypeSymbol{Kind: apexast.DeclarationClass, Name: "__GladeAnonymous", LocalName: "__GladeAnonymous", EffectiveAPIVersion: apiVersion, Range: rng}
	member := typesys.MemberSymbol{Kind: apexast.DeclarationMethod, Name: "execute", Type: "void", Modifiers: []string{"static"}, HasBody: true, Range: rng}
	return typ, member
}

// ApprovedAnonymousPrefixStatements returns the byte offsets of the ++/--
// statements an execute-anonymous body may lower through
// vm.CompileOptions.ApprovedPrefixStatements: the prefix candidates a captured
// anonymous rule covers, decided as AnalyzeAnonymous decides them. It returns
// nil when the name path lowers the body or the body keeps the name-path result.
func ApprovedAnonymousPrefixStatements(index typesys.Index, source, apiVersion string) map[int]bool {
	apiVersion, err := apexversion.PreserveSource(apiVersion)
	if err != nil {
		return nil
	}
	if !semaPrefixCandidatesLowerBody(source) {
		return nil
	}
	setup := anonymousSetupFor(index, apiVersion)
	rng := diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1, Offset: 0}, End: diagnostic.Position{Line: 1, Column: 1 + len(source), Offset: len(source)}}
	typ, member := anonymousExecuteSymbols(apiVersion, rng)
	approved := map[int]bool{}
	if _, compiled := setup.analyzer().checkBodyIRWithScopeOptions(typ, member, source, 0, source, map[string]string{}, setup.typeMembers.view(), buildConstructability(setup.index), true, approved); !compiled || len(approved) == 0 {
		return nil
	}
	return approved
}

func anonymousCompileFailure(index typesys.Index, source string, rng diagnostic.Range, err error) Result {
	item := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_ANONYMOUS_PARSE", Message: err.Error(), Range: &rng}
	if syntax, ok := err.(*vm.ApexSyntaxError); ok && syntax.NativeMessage != "" {
		item.NativeMessage = syntax.NativeMessage
		item.Range = semaRange(source, syntax.Offset, syntax.Offset+1)
	}
	if syntax, ok := err.(*vm.ApexSyntaxError); ok && syntax.Offset >= 0 && syntax.Offset < len(source) && (source[syntax.Offset] == '@' || strings.HasPrefix(source[syntax.Offset:], "webservice")) {
		item.NativeMessage = syntax.Message
		item.Range = semaRange(source, syntax.Offset, syntax.Offset+1)
	}
	// Q003-Q004: preserve the measured reserved-name text and occurrence
	// line rather than reporting the synthetic method's first line.
	if nameAndOffset, reserved := strings.CutPrefix(err.Error(), "identifier name is reserved: "); reserved {
		if name, offset, ok := strings.Cut(nameAndOffset, " at byte "); ok {
			if pos, parseErr := strconv.Atoi(offset); parseErr == nil {
				item.NativeMessage = "Identifier name is reserved: " + name
				item.Range = semaRange(source, pos, pos+len(name))
			}
		}
	}
	return Result{Project: index.Project, Summary: Summary{Diagnostics: 1}, Diagnostics: []diagnostic.Diagnostic{item}}
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
