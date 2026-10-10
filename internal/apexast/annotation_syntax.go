package apexast

import (
	"strings"
	"unicode"

	"github.com/glade-sh/glade/internal/diagnostic"
)

type annotationSyntaxToken struct {
	text   string
	offset int
}

// C142/C180/C220/C233 observe native recovery for null annotation literals.
// Strings and comments are never tokens. The adapter also rejects these null
// literals when the grammar accepts a broader annotation-expression shape.
func nativeAnnotationSyntax(file File, source string) File {
	if !strings.Contains(source, "@") {
		return file
	}
	parseError := -1
	for i, d := range file.Diagnostics {
		if d.Code == "APEXPARSE001" && d.Severity == diagnostic.Error {
			parseError = i
			break
		}
	}
	tokens := annotationSyntaxTokens(source)
	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i].text != "@" || tokens[i+2].text != "(" {
			continue
		}
		name := strings.ToLower(tokens[i+1].text)
		if name != "auraenabled" && name != "restresource" && name != "suppresswarnings" && name != "jsonaccess" {
			continue
		}
		end := i + 3
		for end < len(tokens) && tokens[end].text != ")" {
			end++
		}
		if end == len(tokens) {
			continue
		}
		null := false
		for _, token := range tokens[i+3 : end] {
			if strings.EqualFold(token.text, "null") {
				null = true
			}
		}
		if !null {
			continue
		}
		message := "Unexpected token '@'."
		if name == "auraenabled" {
			// Restrict C142's recovery to a field inside a class. The native
			// row does not establish recovery for other AuraEnabled targets.
			field := false
			for j := end + 1; j < len(tokens); j++ {
				if tokens[j].text == ";" {
					field = true
					break
				}
				if tokens[j].text == "{" || tokens[j].text == "(" {
					break
				}
			}
			classDepth := annotationClassDepth(tokens[:i])
			if !field || classDepth == 0 {
				continue
			}
			if classDepth == 1 {
				message = "Unexpected token 'class'."
			}
		}
		offset := tokens[i].offset
		prefix := source[:offset]
		line := strings.Count(prefix, "\n") + 1
		column := offset - strings.LastIndex(prefix, "\n")
		if parseError < 0 {
			parseError = len(file.Diagnostics)
			file.Diagnostics = append(file.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "APEXPARSE001", Message: "syntax error", File: file.Path})
		}
		d := &file.Diagnostics[parseError]
		d.NativeMessage = message
		d.Range = &diagnostic.Range{Start: diagnostic.Position{Offset: offset, Line: line, Column: column}, End: diagnostic.Position{Offset: offset + 1, Line: line, Column: column + 1}}
		break
	}
	return file
}

func annotationClassDepth(tokens []annotationSyntaxToken) int {
	var blocks []bool
	depth, pending := 0, false
	for i, token := range tokens {
		switch strings.ToLower(token.text) {
		case "class", "interface", "enum":
			if i > 0 && tokens[i-1].text == "." {
				continue // A .class literal does not start a declaration.
			}
			pending = true
		case "{":
			blocks = append(blocks, pending)
			if pending {
				depth++
			}
			pending = false
		case "}":
			if len(blocks) > 0 {
				if blocks[len(blocks)-1] {
					depth--
				}
				blocks = blocks[:len(blocks)-1]
			}
		}
	}
	return depth
}

func annotationSyntaxTokens(source string) []annotationSyntaxToken {
	var tokens []annotationSyntaxToken
	for i := 0; i < len(source); {
		start := i
		switch {
		case unicode.IsSpace(rune(source[i])):
			i++
		case strings.HasPrefix(source[i:], "//"):
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case strings.HasPrefix(source[i:], "/*"):
			i += 2
			for i < len(source) && !strings.HasPrefix(source[i:], "*/") {
				i++
			}
			if i < len(source) {
				i += 2
			}
		case source[i] == '\'':
			i++
			for i < len(source) {
				if source[i] == '\\' && i+1 < len(source) {
					i += 2
				} else if source[i] == '\'' {
					i++
					break
				} else {
					i++
				}
			}
			tokens = append(tokens, annotationSyntaxToken{text: source[start:i], offset: start})
		case source[i] == '_' || unicode.IsLetter(rune(source[i])):
			i++
			for i < len(source) && (source[i] == '_' || unicode.IsLetter(rune(source[i])) || unicode.IsDigit(rune(source[i]))) {
				i++
			}
			tokens = append(tokens, annotationSyntaxToken{text: source[start:i], offset: start})
		default:
			i++
			tokens = append(tokens, annotationSyntaxToken{text: source[start:i], offset: start})
		}
	}
	return tokens
}
