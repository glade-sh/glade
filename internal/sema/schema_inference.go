package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func enrichIndexWithProjectReferencedSchemaFields(index typesys.Index) typesys.Index {
	if len(index.Objects) == 0 || len(index.Types) == 0 {
		return index
	}
	ctx := newSemaProjectReferencedSchemaContext(index.Objects, index.Project.Namespace)
	sourceCache := make(map[string]string)
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) || typ.File == "" {
			continue
		}
		source, ok := readSemaSourceForType(typ, sourceCache)
		if !ok {
			continue
		}
		semaProjectReferencedSchemaFieldsFromSource(ctx, source)
	}
	index.Objects = ctx.objects
	return index
}

type semaProjectReferencedSchemaContext struct {
	objects                    []schema.Object
	namespace                  string
	exactObjectIndexes         map[string]int
	equivalentObjectIndexes    map[string]int
	inferenceAllowedObjectKeys map[string]bool
	childRelationshipTargetSet map[string]map[string]struct{}
}

func newSemaProjectReferencedSchemaContext(objects []schema.Object, namespace string) *semaProjectReferencedSchemaContext {
	ctx := &semaProjectReferencedSchemaContext{
		objects:                    objects,
		namespace:                  namespace,
		exactObjectIndexes:         make(map[string]int, len(objects)),
		equivalentObjectIndexes:    make(map[string]int, len(objects)),
		inferenceAllowedObjectKeys: make(map[string]bool, len(objects)),
		childRelationshipTargetSet: make(map[string]map[string]struct{}),
	}
	for i := range objects {
		ctx.indexObject(i)
		if len(objects[i].Fields) == 0 {
			ctx.markObjectInferenceAllowed(objects[i].Name)
		}
	}
	ctx.indexChildRelationships()
	return ctx
}

func (ctx *semaProjectReferencedSchemaContext) indexObject(idx int) {
	objectName := ctx.objects[idx].Name
	exactKey := semaProjectReferencedSchemaNameKey(objectName)
	if exactKey != "" {
		if _, exists := ctx.exactObjectIndexes[exactKey]; !exists {
			ctx.exactObjectIndexes[exactKey] = idx
		}
	}
	equivalentKey := semaProjectReferencedSchemaEquivalentNameKey(objectName)
	if equivalentKey != "" {
		if _, exists := ctx.equivalentObjectIndexes[equivalentKey]; !exists {
			ctx.equivalentObjectIndexes[equivalentKey] = idx
		}
	}
}

func (ctx *semaProjectReferencedSchemaContext) indexChildRelationships() {
	for _, object := range ctx.objects {
		for _, field := range object.Fields {
			if field.ChildRelationshipName == "" || len(field.ReferenceTo) == 0 {
				continue
			}
			for _, relationshipKey := range semaProjectReferencedSchemaNameKeys(ctx.namespace, field.ChildRelationshipName) {
				targetSet := ctx.childRelationshipTargetSet[relationshipKey]
				if targetSet == nil {
					targetSet = make(map[string]struct{})
					ctx.childRelationshipTargetSet[relationshipKey] = targetSet
				}
				for _, target := range field.ReferenceTo {
					for _, targetKey := range semaProjectReferencedSchemaNameKeys(ctx.namespace, target) {
						targetSet[targetKey] = struct{}{}
					}
				}
			}
		}
	}
}

func (ctx *semaProjectReferencedSchemaContext) schemaObjectIndex(name string, create bool) (int, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false
	}
	if idx, ok := ctx.exactObjectIndexes[semaProjectReferencedSchemaNameKey(name)]; ok {
		return idx, true
	}
	if namespaced, ok := semaProjectNamespacedAPIName(ctx.namespace, name); ok {
		if idx, found := ctx.exactObjectIndexes[semaProjectReferencedSchemaNameKey(namespaced)]; found {
			return idx, true
		}
	}
	if local, ok := semaProjectLocalAPIName(ctx.namespace, name); ok {
		if idx, found := ctx.exactObjectIndexes[semaProjectReferencedSchemaNameKey(local)]; found {
			return idx, true
		}
	}
	if idx, ok := ctx.equivalentObjectIndexes[semaProjectReferencedSchemaEquivalentNameKey(name)]; ok {
		return idx, true
	}
	if !create || !semaProjectReferencedSchemaObjectLikeName(name) {
		return 0, false
	}
	ctx.objects = append(ctx.objects, schema.Object{Name: name})
	idx := len(ctx.objects) - 1
	ctx.indexObject(idx)
	ctx.markObjectInferenceAllowed(name)
	return idx, true
}

func (ctx *semaProjectReferencedSchemaContext) markObjectInferenceAllowed(name string) {
	for _, key := range semaProjectReferencedSchemaNameKeys(ctx.namespace, name) {
		ctx.inferenceAllowedObjectKeys[key] = true
	}
}

func (ctx *semaProjectReferencedSchemaContext) allowsFieldInference(objectName string) bool {
	if semaIsExternalManagedPackageAPIName(ctx.namespace, objectName) {
		return true
	}
	for _, key := range semaProjectReferencedSchemaNameKeys(ctx.namespace, objectName) {
		if ctx.inferenceAllowedObjectKeys[key] {
			return true
		}
	}
	return false
}

func (ctx *semaProjectReferencedSchemaContext) schemaNameIsChildRelationship(objectName, fieldName string) bool {
	for _, relationshipKey := range semaProjectReferencedSchemaNameKeys(ctx.namespace, fieldName) {
		targetSet := ctx.childRelationshipTargetSet[relationshipKey]
		if len(targetSet) == 0 {
			continue
		}
		for _, objectKey := range semaProjectReferencedSchemaNameKeys(ctx.namespace, objectName) {
			if _, ok := targetSet[objectKey]; ok {
				return true
			}
		}
	}
	return false
}

func semaProjectReferencedSchemaNameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func semaProjectReferencedSchemaEquivalentNameKey(name string) string {
	return strings.ToLower(semaSchemaLocalAPIName(strings.TrimSpace(name)))
}

func semaProjectReferencedSchemaNameKeys(namespace, name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	keys := make([]string, 0, 4)
	add := func(candidate string) {
		key := semaProjectReferencedSchemaNameKey(candidate)
		if key == "" {
			return
		}
		for _, existing := range keys {
			if existing == key {
				return
			}
		}
		keys = append(keys, key)
	}
	add(name)
	if namespaced, ok := semaProjectNamespacedAPIName(namespace, name); ok {
		add(namespaced)
	}
	if local, ok := semaProjectLocalAPIName(namespace, name); ok {
		add(local)
	}
	add(semaSchemaLocalAPIName(name))
	return keys
}

func semaProjectReferencedSchemaFieldsFromSource(ctx *semaProjectReferencedSchemaContext, source string) {
	scanSource := semaProjectReferencedSchemaScanSource(source)
	varTypes := semaProjectReferencedSchemaVariableTypes(ctx, scanSource)
	literals := semaProjectReferencedSObjectLiterals(ctx, scanSource)

	for _, literal := range literals {
		semaRecordProjectReferencedSObjectLiteralLookupFields(ctx, literal, varTypes)
	}
	for _, literal := range literals {
		semaRecordProjectReferencedSObjectLiteralFields(ctx, literal)
	}

	semaRecordProjectReferencedMemberFields(ctx, scanSource, varTypes)
}

func semaProjectReferencedSchemaVariableTypes(ctx *semaProjectReferencedSchemaContext, scanSource string) map[string]string {
	varTypes := make(map[string]string)
	for i := 0; i < len(scanSource); {
		start, end, ok := semaProjectReferencedScanIdentifier(scanSource, i)
		if !ok {
			i++
			continue
		}
		typeName := scanSource[start:end]
		if strings.EqualFold(typeName, "new") {
			i = end
			continue
		}
		j := semaProjectReferencedSkipSpaces(scanSource, end)
		if j >= len(scanSource) || scanSource[j] == '.' {
			i = end
			continue
		}
		if j+1 < len(scanSource) && scanSource[j] == '[' && scanSource[j+1] == ']' {
			j = semaProjectReferencedSkipSpaces(scanSource, j+2)
		}
		varStart, varEnd, varOK := semaProjectReferencedScanIdentifier(scanSource, j)
		if !varOK {
			i = end
			continue
		}
		if idx, ok := ctx.schemaObjectIndex(typeName, false); ok {
			varTypes[normalizeName(scanSource[varStart:varEnd])] = ctx.objects[idx].Name
			i = varEnd
			continue
		}
		if semaProjectReferencedSchemaObjectLikeName(typeName) {
			varTypes[normalizeName(scanSource[varStart:varEnd])] = typeName
			i = varEnd
			continue
		}
		i = end
	}
	return varTypes
}

type semaProjectReferencedSObjectLiteral struct {
	objectName string
	body       string
}

func semaProjectReferencedSObjectLiterals(ctx *semaProjectReferencedSchemaContext, scanSource string) []semaProjectReferencedSObjectLiteral {
	var literals []semaProjectReferencedSObjectLiteral
	for i := 0; i < len(scanSource); {
		start, end, ok := semaProjectReferencedScanIdentifier(scanSource, i)
		if !ok {
			i++
			continue
		}
		objectName := scanSource[start:end]
		j := semaProjectReferencedSkipSpaces(scanSource, end)
		if strings.EqualFold(objectName, "new") {
			objectStart, objectEnd, objectOK := semaProjectReferencedScanIdentifier(scanSource, j)
			if !objectOK {
				i = end
				continue
			}
			objectName = scanSource[objectStart:objectEnd]
			j = semaProjectReferencedSkipSpaces(scanSource, objectEnd)
		}
		if j >= len(scanSource) || scanSource[j] != '(' {
			i = end
			continue
		}
		if _, ok := ctx.schemaObjectIndex(objectName, false); !ok && !semaProjectReferencedSchemaObjectLikeName(objectName) {
			i = end
			continue
		}
		closeParen, ok := semaProjectReferencedMatchingParen(scanSource, j)
		if !ok {
			i = end
			continue
		}
		literals = append(literals, semaProjectReferencedSObjectLiteral{
			objectName: objectName,
			body:       scanSource[j+1 : closeParen],
		})
		i = closeParen + 1
	}
	return literals
}

func semaRecordProjectReferencedSObjectLiteralFields(ctx *semaProjectReferencedSchemaContext, literal semaProjectReferencedSObjectLiteral) {
	for _, arg := range semaProjectReferencedSObjectLiteralNamedArgs(literal.body) {
		fieldType := semaProjectReferencedSchemaFieldTypeFromValue(arg.value)
		semaRecordProjectReferencedSchemaFieldWithType(ctx, literal.objectName, arg.name, fieldType)
	}
}

func semaRecordProjectReferencedSObjectLiteralLookupFields(ctx *semaProjectReferencedSchemaContext, literal semaProjectReferencedSObjectLiteral, varTypes map[string]string) {
	for _, arg := range semaProjectReferencedSObjectLiteralNamedArgs(literal.body) {
		parentVar := semaProjectReferencedSObjectIDValue(arg.value)
		if parentVar == "" {
			continue
		}
		parentObjectName, ok := varTypes[normalizeName(parentVar)]
		if !ok {
			continue
		}
		semaRecordProjectReferencedLookupSchemaField(ctx, literal.objectName, arg.name, parentObjectName)
	}
}

type semaProjectReferencedNamedArg struct {
	name  string
	value string
}

func semaProjectReferencedSObjectLiteralNamedArgs(body string) []semaProjectReferencedNamedArg {
	var args []semaProjectReferencedNamedArg
	for i := 0; i < len(body); {
		i = semaProjectReferencedSkipDelimiters(body, i)
		nameStart, nameEnd, ok := semaProjectReferencedScanIdentifierAt(body, i)
		if !ok {
			i++
			continue
		}
		j := semaProjectReferencedSkipSpaces(body, nameEnd)
		if j >= len(body) || body[j] != '=' || semaProjectReferencedAssignmentOperatorNeighbor(body, j) {
			i = nameEnd
			continue
		}
		valueStart := semaProjectReferencedSkipSpaces(body, j+1)
		valueEnd := semaProjectReferencedTopLevelValueEnd(body, valueStart)
		args = append(args, semaProjectReferencedNamedArg{
			name:  body[nameStart:nameEnd],
			value: strings.TrimSpace(body[valueStart:valueEnd]),
		})
		i = valueEnd
		if i < len(body) && body[i] == ',' {
			i++
		}
	}
	return args
}

func semaProjectReferencedAssignmentOperatorNeighbor(source string, idx int) bool {
	if idx > 0 {
		switch source[idx-1] {
		case '=', '!', '<', '>':
			return true
		}
	}
	if idx+1 < len(source) {
		switch source[idx+1] {
		case '=', '>':
			return true
		}
	}
	return false
}

func semaProjectReferencedSkipDelimiters(source string, idx int) int {
	for idx < len(source) {
		switch source[idx] {
		case ' ', '\t', '\r', '\n', ',':
			idx++
		default:
			return idx
		}
	}
	return idx
}

func semaProjectReferencedTopLevelValueEnd(source string, start int) int {
	parens, brackets, braces, angles := 0, 0, 0, 0
	for i := start; i < len(source); i++ {
		switch source[i] {
		case '/':
			if end, ok := skipSemaComment(source, i); ok {
				i = end
			}
		case '\'':
			i = skipSemaString(source, i)
		case '(':
			parens++
		case ')':
			if parens > 0 {
				parens--
			}
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		case '<':
			if parens == 0 && brackets == 0 && braces == 0 && looksLikeSemaGenericOpen(source, i) {
				angles++
			}
		case '>':
			if parens == 0 && brackets == 0 && braces == 0 && angles > 0 {
				angles--
			}
		case ',':
			if parens == 0 && brackets == 0 && braces == 0 && angles == 0 {
				return i
			}
		}
	}
	return len(source)
}

func semaProjectReferencedSObjectIDValue(value string) string {
	value = strings.TrimSpace(value)
	varStart, varEnd, ok := semaProjectReferencedScanIdentifierAt(value, 0)
	if !ok {
		return ""
	}
	i := semaProjectReferencedSkipSpaces(value, varEnd)
	if i >= len(value) || value[i] != '.' {
		return ""
	}
	fieldStart := semaProjectReferencedSkipSpaces(value, i+1)
	fieldStart, fieldEnd, ok := semaProjectReferencedScanIdentifierAt(value, fieldStart)
	if !ok || !strings.EqualFold(value[fieldStart:fieldEnd], "Id") {
		return ""
	}
	if semaProjectReferencedSkipSpaces(value, fieldEnd) != len(value) {
		return ""
	}
	return value[varStart:varEnd]
}

func semaRecordProjectReferencedMemberFields(ctx *semaProjectReferencedSchemaContext, scanSource string, varTypes map[string]string) {
	for i := 0; i < len(scanSource); {
		start, end, ok := semaProjectReferencedScanIdentifier(scanSource, i)
		if !ok {
			i++
			continue
		}
		parts := []string{scanSource[start:end]}
		j := semaProjectReferencedSkipSpaces(scanSource, end)
		for j < len(scanSource) && scanSource[j] == '.' {
			nextStart := semaProjectReferencedSkipSpaces(scanSource, j+1)
			partStart, partEnd, partOK := semaProjectReferencedScanIdentifierAt(scanSource, nextStart)
			if !partOK {
				break
			}
			parts = append(parts, scanSource[partStart:partEnd])
			end = partEnd
			j = semaProjectReferencedSkipSpaces(scanSource, end)
		}
		if len(parts) < 2 {
			i = end
			continue
		}
		if len(parts) >= 5 &&
			strings.EqualFold(parts[0], "Schema") &&
			strings.EqualFold(parts[1], "SObjectType") &&
			strings.EqualFold(parts[3], "fields") {
			semaRecordProjectReferencedSchemaField(ctx, parts[2], parts[4])
			i = end
			continue
		}
		if len(parts) >= 4 &&
			strings.EqualFold(parts[1], "SObjectType") &&
			strings.EqualFold(parts[2], "fields") {
			semaRecordProjectReferencedSchemaField(ctx, parts[0], parts[3])
			i = end
			continue
		}
		if j < len(scanSource) && scanSource[j] == '(' {
			i = end
			continue
		}
		if objectName, ok := varTypes[normalizeName(parts[0])]; ok {
			semaRecordProjectReferencedSchemaField(ctx, objectName, parts[1])
		}
		if _, ok := ctx.schemaObjectIndex(parts[0], false); ok || semaProjectReferencedSchemaObjectLikeName(parts[0]) {
			semaRecordProjectReferencedSchemaField(ctx, parts[0], parts[1])
		}
		i = end
	}
}

func semaRecordProjectReferencedSchemaField(ctx *semaProjectReferencedSchemaContext, objectName, fieldName string) {
	semaRecordProjectReferencedSchemaFieldWithType(ctx, objectName, fieldName, "")
}

func semaRecordProjectReferencedSchemaFieldWithType(ctx *semaProjectReferencedSchemaContext, objectName, fieldName, fieldType string) {
	fieldName = strings.TrimSpace(fieldName)
	if semaSkipProjectReferencedSchemaField(fieldName) {
		return
	}
	idx, ok := ctx.schemaObjectIndex(objectName, semaProjectReferencedSchemaObjectLikeName(objectName))
	if !ok {
		return
	}
	if ctx.schemaNameIsChildRelationship(ctx.objects[idx].Name, fieldName) {
		return
	}
	for _, field := range ctx.objects[idx].Fields {
		if semaProjectReferencedSchemaAPINamesMatch(ctx.namespace, field.Name, fieldName) || semaProjectReferencedSchemaAPINamesMatch(ctx.namespace, field.RelationshipName, fieldName) {
			if fieldType != "" {
				ctx.objects[idx].Fields = semaProjectReferencedUpsertSchemaField(ctx.objects[idx].Fields, schema.Field{Name: fieldName, Type: fieldType})
			}
			return
		}
	}
	if !ctx.allowsFieldInference(ctx.objects[idx].Name) {
		return
	}
	if fieldType == "" {
		fieldType = semaApexTypeForSchemaField(schema.Field{Name: fieldName, Type: "Object"})
	}
	if fieldType == "" {
		fieldType = "Any"
	}
	ctx.objects[idx].Fields = semaProjectReferencedUpsertSchemaField(ctx.objects[idx].Fields, schema.Field{Name: fieldName, Type: fieldType})
}

func semaRecordProjectReferencedLookupSchemaField(ctx *semaProjectReferencedSchemaContext, objectName, fieldName, parentObjectName string) {
	fieldName = strings.TrimSpace(fieldName)
	parentObjectName = strings.TrimSpace(parentObjectName)
	if semaSkipProjectReferencedSchemaField(fieldName) || parentObjectName == "" {
		return
	}
	idx, ok := ctx.schemaObjectIndex(objectName, semaProjectReferencedSchemaObjectLikeName(objectName))
	if !ok {
		return
	}
	parentIdx, parentOK := ctx.schemaObjectIndex(parentObjectName, semaProjectReferencedSchemaObjectLikeName(parentObjectName))
	if parentOK {
		parentObjectName = ctx.objects[parentIdx].Name
	}
	if ctx.schemaNameIsChildRelationship(ctx.objects[idx].Name, fieldName) {
		return
	}
	relationshipName := semaProjectReferencedParentRelationshipName(fieldName)
	if relationshipName == "" {
		semaRecordProjectReferencedSchemaField(ctx, objectName, fieldName)
		return
	}
	for _, field := range ctx.objects[idx].Fields {
		if semaProjectReferencedSchemaAPINamesMatch(ctx.namespace, field.RelationshipName, relationshipName) && len(field.ReferenceTo) > 0 {
			return
		}
	}
	ctx.objects[idx].Fields = semaProjectReferencedUpsertSchemaField(ctx.objects[idx].Fields, schema.Field{
		Name:             fieldName,
		Type:             "Lookup",
		ReferenceTo:      []string{parentObjectName},
		RelationshipName: relationshipName,
	})
}

func semaProjectReferencedUpsertSchemaField(fields []schema.Field, incoming schema.Field) []schema.Field {
	for i, existing := range fields {
		if !semaSchemaAPINameEquivalent(existing.Name, incoming.Name) {
			continue
		}
		if fields[i].Type == "" || strings.EqualFold(fields[i].Type, "Any") || strings.EqualFold(fields[i].Type, "Object") {
			fields[i].Type = incoming.Type
		}
		if len(fields[i].ReferenceTo) == 0 && len(incoming.ReferenceTo) > 0 {
			fields[i].ReferenceTo = append([]string(nil), incoming.ReferenceTo...)
		}
		if fields[i].RelationshipName == "" {
			fields[i].RelationshipName = incoming.RelationshipName
		}
		if fields[i].ChildRelationshipName == "" {
			fields[i].ChildRelationshipName = incoming.ChildRelationshipName
		}
		return fields
	}
	return append(fields, incoming)
}

func semaProjectReferencedSchemaFieldTypeFromValue(value string) string {
	value = strings.TrimSpace(value)
	compact := strings.ToLower(strings.Join(strings.Fields(value), ""))
	switch {
	case value == "":
		return ""
	case strings.HasPrefix(value, "'"):
		return "Text"
	case strings.EqualFold(value, "true"), strings.EqualFold(value, "false"):
		return "Checkbox"
	case strings.HasPrefix(compact, "date.today("), strings.HasPrefix(compact, "system.date.today("), strings.HasPrefix(compact, "date.newinstance("):
		return "Date"
	case strings.HasPrefix(compact, "datetime.now("), strings.HasPrefix(compact, "system.now("), strings.HasPrefix(compact, "system.datetime.now("), strings.HasPrefix(compact, "datetime.newinstance("):
		return "Datetime"
	case decimalLiteralPattern.MatchString(value):
		return "Number"
	case intLiteralPattern.MatchString(value):
		return "Integer"
	default:
		return ""
	}
}

func semaProjectReferencedSchemaAPINamesMatch(namespace, left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if strings.EqualFold(left, right) {
		return true
	}
	if namespaced, ok := semaProjectNamespacedAPIName(namespace, right); ok && strings.EqualFold(left, namespaced) {
		return true
	}
	if namespaced, ok := semaProjectNamespacedAPIName(namespace, left); ok && strings.EqualFold(namespaced, right) {
		return true
	}
	if local, ok := semaProjectLocalAPIName(namespace, left); ok && strings.EqualFold(local, right) {
		return true
	}
	if local, ok := semaProjectLocalAPIName(namespace, right); ok && strings.EqualFold(left, local) {
		return true
	}
	return semaSchemaAPINameEquivalent(left, right)
}

func semaProjectReferencedSchemaObjectLikeName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, "__c") || strings.HasSuffix(lower, "__mdt") || strings.HasSuffix(lower, "__e")
}

func semaProjectReferencedParentRelationshipName(fieldName string) string {
	fieldName = strings.TrimSpace(fieldName)
	lower := strings.ToLower(fieldName)
	switch {
	case strings.HasSuffix(lower, "__c") && len(fieldName) > 3:
		return fieldName[:len(fieldName)-3] + "__r"
	case strings.HasSuffix(lower, "id") && len(fieldName) > 2:
		return fieldName[:len(fieldName)-2]
	default:
		return ""
	}
}

func semaSkipProjectReferencedSchemaField(fieldName string) bool {
	lower := strings.ToLower(strings.TrimSpace(fieldName))
	return fieldName == "" ||
		strings.HasSuffix(lower, "__r") ||
		semaProjectReferencedCommonRelationshipName(lower) ||
		strings.EqualFold(fieldName, "class") ||
		strings.EqualFold(fieldName, "SObjectType") ||
		strings.EqualFold(fieldName, "Fields")
}

func semaProjectReferencedCommonRelationshipName(lower string) bool {
	switch lower {
	case "recordtype", "owner", "createdby", "lastmodifiedby", "setupowner",
		"contact", "account", "parent", "parentaccount", "user":
		return true
	default:
		return false
	}
}

func semaProjectReferencedScanIdentifier(source string, start int) (int, int, bool) {
	if start < 0 {
		start = 0
	}
	for start < len(source) && !semaProjectReferencedIdentifierStart(source[start]) {
		start++
	}
	return semaProjectReferencedScanIdentifierAt(source, start)
}

func semaProjectReferencedScanIdentifierAt(source string, start int) (int, int, bool) {
	if start >= len(source) {
		return 0, 0, false
	}
	if !semaProjectReferencedIdentifierStart(source[start]) {
		return 0, 0, false
	}
	if start > 0 && semaProjectReferencedIdentifierPart(source[start-1]) {
		return 0, 0, false
	}
	end := start + 1
	for end < len(source) && semaProjectReferencedIdentifierPart(source[end]) {
		end++
	}
	return start, end, true
}

func semaProjectReferencedIdentifierStart(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_'
}

func semaProjectReferencedIdentifierPart(ch byte) bool {
	return semaProjectReferencedIdentifierStart(ch) || (ch >= '0' && ch <= '9')
}

func semaProjectReferencedSkipSpaces(source string, idx int) int {
	for idx < len(source) {
		switch source[idx] {
		case ' ', '\t', '\r', '\n':
			idx++
		default:
			return idx
		}
	}
	return idx
}

func semaProjectReferencedMatchingParen(source string, open int) (int, bool) {
	if open < 0 || open >= len(source) || source[open] != '(' {
		return 0, false
	}
	depth := 0
	for i := open; i < len(source); i++ {
		switch source[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func semaProjectReferencedSchemaScanSource(source string) string {
	out := []byte(source)
	mask := func(start, end int) {
		if start < 0 {
			start = 0
		}
		if end > len(out) {
			end = len(out)
		}
		for i := start; i < end; i++ {
			if out[i] != '\n' && out[i] != '\r' {
				out[i] = ' '
			}
		}
	}
	for i := 0; i < len(out); {
		switch {
		case i+1 < len(out) && out[i] == '/' && out[i+1] == '/':
			start := i
			i += 2
			for i < len(out) && out[i] != '\n' && out[i] != '\r' {
				i++
			}
			mask(start, i)
		case i+1 < len(out) && out[i] == '/' && out[i+1] == '*':
			start := i
			i += 2
			for i+1 < len(out) && !(out[i] == '*' && out[i+1] == '/') {
				i++
			}
			if i+1 < len(out) {
				i += 2
			}
			mask(start, i)
		case out[i] == '\'':
			start := i
			i++
			for i < len(out) {
				if out[i] == '\'' {
					if i+1 < len(out) && out[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				if out[i] == '\\' && i+1 < len(out) {
					i += 2
					continue
				}
				i++
			}
			mask(start, i)
		default:
			i++
		}
	}
	return string(out)
}
