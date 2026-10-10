package apexast

import (
	"regexp"

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
