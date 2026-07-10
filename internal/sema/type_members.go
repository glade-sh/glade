package sema

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

type typeMembers struct {
	name                        string
	shortKey                    string
	namespace                   string
	dependency                  bool
	sobject                     bool
	externalPackageSObject      bool
	partialSObject              bool
	kind                        apexast.DeclarationKind
	superClass                  string
	interfaces                  []string
	methods                     map[string][]typesys.MemberSymbol
	constructors                []typesys.MemberSymbol
	fields                      map[string]typesys.MemberSymbol
	syntheticStandardSObject    bool
	standardSObjectFieldsLoaded bool
}

const semaCurrentTypeScopeKey = "__glade_current_type"

const semaInferenceDepthScopeKey = "__glade_inference_depth"

const semaSyntheticStandardSObjectFieldModifier = "__glade_standard_sobject_field"

func (a *Analyzer) checkMethodBodies(index typesys.Index) []diagnostic.Diagnostic {
	return a.checkMethodBodiesWithRecorder(index, nil)
}

func (a *Analyzer) checkMethodBodiesWithRecorder(index typesys.Index, recorder *perfRecorder) []diagnostic.Diagnostic {
	modelStarted := recorder.beginPhase()
	model := buildTypeMembers(index)
	if recorder != nil {
		recorder.endPhase(&recorder.counters.TypeMemberModel, modelStarted)
	}
	defer unregisterSemaShortCandidateIndex(model)
	constructability := buildConstructability(index)
	duplicateTypes := semaDuplicateTypeKeys(index)
	sources := make(map[string]string)
	var diagnostics []diagnostic.Diagnostic
	bodyStarted := recorder.beginPhase()
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		bodyModel := model
		if duplicateTypes[semaTypeSymbolKey(typ)] > 1 {
			bodyModel = semaModelWithCurrentType(model, typ)
		}
		for _, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationMethod, apexast.DeclarationConstructor, apexast.DeclarationInitializer:
				source, ok := readSemaSourceForType(typ, sources)
				if !ok {
					continue
				}
				body, bodyOffset, ok := extractBodyForSema(source, member.Range)
				if !ok {
					continue
				}
				diagnostics = append(diagnostics, a.checkBodyText(typ, member, body, bodyOffset, source, bodyModel, constructability)...)
			case apexast.DeclarationProperty:
				for _, accessor := range member.Accessors {
					if !accessor.HasBody {
						continue
					}
					source, ok := readSemaSourceForType(typ, sources)
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
					diagnostics = append(diagnostics, a.checkBodyText(typ, accessorMember, body, bodyOffset, source, bodyModel, constructability)...)
				}
			}
		}
	}
	if recorder != nil {
		recorder.endPhase(&recorder.counters.MethodBodies, bodyStarted)
	}
	return diagnostics
}

func semaDuplicateTypeKeys(index typesys.Index) map[string]int {
	counts := make(map[string]int)
	for _, typ := range index.Types {
		counts[semaTypeSymbolKey(typ)]++
	}
	return counts
}

func semaTypeSymbolKey(typ typesys.TypeSymbol) string {
	if typ.Namespace != "" {
		return normalizeName(typ.Namespace + "." + typ.Name)
	}
	return normalizeName(typ.Name)
}

func semaModelWithCurrentType(model map[string]typeMembers, typ typesys.TypeSymbol) map[string]typeMembers {
	out := make(map[string]typeMembers, len(model)+1)
	for key, members := range model {
		out[key] = members
	}
	members := semaTypeMembersFromSymbol(typ)
	out[normalizeName(typ.Name)] = members
	if typ.Namespace != "" {
		out[normalizeName(typ.Namespace+"."+typ.Name)] = members
	}
	members.superClass = resolveNestedTypeName(out, members.name, members.superClass)
	for i, iface := range members.interfaces {
		members.interfaces[i] = resolveNestedTypeName(out, members.name, iface)
	}
	for fieldKey, field := range members.fields {
		field.Type = resolveNestedTypeReference(out, members.name, field.Type)
		members.fields[fieldKey] = field
	}
	for methodKey, overloads := range members.methods {
		for i := range overloads {
			overloads[i].Type = resolveNestedTypeReference(out, members.name, overloads[i].Type)
			for j := range overloads[i].Parameters {
				overloads[i].Parameters[j].Type = resolveNestedTypeReference(out, members.name, overloads[i].Parameters[j].Type)
			}
		}
		members.methods[methodKey] = overloads
	}
	for i := range members.constructors {
		for j := range members.constructors[i].Parameters {
			members.constructors[i].Parameters[j].Type = resolveNestedTypeReference(out, members.name, members.constructors[i].Parameters[j].Type)
		}
	}
	out[normalizeName(typ.Name)] = members
	if typ.Namespace != "" {
		out[normalizeName(typ.Namespace+"."+typ.Name)] = members
	}
	return out
}

func semaTypeMembersFromSymbol(typ typesys.TypeSymbol) typeMembers {
	members := typeMembers{
		name:       semaTypeMembersName(typ),
		shortKey:   semaShortTypeKey(typ.Name),
		namespace:  typ.Namespace,
		dependency: typ.Dependency,
		kind:       typ.Kind,
		superClass: typ.SuperClass,
		interfaces: append([]string(nil), typ.Interfaces...),
		methods:    make(map[string][]typesys.MemberSymbol),
		fields:     make(map[string]typesys.MemberSymbol),
	}
	for _, member := range typ.Members {
		member = semaCloneMemberSymbol(member)
		switch member.Kind {
		case apexast.DeclarationMethod:
			members.methods[normalizeName(member.Name)] = append(members.methods[normalizeName(member.Name)], member)
		case apexast.DeclarationConstructor:
			members.constructors = append(members.constructors, member)
		case apexast.DeclarationField, apexast.DeclarationProperty:
			members.fields[normalizeName(member.Name)] = member
		}
	}
	if typ.Kind == apexast.DeclarationEnum {
		for _, value := range semaEnumValues(typ) {
			members.fields[normalizeName(value)] = typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      value,
				Type:      typ.Name,
				Modifiers: []string{"public", "static"},
			}
		}
	}
	return members
}

func readSemaSource(path string, cache map[string]string) (string, bool) {
	return readSemaSourceWithRemaps(path, "", nil, cache)
}

func readSemaSourceForType(typ typesys.TypeSymbol, cache map[string]string) (string, bool) {
	return readSemaSourceWithRemaps(typ.File, typ.Namespace, typ.SourceNamespaceRemaps, cache)
}

func readSemaSourceWithRemaps(path, namespace string, remaps []namespaceremap.Rule, cache map[string]string) (string, bool) {
	if path == "" {
		return "", false
	}
	key := semaSourceCacheKey(path, namespace, remaps)
	if source, ok := cache[key]; ok {
		return source, true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	source := project.NormalizeApexNamespaceTokens(string(data), namespace)
	if len(remaps) > 0 {
		source = namespaceremap.ApplySource(remaps, source)
	}
	cache[key] = source
	return source, true
}

func semaSourceCacheKey(path, namespace string, remaps []namespaceremap.Rule) string {
	fingerprint := namespaceremap.Fingerprint(remaps)
	return path + "\x00" + namespace + "\x00" + fingerprint
}

func buildTypeMembers(index typesys.Index) map[string]typeMembers {
	out := make(map[string]typeMembers)
	shortAliases := make(map[string][]string)
	projectNamespace := index.Project.Namespace
	for _, typ := range index.Types {
		members := typeMembers{
			name:       semaTypeMembersName(typ),
			shortKey:   semaShortTypeKey(typ.Name),
			namespace:  typ.Namespace,
			dependency: typ.Dependency,
			kind:       typ.Kind,
			superClass: typ.SuperClass,
			interfaces: append([]string(nil), typ.Interfaces...),
			methods:    make(map[string][]typesys.MemberSymbol),
			fields:     make(map[string]typesys.MemberSymbol),
		}
		for _, member := range typ.Members {
			member = semaCloneMemberSymbol(member)
			switch member.Kind {
			case apexast.DeclarationMethod:
				members.methods[normalizeName(member.Name)] = append(members.methods[normalizeName(member.Name)], member)
			case apexast.DeclarationConstructor:
				members.constructors = append(members.constructors, member)
			case apexast.DeclarationField, apexast.DeclarationProperty:
				members.fields[normalizeName(member.Name)] = member
			case apexast.DeclarationEnum:
				nestedName := typ.Name + "." + member.Name
				enumMembers := typeMembers{
					name:       semaQualifiedDependencyTypeName(typ.Namespace, nestedName, typ.Dependency, typ.Artifact, typ.SourceRoot),
					shortKey:   semaShortTypeKey(nestedName),
					namespace:  typ.Namespace,
					dependency: typ.Dependency,
					kind:       apexast.DeclarationEnum,
					methods:    make(map[string][]typesys.MemberSymbol),
					fields:     make(map[string]typesys.MemberSymbol),
				}
				enumType := typesys.TypeSymbol{
					Kind:  apexast.DeclarationEnum,
					Name:  nestedName,
					File:  typ.File,
					Range: member.Range,
				}
				for _, value := range semaEnumValues(enumType) {
					enumMembers.fields[normalizeName(value)] = typesys.MemberSymbol{
						Kind:      apexast.DeclarationField,
						Name:      value,
						Type:      nestedName,
						Modifiers: []string{"public", "static"},
					}
				}
				if !semaRequiresQualifiedDependencyName(typ) {
					key := normalizeName(nestedName)
					if semaShouldStoreTypeMembers(out[key], typ) {
						out[key] = enumMembers
					}
					shortAliases[normalizeName(member.Name)] = append(shortAliases[normalizeName(member.Name)], nestedName)
				}
				if typ.Namespace != "" {
					out[normalizeName(typ.Namespace+"."+nestedName)] = enumMembers
				}
			}
		}
		if typ.Kind == apexast.DeclarationEnum {
			for _, value := range semaEnumValues(typ) {
				members.fields[normalizeName(value)] = typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      value,
					Type:      typ.Name,
					Modifiers: []string{"public", "static"},
				}
			}
		}
		if !semaRequiresQualifiedDependencyName(typ) {
			key := normalizeName(typ.Name)
			if semaShouldStoreTypeMembers(out[key], typ) {
				out[key] = members
			}
		}
		if typ.Namespace != "" {
			key := normalizeName(typ.Namespace + "." + typ.Name)
			if semaShouldStoreTypeMembers(out[key], typ) {
				out[key] = members
			}
		}
		if short := shortNestedTypeName(typ.Name); !semaRequiresQualifiedDependencyName(typ) && short != typ.Name {
			shortAliases[normalizeName(short)] = append(shortAliases[normalizeName(short)], typ.Name)
		}
	}
	for _, object := range index.Objects {
		objectKey := normalizeName(object.Name)
		objectMembers, objectOK := out[objectKey]
		if objectOK && !objectMembers.sobject && !semaShouldMergeStandardSObjectMembers(object.Name, objectMembers) {
			continue
		}
		if !objectOK {
			objectMembers = typeMembers{
				name:                   object.Name,
				shortKey:               semaShortTypeKey(object.Name),
				namespace:              semaNamespaceFromAPIName(object.Name),
				sobject:                true,
				externalPackageSObject: semaIsExternalManagedPackageAPIName(projectNamespace, object.Name),
				partialSObject:         object.Partial,
				kind:                   apexast.DeclarationClass,
				fields:                 make(map[string]typesys.MemberSymbol),
				methods:                make(map[string][]typesys.MemberSymbol),
			}
		}
		objectMembers.sobject = true
		objectMembers.partialSObject = objectMembers.partialSObject || object.Partial
		if objectMembers.namespace == "" {
			objectMembers.namespace = semaNamespaceFromAPIName(object.Name)
		}
		objectMembers.externalPackageSObject = semaIsExternalManagedPackageAPIName(projectNamespace, object.Name)
		if objectMembers.fields == nil {
			objectMembers.fields = make(map[string]typesys.MemberSymbol)
		}
		semaAddCommonSObjectFields(objectMembers.fields)
		if semaObjectSupportsRecordTypeRelationship(object) {
			semaAddSObjectRecordTypeRelationship(objectMembers.fields)
		}
		for _, field := range object.Fields {
			if field.Name != "" {
				fieldType := semaApexTypeForSchemaFieldInObjects(index.Objects, field)
				if fieldType == "" {
					if _, exists := objectMembers.fields[normalizeName(field.Name)]; exists {
						continue
					}
				}
				semaAddSchemaFieldMember(objectMembers.fields, projectNamespace, typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      field.Name,
					Type:      fieldType,
					Modifiers: []string{"public"},
				})
				if strings.EqualFold(field.Type, "Location") {
					for _, componentName := range semaLocationComponentFieldNames(field.Name) {
						semaAddSchemaFieldMember(objectMembers.fields, projectNamespace, typesys.MemberSymbol{
							Kind:      apexast.DeclarationField,
							Name:      componentName,
							Type:      "Decimal",
							Modifiers: []string{"public"},
						})
					}
				}
			}
			if field.RelationshipName != "" && len(field.ReferenceTo) != 0 {
				relationshipType := "SObject"
				if len(field.ReferenceTo) == 1 {
					relationshipType = field.ReferenceTo[0]
				}
				semaAddSchemaFieldMember(objectMembers.fields, projectNamespace, typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      field.RelationshipName,
					Type:      relationshipType,
					Modifiers: []string{"public"},
				})
			}
			if relationshipFieldName := semaParentRelationshipFieldName(field.Name); relationshipFieldName != "" && len(field.ReferenceTo) != 0 {
				relationshipType := "SObject"
				if len(field.ReferenceTo) == 1 {
					relationshipType = field.ReferenceTo[0]
				}
				semaAddSchemaFieldMember(objectMembers.fields, projectNamespace, typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      relationshipFieldName,
					Type:      relationshipType,
					Modifiers: []string{"public"},
				})
			}
			childRelationshipNames := []string{}
			if field.ChildRelationshipName != "" {
				childRelationshipNames = append(childRelationshipNames, field.ChildRelationshipName)
			}
			if len(childRelationshipNames) == 0 {
				continue
			}
			for _, parent := range field.ReferenceTo {
				parentKey := normalizeName(parent)
				if parentKey == "" {
					continue
				}
				parentMembers := objectMembers
				if parentKey != objectKey {
					var ok bool
					parentMembers, ok = out[parentKey]
					if !ok {
						parentMembers = typeMembers{
							name:     parent,
							shortKey: semaShortTypeKey(parent),
							sobject:  true,
							kind:     apexast.DeclarationClass,
							fields:   make(map[string]typesys.MemberSymbol),
							methods:  make(map[string][]typesys.MemberSymbol),
						}
					}
				} else if parentMembers.fields == nil {
					parentMembers = typeMembers{
						name:     object.Name,
						shortKey: semaShortTypeKey(object.Name),
						sobject:  true,
						kind:     apexast.DeclarationClass,
						fields:   make(map[string]typesys.MemberSymbol),
						methods:  make(map[string][]typesys.MemberSymbol),
					}
				}
				parentMembers.sobject = true
				if parentMembers.fields == nil {
					parentMembers.fields = make(map[string]typesys.MemberSymbol)
				}
				for _, childRelationshipName := range childRelationshipNames {
					semaAddSchemaFieldMemberIfAbsent(parentMembers.fields, projectNamespace, typesys.MemberSymbol{
						Kind:      apexast.DeclarationField,
						Name:      childRelationshipName,
						Type:      "List<" + object.Name + ">",
						Modifiers: []string{"public"},
					})
					if strings.HasSuffix(childRelationshipName, "__r") {
						continue
					}
					childRelationshipAlias := childRelationshipName + "__r"
					semaAddSchemaFieldMemberIfAbsent(parentMembers.fields, projectNamespace, typesys.MemberSymbol{
						Kind:      apexast.DeclarationField,
						Name:      childRelationshipAlias,
						Type:      "List<" + object.Name + ">",
						Modifiers: []string{"public"},
					})
				}
				if parentKey == objectKey {
					objectMembers = parentMembers
				} else {
					semaStoreSObjectTypeMembers(out, projectNamespace, parent, parentMembers)
				}
			}
		}
		semaStoreSObjectTypeMembers(out, projectNamespace, object.Name, objectMembers)
	}
	addStandardSObjectMembers(out)
	semaApplyPlatformInterfaceOverlays(out)
	semaApplyPlatformFieldOverlays(out)
	for short, names := range shortAliases {
		if len(names) == 1 {
			if _, exists := out[short]; exists {
				continue
			}
			out[short] = out[normalizeName(names[0])]
		}
	}
	for key, members := range out {
		if members.shortKey == "" {
			members.shortKey = semaShortTypeKey(members.name)
		}
		if members.syntheticStandardSObject {
			out[key] = members
			continue
		}
		members.superClass = resolveNestedTypeName(out, members.name, members.superClass)
		for i, iface := range members.interfaces {
			members.interfaces[i] = resolveNestedTypeName(out, members.name, iface)
		}
		for fieldKey, field := range members.fields {
			field.Type = resolveNestedTypeReference(out, members.name, field.Type)
			members.fields[fieldKey] = field
		}
		for methodKey, overloads := range members.methods {
			for i := range overloads {
				overloads[i].Type = resolveNestedTypeReference(out, members.name, overloads[i].Type)
				for j := range overloads[i].Parameters {
					overloads[i].Parameters[j].Type = resolveNestedTypeReference(out, members.name, overloads[i].Parameters[j].Type)
				}
			}
			members.methods[methodKey] = overloads
		}
		for i := range members.constructors {
			for j := range members.constructors[i].Parameters {
				members.constructors[i].Parameters[j].Type = resolveNestedTypeReference(out, members.name, members.constructors[i].Parameters[j].Type)
			}
		}
		out[key] = members
	}
	registerSemaShortCandidateIndex(out)
	return out
}

func semaRequiresQualifiedDependencyName(typ typesys.TypeSymbol) bool {
	return semaRequiresQualifiedDependencyNameValues(typ.Namespace, typ.Dependency, typ.Artifact, typ.SourceRoot)
}

func semaCloneMemberSymbol(member typesys.MemberSymbol) typesys.MemberSymbol {
	member.Modifiers = append([]string(nil), member.Modifiers...)
	member.Parameters = semaCloneParameters(member.Parameters)
	member.Accessors = semaCloneAccessors(member.Accessors)
	return member
}

func semaCloneParameters(parameters []apexast.Parameter) []apexast.Parameter {
	if len(parameters) == 0 {
		return nil
	}
	out := make([]apexast.Parameter, len(parameters))
	for i, parameter := range parameters {
		out[i] = parameter
		out[i].Modifiers = append([]string(nil), parameter.Modifiers...)
	}
	return out
}

func semaCloneAccessors(accessors []apexast.Accessor) []apexast.Accessor {
	if len(accessors) == 0 {
		return nil
	}
	out := make([]apexast.Accessor, len(accessors))
	for i, accessor := range accessors {
		out[i] = accessor
		out[i].Modifiers = append([]string(nil), accessor.Modifiers...)
	}
	return out
}

func semaTypeMembersName(typ typesys.TypeSymbol) string {
	return semaQualifiedDependencyTypeName(typ.Namespace, typ.Name, typ.Dependency, typ.Artifact, typ.SourceRoot)
}

func semaQualifiedDependencyTypeName(namespace, name string, dependency, artifact bool, sourceRoot string) string {
	if semaRequiresQualifiedDependencyNameValues(namespace, dependency, artifact, sourceRoot) {
		return namespace + "." + name
	}
	return name
}

func semaRequiresQualifiedDependencyNameValues(namespace string, dependency, artifact bool, sourceRoot string) bool {
	return dependency && namespace != "" && (artifact || sourceRoot != "")
}

func semaShouldStoreTypeMembers(existing typeMembers, typ typesys.TypeSymbol) bool {
	if existing.name == "" {
		return true
	}
	return existing.dependency && !typ.Dependency
}

var semaShortCandidateIndexes sync.Map

func registerSemaShortCandidateIndex(model map[string]typeMembers) {
	index := make(map[string][]string)
	for key, members := range model {
		short := members.shortKey
		if short == "" {
			short = semaShortTypeKeyFromNormalizedKey(key)
		}
		if short == "" {
			continue
		}
		index[short] = append(index[short], key)
	}
	semaShortCandidateIndexes.Store(semaModelCacheKey(model), index)
}

func unregisterSemaShortCandidateIndex(model map[string]typeMembers) {
	semaShortCandidateIndexes.Delete(semaModelCacheKey(model))
}

func semaAddSchemaFieldMember(fields map[string]typesys.MemberSymbol, namespace string, member typesys.MemberSymbol) {
	if fields == nil || strings.TrimSpace(member.Name) == "" {
		return
	}
	fields[normalizeName(member.Name)] = member
	if localName, ok := semaProjectLocalAPIName(namespace, member.Name); ok {
		alias := member
		alias.Name = localName
		fields[normalizeName(localName)] = alias
	}
}

func semaAddSchemaFieldMemberIfAbsent(fields map[string]typesys.MemberSymbol, namespace string, member typesys.MemberSymbol) {
	if fields == nil || strings.TrimSpace(member.Name) == "" {
		return
	}
	if _, exists := fields[normalizeName(member.Name)]; exists {
		return
	}
	fields[normalizeName(member.Name)] = member
	if localName, ok := semaProjectLocalAPIName(namespace, member.Name); ok {
		localKey := normalizeName(localName)
		if _, exists := fields[localKey]; !exists {
			alias := member
			alias.Name = localName
			fields[localKey] = alias
		}
	}
}

func semaLocationComponentFieldNames(fieldName string) []string {
	if !strings.HasSuffix(fieldName, "__c") {
		return nil
	}
	base := strings.TrimSuffix(fieldName, "__c")
	return []string{base + "__Latitude__s", base + "__Longitude__s"}
}

func semaStoreSObjectTypeMembers(out map[string]typeMembers, namespace, objectName string, members typeMembers) {
	key := normalizeName(objectName)
	if key == "" {
		return
	}
	out[key] = members
	if localName, ok := semaProjectLocalAPIName(namespace, objectName); ok {
		localKey := normalizeName(localName)
		if existing, exists := out[localKey]; !exists || existing.sobject {
			out[localKey] = members
		}
	}
}

func semaShortCandidateKeys(model map[string]typeMembers, short string) []string {
	if cached, ok := semaShortCandidateIndexes.Load(semaModelCacheKey(model)); ok {
		if index, ok := cached.(map[string][]string); ok {
			return index[short]
		}
	}
	var keys []string
	for candidateKey, members := range model {
		if members.shortKey == short {
			keys = append(keys, candidateKey)
		}
	}
	return keys
}

func semaModelCacheKey(model map[string]typeMembers) uintptr {
	return reflect.ValueOf(model).Pointer()
}

func addStandardSObjectMembers(out map[string]typeMembers) {
	names, _ := semaStandardSObjectMembers()
	for _, objectName := range names {
		key := normalizeName(objectName)
		if key == "" {
			continue
		}
		members, ok := out[key]
		if ok && !members.sobject && !semaShouldMergeStandardSObjectMembers(objectName, members) {
			continue
		}
		if !ok {
			out[key] = semaStandardSObjectPlaceholder(objectName)
			continue
		}
		members.sobject = true
		if members.fields == nil {
			members.fields = make(map[string]typesys.MemberSymbol)
		}
		addStandardSObjectFields(&members, objectName, false)
		out[key] = members
	}
}

func semaShouldMergeStandardSObjectMembers(objectName string, members typeMembers) bool {
	if !members.sobject && !members.dependency {
		return false
	}
	if storage.IsKnownStandardObject(objectName) {
		return true
	}
	if !members.dependency {
		return false
	}
	switch normalizeName(objectName) {
	case "profile", "user", "userlicense":
		return true
	default:
		return false
	}
}

var semaStandardSObjectMembersCache struct {
	once      sync.Once
	names     []string
	members   map[string]typeMembers
	nameByKey map[string]string
}

func semaStandardSObjectMembers() ([]string, map[string]typeMembers) {
	semaStandardSObjectMembersCache.once.Do(func() {
		sourceNames := append(storage.KnownStandardObjectNames(), semaAdditionalStandardSObjectNames()...)
		sourceNames = append(sourceNames, semaStandardChangeEventNames(sourceNames)...)
		names := make([]string, 0, len(sourceNames))
		nameByKey := make(map[string]string, len(sourceNames))
		seen := make(map[string]bool, len(sourceNames))
		for _, objectName := range sourceNames {
			key := normalizeName(objectName)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			names = append(names, objectName)
			nameByKey[key] = objectName
		}
		semaStandardSObjectMembersCache.names = names
		semaStandardSObjectMembersCache.members = map[string]typeMembers{}
		semaStandardSObjectMembersCache.nameByKey = nameByKey
	})
	return semaStandardSObjectMembersCache.names, semaStandardSObjectMembersCache.members
}

func semaStandardSObjectPlaceholder(objectName string) typeMembers {
	return typeMembers{
		name:                     objectName,
		shortKey:                 semaShortTypeKey(objectName),
		sobject:                  true,
		kind:                     apexast.DeclarationClass,
		syntheticStandardSObject: true,
	}
}

func semaBuildStandardSObjectMembers(objectName string) typeMembers {
	members := semaStandardSObjectPlaceholder(objectName)
	members.fields = make(map[string]typesys.MemberSymbol)
	members.methods = make(map[string][]typesys.MemberSymbol)
	addStandardSObjectFields(&members, objectName, true)
	return members
}

func semaEnsureStandardSObjectTypeMembers(model map[string]typeMembers, key string, members typeMembers) typeMembers {
	if !members.syntheticStandardSObject || members.standardSObjectFieldsLoaded {
		return members
	}
	hydrated := semaBuildStandardSObjectMembers(members.name)
	for fieldKey, field := range members.fields {
		hydrated.fields[fieldKey] = field
	}
	for methodKey, methods := range members.methods {
		hydrated.methods[methodKey] = append(hydrated.methods[methodKey], methods...)
	}
	if key == "" {
		key = normalizeName(members.name)
	}
	if key != "" {
		model[key] = hydrated
	}
	return hydrated
}

func addStandardSObjectFields(members *typeMembers, objectName string, synthetic bool) {
	semaAddCommonSObjectFields(members.fields)
	members.fields[normalizeName("SObjectType")] = typesys.MemberSymbol{
		Kind:      apexast.DeclarationField,
		Name:      "SObjectType",
		Type:      "Schema.SObjectType",
		Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
	}
	definitionName := objectName
	isChangeEvent := false
	if baseName, ok := semaChangeEventBaseObjectName(objectName); ok {
		definitionName = baseName
		isChangeEvent = true
	}
	if definition, ok := storage.StandardObjectDefinition(definitionName); ok {
		for _, field := range definition.Fields {
			if field.APIName == "" {
				continue
			}
			members.fields[normalizeName(field.APIName)] = typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.APIName,
				Type:      semaApexTypeForStorageField(field),
				Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
			}
			if field.RelationshipName != "" && len(field.ReferenceTo) != 0 {
				relationshipType := "SObject"
				if len(field.ReferenceTo) == 1 {
					relationshipType = field.ReferenceTo[0]
				}
				members.fields[normalizeName(field.RelationshipName)] = typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      field.RelationshipName,
					Type:      relationshipType,
					Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
				}
			}
		}
	}
	if isChangeEvent {
		members.fields[normalizeName("ChangeEventHeader")] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      "ChangeEventHeader",
			Type:      "EventBus.ChangeEventHeader",
			Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
		}
	}
	for _, field := range semaFallbackStandardSObjectFields(objectName, synthetic) {
		members.fields[normalizeName(field.Name)] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      field.Name,
			Type:      semaApexTypeForSchemaField(field),
			Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
		}
		if field.RelationshipName != "" && len(field.ReferenceTo) != 0 {
			relationshipType := "SObject"
			if len(field.ReferenceTo) == 1 {
				relationshipType = field.ReferenceTo[0]
			}
			members.fields[normalizeName(field.RelationshipName)] = typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.RelationshipName,
				Type:      relationshipType,
				Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
			}
		}
	}
	addFallbackStandardSObjectRelationshipMembers(members, objectName)
	members.standardSObjectFieldsLoaded = true
}

type semaStandardChildRelationshipMember struct {
	name string
	typ  string
}

var semaStandardChildRelationshipCache struct {
	once     sync.Once
	byParent map[string][]semaStandardChildRelationshipMember
}

func semaStandardChildRelationshipMembers(objectName string) []semaStandardChildRelationshipMember {
	semaStandardChildRelationshipCache.once.Do(func() {
		byParent := make(map[string][]semaStandardChildRelationshipMember)
		seen := make(map[string]bool)
		for _, childObject := range storage.KnownStandardObjectNames() {
			childName := childObject
			if canonical, ok := storage.ResolveKnownStandardObjectName(childObject); ok {
				childName = canonical
			}
			storage.VisitStandardObjectRelationships(childName, nil, func(relationship storage.Relationship) {
				if relationship.ChildRelationship == "" {
					return
				}
				for _, parent := range relationship.ParentObjects {
					parentName := parent
					if canonical, ok := storage.ResolveKnownStandardObjectName(parent); ok {
						parentName = canonical
					}
					parentKey := normalizeName(parentName)
					if parentKey == "" {
						continue
					}
					member := semaStandardChildRelationshipMember{
						name: relationship.ChildRelationship,
						typ:  "List<" + childName + ">",
					}
					dedupeKey := parentKey + "\x00" + normalizeName(member.name) + "\x00" + normalizeName(member.typ)
					if seen[dedupeKey] {
						continue
					}
					seen[dedupeKey] = true
					byParent[parentKey] = append(byParent[parentKey], member)
				}
			})
		}
		semaStandardChildRelationshipCache.byParent = byParent
	})
	key := normalizeName(objectName)
	if canonical, ok := storage.ResolveKnownStandardObjectName(objectName); ok {
		key = normalizeName(canonical)
	}
	return semaStandardChildRelationshipCache.byParent[key]
}

func semaStandardChildRelationshipMemberForKey(objectName, fieldKey string) (typesys.MemberSymbol, bool) {
	for _, relationship := range semaStandardChildRelationshipMembers(objectName) {
		if normalizeName(relationship.name) != fieldKey {
			continue
		}
		return typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      relationship.name,
			Type:      relationship.typ,
			Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
		}, true
	}
	return typesys.MemberSymbol{}, false
}

func semaStandardSObjectNameForKey(key string) (string, bool) {
	semaStandardSObjectMembers()
	name, ok := semaStandardSObjectMembersCache.nameByKey[key]
	return name, ok
}

func semaStandardChangeEventNames(objectNames []string) []string {
	seen := make(map[string]bool, len(objectNames))
	out := make([]string, 0, len(objectNames))
	for _, objectName := range objectNames {
		if _, ok := semaChangeEventBaseObjectName(objectName); ok {
			continue
		}
		name := objectName + "ChangeEvent"
		key := normalizeName(name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func semaChangeEventBaseObjectName(objectName string) (string, bool) {
	name := strings.TrimSpace(objectName)
	if strings.HasSuffix(name, "__ChangeEvent") {
		base := strings.TrimSuffix(name, "__ChangeEvent") + "__c"
		return base, base != "__c"
	}
	if strings.HasSuffix(name, "ChangeEvent") {
		base := strings.TrimSuffix(name, "ChangeEvent")
		return base, base != ""
	}
	return "", false
}

func semaApplyPlatformInterfaceOverlays(model map[string]typeMembers) {
	semaSetPlatformInterface(model, "Callable", []string{"System.Callable"}, []typesys.MemberSymbol{{
		Kind: apexast.DeclarationMethod,
		Name: "call",
		Type: "Object",
		Parameters: []apexast.Parameter{
			{Name: "action", Type: "String"},
			{Name: "args", Type: "Map<String,Object>"},
		},
	}})
	semaSetPlatformInterface(model, "StubProvider", []string{"System.StubProvider"}, []typesys.MemberSymbol{{
		Kind: apexast.DeclarationMethod,
		Name: "handleMethodCall",
		Type: "Object",
		Parameters: []apexast.Parameter{
			{Name: "stubbedObject", Type: "Object"},
			{Name: "stubbedMethodName", Type: "String"},
			{Name: "returnType", Type: "Type"},
			{Name: "listOfParamTypes", Type: "List<Type>"},
			{Name: "listOfParamNames", Type: "List<String>"},
			{Name: "listOfArgs", Type: "List<Object>"},
		},
	}})
	semaSetPlatformInterface(model, "HttpCalloutMock", []string{"System.HttpCalloutMock"}, []typesys.MemberSymbol{{
		Kind: apexast.DeclarationMethod,
		Name: "respond",
		Type: "HttpResponse",
		Parameters: []apexast.Parameter{
			{Name: "request", Type: "HttpRequest"},
		},
	}})
}

func semaSetPlatformInterface(model map[string]typeMembers, name string, aliases []string, methods []typesys.MemberSymbol) {
	members, ok := model[normalizeName(name)]
	if !ok {
		members = typeMembers{
			name:     name,
			shortKey: semaShortTypeKey(name),
			kind:     apexast.DeclarationInterface,
			methods:  make(map[string][]typesys.MemberSymbol),
			fields:   make(map[string]typesys.MemberSymbol),
		}
	}
	if members.methods == nil {
		members.methods = make(map[string][]typesys.MemberSymbol)
	}
	if members.fields == nil {
		members.fields = make(map[string]typesys.MemberSymbol)
	}
	if members.kind == "" {
		members.kind = apexast.DeclarationInterface
	}
	for _, method := range methods {
		key := normalizeName(method.Name)
		if len(members.methods[key]) == 0 {
			members.methods[key] = append(members.methods[key], method)
		}
	}
	model[normalizeName(name)] = members
	for _, alias := range aliases {
		if _, exists := model[normalizeName(alias)]; !exists {
			model[normalizeName(alias)] = members
		}
	}
}

func semaApplyPlatformFieldOverlays(model map[string]typeMembers) {
	semaSetPlatformField(model, "RestContext", "request", "RestRequest", true)
	semaSetPlatformField(model, "RestContext", "response", "RestResponse", true)
	for _, field := range []struct {
		name string
		typ  string
	}{
		{"headers", "Map<String,String>"},
		{"httpMethod", "String"},
		{"params", "Map<String,String>"},
		{"remoteAddress", "String"},
		{"requestBody", "Blob"},
		{"requestURI", "String"},
		{"resourcePath", "String"},
	} {
		semaSetPlatformField(model, "RestRequest", field.name, field.typ, false)
	}
	for _, field := range []struct {
		name string
		typ  string
	}{
		{"headers", "Map<String,String>"},
		{"responseBody", "Blob"},
		{"statusCode", "Integer"},
	} {
		semaSetPlatformField(model, "RestResponse", field.name, field.typ, false)
	}
}

func semaSetPlatformField(model map[string]typeMembers, typeName, fieldName, fieldType string, static bool) {
	key := normalizeName(typeName)
	members, ok := model[key]
	if !ok {
		members = typeMembers{name: typeName, shortKey: semaShortTypeKey(typeName), methods: make(map[string][]typesys.MemberSymbol), fields: make(map[string]typesys.MemberSymbol)}
	}
	if members.fields == nil {
		members.fields = make(map[string]typesys.MemberSymbol)
	}
	field := members.fields[normalizeName(fieldName)]
	field.Kind = apexast.DeclarationProperty
	field.Name = fieldName
	field.Type = fieldType
	field.Modifiers = semaWithModifier(field.Modifiers, "public")
	if static {
		field.Modifiers = semaWithModifier(field.Modifiers, "static")
	}
	members.fields[normalizeName(fieldName)] = field
	model[key] = members
}

func semaWithModifier(modifiers []string, modifier string) []string {
	for _, existing := range modifiers {
		if strings.EqualFold(existing, modifier) {
			return modifiers
		}
	}
	return append(modifiers, modifier)
}

func semaAdditionalStandardSObjectNames() []string {
	return []string{"ApexClass", "ApexPage", "CronJobDetail", "CronTrigger", "EntityDefinition", "EntityParticle", "FieldDefinition", "FlowDefinitionView", "Folder", "NamedCredential", "Note", "RecentlyViewed", "Report", "UserEntityAccess", "UserFieldAccess", "UserRecordAccess"}
}

func semaAddCommonSObjectFields(fields map[string]typesys.MemberSymbol) {
	fields[normalizeName("SObjectType")] = typesys.MemberSymbol{
		Kind:      apexast.DeclarationField,
		Name:      "SObjectType",
		Type:      "Schema.SObjectType",
		Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
	}
	for _, field := range []schema.Field{
		{Name: "Id", Type: "Id"},
		{Name: "Name", Type: "Text"},
		{Name: "IsDeleted", Type: "Checkbox"},
		{Name: "CreatedDate", Type: "Datetime"},
		{Name: "LastActivityDate", Type: "Date"},
		{Name: "LastModifiedDate", Type: "Datetime"},
		{Name: "SystemModstamp", Type: "Datetime"},
		{Name: "CreatedById", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"},
		{Name: "LastModifiedById", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"},
		{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
	} {
		key := normalizeName(field.Name)
		if _, exists := fields[key]; !exists {
			fields[key] = typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.Name,
				Type:      semaApexTypeForSchemaField(field),
				Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
			}
		}
		if field.RelationshipName == "" || len(field.ReferenceTo) == 0 {
			continue
		}
		relationshipKey := normalizeName(field.RelationshipName)
		if _, exists := fields[relationshipKey]; exists {
			continue
		}
		relationshipType := "SObject"
		if len(field.ReferenceTo) == 1 {
			relationshipType = field.ReferenceTo[0]
		}
		fields[relationshipKey] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      field.RelationshipName,
			Type:      relationshipType,
			Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
		}
	}
}

func semaObjectSupportsRecordTypeRelationship(object schema.Object) bool {
	if strings.HasSuffix(strings.ToLower(object.Name), "__c") {
		return true
	}
	if len(object.RecordTypes) > 0 {
		return true
	}
	for _, field := range object.Fields {
		if strings.EqualFold(field.Name, "RecordTypeId") || strings.EqualFold(field.RelationshipName, "RecordType") {
			return true
		}
	}
	return false
}

func semaAddSObjectRecordTypeRelationship(fields map[string]typesys.MemberSymbol) {
	if _, exists := fields[normalizeName("RecordTypeId")]; !exists {
		fields[normalizeName("RecordTypeId")] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      "RecordTypeId",
			Type:      "Id",
			Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
		}
	}
	if _, exists := fields[normalizeName("RecordType")]; exists {
		return
	}
	fields[normalizeName("RecordType")] = typesys.MemberSymbol{
		Kind:      apexast.DeclarationField,
		Name:      "RecordType",
		Type:      "RecordType",
		Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
	}
}

func semaFallbackStandardSObjectFields(objectName string, synthetic bool) []schema.Field {
	switch {
	case strings.EqualFold(objectName, "Name"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
			{Name: "Type", Type: "Text"},
		}
	case strings.EqualFold(objectName, "FlowDefinitionView"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "ActiveVersionId", Type: "Id"},
			{Name: "ApiVersionRuntime", Type: "Number"},
			{Name: "ApiName", Type: "Text"},
			{Name: "Description", Type: "LongTextArea"},
			{Name: "DurableId", Type: "Text"},
			{Name: "FlowDefinitionViewId", Type: "Id"},
			{Name: "Label", Type: "Text"},
			{Name: "LastModifiedBy", Type: "Text"},
			{Name: "LastModifiedDate", Type: "Datetime"},
			{Name: "ManageableState", Type: "Picklist"},
			{Name: "OverriddenById", Type: "Lookup", ReferenceTo: []string{"Schema.FlowDefinitionView"}, RelationshipName: "OverriddenBy"},
			{Name: "OverriddenFlowId", Type: "Lookup", ReferenceTo: []string{"Schema.FlowDefinitionView"}, RelationshipName: "OverriddenFlow"},
			{Name: "ProcessType", Type: "Text"},
			{Name: "RecordTriggerType", Type: "Text"},
			{Name: "SourceTemplateId", Type: "Lookup", ReferenceTo: []string{"Schema.FlowDefinitionView"}, RelationshipName: "SourceTemplate"},
			{Name: "TriggerOrder", Type: "Number"},
			{Name: "TriggerType", Type: "Text"},
			{Name: "TriggerObjectOrEventId", Type: "Lookup", ReferenceTo: []string{"EntityDefinition"}, RelationshipName: "TriggerObjectOrEvent"},
		}
	case strings.EqualFold(objectName, "Event"):
		return []schema.Field{
			{Name: "IsClosed", Type: "Checkbox"},
		}
	case strings.EqualFold(objectName, "AccountShare"):
		return semaStandardShareFallbackFields("Account", []string{"AccountAccessLevel", "CaseAccessLevel", "ContactAccessLevel", "OpportunityAccessLevel"})
	case strings.EqualFold(objectName, "CaseShare"):
		return semaStandardShareFallbackFields("Case", []string{"CaseAccessLevel"})
	case strings.EqualFold(objectName, "ContactShare"):
		return semaStandardShareFallbackFields("Contact", []string{"ContactAccessLevel"})
	case strings.EqualFold(objectName, "LeadShare"):
		return semaStandardShareFallbackFields("Lead", []string{"LeadAccessLevel"})
	case strings.EqualFold(objectName, "OpportunityShare"):
		return semaStandardShareFallbackFields("Opportunity", []string{"OpportunityAccessLevel"})
	case strings.EqualFold(objectName, "ApexClass"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
			{Name: "NamespacePrefix", Type: "Text"},
			{Name: "Body", Type: "LongTextArea"},
		}
	case strings.EqualFold(objectName, "CronJobDetail"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
		}
	case strings.EqualFold(objectName, "CronTrigger"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "CronExpression", Type: "Text"},
			{Name: "TimesTriggered", Type: "Number"},
			{Name: "NextFireTime", Type: "Datetime"},
			{Name: "CronJobDetailId", Type: "Id"},
		}
	case synthetic:
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
		}
	default:
		return nil
	}
}

func semaStandardShareFallbackFields(parentObject string, accessFields []string) []schema.Field {
	fields := []schema.Field{
		{Name: "Id", Type: "Id"},
		{Name: parentObject + "Id", Type: "Lookup", ReferenceTo: []string{parentObject}, RelationshipName: parentObject},
		{Name: "UserOrGroupId", Type: "Lookup", ReferenceTo: []string{"Name"}, RelationshipName: "UserOrGroup"},
		{Name: "RowCause", Type: "Picklist"},
	}
	for _, accessField := range accessFields {
		fields = append(fields, schema.Field{Name: accessField, Type: "Picklist"})
	}
	return fields
}

func addFallbackStandardSObjectRelationshipMembers(members *typeMembers, objectName string) {
	if !strings.EqualFold(objectName, "PermissionSet") {
		return
	}
	members.fields[normalizeName("Assignments")] = typesys.MemberSymbol{
		Kind:      apexast.DeclarationField,
		Name:      "Assignments",
		Type:      "List<PermissionSetAssignment>",
		Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
	}
}

func semaApexTypeForSchemaField(field schema.Field) string {
	return semaApexTypeForSchemaFieldInObjects(nil, field)
}

func semaApexTypeForSchemaFieldInObjects(objects []schema.Object, field schema.Field) string {
	fieldType := normalizeName(field.Type)
	if fieldType == "any" || fieldType == "object" {
		fieldType = ""
	}
	if fieldType == "summary" {
		if summarizedType := semaApexTypeForSummarizedField(objects, field.SummarizedField); summarizedType != "" {
			return summarizedType
		}
	}
	switch fieldType {
	case "id":
		return "Id"
	case "checkbox", "boolean":
		return "Boolean"
	case "int", "integer":
		return "Integer"
	case "long":
		return "Long"
	case "double", "currency", "percent", "number", "summary":
		return "Decimal"
	case "date":
		return "Date"
	case "datetime":
		return "Datetime"
	case "time":
		return "Time"
	case "base64", "blob":
		return "Blob"
	case "address":
		return "Address"
	case "location":
		return "Location"
	case "metadatarelationship":
		return "String"
	case "lookup", "masterdetail", "externallookup", "indirectlookup", "reference":
		return "Id"
	case "text", "textarea", "longtextarea", "html", "encryptedtext", "email", "phone", "url", "picklist", "multipicklist", "multiselectpicklist", "combobox", "autonumber":
		return "String"
	}
	if field.Name != "" {
		key := normalizeName(field.Name)
		localKey := normalizeName(semaSchemaLocalAPIName(field.Name))
		switch {
		case key == "id" || strings.HasSuffix(key, "id"):
			return "Id"
		case strings.HasSuffix(key, "email"):
			return "String"
		case key == "isdeleted" || strings.HasPrefix(key, "is") || strings.HasPrefix(key, "has") || localKey == "mandatory__c":
			return "Boolean"
		case strings.HasSuffix(localKey, "datetime__c"):
			return "Datetime"
		case strings.HasSuffix(localKey, "date__c"):
			return "Date"
		case key == "name" || key == "developername" || key == "masterlabel" || key == "label" || key == "namespaceprefix" || key == "qualifiedapiname":
			return "String"
		case strings.Contains(key, "name") || strings.Contains(key, "class") || strings.HasSuffix(key, "type") ||
			strings.Contains(localKey, "name") || strings.Contains(localKey, "class") || strings.HasSuffix(localKey, "type__c"):
			return "String"
		}
	}
	if fieldType == "" {
		return ""
	}
	return "Object"
}

func semaApexTypeForSummarizedField(objects []schema.Object, summarizedField string) string {
	objectName, fieldName, ok := strings.Cut(strings.TrimSpace(summarizedField), ".")
	if !ok || objectName == "" || fieldName == "" {
		return ""
	}
	for _, object := range objects {
		if !semaSchemaAPINameEquivalent(object.Name, objectName) {
			continue
		}
		for _, field := range object.Fields {
			if semaSchemaAPINameEquivalent(field.Name, fieldName) {
				return semaApexTypeForSchemaFieldInObjects(objects, field)
			}
		}
	}
	return ""
}

func semaSchemaAPINameEquivalent(left, right string) bool {
	if strings.EqualFold(left, right) {
		return true
	}
	return strings.EqualFold(semaSchemaLocalAPIName(left), semaSchemaLocalAPIName(right))
}

func semaSchemaLocalAPIName(name string) string {
	name = strings.TrimSpace(name)
	if !semaIsCustomAPIName(name) || !semaHasNamespaceToken(name) {
		return name
	}
	first := strings.Index(name, "__")
	if first <= 0 || first+2 >= len(name) {
		return name
	}
	return name[first+2:]
}

func semaApexTypeForStorageField(field storage.Field) string {
	switch field.Type {
	case storage.FieldID:
		return "Id"
	case storage.FieldBoolean:
		return "Boolean"
	case storage.FieldInteger:
		return "Integer"
	case storage.FieldDecimal:
		return "Decimal"
	case storage.FieldDate:
		return "Date"
	case storage.FieldDateTime:
		return "Datetime"
	case storage.FieldAddress:
		return "Address"
	case storage.FieldBlob:
		return "Blob"
	case storage.FieldReference:
		return "Id"
	case storage.FieldString, storage.FieldPicklist, storage.FieldMultiPicklist:
		return "String"
	default:
		if strings.HasSuffix(normalizeName(field.APIName), "address") {
			return "Address"
		}
		return semaApexTypeForSchemaField(schema.Field{Name: field.APIName})
	}
}

func semaEnumValues(typ typesys.TypeSymbol) []string {
	if typ.File == "" || typ.Range.Start.Offset < 0 || typ.Range.End.Offset <= typ.Range.Start.Offset {
		return nil
	}
	data, err := os.ReadFile(typ.File)
	if err != nil {
		return nil
	}
	source := string(data)
	if typ.Range.End.Offset > len(source) {
		return nil
	}
	decl := source[typ.Range.Start.Offset:typ.Range.End.Offset]
	open := strings.IndexByte(decl, '{')
	close := strings.LastIndexByte(decl, '}')
	if open < 0 || close <= open {
		return nil
	}
	body := decl[open+1 : close]
	// Strip both line and block comments before splitting on commas
	body = stripComments(body)
	parts := strings.Split(body, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if idx := strings.IndexFunc(value, func(r rune) bool {
			return !(r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
		}); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func stripComments(source string) string {
	var sb strings.Builder
	for len(source) > 0 {
		// Find the next comment start
		lineIdx := strings.Index(source, "//")
		blockIdx := strings.Index(source, "/*")
		if lineIdx < 0 && blockIdx < 0 {
			sb.WriteString(source)
			break
		}
		// Determine which comment comes first
		idx := lineIdx
		if blockIdx >= 0 && (lineIdx < 0 || blockIdx < lineIdx) {
			idx = blockIdx
		}
		sb.WriteString(source[:idx])
		if idx == lineIdx {
			// Line comment: skip to end of line
			if nl := strings.IndexByte(source[idx:], '\n'); nl >= 0 {
				source = source[idx+nl:]
			} else {
				break
			}
		} else {
			// Block comment: skip to */
			if end := strings.Index(source[idx+2:], "*/"); end >= 0 {
				source = source[idx+2+end+2:]
			} else {
				break
			}
		}
	}
	return sb.String()
}

func shortNestedTypeName(typeName string) string {
	if idx := strings.LastIndexByte(typeName, '.'); idx >= 0 {
		return typeName[idx+1:]
	}
	return typeName
}

func resolveNestedTypeName(model map[string]typeMembers, owner, typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	if semaShouldPreserveExplicitPlatformType(typeName) {
		return typeName
	}
	if strings.Contains(typeName, ".") {
		if owner != "" {
			candidate := owner + "." + typeName
			if _, ok := model[normalizeName(candidate)]; ok {
				return candidate
			}
		}
		ownerParts := strings.Split(owner, ".")
		for i := len(ownerParts) - 1; i > 0; i-- {
			candidate := strings.Join(append(append([]string{}, ownerParts[:i]...), typeName), ".")
			if _, ok := model[normalizeName(candidate)]; ok {
				return candidate
			}
		}
		if _, ok := model[normalizeName(typeName)]; ok {
			return typeName
		}
		return semaCanonicalPlatformAlias(typeName)
	}
	ownerParts := strings.Split(owner, ".")
	if len(ownerParts) > 0 && strings.EqualFold(ownerParts[0], typeName) {
		return typeName
	}
	if semaIsCustomAPIName(typeName) {
		if _, ok := model[normalizeName(typeName)]; ok {
			return typeName
		}
	}
	if namespace := semaOwnerTypeNamespace(model, owner); namespace != "" {
		if namespaced, ok := semaProjectNamespacedAPIName(namespace, typeName); ok {
			if _, exists := model[normalizeName(namespaced)]; exists {
				return namespaced
			}
		}
	}
	if owner != "" {
		candidate := owner + "." + typeName
		if _, ok := model[normalizeName(candidate)]; ok {
			return candidate
		}
	}
	for i := len(ownerParts) - 1; i > 0; i-- {
		candidate := strings.Join(append(append([]string{}, ownerParts[:i]...), typeName), ".")
		if _, ok := model[normalizeName(candidate)]; ok {
			return candidate
		}
	}
	for i := len(ownerParts) - 1; i > 0; i-- {
		enclosing := strings.Join(ownerParts[:i], ".")
		if resolved := resolveNestedTypeNameFromSuperclasses(model, enclosing, typeName); resolved != "" {
			return resolved
		}
	}
	if resolved := resolveNestedTypeNameFromSuperclasses(model, owner, typeName); resolved != "" {
		return resolved
	}
	return semaCanonicalPlatformAlias(typeName)
}

func resolveNestedTypeNameFromSuperclasses(model map[string]typeMembers, owner, typeName string) string {
	seen := make(map[string]bool)
	for current := owner; current != ""; {
		key := normalizeName(current)
		if key == "" || seen[key] {
			break
		}
		seen[key] = true
		members, ok := model[key]
		if !ok || strings.TrimSpace(members.superClass) == "" {
			break
		}
		candidate := members.superClass + "." + typeName
		if _, ok := model[normalizeName(candidate)]; ok {
			return candidate
		}
		current = members.superClass
	}
	return ""
}

func semaShouldPreserveExplicitPlatformType(typeName string) bool {
	if !semaExplicitPlatformQualifiedName(typeName) {
		return false
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	return !strings.EqualFold(canonical, typeName)
}

func semaOwnerTypeNamespace(model map[string]typeMembers, owner string) string {
	members, _, ok := semaLookupTypeMembers(model, owner)
	if !ok || strings.TrimSpace(members.namespace) == "" {
		return ""
	}
	return members.namespace
}

func resolveNestedTypeReference(model map[string]typeMembers, owner, typeName string) string {
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) == 0 {
		return resolveNestedTypeName(model, owner, typeName)
	}
	resolvedArgs := make([]string, len(args))
	for i, arg := range args {
		resolvedArgs[i] = resolveNestedTypeReference(model, owner, arg)
	}
	return resolveNestedTypeName(model, owner, base) + "<" + strings.Join(resolvedArgs, ",") + ">"
}

func buildConstructability(index typesys.Index) map[string]typesys.TypeSymbol {
	out := make(map[string]typesys.TypeSymbol)
	for _, typ := range index.Types {
		if !typ.Dependency {
			out[normalizeName(typ.Name)] = typ
		}
		if typ.Namespace != "" {
			out[normalizeName(typ.Namespace+"."+typ.Name)] = typ
		}
	}
	return out
}
