package apexast

import (
	"strings"

	external "github.com/glade-sh/apex-parser"
	"github.com/glade-sh/glade/internal/sosl"
)

func hasGenericSOSLSyntaxDiagnostic(items []external.Diagnostic) bool {
	for _, item := range items {
		if item.Severity == external.Error && item.Code == "APEXPARSE001" && item.Message == "syntax error" {
			return true
		}
	}
	return false
}

// Refine at the parser entry point: index/CLI callers can stop before sema.
// Reparse a private, byte-aligned repair to retain unrelated Apex syntax errors.
func (p *Parser) refineInlineSOSLDiagnostics(path, source string, items []external.Diagnostic) []external.Diagnostic {
	if !hasGenericSOSLSyntaxDiagnostic(items) {
		return items
	}
	const validQuery = "FIND 'x' RETURNING Account(Id)"
	repaired := []byte(source)
	lineMap := external.NewLineMap(source)
	var native []external.Diagnostic
	for i := 0; i < len(source); {
		if end := soslIgnoredSourceEnd(source, i); end > i {
			i = end
			continue
		}
		if source[i] != '[' {
			i++
			continue
		}
		start := i + 1
		for start < len(source) && strings.ContainsRune(" \t\r\n", rune(source[start])) {
			start++
		}
		if start+len("FIND") >= len(source) || !strings.EqualFold(source[start:start+len("FIND")], "FIND") || !strings.ContainsRune(" \t\r\n", rune(source[start+len("FIND")])) {
			i++
			continue
		}
		end := soslQuerySourceEnd(source, start)
		if end < 0 {
			i++
			continue
		}
		raw := source[start:end]
		query, err := sosl.Parse(raw)
		message := sosl.InlineCompileDiagnosticMessage(raw, query, err)
		if message != "" {
			if len(raw) < len(validQuery) {
				return items
			}
			for cursor := start; cursor < end; cursor++ {
				if repaired[cursor] != '\n' && repaired[cursor] != '\r' {
					repaired[cursor] = ' '
				}
			}
			copy(repaired[start:end], validQuery)
			rng := external.Range{Start: lineMap.Position(start), End: lineMap.Position(end)}
			native = append(native, external.Diagnostic{Severity: external.Error, Code: "APEXPARSE001", Message: message, File: path, Range: &rng})
		}
		i = end + 1
	}
	if len(native) == 0 {
		return items
	}
	for _, item := range p.parser.ParseSource(path, string(repaired)).Diagnostics {
		if item.Severity == external.Error {
			return items
		}
	}
	out := make([]external.Diagnostic, 0, len(items)+len(native))
	for _, item := range items {
		if item.Severity == external.Error {
			if item.Code == "APEXPARSE001" && item.Message == "syntax error" {
				continue
			}
			// C002: tree recovery can reinterpret WHERE as a declaration.
			// The clean repair proves these query-contained identifier errors
			// belong to the malformed SOSL, not an Apex declaration.
			if item.Code == "APEXPARSE002" && item.Range != nil {
				insideQuery := false
				for _, query := range native {
					if item.Range.Start.Offset >= query.Range.Start.Offset && item.Range.End.Offset <= query.Range.End.Offset {
						insideQuery = true
						break
					}
				}
				if insideQuery {
					continue
				}
			}
		}
		out = append(out, item)
	}
	return append(out, native...)
}

func soslQuerySourceEnd(source string, start int) int {
	depth := 1
	for i := start; i < len(source); {
		if end := soslIgnoredSourceEnd(source, i); end > i {
			i = end
			continue
		}
		switch source[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

func soslIgnoredSourceEnd(source string, start int) int {
	if source[start] == '\'' {
		for i := start + 1; i < len(source); i++ {
			if source[i] == '\\' {
				i++
				continue
			}
			if source[i] == '\'' {
				if i+1 < len(source) && source[i+1] == '\'' {
					i++
					continue
				}
				return i + 1
			}
		}
		return len(source)
	}
	if strings.HasPrefix(source[start:], "//") {
		if end := strings.IndexByte(source[start:], '\n'); end >= 0 {
			return start + end + 1
		}
		return len(source)
	}
	if strings.HasPrefix(source[start:], "/*") {
		if end := strings.Index(source[start+2:], "*/"); end >= 0 {
			return start + 2 + end + 2
		}
		return len(source)
	}
	return start
}
