package sema

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

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
		if trigger.Dependency {
			continue
		}
		diagnostics = append(diagnostics, triggerEventDiagnostics(trigger)...)
		if trigger.ObjectName == "" {
			continue
		}
		if !a.hasKnown(trigger.ObjectName) {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA001",
				Message:  fmt.Sprintf("trigger %q references unknown SObject %q", trigger.Name, trigger.ObjectName),
				File:     trigger.File,
				Range:    &trigger.Range,
			})
			continue
		}
		if supported, known := triggerObjectSupportsTriggers(index, trigger.ObjectName); known && !supported {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA029",
				Message:  fmt.Sprintf("trigger %q targets SObject %q, which does not support triggers", trigger.Name, trigger.ObjectName),
				File:     trigger.File,
				Range:    &trigger.Range,
			})
		}
	}
	return diagnostics
}

func triggerEventDiagnostics(trigger typesys.TriggerSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	seen := make(map[string]bool, len(trigger.Events))
	for _, event := range trigger.Events {
		canonical := normalizeTriggerEvent(event)
		if canonical == "" {
			continue
		}
		if !seen[canonical] {
			seen[canonical] = true
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA030",
			Message:  fmt.Sprintf("trigger %q declares duplicate event %q", trigger.Name, event),
			File:     trigger.File,
			Range:    &trigger.Range,
		})
	}
	return diagnostics
}

func normalizeTriggerEvent(event string) string {
	var canonical strings.Builder
	for _, r := range strings.ToLower(event) {
		if unicode.IsSpace(r) {
			continue
		}
		canonical.WriteRune(r)
	}
	return canonical.String()
}

// triggerObjectSupportsTriggers reports describe-provided trigger capability.
// Project metadata never states the flag, so an object without describe
// evidence stays allowed.
func triggerObjectSupportsTriggers(index typesys.Index, objectName string) (supported, known bool) {
	for _, object := range index.Objects {
		if !strings.EqualFold(object.Name, objectName) {
			continue
		}
		if supported, known := object.SupportsTriggers(); known {
			return supported, true
		}
		break
	}
	return storage.StandardObjectTriggerable(objectName)
}
func (a *Analyzer) checkMemberTypes(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
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
		if skipProjectDiagnosticType(typ) {
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
	checker := newQuerySemanticsChecker(index, a.queryDeclaredObjects...)
	if len(checker.objects) == 0 {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	sources := a.sources
	if sources == nil {
		sources = newSemaSources(nil, nil)
	}
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) || typ.File == "" {
			continue
		}
		source, ok := sources.normalizedForType(typ)
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, checker.checkFile(typ.File, source)...)
	}
	return diagnostics
}

type querySemanticsChecker struct {
	namespace      string
	objects        map[string]schema.Object
	providers      map[string]semaSObjectFieldProvider
	declaredFields map[string]int
	hasBaseline    bool
}

func newQuerySemanticsChecker(index typesys.Index, declaredObjects ...schema.Object) querySemanticsChecker {
	checker := querySemanticsChecker{
		namespace:      index.Project.Namespace,
		objects:        make(map[string]schema.Object, len(index.Objects)),
		providers:      make(map[string]semaSObjectFieldProvider, len(index.Objects)),
		declaredFields: make(map[string]int, len(index.Objects)),
		hasBaseline:    len(declaredObjects) > 0,
	}
	for _, object := range declaredObjects {
		checker.recordDeclaredFields(object)
	}
	for _, object := range index.Objects {
		checker.addObject(object)
	}
	checker.addActivityFieldsToTaskAndEvent()
	return checker
}

func (c querySemanticsChecker) checkFile(file, source string) []diagnostic.Diagnostic {
	locator := newSemaSourceLocator(source)
	spans := newSemaCodeSpans(source)
	bindingResolver := newSemaBindingResolver(source, spans)
	var diagnostics []diagnostic.Diagnostic
	for _, literal := range semaSOQLLiterals(source, spans) {
		query, err := soql.Parse(literal.text)
		if err != nil {
			ctx := queryTextContext{file: file, queryText: literal.text, queryOffset: literal.queryOffset, locator: locator}
			diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_PARSE", fmt.Sprintf("invalid SOQL query: %v", err), literal.text, 0))
			continue
		}
		ctx := queryTextContext{
			file:        file,
			queryText:   literal.text,
			queryOffset: literal.queryOffset,
			locator:     locator,
		}
		diagnostics = append(diagnostics, c.checkSOQLQuery(query, query.Object, ctx, 0, nil)...)
		bindings := bindingResolver.bindingsAt(literal.queryOffset)
		diagnostics = append(diagnostics, inlineQueryBindDiagnostics(ctx, bindings)...)
		diagnostics = append(diagnostics, queryWindowBindDiagnostics(query, ctx, bindings)...)
	}
	for _, literal := range semaSOSLLiterals(source, spans) {
		query, err := sosl.Parse(literal.text)
		if err != nil {
			ctx := queryTextContext{file: file, queryText: literal.text, queryOffset: literal.queryOffset, locator: locator}
			diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_SOSL_PARSE", fmt.Sprintf("invalid SOSL query: %v", err), literal.text, 0))
			continue
		}
		ctx := queryTextContext{
			file:        file,
			queryText:   literal.text,
			queryOffset: literal.queryOffset,
			locator:     locator,
		}
		diagnostics = append(diagnostics, c.checkSOSLQuery(query, ctx)...)
		bindings := bindingResolver.bindingsAt(literal.queryOffset)
		diagnostics = append(diagnostics, inlineQueryBindDiagnostics(ctx, bindings)...)
	}
	return diagnostics
}

var (
	semaInlineBindPattern  = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)
	semaBindingDeclaration = regexp.MustCompile(`(?m)(?:^|[;({,])\s*(?:(?:public|private|protected|global|static|final|transient)\s+)*([A-Za-z_][A-Za-z0-9_]*(?:\s*<[^;=(){}]+>)?(?:\[\])?)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
)

type semaBindingResolver struct {
	source    string
	spans     semaCodeSpans
	locations []semaMethodLocation
	methods   map[int]semaBindingScope
}

type semaMethodLocation struct {
	methodStart int
	methodEnd   int
	headerStart int
	typeStart   int
	typeEnd     int
}

type semaBindingScope struct {
	methodStart int
	typeStart   int
	bindings    []semaScopedBinding
}

type semaScopedBinding struct {
	name     string
	typeName string
	start    int
	end      int
	field    bool
}

func newSemaBindingResolver(source string, spans semaCodeSpans) *semaBindingResolver {
	return &semaBindingResolver{source: source, spans: spans, locations: semaMethodLocations(source, spans), methods: make(map[int]semaBindingScope)}
}

// bindingsAt returns source-backed parameter, field, and local declarations
// visible at a query literal. Per-method declarations are cached because large
// classes commonly contain many literals in the same method.
func (r *semaBindingResolver) bindingsAt(offset int) map[string]string {
	bindings := make(map[string]string)
	location, ok := r.methodAt(offset)
	if !ok {
		return bindings
	}
	scope, ok := r.methods[location.methodStart]
	if !ok {
		scope = semaBuildBindingScope(r.source, location.methodStart, location.methodEnd, location.headerStart, location.typeStart, location.typeEnd, r.spans)
		r.methods[location.methodStart] = scope
	}
	for _, binding := range scope.bindings {
		if !binding.field && (binding.start >= offset || offset > binding.end) {
			continue
		}
		bindings[strings.ToLower(binding.name)] = binding.typeName
	}
	return bindings
}

func (r *semaBindingResolver) methodAt(offset int) (semaMethodLocation, bool) {
	var found semaMethodLocation
	for _, location := range r.locations {
		if offset <= location.methodStart || offset >= location.methodEnd || (found.methodStart != 0 && location.methodStart <= found.methodStart) {
			continue
		}
		found = location
	}
	return found, found.methodStart != 0
}

func semaMethodLocations(source string, spans semaCodeSpans) []semaMethodLocation {
	var locations []semaMethodLocation
	var braces []int
	for i := 0; i < len(source); i++ {
		if !spans.contains(i) {
			continue
		}
		switch source[i] {
		case '{':
			headerStart := semaHeaderStart(source, i, spans)
			if semaMethodHeader(source[headerStart:i]) {
				if end := semaMatchingCodeBrace(source, i, spans); end >= 0 {
					typeStart, typeEnd := -1, -1
					if len(braces) > 0 {
						typeStart = braces[len(braces)-1]
						typeEnd = semaMatchingCodeBrace(source, typeStart, spans)
					}
					locations = append(locations, semaMethodLocation{methodStart: i, methodEnd: end, headerStart: headerStart, typeStart: typeStart, typeEnd: typeEnd})
				}
			}
			braces = append(braces, i)
		case '}':
			if len(braces) > 0 {
				braces = braces[:len(braces)-1]
			}
		}
	}
	return locations
}

func semaBuildBindingScope(source string, methodStart, methodEnd, headerStart, typeStart, typeEnd int, spans semaCodeSpans) semaBindingScope {
	scope := semaBindingScope{methodStart: methodStart, typeStart: typeStart}
	for _, match := range semaBindingDeclaration.FindAllStringSubmatchIndex(source[headerStart:methodStart], -1) {
		if len(match) != 6 {
			continue
		}
		scope.bindings = append(scope.bindings, semaScopedBinding{name: source[headerStart+match[4] : headerStart+match[5]], typeName: strings.TrimSpace(source[headerStart+match[2] : headerStart+match[3]]), start: headerStart + match[0], end: methodEnd})
	}
	for _, match := range semaBindingDeclaration.FindAllStringSubmatchIndex(source[methodStart+1:methodEnd], -1) {
		if len(match) != 6 {
			continue
		}
		start := methodStart + 1 + match[0]
		scope.bindings = append(scope.bindings, semaScopedBinding{name: source[methodStart+1+match[4] : methodStart+1+match[5]], typeName: strings.TrimSpace(source[methodStart+1+match[2] : methodStart+1+match[3]]), start: start, end: semaEnclosingCodeBraceEnd(source, methodStart+1, start, spans)})
	}
	if typeStart >= 0 && typeEnd > typeStart {
		for _, match := range semaBindingDeclaration.FindAllStringSubmatchIndex(source[typeStart+1:typeEnd], -1) {
			if len(match) != 6 || !semaDeclarationAtTypeScope(source, typeStart+1, typeStart+1+match[0], spans) {
				continue
			}
			scope.bindings = append(scope.bindings, semaScopedBinding{name: source[typeStart+1+match[4] : typeStart+1+match[5]], typeName: strings.TrimSpace(source[typeStart+1+match[2] : typeStart+1+match[3]]), field: true})
		}
	}
	return scope
}

func semaMethodHeader(header string) bool {
	header = strings.TrimSpace(header)
	open := strings.LastIndex(header, "(")
	if open < 0 || !strings.HasSuffix(header, ")") {
		return false
	}
	nameFields := strings.Fields(header[:open])
	if len(nameFields) == 0 {
		return false
	}
	switch strings.ToLower(nameFields[len(nameFields)-1]) {
	case "if", "for", "while", "switch", "catch":
		return false
	}
	return true
}

func semaHeaderStart(source string, brace int, spans semaCodeSpans) int {
	for i := brace - 1; i >= 0; i-- {
		if !spans.contains(i) {
			continue
		}
		switch source[i] {
		case '{', '}', ';':
			return i + 1
		}
	}
	return 0
}

func semaOpenBraces(source string, start, offset int, spans semaCodeSpans) []int {
	var braces []int
	for i := start; i < offset && i < len(source); i++ {
		if !spans.contains(i) {
			continue
		}
		switch source[i] {
		case '{':
			braces = append(braces, i)
		case '}':
			if len(braces) > 0 {
				braces = braces[:len(braces)-1]
			}
		}
	}
	return braces
}

func semaMatchingCodeBrace(source string, start int, spans semaCodeSpans) int {
	depth := 0
	for i := start; i < len(source); i++ {
		if !spans.contains(i) {
			continue
		}
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func semaEnclosingCodeBraceEnd(source string, start, offset int, spans semaCodeSpans) int {
	braces := semaOpenBraces(source, start, offset, spans)
	if len(braces) == 0 {
		return len(source)
	}
	if end := semaMatchingCodeBrace(source, braces[len(braces)-1], spans); end >= 0 {
		return end
	}
	return len(source)
}

func semaDeclarationAtTypeScope(source string, start, declaration int, spans semaCodeSpans) bool {
	return len(semaOpenBraces(source, start, declaration, spans)) == 0
}

func inlineQueryBindDiagnostics(ctx queryTextContext, bindings map[string]string) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, match := range semaInlineBindPattern.FindAllStringSubmatchIndex(ctx.queryText, -1) {
		if len(match) != 4 {
			continue
		}
		name := ctx.queryText[match[2]:match[3]]
		if _, ok := bindings[strings.ToLower(name)]; ok {
			continue
		}
		offset := match[0]
		diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_BIND", fmt.Sprintf("query bind variable %q is not declared", name), ctx.queryText[match[0]:match[1]], offset))
	}
	return diagnostics
}

func queryWindowBindDiagnostics(query soql.Query, ctx queryTextContext, bindings map[string]string) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, bind := range []struct {
		name   string
		clause string
	}{{query.LimitBind, "LIMIT"}, {query.OffsetBind, "OFFSET"}} {
		if bind.name == "" {
			continue
		}
		typeName := strings.ToLower(strings.TrimSpace(bindings[strings.ToLower(bind.name)]))
		if typeName == "integer" || typeName == "int" || typeName == "long" || typeName == "decimal" || typeName == "double" {
			continue
		}
		offset := findQueryIdentifier(ctx.queryText, ":"+bind.name, 0)
		diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_BIND", fmt.Sprintf("%s bind variable %q must have a numeric type", bind.clause, bind.name), ":"+bind.name, offset))
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
	diagnostics = append(diagnostics, queryShapeDiagnostics(query, ctx)...)
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
		diagnostics = append(diagnostics, c.checkSOQLFieldCapability(object.Name, aggregate.Field, "aggregatable", ctx, cursor)...)
	}
	for _, field := range query.Fields {
		diagnostics = append(diagnostics, c.checkSOQLField(object.Name, field, ctx, cursor)...)
	}
	for _, field := range query.GroupBy {
		if aggregateAliases[strings.ToLower(field)] {
			continue
		}
		diagnostics = append(diagnostics, c.checkSOQLField(object.Name, field, ctx, cursor)...)
		diagnostics = append(diagnostics, c.checkSOQLFieldCapability(object.Name, field, "groupable", ctx, cursor)...)
	}
	for _, order := range query.Order {
		if aggregateAliases[strings.ToLower(order.Field)] {
			continue
		}
		diagnostics = append(diagnostics, c.checkSOQLField(object.Name, order.Field, ctx, cursor)...)
		diagnostics = append(diagnostics, c.checkSOQLFieldCapability(object.Name, order.Field, "sortable", ctx, cursor)...)
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

func queryShapeDiagnostics(query soql.Query, ctx queryTextContext) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	diagnosticFor := func(message, token string) {
		diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_CONTRACT", message, token, findQueryIdentifier(ctx.queryText, token, 0)))
	}
	if len(query.Aggregates) > 0 && query.HasLimit && len(query.GroupBy) == 0 {
		diagnosticFor("aggregate SOQL queries require GROUP BY before LIMIT", "LIMIT")
	}
	if strings.EqualFold(query.GroupMode, "ROLLUP") && len(query.GroupBy) > 2 {
		diagnosticFor("ROLLUP supports at most two grouping fields", "ROLLUP")
	}
	if len(query.Typeofs) > 0 && len(query.GroupBy) > 0 {
		diagnosticFor("TYPEOF cannot be combined with GROUP BY", "TYPEOF")
	}
	if query.Where != nil && containsSelfSemiJoin(*query.Where, query.Object) {
		diagnosticFor("SOQL semi-join subqueries cannot reference the same SObject as the outer query", "SELECT")
	}
	if query.ForUpdate && len(query.Order) > 0 {
		diagnosticFor("FOR UPDATE cannot be combined with ORDER BY", "FOR UPDATE")
	}
	for _, field := range query.Fields {
		if strings.EqualFold(strings.TrimSpace(field), "FIELDS(ALL)") && !query.HasLimit {
			diagnosticFor("FIELDS(ALL) requires LIMIT in Apex", "FIELDS(ALL)")
		}
	}
	return diagnostics
}

func containsSelfSemiJoin(condition soql.Condition, outerObject string) bool {
	if condition.Subquery != nil && strings.EqualFold(condition.Subquery.Object, outerObject) {
		return true
	}
	for _, child := range condition.And {
		if containsSelfSemiJoin(child, outerObject) {
			return true
		}
	}
	for _, child := range condition.Or {
		if containsSelfSemiJoin(child, outerObject) {
			return true
		}
	}
	return false
}

func (c querySemanticsChecker) checkSOQLCondition(objectName string, condition soql.Condition, ctx queryTextContext, cursor int, aggregateAliases map[string]bool) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if condition.Field != "" && !aggregateAliases[strings.ToLower(condition.Field)] {
		diagnostics = append(diagnostics, c.checkSOQLField(objectName, condition.Field, ctx, cursor)...)
		diagnostics = append(diagnostics, c.checkSOQLFieldCapability(objectName, condition.Field, "filterable", ctx, cursor)...)
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

func (c querySemanticsChecker) checkSOQLFieldCapability(objectName, fieldPath, capability string, ctx queryTextContext, cursor int) []diagnostic.Diagnostic {
	fieldPath = semaSOQLFieldReference(fieldPath)
	if fieldPath == "" || strings.Contains(fieldPath, ".") || strings.Contains(fieldPath, "(") || !c.hasFieldMetadata(objectName) {
		return nil
	}
	field, ok := c.field(objectName, fieldPath)
	if !ok {
		return nil
	}
	var supported *bool
	switch capability {
	case "filterable":
		supported = field.Filterable
	case "groupable":
		supported = field.Groupable
	case "sortable":
		supported = field.Sortable
	case "aggregatable":
		supported = field.Aggregatable
	}
	if supported == nil || *supported {
		return nil
	}
	offset := findQueryIdentifier(ctx.queryText, fieldPath, cursor)
	return []diagnostic.Diagnostic{ctx.diagnostic("GLADESEMA_QUERY_CAPABILITY", fmt.Sprintf("SOQL field %s.%s is not %s according to describe metadata", objectName, fieldPath, capability), fieldPath, offset)}
}

func (c querySemanticsChecker) checkSOQLField(objectName, fieldPath string, ctx queryTextContext, cursor int) []diagnostic.Diagnostic {
	fieldPath = semaSOQLFieldReference(fieldPath)
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
		if c.allowsIncompleteExternalManagedPackageField(objectName, fieldPath) {
			return nil
		}
		if c.allowsIncompleteExternalManagedPackageObject(objectName) {
			return nil
		}
		return []diagnostic.Diagnostic{ctx.diagnostic("GLADESEMA_QUERY_FIELD", fmt.Sprintf("SOQL query references unknown field %s.%s", objectName, fieldPath), fieldPath, offset)}
	}
	parts := strings.Split(fieldPath, ".")
	current := objectName
	if len(parts) > 1 && c.apiNamesMatch(parts[0], objectName) {
		parts = parts[1:]
		if len(parts) == 1 {
			if _, ok := c.field(current, parts[0]); ok {
				return nil
			}
			if c.allowsIncompleteExternalManagedPackageField(current, parts[0]) {
				return nil
			}
			if c.allowsIncompleteExternalManagedPackageObject(current) {
				return nil
			}
			return []diagnostic.Diagnostic{ctx.diagnostic("GLADESEMA_QUERY_FIELD", fmt.Sprintf("SOQL query references unknown field %s.%s via %q", current, parts[0], fieldPath), fieldPath, offset)}
		}
	}
	for _, relationship := range parts[:len(parts)-1] {
		if !c.hasFieldMetadata(current) {
			return nil
		}
		_, target, ok := c.relationshipField(current, relationship)
		if !ok {
			if c.allowsIncompleteExternalManagedPackageObject(current) {
				return nil
			}
			return []diagnostic.Diagnostic{ctx.diagnostic("GLADESEMA_QUERY_RELATIONSHIP", fmt.Sprintf("SOQL query references unknown relationship path %q on %s", fieldPath, current), fieldPath, offset)}
		}
		current = target
	}
	if _, ok := c.field(current, parts[len(parts)-1]); !ok {
		if c.allowsIncompleteExternalManagedPackageField(current, parts[len(parts)-1]) {
			return nil
		}
		if c.allowsIncompleteExternalManagedPackageObject(current) {
			return nil
		}
		return []diagnostic.Diagnostic{ctx.diagnostic("GLADESEMA_QUERY_FIELD", fmt.Sprintf("SOQL query references unknown field %s.%s via %q", current, parts[len(parts)-1], fieldPath), fieldPath, offset)}
	}
	return nil
}

func semaSOQLFieldReference(fieldPath string) string {
	fieldPath = strings.TrimSpace(fieldPath)
	if fieldPath == "" || strings.Contains(fieldPath, "(") {
		return fieldPath
	}
	parts := strings.Fields(fieldPath)
	switch {
	case len(parts) >= 3 && strings.EqualFold(parts[len(parts)-2], "AS"):
		return strings.Join(parts[:len(parts)-2], " ")
	case len(parts) >= 2:
		return parts[0]
	default:
		return fieldPath
	}
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
			if strings.Contains(field, ".") && len(c.checkSOQLField(object.Name, field, ctx, cursor)) == 0 {
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
	key := normalizeName(name)
	if object, ok := c.objects[key]; ok {
		return object, true
	}
	if namespaced, ok := semaProjectNamespacedAPIName(c.namespace, name); ok {
		if object, ok := c.objects[normalizeName(namespaced)]; ok {
			return object, true
		}
	}
	if local := semaSchemaLocalAPIName(name); !strings.EqualFold(local, name) {
		if object, ok := c.objects[normalizeName(local)]; ok {
			return object, true
		}
	}
	if strings.EqualFold(name, "Name") {
		object := schema.Object{Name: "Name", Fields: []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
			{Name: "Type", Type: "Text"},
		}}
		c.addObject(object)
		return object, true
	}
	canonical, ok := storage.ResolveKnownStandardObjectName(name)
	if !ok {
		return schema.Object{}, false
	}
	object := schema.Object{Name: canonical}
	c.addObject(object)
	return object, true
}

func (c querySemanticsChecker) field(objectName, fieldName string) (schema.Field, bool) {
	object, ok := c.object(objectName)
	if !ok {
		return schema.Field{}, false
	}
	provider, ok := c.providers[normalizeName(object.Name)]
	if !ok || provider == nil {
		return schema.Field{}, false
	}
	if field, ok := provider.lookup(fieldName); ok {
		return field, true
	}
	if field, ok := c.locationComponentField(object.Name, fieldName); ok {
		return field, true
	}
	return schema.Field{}, false
}

func (c querySemanticsChecker) locationComponentField(objectName, fieldName string) (schema.Field, bool) {
	component := ""
	baseName := ""
	switch {
	case strings.HasSuffix(fieldName, "__Latitude__s"):
		component = "Latitude"
		baseName = strings.TrimSuffix(fieldName, "__Latitude__s") + "__c"
	case strings.HasSuffix(fieldName, "__Longitude__s"):
		component = "Longitude"
		baseName = strings.TrimSuffix(fieldName, "__Longitude__s") + "__c"
	default:
		return schema.Field{}, false
	}
	baseField, ok := c.field(objectName, baseName)
	if !ok || !strings.EqualFold(baseField.Type, "Location") {
		return schema.Field{}, false
	}
	return schema.Field{Name: fieldName, Label: baseField.Label + " " + component, Type: "Number"}, true
}

func (c querySemanticsChecker) hasFieldMetadata(objectName string) bool {
	object, ok := c.object(objectName)
	if !ok {
		return false
	}
	if object.Partial {
		return false
	}
	provider, ok := c.providers[normalizeName(object.Name)]
	return ok && provider != nil && provider.hasFields()
}

func (c querySemanticsChecker) hasDeclaredFieldMetadata(objectName string) bool {
	object, ok := c.object(objectName)
	if !ok {
		return false
	}
	return c.declaredFields[normalizeName(object.Name)] > 0
}

func (c querySemanticsChecker) allowsIncompleteExternalManagedPackageObject(objectName string) bool {
	object, ok := c.object(objectName)
	if !ok {
		return false
	}
	namespace := semaNamespaceFromAPIName(object.Name)
	if namespace == "" {
		return false
	}
	return c.namespace == "" || !strings.EqualFold(namespace, c.namespace)
}

func (c querySemanticsChecker) allowsIncompleteExternalManagedPackageField(objectName, fieldName string) bool {
	fieldNamespace := semaNamespaceFromAPIName(fieldName)
	if fieldNamespace == "" || strings.EqualFold(fieldNamespace, c.namespace) {
		return false
	}
	object, ok := c.object(objectName)
	if !ok {
		return false
	}
	if objectNamespace := semaNamespaceFromAPIName(object.Name); objectNamespace != "" {
		return !strings.EqualFold(objectNamespace, c.namespace)
	}
	_, ok = storage.StandardObjectDefinition(object.Name)
	return ok
}

func (c querySemanticsChecker) relationshipField(objectName, relationshipName string) (schema.Field, string, bool) {
	object, ok := c.object(objectName)
	if !ok {
		return schema.Field{}, "", false
	}
	provider, ok := c.providers[normalizeName(object.Name)]
	if !ok || provider == nil {
		return c.inferredPackageParentRelationshipField(relationshipName)
	}
	var matched schema.Field
	found := false
	provider.visit(func(field schema.Field) {
		if found {
			return
		}
		if c.apiNamesMatch(field.RelationshipName, relationshipName) && len(field.ReferenceTo) > 0 {
			matched = field
			found = true
		}
	})
	if found {
		target := matched.ReferenceTo[0]
		targetObject, hasTargetObject := c.object(target)
		canonicalTarget, hasCanonicalTarget := c.standardTargetForPackageRelationship(matched.RelationshipName, target)
		if !hasCanonicalTarget {
			canonicalTarget, hasCanonicalTarget = c.standardTargetForPackageRelationship(relationshipName, target)
		}
		if hasTargetObject && (!hasCanonicalTarget || c.hasDeclaredFieldMetadata(targetObject.Name)) {
			return matched, targetObject.Name, true
		}
		if hasCanonicalTarget {
			target = canonicalTarget
		}
		if targetObject, ok := c.object(target); ok {
			target = targetObject.Name
		}
		return matched, target, true
	}
	return c.inferredPackageParentRelationshipField(relationshipName)
}

func (c querySemanticsChecker) standardTargetForPackageRelationship(relationshipName, target string) (string, bool) {
	relationshipName = strings.TrimSpace(relationshipName)
	if !strings.HasSuffix(strings.ToLower(relationshipName), "__r") {
		return "", false
	}
	hasNamespace := semaHasNamespaceToken(relationshipName)
	if !hasNamespace && c.namespace == "" {
		return "", false
	}
	candidates := []string{}
	if local, ok := semaProjectLocalAPIName(c.namespace, relationshipName); ok {
		candidates = append(candidates, strings.TrimSuffix(local, "__r"))
	}
	if local := semaSchemaLocalAPIName(relationshipName); !strings.EqualFold(local, relationshipName) {
		candidates = append(candidates, strings.TrimSuffix(local, "__r"))
	}
	if !hasNamespace {
		candidates = append(candidates, strings.TrimSuffix(relationshipName, "__r"))
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		targetLocal := semaSchemaLocalAPIName(target)
		if !strings.EqualFold(targetLocal, candidate+"__c") {
			continue
		}
		if _, ok := storage.StandardObjectDefinition(candidate); ok {
			return candidate, true
		}
	}
	return "", false
}

func (c querySemanticsChecker) inferredPackageParentRelationshipField(relationshipName string) (schema.Field, string, bool) {
	relationshipName = strings.TrimSpace(relationshipName)
	if !strings.HasSuffix(strings.ToLower(relationshipName), "__r") {
		return schema.Field{}, "", false
	}
	candidates := []string{}
	if local, ok := semaProjectLocalAPIName(c.namespace, relationshipName); ok {
		candidates = append(candidates, strings.TrimSuffix(local, "__r"))
		candidates = append(candidates, strings.TrimSuffix(local, "__r")+"__c")
	}
	if local := semaSchemaLocalAPIName(relationshipName); !strings.EqualFold(local, relationshipName) {
		candidates = append(candidates, strings.TrimSuffix(local, "__r"))
		candidates = append(candidates, strings.TrimSuffix(local, "__r")+"__c")
	}
	if c.namespace != "" && !semaHasNamespaceToken(relationshipName) {
		candidates = append(candidates, strings.TrimSuffix(relationshipName, "__r"))
	}
	candidates = append(candidates, strings.TrimSuffix(relationshipName, "__r")+"__c")
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		key := normalizeName(candidate)
		if candidate == "" || seen[key] {
			continue
		}
		seen[key] = true
		if object, ok := c.object(candidate); ok {
			return schema.Field{Name: strings.TrimSuffix(relationshipName, "__r") + "__c", Type: "Lookup", RelationshipName: relationshipName, ReferenceTo: []string{object.Name}}, object.Name, true
		}
	}
	return schema.Field{}, "", false
}

func (c querySemanticsChecker) childObjectForRelationship(parentObject, relationship string) (schema.Object, bool) {
	for _, object := range c.objects {
		provider := c.providers[normalizeName(object.Name)]
		if provider == nil {
			continue
		}
		matched := false
		provider.visitDeclared(func(field schema.Field) {
			if matched {
				return
			}
			if !c.apiNamesMatch(field.ChildRelationshipName, relationship) {
				return
			}
			for _, target := range field.ReferenceTo {
				if c.apiNamesMatch(target, parentObject) {
					matched = true
					return
				}
			}
		})
		if matched {
			return object, true
		}
	}
	for _, relationshipMember := range semaStandardChildRelationshipMembers(parentObject) {
		if !c.apiNamesMatch(relationshipMember.name, relationship) {
			continue
		}
		base, args := semaGenericBaseAndArgs(relationshipMember.typ)
		if !strings.EqualFold(base, "List") || len(args) != 1 {
			continue
		}
		if object, ok := c.object(args[0]); ok {
			return object, true
		}
	}
	if object, ok := c.inferredPackageChildRelationshipObject(relationship); ok {
		return object, true
	}
	return schema.Object{}, false
}

func (c querySemanticsChecker) inferredPackageChildRelationshipObject(relationship string) (schema.Object, bool) {
	relationship = strings.TrimSpace(relationship)
	if !strings.HasSuffix(strings.ToLower(relationship), "__r") {
		return schema.Object{}, false
	}
	candidates := []string{}
	if local, ok := semaProjectLocalAPIName(c.namespace, relationship); ok {
		candidates = append(candidates, strings.TrimSuffix(local, "__r")+"__c")
	}
	if local := semaSchemaLocalAPIName(relationship); !strings.EqualFold(local, relationship) {
		candidates = append(candidates, strings.TrimSuffix(local, "__r")+"__c")
	}
	candidates = append(candidates, strings.TrimSuffix(relationship, "__r")+"__c")
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		key := normalizeName(candidate)
		if candidate == "" || seen[key] {
			continue
		}
		seen[key] = true
		if object, ok := c.object(candidate); ok {
			return object, true
		}
	}
	return schema.Object{}, false
}

func (c querySemanticsChecker) addObject(object schema.Object) {
	if strings.TrimSpace(object.Name) == "" {
		return
	}
	if !c.hasBaseline {
		c.recordDeclaredFields(object)
	}
	key := normalizeName(object.Name)
	existing, duplicate := c.objects[key]
	if duplicate {
		object = mergeQueryDuplicateObject(existing, object)
	}
	provider := newSemaSObjectFieldProvider(c.namespace, object)
	c.objects[key] = object
	c.providers[key] = provider
	if localName, ok := semaProjectLocalAPIName(c.namespace, object.Name); ok {
		c.objects[normalizeName(localName)] = object
		c.providers[normalizeName(localName)] = provider
	}
}

func (c querySemanticsChecker) recordDeclaredFields(object schema.Object) {
	if strings.TrimSpace(object.Name) == "" {
		return
	}
	declaredFieldCount := len(object.Fields)
	key := normalizeName(object.Name)
	if declaredFieldCount > c.declaredFields[key] {
		c.declaredFields[key] = declaredFieldCount
	}
	if localName, ok := semaProjectLocalAPIName(c.namespace, object.Name); ok {
		localKey := normalizeName(localName)
		if declaredFieldCount > c.declaredFields[localKey] {
			c.declaredFields[localKey] = declaredFieldCount
		}
	}
}

func (c querySemanticsChecker) addActivityFieldsToTaskAndEvent() {
	activity, ok := c.object("Activity")
	if !ok {
		return
	}
	activityProvider := c.providers[normalizeName(activity.Name)]
	if activityProvider == nil || !activityProvider.hasFields() {
		return
	}
	for _, target := range []string{"Task", "Event"} {
		object, ok := c.object(target)
		if !ok {
			object = schema.Object{Name: target}
		}
		if _, exists := c.providers[normalizeName(object.Name)]; !exists {
			c.addObject(object)
		}
		targetProvider := c.providers[normalizeName(object.Name)]
		provider := &semaLayeredSObjectFieldProvider{
			layers:         []semaSObjectFieldProvider{targetProvider, activityProvider},
			declaredLayers: 2,
		}
		c.providers[normalizeName(object.Name)] = provider
		if localName, ok := semaProjectLocalAPIName(c.namespace, object.Name); ok {
			c.providers[normalizeName(localName)] = provider
		}
	}
}

func mergeQueryDuplicateObject(existing, incoming schema.Object) schema.Object {
	merged := existing
	if merged.Name == "" {
		merged.Name = incoming.Name
	}
	if merged.Label == "" {
		merged.Label = incoming.Label
	}
	if merged.PluralLabel == "" {
		merged.PluralLabel = incoming.PluralLabel
	}
	if merged.SharingModel == "" {
		merged.SharingModel = incoming.SharingModel
	}
	if merged.CustomSettingsType == "" {
		merged.CustomSettingsType = incoming.CustomSettingsType
	}
	merged.NameField = mergeQueryNameField(merged.NameField, incoming.NameField)
	merged.Fields = mergeQueryFields(existing.Fields, incoming.Fields)
	merged.RecordTypes = mergeQueryRecordTypes(existing.RecordTypes, incoming.RecordTypes)
	merged.ValidationRules = mergeQueryValidationRules(existing.ValidationRules, incoming.ValidationRules)
	return merged
}

func mergeQueryNameField(existing, incoming schema.NameField) schema.NameField {
	if existing.Label == "" {
		existing.Label = incoming.Label
	}
	if existing.Type == "" {
		existing.Type = incoming.Type
	}
	if existing.DisplayFormat == "" {
		existing.DisplayFormat = incoming.DisplayFormat
	}
	return existing
}

func mergeQueryFields(existing, incoming []schema.Field) []schema.Field {
	out := make([]schema.Field, 0, len(existing)+len(incoming))
	seen := make(map[string]int, len(existing)+len(incoming))
	for _, field := range existing {
		key := normalizeName(field.Name)
		if key == "" {
			continue
		}
		seen[key] = len(out)
		out = append(out, field)
	}
	for _, field := range incoming {
		key := normalizeName(field.Name)
		if key == "" {
			continue
		}
		if i, ok := seen[key]; ok {
			out[i] = mergeQueryField(out[i], field)
			continue
		}
		seen[key] = len(out)
		out = append(out, field)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func mergeQueryField(existing, incoming schema.Field) schema.Field {
	if existing.Label == "" {
		existing.Label = incoming.Label
	}
	if existing.InlineHelpText == "" {
		existing.InlineHelpText = incoming.InlineHelpText
	}
	if existing.Type == "" {
		existing.Type = incoming.Type
	}
	if existing.Length == 0 {
		existing.Length = incoming.Length
	}
	if existing.Precision == 0 {
		existing.Precision = incoming.Precision
	}
	if existing.Scale == 0 {
		existing.Scale = incoming.Scale
	}
	if len(existing.ReferenceTo) == 0 {
		existing.ReferenceTo = append([]string(nil), incoming.ReferenceTo...)
	}
	if existing.RelationshipName == "" {
		existing.RelationshipName = incoming.RelationshipName
	}
	if existing.ChildRelationshipName == "" {
		existing.ChildRelationshipName = incoming.ChildRelationshipName
	}
	if existing.DeleteConstraint == "" {
		existing.DeleteConstraint = incoming.DeleteConstraint
	}
	if existing.DefaultValue == "" {
		existing.DefaultValue = incoming.DefaultValue
	}
	if existing.Formula == "" {
		existing.Formula = incoming.Formula
	}
	if existing.SummarizedField == "" {
		existing.SummarizedField = incoming.SummarizedField
	}
	if existing.SummaryForeignKey == "" {
		existing.SummaryForeignKey = incoming.SummaryForeignKey
	}
	if existing.SummaryOperation == "" {
		existing.SummaryOperation = incoming.SummaryOperation
	}
	if len(existing.SummaryFilterItems) == 0 {
		existing.SummaryFilterItems = append([]schema.SummaryFilter(nil), incoming.SummaryFilterItems...)
	}
	if len(existing.FilteredLookupInfo.ControllingFields) == 0 && !existing.FilteredLookupInfo.Dependent && !existing.FilteredLookupInfo.OptionalFilter {
		existing.FilteredLookupInfo = incoming.FilteredLookupInfo
	}
	if existing.PicklistController == "" {
		existing.PicklistController = incoming.PicklistController
	}
	if len(existing.PicklistValueSettings) == 0 {
		existing.PicklistValueSettings = append([]schema.PicklistSetting(nil), incoming.PicklistValueSettings...)
	}
	if existing.ValueSetName == "" {
		existing.ValueSetName = incoming.ValueSetName
	}
	if len(existing.PicklistValues) == 0 {
		existing.PicklistValues = append([]schema.PicklistValue(nil), incoming.PicklistValues...)
	}
	return existing
}

func mergeQueryRecordTypes(existing, incoming []schema.RecordType) []schema.RecordType {
	out := make([]schema.RecordType, 0, len(existing)+len(incoming))
	seen := make(map[string]bool, len(existing)+len(incoming))
	for _, recordType := range existing {
		key := normalizeName(recordType.DeveloperName)
		if key == "" {
			continue
		}
		seen[key] = true
		out = append(out, recordType)
	}
	for _, recordType := range incoming {
		key := normalizeName(recordType.DeveloperName)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, recordType)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DeveloperName < out[j].DeveloperName
	})
	return out
}

func mergeQueryValidationRules(existing, incoming []schema.ValidationRule) []schema.ValidationRule {
	out := make([]schema.ValidationRule, 0, len(existing)+len(incoming))
	seen := make(map[string]bool, len(existing)+len(incoming))
	for _, rule := range existing {
		key := normalizeName(rule.Namespace + "." + rule.Name)
		if key == "." {
			continue
		}
		seen[key] = true
		out = append(out, rule)
	}
	for _, rule := range incoming {
		key := normalizeName(rule.Namespace + "." + rule.Name)
		if key == "." || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (c querySemanticsChecker) apiNamesMatch(left, right string) bool {
	if strings.EqualFold(left, right) {
		return true
	}
	if namespaced, ok := semaProjectNamespacedAPIName(c.namespace, right); ok && strings.EqualFold(left, namespaced) {
		return true
	}
	if namespaced, ok := semaProjectNamespacedAPIName(c.namespace, left); ok && strings.EqualFold(namespaced, right) {
		return true
	}
	if local, ok := semaProjectLocalAPIName(c.namespace, left); ok && strings.EqualFold(local, right) {
		return true
	}
	if local, ok := semaProjectLocalAPIName(c.namespace, right); ok && strings.EqualFold(left, local) {
		return true
	}
	if localLeft, localRight := semaSchemaLocalAPIName(left), semaSchemaLocalAPIName(right); strings.EqualFold(localLeft, localRight) {
		return true
	}
	return false
}

func hasQuerySchemaField(fields []schema.Field, name string) bool {
	key := normalizeName(name)
	for _, field := range fields {
		if normalizeName(field.Name) == key {
			return true
		}
	}
	return false
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
	fieldNames := make(map[string]bool, len(object.Fields)+len(definition.Relations))
	for _, field := range object.Fields {
		fieldNames[normalizeName(field.Name)] = true
	}
	for _, relationship := range definition.Relations {
		if relationship.Field == "" || fieldNames[normalizeName(relationship.Field)] {
			continue
		}
		object.Fields = append(object.Fields, schema.Field{
			Name:                  relationship.Field,
			Type:                  "Reference",
			ReferenceTo:           append([]string(nil), relationship.ParentObjects...),
			RelationshipName:      relationship.ParentRelationship,
			ChildRelationshipName: relationship.ChildRelationship,
		})
		fieldNames[normalizeName(relationship.Field)] = true
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
		if skipProjectDiagnosticType(typ) {
			continue
		}
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
		if skipProjectDiagnosticType(typ) {
			continue
		}
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
		if dep.Status == "loaded" && semaDependencyBackedByArtifact(index, dep.Namespace) {
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
	sources := a.sources
	if sources == nil {
		sources = newSemaSources(nil, nil)
	}
	seen := make(map[string]bool)
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		source, ok := sources.normalizedForType(typ)
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

func semaDependencyBackedByArtifact(index typesys.Index, namespace string) bool {
	for _, typ := range index.Types {
		if typ.Dependency && typ.Artifact && strings.EqualFold(typ.Namespace, namespace) {
			return true
		}
	}
	return false
}

func (a *Analyzer) checkInheritanceContracts(index typesys.Index) []diagnostic.Diagnostic {
	return a.checkInheritanceContractsWithRecorder(index, nil)
}

func (a *Analyzer) checkInheritanceContractsWithRecorder(index typesys.Index, recorder *perfRecorder) []diagnostic.Diagnostic {
	return a.checkInheritanceContractsWithState(index, buildSemaTypeMemberState(index, recorder, a.sources), recorder)
}

func (a *Analyzer) checkInheritanceContractsWithState(index typesys.Index, state *semaTypeMemberState, recorder *perfRecorder) []diagnostic.Diagnostic {
	return a.checkInheritanceContractsWithView(index, state.view(), recorder)
}

func (a *Analyzer) checkInheritanceContractsWithView(index typesys.Index, model *semaTypeMemberView, recorder *perfRecorder) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	inheritanceStarted := recorder.beginPhase()
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		diagnostics = append(diagnostics, inheritanceTargetDiagnostics(model, typ)...)
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		abstractClass := hasModifier(typ.Modifiers, "abstract")
		missingSuperclass := semaTypeMissingSuperclass(model, typ)
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod {
				continue
			}
			overridden, hasOverridden := overridableInheritedMethod(model, typ, member)
			if hasModifier(member.Modifiers, "override") && !missingSuperclass && !hasOverridden && !hasPlatformInheritedMethodSignature(typ, member) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA016",
					Message:  fmt.Sprintf("method %q is marked override but no inherited method has the same signature", member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
			if !hasModifier(member.Modifiers, "override") && hasOverridden {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA016",
					Message:  fmt.Sprintf("method %q overrides inherited method and must use the override modifier", member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
			if hasModifier(member.Modifiers, "override") && hasOverridden && !semaOverriddenMethodFromDependency(model, typ, member) && hasExplicitDeclarationVisibility(member.Modifiers) && declarationVisibilityRank(member.Modifiers) < declarationVisibilityRank(overridden.Modifiers) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA016",
					Message:  fmt.Sprintf("method %q cannot reduce inherited visibility", member.Name),
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
			requirePublic := requirement.sourceKind == "interface" && typ.Range.End.Offset > typ.Range.Start.Offset
			if hasConcreteMethodSignature(model, typ.Name, requirement.member, requirePublic) {
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
	if recorder != nil {
		recorder.endPhase(&recorder.counters.Inheritance, inheritanceStarted)
	}
	return diagnostics
}

func semaTypeMissingSuperclass(model *semaTypeMemberView, typ typesys.TypeSymbol) bool {
	superClass := strings.TrimSpace(typ.SuperClass)
	if superClass == "" {
		return false
	}
	currentType := semaTypeMembersName(typ)
	resolved := resolveNestedTypeName(model, currentType, superClass)
	if resolved == "" {
		resolved = superClass
	}
	_, ok := model.lookup(normalizeName(resolved))
	return !ok
}

func hasExplicitDeclarationVisibility(modifiers []string) bool {
	return hasModifier(modifiers, "private") || hasModifier(modifiers, "protected") || hasModifier(modifiers, "public") || hasModifier(modifiers, "global")
}

func inheritanceTargetDiagnostics(model *semaTypeMemberView, typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	checkTarget := func(target, role string, expected apexast.DeclarationKind) {
		if strings.TrimSpace(target) == "" {
			return
		}
		resolved := resolveNestedTypeName(model, typ.Name, target)
		if resolved == "" {
			resolved = target
		}
		members, _, ok := semaLookupTypeMembers(model, resolved)
		if !ok || members.kind == expected {
			return
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA017",
			Message:  fmt.Sprintf("%s %q cannot use %s %q", typ.Kind, typ.Name, role, target),
			File:     typ.File,
			Range:    &typ.Range,
		})
	}
	if typ.Kind == apexast.DeclarationClass {
		checkTarget(typ.SuperClass, "extends", apexast.DeclarationClass)
		for _, iface := range typ.Interfaces {
			checkTarget(iface, "implements", apexast.DeclarationInterface)
		}
	} else {
		for _, parent := range typ.Interfaces {
			checkTarget(parent, "extends", apexast.DeclarationInterface)
		}
	}
	return diagnostics
}

func overridableInheritedMethod(model *semaTypeMemberView, typ typesys.TypeSymbol, member typesys.MemberSymbol) (typesys.MemberSymbol, bool) {
	if isObjectOverrideSignature(member) {
		return typesys.MemberSymbol{Modifiers: []string{"public", "virtual"}}, true
	}
	for current := typ.SuperClass; current != ""; {
		members, ok := model.lookup(normalizeName(current))
		if !ok {
			break
		}
		for _, candidate := range members.methods[normalizeName(member.Name)] {
			if sameSemaSignature(candidate, member) && (hasModifier(candidate.Modifiers, "virtual") || hasModifier(candidate.Modifiers, "abstract")) {
				return candidate, true
			}
		}
		current = members.superClass
	}
	return typesys.MemberSymbol{}, false
}

func semaOverriddenMethodFromDependency(model *semaTypeMemberView, typ typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	for current := typ.SuperClass; current != ""; {
		members, ok := model.lookup(normalizeName(current))
		if !ok {
			return false
		}
		for _, candidate := range members.methods[normalizeName(member.Name)] {
			if sameSemaSignature(candidate, member) && (hasModifier(candidate.Modifiers, "virtual") || hasModifier(candidate.Modifiers, "abstract")) {
				return members.dependency
			}
		}
		current = members.superClass
	}
	return false
}

func checkDatabaseBatchableGenericContract(model *semaTypeMemberView, typ typesys.TypeSymbol) []diagnostic.Diagnostic {
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
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA017",
				Message:  fmt.Sprintf("concrete class %q must parameterize Database.Batchable", typ.Name),
				File:     typ.File,
				Range:    &typ.Range,
			})
			continue
		}
		itemType := "Object"
		if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
			itemType = strings.TrimSpace(args[0])
		}
		for _, methodName := range []string{"start", "execute", "finish"} {
			methods := concreteMethodsByName(model, typ.Name, methodName)
			if len(methods) == 0 {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA017",
					Message:  fmt.Sprintf("concrete class %q must implement Database.Batchable<%s> method %q with the matching signature", typ.Name, itemType, methodName),
					File:     typ.File,
					Range:    &typ.Range,
				})
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

func concreteMethodsByName(model *semaTypeMemberView, typeName, methodName string) []typesys.MemberSymbol {
	var out []typesys.MemberSymbol
	for current := typeName; current != ""; {
		members, ok := model.lookup(normalizeName(current))
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

func databaseBatchableMethodCompatible(methodName, itemType string, methods []typesys.MemberSymbol, model *semaTypeMemberView) bool {
	for _, method := range methods {
		switch strings.ToLower(methodName) {
		case "start":
			if len(method.Parameters) != 1 || !sameSemaSignatureType(method.Parameters[0].Type, "Database.BatchableContext") {
				continue
			}
			if strings.EqualFold(method.Type, "Database.QueryLocator") || databaseBatchableStartReturnCompatible(itemType, method.Type, model) {
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

func databaseBatchableExecuteScopeCompatible(itemType, scopeType string, model *semaTypeMemberView) bool {
	required := "List<" + itemType + ">"
	if sameSemaSignatureType(scopeType, required) {
		return true
	}
	scopeBase, scopeArgs := semaGenericBaseAndArgs(scopeType)
	if !strings.EqualFold(scopeBase, "List") || len(scopeArgs) != 1 {
		return false
	}
	return semaAssignableToType(itemType, scopeArgs[0], model) || semaAssignableToType(scopeArgs[0], itemType, model)
}

func databaseBatchableStartReturnCompatible(itemType, returnType string, model *semaTypeMemberView) bool {
	returnType = semaCanonicalPlatformAlias(returnType)
	base, args := semaGenericBaseAndArgs(returnType)
	if (strings.EqualFold(base, "Iterable") || strings.EqualFold(base, "List")) && len(args) == 1 {
		return sameSemaSignatureType(args[0], itemType) ||
			semaAssignableToType(itemType, args[0], model) ||
			semaAssignableToType(args[0], itemType, model)
	}
	// A concrete class implementing Iterable<T> is also a valid start() return type
	// (e.g. a custom scratch-proven Iterable), not just the literal Iterable<T>/List<T> spelling.
	elementType, ok := semaIterableElementTypeInModel(returnType, model)
	if !ok {
		return false
	}
	return sameSemaSignatureType(elementType, itemType) ||
		semaAssignableToType(itemType, elementType, model) ||
		semaAssignableToType(elementType, itemType, model)
}

func (a *Analyzer) checkBodyAssignments(typ typesys.TypeSymbol, member typesys.MemberSymbol, scan *semaBodyExpressionScan, bodyOffset int, source string, scopes semaScopeModel, model *semaTypeMemberView) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	body := scan.body
	for _, match := range scan.assignmentMatches {
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
			valueType := semaResolveConstructedExpressionType(model, typ.Name, value.text, scopes.flat())
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
func (a *Analyzer) checkBodyReturns(typ typesys.TypeSymbol, member typesys.MemberSymbol, scan *semaBodyExpressionScan, bodyOffset int, source string, scopes semaScopeModel, model *semaTypeMemberView) []diagnostic.Diagnostic {
	if member.Type == "" {
		return nil
	}
	body := scan.body
	var diagnostics []diagnostic.Diagnostic
	returnType := strings.TrimSpace(member.Type)
	foundReturn := false
	for _, match := range scan.returnMatches {
		if semaReturnMatchInIgnoredText(body, match) {
			continue
		}
		foundReturn = true
		valueStart, valueEnd, hasValue := semaReturnValueRange(match)
		if strings.EqualFold(returnType, "void") {
			if hasValue {
				diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, "void method cannot return a value", bodyOffset+valueStart, bodyOffset+valueEnd, source))
			}
			continue
		}
		if !hasValue {
			diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s", returnType), bodyOffset+match[0], bodyOffset+match[1], source))
			continue
		}
		value := trimSemaArg(body, valueStart, valueEnd)
		valueType := semaResolveConstructedExpressionType(model, typ.Name, value.text, scopes.flatAt(value.start))
		if strings.EqualFold(returnType, "Boolean") && semaExprContainsComparison(value.text) {
			valueType = "Boolean"
		}
		if valueType == "" || valueType == "null" || semaAssignableToType(returnType, valueType, model) {
			continue
		}
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("returns %s from %s method", valueType, returnType), bodyOffset+value.start, bodyOffset+value.end, source))
	}
	if !foundReturn && !strings.EqualFold(returnType, "void") && !semaBodyEndsWithThrow(body) && !semaBodyContainsReturnKeyword(body) {
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s", returnType), member.Range.Start.Offset, member.Range.End.Offset, source))
	}
	return diagnostics
}

func semaBodyContainsReturnKeyword(body string) bool {
	for _, match := range semaReturnKeywordPattern.FindAllStringIndex(body, -1) {
		if !semaOffsetInIgnoredText(body, match[0]) {
			return true
		}
	}
	return false
}

var semaReturnKeywordPattern = regexp.MustCompile(`\breturn\b`)

func (a *Analyzer) checkBodyTernaryConditions(typ typesys.TypeSymbol, member typesys.MemberSymbol, scan *semaBodyExpressionScan, bodyOffset int, source string, scopes semaScopeModel, model *semaTypeMemberView) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	seen := make(map[int]bool)
	for _, expr := range scan.expressions() {
		if seen[expr.start] {
			continue
		}
		seen[expr.start] = true
		diagnostics = append(diagnostics, checkSemaTernaryCondition(typ, member, expr.text, bodyOffset+expr.start, source, scopes.flat(), model)...)
	}
	return diagnostics
}
func checkSemaTernaryCondition(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr string, exprStart int, source string, scope map[string]string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	question, colon, ok := semaTernaryPositions(expr)
	if !ok {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	condition := strings.TrimSpace(expr[:question])
	if semaTopLevelByte(condition, '=') >= 0 {
		return nil
	}
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
func (a *Analyzer) checkBodyExpressionTypeReferences(typ typesys.TypeSymbol, member typesys.MemberSymbol, scan *semaBodyExpressionScan, bodyOffset int, source string) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	seen := make(map[string]bool)
	for _, expr := range scan.expressions() {
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
func checkSemaPlatformCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []semaArg, start, end int, source string, scope map[string]string, model *semaTypeMemberView, receiverMode string) ([]diagnostic.Diagnostic, bool) {
	if semaProjectTypeShadowsPlatform(model, receiverType) {
		return nil, false
	}
	if strings.EqualFold(receiverType, "System") && strings.EqualFold(method, "runAs") && len(args) == 1 {
		return nil, true
	}
	if semaDatabaseDynamicQueryCall(receiverType, method) {
		return nil, true
	}
	if _, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return nil, false
	}
	if candidates := resolveMemberMethods(model, receiverType, method); len(candidates) != 0 && !semaResolvedMembersAllPlatformBacked(model, candidates) {
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
func checkGeneratedPlatformStaticAccess(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method, receiverMode string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
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
	if owner, ok := model.lookup(normalizeName(candidates[0].owner)); !ok || (!owner.dependency && !owner.sobject) {
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
