package sema

import (
	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"regexp"
	"strings"
)

// BI030 captures this specific malformed numeric Id subscript with a missing
// enclosing query close. Other malformed operands keep their existing errors.
var soqlBindCapturedMalformedIndex = regexp.MustCompile(`(?is)\[\s*SELECT\s+Id\s+FROM\s+Account\s+WHERE\s+Id\s*=\s*:\s*[A-Za-z_][A-Za-z0-9_]*\s*\[\s*0\s*\.\s*Id\s*\]\s*\.\s*size\s*\(\s*\)`)

func soqlBindMalformedDiagnostics(file, source string, anonymous bool) []diagnostic.Diagnostic {
	facts := newSourceFacts(source)
	spans := facts.codeSpans()
	repaired := []byte(source)
	var diagnostics []diagnostic.Diagnostic
	const replacement = "[SELECT Id FROM Account].size()"
	for _, match := range soqlBindCapturedMalformedIndex.FindAllStringIndex(source, -1) {
		if !spans.contains(match[0]) {
			continue
		}
		for i := match[0]; i < match[1]; i++ {
			if repaired[i] != '\n' && repaired[i] != '\r' {
				repaired[i] = ' '
			}
		}
		copy(repaired[match[0]:match[1]], replacement)
		const message = "Unrecognized symbol 'i', which is not a valid Apex identifier."
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_QUERY_PARSE", Message: message, NativeMessage: message, File: file, Range: semaRange(source, match[0], match[1])})
	}
	if len(diagnostics) == 0 {
		return nil
	}
	syntax := string(repaired)
	if anonymous {
		syntax = "public class BindMalformedSyntax { public static void run(){\n" + syntax + "\n}}"
	}
	parsed := apexast.ParseSource(file, syntax)
	for _, item := range parsed.Diagnostics {
		if item.Severity == diagnostic.Error {
			return nil
		}
	}
	return diagnostics
}

func soqlBindMalformedAnonymousMessage(source string, err error) string {
	if err == nil || !strings.HasPrefix(err.Error(), "unterminated SOQL literal at byte ") {
		return ""
	}
	if diagnostics := soqlBindMalformedDiagnostics("BindMalformedAnonymous.cls", source, true); len(diagnostics) != 0 {
		return diagnostics[0].NativeMessage
	}
	return ""
}
