package sema

import (
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

var parameterizedClassMemberAccessPattern = regexp.MustCompile(`(?i)\b([a-z][a-z0-9_]*(?:\s*\.\s*[a-z][a-z0-9_]*)*)\s*<[a-z0-9_.,<>\[\]\s]+>\s*\.\s*class\s*\.`)
var anonymousSystemModeQueryPattern = regexp.MustCompile(`(?i)\bWITH\s+SYSTEM_MODE\b`)
var sourceSyntaxTokenPattern = regexp.MustCompile(`[a-zA-Z_][a-zA-Z_0-9]*|[^\s]`)

func sourceSyntaxDiagnostics(typ typesys.TypeSymbol, body string, bodyOffset int, source string, anonymous bool) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	code := sourceSyntaxCodeText(body)
	diagnostics = append(diagnostics, sourceSyntaxStatementDiagnostics(typ, code, bodyOffset, source)...)
	// R071/R072 reject direct member access on a parameterized .class literal.
	// The standalone literals used by JSON.deserialize remain valid.
	for _, match := range parameterizedClassMemberAccessPattern.FindAllStringSubmatchIndex(code, -1) {
		name := strings.TrimSpace(body[match[2]:match[3]])
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_SOURCE_SYNTAX", Message: "Unexpected token '" + name + "'.", File: typ.File, Range: semaRange(source, bodyOffset+match[2], bodyOffset+match[3])})
	}
	if anonymous {
		// C040: the source restriction belongs to anonymous SOQL, not named Apex.
		ignored := newSemaIgnoredText(body)
		for _, match := range anonymousSystemModeQueryPattern.FindAllStringIndex(code, -1) {
			if !ignored.inSOQLLiteral(match[0]) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_SOURCE_SYNTAX", Message: "Cannot use SYSTEM_MODE access level in anonymous execution of Apex.", File: typ.File, Range: semaRange(source, bodyOffset+match[0], bodyOffset+match[1])})
		}
	}
	return diagnostics
}

// R078/R079/R081/R095 and G001-G013 distinguish rejected empty statements
// and unbraced do bodies from accepted loop delimiters and empty loop bodies.
func sourceSyntaxStatementDiagnostics(typ typesys.TypeSymbol, code string, bodyOffset int, source string) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	add := func(message string, start, end int) {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_SOURCE_SYNTAX", Message: message, File: typ.File, Range: semaRange(source, bodyOffset+start, bodyOffset+end)})
	}
	tokens := sourceSyntaxTokenPattern.FindAllStringIndex(code, -1)
	var parens []string
	var blocks []bool
	previous := ""
	previousBlock, previousIf := false, false
	for i, token := range tokens {
		text := strings.ToLower(code[token[0]:token[1]])
		closedBlock, closedIf := false, false
		switch text {
		case "(":
			parens = append(parens, previous)
		case ")":
			if len(parens) > 0 {
				closedIf = parens[len(parens)-1] == "if"
				parens = parens[:len(parens)-1]
			}
		case "{":
			// Collection and array literals need their terminating semicolon.
			blocks = append(blocks, previous != ">" && previous != "]")
		case "}":
			if len(blocks) > 0 {
				closedBlock = blocks[len(blocks)-1]
				blocks = blocks[:len(blocks)-1]
			}
		case ";":
			empty := previous == "" || previous == ";" || previous == "{" || previous == "else"
			empty = empty || previous == "}" && previousBlock || previous == ")" && previousIf
			if len(parens) == 0 && empty {
				add("Unexpected token ';'.", token[0], token[1])
			}
		case "do":
			if i+1 < len(tokens) {
				next := tokens[i+1]
				if code[next[0]:next[1]] != "{" {
					add("Missing '{' at '"+code[next[0]:next[1]]+"'", next[0], next[1])
				}
			}
		}
		previous, previousBlock, previousIf = text, closedBlock, closedIf
	}
	return diagnostics
}

// Keep byte positions while removing quoted data and comments from syntax scans.
func sourceSyntaxCodeText(source string) string {
	code := []byte(source)
	for i := 0; i < len(source); {
		var end int
		switch {
		case source[i] == '\'':
			end = min(len(source), skipSemaString(source, i)+1)
		case strings.HasPrefix(source[i:], "//"):
			end = i + 2
			for end < len(source) && source[end] != '\n' && source[end] != '\r' {
				end++
			}
		case strings.HasPrefix(source[i:], "/*"):
			end = len(source)
			if close := strings.Index(source[i+2:], "*/"); close >= 0 {
				end = i + 2 + close + 2
			}
		default:
			i++
			continue
		}
		for j := i; j < end; j++ {
			if code[j] != '\n' && code[j] != '\r' {
				code[j] = ' '
			}
		}
		i = end
	}
	return string(code)
}
