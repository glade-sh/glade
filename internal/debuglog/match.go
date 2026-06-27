package debuglog

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/typesys"
)

// Annotate matches log entries to likely Apex source locations.
func Annotate(log apexlog.Log, index typesys.Index, maxCandidates int) (AnnotatedLog, error) {
	sourceIndex := BuildSourceIndex(index)
	return annotateWithSourceIndex(log, sourceIndex, maxCandidates)
}

func annotateWithSourceIndex(log apexlog.Log, sourceIndex SourceIndex, maxCandidates int) (AnnotatedLog, error) {
	if maxCandidates <= 0 {
		maxCandidates = 5
	}

	out := AnnotatedLog{
		Log:     log,
		Entries: make([]AnnotatedEntry, 0, len(log.Entries)),
	}

	currentCodeUnitMethods := []sourceMethod{}
	for _, entry := range log.Entries {
		if entry.Kind == apexlog.EntryCodeUnitStarted {
			currentCodeUnitMethods = methodsFromCodeUnit(entry.Data.CodeUnit, sourceIndex)
		}

		candidates := matchEntry(entry, sourceIndex, currentCodeUnitMethods)
		sortCandidates(candidates, preferredFileForEntry(currentCodeUnitMethods, entry))
		if len(candidates) > maxCandidates {
			candidates = candidates[:maxCandidates]
		}

		annotated := AnnotatedEntry{Entry: entry}
		if len(candidates) > 0 {
			annotated.Candidates = candidates
			annotated.Best = candidates[0]
		}
		out.Entries = append(out.Entries, annotated)

		if entry.Kind == apexlog.EntryCodeUnitFinished {
			currentCodeUnitMethods = []sourceMethod{}
		}
	}

	return out, nil
}

func matchEntry(entry apexlog.Entry, index SourceIndex, currentCodeUnitMethods []sourceMethod) []SourceCandidate {
	candidates := map[string]SourceCandidate{}

	for _, frame := range entry.Data.StackFrames {
		for _, method := range methodBySymbol(index, frame.Namespace, frame.Class, frame.Method) {
			line := frame.Line
			if line == 0 {
				line = method.StartLine
			}
			addCandidate(candidates, SourceCandidate{
				File:       method.File,
				Line:       line,
				Symbol:     methodSymbol(method.Namespace, method.TypeName, method.MethodKey()),
				Reason:     "stack frame",
				Confidence: 0.95,
			})
		}
	}

	if isMethodLikeEntry(entry.Kind) && entry.Data.MethodSymbol != "" {
		for _, method := range methodsForEntrySymbol(index, entry.Kind, entry.Data.MethodSymbol) {
			addCandidate(candidates, SourceCandidate{
				File:       method.File,
				Line:       method.StartLine,
				Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
				Reason:     "method symbol",
				Confidence: 0.90,
			})
		}
	}

	if entry.Data.CodeUnit != "" {
		ns, typeName, methodName := parseCodeUnitSymbol(entry.Data.CodeUnit)
		for _, method := range methodsForSymbol(index, ns, typeName, methodName) {
			addCandidate(candidates, SourceCandidate{
				File:       method.File,
				Line:       method.StartLine,
				Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
				Reason:     "code unit",
				Confidence: 0.85,
			})
		}
	}

	sourceLine := entry.Data.SourceLine
	methodCandidates := currentCodeUnitMethods
	if sourceLine > 0 && len(methodCandidates) == 0 {
		if method := methodByLine(index.methods, sourceLine); method.Name != "" {
			methodCandidates = append(methodCandidates, method)
		}
	}

	if entry.Kind == apexlog.EntryUserDebug && entry.Data.DebugMessage != "" {
		normalized := normalizeForMatch(entry.Data.DebugMessage)
		if normalized != "" {
			for _, method := range methodCandidates {
				key := methodLookupKey(method.Namespace, method.TypeName, method.Name)
				for _, literal := range index.debugLiteralsByKey[key] {
					if literal.Normalized == "" || literal.Normalized != normalized {
						continue
					}
					addCandidate(candidates, SourceCandidate{
						File:       literal.File,
						Line:       literal.Line,
						Symbol:     literal.Symbol,
						Reason:     "debug literal",
						Confidence: 0.90,
					})
				}
			}
		}
	}

	if entry.Data.SOQLQuery != "" {
		normalizedQuery := normalizeQuery(entry.Data.SOQLQuery)
		fromObject := strings.ToLower(normalizeType(parseFromObject(entry.Data.SOQLQuery)))
		for _, method := range methodCandidates {
			key := methodLookupKey(method.Namespace, method.TypeName, method.Name)
			for _, q := range index.soqlByKey[key] {
				if q.Normalized == "" {
					continue
				}
				if q.Normalized == normalizedQuery {
					addCandidate(candidates, SourceCandidate{
						File:       q.File,
						Line:       q.Line,
						Symbol:     q.Symbol,
						Reason:     "soql exact",
						Confidence: 0.85,
					})
					continue
				}
				if fromObject != "" && strings.EqualFold(normalizeType(q.FromObject), fromObject) {
					addCandidate(candidates, SourceCandidate{
						File:       q.File,
						Line:       q.Line,
						Symbol:     q.Symbol,
						Reason:     "soql object",
						Confidence: 0.45,
					})
				}
			}
		}
	}

	if entry.Kind == apexlog.EntryDMLBegin && entry.Data.DMLOperation != "" {
		op := normalizeType(entry.Data.DMLOperation)
		obj := normalizeType(entry.Data.DMLType)
		for _, method := range methodCandidates {
			key := methodLookupKey(method.Namespace, method.TypeName, method.Name)
			for _, dml := range index.dmlByKey[key] {
				if dml.Operation == "" {
					continue
				}
				if strings.EqualFold(dml.Operation, op) {
					if obj == "" || strings.EqualFold(normalizeType(dml.ObjectType), obj) {
						addCandidate(candidates, SourceCandidate{
							File:       dml.File,
							Line:       dml.Line,
							Symbol:     dml.Symbol,
							Reason:     "dml exact",
							Confidence: 0.65,
						})
					} else {
						addCandidate(candidates, SourceCandidate{
							File:       dml.File,
							Line:       dml.Line,
							Symbol:     dml.Symbol,
							Reason:     "dml operation",
							Confidence: 0.35,
						})
					}
				}
			}
		}
	}

	if sourceLine > 0 {
		if len(currentCodeUnitMethods) > 0 {
			if method := methodByLine(methodsForFile(currentCodeUnitMethods), sourceLine); method.Name != "" {
				addCandidate(candidates, SourceCandidate{
					File:       method.File,
					Line:       sourceLine,
					Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
					Reason:     "debug source line",
					Confidence: 0.75,
				})
			}
		} else if method := methodByLine(index.methods, sourceLine); method.Name != "" {
			addCandidate(candidates, SourceCandidate{
				File:       method.File,
				Line:       sourceLine,
				Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
				Reason:     "line",
				Confidence: 0.50,
			})
		}
	}

	out := make([]SourceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Reason == "" {
			candidate.Reason = "match"
		}
		out = append(out, candidate)
	}

	return out
}

func methodsForEntrySymbol(index SourceIndex, kind apexlog.EntryKind, symbol string) []sourceMethod {
	ns, typeName, methodName := parseMethodLikeSymbol(symbol)
	methods := methodsForSymbol(index, ns, typeName, methodName)
	if len(methods) > 0 {
		return methods
	}
	if !isConstructorLikeEntry(kind) && strings.Contains(symbol, "(") {
		return nil
	}
	ns, typeName, methodName = parseConstructorLikeSymbol(symbol)
	if typeName == "" || methodName == "" {
		return nil
	}
	return methodsForSymbol(index, ns, typeName, methodName)
}

func isMethodLikeEntry(kind apexlog.EntryKind) bool {
	switch kind {
	case apexlog.EntryMethodEntry, apexlog.EntryMethodExit,
		apexlog.EntryConstructorEntry, apexlog.EntryConstructorExit,
		apexlog.EntrySystemMethodEntry, apexlog.EntrySystemMethodExit:
		return true
	default:
		return false
	}
}

func isConstructorLikeEntry(kind apexlog.EntryKind) bool {
	switch kind {
	case apexlog.EntryConstructorEntry, apexlog.EntryConstructorExit:
		return true
	default:
		return false
	}
}

func parseConstructorLikeSymbol(value string) (namespace, typeName, methodName string) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "Class."))
	if value == "" || strings.HasPrefix(value, "<") {
		return "", "", ""
	}
	if open := strings.Index(value, "("); open >= 0 {
		value = strings.TrimSpace(value[:open])
	}
	parts := strings.Split(value, ".")
	if len(parts) == 0 {
		return "", "", ""
	}
	typeName = strings.TrimSpace(parts[len(parts)-1])
	namespace = strings.Join(parts[:len(parts)-1], ".")
	methodName = constructorNameForType(typeName)
	return namespace, typeName, methodName
}

func addCandidate(candidates map[string]SourceCandidate, candidate SourceCandidate) {
	if candidate.Confidence < 0 {
		return
	}
	if candidate.File == "" {
		return
	}
	if candidate.Line < 0 {
		candidate.Line = 0
	}
	key := candidateKey(candidate)
	existing, ok := candidates[key]
	if !ok || candidate.Confidence > existing.Confidence {
		candidates[key] = candidate
	}
}

func candidateKey(candidate SourceCandidate) string {
	return filepath.Clean(candidate.File) + "|" + strconv.Itoa(candidate.Line) + "|" + strings.ToLower(candidate.Symbol)
}

func methodsForFile(methods []sourceMethod) []sourceMethod {
	out := make([]sourceMethod, 0, len(methods))
	seen := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		file := filepath.Clean(method.File)
		key := file + "|" + methodKey(method)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, method)
	}
	return out
}

func sortCandidates(candidates []SourceCandidate, contextFile string) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		if contextFile != "" {
			iExact := samePath(candidates[i].File, contextFile)
			jExact := samePath(candidates[j].File, contextFile)
			if iExact != jExact {
				return iExact
			}
		}
		if candidates[i].Line != candidates[j].Line {
			return candidates[i].Line < candidates[j].Line
		}
		return strings.ToLower(candidates[i].Symbol) < strings.ToLower(candidates[j].Symbol)
	})
}

func samePath(file string, expected string) bool {
	if expected == "" {
		return false
	}
	return filepath.Clean(file) == filepath.Clean(expected)
}

func preferredFileForEntry(currentCodeUnitMethods []sourceMethod, entry apexlog.Entry) string {
	if method := methodByLine(currentCodeUnitMethods, entry.Data.SourceLine); method.Name != "" {
		return filepath.Clean(method.File)
	}
	if len(currentCodeUnitMethods) == 0 {
		return ""
	}
	return filepath.Clean(currentCodeUnitMethods[0].File)
}

func methodsFromCodeUnit(codeUnit string, sourceIndex SourceIndex) []sourceMethod {
	ns, typeName, methodName := parseCodeUnitSymbol(codeUnit)
	if ns == "" && typeName == "" && methodName == "" {
		return []sourceMethod{}
	}
	return methodsForSymbol(sourceIndex, ns, typeName, methodName)
}

func methodsForSymbol(index SourceIndex, namespace, typeName, methodName string) []sourceMethod {
	methods := methodBySymbol(index, namespace, typeName, methodName)
	if namespace == "" {
		return dedupeMethods(methods)
	}
	for _, fallback := range methodLookupKeys(namespace, typeName, methodName) {
		for _, method := range index.methodsBySymbol[fallback] {
			methods = append(methods, method)
		}
	}
	return dedupeMethods(methods)
}

func parseCodeUnitSymbol(raw string) (namespace, typeName, methodName string) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "Class."))
	raw = strings.TrimPrefix(raw, "Trigger.")
	if raw == "" {
		return "", "", ""
	}
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return "", "", ""
	}
	methodName = strings.TrimSpace(parts[len(parts)-1])
	typeName = strings.TrimSpace(parts[len(parts)-2])
	namespace = strings.Join(parts[:len(parts)-2], ".")
	return namespace, typeName, methodName
}

func (m sourceMethod) MethodKey() string {
	if m.Name == "" || m.TypeName == "" {
		return ""
	}
	return methodSymbol(m.Namespace, m.TypeName, m.Name)
}
