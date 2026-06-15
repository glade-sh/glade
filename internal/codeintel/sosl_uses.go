package codeintel

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sosl"
)

func CollectSOSLUses(file, source string) []Use {
	lineMap := apexast.NewLineMap(source)
	var uses []Use
	for start := 0; start < len(source); start++ {
		switch source[start] {
		case '\'', '"':
			start = skipQuoted(source, start)
			continue
		case '/':
			if start+1 >= len(source) {
				continue
			}
			switch source[start+1] {
			case '/':
				start = skipLineComment(source, start+2)
				continue
			case '*':
				start = skipBlockComment(source, start+2)
				continue
			}
		case '[':
		default:
			continue
		}
		end := soslBracketEnd(source, start+1)
		if end < 0 {
			continue
		}
		queryText := source[start+1 : end]
		trimmed := strings.TrimLeft(queryText, " \t\r\n")
		leading := len(queryText) - len(trimmed)
		if !startsWithKeyword(trimmed, "FIND") {
			start = end
			continue
		}
		query, err := sosl.Parse(trimmed)
		if err != nil {
			start = end
			continue
		}
		uses = append(uses, collectParsedSOSLUses(file, lineMap, start+1+leading, trimmed, query)...)
		start = end
	}
	return uses
}

func collectParsedSOSLUses(file string, lineMap apexast.LineMap, queryOffset int, queryText string, query sosl.Query) []Use {
	returning := keywordIndex(queryText, "RETURNING")
	if returning < 0 {
		return nil
	}
	cursor := returning + len("RETURNING")
	var uses []Use
	for _, object := range query.Returning {
		objectOffset := findIdentifier(queryText, object.Object, cursor)
		if objectOffset < 0 {
			continue
		}
		uses = append(uses, newSOSLUse(file, lineMap, queryOffset+objectOffset, object.Object, SObjectID(object.Object)))
		cursor = objectOffset + len(object.Object)
		fieldCursor := cursor
		if paren := strings.IndexByte(queryText[fieldCursor:], '('); paren >= 0 {
			fieldCursor += paren + 1
		}
		for _, field := range object.Fields {
			fieldOffset := findIdentifier(queryText, field, fieldCursor)
			if fieldOffset < 0 {
				continue
			}
			uses = append(uses, newSOSLUse(file, lineMap, queryOffset+fieldOffset, field, SObjectFieldID(object.Object, field)))
			fieldCursor = fieldOffset + len(field)
		}
		cursor = fieldCursor
	}
	return uses
}

func newSOSLUse(file string, lineMap apexast.LineMap, offset int, name string, id SymbolID) Use {
	start := lineMap.Position(offset)
	end := lineMap.Position(offset + len(name))
	return Use{
		SymbolID: id,
		Kind:     UseQuery,
		Name:     name,
		File:     file,
		Range:    diagnostic.Range{Start: start, End: end},
		Resolved: true,
		Metadata: map[string]string{
			"query": "sosl",
		},
	}
}

func soslBracketEnd(source string, start int) int {
	for i := start; i < len(source); i++ {
		switch source[i] {
		case '\'', '"':
			i = skipQuoted(source, i)
		case ']':
			return i
		}
	}
	return -1
}

func skipLineComment(source string, start int) int {
	for i := start; i < len(source); i++ {
		if source[i] == '\n' {
			return i
		}
	}
	return len(source) - 1
}

func skipBlockComment(source string, start int) int {
	for i := start; i+1 < len(source); i++ {
		if source[i] == '*' && source[i+1] == '/' {
			return i + 1
		}
	}
	return len(source) - 1
}

func skipQuoted(source string, quote int) int {
	for i := quote + 1; i < len(source); i++ {
		if source[i] == '\\' {
			i++
			continue
		}
		if source[i] == source[quote] {
			return i
		}
	}
	return len(source) - 1
}

func startsWithKeyword(source, keyword string) bool {
	if len(source) < len(keyword) || !strings.EqualFold(source[:len(keyword)], keyword) {
		return false
	}
	return len(source) == len(keyword) || !isSOSLIdentPart(source[len(keyword)])
}

func keywordIndex(source, keyword string) int {
	upperKeyword := strings.ToUpper(keyword)
	for i := 0; i+len(keyword) <= len(source); i++ {
		if i > 0 && isSOSLIdentPart(source[i-1]) {
			continue
		}
		candidate := source[i : i+len(keyword)]
		if strings.ToUpper(candidate) != upperKeyword {
			continue
		}
		if i+len(keyword) < len(source) && isSOSLIdentPart(source[i+len(keyword)]) {
			continue
		}
		return i
	}
	return -1
}

func findIdentifier(source, ident string, start int) int {
	for i := start; i+len(ident) <= len(source); i++ {
		if i > 0 && isSOSLIdentPart(source[i-1]) {
			continue
		}
		if source[i:i+len(ident)] != ident {
			continue
		}
		if i+len(ident) < len(source) && isSOSLIdentPart(source[i+len(ident)]) {
			continue
		}
		return i
	}
	return -1
}

func isSOSLIdentPart(ch byte) bool {
	return ch == '_' || ch == '$' || ('0' <= ch && ch <= '9') || ('A' <= ch && ch <= 'Z') || ('a' <= ch && ch <= 'z')
}
