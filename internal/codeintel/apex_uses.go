package codeintel

import (
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

type token struct {
	Text  string
	File  string
	Range diagnostic.Range
}

func collectApexUses(index typesys.Index, declarations Graph) []Use {
	resolver := newApexUseResolver(index, declarations)
	files := apexUseFiles(index)
	var uses []Use
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		tokens := apexTokens(file, string(data))
		uses = append(uses, resolver.collectFile(tokens)...)
	}
	sortUses(uses)
	return uses
}

func apexUseFiles(index typesys.Index) []string {
	seen := make(map[string]bool)
	var files []string
	add := func(file string) {
		if file == "" || seen[file] {
			return
		}
		seen[file] = true
		files = append(files, file)
	}
	for _, typ := range index.Types {
		if !typ.Dependency && !typ.Artifact {
			add(typ.File)
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.Dependency {
			add(trigger.File)
		}
	}
	sort.Strings(files)
	return files
}

func apexTokens(file, source string) []token {
	var out []token
	line, col := 1, 1
	for i := 0; i < len(source); {
		ch := source[i]
		if ch == '\r' {
			i++
			continue
		}
		if ch == '\n' {
			i++
			line++
			col = 1
			continue
		}
		if unicode.IsSpace(rune(ch)) {
			i++
			col++
			continue
		}
		if ch == '/' && i+1 < len(source) && source[i+1] == '/' {
			i += 2
			col += 2
			for i < len(source) && source[i] != '\n' {
				i++
				col++
			}
			continue
		}
		if ch == '/' && i+1 < len(source) && source[i+1] == '*' {
			i += 2
			col += 2
			for i < len(source) {
				if source[i] == '*' && i+1 < len(source) && source[i+1] == '/' {
					i += 2
					col += 2
					break
				}
				if source[i] == '\n' {
					i++
					line++
					col = 1
					continue
				}
				i++
				col++
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote := ch
			i++
			col++
			for i < len(source) {
				if source[i] == '\\' && i+1 < len(source) {
					i += 2
					col += 2
					continue
				}
				if source[i] == quote {
					i++
					col++
					break
				}
				if source[i] == '\n' {
					i++
					line++
					col = 1
					continue
				}
				i++
				col++
			}
			continue
		}
		startLine, startCol, startOffset := line, col, i
		if isApexIdentStart(ch) {
			i++
			col++
			for i < len(source) && isApexIdentPart(source[i]) {
				i++
				col++
			}
			out = append(out, token{
				Text:  source[startOffset:i],
				File:  file,
				Range: tokenRange(startLine, startCol, startOffset, line, col, i),
			})
			continue
		}
		i++
		col++
		out = append(out, token{
			Text:  string(ch),
			File:  file,
			Range: tokenRange(startLine, startCol, startOffset, line, col, i),
		})
	}
	return out
}

func tokenRange(startLine, startCol, startOffset, endLine, endCol, endOffset int) diagnostic.Range {
	return diagnostic.Range{
		Start: diagnostic.Position{Line: startLine, Column: startCol, Offset: startOffset},
		End:   diagnostic.Position{Line: endLine, Column: endCol, Offset: endOffset},
	}
}

func isApexIdentStart(ch byte) bool {
	return ch == '_' || unicode.IsLetter(rune(ch))
}

func isApexIdentPart(ch byte) bool {
	return ch == '_' || unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch))
}

type apexUseResolver struct {
	typeByName   map[string][]Symbol
	memberByType map[string]map[string][]Symbol
	objectByName map[string]Symbol
}

func newApexUseResolver(index typesys.Index, declarations Graph) apexUseResolver {
	resolver := apexUseResolver{
		typeByName:   make(map[string][]Symbol),
		memberByType: make(map[string]map[string][]Symbol),
		objectByName: make(map[string]Symbol),
	}
	for _, symbol := range declarations.Symbols {
		switch symbol.Kind {
		case SymbolApexType:
			resolver.typeByName[strings.ToLower(symbol.Name)] = append(resolver.typeByName[strings.ToLower(symbol.Name)], symbol)
		case SymbolApexMember:
			parts := ParseID(symbol.Container)
			if len(parts) < 4 {
				continue
			}
			typeName := strings.ToLower(parts[3])
			memberName := strings.ToLower(symbol.Name)
			if resolver.memberByType[typeName] == nil {
				resolver.memberByType[typeName] = make(map[string][]Symbol)
			}
			resolver.memberByType[typeName][memberName] = append(resolver.memberByType[typeName][memberName], symbol)
		case SymbolSObject:
			resolver.objectByName[strings.ToLower(symbol.Name)] = symbol
		}
	}
	for _, object := range index.Objects {
		symbol := SymbolForObject(object)
		resolver.objectByName[strings.ToLower(symbol.Name)] = symbol
	}
	for _, trigger := range index.Triggers {
		if trigger.ObjectName == "" {
			continue
		}
		resolver.objectByName[strings.ToLower(trigger.ObjectName)] = Symbol{
			ID:   SObjectID(trigger.ObjectName),
			Kind: SymbolSObject,
			Name: trigger.ObjectName,
		}
	}
	return resolver
}

func (r apexUseResolver) collectFile(tokens []token) []Use {
	var uses []Use
	for i := range tokens {
		if _, _, ok := localDeclarationAt(tokens, i); ok {
			if _, resolved := r.uniqueType(tokens[i].Text); resolved {
				uses = append(uses, r.typeUse(tokens[i], UseRead)...)
			}
		}
		lower := strings.ToLower(tokens[i].Text)
		switch lower {
		case "extends":
			if i+1 < len(tokens) {
				uses = append(uses, r.typeUse(tokens[i+1], UseExtends)...)
			}
		case "implements":
			for j := i + 1; j < len(tokens); j++ {
				if tokens[j].Text == "{" || strings.EqualFold(tokens[j].Text, "extends") {
					break
				}
				if tokens[j].Text == "," || tokens[j].Text == "<" || tokens[j].Text == ">" {
					continue
				}
				if isIdentifierToken(tokens[j]) {
					uses = append(uses, r.typeUse(tokens[j], UseImplements)...)
				}
			}
		case "trigger":
			if i+3 < len(tokens) && strings.EqualFold(tokens[i+2].Text, "on") {
				uses = append(uses, r.objectUse(tokens[i+3])...)
			}
		case "new":
			if i+1 >= len(tokens) || !isIdentifierToken(tokens[i+1]) {
				continue
			}
			typeTok := tokens[i+1]
			if _, resolved := r.uniqueType(typeTok.Text); resolved {
				uses = append(uses, r.typeUse(typeTok, UseRead)...)
			}
			uses = append(uses, r.constructUse(typeTok)...)
			if methodIndex := chainedMethodAfterConstruct(tokens, i+2); methodIndex >= 0 {
				uses = append(uses, r.memberUse(typeTok.Text, tokens[methodIndex])...)
			}
		default:
			if !isIdentifierToken(tokens[i]) || i+3 >= len(tokens) || tokens[i+1].Text != "." || tokens[i+3].Text != "(" {
				continue
			}
			memberTok := tokens[i+2]
			if !isIdentifierToken(memberTok) {
				continue
			}
			if _, ok := r.uniqueType(tokens[i].Text); ok {
				uses = append(uses, r.typeUse(tokens[i], UseRead)...)
				uses = append(uses, r.memberUse(tokens[i].Text, memberTok)...)
				continue
			}
			locals := localTypesBefore(tokens, i)
			if typeName := locals[strings.ToLower(tokens[i].Text)]; typeName != "" {
				uses = append(uses, r.memberUse(typeName, memberTok)...)
				continue
			}
			uses = append(uses, unresolvedUse(memberTok, UseCall))
		}
	}
	return uses
}

func chainedMethodAfterConstruct(tokens []token, start int) int {
	depth := 0
	for i := start; i < len(tokens); i++ {
		switch tokens[i].Text {
		case "(":
			depth++
		case ")":
			if depth > 0 {
				depth--
			}
			if depth == 0 && i+3 < len(tokens) && tokens[i+1].Text == "." && isIdentifierToken(tokens[i+2]) && tokens[i+3].Text == "(" {
				return i + 2
			}
		case ";", "{", "}":
			if depth == 0 {
				return -1
			}
		}
	}
	return -1
}

func localTypesBefore(tokens []token, offsetToken int) map[string]string {
	locals := make(map[string]string)
	for i := 0; i+1 < offsetToken; i++ {
		typeName, nameIndex, ok := localDeclarationAt(tokens, i)
		if !ok || nameIndex >= offsetToken {
			continue
		}
		locals[strings.ToLower(tokens[nameIndex].Text)] = typeName
	}
	return locals
}

func localDeclarationAt(tokens []token, i int) (string, int, bool) {
	if !isIdentifierToken(tokens[i]) || isKeyword(tokens[i].Text) {
		return "", 0, false
	}
	if i+3 < len(tokens) && tokens[i+1].Text == "<" {
		j := i + 2
		depth := 1
		var inner string
		for ; j < len(tokens); j++ {
			if tokens[j].Text == "<" {
				depth++
				continue
			}
			if tokens[j].Text == ">" {
				depth--
				if depth == 0 {
					break
				}
				continue
			}
			if inner == "" && isIdentifierToken(tokens[j]) {
				inner = tokens[j].Text
			}
		}
		if inner != "" && j+1 < len(tokens) && isIdentifierToken(tokens[j+1]) {
			return tokens[i].Text + "<" + inner + ">", j + 1, true
		}
		return "", 0, false
	}
	if isIdentifierToken(tokens[i+1]) && !isKeyword(tokens[i+1].Text) {
		return tokens[i].Text, i + 1, true
	}
	return "", 0, false
}

func (r apexUseResolver) typeUse(tok token, kind UseKind) []Use {
	symbol, ok := r.uniqueType(tok.Text)
	if !ok {
		return []Use{unresolvedUse(tok, kind)}
	}
	return []Use{resolvedUse(tok, kind, symbol.ID)}
}

func (r apexUseResolver) objectUse(tok token) []Use {
	symbol, ok := r.objectByName[strings.ToLower(tok.Text)]
	if !ok {
		return []Use{unresolvedUse(tok, UseRead)}
	}
	return []Use{resolvedUse(tok, UseRead, symbol.ID)}
}

func (r apexUseResolver) constructUse(tok token) []Use {
	symbols := r.memberByType[strings.ToLower(tok.Text)][strings.ToLower(tok.Text)]
	if len(symbols) == 1 && symbols[0].Metadata["declarationKind"] == string(apexast.DeclarationConstructor) {
		return []Use{resolvedUse(tok, UseConstruct, symbols[0].ID)}
	}
	if _, ok := r.uniqueType(tok.Text); ok {
		return r.typeUse(tok, UseConstruct)
	}
	return []Use{unresolvedUse(tok, UseConstruct)}
}

func (r apexUseResolver) memberUse(typeName string, tok token) []Use {
	symbols := r.memberByType[strings.ToLower(typeName)][strings.ToLower(tok.Text)]
	if len(symbols) != 1 {
		return []Use{unresolvedUse(tok, UseCall)}
	}
	return []Use{resolvedUse(tok, UseCall, symbols[0].ID)}
}

func (r apexUseResolver) uniqueType(name string) (Symbol, bool) {
	symbols := r.typeByName[strings.ToLower(name)]
	if len(symbols) != 1 {
		return Symbol{}, false
	}
	return symbols[0], true
}

func resolvedUse(tok token, kind UseKind, id SymbolID) Use {
	return Use{
		SymbolID: id,
		Kind:     kind,
		Name:     tok.Text,
		File:     tok.File,
		Range:    tok.Range,
		Resolved: true,
	}
}

func unresolvedUse(tok token, kind UseKind) Use {
	return Use{
		Kind:     kind,
		Name:     tok.Text,
		File:     tok.File,
		Range:    tok.Range,
		Resolved: false,
	}
}

func isIdentifierToken(tok token) bool {
	return tok.Text != "" && isApexIdentStart(tok.Text[0])
}

func isKeyword(text string) bool {
	switch strings.ToLower(text) {
	case "abstract", "after", "before", "break", "catch", "class", "continue", "delete", "do", "else", "enum", "extends", "final", "finally", "for", "global", "if", "implements", "insert", "interface", "new", "on", "override", "private", "protected", "public", "return", "static", "testmethod", "this", "trigger", "try", "update", "upsert", "virtual", "void", "while", "with", "without":
		return true
	default:
		return false
	}
}
