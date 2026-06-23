package sema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/schema"
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
	deps      map[string]bool
}

type AnalyzeOptions struct {
	Diagnostics bool
	ExportTypes bool
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
	standardObjectNames := append(storage.KnownStandardObjectNames(), semaAdditionalStandardSObjectNames()...)
	for _, name := range semaStandardChangeEventNames(standardObjectNames) {
		a.addKnown(name, TypePlatform, "")
	}
	return a
}

func Analyze(index typesys.Index) Result {
	return AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, ExportTypes: true})
}

func AnalyzeWithOptions(index typesys.Index, opts AnalyzeOptions) Result {
	return NewAnalyzer().AnalyzeWithOptions(index, opts)
}

func (a *Analyzer) Analyze(index typesys.Index) (result Result) {
	return a.AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, ExportTypes: true})
}

func (a *Analyzer) AnalyzeWithOptions(index typesys.Index, opts AnalyzeOptions) (result Result) {
	a.namespace = index.Project.Namespace
	a.deps = make(map[string]bool)
	index = enrichIndexWithStandardSymbols(index)
	index = enrichIndexWithSchemaDerivedObjects(index)
	result = Result{
		Project: index.Project,
	}
	if opts.Diagnostics {
		result.Diagnostics = append([]diagnostic.Diagnostic{}, index.Diagnostics...)
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
	for _, dep := range index.Dependencies {
		if dep.Status != "loaded" || strings.TrimSpace(dep.Namespace) == "" {
			continue
		}
		a.deps[normalizeName(dep.Namespace)] = true
		a.addKnown(dep.Namespace, TypePlatform, dep.SourceRoot)
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

	if opts.Diagnostics {
		result.Diagnostics = append(result.Diagnostics, a.checkTriggers(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkMemberTypes(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkMethodParameters(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkAnnotations(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkMethodBodies(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkPerformancePatterns(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkVisibility(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkManagedPackageAccess(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkInheritanceContracts(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkSchemaReferences(index)...)
		result.Diagnostics = append(result.Diagnostics, a.checkQuerySemantics(index)...)
	}
	if indexHasSourceBackedDependency(index) {
		result.Diagnostics = downgradeSourceDependencySemanticDiagnostics(result.Diagnostics)
	}

	result.Summary = Summary{
		Types:       countProjectTypes(index.Types),
		Triggers:    len(index.Triggers),
		Objects:     len(index.Objects),
		Diagnostics: len(result.Diagnostics),
	}
	if opts.ExportTypes {
		result.Types = a.exportKnownTypes()
	}
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

func skipProjectDiagnosticType(typ typesys.TypeSymbol) bool {
	return typ.Dependency || typ.Artifact
}

func indexHasSourceBackedDependency(index typesys.Index) bool {
	for _, dep := range index.Dependencies {
		if dep.Status == "loaded" && dep.SourceRoot != "" && !semaDependencyBackedByArtifact(index, dep.Namespace) {
			return true
		}
	}
	return false
}

func downgradeSourceDependencySemanticDiagnostics(diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	out := append([]diagnostic.Diagnostic(nil), diagnostics...)
	for i := range out {
		if out[i].Severity != diagnostic.Error {
			continue
		}
		if strings.HasPrefix(out[i].Code, "GLADESEMA") || strings.HasPrefix(out[i].Code, "dependency_") {
			out[i].Severity = diagnostic.Warning
		}
	}
	return out
}

func enrichIndexWithStandardSymbols(index typesys.Index) typesys.Index {
	seen := make(map[string]bool, len(index.Types))
	for _, typ := range index.Types {
		seen[semaTypeKey(typ.Namespace, typ.Name)] = true
	}
	for _, typ := range typesys.StandardPlatformSymbolView() {
		if seen[semaTypeKey(typ.Namespace, typ.Name)] {
			continue
		}
		seen[semaTypeKey(typ.Namespace, typ.Name)] = true
		index.Types = append(index.Types, typ)
	}
	return index
}

func enrichIndexWithSchemaDerivedObjects(index typesys.Index) typesys.Index {
	if len(index.Objects) == 0 {
		return index
	}
	out := make([]schema.Object, 0, len(index.Objects)*2)
	seen := make(map[string]bool, len(index.Objects)*2)
	for _, object := range index.Objects {
		object = semaEnrichSchemaObject(object)
		out = append(out, object)
		seen[normalizeName(object.Name)] = true
		if shareObject, ok := semaShareObjectForSchemaObject(object); ok && !seen[normalizeName(shareObject.Name)] {
			out = append(out, shareObject)
			seen[normalizeName(shareObject.Name)] = true
		}
	}
	index.Objects = out
	return index
}

func semaEnrichSchemaObject(object schema.Object) schema.Object {
	if strings.EqualFold(object.CustomSettingsType, "Hierarchy") {
		object.Fields = semaMergeSchemaField(object.Fields, schema.Field{
			Name:             "SetupOwnerId",
			Type:             "Lookup",
			ReferenceTo:      []string{"Organization", "Profile", "User"},
			RelationshipName: "SetupOwner",
		})
	}
	if hasQuerySchemaField(object.Fields, "RecordTypeId") || len(object.RecordTypes) > 0 {
		object.Fields = semaMergeSchemaField(object.Fields, schema.Field{
			Name:             "RecordTypeId",
			Type:             "Lookup",
			ReferenceTo:      []string{"RecordType"},
			RelationshipName: "RecordType",
		})
	}
	return object
}

func semaShareObjectForSchemaObject(object schema.Object) (schema.Object, bool) {
	if !strings.HasSuffix(normalizeName(object.Name), "__c") || strings.TrimSpace(object.SharingModel) == "" {
		return schema.Object{}, false
	}
	name := strings.TrimSuffix(object.Name, "__c") + "__Share"
	return schema.Object{
		Name:         name,
		Label:        object.Label + " Share",
		SharingModel: "ReadWrite",
		Fields: []schema.Field{
			{Name: "ParentId", Type: "Lookup", ReferenceTo: []string{object.Name}, RelationshipName: "Parent"},
			{Name: "UserOrGroupId", Type: "Lookup", ReferenceTo: []string{"User", "Group"}, RelationshipName: "UserOrGroup"},
			{Name: "AccessLevel", Type: "Picklist"},
			{Name: "RowCause", Type: "Picklist"},
		},
	}, true
}

func semaAppendSchemaFieldIfMissing(fields []schema.Field, field schema.Field) []schema.Field {
	for _, existing := range fields {
		if strings.EqualFold(existing.Name, field.Name) {
			return fields
		}
	}
	return append(fields, field)
}

func semaMergeSchemaField(fields []schema.Field, field schema.Field) []schema.Field {
	for i, existing := range fields {
		if !strings.EqualFold(existing.Name, field.Name) {
			continue
		}
		if fields[i].Type == "" {
			fields[i].Type = field.Type
		}
		if len(field.ReferenceTo) > 0 {
			fields[i].ReferenceTo = append([]string(nil), field.ReferenceTo...)
		}
		if field.RelationshipName != "" {
			fields[i].RelationshipName = field.RelationshipName
		}
		return fields
	}
	return append(fields, field)
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

func annotationDiagnostic(file string, rng diagnostic.Range, detail string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA026",
		Message:  "invalid annotation usage: " + detail,
		File:     file,
		Range:    &rng,
	}
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
			if semaImplicitObjectInterfaceMethod(method) {
				continue
			}
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

func semaImplicitObjectInterfaceMethod(method typesys.MemberSymbol) bool {
	switch normalizeName(method.Name) {
	case "clone":
		return len(method.Parameters) == 0 && strings.EqualFold(method.Type, "Object")
	case "hashcode":
		return len(method.Parameters) == 0 && strings.EqualFold(method.Type, "Integer")
	case "tostring":
		return len(method.Parameters) == 0 && strings.EqualFold(method.Type, "String")
	case "equals":
		return len(method.Parameters) == 1 && strings.EqualFold(method.Parameters[0].Type, "Object") && strings.EqualFold(method.Type, "Boolean")
	default:
		return false
	}
}

func hasConcreteMethodSignature(model map[string]typeMembers, typeName string, required typesys.MemberSymbol) bool {
	for current := typeName; current != ""; {
		members, ok := model[normalizeName(current)]
		if !ok {
			return false
		}
		for _, method := range members.methods[normalizeName(required.Name)] {
			if (sameSemaSignature(method, required) || semaOverrideCompatibleSignature(method, required, model)) &&
				semaInterfaceReturnCompatible(method, required, model) &&
				!hasModifier(method.Modifiers, "abstract") {
				return true
			}
		}
		current = members.superClass
	}
	return false
}

func semaInterfaceReturnCompatible(method, required typesys.MemberSymbol, model map[string]typeMembers) bool {
	requiredType := strings.TrimSpace(required.Type)
	methodType := strings.TrimSpace(method.Type)
	if requiredType == "" {
		return true
	}
	if methodType == "" {
		methodType = "void"
	}
	if strings.EqualFold(requiredType, "void") || strings.EqualFold(methodType, "void") {
		return strings.EqualFold(requiredType, methodType)
	}
	if strings.EqualFold(required.Name, "start") &&
		len(required.Parameters) == 1 &&
		strings.EqualFold(required.Parameters[0].Type, "Database.BatchableContext") &&
		strings.EqualFold(methodType, "Database.QueryLocator") {
		return true
	}
	return semaAssignableToType(requiredType, methodType, model)
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
			_, scopeEnd = statementOrBlockBoundsAfter(body, closeParen+1)
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
	if semaLocalDeclLooksLikeSOQLClause(typeName, "") {
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
			if segment[i] == '>' && i > 0 && segment[i-1] == '=' {
				continue
			}
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

func semaEnhancedForBodySearchStart(body string, start, fallback int) int {
	open := -1
	for i := start; i < len(body); i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
		case '\'':
			i = skipSemaString(body, i)
		case '(':
			open = i
			i = len(body)
		}
	}
	if open < 0 {
		return fallback
	}
	depth := 0
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
		case '\'':
			i = skipSemaString(body, i)
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
			if depth == 0 {
				return i + 1
			}
		}
	}
	return fallback
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
		if valueType != "" && valueType != "null" && !semaAssignableToType(resolvedTypeName, valueType, model) && !semaSOQLSingletonAssignable(resolvedTypeName, valueType, value.text, model) {
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

func semaSOQLSingletonAssignable(targetType, valueType, expr string, model map[string]typeMembers) bool {
	if strings.EqualFold(targetType, "AggregateResult") && strings.EqualFold(valueType, "List<AggregateResult>") {
		return semaExprLooksLikeSOQLLiteral(expr)
	}
	base, args := semaGenericBaseAndArgs(valueType)
	if !strings.EqualFold(base, "List") || len(args) != 1 || !semaAssignableToType(targetType, args[0], model) {
		return false
	}
	return semaExprLooksLikeSOQLLiteral(expr)
}

func semaExprLooksLikeSOQLLiteral(expr string) bool {
	expr = strings.TrimSpace(expr)
	return strings.HasPrefix(expr, "[") && strings.HasSuffix(expr, "]")
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

func (s semaScopeModel) localInBlock(name string, start, end int) (semaLocal, bool) {
	key := normalizeName(name)
	for _, local := range s.locals {
		if normalizeName(local.name) == key && local.scopeStart == start && local.scopeEnd == end {
			return local, true
		}
	}
	return semaLocal{}, false
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

func semaAssignmentLooksLikeComparison(body string, assignmentEnd int) bool {
	eq := assignmentEnd - 1
	return eq >= 0 && eq+1 < len(body) && body[eq+1] == '='
}

func semaBodyEndsWithThrow(body string) bool {
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "{") && strings.HasSuffix(body, "}") {
		body = strings.TrimSpace(body[1 : len(body)-1])
	}
	match := trailingThrowPattern.FindStringIndex(body)
	return match != nil && !semaOffsetInIgnoredText(body, match[0]+strings.Index(strings.ToLower(body[match[0]:match[1]]), "throw"))
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
		if semaAssignmentLooksLikeComparison(body, match[1]) {
			continue
		}
		exprs = append(exprs, trimSemaArg(body, match[1], semaStatementEnd(body, match[1])))
	}
	for _, match := range returnPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaReturnMatchInIgnoredText(body, match) {
			continue
		}
		if start, end, ok := semaReturnValueRange(match); ok {
			exprs = append(exprs, trimSemaArg(body, start, end))
		}
	}
	return exprs
}

func semaReturnValueRange(match []int) (int, int, bool) {
	for i := 2; i+1 < len(match); i += 2 {
		if match[i] >= 0 {
			return match[i], match[i+1], true
		}
	}
	return 0, 0, false
}

func semaLocalDeclMatchInIgnoredText(body string, match []int) bool {
	if len(match) >= 4 && match[2] >= 0 {
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		name := ""
		if len(match) >= 6 && match[4] >= 0 {
			name = strings.TrimSpace(body[match[4]:match[5]])
		}
		if semaLocalDeclLooksLikeSOQLClause(typeName, name) {
			return true
		}
		return semaOffsetInIgnoredText(body, match[2]) || semaOffsetInParenGroup(body, match[2])
	}
	return len(match) < 1 || match[0] < 0 || semaOffsetInIgnoredText(body, match[0]) || semaOffsetInParenGroup(body, match[0])
}

func semaLocalDeclLooksLikeSOQLClause(typeName, name string) bool {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "select", "find", "from", "where", "and", "or", "limit", "offset", "having":
		return true
	case "order", "group":
		return strings.EqualFold(strings.TrimSpace(name), "by")
	default:
		return false
	}
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

func semaOffsetInParenGroup(body string, pos int) bool {
	depth := 0
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
			i = skipSemaString(body, i)
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '/' {
			for i < len(body) && body[i] != '\n' {
				i++
			}
			continue
		}
		switch body[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth > 0
}

func semaOffsetInSOQLLiteral(body string, pos int) bool {
	for i := 0; i < len(body) && i < pos; i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
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
				case '/':
					if end, ok := skipSemaComment(body, j); ok {
						j = end
					}
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
			if !semaModelHasType(model, candidateRoot) {
				continue
			}
			candidateFieldPath := strings.Join(parts[i:], ".")
			if target, ok := semaStaticClassFieldPathMember(model, candidateRoot, candidateFieldPath); ok {
				return target, true
			}
		}
	}
	if target, ok := semaExplicitPlatformStaticFieldPathMember(root, fieldPath); ok {
		return target, true
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

func semaExplicitPlatformStaticFieldPathMember(root, fieldPath string) (resolvedMember, bool) {
	if !semaExplicitPlatformQualifiedName(root) || strings.Contains(fieldPath, ".") {
		return resolvedMember{}, false
	}
	canonical := semaCanonicalPlatformAlias(root)
	for _, symbol := range typesys.StandardPlatformSymbols() {
		if !strings.EqualFold(symbol.Name, canonical) {
			continue
		}
		for _, member := range symbol.Members {
			if member.Kind == apexast.DeclarationMethod || !hasModifier(member.Modifiers, "static") || !strings.EqualFold(member.Name, fieldPath) {
				continue
			}
			member.Type = semaCanonicalPlatformAlias(member.Type)
			return resolvedMember{owner: symbol.Name, member: member}, true
		}
	}
	return resolvedMember{}, false
}

func semaModelHasType(model map[string]typeMembers, typeName string) bool {
	if _, ok := model[normalizeName(typeName)]; ok {
		return true
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	if strings.EqualFold(canonical, typeName) {
		return false
	}
	_, ok := model[normalizeName(canonical)]
	return ok
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
	typeName := strings.Join(parts[:len(parts)-1], ".")
	valueName := parts[len(parts)-1]
	if typ := semaExplicitPlatformEnumValueType(typeName, valueName); typ != "" {
		return typ
	}
	members, ok := model[normalizeName(typeName)]
	if !ok || members.kind != apexast.DeclarationEnum {
		canonical := semaCanonicalPlatformAlias(typeName)
		if !strings.EqualFold(canonical, typeName) {
			members, ok = model[normalizeName(canonical)]
		}
	}
	if !ok || members.kind != apexast.DeclarationEnum {
		return ""
	}
	if _, ok := members.fields[normalizeName(valueName)]; ok {
		return members.name
	}
	return ""
}

func semaExplicitPlatformEnumValueType(typeName, valueName string) string {
	canonical := semaCanonicalPlatformAlias(typeName)
	if strings.EqualFold(canonical, strings.TrimSpace(typeName)) {
		return ""
	}
	if semaStandardPlatformEnumValue(canonical, valueName) {
		return canonical
	}
	return ""
}

func semaExplicitPlatformEnumType(typeName string) bool {
	canonical := semaCanonicalPlatformAlias(typeName)
	if strings.EqualFold(canonical, strings.TrimSpace(typeName)) {
		return false
	}
	return semaStandardPlatformEnumType(canonical)
}

var (
	semaStandardPlatformEnumOnce   sync.Once
	semaStandardPlatformEnumTypes  map[string]string
	semaStandardPlatformEnumValues map[string]map[string]bool
)

func semaStandardPlatformEnumType(typeName string) bool {
	semaInitStandardPlatformEnums()
	_, ok := semaStandardPlatformEnumTypes[normalizeName(typeName)]
	return ok
}

func semaStandardPlatformEnumValue(typeName, valueName string) bool {
	semaInitStandardPlatformEnums()
	values := semaStandardPlatformEnumValues[normalizeName(typeName)]
	return values != nil && values[normalizeName(valueName)]
}

func semaInitStandardPlatformEnums() {
	semaStandardPlatformEnumOnce.Do(func() {
		semaStandardPlatformEnumTypes = map[string]string{}
		semaStandardPlatformEnumValues = map[string]map[string]bool{}
		for _, symbol := range typesys.StandardPlatformSymbols() {
			if symbol.Kind != apexast.DeclarationEnum {
				continue
			}
			typeKey := normalizeName(symbol.Name)
			semaStandardPlatformEnumTypes[typeKey] = symbol.Name
			values := map[string]bool{}
			for _, member := range symbol.Members {
				if member.Kind == apexast.DeclarationMethod || !hasModifier(member.Modifiers, "static") {
					continue
				}
				values[normalizeName(member.Name)] = true
			}
			semaStandardPlatformEnumValues[typeKey] = values
		}
	})
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
	if len(parts) == 3 && strings.EqualFold(parts[0], "Schema") && semaFieldTokenPart(parts[1]) && semaFieldTokenPart(parts[2]) && isSemaSObjectLike(parts[1], model) && !strings.EqualFold(parts[2], "SObjectType") && !semaLooksLikeStaticConstantName(parts[2]) {
		return true
	}
	if len(parts) == 3 && semaFieldTokenPart(parts[0]) && semaFieldTokenPart(parts[1]) && semaFieldTokenPart(parts[2]) && isSemaSObjectLike(parts[0], model) && !strings.EqualFold(parts[1], "SObjectType") && !strings.EqualFold(parts[1], "fields") && !semaLooksLikeStaticConstantName(parts[1]) && !semaLooksLikeStaticConstantName(parts[2]) {
		return true
	}
	if len(parts) == 3 && semaFieldTokenPart(parts[0]) && strings.EqualFold(parts[1], "fields") && semaFieldTokenPart(parts[2]) && isSemaSObjectLike(parts[0], model) {
		return true
	}
	for i := 0; i+2 < len(parts); i++ {
		if strings.EqualFold(parts[i], "SObjectType") && strings.EqualFold(parts[i+1], "fields") && semaFieldTokenPart(parts[i+2]) {
			return true
		}
		if i+3 < len(parts) && strings.EqualFold(parts[i], "SObjectType") && semaFieldTokenPart(parts[i+1]) && strings.EqualFold(parts[i+2], "fields") && semaFieldTokenPart(parts[i+3]) {
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

func semaLooksLikeSObjectTypeToken(expr string) bool {
	return semaLooksLikeSObjectTypeTokenInModel(expr, nil)
}

func inferSemaMethodCallType(arg string, scope map[string]string, model map[string]typeMembers) string {
	arg = strings.TrimSpace(arg)
	if typ, handled := inferSemaCallChainType(arg, scope, model); handled {
		return typ
	}
	if receiverExpr, method, args, ok := splitLastSemaCall(arg); ok {
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

type semaCallSegment struct {
	method string
	args   []semaArg
}

func inferSemaCallChainType(arg string, scope map[string]string, model map[string]typeMembers) (string, bool) {
	base, calls, ok := splitSemaCallChain(arg)
	if !ok {
		return "", false
	}
	receiverType := semaInitialCallReceiverType(base, scope, model)
	if receiverType == "" {
		return "", true
	}
	for _, call := range calls {
		nextType := semaResolvedCallReturnType(model, receiverType, call.method, call.args, scope)
		if nextType == "" {
			return "", true
		}
		receiverType = nextType
	}
	return receiverType, true
}

func splitSemaCallChain(arg string) (string, []semaCallSegment, bool) {
	expr := strings.TrimSpace(arg)
	var reversed []semaCallSegment
	for {
		if semaConstructorReceiverType(expr) != "" {
			break
		}
		receiver, method, args, ok := splitLastSemaCall(expr)
		if !ok {
			break
		}
		reversed = append(reversed, semaCallSegment{method: method, args: args})
		expr = strings.TrimSpace(receiver)
	}
	if expr == "" || len(reversed) == 0 {
		return "", nil, false
	}
	calls := make([]semaCallSegment, len(reversed))
	for i := range reversed {
		calls[len(reversed)-1-i] = reversed[i]
	}
	return expr, calls, true
}

func semaInitialCallReceiverType(expr string, scope map[string]string, model map[string]typeMembers) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	if typ := inferSemaArgTypeWithModel(expr, scope, model); typ != "" {
		return typ
	}
	if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
		if resolved := resolveNestedTypeName(model, currentType, expr); resolved != "" {
			if members, ok := model[normalizeName(resolved)]; ok {
				return members.name
			}
		}
	}
	if members, _, ok := semaLookupTypeMembers(model, expr); ok {
		return members.name
	}
	return semaTextReceiverType(expr, scope, model)
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
	if semaExplicitPlatformQualifiedName(receiver) {
		return receiver
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

func semaResolvedCallReturnType(model map[string]typeMembers, receiverType, method string, args []semaArg, scope map[string]string) string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if stubbedType := semaCreateStubReturnTypeFromText(model, receiverType, method, args); stubbedType != "" {
		return stubbedType
	}
	if sig, ok := semaEnumMethodSignature(model, receiverType, method); ok {
		return sig.returnType
	}
	candidates := resolveMemberMethods(model, receiverType, method)
	platformBackedCandidates := semaResolvedMembersAllPlatformBacked(model, candidates)
	if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok && !platformBackedCandidates {
		return candidate.member.Type
	}
	if sig, ok := semaSObjectCloneSignature(model, receiverType, method); ok {
		return sig.returnType
	}
	if sig, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return sig.returnType
	}
	if semaDatabaseDynamicQueryCall(receiverType, method) {
		return "Database.QueryResult"
	}
	if returnType := semaDatabaseDMLReturnType(receiverType, method, argTypes); returnType != "" {
		return returnType
	}
	if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok {
		return candidate.member.Type
	}
	if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok {
		return sig.returnType
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
	if sig, ok := semaSObjectCloneSignature(model, receiverType, method); ok {
		return sig, true
	}
	if sig, ok := semaPlatformMethodSignature(receiverType, method); ok {
		return sig, true
	}
	if sig, ok := semaGeneratedPlatformMethodSignature(model, receiverType, method, receiverMode); ok {
		return sig, true
	}
	if sig, ok := semaUserDefinedCloneSignature(model, receiverType, method); ok {
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
			return semaCollectionSignature{returnType: "List<Database.Error>", params: [][]string{{}}}, true
		case "setoptions":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Database.DMLOptions"}}}, true
		case "getoptions":
			return semaCollectionSignature{returnType: "Database.DMLOptions", params: [][]string{{}}}, true
		}
	}
	if semaTypeMatches(model, receiverType, "Exception", make(map[string]bool)) {
		return semaPlatformMethodSignature("Exception", method)
	}
	return semaCollectionSignature{}, false
}

func semaSObjectCloneSignature(model map[string]typeMembers, receiverType, method string) (semaCollectionSignature, bool) {
	if isSemaSObjectLike(receiverType, model) && normalizeName(method) == "clone" {
		return semaCollectionSignature{returnType: receiverType, params: [][]string{{}, {"Boolean"}, {"Boolean", "Boolean"}, {"Boolean", "Boolean", "Boolean"}, {"Boolean", "Boolean", "Boolean", "Boolean"}}}, true
	}
	return semaCollectionSignature{}, false
}

func semaUserDefinedCloneSignature(model map[string]typeMembers, receiverType, method string) (semaCollectionSignature, bool) {
	if normalizeName(method) != "clone" {
		return semaCollectionSignature{}, false
	}
	members, _, ok := semaLookupTypeMembers(model, receiverType)
	if !ok || members.kind != apexast.DeclarationClass {
		return semaCollectionSignature{}, false
	}
	return semaCollectionSignature{returnType: members.name, params: [][]string{{}}}, true
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
		if inner != "" {
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
	typeName = semaCanonicalPlatformAlias(typeName)
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
	if semaHasUnmatchedCloseParen(body[exprStart:dot]) {
		if newStart := semaLastNewExpressionStart(body[:exprStart]); newStart >= 0 {
			exprStart = newStart
		}
	}
	fullReceiverExpr := strings.TrimSpace(body[exprStart:dot])
	fullReceiverType := ""
	boundedFullReceiver := semaReceiverProbeIsBounded(fullReceiverExpr)
	if boundedFullReceiver {
		fullReceiverType = inferSemaArgTypeWithModel(fullReceiverExpr, scope, model)
	}
	if receiverCallStart > exprStart && receiverCallStart < open && !semaReceiverCallLooksLikeConstructor(body[exprStart:receiverCallStart]) {
		if semaReceiverCallStartDropsDottedReceiver(body, receiverCallStart) {
			if fullReceiverType == "" && !boundedFullReceiver {
				return "", "", false
			}
		} else {
			exprStart = receiverCallStart
		}
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
		if !semaReceiverProbeIsBounded(receiverExpr) {
			return "", "", false
		}
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

func semaHasUnmatchedCloseParen(expr string) bool {
	depth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		switch expr[i] {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				return false
			}
			depth--
		}
	}
	return depth > 0
}

func semaLastNewExpressionStart(prefix string) int {
	lower := strings.ToLower(prefix)
	for idx := strings.LastIndex(lower, "new "); idx >= 0; idx = strings.LastIndex(lower[:idx], "new ") {
		if idx == 0 || !isIdentifierByte(lower[idx-1]) {
			return idx
		}
	}
	return -1
}

func semaReceiverCallLooksLikeConstructor(prefix string) bool {
	fields := strings.Fields(strings.TrimSpace(prefix))
	return len(fields) > 0 && strings.EqualFold(fields[len(fields)-1], "new")
}

func semaReceiverCallStartDropsDottedReceiver(body string, receiverCallStart int) bool {
	before := receiverCallStart - 1
	for before >= 0 && isWhitespace(body[before]) {
		before--
	}
	return before >= 0 && body[before] == '.'
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
			if depth == 0 && angleDepth == 0 && semaPreviousNonWhitespaceByte(expr, i-1) == '(' {
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

func semaPreviousNonWhitespaceByte(expr string, start int) byte {
	for i := start; i >= 0; i-- {
		if isWhitespace(expr[i]) {
			continue
		}
		return expr[i]
	}
	return 0
}

func isIdentifierByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
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
	canonical := semaCanonicalPlatformAlias(name)
	if !strings.EqualFold(canonical, name) {
		if _, ok := a.known[normalizeName(canonical)]; ok {
			return true
		}
	}
	if a.hasExternalDependencyName(name) || semaLooksLikeUnconfiguredManagedPackageType(name) {
		return true
	}
	if a.namespace != "" {
		if namespaced, ok := semaProjectNamespacedAPIName(a.namespace, name); ok {
			if _, ok := a.known[normalizeName(namespaced)]; ok {
				return true
			}
		}
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

func (a *Analyzer) hasExternalDependencyName(name string) bool {
	normalized := normalizeName(name)
	for namespace := range a.deps {
		if strings.HasPrefix(normalized, namespace+".") || strings.HasPrefix(normalized, namespace+"__") {
			return true
		}
	}
	return false
}

func semaLooksLikeUnconfiguredManagedPackageType(name string) bool {
	parts := strings.Split(strings.TrimSpace(name), ".")
	if len(parts) < 2 {
		return false
	}
	root := strings.TrimSpace(parts[0])
	if len(root) < 2 || root != strings.ToUpper(root) {
		return false
	}
	for _, r := range root {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func semaProjectNamespacedAPIName(namespace, name string) (string, bool) {
	if namespace == "" || name == "" || !semaIsCustomAPIName(name) || semaHasNamespaceToken(name) {
		return "", false
	}
	return namespace + "__" + name, true
}

func semaProjectLocalAPIName(namespace, name string) (string, bool) {
	if namespace == "" || name == "" || !semaIsCustomAPIName(name) || !semaHasNamespaceToken(name) {
		return "", false
	}
	prefix := namespace + "__"
	if len(name) <= len(prefix) || !strings.EqualFold(name[:len(prefix)], prefix) {
		return "", false
	}
	return name[len(prefix):], true
}

func semaOwnerNamespacedAPIName(ownerName, name string) (string, bool) {
	namespace := semaNamespaceFromAPIName(ownerName)
	if namespace == "" {
		return "", false
	}
	return semaProjectNamespacedAPIName(namespace, name)
}

func semaNamespaceFromAPIName(name string) string {
	if !semaIsCustomAPIName(name) {
		return ""
	}
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return ""
	}
	return name[:first]
}

func semaIsCustomAPIName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, suffix := range []string{"__c", "__r", "__e", "__mdt", "__b", "__s"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func semaHasNamespaceToken(name string) bool {
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	return first > 0 && first < last
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
	typeIdentifierPattern          = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*`)
	typeReferencePattern           = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*$`)
	lineLocalDeclPattern           = regexp.MustCompile(`(?m)^\s*(?:final\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:,|=|;)`)
	wrappedLocalDeclPattern        = regexp.MustCompile(`(?m)^\s*(?:final\s+)?([A-Za-z_][^\n;=(){}]+)[ \t]*\r?\n\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	noSpaceGenericLocalDeclPattern = regexp.MustCompile(`(?m)(?:^|[;\n])\s*(?:final\s+)?([A-Za-z_][A-Za-z0-9_.]*\s*<[^;=(){}]+>)([A-Za-z_][A-Za-z0-9_]*)\s*(?:,|=|;)`)
	localDeclPattern               = regexp.MustCompile(`(?m)(?:^|[;\n])\s*(?:final\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:,|=|;)`)
	enhancedForLocalPattern        = regexp.MustCompile(`(?im)\bfor\s*\(\s*(?:final\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*:`)
	forHeaderPattern               = regexp.MustCompile(`(?i)\bfor\s*\(`)
	catchLocalPattern              = regexp.MustCompile(`(?im)\bcatch\s*\(\s*([A-Za-z_][A-Za-z0-9_.]*(?:\s*\|\s*[A-Za-z_][A-Za-z0-9_.]*)*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
	assignmentPattern              = regexp.MustCompile(`(?m)(?:^|[;{}\n])\s*([A-Za-z_][A-Za-z0-9_]*)\s=`)
	callPattern                    = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s*\(`)
	constructorPattern             = regexp.MustCompile(`(?i)\bnew\s+([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?)\s*\(`)
	newExprPattern                 = regexp.MustCompile(`(?is)^new\s+([A-Za-z_][A-Za-z0-9_.]*(?:\s*<[^;=(){}]+>)?(?:\s*\[\s*\])*)\s*(?:\([^)]*\)|\{.*\})\s*$`)
	decimalLiteralPattern          = regexp.MustCompile(`^-?(?:[0-9]+\.[0-9]*|[0-9]*\.[0-9]+)$`)
	intLiteralPattern              = regexp.MustCompile(`^-?[0-9]+$`)
	returnPattern                  = regexp.MustCompile(`(?is)(?:^|[;{}\n])\s*return(?:\s+([^;\s][^;]*)|(\([^;]+))?\s*;`)
	trailingThrowPattern           = regexp.MustCompile(`(?is)(?:^|[;{}\n])\s*throw\s+[^;]+;\s*$`)
	simpleIdentifierPattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

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

func semaStatementEnd(body string, start int) int {
	depth := 0
	for i := start; i < len(body); i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
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
	if typ := inferSemaBinaryType(arg, scope); typ != "" {
		return typ
	}
	if typ, ok := scope[normalizeName(arg)]; ok {
		return typ
	}
	if receiver, name, ok := strings.Cut(arg, "."); ok && strings.EqualFold(receiver, "Page") && semaPageReferenceTokenName(name) {
		return "PageReference"
	}
	return ""
}

func semaPageReferenceTokenName(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && !strings.ContainsAny(name, "().[]{}")
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
	for _, op := range []string{"<<", ">>", ">>>", "|", "&", "^"} {
		left, right, ok := splitSemaBinary(arg, op)
		if !ok {
			continue
		}
		return semaIntegralResultType(inferSemaArgType(left, scope), inferSemaArgType(right, scope))
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

func isSemaNumericType(typeName string) bool {
	return strings.EqualFold(typeName, "Integer") || strings.EqualFold(typeName, "Long") || strings.EqualFold(typeName, "Decimal") || strings.EqualFold(typeName, "Double")
}

func isSemaIntegralType(typeName string) bool {
	return strings.EqualFold(typeName, "Integer") || strings.EqualFold(typeName, "Long")
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

func semaKnownPlatformTypeReceiver(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.ContainsAny(typeName, "()[]{};=,+-*/%&|?:") {
		return false
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	for _, known := range platformTypes {
		if typeName == known {
			return true
		}
		if lastDot := strings.LastIndex(known, "."); lastDot >= 0 && typeName == known[lastDot+1:] {
			return true
		}
		if strings.Contains(typeName, ".") && canonical == known {
			return true
		}
	}
	return false
}

func semaPlatformReceiverSpellingMatches(receiver string, members typeMembers) bool {
	if !members.dependency || !semaKnownPlatformTypeReceiver(members.name) {
		return true
	}
	receiver = strings.TrimSpace(receiver)
	if receiver == members.name {
		return true
	}
	canonical := semaCanonicalPlatformAlias(members.name)
	if receiver == canonical {
		return true
	}
	if lastDot := strings.LastIndex(canonical, "."); lastDot >= 0 && receiver == canonical[lastDot+1:] {
		return true
	}
	if strings.Contains(receiver, ".") && semaCanonicalPlatformAlias(receiver) == canonical {
		return true
	}
	return false
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
	name = strings.TrimSpace(name)
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= utf8.RuneSelf {
			return strings.ToLower(name)
		}
		if c >= 'A' && c <= 'Z' {
			buf := []byte(name)
			buf[i] = c + ('a' - 'A')
			for j := i + 1; j < len(buf); j++ {
				c = buf[j]
				if c >= utf8.RuneSelf {
					return strings.ToLower(name)
				}
				if c >= 'A' && c <= 'Z' {
					buf[j] = c + ('a' - 'A')
				}
			}
			return string(buf)
		}
	}
	return name
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
