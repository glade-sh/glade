package sema

import (
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

type typeMembers struct {
	name         string
	shortKey     string
	namespace    string
	dependency   bool
	sobject      bool
	kind         apexast.DeclarationKind
	superClass   string
	interfaces   []string
	methods      map[string][]typesys.MemberSymbol
	constructors []typesys.MemberSymbol
	fields       map[string]typesys.MemberSymbol
}

const semaCurrentTypeScopeKey = "__glade_current_type"

const semaInferenceDepthScopeKey = "__glade_inference_depth"

const semaSyntheticStandardSObjectFieldModifier = "__glade_standard_sobject_field"

func (a *Analyzer) checkMethodBodies(index typesys.Index) []diagnostic.Diagnostic {
	model := buildTypeMembers(index)
	defer unregisterSemaShortCandidateIndex(model)
	constructability := buildConstructability(index)
	sources := make(map[string]string)
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if typ.Artifact {
			continue
		}
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
					name:       nestedName,
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
				out[normalizeName(nestedName)] = enumMembers
				if typ.Namespace != "" {
					out[normalizeName(typ.Namespace+"."+nestedName)] = enumMembers
				}
				shortAliases[normalizeName(member.Name)] = append(shortAliases[normalizeName(member.Name)], nestedName)
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
		if !semaRequiresQualifiedDependencyName(typ) && out[normalizeName(typ.Name)].name == "" {
			out[normalizeName(typ.Name)] = members
		}
		if typ.Namespace != "" {
			out[normalizeName(typ.Namespace+"."+typ.Name)] = members
		}
		if short := shortNestedTypeName(typ.Name); !semaRequiresQualifiedDependencyName(typ) && short != typ.Name {
			shortAliases[normalizeName(short)] = append(shortAliases[normalizeName(short)], typ.Name)
		}
	}
	for _, object := range index.Objects {
		objectKey := normalizeName(object.Name)
		objectMembers, objectOK := out[objectKey]
		if objectOK && !objectMembers.sobject {
			continue
		}
		if !objectOK {
			objectMembers = typeMembers{
				name:     object.Name,
				shortKey: semaShortTypeKey(object.Name),
				sobject:  true,
				kind:     apexast.DeclarationClass,
				fields:   make(map[string]typesys.MemberSymbol),
				methods:  make(map[string][]typesys.MemberSymbol),
			}
		}
		objectMembers.sobject = true
		if objectMembers.fields == nil {
			objectMembers.fields = make(map[string]typesys.MemberSymbol)
		}
		semaAddCommonSObjectFields(objectMembers.fields)
		for _, field := range object.Fields {
			if field.Name != "" {
				objectMembers.fields[normalizeName(field.Name)] = typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      field.Name,
					Type:      semaApexTypeForSchemaField(field),
					Modifiers: []string{"public"},
				}
			}
			if field.RelationshipName != "" && len(field.ReferenceTo) != 0 {
				relationshipType := "SObject"
				if len(field.ReferenceTo) == 1 {
					relationshipType = field.ReferenceTo[0]
				}
				objectMembers.fields[normalizeName(field.RelationshipName)] = typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      field.RelationshipName,
					Type:      relationshipType,
					Modifiers: []string{"public"},
				}
			}
			if relationshipFieldName := semaParentRelationshipFieldName(field.Name); relationshipFieldName != "" && len(field.ReferenceTo) != 0 {
				relationshipType := "SObject"
				if len(field.ReferenceTo) == 1 {
					relationshipType = field.ReferenceTo[0]
				}
				objectMembers.fields[normalizeName(relationshipFieldName)] = typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      relationshipFieldName,
					Type:      relationshipType,
					Modifiers: []string{"public"},
				}
			}
			childRelationshipNames := []string{}
			if field.ChildRelationshipName != "" {
				childRelationshipNames = append(childRelationshipNames, field.ChildRelationshipName)
			}
			if field.RelationshipName != "" {
				childRelationshipNames = append(childRelationshipNames, field.RelationshipName)
			}
			if len(childRelationshipNames) == 0 {
				continue
			}
			for _, parent := range field.ReferenceTo {
				parentKey := normalizeName(parent)
				if parentKey == "" {
					continue
				}
				parentMembers, ok := out[parentKey]
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
				parentMembers.sobject = true
				if parentMembers.fields == nil {
					parentMembers.fields = make(map[string]typesys.MemberSymbol)
				}
				for _, childRelationshipName := range childRelationshipNames {
					parentMembers.fields[normalizeName(childRelationshipName)] = typesys.MemberSymbol{
						Kind:      apexast.DeclarationField,
						Name:      childRelationshipName,
						Type:      "List<" + object.Name + ">",
						Modifiers: []string{"public"},
					}
					if strings.HasSuffix(childRelationshipName, "__r") {
						continue
					}
					childRelationshipAlias := childRelationshipName + "__r"
					parentMembers.fields[normalizeName(childRelationshipAlias)] = typesys.MemberSymbol{
						Kind:      apexast.DeclarationField,
						Name:      childRelationshipAlias,
						Type:      "List<" + object.Name + ">",
						Modifiers: []string{"public"},
					}
				}
				out[parentKey] = parentMembers
			}
		}
		out[objectKey] = objectMembers
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
	return typ.Dependency && typ.Namespace != "" && (typ.Artifact || typ.SourceRoot != "")
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
	for _, objectName := range append(storage.KnownStandardObjectNames(), semaAdditionalStandardSObjectNames()...) {
		key := normalizeName(objectName)
		if key == "" {
			continue
		}
		members, ok := out[key]
		synthetic := !ok
		if ok && !members.sobject {
			continue
		}
		if !ok {
			members = typeMembers{
				name:     objectName,
				shortKey: semaShortTypeKey(objectName),
				sobject:  true,
				kind:     apexast.DeclarationClass,
				fields:   make(map[string]typesys.MemberSymbol),
				methods:  make(map[string][]typesys.MemberSymbol),
			}
		}
		members.sobject = true
		if members.fields == nil {
			members.fields = make(map[string]typesys.MemberSymbol)
		}
		semaAddCommonSObjectFields(members.fields)
		members.fields[normalizeName("SObjectType")] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      "SObjectType",
			Type:      "Schema.SObjectType",
			Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
		}
		if definition, ok := storage.StandardObjectDefinition(objectName); ok {
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
		for _, field := range semaFallbackStandardSObjectFields(objectName, synthetic) {
			members.fields[normalizeName(field.Name)] = typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.Name,
				Type:      semaApexTypeForSchemaField(field),
				Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
			}
		}
		out[key] = members
	}
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
	return []string{"ApexClass", "ApexPage", "CronJobDetail", "CronTrigger", "EntityDefinition", "EntityParticle", "FieldDefinition", "Folder", "NamedCredential", "Note", "RecentlyViewed", "Report", "UserEntityAccess", "UserFieldAccess", "UserRecordAccess"}
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
		{Name: "CreatedDate", Type: "Datetime"},
		{Name: "LastModifiedDate", Type: "Datetime"},
		{Name: "SystemModstamp", Type: "Datetime"},
		{Name: "CreatedById", Type: "Id"},
		{Name: "LastModifiedById", Type: "Id"},
	} {
		key := normalizeName(field.Name)
		if _, exists := fields[key]; exists {
			continue
		}
		fields[key] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      field.Name,
			Type:      semaApexTypeForSchemaField(field),
			Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
		}
	}
}

func semaFallbackStandardSObjectFields(objectName string, synthetic bool) []schema.Field {
	switch {
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

func semaApexTypeForSchemaField(field schema.Field) string {
	fieldType := normalizeName(field.Type)
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
	case "lookup", "masterdetail", "metadatarelationship", "externallookup", "indirectlookup", "reference":
		return "Id"
	case "text", "textarea", "longtextarea", "html", "encryptedtext", "email", "phone", "url", "picklist", "multipicklist", "multiselectpicklist", "combobox", "autonumber":
		return "String"
	}
	if field.Name != "" {
		key := normalizeName(field.Name)
		switch {
		case key == "id" || strings.HasSuffix(key, "id"):
			return "Id"
		case key == "isdeleted" || strings.HasPrefix(key, "is") || strings.HasPrefix(key, "has"):
			return "Boolean"
		case key == "name" || key == "developername" || key == "masterlabel":
			return "String"
		}
	}
	return "Object"
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
	case storage.FieldString, storage.FieldPicklist:
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
	if typeName == "" || strings.Contains(typeName, ".") {
		return typeName
	}
	ownerParts := strings.Split(owner, ".")
	if len(ownerParts) > 0 && strings.EqualFold(ownerParts[0], typeName) {
		return typeName
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
	return typeName
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
