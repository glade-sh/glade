package sema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
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
	for _, name := range storage.KnownStandardObjectNames() {
		a.addKnown(name, TypePlatform, "")
	}
	for _, name := range semaAdditionalStandardSObjectNames() {
		a.addKnown(name, TypePlatform, "")
	}
	return a
}

func Analyze(index typesys.Index) Result {
	return NewAnalyzer().Analyze(index)
}

func (a *Analyzer) Analyze(index typesys.Index) (result Result) {
	a.namespace = index.Project.Namespace
	index = enrichIndexWithStandardSymbols(index)
	result = Result{
		Project:     index.Project,
		Diagnostics: append([]diagnostic.Diagnostic{}, index.Diagnostics...),
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA000",
				Message:  fmt.Sprintf("internal sema panic: %v", recovered),
			})
			result.Summary.Diagnostics = len(result.Diagnostics)
		}
	}()

	for _, object := range index.Objects {
		a.addKnown(object.Name, TypeSchema, "")
	}
	for _, typ := range index.Types {
		if !typ.Dependency {
			a.addKnown(typ.Name, TypeApex, typ.File)
		} else if typ.Namespace == "" {
			a.addKnown(typ.Name, TypePlatform, typ.File)
		}
		if typ.Namespace != "" {
			kind := TypeApex
			if typ.Dependency {
				kind = TypePlatform
			}
			a.addKnown(typ.Namespace+"."+typ.Name, kind, typ.File)
		}
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationClass || member.Kind == apexast.DeclarationInterface || member.Kind == apexast.DeclarationEnum {
				if !typ.Dependency {
					a.addKnown(member.Name, TypeApex, typ.File)
					a.addKnown(typ.Name+"."+member.Name, TypeApex, typ.File)
				}
				if typ.Namespace != "" {
					a.addKnown(typ.Namespace+"."+typ.Name+"."+member.Name, TypeApex, typ.File)
				}
			}
		}
	}

	result.Diagnostics = append(result.Diagnostics, a.checkTriggers(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkMemberTypes(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkMethodParameters(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkAnnotations(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkMethodBodies(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkVisibility(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkManagedPackageAccess(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkInheritanceContracts(index)...)
	result.Diagnostics = append(result.Diagnostics, a.checkSchemaReferences(index)...)

	result.Summary = Summary{
		Types:       countProjectTypes(index.Types),
		Triggers:    len(index.Triggers),
		Objects:     len(index.Objects),
		Diagnostics: len(result.Diagnostics),
	}
	result.Types = a.exportKnownTypes()
	return result
}

func countProjectTypes(types []typesys.TypeSymbol) int {
	count := 0
	for _, typ := range types {
		if !typ.Dependency {
			count++
		}
	}
	return count
}

func enrichIndexWithStandardSymbols(index typesys.Index) typesys.Index {
	seen := make(map[string]bool, len(index.Types))
	for _, typ := range index.Types {
		seen[semaTypeKey(typ.Namespace, typ.Name)] = true
	}
	for _, typ := range typesys.StandardPlatformSymbols() {
		if seen[semaTypeKey(typ.Namespace, typ.Name)] {
			continue
		}
		seen[semaTypeKey(typ.Namespace, typ.Name)] = true
		index.Types = append(index.Types, typ)
	}
	return index
}

func semaTypeKey(namespace, name string) string {
	if namespace == "" {
		return normalizeName(name)
	}
	return normalizeName(namespace + "." + name)
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
			Code:     "GLADESEMA001",
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
		if typ.Artifact {
			continue
		}
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
					Code:     "GLADESEMA002",
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
		if typ.Artifact {
			continue
		}
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
						Code:     "GLADESEMA004",
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
					Code:     "GLADESEMA003",
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
		Code:     "GLADESEMA026",
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
				Code:     "GLADESEMA005",
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
					Code:     "GLADESEMA005",
					Message:  fmt.Sprintf("interface method %q cannot be private or protected", member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
	}
	return diagnostics
}

func (a *Analyzer) checkManagedPackageAccess(index typesys.Index) []diagnostic.Diagnostic {
	dependencyNamespaces := make(map[string]typesys.DependencyInfo)
	for _, dep := range index.Dependencies {
		if dep.Status == "loaded" {
			dependencyNamespaces[strings.ToLower(dep.Namespace)] = dep
		}
	}
	if len(dependencyNamespaces) == 0 {
		return nil
	}
	typesByNamespace := make(map[string][]typesys.TypeSymbol)
	for _, typ := range index.Types {
		if typ.Namespace == "" {
			continue
		}
		typesByNamespace[strings.ToLower(typ.Namespace)] = append(typesByNamespace[strings.ToLower(typ.Namespace)], typ)
	}
	var diagnostics []diagnostic.Diagnostic
	sourceCache := make(map[string]string)
	seen := make(map[string]bool)
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		source, ok := readSemaSource(typ.File, sourceCache)
		if !ok {
			continue
		}
		for namespace, dep := range dependencyNamespaces {
			for _, ref := range managedPackageReferences(source, dep.Namespace) {
				key := typ.File + ":" + ref.Namespace + "." + ref.TypeName + ":" + ref.MemberName
				if seen[key] {
					continue
				}
				seen[key] = true
				depType, ok := findManagedPackageType(typesByNamespace[namespace], ref.TypeName)
				if !ok {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     "dependency_unknown_symbol",
						Message:  fmt.Sprintf("managed package dependency %s does not expose type %q", dep.Namespace, ref.TypeName),
						File:     typ.File,
					})
					continue
				}
				if !hasModifier(depType.Modifiers, "global") && !hasModifier(depType.Modifiers, "webservice") {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     "dependency_access_denied",
						Message:  fmt.Sprintf("managed package dependency %s type %q is not global", dep.Namespace, depType.Name),
						File:     typ.File,
					})
					continue
				}
				if ref.MemberName == "" {
					continue
				}
				member, ok := findManagedPackageMember(depType, ref.MemberName)
				if !ok {
					continue
				}
				if !hasModifier(member.Modifiers, "global") && !hasModifier(member.Modifiers, "webservice") {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     "dependency_member_access_denied",
						Message:  fmt.Sprintf("managed package dependency %s member %q on %q is not global", dep.Namespace, member.Name, depType.Name),
						File:     typ.File,
					})
				}
			}
		}
	}
	return diagnostics
}

type managedPackageReference struct {
	Namespace  string
	TypeName   string
	MemberName string
}

func managedPackageReferences(source, namespace string) []managedPackageReference {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(namespace) + `\.([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?)`)
	matches := pattern.FindAllStringSubmatch(source, -1)
	out := make([]managedPackageReference, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		parts := strings.Split(match[1], ".")
		ref := managedPackageReference{Namespace: namespace, TypeName: match[1]}
		if len(parts) > 1 {
			ref.TypeName = strings.Join(parts[:len(parts)-1], ".")
			ref.MemberName = parts[len(parts)-1]
		}
		out = append(out, ref)
	}
	return out
}

func findManagedPackageType(types []typesys.TypeSymbol, name string) (typesys.TypeSymbol, bool) {
	for _, typ := range types {
		if strings.EqualFold(typ.Name, name) {
			return typ, true
		}
	}
	for _, typ := range types {
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(typ.Name)+".") {
			return typ, true
		}
	}
	return typesys.TypeSymbol{}, false
}

func findManagedPackageMember(typ typesys.TypeSymbol, name string) (typesys.MemberSymbol, bool) {
	for _, member := range typ.Members {
		if strings.EqualFold(member.Name, name) {
			return member, true
		}
	}
	return typesys.MemberSymbol{}, false
}

func (a *Analyzer) checkInheritanceContracts(index typesys.Index) []diagnostic.Diagnostic {
	model := buildTypeMembers(index)
	defer unregisterSemaShortCandidateIndex(model)
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if typ.Artifact {
			continue
		}
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
					Code:     "GLADESEMA016",
					Message:  fmt.Sprintf("method %q is marked override but no inherited method has the same signature", member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
			if hasModifier(member.Modifiers, "abstract") && !abstractClass {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA017",
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
				Code:     "GLADESEMA017",
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
	members, _ := model[normalizeName(typ.Name)]
	superClass := typ.SuperClass
	interfaces := typ.Interfaces
	if members.name != "" {
		superClass = members.superClass
		interfaces = members.interfaces
	}
	for _, candidate := range resolveMemberMethods(model, superClass, member.Name) {
		if sameSemaSignature(candidate.member, member) {
			return true
		}
	}
	for _, iface := range interfaces {
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
	members, _ := model[normalizeName(typ.Name)]
	interfaces := typ.Interfaces
	superClass := typ.SuperClass
	if members.name != "" {
		interfaces = members.interfaces
		superClass = members.superClass
	}
	for _, iface := range interfaces {
		out = append(out, collectRequiredMethods(model, iface, "interface", seen)...)
	}
	for current := superClass; current != ""; {
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
	members, _, ok := semaLookupTypeMembers(model, typeName)
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
			if (sameSemaSignature(method, required) || semaOverrideCompatibleSignature(method, required, model)) && !hasModifier(method.Modifiers, "abstract") {
				return true
			}
		}
		current = members.superClass
	}
	return false
}

func semaOverrideCompatibleSignature(method, required typesys.MemberSymbol, model map[string]typeMembers) bool {
	if !strings.EqualFold(method.Name, required.Name) || len(method.Parameters) != len(required.Parameters) {
		return false
	}
	for i := range method.Parameters {
		if !semaAssignableToType(required.Parameters[i].Type, method.Parameters[i].Type, model) &&
			!semaAssignableToType(method.Parameters[i].Type, required.Parameters[i].Type, model) {
			return false
		}
	}
	return true
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

func collectClassicForLocals(body string) []semaLocal {
	var out []semaLocal
	for _, match := range forHeaderPattern.FindAllStringSubmatchIndex(body, -1) {
		open := match[1] - 1
		header, headerStart, ok := semaBalancedUntil(body, open, '(', ')')
		if !ok {
			continue
		}
		firstSemi := semaTopLevelByte(header, ';')
		if firstSemi < 0 {
			continue
		}
		init := strings.TrimSpace(header[:firstSemi])
		if init == "" {
			continue
		}
		typeName, rest, ok := splitSemaLeadingType(init)
		if !ok || isSemaKeyword(typeName) {
			continue
		}
		scopeEnd := len(body)
		if closeParen := headerStart + len(header); closeParen < len(body) {
			_, scopeEnd = blockBoundsAfter(body, closeParen+1)
		}
		for _, part := range splitTopLevelSemaList(rest) {
			name := semaDeclaratorName(part)
			if name == "" {
				continue
			}
			localStart := strings.Index(body[headerStart:headerStart+firstSemi], name)
			if localStart >= 0 {
				localStart += headerStart
			} else {
				localStart = headerStart
			}
			out = append(out, semaLocal{name: name, typeName: typeName, start: localStart + len(name), scopeStart: open, scopeEnd: scopeEnd})
		}
	}
	return out
}

func (a *Analyzer) collectAdditionalSemaLocalDecls(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes *semaScopeModel, model map[string]typeMembers, match []int) []diagnostic.Diagnostic {
	typeName := strings.TrimSpace(body[match[2]:match[3]])
	if isSemaKeyword(typeName) {
		return nil
	}
	if match[1] <= 0 || body[match[1]-1] == ';' {
		return nil
	}
	statementStart := match[1]
	if statementStart > 0 && body[statementStart-1] == ',' {
		statementStart--
	}
	end := semaStatementEnd(body, statementStart)
	if end <= statementStart || end > len(body) {
		return nil
	}
	scopeStart, scopeEnd := blockBoundsAt(body, match[0])
	var diagnostics []diagnostic.Diagnostic
	segment := body[statementStart:end]
	depth := 0
	for i := 0; i < len(segment); i++ {
		switch segment[i] {
		case '\'':
			i = skipSemaString(segment, i)
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}', '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth != 0 {
				continue
			}
			j := i + 1
			for j < len(segment) && isWhitespace(segment[j]) {
				j++
			}
			nameStart := j
			if j >= len(segment) || !((segment[j] >= 'A' && segment[j] <= 'Z') || (segment[j] >= 'a' && segment[j] <= 'z') || segment[j] == '_') {
				continue
			}
			j++
			for j < len(segment) && isIdentifierByte(segment[j]) {
				j++
			}
			name := segment[nameStart:j]
			for j < len(segment) && isWhitespace(segment[j]) {
				j++
			}
			if j+1 < len(segment) && segment[j] == '=' && segment[j+1] == '=' {
				continue
			}
			if j < len(segment) && segment[j] != '=' && segment[j] != ',' && segment[j] != ';' {
				continue
			}
			if _, exists := scopes.localInBlock(name, scopeStart, scopeEnd); exists {
				continue
			}
			visibleStart := semaLocalVisibleStart(body, statementStart+j, statementStart+j)
			scopes.locals = append(scopes.locals, semaLocal{name: name, typeName: resolveNestedTypeReference(model, typ.Name, typeName), start: visibleStart, scopeStart: scopeStart, scopeEnd: scopeEnd})
		}
	}
	return diagnostics
}

func (a *Analyzer) collectSemaLocalDecl(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes *semaScopeModel, model map[string]typeMembers, match []int) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if match[1] < len(body) && body[match[1]-1] == '=' && body[match[1]] == '=' {
		return nil
	}
	typeName := strings.TrimSpace(body[match[2]:match[3]])
	name := strings.TrimSpace(body[match[4]:match[5]])
	if isSemaKeyword(typeName) {
		return nil
	}
	scopeStart, scopeEnd := blockBoundsAt(body, match[0])
	visibleStart := semaLocalVisibleStart(body, match[1]-1, match[5])
	if existing, exists := scopes.localInBlock(name, scopeStart, scopeEnd); exists {
		if existing.start == visibleStart {
			return nil
		}
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA014",
			Message:  fmt.Sprintf("%s %q redeclares local variable %q in the same scope", member.Kind, member.Name, name),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+match[4], bodyOffset+match[5]),
		}}
	}
	for _, ref := range extractTypeNames(typeName) {
		if !a.hasKnown(ref) {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA006",
				Message:  fmt.Sprintf("%s %q declares local %q with unknown type %q", member.Kind, member.Name, name, ref),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+match[2], bodyOffset+match[3]),
			})
		}
	}
	if match[1] > 0 && body[match[1]-1] == '=' {
		value := trimSemaArg(body, match[1], semaStatementEnd(body, match[1]))
		resolvedTypeName := resolveNestedTypeReference(model, typ.Name, typeName)
		valueType := resolveNestedTypeReference(model, typ.Name, inferSemaArgTypeWithModel(value.text, scopes.flat(), model))
		if valueType != "" && valueType != "null" && !semaAssignableToType(resolvedTypeName, valueType, model) {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA018",
				Message:  fmt.Sprintf("%s %q initializes %s local %q with %s", member.Kind, member.Name, typeName, name, valueType),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+value.start, bodyOffset+value.end),
			})
		}
	}
	scopes.locals = append(scopes.locals, semaLocal{name: name, typeName: resolveNestedTypeReference(model, typ.Name, typeName), start: visibleStart, scopeStart: scopeStart, scopeEnd: scopeEnd})
	return diagnostics
}

func semaLocalVisibleStart(body string, delimiter, fallback int) int {
	if delimiter < 0 || delimiter >= len(body) || body[delimiter] != '=' {
		return fallback
	}
	end := semaStatementEnd(body, delimiter+1)
	if end <= delimiter {
		return fallback
	}
	return end
}

func semaBalancedUntil(body string, open int, openByte, closeByte byte) (string, int, bool) {
	if open < 0 || open >= len(body) || body[open] != openByte {
		return "", 0, false
	}
	depth := 0
	start := open + 1
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case openByte:
			depth++
		case closeByte:
			depth--
			if depth == 0 {
				return body[start:i], start, true
			}
		}
	}
	return "", 0, false
}

func semaTopLevelByte(text string, needle byte) int {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\'':
			i = skipSemaString(text, i)
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if text[i] == needle && depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitSemaLeadingType(text string) (string, string, bool) {
	text = strings.TrimSpace(text)
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ' ', '\t', '\r', '\n':
			if depth == 0 {
				return strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+1:]), true
			}
		}
	}
	return "", "", false
}

func splitTopLevelSemaList(text string) []string {
	var out []string
	start := 0
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\'':
			i = skipSemaString(text, i)
		case '/':
			if end, ok := skipSemaComment(text, i); ok {
				i = end
			}
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}', '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(text[start:]))
	return out
}

func semaDeclaratorName(text string) string {
	text = strings.TrimSpace(text)
	if idx := strings.IndexByte(text, '='); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	if !simpleIdentifierPattern.MatchString(text) {
		return ""
	}
	return text
}

func firstCatchType(typeName string) string {
	parts := strings.Split(typeName, "|")
	if len(parts) == 0 {
		return "Exception"
	}
	first := strings.TrimSpace(parts[0])
	if first == "" {
		return "Exception"
	}
	return first
}

func blockBoundsAfter(body string, pos int) (int, int) {
	for i := pos; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '{':
			return blockBoundsAt(body, i+1)
		case ';':
			return pos, pos
		}
	}
	return pos, len(body)
}

func (s semaScopeModel) visibleAt(name string, pos int) (string, bool) {
	key := normalizeName(name)
	bestStart := -1
	bestType := ""
	for i := range s.locals {
		local := s.locals[i]
		if normalizeName(local.name) == key && pos >= local.start && pos <= local.scopeEnd && local.start >= bestStart {
			bestStart = local.start
			bestType = local.typeName
		}
	}
	if bestStart >= 0 {
		return bestType, true
	}
	typeName, ok := s.base[key]
	return typeName, ok
}

func (s semaScopeModel) flat() map[string]string {
	out := make(map[string]string, len(s.base)+len(s.locals))
	for name, typeName := range s.base {
		out[name] = typeName
	}
	starts := make(map[string]int, len(s.locals))
	for _, local := range s.locals {
		key := normalizeName(local.name)
		if local.start >= starts[key] {
			out[key] = local.typeName
			starts[key] = local.start
		}
	}
	return out
}

func (s semaScopeModel) flatAt(pos int) map[string]string {
	out := make(map[string]string, len(s.base)+len(s.locals))
	for name, typeName := range s.base {
		out[name] = typeName
	}
	starts := make(map[string]int, len(s.locals))
	for _, local := range s.locals {
		key := normalizeName(local.name)
		if pos >= local.start && pos <= local.scopeEnd && local.start >= starts[key] {
			out[key] = local.typeName
			starts[key] = local.start
		}
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
		if semaOffsetInIgnoredText(body, match[0]) {
			continue
		}
		target := strings.TrimSpace(body[match[2]:match[3]])
		if semaAssignmentLooksLikeNamedArg(body, match[2]) {
			continue
		}
		if semaAssignmentLooksLikeMapEntry(body, match[1]) {
			continue
		}
		if semaAssignmentLooksLikeLocalDeclaration(body, match[2]) {
			continue
		}
		targetType, ok := scopes.visibleAt(target, match[2])
		if ok {
			value := trimSemaArg(body, match[1], semaStatementEnd(body, match[1]))
			valueType := inferSemaArgTypeWithModel(value.text, scopes.flat(), model)
			if valueType == "" || valueType == "null" || semaAssignableToType(targetType, valueType, model) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA018",
				Message:  fmt.Sprintf("%s %q assigns %s to %s variable %q", member.Kind, member.Name, valueType, targetType, target),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+value.start, bodyOffset+value.end),
			})
			continue
		}
		if semaAnyKnownField(model, target) {
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA013",
			Message:  fmt.Sprintf("%s %q assigns unknown variable %q", member.Kind, member.Name, target),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+match[2], bodyOffset+match[3]),
		})
	}
	return diagnostics
}

func semaAnyKnownField(model map[string]typeMembers, name string) bool {
	key := normalizeName(name)
	if key == "" || strings.Contains(key, ".") {
		return false
	}
	for _, members := range model {
		if _, ok := members.fields[key]; ok {
			return true
		}
	}
	return false
}

func semaAssignmentLooksLikeMapEntry(body string, afterEquals int) bool {
	for i := afterEquals; i < len(body); i++ {
		switch body[i] {
		case ' ', '\t', '\r', '\n':
			continue
		case '>':
			return true
		default:
			return false
		}
	}
	return false
}

func semaAssignmentLooksLikeLocalDeclaration(body string, targetStart int) bool {
	if targetStart <= 0 || targetStart > len(body) {
		return false
	}
	start := targetStart - 1
	for start >= 0 {
		switch body[start] {
		case ';', '{', '}':
			start++
			goto done
		default:
			start--
		}
	}
	start = 0
done:
	prefix := strings.TrimSpace(body[start:targetStart])
	return prefix != "" && typeReferencePattern.MatchString(prefix)
}

func semaAssignmentLooksLikeNamedArg(body string, targetStart int) bool {
	for i := targetStart - 1; i >= 0; i-- {
		switch body[i] {
		case ' ', '\t', '\r', '\n':
			continue
		case '(', ',':
			return true
		default:
			return false
		}
	}
	return false
}

func (a *Analyzer) checkBodyReturns(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	if member.Type == "" {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	returnType := strings.TrimSpace(member.Type)
	foundReturn := false
	for _, match := range returnPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaReturnMatchInIgnoredText(body, match) {
			continue
		}
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
		valueType := resolveNestedTypeReference(model, typ.Name, inferSemaArgTypeWithModel(value.text, scopes.flat(), model))
		if strings.EqualFold(returnType, "Boolean") && semaExprContainsComparison(value.text) {
			valueType = "Boolean"
		}
		if valueType == "" || valueType == "null" || semaAssignableToType(returnType, valueType, model) {
			continue
		}
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("returns %s from %s method", valueType, returnType), bodyOffset+value.start, bodyOffset+value.end, source))
	}
	if !foundReturn && !strings.EqualFold(returnType, "void") && !semaBodyEndsWithThrow(body) {
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s", returnType), member.Range.Start.Offset, member.Range.End.Offset, source))
	}
	return diagnostics
}

func semaBodyEndsWithThrow(body string) bool {
	body = strings.TrimSpace(body)
	if !strings.HasSuffix(body, ";") {
		return false
	}
	lastSemicolon := strings.LastIndex(strings.TrimSuffix(body, ";"), ";")
	if lastSemicolon >= 0 {
		body = strings.TrimSpace(body[lastSemicolon+1:])
	}
	return strings.HasPrefix(strings.ToLower(body), "throw ")
}

func semaExprContainsComparison(expr string) bool {
	return strings.Contains(expr, "==") || strings.Contains(expr, "!=") ||
		strings.Contains(expr, "<=") || strings.Contains(expr, ">=") ||
		strings.Contains(expr, "<") || strings.Contains(expr, ">")
}

func returnTypeDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, detail string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA019",
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
		if semaLocalDeclMatchInIgnoredText(body, match) {
			continue
		}
		if match[1] > 0 && body[match[1]-1] == '=' {
			exprs = append(exprs, trimSemaArg(body, match[1], semaStatementEnd(body, match[1])))
		}
	}
	for _, match := range assignmentPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaOffsetInIgnoredText(body, match[0]) {
			continue
		}
		exprs = append(exprs, trimSemaArg(body, match[1], semaStatementEnd(body, match[1])))
	}
	for _, match := range returnPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaReturnMatchInIgnoredText(body, match) {
			continue
		}
		if match[2] >= 0 {
			exprs = append(exprs, trimSemaArg(body, match[2], match[3]))
		}
	}
	return exprs
}

func semaLocalDeclMatchInIgnoredText(body string, match []int) bool {
	if len(match) >= 4 && match[2] >= 0 {
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		if semaLooksLikeSOQLClauseKeyword(typeName) {
			return true
		}
		return semaOffsetInIgnoredText(body, match[2])
	}
	return len(match) < 1 || match[0] < 0 || semaOffsetInIgnoredText(body, match[0])
}

func semaLooksLikeSOQLClauseKeyword(typeName string) bool {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "select", "find", "from", "where", "and", "or", "limit", "offset", "order", "group", "having":
		return true
	default:
		return false
	}
}

func semaReturnMatchInIgnoredText(body string, match []int) bool {
	if len(match) < 2 || match[0] < 0 || match[1] > len(body) {
		return true
	}
	keyword := strings.Index(strings.ToLower(body[match[0]:match[1]]), "return")
	if keyword < 0 {
		return true
	}
	return semaOffsetInIgnoredText(body, match[0]+keyword)
}

func semaOffsetInIgnoredText(body string, pos int) bool {
	if semaOffsetInSOQLLiteral(body, pos) {
		return true
	}
	inBlock := false
	for i := 0; i < len(body) && i < pos; i++ {
		if inBlock {
			if i+1 < len(body) && body[i] == '*' && body[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if body[i] == '\'' {
			end := skipSemaString(body, i)
			if pos <= end {
				return true
			}
			i = end
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '/' {
			lineEnd := strings.IndexAny(body[i+2:], "\r\n")
			if lineEnd < 0 || i+2+lineEnd >= pos {
				return true
			}
			i += 2 + lineEnd
		}
	}
	if inBlock {
		return true
	}
	lineStart := strings.LastIndexAny(body[:pos], "\r\n") + 1
	if comment := strings.Index(body[lineStart:pos], "//"); comment >= 0 {
		return true
	}
	return false
}

func semaOffsetInSOQLLiteral(body string, pos int) bool {
	for i := 0; i < len(body) && i < pos; i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '[':
			queryStart := i + 1
			for queryStart < len(body) && isWhitespace(body[queryStart]) {
				queryStart++
			}
			if !strings.HasPrefix(strings.ToLower(body[queryStart:]), "select") && !strings.HasPrefix(strings.ToLower(body[queryStart:]), "find") {
				continue
			}
			depth := 1
			for j := i + 1; j < len(body); j++ {
				switch body[j] {
				case '\'':
					j = skipSemaString(body, j)
				case '[':
					depth++
				case ']':
					depth--
					if depth == 0 {
						if pos > i && pos < j {
							return true
						}
						i = j
						j = len(body)
					}
				}
			}
		}
	}
	return false
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
			Code:     "GLADESEMA020",
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
				Code:     "GLADESEMA006",
				Message:  fmt.Sprintf("%s %q references unknown expression type %q", member.Kind, member.Name, ref),
				File:     typ.File,
				Range:    semaRange(source, start, start+max(1, len(typeName))),
			})
		}
	}
	return diagnostics
}

func semaLooksLikeLabelReference(expr string) bool {
	expr = strings.TrimSpace(expr)
	return strings.HasPrefix(normalizeName(expr), "system.label.") || strings.HasPrefix(normalizeName(expr), "label.")
}

func semaFallbackFieldPathType(fieldName string) string {
	key := normalizeName(fieldName)
	switch {
	case key == "id":
		return "Id"
	case key == "name":
		return "String"
	case strings.HasSuffix(key, "date"):
		return "Date"
	case strings.HasSuffix(key, "infos"), strings.HasSuffix(key, "items"), strings.HasSuffix(key, "records"), strings.HasSuffix(key, "list"):
		return "List<Object>"
	default:
		return ""
	}
}

func semaReceiverExprResolvesFieldPath(expr string, scope map[string]string, model map[string]typeMembers) bool {
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) < 2 {
		return false
	}
	receiverType := ""
	startIndex := 1
	if strings.EqualFold(parts[0], "this") && len(parts) > 1 {
		receiverType = scope[semaCurrentTypeScopeKey]
	} else if scoped, ok := scope[normalizeName(parts[0])]; ok {
		receiverType = scoped
	} else if members, ok := model[normalizeName(parts[0])]; ok {
		receiverType = members.name
	}
	if receiverType == "" || startIndex >= len(parts) {
		return false
	}
	_, ok := semaResolveFieldPath(model, receiverType, strings.Join(parts[startIndex:], "."))
	return ok
}

func semaStaticClassFieldPathType(model map[string]typeMembers, root, fieldPath string) (string, bool) {
	target, ok := semaStaticClassFieldPathMember(model, root, fieldPath)
	if !ok {
		return "", false
	}
	return target.member.Type, true
}

func semaStaticClassFieldPathMember(model map[string]typeMembers, root, fieldPath string) (resolvedMember, bool) {
	if root != "" && fieldPath != "" {
		parts := strings.Split(fieldPath, ".")
		for i := len(parts) - 1; i > 0; i-- {
			candidateRoot := root + "." + strings.Join(parts[:i], ".")
			candidateFieldPath := strings.Join(parts[i:], ".")
			if target, ok := semaStaticClassFieldPathMember(model, candidateRoot, candidateFieldPath); ok {
				return target, true
			}
		}
	}
	members, ok := model[normalizeName(root)]
	if !ok {
		canonical := semaCanonicalPlatformAlias(root)
		if strings.EqualFold(canonical, root) {
			return resolvedMember{}, false
		}
		members, ok = model[normalizeName(canonical)]
		if !ok {
			return resolvedMember{}, false
		}
	}
	target, ok := semaResolveFieldPath(model, members.name, fieldPath)
	if !ok || !hasModifier(target.member.Modifiers, "static") {
		return resolvedMember{}, false
	}
	return target, true
}

func semaStaticClassFieldPathMemberInContext(model map[string]typeMembers, currentType, root, fieldPath string) (resolvedMember, bool) {
	if currentType != "" {
		resolvedRoot := resolveNestedTypeName(model, currentType, root)
		if !strings.EqualFold(resolvedRoot, root) {
			if target, ok := semaStaticClassFieldPathMember(model, resolvedRoot, fieldPath); ok {
				return target, true
			}
		}
	}
	return semaStaticClassFieldPathMember(model, root, fieldPath)
}

func semaEnumValuePathType(model map[string]typeMembers, expr string) string {
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) < 2 {
		return ""
	}
	for i := len(parts) - 1; i > 0; i-- {
		typeName := strings.Join(parts[:i], ".")
		valueName := parts[i]
		members, ok := model[normalizeName(typeName)]
		if !ok || members.kind != apexast.DeclarationEnum {
			continue
		}
		if _, ok := members.fields[normalizeName(valueName)]; ok {
			return members.name
		}
	}
	return ""
}

func semaExprLooksLikeStaticSObjectToken(expr string, scope map[string]string) bool {
	return semaExprLooksLikeStaticSObjectTokenInModel(expr, scope, nil)
}

func semaExprLooksLikeStaticSObjectTokenInModel(expr string, scope map[string]string, model map[string]typeMembers) bool {
	root, _, ok := strings.Cut(strings.TrimSpace(expr), ".")
	if !ok || root == "" {
		return false
	}
	if _, scoped := scope[normalizeName(root)]; scoped {
		return false
	}
	return semaLooksLikeSObjectFieldTokenInModel(expr, model) || semaLooksLikeSObjectTypeTokenInModel(expr, model)
}

func semaLooksLikeSObjectFieldToken(expr string) bool {
	return semaLooksLikeSObjectFieldTokenInModel(expr, nil)
}

func semaLooksLikeSObjectFieldTokenInModel(expr string, model map[string]typeMembers) bool {
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) == 2 && semaFieldTokenPart(parts[0]) && semaFieldTokenPart(parts[1]) && isSemaSObjectLike(parts[0], model) && !strings.EqualFold(parts[1], "SObjectType") && !semaLooksLikeStaticConstantName(parts[1]) {
		return true
	}
	for i := 0; i+2 < len(parts); i++ {
		if strings.EqualFold(parts[i], "SObjectType") && strings.EqualFold(parts[i+1], "fields") && semaFieldTokenPart(parts[i+2]) {
			return true
		}
	}
	return false
}

func semaLooksLikeStaticConstantName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || !strings.Contains(name, "_") {
		return false
	}
	hasLetter := false
	for _, ch := range name {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasLetter = true
		case ch >= '0' && ch <= '9', ch == '_':
		default:
			return false
		}
	}
	return hasLetter
}

func semaLooksLikeSObjectTypeTokenInModel(expr string, model map[string]typeMembers) bool {
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) < 2 {
		return false
	}
	if len(parts) == 3 && strings.EqualFold(parts[0], "Schema") && strings.EqualFold(parts[1], "SObjectType") && isSemaSObjectLike(parts[2], model) {
		return true
	}
	if len(parts) >= 3 && strings.EqualFold(parts[len(parts)-1], "SObjectType") && strings.EqualFold(parts[len(parts)-2], "SObjectType") {
		return isSemaSObjectLike(parts[len(parts)-3], model)
	}
	if !strings.EqualFold(parts[len(parts)-1], "SObjectType") {
		return false
	}
	objectName := parts[len(parts)-2]
	return isSemaSObjectLike(objectName, model)
}

func semaFieldTokenPart(part string) bool {
	return simpleIdentifierPattern.MatchString(strings.TrimSpace(part))
}

func startsWithUpperASCII(text string) bool {
	text = strings.TrimSpace(text)
	return text != "" && text[0] >= 'A' && text[0] <= 'Z'
}

func semaLooksLikeSObjectTypeToken(expr string) bool {
	return semaLooksLikeSObjectTypeTokenInModel(expr, nil)
}

func splitSemaMethodPath(callee string) (string, string, bool) {
	depth := 0
	idx := -1
	for i := 0; i < len(callee); i++ {
		switch callee[i] {
		case '\'':
			i = skipSemaString(callee, i)
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case '.':
			if depth == 0 {
				idx = i
			}
		}
	}
	if idx <= 0 || idx >= len(callee)-1 {
		return "", "", false
	}
	return strings.TrimSpace(callee[:idx]), strings.TrimSpace(callee[idx+1:]), true
}

func inferSemaMethodCallType(arg string, scope map[string]string, model map[string]typeMembers) string {
	arg = strings.TrimSpace(arg)
	if receiverExpr, method, args, ok := splitLastSemaCall(arg); ok {
		if semaDatabaseDynamicQueryTextCall(receiverExpr + "." + method) {
			return "Database.QueryResult"
		}
		receiverType := semaTextReceiverType(receiverExpr, scope, model)
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
		if semaDatabaseDynamicQueryTextCall(callee) {
			return "Database.QueryResult"
		}
		receiver, method, ok := splitSemaMethodPath(callee)
		if !ok || method == "" {
			return ""
		}
		receiverType := semaTextReceiverType(receiver, scope, model)
		return semaResolvedCallReturnType(model, receiverType, method, args, scope)
	}
	if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
		return semaResolvedImplicitCallReturnType(model, currentType, callee, args, scope)
	}
	return ""
}

func semaTextReceiverType(receiver string, scope map[string]string, model map[string]typeMembers) string {
	receiver = strings.TrimSpace(receiver)
	if strings.EqualFold(receiver, "this") {
		return scope[semaCurrentTypeScopeKey]
	}
	if strings.EqualFold(receiver, "super") {
		if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
			if members, ok := model[normalizeName(currentType)]; ok {
				return members.superClass
			}
		}
		return ""
	}
	if strings.HasPrefix(strings.ToLower(receiver), "super.") {
		if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
			if members, ok := model[normalizeName(currentType)]; ok && members.superClass != "" {
				if target, ok := semaResolveFieldPath(model, members.superClass, strings.TrimPrefix(receiver, "super.")); ok {
					return target.member.Type
				}
			}
		}
	}
	receiverType := inferSemaArgTypeWithModel(receiver, scope, model)
	if receiverType != "" {
		return receiverType
	}
	if scoped, ok := scope[normalizeName(receiver)]; ok {
		return scoped
	}
	if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
		if resolved := resolveNestedTypeName(model, currentType, receiver); resolved != "" {
			if _, ok := model[normalizeName(resolved)]; ok {
				return resolved
			}
		}
	}
	return receiver
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
	dot := methodStart - 1
	for dot >= 0 && isWhitespace(arg[dot]) {
		dot--
	}
	if methodStart == methodEnd || dot < 0 || arg[dot] != '.' {
		return "", "", nil, false
	}
	receiver := strings.TrimSpace(arg[:dot])
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
	if stubbedType := semaCreateStubReturnTypeFromText(model, receiverType, method, args); stubbedType != "" {
		return stubbedType
	}
	if semaDatabaseDynamicQueryCall(receiverType, method) {
		return "Database.QueryResult"
	}
	if returnType := semaDatabaseDMLReturnType(receiverType, method, argTypes); returnType != "" {
		return returnType
	}
	if sig, ok := semaEnumMethodSignature(model, receiverType, method); ok {
		return sig.returnType
	}
	if sig, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return sig.returnType
	}
	if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok {
		return sig.returnType
	}
	if candidate, ok, _ := bestResolvedMemberByArgTypes(resolveMemberMethods(model, receiverType, method), argTypes, model); ok {
		return candidate.member.Type
	}
	return ""
}

func semaCreateStubReturnTypeFromIR(model map[string]typeMembers, receiverType, method string, args []ir.Expr, currentType string) string {
	if len(args) < 1 || !semaTestCreateStubCall(receiverType, method) {
		return ""
	}
	if args[0].Kind != ir.ExprVariable {
		return ""
	}
	return semaClassLiteralStubbedType(model, currentType, args[0].Name)
}

func semaCreateStubReturnTypeFromText(model map[string]typeMembers, receiverType, method string, args []semaArg) string {
	if len(args) < 1 || !semaTestCreateStubCall(receiverType, method) {
		return ""
	}
	return semaClassLiteralStubbedType(model, "", args[0].text)
}

func semaTestCreateStubCall(receiverType, method string) bool {
	return strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "Test") && strings.EqualFold(method, "createStub")
}

func semaClassLiteralStubbedType(model map[string]typeMembers, currentType, expr string) string {
	expr = strings.TrimSpace(expr)
	if !strings.HasSuffix(strings.ToLower(expr), ".class") {
		return ""
	}
	typeName := strings.TrimSpace(expr[:len(expr)-len(".class")])
	if typeName == "" {
		return ""
	}
	return resolveNestedTypeReference(model, currentType, typeName)
}

func semaDatabaseDynamicQueryCall(receiverType, method string) bool {
	if !strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "Database") {
		return false
	}
	switch normalizeName(method) {
	case "query", "querywithbinds":
		return true
	default:
		return false
	}
}

func semaDatabaseDMLReturnType(receiverType, method string, argTypes []string) string {
	if !strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "Database") || len(argTypes) == 0 {
		return ""
	}
	resultType := ""
	switch normalizeName(method) {
	case "insert", "update":
		resultType = "Database.SaveResult"
	case "delete":
		resultType = "Database.DeleteResult"
	case "upsert":
		resultType = "Database.UpsertResult"
	case "undelete":
		resultType = "Database.UndeleteResult"
	default:
		return ""
	}
	base, _ := semaGenericBaseAndArgs(argTypes[0])
	switch normalizeName(base) {
	case "list", "set":
		return "List<" + resultType + ">"
	default:
		return resultType
	}
}

func semaDatabaseDynamicQueryTextCall(callee string) bool {
	receiver, method, ok := strings.Cut(strings.TrimSpace(callee), ".")
	return ok && semaDatabaseDynamicQueryCall(receiver, method)
}

func semaResolvedImplicitCallReturnType(model map[string]typeMembers, receiverType, method string, args []semaArg, scope map[string]string) string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok && semaArgsMatchAny(sig.params, argTypes, model) {
		return sig.returnType
	}
	if candidate, ok, _ := bestResolvedMemberByArgTypes(resolveImplicitMemberMethods(model, receiverType, method), argTypes, model); ok {
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

func semaPlatformMethodSignatureFor(model map[string]typeMembers, receiverType, method string) (semaCollectionSignature, bool) {
	return semaPlatformMethodSignatureForMode(model, receiverType, method, "")
}

func semaPlatformMethodSignatureForMode(model map[string]typeMembers, receiverType, method, receiverMode string) (semaCollectionSignature, bool) {
	if sig, ok := semaPlatformMethodSignature(receiverType, method); ok {
		return sig, true
	}
	if isSemaSObjectLike(receiverType, model) && normalizeName(method) == "clone" {
		return semaCollectionSignature{returnType: receiverType, params: [][]string{{}, {"Boolean"}, {"Boolean", "Boolean"}, {"Boolean", "Boolean", "Boolean"}, {"Boolean", "Boolean", "Boolean", "Boolean"}}}, true
	}
	if sig, ok := semaGeneratedPlatformMethodSignature(model, receiverType, method, receiverMode); ok {
		return sig, true
	}
	if sig, ok := semaCustomDataStaticMethodSignature(receiverType, method); ok {
		return sig, true
	}
	if normalizeName(method) == "tostring" && receiverType != "" {
		return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
	}
	if normalizeName(method) == "equals" && receiverType != "" {
		return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"Object"}}}, true
	}
	if normalizeName(method) == "getclass" && receiverType != "" {
		return semaCollectionSignature{returnType: "Type", params: [][]string{{}}}, true
	}
	if strings.EqualFold(receiverType, "Address") && normalizeName(method) == "clone" {
		return semaCollectionSignature{returnType: "Address", params: [][]string{{}}}, true
	}
	if isSemaSObjectLike(receiverType, model) {
		switch normalizeName(method) {
		case "get":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"String"}, {"Schema.SObjectField"}}}, true
		case "put":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"String", "Object"}, {"Schema.SObjectField", "Object"}}}, true
		case "putsobject":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "SObject"}, {"Schema.SObjectField", "SObject"}}}, true
		case "getsobject":
			return semaCollectionSignature{returnType: "SObject", params: [][]string{{"String"}, {"Schema.SObjectField"}}}, true
		case "getsobjects":
			return semaCollectionSignature{returnType: "List<SObject>", params: [][]string{{"String"}, {"Schema.SObjectField"}}}, true
		case "isset":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String"}, {"Schema.SObjectField"}}}, true
		case "getsobjecttype":
			return semaCollectionSignature{returnType: "Schema.SObjectType", params: [][]string{{}}}, true
		case "getpopulatedfieldsasmap":
			return semaCollectionSignature{returnType: "Map<String,Object>", params: [][]string{{}}}, true
		case "clone":
			return semaCollectionSignature{returnType: receiverType, params: [][]string{{}, {"Boolean"}, {"Boolean", "Boolean"}, {"Boolean", "Boolean", "Boolean"}, {"Boolean", "Boolean", "Boolean", "Boolean"}}}, true
		case "adderror":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}, {"Exception"}, {"String", "Boolean"}, {"Exception", "Boolean"}, {"String", "String"}, {"String", "String", "Boolean"}, {"Schema.SObjectField", "String"}, {"Schema.SObjectField", "String", "Boolean"}}}, true
		case "haserrors":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "geterrors":
			return semaCollectionSignature{returnType: "List<Object>", params: [][]string{{}}}, true
		}
	}
	if semaTypeMatches(model, receiverType, "Exception", make(map[string]bool)) {
		return semaPlatformMethodSignature("Exception", method)
	}
	return semaCollectionSignature{}, false
}

func semaGeneratedPlatformMethodSignature(model map[string]typeMembers, receiverType, method, receiverMode string) (semaCollectionSignature, bool) {
	if strings.TrimSpace(receiverType) == "" || strings.TrimSpace(method) == "" {
		return semaCollectionSignature{}, false
	}
	candidates := resolveMemberMethods(model, receiverType, method)
	if len(candidates) == 0 {
		canonical := semaCanonicalPlatformAlias(receiverType)
		if !strings.EqualFold(canonical, receiverType) {
			candidates = resolveMemberMethods(model, canonical, method)
		}
	}
	if len(candidates) == 0 {
		return semaCollectionSignature{}, false
	}
	candidates = filterGeneratedPlatformMethodsByReceiverMode(candidates, receiverMode)
	if len(candidates) == 0 {
		return semaCollectionSignature{}, false
	}
	if owner, ok := model[normalizeName(candidates[0].owner)]; !ok || (!owner.dependency && !owner.sobject) {
		return semaCollectionSignature{}, false
	}
	returnType := strings.TrimSpace(candidates[0].member.Type)
	params := make([][]string, 0, len(candidates))
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		memberReturn := strings.TrimSpace(candidate.member.Type)
		if memberReturn == "" {
			memberReturn = "void"
		}
		if returnType == "" {
			returnType = memberReturn
		} else if !strings.EqualFold(returnType, memberReturn) {
			return semaCollectionSignature{}, false
		}
		memberParams := make([]string, 0, len(candidate.member.Parameters))
		for _, param := range candidate.member.Parameters {
			memberParams = append(memberParams, param.Type)
		}
		signature := strings.Join(memberParams, "\x00")
		if seen[signature] {
			continue
		}
		seen[signature] = true
		params = append(params, memberParams)
	}
	if returnType == "" {
		returnType = "void"
	}
	return semaCollectionSignature{returnType: returnType, params: params}, true
}

func filterGeneratedPlatformMethodsByReceiverMode(candidates []resolvedMember, receiverMode string) []resolvedMember {
	switch receiverMode {
	case "class":
		filtered := make([]resolvedMember, 0, len(candidates))
		for _, candidate := range candidates {
			if hasModifier(candidate.member.Modifiers, "static") {
				filtered = append(filtered, candidate)
			}
		}
		return filtered
	case "instance", "super":
		filtered := make([]resolvedMember, 0, len(candidates))
		for _, candidate := range candidates {
			if !hasModifier(candidate.member.Modifiers, "static") {
				filtered = append(filtered, candidate)
			}
		}
		return filtered
	default:
		return candidates
	}
}

func semaCustomDataStaticMethodSignature(receiverType, method string) (semaCollectionSignature, bool) {
	receiverType = strings.TrimSpace(receiverType)
	if receiverType == "" {
		return semaCollectionSignature{}, false
	}
	key := normalizeName(receiverType)
	if !strings.HasSuffix(key, "__mdt") && !strings.HasSuffix(key, "__c") {
		return semaCollectionSignature{}, false
	}
	switch normalizeName(method) {
	case "getall":
		return semaCollectionSignature{returnType: "Map<String," + receiverType + ">", params: [][]string{{}}}, true
	case "getinstance", "getvalues":
		return semaCollectionSignature{returnType: receiverType, params: [][]string{{}, {"String"}, {"Id"}}}, true
	case "getorgdefaults":
		return semaCollectionSignature{returnType: receiverType, params: [][]string{{}}}, true
	}
	return semaCollectionSignature{}, false
}

func checkSemaPlatformCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []semaArg, start, end int, source string, scope map[string]string, model map[string]typeMembers, receiverMode string) ([]diagnostic.Diagnostic, bool) {
	if strings.EqualFold(receiverType, "System") && strings.EqualFold(method, "runAs") && len(args) == 1 {
		return nil, true
	}
	if semaDatabaseDynamicQueryCall(receiverType, method) {
		return nil, true
	}
	if _, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return nil, false
	}
	if staticDiagnostic, blocked := checkGeneratedPlatformStaticAccess(typ, member, receiverType, method, receiverMode, start, end, source, model); blocked {
		return []diagnostic.Diagnostic{staticDiagnostic}, true
	}
	sig, ok := semaPlatformMethodSignatureForMode(model, receiverType, method, receiverMode)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if semaDatabaseDMLReturnType(receiverType, method, argTypes) != "" && len(args) <= 4 {
		return nil, true
	}
	if semaArgsMatchAny(sig.params, argTypes, model) || semaCollectionFieldPathArgsMatch(sig.params, args, scope, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), start, end, source)}, true
}

func checkGeneratedPlatformStaticAccess(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method, receiverMode string, start, end int, source string, model map[string]typeMembers) (diagnostic.Diagnostic, bool) {
	switch receiverMode {
	case "class", "instance", "implicit":
	default:
		return diagnostic.Diagnostic{}, false
	}
	candidates := resolveMemberMethods(model, receiverType, method)
	if len(candidates) == 0 {
		canonical := semaCanonicalPlatformAlias(receiverType)
		if !strings.EqualFold(canonical, receiverType) {
			candidates = resolveMemberMethods(model, canonical, method)
		}
	}
	if len(candidates) == 0 {
		return diagnostic.Diagnostic{}, false
	}
	if owner, ok := model[normalizeName(candidates[0].owner)]; !ok || (!owner.dependency && !owner.sobject) {
		return diagnostic.Diagnostic{}, false
	}
	if len(filterGeneratedPlatformMethodsByReceiverMode(candidates, receiverMode)) != 0 {
		return diagnostic.Diagnostic{}, false
	}
	return checkSemaStaticAccess(typ, member, method, candidates[0], receiverMode, start, end, source)
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

func semaArgTypesContainNullish(argTypes []string) bool {
	for _, argType := range argTypes {
		if argType == "" || strings.EqualFold(argType, "null") {
			return true
		}
	}
	return false
}

func semaSystemRunAsBlockCall(receiverType, method, callee string, args []ir.Expr) bool {
	if len(args) != 1 {
		return false
	}
	if strings.EqualFold(receiverType, "System") && strings.EqualFold(method, "runAs") {
		return true
	}
	receiver, method, ok := strings.Cut(callee, ".")
	return ok && strings.EqualFold(receiver, "System") && strings.EqualFold(method, "runAs")
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
		Code:     "GLADESEMA023",
		Message:  fmt.Sprintf("%s %q has invalid collection call %q with %d argument(s)", member.Kind, member.Name, method, argc),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func collectionConstructorDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, typeName string, argc, start int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA025",
		Message:  fmt.Sprintf("%s %q has invalid %s initializer with %d argument(s)", member.Kind, member.Name, typeName, argc),
		File:     typ.File,
		Range:    semaRange(source, start, start+max(1, len(typeName))),
	}
}

func semaGenericBaseAndArgs(typeName string) (string, []string) {
	typeName = semaStripTypeModifiers(typeName)
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
	typeName = semaStripTypeModifiers(typeName)
	typeName = strings.TrimSpace(typeName)
	if open := strings.LastIndexByte(typeName, '['); open > 0 && strings.HasSuffix(typeName, "]") {
		inner := strings.TrimSpace(typeName[open+1 : len(typeName)-1])
		if inner != "" && intLiteralPattern.MatchString(inner) {
			typeName = strings.TrimSpace(typeName[:open]) + "[]"
		}
	}
	for strings.HasSuffix(typeName, "[]") {
		typeName = "List<" + strings.TrimSpace(typeName[:len(typeName)-2]) + ">"
	}
	return typeName
}

func semaStripTypeModifiers(typeName string) string {
	for {
		trimmed := strings.TrimSpace(typeName)
		mod, rest, ok := strings.Cut(trimmed, " ")
		if !ok || !semaTypeModifier(mod) {
			return trimmed
		}
		typeName = rest
	}
}

func semaTypeModifier(word string) bool {
	switch normalizeName(word) {
	case "global", "public", "private", "protected", "static", "final", "virtual", "abstract", "override", "transient", "webservice", "testmethod":
		return true
	default:
		return false
	}
}

func semaIterableElementType(typeName string) (string, bool) {
	if strings.EqualFold(typeName, "Database.QueryResult") {
		return "SObject", true
	}
	base, args := semaGenericBaseAndArgs(typeName)
	switch normalizeName(base) {
	case "list", "set":
		if len(args) == 1 {
			return args[0], true
		}
		return "", true
	case "iterable":
		if len(args) == 1 {
			return args[0], true
		}
		return "", true
	default:
		return "", false
	}
}

func semaIterableElementTypeInModel(typeName string, model map[string]typeMembers) (string, bool) {
	if elementType, ok := semaIterableElementType(typeName); ok {
		return elementType, true
	}
	members, _, ok := semaLookupTypeMembers(model, typeName)
	if !ok {
		return "", false
	}
	for _, iface := range members.interfaces {
		base, args := semaGenericBaseAndArgs(iface)
		if strings.EqualFold(base, "Iterable") {
			if len(args) == 1 {
				return resolveNestedTypeReference(model, members.name, args[0]), true
			}
			return "", true
		}
	}
	return "", false
}

func semaChainedCallReceiver(body string, callStart int, scope map[string]string, model map[string]typeMembers, currentType string) (string, string, bool) {
	dot := callStart - 1
	for dot >= 0 && isWhitespace(body[dot]) {
		dot--
	}
	if dot < 1 || body[dot] != '.' {
		return "", "", false
	}
	methodStart := callStart
	methodEnd := callStart
	for methodEnd < len(body) && isIdentifierByte(body[methodEnd]) {
		methodEnd++
	}
	receiverEnd := dot - 1
	for receiverEnd >= 0 && isWhitespace(body[receiverEnd]) {
		receiverEnd--
	}
	if methodEnd == methodStart || receiverEnd < 0 || body[receiverEnd] != ')' {
		return "", "", false
	}
	open := matchingOpenParenBefore(body, receiverEnd)
	if open < 0 {
		return "", "", false
	}
	receiverCallStart := open
	for receiverCallStart > 0 && isIdentifierByte(body[receiverCallStart-1]) {
		receiverCallStart--
	}
	exprStart := semaExpressionStart(body[:callStart])
	if exprStart > dot {
		return "", "", false
	}
	fullReceiverExpr := strings.TrimSpace(body[exprStart:dot])
	fullReceiverType := ""
	if semaReceiverProbeIsBounded(fullReceiverExpr) {
		fullReceiverType = inferSemaArgTypeWithModel(fullReceiverExpr, scope, model)
	}
	if fullReceiverType == "" && receiverCallStart > exprStart && receiverCallStart < open && !semaReceiverCallLooksLikeConstructor(body[exprStart:receiverCallStart]) {
		exprStart = receiverCallStart
	}
	if exprStart > 0 && body[exprStart-1] == '(' {
		methodEnd := exprStart - 1
		methodStart := methodEnd
		for methodStart > 0 && isIdentifierByte(body[methodStart-1]) {
			methodStart--
		}
		dotBeforeMethod := methodStart - 1
		for dotBeforeMethod >= 0 && isWhitespace(body[dotBeforeMethod]) {
			dotBeforeMethod--
		}
		if dotBeforeMethod >= 0 && body[dotBeforeMethod] == '.' {
			exprStart = semaExpressionStart(body[:methodStart])
		}
	}
	if exprStart > open {
		exprStart = semaExpressionStart(body[:open])
	}
	if exprStart > dot {
		return "", "", false
	}
	receiverExpr := strings.TrimSpace(body[exprStart:dot])
	receiverExpr = strings.TrimSpace(strings.TrimPrefix(receiverExpr, "return "))
	receiverExpr = strings.TrimSpace(strings.TrimPrefix(receiverExpr, "!"))
	receiverExpr = semaTrimLeadingCast(receiverExpr)
	receiverType := fullReceiverType
	if receiverType == "" || !strings.EqualFold(receiverExpr, fullReceiverExpr) {
		receiverType = inferSemaArgTypeWithModel(receiverExpr, scope, model)
	}
	if receiverType == "" {
		if method, args, ok := splitBareSemaCall(receiverExpr); ok {
			receiverType = semaResolvedCallReturnType(model, currentType, method, args, scope)
		}
	}
	if receiverType == "" {
		receiverType = semaConstructorReceiverType(receiverExpr)
	}
	if receiverType == "" {
		return "", "", false
	}
	return receiverType, strings.TrimSpace(body[methodStart:methodEnd]), true
}

func semaReceiverProbeIsBounded(expr string) bool {
	if len(expr) > 512 {
		return false
	}
	return strings.Count(expr, "(") <= 8
}

func semaReceiverCallLooksLikeConstructor(prefix string) bool {
	fields := strings.Fields(strings.TrimSpace(prefix))
	return len(fields) > 0 && strings.EqualFold(fields[len(fields)-1], "new")
}

func semaTrimLeadingCast(expr string) string {
	if _, value, ok := splitSemaCast(strings.TrimSpace(expr)); ok {
		return value
	}
	return expr
}

func semaConstructorReceiverType(expr string) string {
	expr = strings.TrimSpace(expr)
	if !strings.HasPrefix(strings.ToLower(expr), "new ") {
		return ""
	}
	typ := strings.TrimSpace(expr[len("new "):])
	angleDepth := 0
	for i := 0; i < len(typ); i++ {
		switch typ[i] {
		case '\'':
			i = skipSemaString(typ, i)
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case '(', '{':
			if angleDepth != 0 {
				continue
			}
			closeByte := byte(')')
			if typ[i] == '{' {
				closeByte = '}'
			}
			depth := 0
			for j := i; j < len(typ); j++ {
				switch typ[j] {
				case '\'':
					j = skipSemaString(typ, j)
				case typ[i]:
					depth++
				case closeByte:
					depth--
					if depth == 0 {
						if strings.TrimSpace(typ[j+1:]) != "" {
							return ""
						}
						return strings.TrimSpace(typ[:i])
					}
				}
			}
			return ""
		}
	}
	return typ
}

func semaLooksLikeDottedCall(body string, callStart int) bool {
	for i := callStart - 1; i >= 0; i-- {
		if isWhitespace(body[i]) {
			continue
		}
		return body[i] == '.'
	}
	return false
}

func splitBareSemaCall(arg string) (string, []semaArg, bool) {
	arg = strings.TrimSpace(arg)
	if !strings.HasSuffix(arg, ")") {
		return "", nil, false
	}
	open := matchingOpenParenBefore(arg, len(arg)-1)
	if open <= 0 {
		return "", nil, false
	}
	method := strings.TrimSpace(arg[:open])
	if !simpleIdentifierPattern.MatchString(method) {
		return "", nil, false
	}
	args, haveArgs := callArgumentsAt(arg, open)
	return method, args, haveArgs
}

func semaChainedCallReceiverNear(body string, pos int, method string, scope map[string]string, model map[string]typeMembers, currentType string) (string, string, bool) {
	if receiverType, chainedMethod, ok := semaChainedCallReceiver(body, pos, scope, model, currentType); ok && strings.EqualFold(chainedMethod, method) {
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
	if start > len(body) {
		start = len(body)
	}
	if receiverType, chainedMethod, ok := semaChainedCallReceiverInRange(body, start, end, method, scope, model, currentType); ok {
		return receiverType, chainedMethod, true
	}
	return semaChainedCallReceiverInRange(body, 0, len(body), method, scope, model, currentType)
}

func semaChainedCallReceiverInRange(body string, start, end int, method string, scope map[string]string, model map[string]typeMembers, currentType string) (string, string, bool) {
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
			if receiverType, chainedMethod, ok := semaChainedCallReceiver(body, callStart, scope, model, currentType); ok && strings.EqualFold(chainedMethod, method) {
				return receiverType, chainedMethod, true
			}
		}
		offset += idx + len(needle)
		if offset >= len(search) {
			return "", "", false
		}
	}
}

func semaSourceHasDottedCall(body, method string) bool {
	needle := method
	for offset := 0; ; {
		idx := strings.Index(body[offset:], needle)
		if idx < 0 {
			return false
		}
		callStart := offset + idx
		before := callStart - 1
		for before >= 0 && isWhitespace(body[before]) {
			before--
		}
		after := callStart + len(needle)
		for after < len(body) && isWhitespace(body[after]) {
			after++
		}
		if before >= 0 && body[before] == '.' && after < len(body) && body[after] == '(' {
			return true
		}
		offset = callStart + len(needle)
		if offset >= len(body) {
			return false
		}
	}
}

func semaExpressionStart(expr string) int {
	depth := 0
	angleDepth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		switch expr[i] {
		case ')', ']', '}':
			depth++
		case '(', '[', '{':
			if depth > 0 {
				depth--
				continue
			}
			return i + 1
		case '>':
			if depth == 0 && (i == 0 || expr[i-1] != '=') {
				angleDepth++
			}
		case '<':
			if depth == 0 && angleDepth > 0 {
				angleDepth--
			} else if depth == 0 && i+1 < len(expr) && expr[i+1] != '=' {
				return i + 1
			}
		case ';':
			if depth == 0 && angleDepth == 0 {
				return i + 1
			}
		case '\n':
			if depth == 0 && angleDepth == 0 && semaContinuesWithDot(expr, i+1) {
				continue
			}
			if angleDepth == 0 {
				return i + 1
			}
		case ',':
			if depth == 0 && angleDepth == 0 {
				return i + 1
			}
		case '?', ':':
			if depth == 0 && angleDepth == 0 {
				return i + 1
			}
		case '+', '*', '/', '%':
			if depth == 0 && angleDepth == 0 {
				return i + 1
			}
		case '&', '|':
			if depth == 0 && angleDepth == 0 {
				return i + 1
			}
		case '!':
			if depth == 0 && angleDepth == 0 {
				return i + 1
			}
		case '-':
			if depth == 0 && angleDepth == 0 && i > 0 && !strings.ContainsRune("([{=,:?+-*/%&|", rune(expr[i-1])) {
				return i + 1
			}
		case '=':
			if depth == 0 && angleDepth == 0 && i+1 < len(expr) && expr[i+1] == '>' {
				return i + 2
			}
			if depth == 0 && angleDepth == 0 && (i == 0 || expr[i-1] != '!' && expr[i-1] != '=' && expr[i-1] != '<' && expr[i-1] != '>') {
				return i + 1
			}
		}
	}
	return 0
}

func semaContinuesWithDot(expr string, start int) bool {
	for i := start; i < len(expr); i++ {
		if isWhitespace(expr[i]) {
			continue
		}
		return expr[i] == '.'
	}
	return false
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
		if strings.EqualFold(name, "this") || strings.EqualFold(name, "super") {
			continue
		}
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
			Code:     "GLADESEMA013",
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
		Code:     "GLADESEMA008",
		Message:  fmt.Sprintf("%s %q calls unknown method %q", member.Kind, member.Name, callee),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func semaKnownAddressValueCall(callee string) bool {
	parts := strings.Split(strings.TrimSpace(callee), ".")
	if len(parts) < 2 {
		return false
	}
	method := normalizeName(parts[len(parts)-1])
	switch method {
	case "getstreet", "getcity", "getstate", "getstatecode", "getpostalcode", "getcountry", "getcountrycode", "getlatitude", "getlongitude", "getgeocodeaccuracy", "clone":
		return strings.HasSuffix(normalizeName(parts[len(parts)-2]), "address")
	default:
		return false
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
	typeReferencePattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*$`)
	lineLocalDeclPattern    = regexp.MustCompile(`(?m)^\s*(?:final\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:,|=|;)`)
	wrappedLocalDeclPattern = regexp.MustCompile(`(?m)^\s*(?:final\s+)?([A-Za-z_][^\n;=(){}]+)[ \t]*\r?\n\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	localDeclPattern        = regexp.MustCompile(`(?m)(?:^|[;\n])\s*(?:final\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:,|=|;)`)
	enhancedForLocalPattern = regexp.MustCompile(`(?im)\bfor\s*\(\s*(?:final\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*:`)
	forHeaderPattern        = regexp.MustCompile(`(?i)\bfor\s*\(`)
	catchLocalPattern       = regexp.MustCompile(`(?im)\bcatch\s*\(\s*([A-Za-z_][A-Za-z0-9_.]*(?:\s*\|\s*[A-Za-z_][A-Za-z0-9_.]*)*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
	assignmentPattern       = regexp.MustCompile(`(?m)(?:^|[;{}\n])\s*([A-Za-z_][A-Za-z0-9_]*)\s=`)
	callPattern             = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s*\(`)
	constructorPattern      = regexp.MustCompile(`(?i)\bnew\s+([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?)\s*\(`)
	newExprPattern          = regexp.MustCompile(`(?is)^new\s+([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*)\s*(?:\([^)]*\)|\{.*\})\s*$`)
	decimalLiteralPattern   = regexp.MustCompile(`^-?(?:[0-9]+\.[0-9]*|[0-9]*\.[0-9]+)$`)
	intLiteralPattern       = regexp.MustCompile(`^-?[0-9]+$`)
	returnPattern           = regexp.MustCompile(`(?is)(?:^|[;{}\n])\s*return(?:\s+([^;]+))?\s*;`)
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
		case '/':
			if end, ok := skipSemaComment(text, i); ok {
				i = end
			}
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
		if source[i] == '\\' && i+1 < len(source) {
			i++
			continue
		}
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

func skipSemaComment(source string, start int) (int, bool) {
	if start+1 >= len(source) || source[start] != '/' {
		return start, false
	}
	switch source[start+1] {
	case '/':
		if end := strings.IndexAny(source[start+2:], "\r\n"); end >= 0 {
			return start + 2 + end, true
		}
		return len(source) - 1, true
	case '*':
		if end := strings.Index(source[start+2:], "*/"); end >= 0 {
			return start + 2 + end + 1, true
		}
		return len(source) - 1, true
	default:
		return start, false
	}
}

func stripSemaComments(source string) string {
	var out strings.Builder
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\'':
			end := skipSemaString(source, i)
			out.WriteString(source[i : end+1])
			i = end
		case '/':
			if end, ok := skipSemaComment(source, i); ok {
				i = end
				continue
			}
			out.WriteByte(source[i])
		default:
			out.WriteByte(source[i])
		}
	}
	return out.String()
}

func blockBoundsAt(body string, pos int) (int, int) {
	start := 0
	stack := make([]int, 0)
	for i := 0; i < len(body) && i < pos; i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
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
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
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
		if semaOffsetInIgnoredText(body, match[0]) {
			continue
		}
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
	parenDepth := 0
	groupDepth := 0
	angleDepth := 0
	start := open + 1
	var args []semaArg
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
		case '(':
			if angleDepth == 0 && groupDepth == 0 {
				parenDepth++
			}
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 && (i == 0 || body[i-1] != '=') {
				angleDepth--
			}
		case '{', '[':
			if angleDepth == 0 {
				groupDepth++
			}
		case ')':
			if angleDepth == 0 && groupDepth == 0 {
				parenDepth--
			}
			if parenDepth == 0 {
				if arg := trimSemaArg(body, start, i); arg.text != "" {
					args = append(args, arg)
				}
				return args, true
			}
		case '}', ']':
			if angleDepth == 0 && groupDepth > 0 {
				groupDepth--
			}
		case ',':
			if parenDepth == 1 && groupDepth == 0 && angleDepth == 0 {
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
	return semaArg{text: strings.TrimSpace(stripSemaComments(body[start:end])), start: start, end: end}
}

func semaStatementEnd(body string, start int) int {
	depth := 0
	for i := start; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				return i
			}
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
	if literalType := semaKeywordLiteralType(arg); literalType != "" {
		return literalType
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
			right := semaLeadingTypeToken(strings.TrimSpace(arg[i+len(op):]))
			return left, right, left != "" && right != ""
		}
	}
	return "", "", false
}

func semaLeadingTypeToken(text string) string {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case '[', ']':
			continue
		default:
			if depth == 0 && !isSemaIdentifierChar(text[i]) {
				return strings.TrimSpace(text[:i])
			}
		}
	}
	return strings.TrimSpace(text)
}

func semaKeywordLiteralType(arg string) string {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "true", "false":
		return "Boolean"
	case "null":
		return "null"
	default:
		return ""
	}
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
	angleDepth := 0
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
		case '<':
			if depth == 0 && looksLikeSemaGenericOpen(arg, i) {
				angleDepth++
				continue
			}
		case '>':
			if angleDepth > 0 {
				angleDepth--
				continue
			}
		}
		if depth == 0 && angleDepth == 0 && strings.HasPrefix(arg[i:], op) {
			if op == "-" && strings.TrimSpace(arg[:i]) == "" {
				continue
			}
			return strings.TrimSpace(arg[:i]), strings.TrimSpace(arg[i+len(op):]), true
		}
	}
	return "", "", false
}

func looksLikeSemaGenericOpen(arg string, pos int) bool {
	left := pos - 1
	for left >= 0 && isWhitespace(arg[left]) {
		left--
	}
	right := pos + 1
	for right < len(arg) && isWhitespace(arg[right]) {
		right++
	}
	if left < 0 || right >= len(arg) || !isSemaIdentifierChar(arg[left]) || !isSemaIdentifierChar(arg[right]) {
		return false
	}
	depth := 0
	for i := pos; i < len(arg); i++ {
		switch arg[i] {
		case '\'':
			i = skipSemaString(arg, i)
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return true
			}
		case '(', '[', '{', ';', '=':
			if depth == 1 {
				return false
			}
		}
	}
	return false
}

func isSemaNumericType(typeName string) bool {
	return strings.EqualFold(typeName, "Integer") || strings.EqualFold(typeName, "Long") || strings.EqualFold(typeName, "Decimal") || strings.EqualFold(typeName, "Double")
}

func skipSemaCall(callee string) bool {
	switch normalizeName(callee) {
	case "if", "for", "while", "switch", "catch", "new", "return", "throw",
		"insert", "update", "upsert", "delete", "undelete", "merge", "on", "when",
		"__mapentry", "__coalesce", "__safe_call:get", "system.assert", "system.assertequals", "system.debug",
		"count", "count_distinct", "sum", "avg", "min", "max":
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
	if strings.HasSuffix(strings.ToLower(text), ".class") {
		text = strings.TrimSpace(text[:len(text)-len(".class")])
	}
	base, _ := semaGenericBaseAndArgs(text)
	switch normalizeName(base) {
	case "list", "set", "map", "iterable", "iterator":
		return typeReferencePattern.MatchString(text)
	}
	return text != "" && text[0] >= 'A' && text[0] <= 'Z' && typeReferencePattern.MatchString(text)
}

func isSemaKeyword(text string) bool {
	switch normalizeName(text) {
	case "return", "throw", "if", "for", "while", "switch", "catch", "else", "do", "when", "on",
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
	"AssertException",
	"AuraHandledException",
	"AsyncException",
	"CalloutException",
	"DmlException",
	"DMLException",
	"EmailException",
	"ExternalObjectException",
	"IllegalArgumentException",
	"IllegalStateException",
	"InvalidParameterValueException",
	"JSONException",
	"LimitException",
	"ListException",
	"MathException",
	"NoAccessException",
	"NoDataFoundException",
	"NoSuchElementException",
	"NullPointerException",
	"PatternSyntaxException",
	"QueryException",
	"RequiredFeatureMissingException",
	"Savepoint",
	"System.Savepoint",
	"SearchException",
	"SecurityException",
	"SerializationException",
	"SObjectException",
	"StringException",
	"TypeException",
	"VisualforceException",
	"XmlException",
}

var platformTypes = []string{
	"Account",
	"AccessLevel",
	"AccessType",
	"AggregateResult",
	"Address",
	"ApexPages",
	"AsyncOptions",
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
	"ChildRelationship",
	"Component",
	"Component.Apex.Column",
	"Component.Apex.OutputPanel",
	"Component.Apex.outputPanel",
	"CronJobDetail",
	"CronTrigger",
	"Database",
	"Database.AssignmentRuleHeader",
	"Database.DMLOptions",
	"Database.DuplicateRuleHeader",
	"Database.EmailHeader",
	"Database.QueryLocator",
	"DescribeFieldResult",
	"DescribeSObjectResult",
	"DescribeTabResult",
	"DescribeTabSetResult",
	"Dom",
	"Dom.Document",
	"Dom.XmlNode",
	"Dom.XmlNodeType",
	"EmailTemplate",
	"EntityDefinition",
	"EntityParticle",
	"FieldPermissions",
	"FieldDefinition",
	"Finalizer",
	"FinalizerContext",
	"Folder",
	"Group",
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
	"Comparator",
	"Datacloud",
	"Datacloud.AdditionalInformationMap",
	"Datacloud.DuplicateResult",
	"Datacloud.FieldDiff",
	"Datacloud.MatchRecord",
	"Datacloud.MatchResult",
	"Http",
	"HttpCalloutMock",
	"HTTPRequest",
	"HttpRequest",
	"HTTPResponse",
	"HttpResponse",
	"WebServiceCallout",
	"WebServiceMock",
	"InstallContext",
	"InstallHandler",
	"Iterable",
	"Iterator",
	"JSONGenerator",
	"JSONParser",
	"JSONToken",
	"Limits",
	"LoggingLevel",
	"Matcher",
	"Messaging",
	"Messaging.Email",
	"Messaging.MassEmailMessage",
	"Messaging.SingleEmailMessage",
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
	"Note",
	"ObjectPermissions",
	"OrgWideEmailAddress",
	"Organization",
	"OpportunityLineItem",
	"PageReference",
	"RecentlyViewed",
	"Pattern",
	"PermissionSetAssignment",
	"PicklistEntry",
	"PricebookEntry",
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
	"Security",
	"Schema",
	"Schema.ChildRelationship",
	"Schema.DescribeFieldResult",
	"Schema.DescribeSObjectResult",
	"Schema.DescribeTabResult",
	"Schema.DescribeTabSetResult",
	"Schema.FieldSet",
	"Schema.FieldSetMember",
	"Schema.PicklistEntry",
	"Schema.RecordTypeInfo",
	"Schema.DisplayType",
	"Schema.SObjectField",
	"Schema.SObjectType",
	"Schema.SoapType",
	"FieldSet",
	"FieldSetMember",
	"SObjectAccessDecision",
	"SObjectDescribeOptions",
	"SObjectField",
	"SObjectType",
	"SoapType",
	"DisplayType",
	"Site",
	"Site.ExternalUserCreateException",
	"Site.UrlRewriter",
	"StaticResource",
	"StatusCode",
	"System",
	"Test",
	"TimeZone",
	"RecordTypeInfo",
	"TriggerOperation",
	"User",
	"UserManagement",
	"UserLicense",
	"URL",
	"UUID",
	"Version",
	"VisualEditor",
	"VisualEditor.DataRow",
	"VisualEditor.DynamicPickListRows",
	"XmlStreamReader",
	"XmlStreamWriter",
}
