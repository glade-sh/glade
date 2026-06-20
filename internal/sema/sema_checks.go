package sema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/sosl"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

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

func (a *Analyzer) checkQuerySemantics(index typesys.Index) []diagnostic.Diagnostic {
	checker := newQuerySemanticsChecker(index)
	if len(checker.objects) == 0 {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	sourceCache := make(map[string]string)
	for _, typ := range index.Types {
		if typ.Artifact || typ.File == "" {
			continue
		}
		source, ok := readSemaSource(typ.File, sourceCache)
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, checker.checkFile(typ.File, source)...)
	}
	return diagnostics
}

type querySemanticsChecker struct {
	objects map[string]schema.Object
	fields  map[string]map[string]schema.Field
}

func newQuerySemanticsChecker(index typesys.Index) querySemanticsChecker {
	checker := querySemanticsChecker{
		objects: make(map[string]schema.Object, len(index.Objects)),
		fields:  make(map[string]map[string]schema.Field, len(index.Objects)),
	}
	for _, object := range index.Objects {
		checker.objects[strings.ToLower(object.Name)] = object
		fieldMap := make(map[string]schema.Field, len(object.Fields))
		for _, field := range object.Fields {
			fieldMap[strings.ToLower(field.Name)] = field
		}
		checker.fields[strings.ToLower(object.Name)] = fieldMap
	}
	return checker
}

func (c querySemanticsChecker) checkFile(file, source string) []diagnostic.Diagnostic {
	locator := newSemaSourceLocator(source)
	spans := newSemaCodeSpans(source)
	var diagnostics []diagnostic.Diagnostic
	for _, literal := range semaSOQLLiterals(source, spans) {
		query, err := soql.Parse(literal.text)
		if err != nil {
			continue
		}
		ctx := queryTextContext{
			file:        file,
			queryText:   literal.text,
			queryOffset: literal.queryOffset,
			locator:     locator,
		}
		diagnostics = append(diagnostics, c.checkSOQLQuery(query, query.Object, ctx, 0, nil)...)
	}
	for _, literal := range semaSOSLLiterals(source, spans) {
		query, err := sosl.Parse(literal.text)
		if err != nil {
			continue
		}
		ctx := queryTextContext{
			file:        file,
			queryText:   literal.text,
			queryOffset: literal.queryOffset,
			locator:     locator,
		}
		diagnostics = append(diagnostics, c.checkSOSLQuery(query, ctx)...)
	}
	return diagnostics
}

type queryTextContext struct {
	file        string
	queryText   string
	queryOffset int
	locator     semaSourceLocator
}

func (c querySemanticsChecker) checkSOQLQuery(query soql.Query, objectName string, ctx queryTextContext, cursor int, aggregateAliases map[string]bool) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	object, ok := c.object(objectName)
	objectCursor := findSOQLFromObject(ctx.queryText, objectName, cursor)
	if !ok {
		return append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_OBJECT", fmt.Sprintf("SOQL query references unknown SObject %q", objectName), objectName, objectCursor))
	}
	if aggregateAliases == nil {
		aggregateAliases = make(map[string]bool)
	}
	for _, aggregate := range query.Aggregates {
		if aggregate.Alias != "" {
			aggregateAliases[strings.ToLower(aggregate.Alias)] = true
		}
		diagnostics = append(diagnostics, c.checkSOQLField(object.Name, aggregate.Field, ctx, cursor)...)
	}
	for _, field := range query.Fields {
		diagnostics = append(diagnostics, c.checkSOQLField(object.Name, field, ctx, cursor)...)
	}
	for _, field := range query.GroupBy {
		if aggregateAliases[strings.ToLower(field)] {
			continue
		}
		diagnostics = append(diagnostics, c.checkSOQLField(object.Name, field, ctx, cursor)...)
	}
	for _, order := range query.Order {
		if aggregateAliases[strings.ToLower(order.Field)] {
			continue
		}
		diagnostics = append(diagnostics, c.checkSOQLField(object.Name, order.Field, ctx, cursor)...)
	}
	if query.Where != nil {
		diagnostics = append(diagnostics, c.checkSOQLCondition(object.Name, *query.Where, ctx, cursor, aggregateAliases)...)
	}
	for _, child := range query.ChildQueries {
		childCursor := findChildQueryCursor(ctx.queryText, child.Query.Object, cursor)
		childObject, ok := c.childObjectForRelationship(object.Name, child.Relationship)
		if !ok {
			diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_RELATIONSHIP", fmt.Sprintf("SOQL query references unknown child relationship %q on %s", child.Relationship, object.Name), child.Relationship, findQueryIdentifier(ctx.queryText, child.Relationship, childCursor)))
			continue
		}
		diagnostics = append(diagnostics, c.checkSOQLQuery(child.Query, childObject.Name, ctx, childCursor, nil)...)
	}
	for _, spec := range query.Typeofs {
		if _, _, ok := c.relationshipField(object.Name, spec.Relationship); !ok {
			diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_RELATIONSHIP", fmt.Sprintf("SOQL query references unknown relationship %q on %s", spec.Relationship, object.Name), spec.Relationship, findQueryIdentifier(ctx.queryText, spec.Relationship, cursor)))
			continue
		}
		for whenObject, fields := range spec.When {
			branch, ok := c.object(whenObject)
			whenCursor := findTypeofWhenObject(ctx.queryText, whenObject, cursor)
			if !ok {
				diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_OBJECT", fmt.Sprintf("TYPEOF branch references unknown SObject %q", whenObject), whenObject, whenCursor))
				continue
			}
			for _, field := range fields {
				diagnostics = append(diagnostics, c.checkSOQLField(branch.Name, field, ctx, whenCursor)...)
			}
		}
		for _, field := range spec.Else {
			diagnostics = append(diagnostics, c.checkSOQLField(object.Name, spec.Relationship+"."+field, ctx, cursor)...)
		}
	}
	return diagnostics
}

func (c querySemanticsChecker) checkSOQLCondition(objectName string, condition soql.Condition, ctx queryTextContext, cursor int, aggregateAliases map[string]bool) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if condition.Field != "" && !aggregateAliases[strings.ToLower(condition.Field)] {
		diagnostics = append(diagnostics, c.checkSOQLField(objectName, condition.Field, ctx, cursor)...)
	}
	if condition.Subquery != nil {
		diagnostics = append(diagnostics, c.checkSOQLQuery(*condition.Subquery, condition.Subquery.Object, ctx, cursor, nil)...)
	}
	for _, child := range condition.And {
		diagnostics = append(diagnostics, c.checkSOQLCondition(objectName, child, ctx, cursor, aggregateAliases)...)
	}
	for _, child := range condition.Or {
		diagnostics = append(diagnostics, c.checkSOQLCondition(objectName, child, ctx, cursor, aggregateAliases)...)
	}
	return diagnostics
}

func (c querySemanticsChecker) checkSOQLField(objectName, fieldPath string, ctx queryTextContext, cursor int) []diagnostic.Diagnostic {
	if fieldPath == "" || fieldPath == "COUNT()" || strings.HasPrefix(strings.ToUpper(fieldPath), "FIELDS(") || strings.Contains(fieldPath, "(") {
		return nil
	}
	if !c.hasFieldMetadata(objectName) {
		return nil
	}
	offset := findQueryIdentifier(ctx.queryText, fieldPath, cursor)
	if !strings.Contains(fieldPath, ".") {
		if _, ok := c.field(objectName, fieldPath); ok {
			return nil
		}
		return []diagnostic.Diagnostic{ctx.diagnostic("GLADESEMA_QUERY_FIELD", fmt.Sprintf("SOQL query references unknown field %s.%s", objectName, fieldPath), fieldPath, offset)}
	}
	parts := strings.Split(fieldPath, ".")
	current := objectName
	for _, relationship := range parts[:len(parts)-1] {
		if !c.hasFieldMetadata(current) {
			return nil
		}
		_, target, ok := c.relationshipField(current, relationship)
		if !ok {
			return []diagnostic.Diagnostic{ctx.diagnostic("GLADESEMA_QUERY_RELATIONSHIP", fmt.Sprintf("SOQL query references unknown relationship path %q on %s", fieldPath, current), fieldPath, offset)}
		}
		current = target
	}
	if _, ok := c.field(current, parts[len(parts)-1]); !ok {
		return []diagnostic.Diagnostic{ctx.diagnostic("GLADESEMA_QUERY_FIELD", fmt.Sprintf("SOQL query references unknown field %s.%s via %q", current, parts[len(parts)-1], fieldPath), fieldPath, offset)}
	}
	return nil
}

func (c querySemanticsChecker) checkSOSLQuery(query sosl.Query, ctx queryTextContext) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	cursor := keywordIndexFold(ctx.queryText, "RETURNING")
	if cursor < 0 {
		cursor = 0
	} else {
		cursor += len("RETURNING")
	}
	for _, returning := range query.Returning {
		objectCursor := findQueryIdentifier(ctx.queryText, returning.Object, cursor)
		object, ok := c.object(returning.Object)
		if !ok {
			diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_OBJECT", fmt.Sprintf("SOSL RETURNING references unknown SObject %q", returning.Object), returning.Object, objectCursor))
			cursor = maxInt(objectCursor+len(returning.Object), cursor)
			continue
		}
		cursor = maxInt(objectCursor+len(returning.Object), cursor)
		for _, field := range returning.Fields {
			fieldCursor := findQueryIdentifier(ctx.queryText, field, cursor)
			if !c.hasFieldMetadata(object.Name) {
				cursor = maxInt(fieldCursor+len(field), cursor)
				continue
			}
			if _, ok := c.field(object.Name, field); !ok {
				diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_SOSL_FIELD", fmt.Sprintf("SOSL RETURNING references unknown field %s.%s", object.Name, field), field, fieldCursor))
			}
			cursor = maxInt(fieldCursor+len(field), cursor)
		}
	}
	return diagnostics
}

func (c querySemanticsChecker) object(name string) (schema.Object, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if object, ok := c.objects[key]; ok {
		return object, true
	}
	definition, ok := storage.StandardObjectDefinition(name)
	if !ok {
		return schema.Object{}, false
	}
	object := schemaObjectFromStorageDefinition(definition)
	c.addObject(object)
	return object, true
}

func (c querySemanticsChecker) field(objectName, fieldName string) (schema.Field, bool) {
	if _, ok := c.object(objectName); !ok {
		return schema.Field{}, false
	}
	fields, ok := c.fields[strings.ToLower(objectName)]
	if !ok {
		return schema.Field{}, false
	}
	field, ok := fields[strings.ToLower(fieldName)]
	return field, ok
}

func (c querySemanticsChecker) hasFieldMetadata(objectName string) bool {
	if _, ok := c.object(objectName); !ok {
		return false
	}
	fields, ok := c.fields[strings.ToLower(objectName)]
	return ok && len(fields) > 0
}

func (c querySemanticsChecker) relationshipField(objectName, relationshipName string) (schema.Field, string, bool) {
	if _, ok := c.object(objectName); !ok {
		return schema.Field{}, "", false
	}
	fields, ok := c.fields[strings.ToLower(objectName)]
	if !ok {
		return schema.Field{}, "", false
	}
	for _, field := range fields {
		if strings.EqualFold(field.RelationshipName, relationshipName) && len(field.ReferenceTo) > 0 {
			return field, field.ReferenceTo[0], true
		}
	}
	return schema.Field{}, "", false
}

func (c querySemanticsChecker) childObjectForRelationship(parentObject, relationship string) (schema.Object, bool) {
	for _, object := range c.objects {
		for _, field := range object.Fields {
			if !strings.EqualFold(field.ChildRelationshipName, relationship) {
				continue
			}
			for _, target := range field.ReferenceTo {
				if strings.EqualFold(target, parentObject) {
					return object, true
				}
			}
		}
	}
	for _, objectName := range storage.KnownStandardObjectNames() {
		object, ok := c.object(objectName)
		if !ok {
			continue
		}
		for _, field := range object.Fields {
			if !strings.EqualFold(field.ChildRelationshipName, relationship) {
				continue
			}
			for _, target := range field.ReferenceTo {
				if strings.EqualFold(target, parentObject) {
					return object, true
				}
			}
		}
	}
	return schema.Object{}, false
}

func (c querySemanticsChecker) addObject(object schema.Object) {
	if strings.TrimSpace(object.Name) == "" {
		return
	}
	key := strings.ToLower(object.Name)
	c.objects[key] = object
	fieldMap := make(map[string]schema.Field, len(object.Fields))
	for _, field := range object.Fields {
		fieldMap[strings.ToLower(field.Name)] = field
	}
	c.fields[key] = fieldMap
}

func schemaObjectFromStorageDefinition(definition storage.ObjectDefinition) schema.Object {
	object := schema.Object{
		Name:         definition.APIName,
		Label:        definition.Label,
		PluralLabel:  definition.PluralLabel,
		SharingModel: definition.SharingModel,
		Fields:       make([]schema.Field, 0, len(definition.Fields)),
	}
	relationships := make(map[string]storage.Relationship, len(definition.Relations))
	for _, relationship := range definition.Relations {
		if relationship.Field != "" {
			relationships[strings.ToLower(relationship.Field)] = relationship
		}
	}
	for _, field := range definition.Fields {
		relationship := relationships[strings.ToLower(field.APIName)]
		referenceTo := append([]string(nil), field.ReferenceTo...)
		if len(referenceTo) == 0 && len(relationship.ParentObjects) > 0 {
			referenceTo = append(referenceTo, relationship.ParentObjects...)
		}
		relationshipName := field.RelationshipName
		if relationshipName == "" {
			relationshipName = relationship.ParentRelationship
		}
		childRelationshipName := field.ChildRelationshipName
		if childRelationshipName == "" {
			childRelationshipName = relationship.ChildRelationship
		}
		object.Fields = append(object.Fields, schema.Field{
			Name:                  field.APIName,
			Label:                 field.Label,
			Type:                  string(field.Type),
			Length:                field.Length,
			Precision:             field.Precision,
			Scale:                 field.Scale,
			ReferenceTo:           referenceTo,
			RelationshipName:      relationshipName,
			ChildRelationshipName: childRelationshipName,
			DefaultValue:          field.DefaultValue,
			Required:              field.Required,
			ExternalID:            field.ExternalID,
			IDLookup:              field.IDLookup,
			Unique:                field.Unique,
			Encrypted:             field.Encrypted,
			Formula:               field.Formula,
		})
	}
	sort.Slice(object.Fields, func(i, j int) bool {
		return object.Fields[i].Name < object.Fields[j].Name
	})
	return object
}

func (ctx queryTextContext) diagnostic(code, message, token string, queryOffset int) diagnostic.Diagnostic {
	if queryOffset < 0 {
		queryOffset = 0
	}
	start := ctx.queryOffset + queryOffset
	end := start + len(token)
	rng := ctx.locator.rangeFor(start, end)
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     code,
		Message:  message,
		File:     ctx.file,
		Range:    &rng,
	}
}

type semaQueryLiteral struct {
	text        string
	queryOffset int
}

func semaSOQLLiterals(source string, spans semaCodeSpans) []semaQueryLiteral {
	var out []semaQueryLiteral
	for i := 0; i < len(source); i++ {
		if source[i] != '[' || !spans.contains(i) {
			continue
		}
		end := semaMatchingBracket(source, i)
		if end < 0 {
			continue
		}
		raw := source[i+1 : end]
		trimmed := strings.TrimLeft(raw, " \t\r\n")
		leading := len(raw) - len(trimmed)
		if startsWithQueryKeyword(trimmed, "SELECT") {
			out = append(out, semaQueryLiteral{text: strings.TrimSpace(raw), queryOffset: i + 1 + leading})
		}
		i = end
	}
	return out
}

func semaSOSLLiterals(source string, spans semaCodeSpans) []semaQueryLiteral {
	var out []semaQueryLiteral
	for i := 0; i < len(source); i++ {
		if source[i] != '[' || !spans.contains(i) {
			continue
		}
		end := semaMatchingBracket(source, i)
		if end < 0 {
			continue
		}
		raw := source[i+1 : end]
		trimmed := strings.TrimLeft(raw, " \t\r\n")
		leading := len(raw) - len(trimmed)
		if startsWithQueryKeyword(trimmed, "FIND") {
			out = append(out, semaQueryLiteral{text: strings.TrimSpace(raw), queryOffset: i + 1 + leading})
		}
		i = end
	}
	return out
}

type semaCodeSpans []bool

func newSemaCodeSpans(source string) semaCodeSpans {
	spans := make(semaCodeSpans, len(source))
	for i := range spans {
		spans[i] = true
	}
	for i := 0; i < len(source); i++ {
		if source[i] == '/' && i+1 < len(source) {
			switch source[i+1] {
			case '/':
				spans[i], spans[i+1] = false, false
				i += 2
				for i < len(source) && source[i] != '\n' {
					spans[i] = false
					i++
				}
				i--
				continue
			case '*':
				spans[i], spans[i+1] = false, false
				i += 2
				for i < len(source) {
					spans[i] = false
					if source[i] == '*' && i+1 < len(source) && source[i+1] == '/' {
						spans[i+1] = false
						i++
						break
					}
					i++
				}
				continue
			}
		}
		if source[i] == '\'' || source[i] == '"' {
			quote := source[i]
			spans[i] = false
			i++
			for i < len(source) {
				spans[i] = false
				if source[i] == '\\' {
					i++
					if i < len(source) {
						spans[i] = false
					}
					i++
					continue
				}
				if source[i] == quote {
					break
				}
				i++
			}
		}
	}
	return spans
}

func (s semaCodeSpans) contains(offset int) bool {
	return offset >= 0 && offset < len(s) && s[offset]
}

func semaMatchingBracket(source string, start int) int {
	quote := byte(0)
	for i := start + 1; i < len(source); i++ {
		if quote != 0 {
			if source[i] == '\\' {
				i++
				continue
			}
			if source[i] == quote {
				quote = 0
			}
			continue
		}
		switch source[i] {
		case '\'', '"':
			quote = source[i]
		case ']':
			return i
		}
	}
	return -1
}

type semaSourceLocator struct {
	source    string
	lineStart []int
}

func newSemaSourceLocator(source string) semaSourceLocator {
	starts := []int{0}
	for offset, ch := range source {
		if ch == '\n' {
			starts = append(starts, offset+1)
		}
	}
	return semaSourceLocator{source: source, lineStart: starts}
}

func (l semaSourceLocator) rangeFor(start, end int) diagnostic.Range {
	return diagnostic.Range{Start: l.position(start), End: l.position(end)}
}

func (l semaSourceLocator) position(offset int) diagnostic.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(l.source) {
		offset = len(l.source)
	}
	line := sort.Search(len(l.lineStart), func(i int) bool {
		return l.lineStart[i] > offset
	})
	if line == 0 {
		line = 1
	}
	lineStart := l.lineStart[line-1]
	column := 1
	for range l.source[lineStart:offset] {
		column++
	}
	return diagnostic.Position{Line: line, Column: column, Offset: offset}
}

func findQueryIdentifier(source, ident string, start int) int {
	if start < 0 {
		start = 0
	}
	for i := start; i+len(ident) <= len(source); i++ {
		if i > 0 && isQueryIdentPart(source[i-1]) {
			continue
		}
		if source[i:i+len(ident)] != ident {
			continue
		}
		if i+len(ident) < len(source) && isQueryIdentPart(source[i+len(ident)]) {
			continue
		}
		return i
	}
	return strings.Index(source, ident)
}

func findSOQLFromObject(source, objectName string, start int) int {
	for i := start; i+len("FROM") <= len(source); i++ {
		if !wordAtFold(source, i, "FROM") {
			continue
		}
		offset := findQueryIdentifier(source, objectName, i+len("FROM"))
		if offset >= 0 {
			return offset
		}
	}
	return findQueryIdentifier(source, objectName, start)
}

func findChildQueryCursor(source, objectName string, start int) int {
	offset := findSOQLFromObject(source, objectName, start)
	if offset < 0 {
		return start
	}
	for i := offset; i >= 0; i-- {
		if source[i] == '(' {
			return i
		}
	}
	return offset
}

func findTypeofWhenObject(source, objectName string, start int) int {
	for i := start; i+len("WHEN") <= len(source); i++ {
		if !wordAtFold(source, i, "WHEN") {
			continue
		}
		offset := findQueryIdentifier(source, objectName, i+len("WHEN"))
		if offset >= 0 {
			return offset
		}
	}
	return findQueryIdentifier(source, objectName, start)
}

func keywordIndexFold(source, keyword string) int {
	for i := 0; i+len(keyword) <= len(source); i++ {
		if wordAtFold(source, i, keyword) {
			return i
		}
	}
	return -1
}

func startsWithQueryKeyword(source, keyword string) bool {
	if len(source) < len(keyword) || !strings.EqualFold(source[:len(keyword)], keyword) {
		return false
	}
	return len(source) == len(keyword) || !isQueryIdentPart(source[len(keyword)])
}

func wordAtFold(source string, offset int, word string) bool {
	if offset < 0 || offset+len(word) > len(source) || !strings.EqualFold(source[offset:offset+len(word)], word) {
		return false
	}
	if offset > 0 && isQueryIdentPart(source[offset-1]) {
		return false
	}
	if offset+len(word) < len(source) && isQueryIdentPart(source[offset+len(word)]) {
		return false
	}
	return true
}

func isQueryIdentPart(ch byte) bool {
	return ch == '_' || ch == '$' || ch == '.' || ('0' <= ch && ch <= '9') || ('A' <= ch && ch <= 'Z') || ('a' <= ch && ch <= 'z')
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
		diagnostics = append(diagnostics, checkDatabaseBatchableGenericContract(model, typ)...)
	}
	return diagnostics
}

func checkDatabaseBatchableGenericContract(model map[string]typeMembers, typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	members, _, ok := semaLookupTypeMembers(model, typ.Name)
	if !ok {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	for _, iface := range members.interfaces {
		base, args := semaGenericBaseAndArgs(iface)
		if !strings.EqualFold(base, "Database.Batchable") && !strings.EqualFold(base, "Batchable") {
			continue
		}
		itemType := "Object"
		if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
			itemType = strings.TrimSpace(args[0])
		}
		for _, methodName := range []string{"start", "execute", "finish"} {
			methods := concreteMethodsByName(model, typ.Name, methodName)
			if len(methods) == 0 {
				continue
			}
			if databaseBatchableMethodCompatible(methodName, itemType, methods, model) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA017",
				Message:  fmt.Sprintf("concrete class %q must implement Database.Batchable<%s> method %q with the matching signature", typ.Name, itemType, methodName),
				File:     typ.File,
				Range:    &typ.Range,
			})
		}
	}
	return diagnostics
}

func concreteMethodsByName(model map[string]typeMembers, typeName, methodName string) []typesys.MemberSymbol {
	var out []typesys.MemberSymbol
	for current := typeName; current != ""; {
		members, ok := model[normalizeName(current)]
		if !ok {
			return out
		}
		for _, method := range members.methods[normalizeName(methodName)] {
			if !hasModifier(method.Modifiers, "abstract") {
				out = append(out, method)
			}
		}
		current = members.superClass
	}
	return out
}

func databaseBatchableMethodCompatible(methodName, itemType string, methods []typesys.MemberSymbol, model map[string]typeMembers) bool {
	for _, method := range methods {
		switch strings.ToLower(methodName) {
		case "start":
			if len(method.Parameters) != 1 || !sameSemaSignatureType(method.Parameters[0].Type, "Database.BatchableContext") {
				continue
			}
			if strings.EqualFold(method.Type, "Database.QueryLocator") || databaseBatchableStartReturnCompatible(itemType, method.Type) {
				return true
			}
		case "execute":
			if len(method.Parameters) != 2 ||
				!sameSemaSignatureType(method.Parameters[0].Type, "Database.BatchableContext") ||
				!databaseBatchableExecuteScopeCompatible(itemType, method.Parameters[1].Type, model) {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(method.Type), "void") || strings.TrimSpace(method.Type) == "" {
				return true
			}
		case "finish":
			if len(method.Parameters) != 1 || !sameSemaSignatureType(method.Parameters[0].Type, "Database.BatchableContext") {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(method.Type), "void") || strings.TrimSpace(method.Type) == "" {
				return true
			}
		}
	}
	return false
}

func databaseBatchableExecuteScopeCompatible(itemType, scopeType string, model map[string]typeMembers) bool {
	required := "List<" + itemType + ">"
	if sameSemaSignatureType(scopeType, required) {
		return true
	}
	scopeBase, scopeArgs := semaGenericBaseAndArgs(scopeType)
	if !strings.EqualFold(scopeBase, "List") || len(scopeArgs) != 1 {
		return false
	}
	return semaAssignableToType(itemType, scopeArgs[0], model)
}

func databaseBatchableStartReturnCompatible(itemType, returnType string) bool {
	base, args := semaGenericBaseAndArgs(returnType)
	if (!strings.EqualFold(base, "Iterable") && !strings.EqualFold(base, "List")) || len(args) != 1 {
		return false
	}
	return sameSemaSignatureType(args[0], itemType)
}

func (a *Analyzer) checkBodyAssignments(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, match := range assignmentPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaOffsetInIgnoredText(body, match[0]) {
			continue
		}
		if semaAssignmentLooksLikeComparison(body, match[1]) {
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
