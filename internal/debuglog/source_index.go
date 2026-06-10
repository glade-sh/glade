package debuglog

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

var (
	varDeclarationRe = regexp.MustCompile(`(?i)^\s*([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)?(?:<[A-Za-z_][A-Za-z0-9_<>, ]*>)?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*([=;].*)?$`)
)

type SourceIndex struct {
	methodsByFile       map[string][]sourceMethod
	methodsBySymbol     map[string][]sourceMethod
	methods             []sourceMethod
	debugLiteralsByKey  map[string][]sourceDebugLiteral
	soqlByKey           map[string][]sourceSoqlQuery
	dmlByKey            map[string][]sourceDML
	debugLiteralsByFile map[string][]sourceDebugLiteral
	soqlByFile          map[string][]sourceSoqlQuery
	dmlByFile           map[string][]sourceDML
}

type sourceMethod struct {
	Namespace string
	TypeName  string
	Name      string
	File      string
	StartLine int
	EndLine   int
}

type sourceDebugLiteral struct {
	File       string
	Line       int
	Symbol     string
	Message    string
	Normalized string
}

type sourceSoqlQuery struct {
	File       string
	Line       int
	Symbol     string
	Query      string
	Normalized string
	FromObject string
}

type sourceDML struct {
	File       string
	Line       int
	Symbol     string
	Operation  string
	ObjectType string
}

// BuildSourceIndex builds a project-wide index of source symbols useful for
// matching debug-log evidence to local files.
func BuildSourceIndex(index typesys.Index) SourceIndex {
	sourceIndex := SourceIndex{
		methodsByFile:       make(map[string][]sourceMethod),
		methodsBySymbol:     make(map[string][]sourceMethod),
		debugLiteralsByKey:  make(map[string][]sourceDebugLiteral),
		soqlByKey:           make(map[string][]sourceSoqlQuery),
		dmlByKey:            make(map[string][]sourceDML),
		debugLiteralsByFile: make(map[string][]sourceDebugLiteral),
		soqlByFile:          make(map[string][]sourceSoqlQuery),
		dmlByFile:           make(map[string][]sourceDML),
	}

	methodsByFile := make(map[string][]sourceMethod)
	for _, typ := range index.Types {
		if typ.File == "" {
			continue
		}
		file := filepath.Clean(typ.File)
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod && member.Kind != apexast.DeclarationConstructor {
				continue
			}
			method := sourceMethod{
				Namespace: typ.Namespace,
				TypeName:  typ.Name,
				Name:      member.Name,
				File:      file,
				StartLine: member.Range.Start.Line,
				EndLine:   member.Range.End.Line,
			}
			if method.StartLine == 0 {
				method.StartLine = typ.Range.Start.Line
			}
			if method.EndLine == 0 {
				method.EndLine = typ.Range.End.Line
			}
			methodsByFile[file] = append(methodsByFile[file], method)
			addMethodLookup(&sourceIndex, method)
			sourceIndex.methods = append(sourceIndex.methods, method)
		}
	}

	for file, methods := range methodsByFile {
		methods = dedupeMethods(methods)
		methodsByFile[file] = methods
		sourceIndex.methodsByFile[file] = methods

		source, readErr := os.ReadFile(file)
		if readErr != nil {
			continue
		}
		parser := apexast.NewParser()
		astFile := parser.ParseSourceAST(file, string(source))
		varLines := variableTypesByMethod(linesOfSource(source), methods, methodByLineMatcher(methods))
		collectByAST(&sourceIndex, file, string(source), methods, varLines, astFile.Nodes)
	}

	return sourceIndex
}

func addMethodLookup(target *SourceIndex, method sourceMethod) {
	for _, key := range methodLookupKeys(method.Namespace, method.TypeName, method.Name) {
		target.methodsBySymbol[key] = append(target.methodsBySymbol[key], method)
	}
}

func collectByAST(
	target *SourceIndex,
	file, source string,
	methods []sourceMethod,
	varTypes map[string]map[string]string,
	nodes []apexast.ASTNode,
) {
	for _, node := range nodes {
		switch node.Kind {
		case "method_invocation":
			methodName, receiver := methodInvocationParts(node, source)
			if methodName == "" || receiver == nil {
				break
			}
			if !strings.EqualFold(methodName, "debug") {
				break
			}
			if strings.TrimSpace(nodeText(source, *receiver)) != "System" {
				break
			}
			method := methodByLine(methods, node.Range.Start.Line)
			if method.Name == "" {
				break
			}
			args := invocationArgumentStrings(node, source)
			for _, arg := range args {
				for _, key := range methodLookupKeys(method.Namespace, method.TypeName, method.Name) {
					target.debugLiteralsByKey[key] = append(target.debugLiteralsByKey[key], sourceDebugLiteral{
						File:       file,
						Line:       node.Range.Start.Line,
						Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
						Message:    arg,
						Normalized: normalizeForMatch(arg),
					})
					target.debugLiteralsByFile[file] = append(target.debugLiteralsByFile[file], sourceDebugLiteral{
						File:       file,
						Line:       node.Range.Start.Line,
						Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
						Message:    arg,
						Normalized: normalizeForMatch(arg),
					})
				}
			}
		case "query_expression":
			method := methodByLine(methods, node.Range.Start.Line)
			query := strings.TrimSpace(nodeText(source, node))
			if query == "" {
				break
			}
			if method.Name != "" {
				for _, key := range methodLookupKeys(method.Namespace, method.TypeName, method.Name) {
					target.soqlByKey[key] = append(target.soqlByKey[key], sourceSoqlQuery{
						File:       file,
						Line:       node.Range.Start.Line,
						Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
						Query:      query,
						Normalized: normalizeQuery(query),
						FromObject: parseFromObject(query),
					})
					target.soqlByFile[file] = append(target.soqlByFile[file], sourceSoqlQuery{
						File:       file,
						Line:       node.Range.Start.Line,
						Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
						Query:      query,
						Normalized: normalizeQuery(query),
						FromObject: parseFromObject(query),
					})
				}
			}
		case "dml_expression":
			method := methodByLine(methods, node.Range.Start.Line)
			if method.Name == "" {
				break
			}
			text := strings.TrimSpace(nodeText(source, node))
			op, targetExpr := parseDMLExpression(text)
			objectType := inferDMLObject(method.Namespace, method.TypeName, method.Name, targetExpr, varTypes)
			for _, key := range methodLookupKeys(method.Namespace, method.TypeName, method.Name) {
				target.dmlByKey[key] = append(target.dmlByKey[key], sourceDML{
					File:       file,
					Line:       node.Range.Start.Line,
					Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
					Operation:  op,
					ObjectType: objectType,
				})
				target.dmlByFile[file] = append(target.dmlByFile[file], sourceDML{
					File:       file,
					Line:       node.Range.Start.Line,
					Symbol:     methodSymbol(method.Namespace, method.TypeName, method.Name),
					Operation:  op,
					ObjectType: objectType,
				})
			}
		}
		for _, child := range node.Children {
			collectByAST(target, file, source, methods, varTypes, []apexast.ASTNode{child})
		}
	}
}

func linesOfSource(source []byte) []string {
	return strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n")
}

func methodByLine(methods []sourceMethod, line int) sourceMethod {
	var out sourceMethod
	bestLen := int(^uint(0) >> 1)
	for _, method := range methods {
		if method.StartLine <= 0 || method.EndLine <= 0 {
			continue
		}
		if line < method.StartLine || line > method.EndLine {
			continue
		}
		length := method.EndLine - method.StartLine
		if out.Name == "" || length < bestLen {
			out = method
			bestLen = length
		}
	}
	return out
}

func variableTypesByMethod(lines []string, methods []sourceMethod, byLine func(int) sourceMethod) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for i, line := range lines {
		lineNum := i + 1
		method := byLine(lineNum)
		if method.Name == "" {
			continue
		}
		matches := varDeclarationRe.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		typeName := strings.TrimSpace(matches[1])
		varName := strings.ToLower(strings.TrimSpace(matches[2]))
		if typeName == "" || varName == "" {
			continue
		}
		key := methodLookupKey(method.Namespace, method.TypeName, method.Name)
		vars := out[key]
		if vars == nil {
			vars = make(map[string]string)
			out[key] = vars
		}
		vars[varName] = normalizeObjectType(typeName)
	}
	return out
}

func methodLookupKeys(namespace, typeName, methodName string) []string {
	keys := make([]string, 0, 3)
	base := methodLookupKey(namespace, typeName, methodName)
	if base != "" {
		keys = append(keys, base)
	}
	if namespace != "" {
		noNamespace := methodLookupKey("", typeName, methodName)
		if noNamespace != "" && noNamespace != base {
			keys = append(keys, noNamespace)
		}
	}
	// Keep namespace-only and no-namespace variants aligned for mixed fixture inputs.
	onlyType := methodLookupKey("", normalizeType(typeName), methodName)
	if onlyType != "" && onlyType != base && onlyType != methodLookupKey("", typeName, methodName) {
		keys = append(keys, onlyType)
	}
	return dedupeStrings(keys)
}

func methodLookupKey(namespace, typeName, methodName string) string {
	ns := strings.ToLower(strings.TrimSpace(namespace))
	t := normalizeType(typeName)
	m := strings.ToLower(strings.TrimSpace(methodName))
	if t == "" || m == "" {
		return ""
	}
	if ns == "" {
		return t + "|" + m
	}
	return ns + "|" + t + "|" + m
}

func methodBySymbol(index SourceIndex, namespace, typeName, methodName string) []sourceMethod {
	seen := make(map[string]struct{})
	methods := append([]sourceMethod{}, index.methodsBySymbol[methodLookupKey(namespace, typeName, methodName)]...)
	if namespace == "" {
		return dedupeMethods(methods)
	}
	for _, fallback := range []string{
		methodLookupKey("", typeName, methodName),
		methodLookupKey("", normalizeType(typeName), methodName),
	} {
		for _, method := range index.methodsBySymbol[fallback] {
			key := methodKey(method)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			methods = append(methods, method)
		}
	}
	return dedupeMethods(methods)
}

func methodSymbol(namespace, typeName, methodName string) string {
	ns := strings.TrimSpace(namespace)
	typeText := normalizeType(typeName)
	if ns == "" {
		return typeText + "." + methodName
	}
	return ns + "." + typeText + "." + methodName
}

func dedupeMethods(methods []sourceMethod) []sourceMethod {
	seen := make(map[string]struct{}, len(methods))
	out := methods[:0]
	for _, method := range methods {
		key := methodKey(method)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, method)
	}
	return out
}

func methodKey(method sourceMethod) string {
	return strings.ToLower(strings.TrimSpace(method.Namespace)) + "|" + normalizeType(method.TypeName) + "|" + strings.ToLower(strings.TrimSpace(method.Name)) + "|" + filepath.Clean(method.File)
}

func parseDMLExpression(text string) (string, string) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", ""
	}
	op := strings.ToLower(fields[0])
	if len(fields) < 2 {
		return op, ""
	}
	target := strings.TrimSuffix(strings.TrimSpace(fields[1]), ";")
	return op, target
}

func inferDMLObject(namespace, typeName, methodName, target string, byMethod map[string]map[string]string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	key := methodLookupKey(namespace, typeName, methodName)
	if vars, ok := byMethod[key]; ok {
		if value, found := vars[strings.ToLower(target)]; found && value != "" {
			return value
		}
	}
	if vars, ok := byMethod[methodLookupKey("", typeName, methodName)]; ok {
		if value, found := vars[strings.ToLower(target)]; found && value != "" {
			return value
		}
	}
	normalized := normalizeType(target)
	if normalized != "" && normalized != target {
		return normalized
	}
	return target
}

func methodByLineMatcher(methods []sourceMethod) func(int) sourceMethod {
	return func(line int) sourceMethod {
		return methodByLine(methods, line)
	}
}

func methodInvocationParts(node apexast.ASTNode, source string) (method string, receiver *apexast.ASTNode) {
	if node.Kind != "method_invocation" {
		return "", nil
	}
	methodIndex := -1
	for i := len(node.Children) - 1; i >= 0; i-- {
		if node.Children[i].Kind == "argument_list" {
			methodIndex = i - 1
			break
		}
	}
	if methodIndex < 0 {
		methodIndex = len(node.Children) - 2
	}
	if methodIndex < 0 || methodIndex >= len(node.Children) {
		return "", nil
	}
	methodNode := node.Children[methodIndex]
	if methodNode.Kind != "identifier" {
		return "", nil
	}
	method = strings.TrimSpace(nodeText(source, methodNode))
	if method == "" {
		return "", nil
	}
	if methodIndex == 0 {
		return method, nil
	}
	return method, &node.Children[methodIndex-1]
}

func invocationArgumentStrings(node apexast.ASTNode, source string) []string {
	for _, child := range node.Children {
		if child.Kind != "argument_list" {
			continue
		}
		args := stringLiteralsFromNode(child, source)
		if len(args) > 0 {
			return args
		}
		text := nodeText(source, child)
		for _, match := range stringLiteralMatches.FindAllStringSubmatch(text, -1) {
			for i := 1; i < len(match); i++ {
				if match[i] == "" {
					continue
				}
				unquoted := unquoteString(match[i])
				if unquoted != "" {
					args = append(args, unquoted)
				}
			}
		}
		if len(args) > 0 {
			return args
		}
	}
	return nil
}

var stringLiteralMatches = regexp.MustCompile(`(?s)("([^"\\]|\\.)*"|'([^'\\]|\\.)*')`)

func stringLiteralsFromNode(node apexast.ASTNode, source string) []string {
	out := make([]string, 0, 4)
	if node.Kind == "string_literal" || node.Kind == "string" {
		out = append(out, unquoteString(strings.TrimSpace(nodeText(source, node))))
	}
	for _, child := range node.Children {
		out = append(out, stringLiteralsFromNode(child, source)...)
	}
	return out
}

func unquoteString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > 1 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
		unquoted, err := strconv.Unquote(value)
		if err == nil {
			return unquoted
		}
		return strings.Trim(value[1:len(value)-1], "\"'")
	}
	return value
}

func nodeText(source string, node apexast.ASTNode) string {
	start := node.Range.Start.Offset
	end := node.Range.End.Offset
	if start < 0 || end < start || end > len(source) {
		return ""
	}
	return source[start:end]
}

func parseFromObject(query string) string {
	match := fromClauseRe.FindStringSubmatch(strings.ToLower(query))
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

var fromClauseRe = regexp.MustCompile(`(?i)\bfrom\s+([a-zA-Z_][a-zA-Z0-9_]*)`)

func normalizeType(name string) string {
	if name == "" {
		return ""
	}
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		return strings.TrimSpace(strings.ToLower(parts[len(parts)-1]))
	}
	return strings.TrimSpace(name)
}

func normalizeForMatch(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var out strings.Builder
	inSingle := false
	inDouble := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch ch {
		case '\'':
			out.WriteByte(ch)
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			out.WriteByte(ch)
			if !inSingle {
				inDouble = !inDouble
			}
		case ' ', '\t', '\n', '\r':
			if out.Len() == 0 || out.String()[out.Len()-1] == ' ' {
				continue
			}
			out.WriteByte(' ')
		default:
			if inSingle || inDouble {
				out.WriteByte(ch)
			} else {
				out.WriteByte(byte(strings.ToLower(string(ch))[0]))
			}
		}
	}
	normalized := strings.TrimSpace(out.String())
	if normalized == "" {
		return ""
	}
	return normalized
}

func normalizeQuery(query string) string {
	return normalizeForMatch(query)
}

func normalizeObjectType(value string) string {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "<") {
		open := strings.Index(value, "<")
		close := strings.Index(value, ">")
		if open >= 0 && close > open {
			value = value[open+1 : close]
		}
	}
	value = strings.TrimSuffix(strings.TrimSuffix(value, "[]"), "{}")
	return strings.TrimSpace(value)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
