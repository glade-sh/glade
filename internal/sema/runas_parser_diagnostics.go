package sema

import (
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

var semaRunAsBlockStart = regexp.MustCompile(`(?i)\bsystem\s*\.\s*runas\s*\(`)

// refineRunAsArgumentParserDiagnostics replaces a generic parser error only
// when every argument is independently syntactically valid and the entire
// source parses after retaining each one in turn. This preserves errors from
// unrelated malformed source. Native rows cover two and three arguments.
func (a *Analyzer) refineRunAsArgumentParserDiagnostics(index typesys.Index, input []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	if a.sources == nil {
		return input
	}
	seen := map[string]bool{}
	replaced := map[string]bool{}
	out := make([]diagnostic.Diagnostic, 0, len(input))
	for _, item := range input {
		if item.Severity != diagnostic.Error || item.Code != "APEXPARSE001" || item.Message != "syntax error" || item.File == "" {
			out = append(out, item)
			continue
		}
		if seen[item.File] {
			if !replaced[item.File] {
				out = append(out, item)
			}
			continue
		}
		seen[item.File] = true
		source, ok := a.sources.normalizedForType(typesys.TypeSymbol{File: item.File, SourceRoot: index.Project.Root, Namespace: index.Project.Namespace})
		if ok {
			if start, end, valid := semaMultipleRunAsCall(item.File, source); valid {
				replaced[item.File] = true
				const message = "runAs requires a single argument of type 'User' or 'Version'"
				out = append(out, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA023", Message: message, NativeMessage: message, File: item.File, Range: semaRange(source, start, end)})
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

type semaRunAsPart struct{ start, end int }

func semaMultipleRunAsCall(path, source string) (int, int, bool) {
	spans := newSemaCodeSpans(source)
	var start, end int
	var parts []semaRunAsPart
	found := false
	for _, match := range semaRunAsBlockStart.FindAllStringIndex(source, -1) {
		if !spans.contains(match[0]) {
			continue
		}
		open := match[1] - 1
		partStart := open + 1
		var stack []byte
		var current []semaRunAsPart
		closeAt := -1
		for i := open + 1; i < len(source); i++ {
			if !spans.contains(i) {
				continue
			}
			switch source[i] {
			case '(', '[', '{':
				stack = append(stack, source[i])
			case ')', ']', '}':
				if len(stack) == 0 {
					if source[i] == ')' {
						closeAt = i
					}
					break
				}
				top := stack[len(stack)-1]
				if !((top == '(' && source[i] == ')') || (top == '[' && source[i] == ']') || (top == '{' && source[i] == '}')) {
					return 0, 0, false
				}
				stack = stack[:len(stack)-1]
			case ',':
				if len(stack) == 0 {
					current = append(current, semaRunAsPart{partStart, i})
					partStart = i + 1
				}
			}
			if closeAt >= 0 {
				break
			}
		}
		if closeAt < 0 {
			continue
		}
		current = append(current, semaRunAsPart{partStart, closeAt})
		if len(current) < 2 || len(current) > 3 {
			continue
		}
		after := closeAt + 1
		for after < len(source) && (source[after] == ' ' || source[after] == '\t' || source[after] == '\r' || source[after] == '\n') {
			after++
		}
		if after == len(source) || source[after] != '{' || found {
			return 0, 0, false
		}
		for _, part := range current {
			if strings.TrimSpace(source[part.start:part.end]) == "" {
				return 0, 0, false
			}
		}
		found, start, end, parts = true, match[0], closeAt+1, current
	}
	if !found {
		return 0, 0, false
	}
	for _, keep := range parts {
		repaired := []byte(source)
		for i := parts[0].start; i < parts[len(parts)-1].end; i++ {
			if (i < keep.start || i >= keep.end) && repaired[i] != '\n' && repaired[i] != '\r' {
				repaired[i] = ' '
			}
		}
		for _, d := range apexast.ParseSource(path, string(repaired)).Diagnostics {
			if d.Severity == diagnostic.Error {
				return 0, 0, false
			}
		}
	}
	return start, end, true
}
