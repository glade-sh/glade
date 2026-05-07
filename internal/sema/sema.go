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
	"github.com/open-aer/oaer/internal/ir"
	"github.com/open-aer/oaer/internal/soql"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/vm"
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
	for _, name := range vm.CommonSObjectTypeNames() {
		a.addKnown(name, TypePlatform, "")
	}
	return a
}

func Analyze(index typesys.Index) Result {
	return NewAnalyzer().Analyze(index)
}

func (a *Analyzer) Analyze(index typesys.Index) (result Result) {
	a.namespace = index.Project.Namespace
	result = Result{
		Project:     index.Project,
		Diagnostics: append([]diagnostic.Diagnostic{}, index.Diagnostics...),
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA000",
				Message:  fmt.Sprintf("internal sema panic: %v", recovered),
			})
			result.Summary.Diagnostics = len(result.Diagnostics)
		}
	}()

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
	result.Diagnostics = append(result.Diagnostics, a.checkAnnotations(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkMethodBodies(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkVisibility(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkInheritanceContracts(index)...)
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
			for _, referenceTo := range field.ReferenceTo {
				if referenceTo == "" || a.hasKnown(referenceTo) {
					continue
				}
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA003",
					Message:  fmt.Sprintf("field %s.%s references unknown SObject %q", object.Name, field.Name, referenceTo),
				})
			}
		}
	}
	return diagnostics
}

func (a *Analyzer) checkAnnotations(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if hasModifier(typ.Modifiers, "RestResource") && typ.Kind != apexast.DeclarationClass {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, typ.Range, "RestResource is only valid on classes"))
		}
		for _, member := range typ.Members {
			diagnostics = append(diagnostics, checkMemberAnnotations(typ, member)...)
		}
	}
	return diagnostics
}

func checkMemberAnnotations(typ typesys.TypeSymbol, member typesys.MemberSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if hasModifier(member.Modifiers, "TestSetup") {
		if member.Kind != apexast.DeclarationMethod || !hasModifier(typ.Modifiers, "IsTest") || !hasModifier(member.Modifiers, "static") || !strings.EqualFold(member.Type, "void") || len(member.Parameters) != 0 {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "TestSetup methods must be static void no-arg methods inside IsTest classes"))
		}
	}
	if hasModifier(member.Modifiers, "future") {
		if member.Kind != apexast.DeclarationMethod || !hasModifier(member.Modifiers, "static") || !strings.EqualFold(member.Type, "void") {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "future methods must be static void methods"))
		}
	}
	if hasAnyAnnotation(member.Modifiers, "HttpDelete", "HttpGet", "HttpPatch", "HttpPost", "HttpPut") {
		if member.Kind != apexast.DeclarationMethod {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "HTTP method annotations are only valid on methods"))
		}
		if !hasModifier(typ.Modifiers, "RestResource") {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "HTTP method annotations require a RestResource class"))
		}
	}
	if hasModifier(member.Modifiers, "InvocableMethod") {
		if member.Kind != apexast.DeclarationMethod || !hasModifier(member.Modifiers, "static") {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "InvocableMethod is only valid on static methods"))
		}
	}
	if hasModifier(member.Modifiers, "InvocableVariable") {
		if member.Kind != apexast.DeclarationField && member.Kind != apexast.DeclarationProperty {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "InvocableVariable is only valid on fields and properties"))
		}
	}
	return diagnostics
}

func annotationDiagnostic(file string, rng diagnostic.Range, detail string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA026",
		Message:  "invalid annotation usage: " + detail,
		File:     file,
		Range:    &rng,
	}
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

func (a *Analyzer) checkInheritanceContracts(index typesys.Index) []diagnostic.Diagnostic {
	model := buildTypeMembers(index)
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		abstractClass := hasModifier(typ.Modifiers, "abstract")
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod {
				continue
			}
			if hasModifier(member.Modifiers, "override") && !hasInheritedMethodSignature(model, typ, member) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA016",
					Message:  fmt.Sprintf("method %q is marked override but no inherited method has the same signature", member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
			if hasModifier(member.Modifiers, "abstract") && !abstractClass {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA017",
					Message:  fmt.Sprintf("concrete class %q declares abstract method %q", typ.Name, member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
		if abstractClass {
			continue
		}
		required := requiredMethodSignatures(model, typ)
		for _, requirement := range required {
			if hasConcreteMethodSignature(model, typ.Name, requirement.member) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA017",
				Message:  fmt.Sprintf("concrete class %q must implement %s method %q from %q", typ.Name, requirement.sourceKind, requirement.member.Name, requirement.owner),
				File:     typ.File,
				Range:    &typ.Range,
			})
		}
	}
	return diagnostics
}

func hasInheritedMethodSignature(model map[string]typeMembers, typ typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if isObjectOverrideSignature(member) {
		return true
	}
	if hasPlatformInheritedMethodSignature(typ, member) {
		return true
	}
	for _, candidate := range resolveMemberMethods(model, typ.SuperClass, member.Name) {
		if sameSemaSignature(candidate.member, member) {
			return true
		}
	}
	for _, iface := range typ.Interfaces {
		for _, candidate := range resolveMemberMethods(model, iface, member.Name) {
			if sameSemaSignature(candidate.member, member) {
				return true
			}
		}
	}
	return false
}

func hasPlatformInheritedMethodSignature(typ typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	superClass := normalizeName(typ.SuperClass)
	name := normalizeName(member.Name)
	switch superClass {
	case "visualeditor.dynamicpicklist":
		return len(member.Parameters) == 0 && (name == "getdefaultvalue" || name == "getvalues")
	case "metadata.deploycallbackcontext":
		return len(member.Parameters) == 0 && name == "getcallbackjobid"
	default:
		return false
	}
}

func isObjectOverrideSignature(member typesys.MemberSymbol) bool {
	if len(member.Parameters) != 0 {
		return false
	}
	switch normalizeName(member.Name) {
	case "tostring", "hashcode":
		return true
	default:
		return false
	}
}

type methodRequirement struct {
	owner      string
	sourceKind string
	member     typesys.MemberSymbol
}

func requiredMethodSignatures(model map[string]typeMembers, typ typesys.TypeSymbol) []methodRequirement {
	var out []methodRequirement
	seen := make(map[string]bool)
	for _, iface := range typ.Interfaces {
		out = append(out, collectRequiredMethods(model, iface, "interface", seen)...)
	}
	for current := typ.SuperClass; current != ""; {
		members, ok := model[normalizeName(current)]
		if !ok {
			break
		}
		for _, overloads := range members.methods {
			for _, method := range overloads {
				if hasModifier(method.Modifiers, "abstract") {
					key := methodSignatureKey(method)
					if !seen[key] {
						seen[key] = true
						out = append(out, methodRequirement{owner: members.name, sourceKind: "abstract", member: method})
					}
				}
			}
		}
		for _, iface := range members.interfaces {
			out = append(out, collectRequiredMethods(model, iface, "interface", seen)...)
		}
		current = members.superClass
	}
	return out
}

func collectRequiredMethods(model map[string]typeMembers, typeName, sourceKind string, seen map[string]bool) []methodRequirement {
	members, ok := model[normalizeName(typeName)]
	if !ok {
		return nil
	}
	var out []methodRequirement
	for _, overloads := range members.methods {
		for _, method := range overloads {
			key := methodSignatureKey(method)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, methodRequirement{owner: members.name, sourceKind: sourceKind, member: method})
		}
	}
	for _, iface := range members.interfaces {
		out = append(out, collectRequiredMethods(model, iface, sourceKind, seen)...)
	}
	return out
}

func hasConcreteMethodSignature(model map[string]typeMembers, typeName string, required typesys.MemberSymbol) bool {
	for current := typeName; current != ""; {
		members, ok := model[normalizeName(current)]
		if !ok {
			return false
		}
		for _, method := range members.methods[normalizeName(required.Name)] {
			if sameSemaSignature(method, required) && !hasModifier(method.Modifiers, "abstract") {
				return true
			}
		}
		current = members.superClass
	}
	return false
}

func sameSemaSignature(left, right typesys.MemberSymbol) bool {
	if !strings.EqualFold(left.Name, right.Name) || len(left.Parameters) != len(right.Parameters) {
		return false
	}
	for i := range left.Parameters {
		if !sameSemaSignatureType(left.Parameters[i].Type, right.Parameters[i].Type) {
			return false
		}
	}
	return true
}

func sameSemaSignatureType(left, right string) bool {
	if strings.EqualFold(left, right) {
		return true
	}
	leftBase, leftArgs := semaGenericBaseAndArgs(left)
	rightBase, rightArgs := semaGenericBaseAndArgs(right)
	if len(leftArgs) > 0 || len(rightArgs) > 0 {
		if !sameSemaSignatureType(leftBase, rightBase) || len(leftArgs) != len(rightArgs) {
			return false
		}
		for i := range leftArgs {
			if !sameSemaSignatureType(leftArgs[i], rightArgs[i]) {
				return false
			}
		}
		return true
	}
	return strings.EqualFold(shortNestedTypeName(left), shortNestedTypeName(right))
}

func methodSignatureKey(member typesys.MemberSymbol) string {
	parts := make([]string, 0, len(member.Parameters)+1)
	parts = append(parts, normalizeName(member.Name))
	for _, param := range member.Parameters {
		parts = append(parts, normalizeName(param.Type))
	}
	return strings.Join(parts, "/")
}

type typeMembers struct {
	name         string
	superClass   string
	interfaces   []string
	methods      map[string][]typesys.MemberSymbol
	constructors []typesys.MemberSymbol
	fields       map[string]typesys.MemberSymbol
}

func (a *Analyzer) checkMethodBodies(index typesys.Index) []diagnostic.Diagnostic {
	model := buildTypeMembers(index)
	constructability := buildConstructability(index)
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
				body, bodyOffset, ok := extractBodyForSema(source, member.Range)
				if !ok {
					continue
				}
				diagnostics = append(diagnostics, a.checkBodyText(typ, member, body, bodyOffset, source, model, constructability)...)
			case apexast.DeclarationProperty:
				for _, accessor := range member.Accessors {
					if !accessor.HasBody {
						continue
					}
					source, ok := readSemaSource(typ.File, sources)
					if !ok {
						continue
					}
					body, bodyOffset, ok := extractBodyForSema(source, accessor.Range)
					if !ok {
						continue
					}
					accessorMember := member
					accessorMember.Kind = apexast.DeclarationMethod
					accessorMember.Name = member.Name + "." + accessor.Kind
					if accessor.Kind == "set" {
						accessorMember.Type = "void"
						accessorMember.Parameters = []apexast.Parameter{{Name: "value", Type: member.Type}}
					}
					diagnostics = append(diagnostics, a.checkBodyText(typ, accessorMember, body, bodyOffset, source, model, constructability)...)
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
	shortAliases := make(map[string][]string)
	for _, typ := range index.Types {
		members := typeMembers{
			name:       typ.Name,
			superClass: typ.SuperClass,
			interfaces: append([]string(nil), typ.Interfaces...),
			methods:    make(map[string][]typesys.MemberSymbol),
			fields:     make(map[string]typesys.MemberSymbol),
		}
		for _, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationMethod:
				members.methods[normalizeName(member.Name)] = append(members.methods[normalizeName(member.Name)], member)
			case apexast.DeclarationConstructor:
				members.constructors = append(members.constructors, member)
			case apexast.DeclarationField, apexast.DeclarationProperty:
				members.fields[normalizeName(member.Name)] = member
			}
		}
		out[normalizeName(typ.Name)] = members
		if index.Project.Namespace != "" {
			out[normalizeName(index.Project.Namespace+"."+typ.Name)] = members
		}
		if short := shortNestedTypeName(typ.Name); short != typ.Name {
			shortAliases[normalizeName(short)] = append(shortAliases[normalizeName(short)], typ.Name)
		}
	}
	for short, names := range shortAliases {
		if len(names) == 1 {
			out[short] = out[normalizeName(names[0])]
		}
	}
	for key, members := range out {
		members.superClass = resolveNestedTypeName(out, members.name, members.superClass)
		for i, iface := range members.interfaces {
			members.interfaces[i] = resolveNestedTypeName(out, members.name, iface)
		}
		out[key] = members
	}
	return out
}

func shortNestedTypeName(typeName string) string {
	if idx := strings.LastIndexByte(typeName, '.'); idx >= 0 {
		return typeName[idx+1:]
	}
	return typeName
}

func resolveNestedTypeName(model map[string]typeMembers, owner, typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.Contains(typeName, ".") {
		return typeName
	}
	ownerParts := strings.Split(owner, ".")
	if len(ownerParts) > 0 && strings.EqualFold(ownerParts[0], typeName) {
		return typeName
	}
	for i := len(ownerParts) - 1; i > 0; i-- {
		candidate := strings.Join(append(append([]string{}, ownerParts[:i]...), typeName), ".")
		if _, ok := model[normalizeName(candidate)]; ok {
			return candidate
		}
	}
	return typeName
}

func buildConstructability(index typesys.Index) map[string]typesys.TypeSymbol {
	out := make(map[string]typesys.TypeSymbol)
	for _, typ := range index.Types {
		out[normalizeName(typ.Name)] = typ
		if index.Project.Namespace != "" {
			out[normalizeName(index.Project.Namespace+"."+typ.Name)] = typ
		}
	}
	return out
}

func (a *Analyzer) checkBodyText(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	baseScope := make(map[string]string)
	for _, param := range member.Parameters {
		baseScope[normalizeName(param.Name)] = param.Type
	}
	for name, fieldType := range semaFieldScope(model, typ.Name, make(map[string]bool)) {
		baseScope[name] = fieldType
	}
	scopes, diagnostics := a.collectBodyScopes(typ, member, body, bodyOffset, source, baseScope, model)
	diagnostics = append(diagnostics, a.checkBodyIR(typ, member, body, bodyOffset, source, baseScope, model, constructability)...)
	for _, ctor := range constructorTypes(body) {
		for _, ref := range extractTypeNames(ctor.text) {
			if !a.hasKnown(ref) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA006",
					Message:  fmt.Sprintf("%s %q constructs unknown type %q", member.Kind, member.Name, ref),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+ctor.start, bodyOffset+ctor.end),
				})
				continue
			}
			if ref != constructedTypeName(ctor.text) {
				continue
			}
			if target, ok := constructability[normalizeName(ref)]; ok && !isConstructableType(target) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA015",
					Message:  fmt.Sprintf("%s %q constructs non-instantiable %s %q", member.Kind, member.Name, target.Kind, target.Name),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+ctor.start, bodyOffset+ctor.end),
				})
			}
		}
	}
	diagnostics = append(diagnostics, a.checkBodyAssignments(typ, member, body, bodyOffset, source, scopes, model)...)
	diagnostics = append(diagnostics, a.checkBodyReturns(typ, member, body, bodyOffset, source, scopes, model)...)
	diagnostics = append(diagnostics, a.checkBodyTernaryConditions(typ, member, body, bodyOffset, source, scopes, model)...)
	diagnostics = append(diagnostics, a.checkBodyExpressionTypeReferences(typ, member, body, bodyOffset, source)...)
	diagnostics = append(diagnostics, a.checkBodyCalls(typ, member, body, bodyOffset, source, scopes, model)...)
	return dedupeBodyDiagnostics(diagnostics)
}

func isConstructableType(typ typesys.TypeSymbol) bool {
	return typ.Kind == apexast.DeclarationClass && !hasModifier(typ.Modifiers, "abstract")
}

func dedupeBodyDiagnostics(diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	seen := make(map[string]bool)
	out := make([]diagnostic.Diagnostic, 0, len(diagnostics))
	for _, diag := range diagnostics {
		key := ""
		if diag.Range != nil {
			switch diag.Code {
			case "OAERSEMA006", "OAERSEMA008", "OAERSEMA009", "OAERSEMA010", "OAERSEMA011", "OAERSEMA015", "OAERSEMA018", "OAERSEMA019", "OAERSEMA020", "OAERSEMA022", "OAERSEMA023", "OAERSEMA024", "OAERSEMA025", "OAERSEMA026", "OAERSEMA027", "OAERSEMA028":
				key = fmt.Sprintf("%s:%s:%d", diag.File, diag.Code, diag.Range.Start.Line)
			}
		}
		if key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, diag)
	}
	return out
}

func semaFieldScope(model map[string]typeMembers, typeName string, seen map[string]bool) map[string]string {
	out := make(map[string]string)
	key := normalizeName(typeName)
	if key == "" || seen[key] {
		return out
	}
	seen[key] = true
	members, ok := model[key]
	if !ok {
		return out
	}
	for name, field := range semaFieldScope(model, members.superClass, seen) {
		out[name] = field
	}
	for name, field := range members.fields {
		out[name] = field.Type
	}
	return out
}

func semaResolveField(model map[string]typeMembers, typeName, fieldName string, seen map[string]bool) (resolvedMember, bool) {
	key := normalizeName(typeName)
	if key == "" || seen[key] {
		return resolvedMember{}, false
	}
	seen[key] = true
	members, ok := model[key]
	if !ok {
		return resolvedMember{}, false
	}
	if field, ok := members.fields[normalizeName(fieldName)]; ok {
		return resolvedMember{owner: members.name, member: field}, true
	}
	if field, ok := semaResolveField(model, members.superClass, fieldName, seen); ok {
		return field, true
	}
	return resolvedMember{}, false
}

type irSemaScope struct {
	frames []map[string]string
}

func newIRSemaScope(base map[string]string) irSemaScope {
	root := make(map[string]string, len(base))
	for name, typ := range base {
		root[normalizeName(name)] = typ
	}
	return irSemaScope{frames: []map[string]string{root}}
}

func (s *irSemaScope) push() {
	s.frames = append(s.frames, make(map[string]string))
}

func (s *irSemaScope) pop() {
	if len(s.frames) > 1 {
		s.frames = s.frames[:len(s.frames)-1]
	}
}

func (s *irSemaScope) declare(name, typ string) {
	if len(s.frames) == 0 {
		s.frames = append(s.frames, make(map[string]string))
	}
	s.frames[len(s.frames)-1][normalizeName(name)] = typ
}

func (s irSemaScope) lookup(name string) (string, bool) {
	key := normalizeName(name)
	for i := len(s.frames) - 1; i >= 0; i-- {
		if typ, ok := s.frames[i][key]; ok {
			return typ, true
		}
	}
	return "", false
}

func (s irSemaScope) flat() map[string]string {
	out := make(map[string]string)
	for _, frame := range s.frames {
		for name, typ := range frame {
			out[name] = typ
		}
	}
	return out
}

func (a *Analyzer) checkBodyIR(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return nil
	}
	scope := newIRSemaScope(base)
	diagnostics := a.checkIRInstructions(typ, member, program.Instructions, &scope, bodyOffset, source, model, constructability)
	returnType := strings.TrimSpace(member.Type)
	if returnType != "" && !strings.EqualFold(returnType, "void") && !irInstructionsTerminate(program.Instructions) {
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s on all paths", returnType), member.Range.Start.Offset, member.Range.End.Offset, source))
	}
	return diagnostics
}

func (a *Analyzer) checkIRInstructions(typ typesys.TypeSymbol, member typesys.MemberSymbol, instructions []ir.Instruction, scope *irSemaScope, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, inst := range instructions {
		switch inst.Op {
		case ir.OpDeclare:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRAssignmentType(typ, member, inst.Type, inst.Name, inst.Expr, scope, inst.Pos, bodyOffset, source, model, "initializes")...)
			scope.declare(inst.Name, inst.Type)
		case ir.OpAssign:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRAssignmentTarget(typ, member, inst.Name, *scope, inst.Pos, bodyOffset, source, model)...)
			if targetType, ok := irAssignmentTargetType(inst.Name, *scope, model, typ.Name); ok {
				diagnostics = append(diagnostics, a.checkIRAssignmentType(typ, member, targetType, inst.Name, inst.Expr, scope, inst.Pos, bodyOffset, source, model, "assigns")...)
			}
		case ir.OpReturn:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			returnType := strings.TrimSpace(member.Type)
			if returnType != "" && !strings.EqualFold(returnType, "void") {
				diagnostics = append(diagnostics, a.checkIRReturnType(typ, member, returnType, inst.Expr, scope, inst.Pos, bodyOffset, source, model)...)
			}
		case ir.OpExpr, ir.OpThrow, ir.OpDML, ir.OpRunAs:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			if inst.Op == ir.OpRunAs {
				scope.push()
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
				scope.pop()
			}
		case ir.OpIf:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRConditionType(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model)...)
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Else, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpWhile, ir.OpDoWhile:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRConditionType(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model)...)
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpFor:
			scope.push()
			if inst.Init != nil {
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, []ir.Instruction{*inst.Init}, scope, bodyOffset, source, model, constructability)...)
			}
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRConditionType(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model)...)
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			if inst.Update != nil {
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, []ir.Instruction{*inst.Update}, scope, bodyOffset, source, model, constructability)...)
			}
			scope.pop()
		case ir.OpForEach:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRForEachType(typ, member, inst, *scope, bodyOffset, source, model)...)
			scope.push()
			scope.declare(inst.Name, inst.Type)
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpTry:
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
			for _, catchClause := range catchClauses(inst) {
				scope.push()
				if catchClause.Name != "" {
					catchType := "Exception"
					if len(catchClause.Types) > 0 {
						catchType = catchClause.Types[0]
					}
					scope.declare(catchClause.Name, catchType)
				}
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, catchClause.Body, scope, bodyOffset, source, model, constructability)...)
				scope.pop()
			}
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Finally, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpSwitch:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			for _, switchCase := range inst.Cases {
				for _, expr := range switchCase.Exprs {
					diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, expr, scope, switchCase.Pos, bodyOffset, source, model, constructability)...)
				}
				scope.push()
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, switchCase.Body, scope, bodyOffset, source, model, constructability)...)
				scope.pop()
			}
		}
	}
	return diagnostics
}

func irInstructionsTerminate(instructions []ir.Instruction) bool {
	for _, inst := range instructions {
		if irInstructionTerminates(inst) {
			return true
		}
	}
	return false
}

func irInstructionTerminates(inst ir.Instruction) bool {
	switch inst.Op {
	case ir.OpReturn, ir.OpThrow:
		return true
	case ir.OpIf:
		return len(inst.Then) > 0 && len(inst.Else) > 0 && irInstructionsTerminate(inst.Then) && irInstructionsTerminate(inst.Else)
	case ir.OpTry:
		if irInstructionsTerminate(inst.Finally) {
			return true
		}
		clauses := catchClauses(inst)
		if len(clauses) == 0 {
			return false
		}
		for _, catchClause := range clauses {
			if !irInstructionsTerminate(catchClause.Body) {
				return false
			}
		}
		return irInstructionsTerminate(inst.Then)
	case ir.OpSwitch:
		hasElse := false
		if len(inst.Cases) == 0 {
			return false
		}
		for _, switchCase := range inst.Cases {
			if switchCase.Else {
				hasElse = true
			}
			if !irInstructionsTerminate(switchCase.Body) {
				return false
			}
		}
		return hasElse
	default:
		return false
	}
}

func catchClauses(inst ir.Instruction) []ir.CatchClause {
	if len(inst.Catches) > 0 {
		return inst.Catches
	}
	if len(inst.Catch) == 0 {
		return nil
	}
	return []ir.CatchClause{{Types: catchTypes(inst), Name: inst.Name, Body: inst.Catch, Pos: inst.Pos}}
}

func catchTypes(inst ir.Instruction) []string {
	if len(inst.CatchTypes) > 0 {
		return inst.CatchTypes
	}
	if inst.Type == "" {
		return nil
	}
	return []string{inst.Type}
}

func (a *Analyzer) checkIRExprVariables(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	if expr.Kind == "" {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	switch expr.Kind {
	case ir.ExprVariable:
		if diag, ok := a.irVariableDiagnostic(typ, member, expr.Name, *scope, model, bodyOffset+pos, source); ok {
			diagnostics = append(diagnostics, diag)
		} else if !a.irVariableKnown(expr.Name, *scope, model, typ.Name) && !isLikelyTypeReference(expr.Name) {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA013",
				Message:  fmt.Sprintf("%s %q reads unknown variable %q", member.Kind, member.Name, expr.Name),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Name))),
			})
		}
	case ir.ExprCall:
		if strings.HasPrefix(expr.Callee, "Search.") {
			return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, expr.Callee+" local search/SOSL surface", bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
		}
		for _, arg := range expr.Args {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, arg, scope, pos, bodyOffset, source, model, constructability)...)
		}
		for _, arg := range expr.NamedArgs {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, arg.Expr, scope, pos, bodyOffset, source, model, constructability)...)
		}
		diagnostics = append(diagnostics, a.checkIRCall(typ, member, expr, *scope, pos, bodyOffset, source, model, constructability)...)
	case ir.ExprUnary:
		if expr.Left != nil {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, *expr.Left, scope, pos, bodyOffset, source, model, constructability)...)
		}
	case ir.ExprBinary:
		if expr.Left != nil {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, *expr.Left, scope, pos, bodyOffset, source, model, constructability)...)
		}
		if expr.Right != nil {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, *expr.Right, scope, pos, bodyOffset, source, model, constructability)...)
		}
	case ir.ExprSOQL:
		if soql.IsSOSLFind(expr.Value) {
			return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, "SOSL/FIND local search surface", bodyOffset+pos, bodyOffset+pos+max(1, len("FIND")), source)}
		}
	}
	return diagnostics
}

func (a *Analyzer) checkIRAssignmentTarget(typ typesys.TypeSymbol, member typesys.MemberSymbol, name string, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
	if diag, ok := a.irVariableDiagnostic(typ, member, name, scope, model, bodyOffset+pos, source); ok {
		return []diagnostic.Diagnostic{diag}
	}
	return nil
}

func (a *Analyzer) checkIRCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	if strings.HasPrefix(expr.Callee, "new:") {
		return a.checkIRConstructorCall(typ, member, expr, scope, pos, bodyOffset, source, model, constructability)
	}
	if strings.HasPrefix(expr.Callee, "__field:") ||
		strings.HasPrefix(expr.Callee, "__safe_field:") ||
		strings.HasPrefix(expr.Callee, "__cast:") ||
		expr.Callee == "__ternary" {
		return nil
	}
	if expr.Callee == "" || expr.Callee == "this" || expr.Callee == "super" || skipSemaCall(expr.Callee) {
		return nil
	}
	receiverType := typ.Name
	method := expr.Callee
	explicitReceiver := false
	if receiver, callee, ok := strings.Cut(expr.Callee, "."); ok {
		explicitReceiver = true
		method = callee
		switch {
		case strings.EqualFold(receiver, "this"):
			receiverType = typ.Name
		case strings.EqualFold(receiver, "super"):
			if members, ok := model[normalizeName(typ.Name)]; ok {
				receiverType = members.superClass
			}
		default:
			if scoped, ok := scope.lookup(receiver); ok {
				receiverType = scoped
			} else if _, ok := model[normalizeName(receiver)]; ok {
				receiverType = receiver
			} else if a.hasKnown(receiver) {
				return nil
			} else {
				return nil
			}
		}
	}
	if receiverType == "" {
		return nil
	}
	if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
		return diagnostics
	}
	candidates := resolveMemberMethods(model, receiverType, method)
	if len(candidates) == 0 && !strings.Contains(expr.Callee, ".") && bodyOffset >= 0 && bodyOffset <= len(source) {
		if chainedReceiver, chainedMethod, ok := semaChainedCallReceiverNear(source[bodyOffset:], pos, method, scope.flat(), model); ok && strings.EqualFold(chainedMethod, method) {
			receiverType = chainedReceiver
			explicitReceiver = true
			if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
				return diagnostics
			}
			candidates = resolveMemberMethods(model, receiverType, method)
		}
	}
	if len(candidates) == 0 {
		if explicitReceiver && a.hasKnown(receiverType) {
			return nil
		}
		return []diagnostic.Diagnostic{unknownCallDiagnostic(typ, member, expr.Callee, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	if _, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model); ok {
		return nil
	} else if ambiguous {
		return []diagnostic.Diagnostic{ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA009",
		Message:  fmt.Sprintf("%s %q has no matching overload for call %q with %d argument(s)", member.Kind, member.Name, expr.Callee, len(expr.Args)),
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee))),
	}}
}

func (a *Analyzer) checkIRCollectionCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) ([]diagnostic.Diagnostic, bool) {
	sig, ok := semaCollectionMethodSignature(receiverType, method)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = a.inferIRExprType(arg, scope, model, typ.Name)
	}
	if semaArgsMatchAny(sig.params, argTypes, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}, true
}

func (a *Analyzer) checkIRConstructorCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	typeName := strings.TrimPrefix(expr.Callee, "new:")
	for _, ref := range extractTypeNames(typeName) {
		if !a.hasKnown(ref) {
			return []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA006",
				Message:  fmt.Sprintf("%s %q constructs unknown type %q", member.Kind, member.Name, ref),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName))),
			}}
		}
	}
	if target, ok := constructability[normalizeName(typeName)]; ok && !isConstructableType(target) {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     "OAERSEMA015",
			Message:  fmt.Sprintf("%s %q constructs non-instantiable %s %q", member.Kind, member.Name, target.Kind, target.Name),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName))),
		}}
	}
	if diagnostics, handled := a.checkIRCollectionConstructor(typ, member, typeName, expr.Args, scope, pos, bodyOffset, source, model); handled {
		return diagnostics
	}
	target, ok := model[normalizeName(typeName)]
	if !ok {
		return nil
	}
	if len(target.constructors) == 0 {
		if len(expr.Args) == 0 {
			return nil
		}
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("no matching %s constructor with %d argument(s)", typeName, len(expr.Args)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if _, ok, ambiguous := bestMemberByArgTypes(target.constructors, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model); ok {
		return nil
	} else if ambiguous {
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("ambiguous %s constructor with %d argument(s)", typeName, len(expr.Args)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("no matching %s constructor with %d argument(s)", typeName, len(expr.Args)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
}

func (a *Analyzer) checkIRCollectionConstructor(typ typesys.TypeSymbol, member typesys.MemberSymbol, typeName string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) ([]diagnostic.Diagnostic, bool) {
	base, params := semaGenericBaseAndArgs(typeName)
	baseKey := normalizeName(base)
	if baseKey != "list" && baseKey != "set" && baseKey != "map" {
		return nil, false
	}
	if len(args) == 0 {
		return nil, true
	}
	if (baseKey == "list" || baseKey == "set") && len(params) == 1 {
		if len(args) == 1 {
			argType := a.inferIRExprType(args[0], scope, model, typ.Name)
			if argType == "" || strings.EqualFold(argType, "null") || semaAssignableToType(typeName, argType, model) || semaCollectionCopyConstructorAccepts(baseKey, params[0], argType, model) {
				return nil, true
			}
		}
		for _, arg := range args {
			argType := a.inferIRExprType(arg, scope, model, typ.Name)
			if argType != "" && !strings.EqualFold(argType, "null") && !semaAssignableToType(params[0], argType, model) {
				return []diagnostic.Diagnostic{collectionConstructorDiagnostic(typ, member, typeName, len(args), bodyOffset+pos, source)}, true
			}
		}
		return nil, true
	}
	if baseKey == "map" && len(params) == 2 && len(args) == 1 {
		argType := a.inferIRExprType(args[0], scope, model, typ.Name)
		if argType == "" || strings.EqualFold(argType, "null") || semaAssignableToType(typeName, argType, model) || semaMapConstructorAccepts(params[0], params[1], argType, model) {
			return nil, true
		}
	}
	return []diagnostic.Diagnostic{collectionConstructorDiagnostic(typ, member, typeName, len(args), bodyOffset+pos, source)}, true
}

func semaCollectionCopyConstructorAccepts(targetBase, targetElement, argType string, model map[string]typeMembers) bool {
	sourceBase, sourceArgs := semaGenericBaseAndArgs(argType)
	sourceBaseKey := normalizeName(sourceBase)
	if sourceBaseKey != "list" && sourceBaseKey != "set" {
		return false
	}
	if (targetBase != "list" && targetBase != "set") || len(sourceArgs) != 1 {
		return false
	}
	return semaAssignableToType(targetElement, sourceArgs[0], model)
}

func semaMapConstructorAccepts(keyType, valueType, argType string, model map[string]typeMembers) bool {
	sourceBase, sourceArgs := semaGenericBaseAndArgs(argType)
	sourceBaseKey := normalizeName(sourceBase)
	if sourceBaseKey == "map" && len(sourceArgs) == 2 {
		return semaAssignableToType(keyType, sourceArgs[0], model) && semaAssignableToType(valueType, sourceArgs[1], model)
	}
	if sourceBaseKey == "list" && len(sourceArgs) == 1 && strings.EqualFold(keyType, "Id") {
		return semaAssignableToType(valueType, sourceArgs[0], model)
	}
	return false
}

func irCallArgTypes(a *Analyzer, args []ir.Expr, scope irSemaScope, model map[string]typeMembers, currentType string) []string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = a.inferIRExprType(arg, scope, model, currentType)
	}
	return argTypes
}

func irCallArgsMatch(a *Analyzer, params []apexast.Parameter, args []ir.Expr, scope irSemaScope, model map[string]typeMembers, currentType string) bool {
	if len(params) != len(args) {
		return false
	}
	for i, param := range params {
		argType := a.inferIRExprType(args[i], scope, model, currentType)
		if semaConversionScore(param.Type, argType, model) < 0 {
			return false
		}
	}
	return true
}

func (a *Analyzer) checkIRAssignmentType(typ typesys.TypeSymbol, member typesys.MemberSymbol, targetType, target string, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers, verb string) []diagnostic.Diagnostic {
	valueType := a.inferIRExprType(expr, *scope, model, typ.Name)
	if valueType == "" || valueType == "null" || semaAssignableToType(targetType, valueType, model) {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA018",
		Message:  fmt.Sprintf("%s %q %s %s with %s", member.Kind, member.Name, verb, target, valueType),
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(target))),
	}}
}

func (a *Analyzer) checkIRReturnType(typ typesys.TypeSymbol, member typesys.MemberSymbol, returnType string, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
	valueType := a.inferIRExprType(expr, *scope, model, typ.Name)
	if valueType == "" || valueType == "null" || semaAssignableToType(returnType, valueType, model) {
		return nil
	}
	return []diagnostic.Diagnostic{returnTypeDiagnostic(typ, member, fmt.Sprintf("returns %s from %s method", valueType, returnType), bodyOffset+pos, bodyOffset+pos+max(1, len(valueType)), source)}
}

func (a *Analyzer) checkIRConditionType(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
	valueType := a.inferIRExprType(expr, *scope, model, typ.Name)
	if valueType == "" || strings.EqualFold(valueType, "Boolean") {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA020",
		Message:  fmt.Sprintf("%s %q uses %s expression as a Boolean condition", member.Kind, member.Name, valueType),
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(valueType))),
	}}
}

func (a *Analyzer) checkIRForEachType(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope irSemaScope, bodyOffset int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
	iterableType := a.inferIRExprType(inst.Expr, scope, model, typ.Name)
	if iterableType == "" {
		return nil
	}
	elementType, ok := semaIterableElementType(iterableType)
	if !ok {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     "OAERSEMA024",
			Message:  fmt.Sprintf("%s %q enhanced-for iterates non-collection type %s", member.Kind, member.Name, iterableType),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(iterableType))),
		}}
	}
	if elementType == "" || semaAssignableToType(inst.Type, elementType, model) {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA024",
		Message:  fmt.Sprintf("%s %q enhanced-for assigns %s elements to %s variable %q", member.Kind, member.Name, elementType, inst.Type, inst.Name),
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Name))),
	}}
}

func (a *Analyzer) inferIRExprType(expr ir.Expr, scope irSemaScope, model map[string]typeMembers, currentType string) string {
	switch expr.Kind {
	case ir.ExprLiteral:
		return inferSemaArgType(expr.Value, scope.flat())
	case ir.ExprVariable:
		if typ, ok := scope.lookup(expr.Name); ok {
			return typ
		}
		if root, field, ok := strings.Cut(expr.Name, "."); ok {
			if receiverType := semaIRReceiverType(root, scope, model, currentType); receiverType != "" {
				return semaFieldScope(model, receiverType, make(map[string]bool))[normalizeName(field)]
			}
		}
	case ir.ExprCall:
		if strings.HasPrefix(expr.Callee, "new:") {
			return strings.TrimPrefix(expr.Callee, "new:")
		}
		if receiver, method, ok := strings.Cut(expr.Callee, "."); ok {
			receiverType := semaIRReceiverType(receiver, scope, model, currentType)
			if receiverType == "" {
				receiverType = receiver
			}
			return semaResolvedIRCallReturnType(a, model, receiverType, method, expr.Args, scope, currentType)
		}
		return semaResolvedIRCallReturnType(a, model, currentType, expr.Callee, expr.Args, scope, currentType)
	case ir.ExprUnary:
		switch expr.Operator {
		case "!":
			return "Boolean"
		case "-":
			if expr.Left != nil {
				return a.inferIRExprType(*expr.Left, scope, model, currentType)
			}
		}
	case ir.ExprBinary:
		leftType := ""
		rightType := ""
		if expr.Left != nil {
			leftType = a.inferIRExprType(*expr.Left, scope, model, currentType)
		}
		if expr.Right != nil {
			rightType = a.inferIRExprType(*expr.Right, scope, model, currentType)
		}
		return semaBinaryType(expr.Operator, leftType, rightType)
	}
	return ""
}

func semaIRReceiverType(receiver string, scope irSemaScope, model map[string]typeMembers, currentType string) string {
	switch {
	case strings.EqualFold(receiver, "this"):
		return currentType
	case strings.EqualFold(receiver, "super"):
		if members, ok := model[normalizeName(currentType)]; ok {
			return members.superClass
		}
	case receiver == "":
		return ""
	default:
		if scoped, ok := scope.lookup(receiver); ok {
			return scoped
		}
		if _, ok := model[normalizeName(receiver)]; ok {
			return receiver
		}
	}
	return ""
}

func semaResolvedIRCallReturnType(a *Analyzer, model map[string]typeMembers, receiverType, method string, args []ir.Expr, scope irSemaScope, currentType string) string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = a.inferIRExprType(arg, scope, model, currentType)
	}
	if sig, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return sig.returnType
	}
	if candidate, ok, _ := bestResolvedMemberByArgTypes(resolveMemberMethods(model, receiverType, method), argTypes, model); ok {
		return candidate.member.Type
	}
	return ""
}

func semaBinaryType(op, leftType, rightType string) string {
	switch op {
	case "&&", "||":
		if strings.EqualFold(leftType, "Boolean") && strings.EqualFold(rightType, "Boolean") {
			return "Boolean"
		}
	case "==", "!=", "<=", ">=", "<", ">":
		return "Boolean"
	case "+":
		if strings.EqualFold(leftType, "String") || strings.EqualFold(rightType, "String") {
			return "String"
		}
		return semaNumericResultType(leftType, rightType)
	case "-", "*", "/", "%":
		return semaNumericResultType(leftType, rightType)
	}
	return ""
}

func semaNumericResultType(leftType, rightType string) string {
	for _, typ := range []string{"Double", "Decimal", "Long", "Integer"} {
		if strings.EqualFold(leftType, typ) || strings.EqualFold(rightType, typ) {
			if isSemaNumericType(leftType) && isSemaNumericType(rightType) {
				return typ
			}
		}
	}
	return ""
}

func irAssignmentTargetType(name string, scope irSemaScope, model map[string]typeMembers, currentType string) (string, bool) {
	if typ, ok := scope.lookup(name); ok {
		return typ, true
	}
	root, field, ok := strings.Cut(name, ".")
	if !ok {
		return "", false
	}
	receiverType := semaIRReceiverType(root, scope, model, currentType)
	if receiverType == "" {
		return "", false
	}
	fieldType := semaFieldScope(model, receiverType, make(map[string]bool))[normalizeName(field)]
	return fieldType, fieldType != ""
}

func (a *Analyzer) irVariableDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, name string, scope irSemaScope, model map[string]typeMembers, start int, source string) (diagnostic.Diagnostic, bool) {
	root, field, hasMember := strings.Cut(name, ".")
	if !hasMember {
		return diagnostic.Diagnostic{}, false
	}
	receiverType := ""
	switch {
	case strings.EqualFold(root, "this"):
		receiverType = typ.Name
	case strings.EqualFold(root, "super"):
		if members, ok := model[normalizeName(typ.Name)]; ok {
			receiverType = members.superClass
		}
	default:
		if scoped, ok := scope.lookup(root); ok {
			receiverType = scoped
		} else if _, ok := model[normalizeName(root)]; ok {
			receiverType = root
		}
	}
	if receiverType == "" {
		return diagnostic.Diagnostic{}, false
	}
	if _, ok := model[normalizeName(receiverType)]; !ok {
		return diagnostic.Diagnostic{}, false
	}
	if strings.EqualFold(field, "class") {
		return diagnostic.Diagnostic{}, false
	}
	if target, ok := semaResolveField(model, receiverType, field, make(map[string]bool)); ok {
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, field, target, start, start+max(1, len(name)), source, model); blocked {
			return visibilityDiagnostic, true
		}
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA021",
		Message:  fmt.Sprintf("%s %q references unknown field %q on %s", member.Kind, member.Name, field, receiverType),
		File:     typ.File,
		Range:    semaRange(source, start, start+max(1, len(name))),
	}, true
}

func (a *Analyzer) irVariableKnown(name string, scope irSemaScope, model map[string]typeMembers, currentType string) bool {
	if name == "" || name == "this" || name == "super" {
		return true
	}
	root, _, hasMember := strings.Cut(name, ".")
	if hasMember {
		if strings.EqualFold(root, "this") || strings.EqualFold(root, "super") {
			return true
		}
		if _, ok := scope.lookup(root); ok {
			return true
		}
		if _, ok := model[normalizeName(root)]; ok {
			return true
		}
	}
	if _, ok := scope.lookup(root); ok {
		return true
	}
	if _, ok := scope.lookup(name); ok {
		return true
	}
	if hasMember && (a.hasKnown(root) || model[normalizeName(root)].name != "") {
		return true
	}
	if a.hasKnown(name) {
		return true
	}
	return false
}

func constructedTypeName(text string) string {
	name := strings.TrimSpace(text)
	if idx := strings.IndexByte(name, '<'); idx >= 0 {
		name = strings.TrimSpace(name[:idx])
	}
	return name
}

type semaLocal struct {
	name       string
	typeName   string
	start      int
	scopeStart int
	scopeEnd   int
}

type semaScopeModel struct {
	base   map[string]string
	locals []semaLocal
}

func (a *Analyzer) collectBodyScopes(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model map[string]typeMembers) (semaScopeModel, []diagnostic.Diagnostic) {
	scopes := semaScopeModel{base: base}
	var diagnostics []diagnostic.Diagnostic
	for _, match := range localDeclPattern.FindAllStringSubmatchIndex(body, -1) {
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		name := strings.TrimSpace(body[match[4]:match[5]])
		if isSemaKeyword(typeName) {
			continue
		}
		scopeStart, scopeEnd := blockBoundsAt(body, match[0])
		if _, exists := scopes.localInBlock(name, scopeStart, scopeEnd); exists {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA014",
				Message:  fmt.Sprintf("%s %q redeclares local variable %q in the same scope", member.Kind, member.Name, name),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+match[4], bodyOffset+match[5]),
			})
		}
		for _, ref := range extractTypeNames(typeName) {
			if !a.hasKnown(ref) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA006",
					Message:  fmt.Sprintf("%s %q declares local %q with unknown type %q", member.Kind, member.Name, name, ref),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+match[2], bodyOffset+match[3]),
				})
			}
		}
		if match[1] > 0 && body[match[1]-1] == '=' {
			value := trimSemaArg(body, match[1], semaStatementEnd(body, match[1]))
			valueType := inferSemaArgTypeWithModel(value.text, scopes.flat(), model)
			if valueType != "" && valueType != "null" && !semaAssignableToType(typeName, valueType, model) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA018",
					Message:  fmt.Sprintf("%s %q initializes %s local %q with %s", member.Kind, member.Name, typeName, name, valueType),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+value.start, bodyOffset+value.end),
				})
			}
		}
		scopes.locals = append(scopes.locals, semaLocal{name: name, typeName: typeName, start: match[5], scopeStart: scopeStart, scopeEnd: scopeEnd})
	}
	for _, match := range enhancedForLocalPattern.FindAllStringSubmatchIndex(body, -1) {
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		name := strings.TrimSpace(body[match[4]:match[5]])
		if isSemaKeyword(typeName) {
			continue
		}
		scopeStart, scopeEnd := blockBoundsAt(body, match[0])
		if scopeStart < match[1] {
			scopeStart = match[1]
		}
		for _, ref := range extractTypeNames(typeName) {
			if !a.hasKnown(ref) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "OAERSEMA006",
					Message:  fmt.Sprintf("%s %q declares enhanced-for local %q with unknown type %q", member.Kind, member.Name, name, ref),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+match[2], bodyOffset+match[3]),
				})
			}
		}
		scopes.locals = append(scopes.locals, semaLocal{name: name, typeName: typeName, start: match[5], scopeStart: scopeStart, scopeEnd: scopeEnd})
	}
	return scopes, diagnostics
}

func (s semaScopeModel) visibleAt(name string, pos int) (string, bool) {
	key := normalizeName(name)
	for i := len(s.locals) - 1; i >= 0; i-- {
		local := s.locals[i]
		if normalizeName(local.name) == key && pos >= local.start && pos <= local.scopeEnd {
			return local.typeName, true
		}
	}
	typeName, ok := s.base[key]
	return typeName, ok
}

func (s semaScopeModel) flat() map[string]string {
	out := make(map[string]string, len(s.base)+len(s.locals))
	for name, typeName := range s.base {
		out[name] = typeName
	}
	for _, local := range s.locals {
		out[normalizeName(local.name)] = local.typeName
	}
	return out
}

func (s semaScopeModel) localInBlock(name string, start, end int) (semaLocal, bool) {
	key := normalizeName(name)
	for _, local := range s.locals {
		if normalizeName(local.name) == key && local.scopeStart == start && local.scopeEnd == end {
			return local, true
		}
	}
	return semaLocal{}, false
}

func (a *Analyzer) checkBodyAssignments(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, match := range assignmentPattern.FindAllStringSubmatchIndex(body, -1) {
		target := strings.TrimSpace(body[match[2]:match[3]])
		targetType, ok := scopes.visibleAt(target, match[2])
		if ok {
			value := trimSemaArg(body, match[1], semaStatementEnd(body, match[1]))
			valueType := inferSemaArgTypeWithModel(value.text, scopes.flat(), model)
			if valueType == "" || valueType == "null" || semaAssignableToType(targetType, valueType, model) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA018",
				Message:  fmt.Sprintf("%s %q assigns %s to %s variable %q", member.Kind, member.Name, valueType, targetType, target),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+value.start, bodyOffset+value.end),
			})
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "OAERSEMA013",
			Message:  fmt.Sprintf("%s %q assigns unknown variable %q", member.Kind, member.Name, target),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+match[2], bodyOffset+match[3]),
		})
	}
	return diagnostics
}

func (a *Analyzer) checkBodyReturns(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	if member.Type == "" {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	returnType := strings.TrimSpace(member.Type)
	foundReturn := false
	for _, match := range returnPattern.FindAllStringSubmatchIndex(body, -1) {
		foundReturn = true
		hasValue := match[2] >= 0
		if strings.EqualFold(returnType, "void") {
			if hasValue {
				diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, "void method cannot return a value", bodyOffset+match[2], bodyOffset+match[3], source))
			}
			continue
		}
		if !hasValue {
			diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s", returnType), bodyOffset+match[0], bodyOffset+match[1], source))
			continue
		}
		value := trimSemaArg(body, match[2], match[3])
		valueType := inferSemaArgTypeWithModel(value.text, scopes.flat(), model)
		if valueType == "" || valueType == "null" || semaAssignableToType(returnType, valueType, model) {
			continue
		}
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("returns %s from %s method", valueType, returnType), bodyOffset+value.start, bodyOffset+value.end, source))
	}
	if !foundReturn && !strings.EqualFold(returnType, "void") {
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s", returnType), member.Range.Start.Offset, member.Range.End.Offset, source))
	}
	return diagnostics
}

func returnTypeDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, detail string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA019",
		Message:  fmt.Sprintf("%s %q has invalid return: %s", member.Kind, member.Name, detail),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func (a *Analyzer) checkBodyTernaryConditions(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	seen := make(map[int]bool)
	for _, expr := range semaBodyExpressions(body) {
		if seen[expr.start] {
			continue
		}
		seen[expr.start] = true
		diagnostics = append(diagnostics, checkSemaTernaryCondition(typ, member, expr.text, bodyOffset+expr.start, source, scopes.flat(), model)...)
	}
	return diagnostics
}

func semaBodyExpressions(body string) []semaArg {
	var exprs []semaArg
	for _, match := range localDeclPattern.FindAllStringSubmatchIndex(body, -1) {
		if match[1] > 0 && body[match[1]-1] == '=' {
			exprs = append(exprs, trimSemaArg(body, match[1], semaStatementEnd(body, match[1])))
		}
	}
	for _, match := range assignmentPattern.FindAllStringSubmatchIndex(body, -1) {
		exprs = append(exprs, trimSemaArg(body, match[1], semaStatementEnd(body, match[1])))
	}
	for _, match := range returnPattern.FindAllStringSubmatchIndex(body, -1) {
		if match[2] >= 0 {
			exprs = append(exprs, trimSemaArg(body, match[2], match[3]))
		}
	}
	return exprs
}

func checkSemaTernaryCondition(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr string, exprStart int, source string, scope map[string]string, model map[string]typeMembers) []diagnostic.Diagnostic {
	question, colon, ok := semaTernaryPositions(expr)
	if !ok {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	condition := strings.TrimSpace(expr[:question])
	conditionStart := exprStart + leadingWhitespaceLen(expr[:question])
	conditionType := inferSemaArgTypeWithModel(condition, scope, model)
	if conditionType != "" && !strings.EqualFold(conditionType, "Boolean") {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "OAERSEMA020",
			Message:  fmt.Sprintf("%s %q uses %s expression as a ternary condition", member.Kind, member.Name, conditionType),
			File:     typ.File,
			Range:    semaRange(source, conditionStart, conditionStart+max(1, len(condition))),
		})
	}
	whenTrue := strings.TrimSpace(expr[question+1 : colon])
	trueStart := exprStart + question + 1 + leadingWhitespaceLen(expr[question+1:colon])
	whenFalse := strings.TrimSpace(expr[colon+1:])
	falseStart := exprStart + colon + 1 + leadingWhitespaceLen(expr[colon+1:])
	diagnostics = append(diagnostics, checkSemaTernaryCondition(typ, member, whenTrue, trueStart, source, scope, model)...)
	diagnostics = append(diagnostics, checkSemaTernaryCondition(typ, member, whenFalse, falseStart, source, scope, model)...)
	return diagnostics
}

func leadingWhitespaceLen(text string) int {
	return len(text) - len(strings.TrimLeftFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}))
}

func (a *Analyzer) checkBodyExpressionTypeReferences(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	seen := make(map[string]bool)
	for _, expr := range semaBodyExpressions(body) {
		diagnostics = append(diagnostics, a.checkSemaExpressionTypeReferences(typ, member, expr.text, bodyOffset+expr.start, source, seen)...)
	}
	return diagnostics
}

func (a *Analyzer) checkSemaExpressionTypeReferences(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr string, exprStart int, source string, seen map[string]bool) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if _, whenTrue, whenFalse, ok := splitSemaTernary(expr); ok {
		question, colon, _ := semaTernaryPositions(expr)
		condition := strings.TrimSpace(expr[:question])
		conditionStart := exprStart + leadingWhitespaceLen(expr[:question])
		trueStart := exprStart + question + 1 + leadingWhitespaceLen(expr[question+1:colon])
		falseStart := exprStart + colon + 1 + leadingWhitespaceLen(expr[colon+1:])
		diagnostics = append(diagnostics, a.checkSemaExpressionTypeReferences(typ, member, condition, conditionStart, source, seen)...)
		diagnostics = append(diagnostics, a.checkSemaExpressionTypeReferences(typ, member, strings.TrimSpace(whenTrue), trueStart, source, seen)...)
		diagnostics = append(diagnostics, a.checkSemaExpressionTypeReferences(typ, member, strings.TrimSpace(whenFalse), falseStart, source, seen)...)
		return diagnostics
	}
	if castType, _, ok := splitSemaCast(expr); ok {
		diagnostics = append(diagnostics, a.expressionTypeReferenceDiagnostics(typ, member, castType, exprStart+1, source, seen)...)
	}
	if _, instanceType, ok := splitSemaInstanceOf(expr); ok {
		typeStart := strings.LastIndex(expr, instanceType)
		if typeStart < 0 {
			typeStart = len(expr) - len(instanceType)
		}
		diagnostics = append(diagnostics, a.expressionTypeReferenceDiagnostics(typ, member, instanceType, exprStart+typeStart, source, seen)...)
	}
	return diagnostics
}

func (a *Analyzer) expressionTypeReferenceDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol, typeName string, start int, source string, seen map[string]bool) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, ref := range extractTypeNames(typeName) {
		key := fmt.Sprintf("%d:%s", start, normalizeName(ref))
		if seen[key] {
			continue
		}
		seen[key] = true
		if !a.hasKnown(ref) {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERSEMA006",
				Message:  fmt.Sprintf("%s %q references unknown expression type %q", member.Kind, member.Name, ref),
				File:     typ.File,
				Range:    semaRange(source, start, start+max(1, len(typeName))),
			})
		}
	}
	return diagnostics
}

func (a *Analyzer) checkBodyCalls(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, match := range callPattern.FindAllStringSubmatchIndex(body, -1) {
		callee := strings.TrimSpace(body[match[2]:match[3]])
		if skipSemaCall(callee) {
			continue
		}
		if isSemaConstructorCallAt(body, match[0]) {
			continue
		}
		args, haveArgs := callArgumentsAt(body, match[3])
		scope := scopes.flat()
		if callee == "this" || callee == "super" {
			diagnostics = append(diagnostics, a.diagnoseConstructorChain(typ, member, callee, args, bodyOffset+match[2], bodyOffset+match[3], source, model)...)
			continue
		}
		diagnostics = append(diagnostics, checkUnknownCallArgs(typ, member, args, match[3], bodyOffset, source, scopes)...)
		if receiverType, method, ok := semaChainedCallReceiver(body, match[0], scope, model); ok {
			if collectionDiagnostics, handled := checkSemaCollectionCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
				diagnostics = append(diagnostics, collectionDiagnostics...)
				continue
			}
			if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
				diagnostics = append(diagnostics, platformDiagnostics...)
				continue
			}
			diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, method, resolveMemberMethods(model, receiverType, method), args, haveArgs, "instance", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
			continue
		}
		if strings.Contains(callee, ".") {
			receiver, method, ok := strings.Cut(callee, ".")
			if !ok || method == "" {
				continue
			}
			if receiver == "super" {
				diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, typ.SuperClass, method), args, haveArgs, "super", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
				continue
			}
			receiverType, ok := scope[normalizeName(receiver)]
			if ok {
				if collectionDiagnostics, handled := checkSemaCollectionCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
					diagnostics = append(diagnostics, collectionDiagnostics...)
					continue
				}
				if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
					diagnostics = append(diagnostics, platformDiagnostics...)
					continue
				}
				if isSemaBuiltinType(receiverType) {
					continue
				}
				if classMembers, ok := model[normalizeName(receiverType)]; ok {
					diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, classMembers.name, method), args, haveArgs, "instance", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
				}
				continue
			}
			if classMembers, ok := model[normalizeName(receiver)]; ok {
				diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, classMembers.name, method), args, haveArgs, "class", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
				continue
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
		diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, classMembers.name, callee), args, haveArgs, "implicit", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
	}
	return diagnostics
}

type resolvedMember struct {
	owner  string
	member typesys.MemberSymbol
}

func resolveMemberMethods(model map[string]typeMembers, typeName, method string) []resolvedMember {
	return resolveMemberMethodsSeen(model, typeName, method, make(map[string]bool))
}

func resolveMemberMethodsSeen(model map[string]typeMembers, typeName, method string, seen map[string]bool) []resolvedMember {
	key := normalizeName(typeName)
	if key == "" || seen[key] {
		return nil
	}
	seen[key] = true
	members, ok := model[key]
	if !ok {
		return nil
	}
	if direct := members.methods[normalizeName(method)]; len(direct) > 0 {
		resolved := make([]resolvedMember, 0, len(direct))
		for _, member := range direct {
			resolved = append(resolved, resolvedMember{owner: members.name, member: member})
		}
		return resolved
	}
	if inherited := resolveMemberMethodsSeen(model, members.superClass, method, seen); len(inherited) > 0 {
		return inherited
	}
	for _, iface := range members.interfaces {
		if inherited := resolveMemberMethodsSeen(model, iface, method, seen); len(inherited) > 0 {
			return inherited
		}
	}
	return nil
}

func (a *Analyzer) diagnoseConstructorChain(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, args []semaArg, start, end int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
	if member.Kind != apexast.DeclarationConstructor {
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, "constructor chaining is only valid inside constructors", start, end, source)}
	}
	targetType := typ.Name
	if callee == "super" {
		if typ.SuperClass == "" {
			if len(args) == 0 {
				return nil
			}
			return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, "super constructor call requires a superclass", start, end, source)}
		}
		targetType = typ.SuperClass
	}
	target, ok := model[normalizeName(targetType)]
	if !ok {
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, fmt.Sprintf("unknown constructor target %q", targetType), start, end, source)}
	}
	if len(target.constructors) == 0 && len(args) == 0 {
		return nil
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, map[string]string{}, model)
		if argTypes[i] == "" {
			for _, ctor := range target.constructors {
				if len(ctor.Parameters) == len(args) {
					return nil
				}
			}
		}
	}
	if _, ok, ambiguous := bestMemberByArgTypes(target.constructors, argTypes, model); ok {
		return nil
	} else if ambiguous {
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, fmt.Sprintf("ambiguous %s constructor with %d argument(s)", targetType, len(args)), start, end, source)}
	}
	return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, fmt.Sprintf("no matching %s constructor with %d argument(s)", targetType, len(args)), start, end, source)}
}

func constructorDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee, detail string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA011",
		Message:  fmt.Sprintf("%s %q has invalid %s(...) call: %s", member.Kind, member.Name, callee, detail),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func (a *Analyzer) diagnoseMethodCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, candidates []resolvedMember, args []semaArg, haveArgs bool, receiverMode string, start, end int, source string, scope map[string]string, model map[string]typeMembers) []diagnostic.Diagnostic {
	if len(candidates) == 0 {
		return []diagnostic.Diagnostic{unknownCallDiagnostic(typ, member, callee, start, end, source)}
	}
	if !haveArgs {
		return nil
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if candidate, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok {
		if staticDiagnostic, blocked := checkSemaStaticAccess(typ, member, callee, candidate, receiverMode, start, end, source); blocked {
			return []diagnostic.Diagnostic{staticDiagnostic}
		}
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, callee, candidate, start, end, source, model); blocked {
			return []diagnostic.Diagnostic{visibilityDiagnostic}
		}
		return nil
	} else if ambiguous {
		return []diagnostic.Diagnostic{ambiguousCallDiagnostic(typ, member, callee, len(args), start, end, source)}
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA009",
		Message:  fmt.Sprintf("%s %q has no matching overload for call %q with %d argument(s)", member.Kind, member.Name, callee, len(args)),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}}
}

func checkSemaStaticAccess(from typesys.TypeSymbol, context typesys.MemberSymbol, callee string, target resolvedMember, receiverMode string, start, end int, source string) (diagnostic.Diagnostic, bool) {
	isStatic := hasModifier(target.member.Modifiers, "static")
	switch receiverMode {
	case "class":
		if !isStatic {
			return staticAccessDiagnostic(from, context, callee, "instance method called through a type", start, end, source), true
		}
	case "instance":
		if isStatic {
			return staticAccessDiagnostic(from, context, callee, "static method called through an instance", start, end, source), true
		}
	case "implicit":
		if hasModifier(context.Modifiers, "static") && !isStatic {
			return staticAccessDiagnostic(from, context, callee, "instance method called from a static context", start, end, source), true
		}
	}
	return diagnostic.Diagnostic{}, false
}

func staticAccessDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee, detail string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA027",
		Message:  fmt.Sprintf("%s %q has invalid static access for %q: %s", member.Kind, member.Name, callee, detail),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func unsupportedLocalFeatureDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, feature string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA028",
		Message:  fmt.Sprintf("%s %q uses unsupported local feature %q", member.Kind, member.Name, feature),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func ambiguousCallDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, argc, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA022",
		Message:  fmt.Sprintf("%s %q has ambiguous overloads for call %q with %d argument(s)", member.Kind, member.Name, callee, argc),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func checkSemaMemberAccess(from typesys.TypeSymbol, context typesys.MemberSymbol, callee string, target resolvedMember, start, end int, source string, model map[string]typeMembers) (diagnostic.Diagnostic, bool) {
	access := accessModifier(target.member.Modifiers)
	if access == "" || access == "public" || access == "global" || access == "webservice" {
		return diagnostic.Diagnostic{}, false
	}
	if from.IsTest && hasModifier(target.member.Modifiers, "testvisible") {
		return diagnostic.Diagnostic{}, false
	}
	allowed := false
	switch access {
	case "private":
		allowed = normalizeName(from.Name) == normalizeName(target.owner)
	case "protected":
		allowed = normalizeName(from.Name) == normalizeName(target.owner) || semaIsSubclass(model, from.Name, target.owner)
	default:
		allowed = true
	}
	if allowed {
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA010",
		Message:  fmt.Sprintf("%s %q cannot access %s %s %q", context.Kind, context.Name, access, target.member.Kind, callee),
		File:     from.File,
		Range:    semaRange(source, start, end),
	}, true
}

func semaIsSubclass(model map[string]typeMembers, child, parent string) bool {
	seen := make(map[string]bool)
	for child != "" {
		key := normalizeName(child)
		if seen[key] {
			return false
		}
		seen[key] = true
		members, ok := model[key]
		if !ok {
			return false
		}
		if normalizeName(members.superClass) == normalizeName(parent) {
			return true
		}
		child = members.superClass
	}
	return false
}

func accessModifier(modifiers []string) string {
	for _, modifier := range modifiers {
		switch strings.ToLower(strings.TrimPrefix(modifier, "@")) {
		case "private", "protected", "public", "global", "webservice":
			return strings.ToLower(strings.TrimPrefix(modifier, "@"))
		}
	}
	return ""
}

func callArgsMatch(params []apexast.Parameter, args []semaArg, scope map[string]string, model map[string]typeMembers) bool {
	if len(params) != len(args) {
		return false
	}
	for i, arg := range args {
		argType := inferSemaArgTypeWithModel(arg.text, scope, model)
		if semaConversionScore(params[i].Type, argType, model) < 0 {
			return false
		}
	}
	return true
}

func bestResolvedMemberByArgTypes(candidates []resolvedMember, argTypes []string, model map[string]typeMembers) (resolvedMember, bool, bool) {
	applicable := make([]resolvedMember, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicable(candidate.member, argTypes, model) {
			applicable = append(applicable, candidate)
		}
	}
	return bestResolvedMemberBySpecificity(applicable, model)
}

func bestMemberByArgTypes(candidates []typesys.MemberSymbol, argTypes []string, model map[string]typeMembers) (typesys.MemberSymbol, bool, bool) {
	applicable := make([]typesys.MemberSymbol, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicable(candidate, argTypes, model) {
			applicable = append(applicable, candidate)
		}
	}
	return bestMemberBySpecificity(applicable, model)
}

func memberApplicable(candidate typesys.MemberSymbol, argTypes []string, model map[string]typeMembers) bool {
	if len(candidate.Parameters) != len(argTypes) {
		return false
	}
	for i, param := range candidate.Parameters {
		if semaConversionScore(param.Type, argTypes[i], model) < 0 {
			return false
		}
	}
	return true
}

func bestResolvedMemberBySpecificity(applicable []resolvedMember, model map[string]typeMembers) (resolvedMember, bool, bool) {
	if len(applicable) == 0 {
		return resolvedMember{}, false, false
	}
	bestIndex := -1
	for i, candidate := range applicable {
		moreSpecificThanAll := true
		for j, other := range applicable {
			if i == j {
				continue
			}
			switch compareSemaMemberSpecificity(candidate.member, other.member, model) {
			case -1, 2:
				moreSpecificThanAll = false
			}
			if !moreSpecificThanAll {
				break
			}
		}
		if moreSpecificThanAll {
			if bestIndex >= 0 && compareSemaMemberSpecificity(candidate.member, applicable[bestIndex].member, model) == 0 {
				continue
			}
			if bestIndex >= 0 {
				return resolvedMember{}, false, true
			}
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return resolvedMember{}, false, true
	}
	return applicable[bestIndex], true, false
}

func bestMemberBySpecificity(applicable []typesys.MemberSymbol, model map[string]typeMembers) (typesys.MemberSymbol, bool, bool) {
	if len(applicable) == 0 {
		return typesys.MemberSymbol{}, false, false
	}
	bestIndex := -1
	for i, candidate := range applicable {
		moreSpecificThanAll := true
		for j, other := range applicable {
			if i == j {
				continue
			}
			switch compareSemaMemberSpecificity(candidate, other, model) {
			case -1, 2:
				moreSpecificThanAll = false
			}
			if !moreSpecificThanAll {
				break
			}
		}
		if moreSpecificThanAll {
			if bestIndex >= 0 && compareSemaMemberSpecificity(candidate, applicable[bestIndex], model) == 0 {
				continue
			}
			if bestIndex >= 0 {
				return typesys.MemberSymbol{}, false, true
			}
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return typesys.MemberSymbol{}, false, true
	}
	return applicable[bestIndex], true, false
}

func compareSemaMemberSpecificity(left, right typesys.MemberSymbol, model map[string]typeMembers) int {
	leftBetter := false
	rightBetter := false
	for i := range left.Parameters {
		switch compareSemaTypeSpecificity(left.Parameters[i].Type, right.Parameters[i].Type, model) {
		case 1:
			leftBetter = true
		case -1:
			rightBetter = true
		case 2:
			return 2
		}
		if leftBetter && rightBetter {
			return 2
		}
	}
	switch {
	case leftBetter:
		return 1
	case rightBetter:
		return -1
	default:
		return 0
	}
}

func compareSemaTypeSpecificity(left, right string, model map[string]typeMembers) int {
	if strings.EqualFold(left, right) {
		return 0
	}
	leftToRight := semaAssignableToType(right, left, model)
	rightToLeft := semaAssignableToType(left, right, model)
	switch {
	case leftToRight && !rightToLeft:
		return 1
	case rightToLeft && !leftToRight:
		return -1
	case !leftToRight && !rightToLeft:
		return 2
	default:
		return 0
	}
}

func semaConversionScore(paramType, argType string, model map[string]typeMembers) int {
	if argType == "" || strings.EqualFold(argType, "null") {
		return 1
	}
	if strings.EqualFold(paramType, argType) {
		return 1000
	}
	if score := semaNumericConversionScore(paramType, argType); score >= 0 {
		return score
	}
	if semaGenericAssignableToType(paramType, argType, model) {
		return 850
	}
	if strings.EqualFold(normalizeArrayType(paramType), "SObject") && isSemaSObjectLike(argType, model) {
		return 750
	}
	if strings.EqualFold(paramType, "Object") {
		return 10
	}
	if distance, ok := semaTypeDistance(model, argType, paramType, make(map[string]bool)); ok {
		return 800 - distance
	}
	return -1
}

func semaNumericConversionScore(paramType, argType string) int {
	switch argType {
	case "Integer":
		switch paramType {
		case "Long":
			return 900
		case "Decimal":
			return 800
		case "Double":
			return 700
		}
	case "Long":
		switch paramType {
		case "Decimal":
			return 800
		case "Double":
			return 700
		}
	case "Decimal":
		if paramType == "Double" {
			return 800
		}
	}
	return -1
}

func semaAssignableToType(paramType, argType string, model map[string]typeMembers) bool {
	paramType = normalizeArrayType(paramType)
	argType = normalizeArrayType(argType)
	if strings.EqualFold(paramType, argType) || strings.EqualFold(paramType, "Object") {
		return true
	}
	if semaGenericAssignableToType(paramType, argType, model) {
		return true
	}
	if strings.EqualFold(paramType, "SObject") && isSemaSObjectLike(argType, model) {
		return true
	}
	if strings.EqualFold(argType, "Integer") {
		return strings.EqualFold(paramType, "Long") || strings.EqualFold(paramType, "Decimal") || strings.EqualFold(paramType, "Double")
	}
	if strings.EqualFold(argType, "Long") {
		return strings.EqualFold(paramType, "Decimal") || strings.EqualFold(paramType, "Double")
	}
	if strings.EqualFold(argType, "Decimal") {
		return strings.EqualFold(paramType, "Double")
	}
	return semaTypeMatches(model, argType, paramType, make(map[string]bool))
}

func semaGenericAssignableToType(paramType, argType string, model map[string]typeMembers) bool {
	paramBase, paramArgs := semaGenericBaseAndArgs(paramType)
	argBase, argArgs := semaGenericBaseAndArgs(argType)
	if !strings.EqualFold(paramBase, argBase) {
		return false
	}
	switch normalizeName(paramBase) {
	case "list", "set":
		if len(paramArgs) == 0 {
			return true
		}
		if len(paramArgs) != 1 || len(argArgs) != 1 {
			return false
		}
		return semaAssignableToType(paramArgs[0], argArgs[0], model)
	case "map":
		if len(paramArgs) == 0 {
			return true
		}
		if len(paramArgs) != 2 || len(argArgs) != 2 {
			return false
		}
		return semaAssignableToType(paramArgs[0], argArgs[0], model) && semaAssignableToType(paramArgs[1], argArgs[1], model)
	default:
		return false
	}
}

func isSemaSObjectLike(typeName string, model map[string]typeMembers) bool {
	typeName = normalizeArrayType(strings.TrimSpace(typeName))
	if typeName == "" || strings.Contains(typeName, "<") {
		return false
	}
	if _, ok := model[normalizeName(typeName)]; ok {
		return false
	}
	switch normalizeName(typeName) {
	case "object", "string", "id", "boolean", "integer", "long", "double", "decimal", "date", "datetime", "time", "blob", "type", "exception":
		return false
	}
	if strings.HasSuffix(normalizeName(typeName), "__c") || strings.HasSuffix(normalizeName(typeName), "__mdt") {
		return true
	}
	if isCommonSemaSObjectName(typeName) {
		return true
	}
	for _, known := range vm.CommonSObjectTypeNames() {
		if strings.EqualFold(typeName, known) {
			return true
		}
	}
	return false
}

func isCommonSemaSObjectName(typeName string) bool {
	switch normalizeName(typeName) {
	case "account", "contact", "opportunity", "lead", "campaign", "campaignmember", "case", "task", "event", "user", "profile", "organization", "staticresource", "product2", "pricebook2", "pricebookentry", "recordtype":
		return true
	default:
		return false
	}
}

func semaTypeDistance(model map[string]typeMembers, typeName, target string, seen map[string]bool) (int, bool) {
	key := normalizeName(typeName)
	targetKey := normalizeName(target)
	if key == "" || seen[key] {
		return 0, false
	}
	if key == targetKey {
		return 0, true
	}
	seen[key] = true
	members, ok := model[key]
	if !ok {
		return 0, false
	}
	best := 0
	found := false
	if distance, ok := semaTypeDistance(model, members.superClass, target, seen); ok {
		best = distance + 1
		found = true
	}
	for _, iface := range members.interfaces {
		if distance, ok := semaTypeDistance(model, iface, target, seen); ok {
			distance++
			if !found || distance < best {
				best = distance
				found = true
			}
		}
	}
	return best, found
}

func semaTypeMatches(model map[string]typeMembers, typeName, target string, seen map[string]bool) bool {
	key := normalizeName(typeName)
	targetKey := normalizeName(target)
	if key == "" || seen[key] {
		return false
	}
	if key == targetKey {
		return true
	}
	seen[key] = true
	members, ok := model[key]
	if !ok {
		return false
	}
	if semaTypeMatches(model, members.superClass, target, seen) {
		return true
	}
	for _, iface := range members.interfaces {
		if semaTypeMatches(model, iface, target, seen) {
			return true
		}
	}
	return false
}

func inferSemaArgTypeWithModel(arg string, scope map[string]string, model map[string]typeMembers) string {
	arg = strings.TrimSpace(arg)
	if condition, whenTrue, whenFalse, ok := splitSemaTernary(strings.TrimSpace(arg)); ok {
		inferSemaArgTypeWithModel(condition, scope, model)
		trueType := inferSemaArgTypeWithModel(whenTrue, scope, model)
		falseType := inferSemaArgTypeWithModel(whenFalse, scope, model)
		return semaCommonType(trueType, falseType, model)
	}
	if castType, _, ok := splitSemaCast(arg); ok {
		return castType
	}
	if match := newExprPattern.FindStringSubmatch(arg); len(match) == 2 {
		return match[1]
	}
	if strings.HasSuffix(arg, ".class") {
		return "Type"
	}
	if typ := inferSemaMethodCallType(arg, scope, model); typ != "" {
		return typ
	}
	return inferSemaArgType(arg, scope)
}

func inferSemaMethodCallType(arg string, scope map[string]string, model map[string]typeMembers) string {
	arg = strings.TrimSpace(arg)
	if receiverExpr, method, args, ok := splitLastSemaCall(arg); ok {
		receiverType := inferSemaArgTypeWithModel(receiverExpr, scope, model)
		if receiverType == "" {
			if scoped, ok := scope[normalizeName(receiverExpr)]; ok {
				receiverType = scoped
			} else {
				receiverType = receiverExpr
			}
		}
		return semaResolvedCallReturnType(model, receiverType, method, args, scope)
	}
	open := strings.Index(arg, "(")
	if open < 0 || !strings.HasSuffix(arg, ")") {
		return ""
	}
	callee := strings.TrimSpace(arg[:open])
	args, haveArgs := callArgumentsAt(arg, open)
	if !haveArgs {
		return ""
	}
	if strings.Contains(callee, ".") {
		receiver, method, ok := strings.Cut(callee, ".")
		if !ok || method == "" {
			return ""
		}
		receiverType := receiver
		if scoped, ok := scope[normalizeName(receiver)]; ok {
			receiverType = scoped
		}
		return semaResolvedCallReturnType(model, receiverType, method, args, scope)
	}
	return ""
}

func splitLastSemaCall(arg string) (string, string, []semaArg, bool) {
	if !strings.HasSuffix(arg, ")") {
		return "", "", nil, false
	}
	open := matchingOpenParenBefore(arg, len(arg)-1)
	if open < 0 {
		return "", "", nil, false
	}
	methodEnd := open
	methodStart := methodEnd
	for methodStart > 0 && isIdentifierByte(arg[methodStart-1]) {
		methodStart--
	}
	if methodStart == methodEnd || methodStart == 0 || arg[methodStart-1] != '.' {
		return "", "", nil, false
	}
	receiver := strings.TrimSpace(arg[:methodStart-1])
	if receiver == "" {
		return "", "", nil, false
	}
	args, haveArgs := callArgumentsAt(arg, open)
	if !haveArgs {
		return "", "", nil, false
	}
	return receiver, strings.TrimSpace(arg[methodStart:methodEnd]), args, true
}

func semaResolvedCallReturnType(model map[string]typeMembers, receiverType, method string, args []semaArg, scope map[string]string) string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if sig, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return sig.returnType
	}
	if sig, ok := semaPlatformMethodSignature(receiverType, method); ok {
		return sig.returnType
	}
	if candidate, ok, _ := bestResolvedMemberByArgTypes(resolveMemberMethods(model, receiverType, method), argTypes, model); ok {
		return candidate.member.Type
	}
	return ""
}

func splitSemaTernary(arg string) (string, string, string, bool) {
	question, colon, ok := semaTernaryPositions(arg)
	if !ok {
		return "", "", "", false
	}
	condition := strings.TrimSpace(arg[:question])
	whenTrue := strings.TrimSpace(arg[question+1 : colon])
	whenFalse := strings.TrimSpace(arg[colon+1:])
	return condition, whenTrue, whenFalse, condition != "" && whenTrue != "" && whenFalse != ""
}

func semaTernaryPositions(arg string) (int, int, bool) {
	depth := 0
	question := -1
	for i := 0; i < len(arg); i++ {
		switch arg[i] {
		case '\'':
			i = skipSemaString(arg, i)
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case '?':
			if depth == 0 {
				question = i
			}
		case ':':
			if depth == 0 && question >= 0 {
				return question, i, true
			}
		}
	}
	return -1, -1, false
}

func splitSemaCast(arg string) (string, string, bool) {
	if !strings.HasPrefix(arg, "(") {
		return "", "", false
	}
	close := strings.IndexByte(arg, ')')
	if close < 0 {
		return "", "", false
	}
	typeName := strings.TrimSpace(arg[1:close])
	value := strings.TrimSpace(arg[close+1:])
	if typeName == "" || value == "" {
		return "", "", false
	}
	if strings.HasPrefix(value, ".") || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "*") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "%") {
		return "", "", false
	}
	if !typeReferencePattern.MatchString(typeName) {
		return "", "", false
	}
	return typeName, value, true
}

func semaCommonType(leftType, rightType string, model map[string]typeMembers) string {
	switch {
	case leftType == "":
		return rightType
	case rightType == "":
		return leftType
	case strings.EqualFold(leftType, "null"):
		return rightType
	case strings.EqualFold(rightType, "null"):
		return leftType
	case semaAssignableToType(leftType, rightType, model):
		return leftType
	case semaAssignableToType(rightType, leftType, model):
		return rightType
	case isSemaNumericType(leftType) && isSemaNumericType(rightType):
		return semaNumericResultType(leftType, rightType)
	default:
		return "Object"
	}
}

type semaCollectionSignature struct {
	returnType string
	params     [][]string
}

func semaCollectionMethodSignature(receiverType, method string) (semaCollectionSignature, bool) {
	receiverType = normalizeArrayType(receiverType)
	base, args := semaGenericBaseAndArgs(receiverType)
	method = normalizeName(method)
	switch normalizeName(base) {
	case "list":
		if len(args) == 0 {
			args = []string{"Object"}
		}
		if len(args) != 1 {
			return semaCollectionSignature{}, false
		}
		switch method {
		case "get":
			return semaCollectionSignature{returnType: args[0], params: [][]string{{"Integer"}}}, true
		case "add":
			return semaCollectionSignature{returnType: "void", params: [][]string{{args[0]}, {"Integer", args[0]}}}, true
		case "addall":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"List<" + args[0] + ">"}, {"Set<" + args[0] + ">"}}}, true
		case "clear", "sort":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}}}, true
		case "size", "hashcode":
			return semaCollectionSignature{returnType: "Integer"}, true
		case "indexof":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{args[0]}}}, true
		case "isempty":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "contains":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{args[0]}}}, true
		case "equals":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"List<" + args[0] + ">"}}}, true
		case "remove":
			return semaCollectionSignature{returnType: args[0], params: [][]string{{"Integer"}}}, true
		case "set":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Integer", args[0]}}}, true
		case "tostring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "clone", "deepclone":
			return semaCollectionSignature{returnType: "List<" + args[0] + ">"}, true
		}
	case "set":
		if len(args) == 0 {
			args = []string{"Object"}
		}
		if len(args) != 1 {
			return semaCollectionSignature{}, false
		}
		switch method {
		case "size", "hashcode":
			return semaCollectionSignature{returnType: "Integer"}, true
		case "add", "contains", "remove":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{args[0]}}}, true
		case "addall", "containsall", "removeall", "retainall":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"List<" + args[0] + ">"}, {"Set<" + args[0] + ">"}}}, true
		case "clear":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}}}, true
		case "isempty":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "equals":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"Set<" + args[0] + ">"}}}, true
		case "tostring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "clone":
			return semaCollectionSignature{returnType: "Set<" + args[0] + ">"}, true
		}
	case "map":
		if len(args) == 0 {
			args = []string{"Object", "Object"}
		}
		if len(args) != 2 {
			return semaCollectionSignature{}, false
		}
		switch method {
		case "get":
			return semaCollectionSignature{returnType: args[1], params: [][]string{{args[0]}}}, true
		case "put":
			return semaCollectionSignature{returnType: args[1], params: [][]string{{args[0], args[1]}}}, true
		case "putall":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Map<" + args[0] + "," + args[1] + ">"}, {"List<" + args[1] + ">"}}}, true
		case "keyset":
			return semaCollectionSignature{returnType: "Set<" + args[0] + ">"}, true
		case "values":
			return semaCollectionSignature{returnType: "List<" + args[1] + ">"}, true
		case "size", "hashcode":
			return semaCollectionSignature{returnType: "Integer"}, true
		case "containskey", "containsvalue", "isempty":
			if method == "containsvalue" {
				return semaCollectionSignature{returnType: "Boolean", params: [][]string{{args[1]}}}, true
			}
			if method == "containskey" {
				return semaCollectionSignature{returnType: "Boolean", params: [][]string{{args[0]}}}, true
			}
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "equals":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"Map<" + args[0] + "," + args[1] + ">"}}}, true
		case "remove":
			return semaCollectionSignature{returnType: args[1], params: [][]string{{args[0]}}}, true
		case "clear":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}}}, true
		case "tostring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "clone", "deepclone":
			return semaCollectionSignature{returnType: "Map<" + args[0] + "," + args[1] + ">"}, true
		}
	}
	return semaCollectionSignature{}, false
}

func checkSemaCollectionCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []semaArg, start, end int, source string, scope map[string]string, model map[string]typeMembers) ([]diagnostic.Diagnostic, bool) {
	sig, ok := semaCollectionMethodSignature(receiverType, method)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if semaArgsMatchAny(sig.params, argTypes, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), start, end, source)}, true
}

func semaPlatformMethodSignature(receiverType, method string) (semaCollectionSignature, bool) {
	method = normalizeName(method)
	switch normalizeName(receiverType) {
	case "auth.communitiesutil":
		if method == "isguestuser" {
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		}
	case "auth.authtoken":
		if method == "revokeaccess" {
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String", "String", "String"}}}, true
		}
	case "auth.sessionmanagement":
		if method == "getcurrentsession" {
			return semaCollectionSignature{returnType: "Map<String,String>", params: [][]string{{}}}, true
		}
	case "auth.authconfiguration":
		switch method {
		case "getauthproviders":
			return semaCollectionSignature{returnType: "List<AuthProvider>", params: [][]string{{}}}, true
		case "getauthconfig":
			return semaCollectionSignature{returnType: "Auth.AuthConfig", params: [][]string{{}}}, true
		case "getstarturl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getauthproviderssourl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String", "String", "String"}}}, true
		}
	case "auth.jwt":
		switch method {
		case "setiss":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}}}, true
		case "tojsonstring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		}
	case "cache.org":
		if method == "getpartition" {
			return semaCollectionSignature{returnType: "Cache.OrgPartition", params: [][]string{{"String"}}}, true
		}
	case "cache.session":
		if method == "getpartition" {
			return semaCollectionSignature{returnType: "Cache.SessionPartition", params: [][]string{{"String"}}}, true
		}
	case "cache.orgpartition", "cache.sessionpartition":
		switch method {
		case "get":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"String"}, {"Type", "String"}}}, true
		case "put":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "Object"}, {"String", "Object", "Integer"}, {"String", "Object", "Integer", "Cache.Visibility", "Boolean"}}}, true
		case "remove":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"String"}, {"Type", "String"}}}, true
		case "contains":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String"}}}, true
		}
	case "connectapi.organization":
		if method == "getsettings" {
			return semaCollectionSignature{returnType: "ConnectApi.OrganizationSettings", params: [][]string{{}}}, true
		}
	case "connectapi.communities":
		if method == "getcommunity" {
			return semaCollectionSignature{returnType: "ConnectApi.Community", params: [][]string{{"String"}}}, true
		}
	case "connectapi.userprofiles":
		switch method {
		case "setphoto":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String", "String", "Object"}}}, true
		case "deletephoto":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String"}}}, true
		}
	case "metadata.operations":
		switch method {
		case "enqueuedeployment":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"Metadata.DeployContainer", "Metadata.DeployCallback"}}}, true
		case "checkdeploystatus":
			return semaCollectionSignature{returnType: "Metadata.DeployResult", params: [][]string{{"Id"}, {"Id", "Boolean"}, {"String"}, {"String", "Boolean"}}}, true
		case "retrieve":
			return semaCollectionSignature{returnType: "List<Metadata.Metadata>", params: [][]string{{"Metadata.MetadataType", "List<String>"}, {"Metadata.MetadataType", "List<String>", "Boolean"}}}, true
		}
	case "metadata.deploycontainer":
		if method == "addmetadata" {
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Object"}}}, true
		}
	case "site":
		switch method {
		case "getsiteid", "getbaseurl", "getpathprefix", "getadminemail", "getadminid", "getmasterlabel", "geterrormessage", "geterrordescription":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "isregistrationenabled", "isloginenabled":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "isvalidusername":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String"}}}, true
		case "setexperienceid":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}}}, true
		case "forgotpassword":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}}}, true
		case "login":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{"String", "String", "String"}}}, true
		case "changepassword":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{"String", "String"}, {"String", "String", "String"}}}, true
		case "validatepassword":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"User", "String", "String"}}}, true
		case "createexternaluser":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"User", "String", "String"}, {"User", "String", "String", "Boolean"}}}, true
		case "createportaluser":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"User", "String", "String"}}}, true
		}
	case "system":
		switch method {
		case "setpassword":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String"}}}, true
		case "currentpagereference":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{}}}, true
		}
	case "usermanagement":
		switch method {
		case "initselfregistration":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"Auth.VerificationMethod", "User"}}}, true
		case "verifyselfregistration":
			return semaCollectionSignature{returnType: "Auth.VerificationResult", params: [][]string{{"Auth.VerificationMethod", "String", "String", "String"}}}, true
		}
	case "network":
		switch method {
		case "getnetworkid":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getloginurl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String"}}}, true
		case "communitieslanding":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{}}}, true
		}
	case "test":
		if method == "setcurrentpagereference" || method == "setcurrentpage" {
			return semaCollectionSignature{returnType: "void", params: [][]string{{"PageReference"}}}, true
		}
	case "apexpages":
		switch method {
		case "currentpage":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{}}}, true
		case "addmessage":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"ApexPages.Message"}}}, true
		case "hasmessages":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		}
	case "pagereference":
		switch method {
		case "getparameters":
			return semaCollectionSignature{returnType: "Map<String,String>", params: [][]string{{}}}, true
		case "geturl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "setredirect":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Boolean"}}}, true
		case "getredirect":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		}
	case "http":
		if method == "send" {
			return semaCollectionSignature{returnType: "HttpResponse", params: [][]string{{"HttpRequest"}}}, true
		}
	case "httpresponse":
		switch method {
		case "getbody":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getstatuscode":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		case "getheader":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String"}}}, true
		}
	case "multistaticresourcecalloutmock":
		switch method {
		case "setstaticresource":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String"}}}, true
		case "setstatuscode":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Integer"}}}, true
		case "setheader":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String"}}}, true
		}
	}
	return semaCollectionSignature{}, false
}

func checkSemaPlatformCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []semaArg, start, end int, source string, scope map[string]string, model map[string]typeMembers) ([]diagnostic.Diagnostic, bool) {
	sig, ok := semaPlatformMethodSignature(receiverType, method)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if semaArgsMatchAny(sig.params, argTypes, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), start, end, source)}, true
}

func semaArgsMatchAny(params [][]string, args []string, model map[string]typeMembers) bool {
	if len(params) == 0 {
		return len(args) == 0
	}
	for _, candidate := range params {
		if semaArgsMatch(candidate, args, model) {
			return true
		}
	}
	return false
}

func semaArgsMatch(params, args []string, model map[string]typeMembers) bool {
	if len(params) != len(args) {
		return false
	}
	for i, param := range params {
		arg := args[i]
		if arg == "" || strings.EqualFold(arg, "null") {
			continue
		}
		if !semaAssignableToType(param, arg, model) {
			return false
		}
	}
	return true
}

func collectionCallDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, method string, argc, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA023",
		Message:  fmt.Sprintf("%s %q has invalid collection call %q with %d argument(s)", member.Kind, member.Name, method, argc),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func collectionConstructorDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, typeName string, argc, start int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA025",
		Message:  fmt.Sprintf("%s %q has invalid %s initializer with %d argument(s)", member.Kind, member.Name, typeName, argc),
		File:     typ.File,
		Range:    semaRange(source, start, start+max(1, len(typeName))),
	}
}

func semaGenericBaseAndArgs(typeName string) (string, []string) {
	typeName = normalizeArrayType(typeName)
	typeName = strings.TrimSpace(typeName)
	open := strings.IndexByte(typeName, '<')
	if open < 0 || !strings.HasSuffix(typeName, ">") {
		return typeName, nil
	}
	base := strings.TrimSpace(typeName[:open])
	inner := strings.TrimSpace(typeName[open+1 : len(typeName)-1])
	if inner == "" {
		return base, nil
	}
	var args []string
	start := 0
	depth := 0
	for i, ch := range inner {
		switch ch {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[start:i]))
				start = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(inner[start:]))
	return base, args
}

func normalizeArrayType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	for strings.HasSuffix(typeName, "[]") {
		typeName = "List<" + strings.TrimSpace(typeName[:len(typeName)-2]) + ">"
	}
	return typeName
}

func semaIterableElementType(typeName string) (string, bool) {
	base, args := semaGenericBaseAndArgs(typeName)
	switch normalizeName(base) {
	case "list", "set":
		if len(args) == 1 {
			return args[0], true
		}
		return "", true
	default:
		return "", false
	}
}

func semaChainedCallReceiver(body string, callStart int, scope map[string]string, model map[string]typeMembers) (string, string, bool) {
	if callStart < 2 || body[callStart-1] != '.' {
		return "", "", false
	}
	methodStart := callStart
	methodEnd := callStart
	for methodEnd < len(body) && isIdentifierByte(body[methodEnd]) {
		methodEnd++
	}
	if methodEnd == methodStart || callStart < 3 || body[callStart-2] != ')' {
		return "", "", false
	}
	open := matchingOpenParenBefore(body, callStart-2)
	if open < 0 {
		return "", "", false
	}
	receiverExpr := strings.TrimSpace(body[:callStart-1])
	exprStart := semaExpressionStart(receiverExpr)
	receiverExpr = strings.TrimSpace(receiverExpr[exprStart:])
	receiverExpr = strings.TrimSpace(strings.TrimPrefix(receiverExpr, "return "))
	receiverType := inferSemaArgTypeWithModel(receiverExpr, scope, model)
	if receiverType == "" {
		receiverType = strings.TrimPrefix(receiverExpr, "new ")
	}
	if receiverType == "" {
		return "", "", false
	}
	return receiverType, strings.TrimSpace(body[methodStart:methodEnd]), true
}

func semaChainedCallReceiverNear(body string, pos int, method string, scope map[string]string, model map[string]typeMembers) (string, string, bool) {
	if receiverType, chainedMethod, ok := semaChainedCallReceiver(body, pos, scope, model); ok && strings.EqualFold(chainedMethod, method) {
		return receiverType, chainedMethod, true
	}
	start := pos - len(method) - 2
	if start < 0 {
		start = 0
	}
	end := pos + 256
	if end > len(body) {
		end = len(body)
	}
	needle := method
	search := body[start:end]
	for offset := 0; ; {
		idx := strings.Index(search[offset:], needle)
		if idx < 0 {
			return "", "", false
		}
		callStart := start + offset + idx
		beforeOK := callStart == 0 || !isIdentifierByte(body[callStart-1])
		after := callStart + len(needle)
		for after < len(body) && (body[after] == ' ' || body[after] == '\t' || body[after] == '\r' || body[after] == '\n') {
			after++
		}
		if beforeOK && after < len(body) && body[after] == '(' {
			if receiverType, chainedMethod, ok := semaChainedCallReceiver(body, callStart, scope, model); ok && strings.EqualFold(chainedMethod, method) {
				return receiverType, chainedMethod, true
			}
		}
		offset += idx + len(needle)
		if offset >= len(search) {
			return "", "", false
		}
	}
}

func semaExpressionStart(expr string) int {
	depth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		switch expr[i] {
		case ')', ']':
			depth++
		case '(', '[':
			if depth > 0 {
				depth--
				continue
			}
			return i + 1
		case ';', '{', '}', '\n':
			return i + 1
		case ',':
			if depth == 0 {
				return i + 1
			}
		case '=':
			if i == 0 || expr[i-1] != '!' && expr[i-1] != '=' && expr[i-1] != '<' && expr[i-1] != '>' {
				return i + 1
			}
		}
	}
	return 0
}

func matchingOpenParenBefore(body string, close int) int {
	depth := 0
	for i := close; i >= 0; i-- {
		switch body[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isIdentifierByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func checkUnknownCallArgs(typ typesys.TypeSymbol, member typesys.MemberSymbol, args []semaArg, pos, bodyOffset int, source string, scopes semaScopeModel) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, arg := range args {
		name := strings.TrimSpace(arg.text)
		if inferSemaArgType(name, scopes.flat()) != "" {
			continue
		}
		if !simpleIdentifierPattern.MatchString(name) {
			continue
		}
		if _, ok := scopes.visibleAt(name, pos); ok {
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "OAERSEMA013",
			Message:  fmt.Sprintf("%s %q references unknown variable %q", member.Kind, member.Name, name),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+arg.start, bodyOffset+arg.end),
		})
	}
	return diagnostics
}

func unknownCallDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERSEMA008",
		Message:  fmt.Sprintf("%s %q calls unknown method %q", member.Kind, member.Name, callee),
		File:     typ.File,
		Range:    semaRange(source, start, end),
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
		ns := normalizeName(a.namespace)
		prefix := ns + "."
		normalized := normalizeName(name)
		if strings.HasPrefix(normalized, prefix) {
			if _, ok := a.known[strings.TrimPrefix(normalized, prefix)]; ok {
				return true
			}
		}
		metadataPrefix := ns + "__"
		if strings.HasPrefix(normalized, metadataPrefix) {
			if _, ok := a.known[strings.TrimPrefix(normalized, metadataPrefix)]; ok {
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

func hasAnyAnnotation(modifiers []string, names ...string) bool {
	for _, name := range names {
		if hasModifier(modifiers, name) {
			return true
		}
	}
	return false
}

func hasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(modifierName(modifier), expected) {
			return true
		}
	}
	return false
}

func modifierName(modifier string) string {
	modifier = strings.TrimPrefix(strings.TrimSpace(modifier), "@")
	if idx := strings.IndexByte(modifier, '('); idx >= 0 {
		modifier = modifier[:idx]
	}
	return modifier
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
	typeIdentifierPattern   = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*`)
	typeReferencePattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?$`)
	localDeclPattern        = regexp.MustCompile(`(?m)(?:^|[;{}\n])\s*([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|;)`)
	enhancedForLocalPattern = regexp.MustCompile(`(?m)\bfor\s*\(\s*([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*:`)
	assignmentPattern       = regexp.MustCompile(`(?m)(?:^|[;{}\n])\s*([A-Za-z_][A-Za-z0-9_]*)\s=`)
	callPattern             = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s*\(`)
	constructorPattern      = regexp.MustCompile(`\bnew\s+([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?)\s*\(`)
	newExprPattern          = regexp.MustCompile(`(?is)^new\s+([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?)\s*(?:\([^)]*\)|\{.*\})\s*$`)
	decimalLiteralPattern   = regexp.MustCompile(`^-?(?:[0-9]+\.[0-9]*|[0-9]*\.[0-9]+)$`)
	intLiteralPattern       = regexp.MustCompile(`^-?[0-9]+$`)
	returnPattern           = regexp.MustCompile(`\breturn(?:\s+([^;\n]+))?\s*;`)
	simpleIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func extractBodyForSema(source string, r diagnostic.Range) (string, int, bool) {
	start := r.Start.Offset
	end := r.End.Offset
	if start < 0 || start >= len(source) || end <= start || end > len(source) {
		return "", 0, false
	}
	text := source[start:end]
	open := strings.IndexByte(text, '{')
	if open < 0 {
		return "", 0, false
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
				return text[open+1 : i], start + open + 1, true
			}
		}
	}
	return "", 0, false
}

func semaRange(source string, start, end int) *diagnostic.Range {
	if start < 0 || end < start || start > len(source) {
		return nil
	}
	if end > len(source) {
		end = len(source)
	}
	r := diagnostic.Range{Start: semaPosition(source, start), End: semaPosition(source, end)}
	return &r
}

func semaPosition(source string, offset int) diagnostic.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	line, column := 1, 1
	for i := 0; i < offset; i++ {
		if source[i] == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return diagnostic.Position{Line: line, Column: column, Offset: offset}
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

func blockBoundsAt(body string, pos int) (int, int) {
	start := 0
	stack := make([]int, 0)
	for i := 0; i < len(body) && i < pos; i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '{':
			stack = append(stack, i)
		case '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) > 0 {
		start = stack[len(stack)-1]
	} else {
		return 0, len(body)
	}
	depth := 0
	for i := start; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return start, i
			}
			depth--
			if depth == 0 {
				return start, i
			}
		}
	}
	return start, len(body)
}

type semaToken struct {
	text       string
	start, end int
}

func constructorTypes(body string) []semaToken {
	matches := constructorPattern.FindAllStringSubmatchIndex(body, -1)
	out := make([]semaToken, 0, len(matches))
	for _, match := range matches {
		out = append(out, semaToken{text: strings.TrimSpace(body[match[2]:match[3]]), start: match[2], end: match[3]})
	}
	return out
}

type semaArg struct {
	text       string
	start, end int
}

func callArgumentsAt(body string, calleeEnd int) ([]semaArg, bool) {
	open := strings.IndexByte(body[calleeEnd:], '(')
	if open < 0 {
		return nil, false
	}
	open += calleeEnd
	depth := 0
	start := open + 1
	var args []semaArg
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				if arg := trimSemaArg(body, start, i); arg.text != "" {
					args = append(args, arg)
				}
				return args, true
			}
		case ',':
			if depth == 1 {
				args = append(args, trimSemaArg(body, start, i))
				start = i + 1
			}
		}
	}
	return nil, false
}

func trimSemaArg(body string, start, end int) semaArg {
	for start < end && (body[start] == ' ' || body[start] == '\t' || body[start] == '\n' || body[start] == '\r') {
		start++
	}
	for end > start && (body[end-1] == ' ' || body[end-1] == '\t' || body[end-1] == '\n' || body[end-1] == '\r') {
		end--
	}
	return semaArg{text: body[start:end], start: start, end: end}
}

func semaStatementEnd(body string, start int) int {
	for i := start; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case ';', '\n':
			return i
		}
	}
	return len(body)
}

func inferSemaArgType(arg string, scope map[string]string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return ""
	}
	if strings.HasPrefix(arg, "!") && strings.EqualFold(inferSemaArgType(strings.TrimSpace(arg[1:]), scope), "Boolean") {
		return "Boolean"
	}
	if _, _, ok := splitSemaInstanceOf(arg); ok {
		return "Boolean"
	}
	if match := newExprPattern.FindStringSubmatch(arg); len(match) == 2 {
		return match[1]
	}
	if strings.HasSuffix(arg, ".class") {
		return "Type"
	}
	if typ := inferSemaBinaryType(arg, scope); typ != "" {
		return typ
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
	if decimalLiteralPattern.MatchString(arg) {
		return "Decimal"
	}
	if intLiteralPattern.MatchString(arg) {
		return "Integer"
	}
	if typ, ok := scope[normalizeName(arg)]; ok {
		return typ
	}
	if receiver, name, ok := strings.Cut(arg, "."); ok && strings.EqualFold(receiver, "Page") && strings.TrimSpace(name) != "" {
		return "PageReference"
	}
	return ""
}

func splitSemaInstanceOf(arg string) (string, string, bool) {
	depth := 0
	const op = " instanceof "
	for i := 0; i <= len(arg)-len(op); i++ {
		switch arg[i] {
		case '\'':
			i = skipSemaString(arg, i)
			continue
		case '(', '[', '{', '<':
			depth++
			continue
		case ')', ']', '}', '>':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 && strings.EqualFold(arg[i:i+len(op)], op) {
			left := strings.TrimSpace(arg[:i])
			right := strings.TrimSpace(arg[i+len(op):])
			return left, right, left != "" && right != ""
		}
	}
	return "", "", false
}

func inferSemaBinaryType(arg string, scope map[string]string) string {
	for _, op := range []string{"&&", "||"} {
		if left, right, ok := splitSemaBinary(arg, op); ok {
			if strings.EqualFold(inferSemaArgType(left, scope), "Boolean") && strings.EqualFold(inferSemaArgType(right, scope), "Boolean") {
				return "Boolean"
			}
			return ""
		}
	}
	for _, op := range []string{"==", "!=", "<=", ">=", "<", ">"} {
		if _, _, ok := splitSemaBinary(arg, op); ok {
			return "Boolean"
		}
	}
	for _, op := range []string{"+", "-", "*", "/"} {
		left, right, ok := splitSemaBinary(arg, op)
		if !ok {
			continue
		}
		leftType := inferSemaArgType(left, scope)
		rightType := inferSemaArgType(right, scope)
		if op == "+" && (strings.EqualFold(leftType, "String") || strings.EqualFold(rightType, "String")) {
			return "String"
		}
		if isSemaNumericType(leftType) && isSemaNumericType(rightType) {
			if strings.EqualFold(leftType, "Decimal") || strings.EqualFold(rightType, "Decimal") || strings.EqualFold(leftType, "Double") || strings.EqualFold(rightType, "Double") {
				return "Decimal"
			}
			if strings.EqualFold(leftType, "Long") || strings.EqualFold(rightType, "Long") {
				return "Long"
			}
			return "Integer"
		}
	}
	return ""
}

func splitSemaBinary(arg, op string) (string, string, bool) {
	depth := 0
	for i := 0; i <= len(arg)-len(op); i++ {
		switch arg[i] {
		case '\'':
			i = skipSemaString(arg, i)
			continue
		case '(', '[', '{':
			depth++
			continue
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 && strings.HasPrefix(arg[i:], op) {
			if op == "-" && strings.TrimSpace(arg[:i]) == "" {
				continue
			}
			return strings.TrimSpace(arg[:i]), strings.TrimSpace(arg[i+len(op):]), true
		}
	}
	return "", "", false
}

func isSemaNumericType(typeName string) bool {
	return strings.EqualFold(typeName, "Integer") || strings.EqualFold(typeName, "Long") || strings.EqualFold(typeName, "Decimal") || strings.EqualFold(typeName, "Double")
}

func skipSemaCall(callee string) bool {
	switch normalizeName(callee) {
	case "if", "for", "while", "switch", "catch", "new", "return", "system.assert", "system.assertequals", "system.debug":
		return true
	default:
		return false
	}
}

func isSemaConstructorCallAt(body string, start int) bool {
	i := start - 1
	for i >= 0 && (body[i] == ' ' || body[i] == '\t' || body[i] == '\r' || body[i] == '\n') {
		i--
	}
	if i < 0 {
		return false
	}
	end := i + 1
	for i >= 0 && isSemaIdentifierChar(body[i]) {
		i--
	}
	if !strings.EqualFold(body[i+1:end], "new") {
		return false
	}
	return i < 0 || !isSemaIdentifierChar(body[i])
}

func isSemaIdentifierChar(ch byte) bool {
	return ch == '_' || ch == '$' || ch == '.' || (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isLikelyTypeReference(text string) bool {
	text = strings.TrimSpace(text)
	return text != "" && text[0] >= 'A' && text[0] <= 'Z' && typeReferencePattern.MatchString(text)
}

func isSemaKeyword(text string) bool {
	switch normalizeName(text) {
	case "return", "throw", "if", "for", "while", "switch", "catch", "else", "do",
		"insert", "update", "upsert", "delete", "undelete", "merge":
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
	"Cache.CacheBuilder",
	"Cache.Org",
	"Cache.OrgPartition",
	"Cache.Session",
	"Cache.SessionPartition",
	"Cache.Visibility",
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
	"Auth",
	"Auth.AuthConfig",
	"Auth.AuthConfiguration",
	"Auth.AuthToken",
	"Auth.CommunitiesUtil",
	"Auth.JWT",
	"Auth.RegistrationHandler",
	"Auth.SessionManagement",
	"Auth.UserData",
	"Auth.VerificationMethod",
	"Auth.VerificationResult",
	"AuthProvider",
	"ConnectApi",
	"ConnectApi.Communities",
	"ConnectApi.Community",
	"ConnectApi.Organization",
	"ConnectApi.OrganizationSettings",
	"ConnectApi.TimeZone",
	"ConnectApi.UserProfiles",
	"ConnectApi.UserSettings",
	"Http",
	"HttpCalloutMock",
	"HTTPRequest",
	"HttpRequest",
	"HTTPResponse",
	"HttpResponse",
	"Iterable",
	"Matcher",
	"Messaging",
	"Metadata",
	"Metadata.CustomMetadata",
	"Metadata.CustomMetadataValue",
	"Metadata.CustomObject",
	"Metadata.CustomField",
	"Metadata.DeployCallback",
	"Metadata.DeployCallBack",
	"Metadata.DeployCallbackContext",
	"Metadata.DeployContainer",
	"Metadata.DeployDetails",
	"Metadata.DeployMessage",
	"Metadata.DeployResult",
	"Metadata.DeployStatus",
	"Metadata.Metadata",
	"Metadata.MetadataType",
	"Metadata.Operations",
	"Metadata.AsyncResult",
	"MultiStaticResourceCalloutMock",
	"Network",
	"ObjectPermissions",
	"OrgWideEmailAddress",
	"Organization",
	"PageReference",
	"Pattern",
	"PermissionSetAssignment",
	"Profile",
	"Database.BatchableContext",
	"Queueable",
	"QueueableContext",
	"RecentlyViewed",
	"RestRequest",
	"RestResponse",
	"Schedulable",
	"SchedulableContext",
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
	"Site",
	"Site.ExternalUserCreateException",
	"Site.UrlRewriter",
	"StaticResource",
	"System",
	"Test",
	"TriggerOperation",
	"User",
	"UserManagement",
	"UserLicense",
	"VisualEditor",
	"VisualEditor.DataRow",
	"VisualEditor.DynamicPickListRows",
}
