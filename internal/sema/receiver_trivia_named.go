package sema

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/typesys"
)

type receiverReplacement struct {
	diagnostic diagnostic.Diagnostic
	file       string
	start, end int
	field      bool
}

type receiverScopeKey struct {
	typ, name  string
	start, end int
}

// receiverNamedDiagnostics scans each source unit once, after the ordinary
// semantic model is available. The resulting slices are local to this analysis.
func (a *Analyzer) receiverNamedDiagnostics(index typesys.Index, model *semaTypeMemberView) []receiverReplacement {
	var out []receiverReplacement
	var duplicateTypes map[string]int
	type sourceUnit struct{ file, source, route, api, namespace, remaps string }
	seen := map[sourceUnit]bool{}
	scan := func(file, source, route, api, namespace string, remaps []namespaceremap.Rule, owner typesys.TypeSymbol, trigger *typesys.TriggerSymbol) {
		major, _ := apexversion.Major(api)
		if major < 62 || major > 67 {
			return
		}
		key := sourceUnit{file: file, source: source, route: route, api: api, namespace: namespace, remaps: namespaceremap.Fingerprint(remaps)}
		if seen[key] {
			return
		}
		seen[key] = true
		items := scanReceiverTrivia(source)
		first := len(out)
		var (
			scoped    bool
			scopeKey  receiverScopeKey
			scopes    semaScopeModel
			bodyModel *semaTypeMemberView
			offset    int
		)
		for _, item := range items {
			if item.kind == receiverInvalidControl {
				continue
			}
			pos := item.tokens[item.dot].start
			typ := owner
			if trigger == nil {
				typ = typesys.TypeSymbol{}
				for _, candidate := range index.Types {
					if candidate.File == file && strings.EqualFold(candidate.Namespace, namespace) && candidate.EffectiveAPIVersion == api && namespaceremap.Fingerprint(candidate.SourceNamespaceRemaps) == key.remaps && candidate.Range.Start.Offset <= pos && pos < candidate.Range.End.Offset && (typ.File == "" || candidate.Range.End.Offset-candidate.Range.Start.Offset < typ.Range.End.Offset-typ.Range.Start.Offset) {
						typ = candidate
					}
				}
				if typ.File == "" {
					continue
				}
			}
			var member typesys.MemberSymbol
			if trigger != nil {
				typ, member = semaTriggerBodyDeclaration(*trigger)
			} else {
				for _, candidate := range typ.Members {
					if candidate.Range.Start.Offset <= pos && pos < candidate.Range.End.Offset {
						if member.Name == "" || candidate.Range.End.Offset-candidate.Range.Start.Offset < member.Range.End.Offset-member.Range.Start.Offset {
							member = candidate
						}
					}
				}
			}
			fieldType := member.Kind == apexast.DeclarationField && !item.declarationHasEquals
			depth, parens := item.depth, item.parens
			if trigger == nil && member.Name == "" {
				fieldType = depth == 1 && parens == 0 && !item.declarationHasEquals
			}
			var d *diagnostic.Diagnostic
			inheritance := false
			if trigger == nil && depth == 0 {
				for i := 0; i < item.dot; i++ {
					if strings.EqualFold(item.tokens[i].text, "extends") || strings.EqualFold(item.tokens[i].text, "implements") {
						inheritance = true
						break
					}
				}
			}
			if trigger == nil && item.kind == receiverFreeName && (depth == 0 && !inheritance || depth == 1 && parens > 0 && !item.declarationHasEquals && (item.dot+2 >= len(item.tokens) || !item.tokens[item.dot+2].word)) {
				continue
			}
			if inheritance && item.dot+1 < len(item.tokens) {
				token := item.tokens[item.dot+1]
				msg := "Extra '{', at '" + token.text + "'."
				if item.kind == receiverFreeName {
					token = item.tokens[item.dot]
					msg = "Expecting '{' but was: '.'"
				}
				d = &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: msg, NativeMessage: msg, File: file, Range: receiverTriviaRange(source, token.start, token.end)}
			} else if trigger == nil && depth == 1 && parens > 0 && !item.declarationHasEquals && item.dot+2 < len(item.tokens) && item.tokens[item.dot+2].word {
				token := item.tokens[item.dot+2]
				msg := "Extra ')', at '" + token.text + "'."
				if item.kind == receiverFreeName {
					token = item.tokens[item.dot]
					msg = "Unexpected token '.'."
				}
				d = &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: msg, NativeMessage: msg, File: file, Range: receiverTriviaRange(source, token.start, token.end)}
			} else if fieldType && item.dot+1 < len(item.tokens) && item.dot+2 < len(item.tokens) && item.tokens[item.dot+2].word {
				token := item.tokens[item.dot+1]
				if item.kind == receiverFreeName {
					token = item.tokens[item.dot]
				}
				msg := "Unexpected token '" + token.text + "'."
				d = &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: msg, NativeMessage: msg, File: file, Range: receiverTriviaRange(source, token.start, token.end)}
			} else if item.kind == receiverBoundBreak {
				d = receiverBoundDiagnostic(source, file, route, item)
			} else if route == "trigger" && item.dot > 0 && strings.EqualFold(item.tokens[item.dot-1].text, "Trigger") && item.dot+1 < len(item.tokens) && strings.EqualFold(item.tokens[item.dot+1].text, "old") && depth > 1 {
				token := item.tokens[item.dot-1]
				msg := "Expecting ')' but was: '" + token.text + "'"
				d = &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: msg, NativeMessage: msg, File: file, Range: receiverTriviaRange(source, token.start, token.end)}
			} else if route == "trigger" && item.dot > 0 && strings.EqualFold(item.tokens[item.dot-1].text, "Trigger") && item.dot+1 < len(item.tokens) && strings.EqualFold(item.tokens[item.dot+1].text, "new") {
				candidate := item
				candidate.kind = receiverBoundBreak
				d = receiverBoundDiagnostic(source, file, route, candidate)
			} else {
				// Scopes depend only on the enclosing member; a body with many
				// detached dots collects them once.
				key := receiverScopeKey{typ: semaTypeSymbolKey(typ), name: member.Name, start: member.Range.Start.Offset, end: member.Range.End.Offset}
				if !scoped || key != scopeKey {
					bodyModel = model
					if trigger != nil {
						bodyModel = semaModelWithTriggerScope(model, *trigger)
					} else {
						if duplicateTypes == nil {
							duplicateTypes = semaDuplicateTypeKeys(index)
						}
						if duplicateTypes[semaTypeSymbolKey(typ)] > 1 || semaSourceUnitHasDuplicate(model, typ) {
							bodyModel = semaModelWithCurrentType(model, typ)
						}
					}
					base := semaBodyBaseScope(typ, member, bodyModel)
					scopes = semaScopeModel{base: base, canonical: a.canonicalNames}
					offset = 0
					if member.BodyRange != nil {
						if body, bodyOffset, ok := semaBodyFromRange(source, member.BodyRange); ok {
							offset = bodyOffset
							scopes, _ = a.collectBodyScopes(typ, member, body, bodyOffset, source, base, bodyModel)
						}
					}
					scoped, scopeKey = true, key
				}
				d = receiverFreeDiagnosticAt(source, namespace, item, scopes, bodyModel, offset)
				if d != nil {
					d.File = file
				}
			}
			if d == nil {
				continue
			}
			start, end := receiverStatementSpan(item.tokens, item.dot, len(source))
			out = append(out, receiverReplacement{diagnostic: *d, file: file, start: start, end: end, field: member.Kind == apexast.DeclarationField})
		}
		// Native class field diagnostics follow method body diagnostics.
		sort.SliceStable(out[first:], func(i, j int) bool { return !out[first+i].field && out[first+j].field })
	}
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) || typ.File == "" {
			continue
		}
		source, ok := a.sources.normalizedForType(typ)
		if ok {
			scan(typ.File, source, "class", typ.EffectiveAPIVersion, typ.Namespace, typ.SourceNamespaceRemaps, typ, nil)
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.Dependency || trigger.File == "" {
			continue
		}
		source, ok := a.sources.normalizedForTrigger(trigger)
		if ok {
			scan(trigger.File, source, "trigger", trigger.EffectiveAPIVersion, trigger.Namespace, trigger.SourceNamespaceRemaps, typesys.TypeSymbol{}, &trigger)
		}
	}
	return out
}

func receiverStatementSpan(tokens []receiverTriviaToken, dot, sourceLen int) (int, int) {
	root := dot - 1
	for root >= 2 && tokens[root-1].text == "." && tokens[root-2].word && tokens[root-2].end == tokens[root-1].start && tokens[root-1].end == tokens[root].start {
		root -= 2
	}
	end := sourceLen
	if dot+1 < len(tokens) {
		end = tokens[dot+1].end
	}
	return tokens[root].start, end
}

func mergeReceiverNamedDiagnostics(existing []diagnostic.Diagnostic, replacements []receiverReplacement) []diagnostic.Diagnostic {
	if len(replacements) == 0 {
		return existing
	}
	out := make([]diagnostic.Diagnostic, 0, len(existing)+len(replacements))
	for _, d := range existing {
		replaced := false
		if d.Range != nil && d.Severity == diagnostic.Error {
			for _, r := range replacements {
				if d.File == r.file && d.Range.Start.Offset >= r.start && d.Range.Start.Offset < r.end {
					replaced = true
					break
				}
			}
		}
		if !replaced {
			out = append(out, d)
		}
	}
	for _, r := range replacements {
		out = append(out, r.diagnostic)
	}
	return out
}
