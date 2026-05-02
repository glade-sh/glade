package sema

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/typesys"
)

type Result struct {
	Project     typesys.ProjectInfo      `json:"project"`
	Summary     Summary                  `json:"summary"`
	Diagnostics []diagnostic.Diagnostic  `json:"diagnostics,omitempty"`
	Types       map[string]TypeReference `json:"types,omitempty"`
}

type Summary struct {
	Types       int `json:"types"`
	Triggers    int `json:"triggers"`
	Objects     int `json:"objects"`
	Diagnostics int `json:"diagnostics"`
}

type TypeReference struct {
	Name   string   `json:"name"`
	Kind   TypeKind `json:"kind"`
	Source string   `json:"source,omitempty"`
}

type TypeKind string

const (
	TypeApex     TypeKind = "apex"
	TypeSchema   TypeKind = "schema"
	TypePlatform TypeKind = "platform"
	TypeBuiltin  TypeKind = "builtin"
)

type Analyzer struct {
	known     map[string]TypeReference
	namespace string
}

func NewAnalyzer() *Analyzer {
	a := &Analyzer{known: make(map[string]TypeReference)}
	for _, name := range builtinTypes {
		a.addKnown(name, TypeBuiltin, "")
	}
	for _, name := range platformTypes {
		a.addKnown(name, TypePlatform, "")
	}
	return a
}

func Analyze(index typesys.Index) Result {
	return NewAnalyzer().Analyze(index)
}

func (a *Analyzer) Analyze(index typesys.Index) Result {
	a.namespace = index.Project.Namespace
	result := Result{
		Project:     index.Project,
		Diagnostics: append([]diagnostic.Diagnostic{}, index.Diagnostics...),
	}

	for _, object := range index.Objects {
		a.addKnown(object.Name, TypeSchema, "")
	}
	for _, typ := range index.Types {
		a.addKnown(typ.Name, TypeApex, typ.File)
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationClass || member.Kind == apexast.DeclarationInterface || member.Kind == apexast.DeclarationEnum {
				a.addKnown(member.Name, TypeApex, typ.File)
				a.addKnown(typ.Name+"."+member.Name, TypeApex, typ.File)
			}
		}
	}

	result.Diagnostics = append(result.Diagnostics, a.checkTriggers(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkMemberTypes(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkMethodParameters(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkMethodBodies(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkVisibility(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkSchemaReferences(index)...)

	result.Summary = Summary{
		Types:       len(index.Types),
		Triggers:    len(index.Triggers),
		Objects:     len(index.Objects),
		Diagnostics: len(result.Diagnostics),
	}
	result.Types = a.exportKnownTypes()
	return result
}

func (r Result) HasErrors() bool {
	for _, diag := range r.Diagnostics {
		if diag.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func (a *Analyzer) checkTriggers(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, trigger := range index.Triggers {
		if trigger.ObjectName == "" || a.hasKnown(trigger.ObjectName) {
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "OAERSEMA001",
			Message:  fmt.Sprintf("trigger %q references unknown SObject %q", trigger.Name, trigger.ObjectName),
			File:     trigger.File,
			Range:    &trigger.Range,
		})
	}
	return diagnostics
}

func (a *Analyzer) checkMemberTypes(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		for _, member := range typ.Members {
			if member.Type == "" || member.Type == "void" {
				continue
			}
			for _, ref := range extractTypeNames(member.Type) {
				if a.hasKnown(ref) {
					continue
				}
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA002",
					Message:  fmt.Sprintf("%s %q references unknown type %q", member.Kind, member.Name, ref),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
	}
	return diagnostics
}

func (a *Analyzer) checkMethodParameters(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod && member.Kind != apexast.DeclarationConstructor {
				continue
			}
			for _, param := range member.Parameters {
				for _, ref := range extractTypeNames(param.Type) {
					if a.hasKnown(ref) {
						continue
					}
					diagnostics = append(diagnostics, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     "OAERSEMA004",
						Message:  fmt.Sprintf("%s %q parameter %q references unknown type %q", member.Kind, member.Name, param.Name, ref),
						File:     typ.File,
						Range:    &param.Range,
					})
				}
			}
		}
	}
	return diagnostics
}

func (a *Analyzer) checkSchemaReferences(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, object := range index.Objects {
		for _, field := range object.Fields {
			if field.ReferenceTo == "" || a.hasKnown(field.ReferenceTo) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA003",
				Message:  fmt.Sprintf("field %s.%s references unknown SObject %q", object.Name, field.Name, field.ReferenceTo),
			})
		}
	}
	return diagnostics
}

func (a *Analyzer) checkVisibility(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if hasAnyModifier(typ.Modifiers, "public", "global") {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA005",
				Message:  fmt.Sprintf("%s %q cannot be both public and global", typ.Kind, typ.Name),
				File:     typ.File,
				Range:    &typ.Range,
			})
		}
		if typ.Kind != apexast.DeclarationInterface {
			continue
		}
		for _, member := range typ.Members {
			if hasModifier(member.Modifiers, "private") || hasModifier(member.Modifiers, "protected") {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA005",
					Message:  fmt.Sprintf("interface method %q cannot be private or protected", member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
	}
	return diagnostics
}

type typeMembers struct {
	methods map[string][]typesys.MemberSymbol
	fields  map[string]typesys.MemberSymbol
}

func (a *Analyzer) checkMethodBodies(index typesys.Index) []diagnostic.Diagnostic {
	model := buildTypeMembers(index)
	sources := make(map[string]string)
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		for _, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationMethod, apexast.DeclarationConstructor, apexast.DeclarationInitializer:
				source, ok := readSemaSource(typ.File, sources)
				if !ok {
					continue
				}
				body, ok := extractBodyForSema(source, member.Range)
				if !ok {
					continue
				}
				diagnostics = append(diagnostics, a.checkBodyText(typ, member, body, model)...)
			case apexast.DeclarationProperty:
				for _, accessor := range member.Accessors {
					if !accessor.HasBody {
						continue
					}
					source, ok := readSemaSource(typ.File, sources)
					if !ok {
						continue
					}
					body, ok := extractBodyForSema(source, accessor.Range)
					if !ok {
						continue
					}
					accessorMember := member
					accessorMember.Kind = apexast.DeclarationMethod
					accessorMember.Name = member.Name + "." + accessor.Kind
					if accessor.Kind == "set" {
						accessorMember.Parameters = []apexast.Parameter{{Name: "value", Type: member.Type}}
					}
					diagnostics = append(diagnostics, a.checkBodyText(typ, accessorMember, body, model)...)
				}
			}
		}
	}
	return diagnostics
}

func readSemaSource(path string, cache map[string]string) (string, bool) {
	if path == "" {
		return "", false
	}
	if source, ok := cache[path]; ok {
		return source, true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	source := string(data)
	cache[path] = source
	return source, true
}

func buildTypeMembers(index typesys.Index) map[string]typeMembers {
	out := make(map[string]typeMembers)
	for _, typ := range index.Types {
		members := typeMembers{methods: make(map[string][]typesys.MemberSymbol), fields: make(map[string]typesys.MemberSymbol)}
		for _, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationMethod, apexast.DeclarationConstructor:
				members.methods[normalizeName(member.Name)] = append(members.methods[normalizeName(member.Name)], member)
			case apexast.DeclarationField, apexast.DeclarationProperty:
				members.fields[normalizeName(member.Name)] = member
			}
		}
		out[normalizeName(typ.Name)] = members
		if index.Project.Namespace != "" {
			out[normalizeName(index.Project.Namespace+"."+typ.Name)] = members
		}
	}
	return out
}

func (a *Analyzer) checkBodyText(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, model map[string]typeMembers) []diagnostic.Diagnostic {
	scope := make(map[string]string)
	for _, param := range member.Parameters {
		scope[normalizeName(param.Name)] = param.Type
	}
	if fields, ok := model[normalizeName(typ.Name)]; ok {
		for name, field := range fields.fields {
			scope[name] = field.Type
		}
	}
	var diagnostics []diagnostic.Diagnostic
	for _, match := range localDeclPattern.FindAllStringSubmatch(body, -1) {
		typeName, name := strings.TrimSpace(match[1]), strings.TrimSpace(match[2])
		if isSemaKeyword(typeName) {
			continue
		}
		for _, ref := range extractTypeNames(typeName) {
			if !a.hasKnown(ref) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA006",
					Message:  fmt.Sprintf("%s %q declares local %q with unknown type %q", member.Kind, member.Name, name, ref),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
		scope[normalizeName(name)] = typeName
	}
	for _, typeName := range constructorTypes(body) {
		for _, ref := range extractTypeNames(typeName) {
			if !a.hasKnown(ref) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA006",
					Message:  fmt.Sprintf("%s %q constructs unknown type %q", member.Kind, member.Name, ref),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
	}
	diagnostics = append(diagnostics, a.checkBodyAssignments(typ, member, body, scope)...)
	diagnostics = append(diagnostics, a.checkBodyCalls(typ, member, body, scope, model)...)
	return diagnostics
}

func (a *Analyzer) checkBodyAssignments(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, scope map[string]string) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, match := range assignmentPattern.FindAllStringSubmatch(body, -1) {
		target := strings.TrimSpace(match[1])
		if _, ok := scope[normalizeName(target)]; ok {
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "OAERSEMA007",
			Message:  fmt.Sprintf("%s %q assigns unknown variable %q", member.Kind, member.Name, target),
			File:     typ.File,
			Range:    &member.Range,
		})
	}
	return diagnostics
}

func (a *Analyzer) checkBodyCalls(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, scope map[string]string, model map[string]typeMembers) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, match := range callPattern.FindAllStringSubmatchIndex(body, -1) {
		callee := strings.TrimSpace(body[match[2]:match[3]])
		if skipSemaCall(callee) {
			continue
		}
		args, haveArgs := callArgumentsAt(body, match[3])
		if strings.Contains(callee, ".") {
			receiver, method, ok := strings.Cut(callee, ".")
			if !ok || method == "" {
				continue
			}
			if classMembers, ok := model[normalizeName(receiver)]; ok {
				diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, classMembers.methods[normalizeName(method)], args, haveArgs, scope)...)
				continue
			}
			receiverType, ok := scope[normalizeName(receiver)]
			if !ok || isSemaBuiltinType(receiverType) {
				continue
			}
			if classMembers, ok := model[normalizeName(receiverType)]; ok {
				diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, classMembers.methods[normalizeName(method)], args, haveArgs, scope)...)
			}
			continue
		}
		if a.hasKnown(callee) {
			continue
		}
		classMembers, ok := model[normalizeName(typ.Name)]
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, classMembers.methods[normalizeName(callee)], args, haveArgs, scope)...)
	}
	return diagnostics
}

func (a *Analyzer) diagnoseMethodCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, candidates []typesys.MemberSymbol, args []string, haveArgs bool, scope map[string]string) []diagnostic.Diagnostic {
	if len(candidates) == 0 {
		return []diagnostic.Diagnostic{unknownCallDiagnostic(typ, member, callee)}
	}
	if !haveArgs {
		return nil
	}
	for _, candidate := range candidates {
		if len(candidate.Parameters) != len(args) {
			continue
		}
		if callArgsMatch(candidate.Parameters, args, scope) {
			return nil
		}
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA009",
		Message:  fmt.Sprintf("%s %q has no matching overload for call %q with %d argument(s)", member.Kind, member.Name, callee, len(args)),
		File:     typ.File,
		Range:    &member.Range,
	}}
}

func callArgsMatch(params []apexast.Parameter, args []string, scope map[string]string) bool {
	for i, arg := range args {
		argType := inferSemaArgType(arg, scope)
		if argType == "" || argType == "null" {
			continue
		}
		paramType := params[i].Type
		if strings.EqualFold(paramType, argType) || strings.EqualFold(paramType, "Object") {
			continue
		}
		return false
	}
	return true
}

func unknownCallDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA008",
		Message:  fmt.Sprintf("%s %q calls unknown method %q", member.Kind, member.Name, callee),
		File:     typ.File,
		Range:    &member.Range,
	}
}

func (a *Analyzer) addKnown(name string, kind TypeKind, source string) {
	if name == "" {
		return
	}
	key := normalizeName(name)
	if _, exists := a.known[key]; exists {
		return
	}
	a.known[key] = TypeReference{Name: name, Kind: kind, Source: source}
}

func (a *Analyzer) hasKnown(name string) bool {
	if name == "" {
		return true
	}
	if _, ok := a.known[normalizeName(name)]; ok {
		return true
	}
	if a.namespace != "" {
		prefix := normalizeName(a.namespace) + "."
		normalized := normalizeName(name)
		if strings.HasPrefix(normalized, prefix) {
			if _, ok := a.known[strings.TrimPrefix(normalized, prefix)]; ok {
				return true
			}
		}
	}
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		_, ok := a.known[normalizeName(parts[0])]
		return ok
	}
	return false
}

func hasAnyModifier(modifiers []string, left, right string) bool {
	return hasModifier(modifiers, left) && hasModifier(modifiers, right)
}

func hasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(modifier, expected) {
			return true
		}
	}
	return false
}

func (a *Analyzer) exportKnownTypes() map[string]TypeReference {
	keys := make([]string, 0, len(a.known))
	for key := range a.known {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make(map[string]TypeReference, len(keys))
	for _, key := range keys {
		ref := a.known[key]
		out[ref.Name] = ref
	}
	return out
}

var (
	typeIdentifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*`)
	localDeclPattern      = regexp.MustCompile(`(?m)(?:^|[;{}\n])\s*([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|;)`)
	assignmentPattern     = regexp.MustCompile(`(?m)(?:^|[;{}\n])\s*([A-Za-z_][A-Za-z0-9_]*)\s=`)
	callPattern           = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s*\(`)
	constructorPattern    = regexp.MustCompile(`\bnew\s+([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?)\s*\(`)
	intLiteralPattern     = regexp.MustCompile(`^-?[0-9]+$`)
)

func extractBodyForSema(source string, r diagnostic.Range) (string, bool) {
	start := r.Start.Offset
	end := r.End.Offset
	if start < 0 || start >= len(source) || end <= start || end > len(source) {
		return "", false
	}
	text := source[start:end]
	open := strings.IndexByte(text, '{')
	if open < 0 {
		return "", false
	}
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '\'':
			i = skipSemaString(text, i)
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[open+1 : i], true
			}
		}
	}
	return "", false
}

func skipSemaString(source string, start int) int {
	for i := start + 1; i < len(source); i++ {
		if source[i] == '\'' {
			if i+1 < len(source) && source[i+1] == '\'' {
				i++
				continue
			}
			return i
		}
	}
	return len(source) - 1
}

func constructorTypes(body string) []string {
	matches := constructorPattern.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, strings.TrimSpace(match[1]))
	}
	return out
}

func callArgumentsAt(body string, calleeEnd int) ([]string, bool) {
	open := strings.IndexByte(body[calleeEnd:], '(')
	if open < 0 {
		return nil, false
	}
	open += calleeEnd
	depth := 0
	start := open + 1
	var args []string
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				if strings.TrimSpace(body[start:i]) != "" {
					args = append(args, strings.TrimSpace(body[start:i]))
				}
				return args, true
			}
		case ',':
			if depth == 1 {
				args = append(args, strings.TrimSpace(body[start:i]))
				start = i + 1
			}
		}
	}
	return nil, false
}

func inferSemaArgType(arg string, scope map[string]string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return ""
	}
	if strings.HasPrefix(arg, "'") {
		return "String"
	}
	if arg == "true" || arg == "false" {
		return "Boolean"
	}
	if arg == "null" {
		return "null"
	}
	if intLiteralPattern.MatchString(arg) {
		return "Integer"
	}
	if typ, ok := scope[normalizeName(arg)]; ok {
		return typ
	}
	return ""
}

func skipSemaCall(callee string) bool {
	switch normalizeName(callee) {
	case "if", "for", "while", "switch", "catch", "new", "return", "system.assert", "system.assertequals", "system.debug":
		return true
	default:
		return false
	}
}

func isSemaKeyword(text string) bool {
	switch normalizeName(text) {
	case "return", "throw", "if", "for", "while", "switch", "catch", "else", "do":
		return true
	default:
		return false
	}
}

func isSemaBuiltinType(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if i := strings.IndexByte(typeName, '<'); i >= 0 {
		typeName = typeName[:i]
	}
	if strings.Contains(typeName, ".") {
		return false
	}
	for _, known := range builtinTypes {
		if strings.EqualFold(typeName, known) {
			return true
		}
	}
	for _, known := range platformTypes {
		if strings.EqualFold(typeName, known) {
			return true
		}
	}
	return false
}

func extractTypeNames(typeRef string) []string {
	matches := typeIdentifierPattern.FindAllString(typeRef, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if isCollectionType(match) {
			continue
		}
		out = append(out, match)
	}
	return out
}

func isCollectionType(name string) bool {
	switch normalizeName(name) {
	case "list", "set", "map":
		return true
	default:
		return false
	}
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (r Result) MarshalJSON() ([]byte, error) {
	type alias Result
	if len(r.Types) > 200 {
		r.Types = nil
	}
	return json.Marshal(alias(r))
}

var builtinTypes = []string{
	"void",
	"Object",
	"SObject",
	"Boolean",
	"boolean",
	"Integer",
	"Long",
	"Double",
	"Decimal",
	"String",
	"string",
	"Id",
	"Blob",
	"Date",
	"Datetime",
	"Time",
	"Type",
	"Exception",
}

var platformTypes = []string{
	"Account",
	"AggregateResult",
	"ApexPages",
	"AsyncApexJob",
	"BatchApexErrorEvent",
	"BrandTemplate",
	"Cache",
	"Cache.OrgPartition",
	"Callable",
	"Component",
	"Component.Apex.Column",
	"Component.Apex.OutputPanel",
	"Component.Apex.outputPanel",
	"CronTrigger",
	"Database",
	"Database.QueryLocator",
	"DescribeSObjectResult",
	"Dom",
	"Dom.Document",
	"Dom.XmlNode",
	"Dom.XmlNodeType",
	"EmailTemplate",
	"EntityDefinition",
	"FieldPermissions",
	"HttpCalloutMock",
	"HTTPResponse",
	"HttpResponse",
	"Iterable",
	"Matcher",
	"Messaging",
	"ObjectPermissions",
	"OrgWideEmailAddress",
	"Organization",
	"PageReference",
	"Pattern",
	"PermissionSetAssignment",
	"Profile",
	"Queueable",
	"RecentlyViewed",
	"RestRequest",
	"RestResponse",
	"Schedulable",
	"SelectOption",
	"Schema",
	"Schema.ChildRelationship",
	"Schema.DescribeSObjectResult",
	"Schema.FieldSet",
	"Schema.RecordTypeInfo",
	"Schema.SObjectField",
	"Schema.SObjectType",
	"SObjectField",
	"SObjectType",
	"StaticResource",
	"System",
	"Test",
	"TriggerOperation",
	"User",
	"UserLicense",
	"VisualEditor",
	"VisualEditor.DataRow",
	"VisualEditor.DynamicPickListRows",
}
