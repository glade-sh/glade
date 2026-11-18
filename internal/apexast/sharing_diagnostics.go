package apexast

import (
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// C009/C010 and NC002-NC006: these malformed sharing headers have measured
// native diagnostics. The leading alternatives consume quoted/comment text
// before matching declaration headers (N012/N013).
var sharingDeclarationPattern = regexp.MustCompile(`'(?:\\.|[^'\\])*(?:'|$)|//[^\r\n]*|/\*[\s\S]*?(?:\*/|$)|(\bclass\s+[A-Za-z_][A-Za-z_0-9]*\s+(?:with|without|inherited)\s+sharing\b)|(\bwith\s+sharing\s+without\s+sharing\s+class\s+[A-Za-z_][A-Za-z_0-9]*\b)|(\binherited\s+sharing\s+with\s+sharing\s+class\s+[A-Za-z_][A-Za-z_0-9]*\b)`)

// SharingDeclarationDiagnostics is shared by declaration parsing and the
// anonymous body compiler, which otherwise lowers declarations differently.
func SharingDeclarationDiagnostics(path, source string) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	if !sharingDeclarationCandidate(source) {
		return out
	}
	lines := NewLineMap(source)
	for _, match := range sharingDeclarationPattern.FindAllStringSubmatchIndex(source, -1) {
		message := ""
		switch {
		case match[2] >= 0:
			message = "Unexpected token 'class'."
		case match[4] >= 0:
			message = "withSharing classes cannot be withoutSharing"
		case match[6] >= 0:
			message = "inheritedSharing classes cannot be withSharing"
		default:
			continue
		}
		rng := diagnostic.Range{Start: lines.Position(match[0]), End: lines.Position(match[1])}
		out = append(out, diagnostic.Diagnostic{
			Severity: diagnostic.Error, Code: "APEXPARSE001", File: path,
			Message: message, NativeMessage: message, Range: &rng,
		})
	}
	return out
}

func withSharingDeclarationDiagnostics(file File, source string) File {
	for _, native := range SharingDeclarationDiagnostics(file.Path, source) {
		replaced := false
		for i, item := range file.Diagnostics {
			if item.Severity != diagnostic.Error || item.Code != "APEXPARSE001" || item.Message != "syntax error" || item.Range == nil {
				continue
			}
			if item.Range.Start.Offset < native.Range.End.Offset && item.Range.End.Offset > native.Range.Start.Offset {
				file.Diagnostics[i] = native
				replaced = true
				break
			}
		}
		if !replaced {
			file.Diagnostics = append(file.Diagnostics, native)
		}
	}
	return file
}

// sharingDeclarationCandidate reports whether sharingDeclarationPattern's
// capturing alternatives can match anywhere in source. Each needs the word
// sharing preceded by whitespace and with, without or inherited; the two
// doubled-sharing forms need that twice, and the class-first form needs
// "class", an identifier and whitespace before the modifier. The scan is
// case-sensitive like the pattern, so an ordinary "with sharing class"
// header is rejected without running the pattern over the source.
func sharingDeclarationCandidate(source string) bool {
	qualifying := 0
	for from := 0; ; {
		at := strings.Index(source[from:], "sharing")
		if at < 0 {
			break
		}
		at += from
		from = at + len("sharing")
		i := at
		for i > 0 && sharingDeclarationSpace(source[i-1]) {
			i--
		}
		if i == at {
			continue
		}
		modifier := source[:i]
		switch {
		case strings.HasSuffix(modifier, "with"), strings.HasSuffix(modifier, "without"), strings.HasSuffix(modifier, "inherited"):
		default:
			continue
		}
		qualifying++
		if qualifying > 1 {
			return true
		}
		if sharingDeclarationClassBefore(modifier) {
			return true
		}
	}
	return false
}

// sharingDeclarationClassBefore reports whether text ends with a sharing
// modifier that "class", whitespace, an identifier and whitespace precede.
func sharingDeclarationClassBefore(text string) bool {
	i := len(text)
	for i > 0 && sharingDeclarationIdentifierByte(text[i-1]) {
		i--
	}
	wordStart := i
	for i > 0 && sharingDeclarationSpace(text[i-1]) {
		i--
	}
	if i == wordStart {
		return false
	}
	identifierEnd := i
	for i > 0 && sharingDeclarationIdentifierByte(text[i-1]) {
		i--
	}
	if i == identifierEnd {
		return false
	}
	spaceEnd := i
	for i > 0 && sharingDeclarationSpace(text[i-1]) {
		i--
	}
	if i == spaceEnd {
		return false
	}
	return strings.HasSuffix(text[:i], "class")
}

// sharingDeclarationSpace matches the pattern's \s class: RE2 Perl whitespace.
func sharingDeclarationSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\f' || value == '\r'
}

func sharingDeclarationIdentifierByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}
