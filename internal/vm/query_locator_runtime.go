package vm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// A locator spends a query, but its records belong to the locator row budget.
// A cursor reserves cursor rows at creation and spends queries/rows at fetch.
type soqlHandleAccounting uint8

const (
	soqlOrdinaryAccounting soqlHandleAccounting = iota
	soqlCursorAccounting
	soqlLocatorAccounting
	soqlPaginationAccounting
)

// R181/R183/N029/N030: reject locators at the value's serialization point so
// the same restriction applies inside collections and on the pretty route.
type queryLocatorJSONError struct{}

func (*queryLocatorJSONError) Error() string {
	return "Apex Type unsupported in JSON: Database.QueryLocator"
}

func (err *queryLocatorJSONError) MarshalJSON() ([]byte, error) { return nil, err }

func (vm *VM) databaseCursorJSONValue(cursor Value, suppressNulls bool) any {
	var queryID any
	if _, storedID, ok := objectFieldValue(cursor, "queryId"); ok && storedID.Kind == ValueString {
		queryID = storedID.Text
	} else if query, ok := cursor.Fields["Query"]; ok && query.Kind == ValueString {
		// N022/N031: the public payload contains only an opaque String queryId.
		// Retain the creation-time snapshot privately instead of publishing rows.
		token := "local-cursor-" + strconv.FormatUint(cursor.Ref, 16)
		if vm.queryCursors == nil {
			vm.queryCursors = make(map[string]Value)
		}
		vm.queryCursors[token] = cursor
		queryID = token
	}
	if queryID == nil && suppressNulls {
		return orderedJSONObject{}
	}
	return orderedJSONObject{{name: "queryId", value: queryID}}
}

func (vm *VM) databaseCursorFromJSON(raw any, strict bool) (Value, error) {
	fields, ok := jsonObjectFields(raw)
	if !ok {
		return Null, jsonTypeMappingError("Database.Cursor", raw)
	}
	cursor := Object("Database.Cursor")
	for _, field := range fields {
		if field.name != "queryId" {
			if strict {
				return Null, jsonDeserializeException("Unknown field: Database.Cursor.%s", field.name)
			}
			continue
		}
		if field.value == nil {
			continue
		}
		token, ok := field.value.(string)
		if snapshot, found := vm.queryCursors[token]; ok && found {
			cursor.Fields = copyValueMap(snapshot.Fields)
			cursor.Fields["queryId"] = String(token)
			continue
		}
		// API64 C1-C3: ordinary deserialization admits this opaque handle
		// shape for storage and serialization, independently of a live query.
		// API65 also admits ordinary alphanumeric text and suffixed handles.
		// Strict and other token shapes remain outside these observations.
		if !strict && ok && (cursorJSONShellToken(token) || vm.cursorJSONTextShellToken(token)) {
			cursor.Fields["queryId"] = String(token)
			continue
		}
		// N032/N035/N036 yielded no framed native answer for unknown tokens.
		// Remote token resolution is an explicit boundary.
		return Null, unsupportedCallError("Database.Cursor JSON queryId resolution outside the local execution")
	}
	return cursor, nil
}

func cursorJSONShellToken(token string) bool {
	if len(token) != 15 || !strings.HasPrefix(token, "0r8") {
		return false
	}
	for i := 3; i < len(token); i++ {
		c := token[i]
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
			return false
		}
	}
	return true
}

// API65 observations cover an opaque word or one word suffix added to a
// registered query token. Admission does not register a query or resolve rows.
func (vm *VM) cursorJSONTextShellToken(token string) bool {
	api, ok := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !ok || api != 65 {
		return false
	}
	if cursorJSONTokenWord(token) {
		return true
	}
	separator := strings.LastIndexByte(token, '-')
	if separator <= 0 || !cursorJSONTokenWord(token[separator+1:]) {
		return false
	}
	_, registered := vm.queryCursors[token[:separator]]
	return registered
}

func cursorJSONTokenWord(token string) bool {
	if token == "" {
		return false
	}
	for i := range token {
		c := token[i]
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
			return false
		}
	}
	return true
}

func unresolvedCursorJSON(receiver Value) bool {
	if !strings.EqualFold(receiver.Type, "Database.Cursor") {
		return false
	}
	_, token, hasToken := objectFieldValue(receiver, "queryId")
	_, hasRecords := receiver.Fields["Records"]
	return hasToken && token.Kind == ValueString && !hasRecords
}

func (vm *VM) databaseQueryHandle(query Value, binds Value, accessLevel Value, cursor bool, result *Result) (Value, error) {
	if query.Kind == ValueNull {
		return Null, newExceptionError("NullPointerException", "Argument 1 cannot be null")
	}
	if query.Kind != ValueString {
		return Null, fmt.Errorf("Database query handle expects query String or inline SOQL")
	}
	if cursor && strings.TrimSpace(query.Text) == "" {
		// R153: the native message includes the exception's type prefix.
		return Null, newExceptionError("InvalidParameterValueException", "System.InvalidParameterValueException: Query must not be empty or null")
	}
	mapBinds := binds.Kind == ValueMap
	emptySumConstruction := false
	emptyDateConstruction := false
	expand := vm.expandSOQLBinds
	if binds.Kind == ValueMap {
		expand = func(raw string) (string, error) { return vm.expandSOQLBindsFromMap(raw, binds) }
	} else {
		binds = typedMap("Map<String,Object>")
	}
	// Validate restrictions on query handles before the ordinary SOQL engine.
	// Normal queries still support COUNT, aggregates, OFFSET and FOR UPDATE.
	if parsed, err := vm.parseSOQLAt(query.Text); err == nil {
		if parsed.Count {
			if !cursor && !mapBinds && accessLevel.Kind == ValueNull {
				return Null, vm.scalarCountRouteError(query.Text, expand)
			}
			return Null, newExceptionError("QueryException", "use countQuery() for [select count()...] queries")
		}
		if len(parsed.Aggregates) > 0 || len(parsed.GroupBy) > 0 {
			if !cursor && !mapBinds && accessLevel.Kind == ValueNull {
				emptySumConstruction = vm.emptyReferenceSumLocatorConstruction(parsed) || vm.emptyReferenceSumCriteriaConstruction(parsed, query.Text)
				if !emptySumConstruction {
					emptyDateConstruction = vm.emptyReferenceDateAggregateLocatorConstruction(parsed, query.Text)
				}
			}
			if !emptySumConstruction && !emptyDateConstruction {
				if !cursor && !mapBinds && accessLevel.Kind == ValueNull {
					if err := vm.emptyReferenceSumMissingCriterion(parsed); err != nil {
						return Null, err
					}
				}
				message := "Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch"
				if cursor {
					message = "Query contains aggregate expressions, these are not permitted in cursor queries"
				}
				return Null, newExceptionError("QueryException", message)
			}
		}
		if cursor && len(parsed.ChildQueries) > 0 {
			// V006/V008: relationship subqueries are rejected on cursor routes,
			// including WithBinds. Locators retain their child projections.
			return Null, newExceptionError("QueryException", "Query contains aggregate relationships, these are not permitted in cursor queries")
		}
		if parsed.ForUpdate {
			message := "You cannot lock records in Batch Apex using FOR UPDATE"
			if cursor {
				message = "You cannot lock records with Cursor using FOR UPDATE"
			}
			return Null, newExceptionError("QueryException", message)
		}
	}
	accounting := soqlLocatorAccounting
	typeName := "Database.QueryLocator"
	if cursor {
		accounting = soqlCursorAccounting
		typeName = "Database.Cursor"
	}
	mode := vm.defaultAccessLevelMode()
	if isDatabaseAccessLevelValue(accessLevel) {
		mode = databaseAccessLevelSecurityMode(accessLevel)
	}
	expandedQuery := query.Text
	expandAtCreation := func(raw string) (string, error) {
		text, err := expand(raw)
		if err == nil {
			expandedQuery = text
		}
		return text, err
	}
	var querySource *storage.OrgState
	values, err := vm.executeSOQLRowsWithAccountingAndSource(query.Text, result, expandAtCreation, binds, mode, accessLevelPermissionSetID(accessLevel), accounting, func(source *storage.OrgState) {
		querySource = source
	})
	if err != nil {
		return Null, err
	}
	records := List(values...)
	records.Type = "List<SObject>"
	handle := Object(typeName)
	handle.Fields["Records"] = records
	handle.Fields["Query"] = query
	if emptySumConstruction || (emptyDateConstruction && len(values) != 0) {
		// SUM iteration remains unobserved. The captured Date aggregate shapes
		// use the ordinary iterator only with a validated empty snapshot.
		handle.Fields["__emptySumConstructionOnly"] = Bool(true)
	}
	// V005/V007: freeze the expansion used at creation, including child binds.
	// Query remains the original public getQuery() text.
	handle.Fields["__expandedQuery"] = String(expandedQuery)
	// V013/V014: alternate sources contain rows absent from the raw org.
	// Keep that exact creation source; ordinary records still refresh live.
	if querySource != nil && querySource != vm.Org {
		if vm.queryHandleSources == nil {
			vm.queryHandleSources = make(map[string]*storage.OrgState)
		}
		token := strconv.FormatUint(handle.Ref, 16)
		vm.queryHandleSources[token] = querySource
		handle.Fields["__querySource"] = String(token)
	}
	if cursor {
		if err := vm.incrementLimit("apexCursors", 1); err != nil {
			return Null, err
		}
	} else if err := vm.incrementQueryLocatorRows(records); err != nil {
		return Null, err
	}
	return handle, nil
}

// This isolated precedence candidate validates one missing scalar string
// equality criterion before aggregate rejection. Native ordering is pending;
// valid criteria still take the prior rejection path, without new admission.
func (vm *VM) emptyReferenceSumMissingCriterion(query soql.Query) error {
	if len(query.Aggregates) != 1 || len(query.GroupBy) != 1 || len(query.Fields) != 2 ||
		!strings.EqualFold(query.Fields[0], query.GroupBy[0]) || query.Where == nil || len(query.Where.And) != 2 {
		return nil
	}
	criterion := query.Where.And[1]
	if criterion.Op != "=" || criterion.Value.Kind != storage.ValueString || len(criterion.Values) != 0 ||
		criterion.Not || len(criterion.And) != 0 || len(criterion.Or) != 0 || criterion.Range || criterion.Subquery != nil ||
		criterion.Field == "" || !isIdentStart(criterion.Field[0]) {
		return nil
	}
	for i := 1; i < len(criterion.Field); i++ {
		if !isIdentPart(criterion.Field[i]) {
			return nil
		}
	}
	// Reuse the existing empty lexical reference/SUM gate unchanged. Replace
	// only a copied predicate to check its input boundary, never the query
	// executed or admitted by databaseQueryHandle.
	shape := query
	where := *query.Where
	where.And = append([]soql.Condition(nil), query.Where.And...)
	where.And[1] = soql.Condition{Field: query.Aggregates[0].Field, Op: ">", Value: storage.IntegerValue(0)}
	shape.Where = &where
	if !vm.emptyReferenceSumLocatorConstruction(shape) {
		return nil
	}
	objectName, known := storage.ResolveObjectName(*vm.Org, query.Object)
	if !known {
		return nil
	}
	definition := vm.Org.Objects[objectName].Definition
	if _, known := storage.ResolveFieldName(definition, vm.Org.Namespace, criterion.Field); known {
		return nil
	}
	return newExceptionError("QueryException", soql.MissingColumnMessage(criterion.Field, objectName))
}

// Additional isolated construction forms, pending native observations. Reuse
// the original lexical/reference/SUM boundary and the ordinary query validator;
// do not admit arbitrary criteria, other date literals, or additional Boolean nesting.
func (vm *VM) emptyReferenceSumCriteriaConstruction(query soql.Query, raw string) bool {
	if len(query.Aggregates) != 1 || len(query.GroupBy) != 1 || len(query.Fields) != 2 ||
		!strings.EqualFold(query.Fields[0], query.GroupBy[0]) || query.Where == nil || len(query.Where.And) != 2 {
		return false
	}
	criterion := query.Where.And[1]
	if criterion.Not || len(criterion.And) != 0 || criterion.Subquery != nil || len(criterion.Values) != 0 {
		return false
	}
	shape := query
	where := *query.Where
	where.And = append([]soql.Condition(nil), query.Where.And...)
	where.And[1] = soql.Condition{Field: query.Aggregates[0].Field, Op: ">", Value: storage.IntegerValue(0)}
	shape.Where = &where
	if !vm.emptyReferenceSumLocatorConstruction(shape) {
		return false
	}
	objectName, known := storage.ResolveObjectName(*vm.Org, query.Object)
	if !known {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	fieldType := func(condition soql.Condition) storage.FieldType {
		if condition.Not || len(condition.And) != 0 || len(condition.Or) != 0 || condition.Subquery != nil ||
			len(condition.Values) != 0 || condition.Field == "" || !isIdentStart(condition.Field[0]) {
			return ""
		}
		for i := 1; i < len(condition.Field); i++ {
			if !isIdentPart(condition.Field[i]) {
				return ""
			}
		}
		name, known := storage.ResolveFieldName(definition, vm.Org.Namespace, condition.Field)
		if !known {
			return ""
		}
		return definition.Fields[name].Type
	}
	dateBeforeToday := func(condition soql.Condition) bool {
		return fieldType(condition) == storage.FieldDate && condition.Op == "<" && condition.Range &&
			condition.Value.Kind == storage.ValueDate && condition.Value2.Kind == storage.ValueDate &&
			locatorCriterionHasTodayLiteral(raw, condition.Field)
	}
	amountComparison := func(condition soql.Condition, operator string) bool {
		return fieldType(condition) == storage.FieldDecimal && !condition.Range && condition.Op == operator &&
			strings.EqualFold(condition.Field, query.Aggregates[0].Field) &&
			(condition.Value.Kind == storage.ValueInteger || condition.Value.Kind == storage.ValueDecimal)
	}
	if len(criterion.Or) != 0 {
		return criterion.Parenthesized && criterion.Field == "" && !criterion.Range && len(criterion.Or) == 2 &&
			dateBeforeToday(criterion.Or[0]) && amountComparison(criterion.Or[1], ">")
	}
	return dateBeforeToday(criterion) || amountComparison(criterion, "<") ||
		(fieldType(criterion) == storage.FieldPicklist && !criterion.Range && criterion.Value.Kind == storage.ValueString &&
			(criterion.Op == "=" || criterion.Op == "!="))
}

// The parsed date range does not retain its literal spelling. Check the source
// tokens so a coincidentally equal range cannot generalize TODAY admission.
func locatorCriterionHasTodayLiteral(raw, field string) bool {
	tokens, err := lex(raw)
	if err != nil {
		return false
	}
	for i := 2; i < len(tokens); i++ {
		if tokens[i].kind == tokenIdent && strings.EqualFold(tokens[i].text, "TODAY") && tokens[i-1].text == "<" &&
			tokens[i-2].kind == tokenIdent && strings.EqualFold(tokens[i-2].text, field) {
			return true
		}
	}
	return false
}

// Owned Date aggregate observations confirm these constructor and empty iterator
// shapes with an empty lexical bind. Matching binds retain aggregate rejection.
func (vm *VM) emptyReferenceDateAggregateLocatorConstruction(query soql.Query, raw string) bool {
	if vm.Org == nil || len(query.Fields) != 2 || len(query.Aggregates) != 1 || len(query.GroupBy) != 1 ||
		!strings.EqualFold(query.Fields[0], query.GroupBy[0]) || query.GroupMode != "" || query.Having != nil ||
		len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 || len(query.Order) != 0 || query.OrderBy != "" ||
		query.HasLimit || query.LimitBind != "" || query.HasOffset || query.OffsetBind != "" ||
		query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" || query.SecurityMode != "" {
		return false
	}
	aggregate := query.Aggregates[0]
	maximum := strings.EqualFold(aggregate.Func, "MAX")
	if !maximum && !strings.EqualFold(aggregate.Func, "COUNT") {
		return false
	}
	where := query.Where
	if where == nil || where.Not || len(where.Or) != 0 || len(where.And) != 2 || where.Field != "" {
		return false
	}
	membership, criterion := where.And[0], where.And[1]
	if membership.Not || len(membership.And) != 0 || len(membership.Or) != 0 || membership.Range || membership.Subquery != nil ||
		!strings.EqualFold(membership.Op, "IN") || len(membership.Values) != 1 || !strings.EqualFold(membership.Field, query.GroupBy[0]) ||
		!criterion.Parenthesized || criterion.Not || len(criterion.And) != 0 || criterion.Subquery != nil || len(criterion.Values) != 0 {
		return false
	}
	if !vm.emptyReferenceLocatorBind(membership.Values[0]) {
		return false
	}
	objectName, known := storage.ResolveObjectName(*vm.Org, query.Object)
	if !known {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	fieldType := func(field string) storage.FieldType {
		if field == "" || !isIdentStart(field[0]) {
			return ""
		}
		for i := 1; i < len(field); i++ {
			if !isIdentPart(field[i]) {
				return ""
			}
		}
		name, known := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
		if !known {
			return ""
		}
		return definition.Fields[name].Type
	}
	if fieldType(membership.Field) != storage.FieldReference || fieldType(aggregate.Field) != storage.FieldDate {
		return false
	}
	scalar := func(condition soql.Condition) bool {
		return !condition.Not && len(condition.And) == 0 && len(condition.Or) == 0 && condition.Subquery == nil && len(condition.Values) == 0
	}
	dateBeforeToday := func(condition soql.Condition) bool {
		return scalar(condition) && strings.EqualFold(condition.Field, aggregate.Field) && condition.Op == "<" && condition.Range &&
			condition.Value.Kind == storage.ValueDate && condition.Value2.Kind == storage.ValueDate && locatorCriterionHasTodayLiteral(raw, condition.Field)
	}
	decimalComparison := func(condition soql.Condition, operator string) bool {
		return scalar(condition) && fieldType(condition.Field) == storage.FieldDecimal && !condition.Range && condition.Op == operator &&
			(condition.Value.Kind == storage.ValueInteger || condition.Value.Kind == storage.ValueDecimal)
	}
	if !maximum {
		return decimalComparison(criterion, "<")
	}
	if len(criterion.Or) == 0 {
		return dateBeforeToday(criterion)
	}
	return criterion.Field == "" && !criterion.Range && len(criterion.Or) == 2 &&
		dateBeforeToday(criterion.Or[0]) && decimalComparison(criterion.Or[1], ">")
}

// Restrict empty aggregate construction to an already-active lexical bind.
// In particular, never evaluate a property while testing candidate admission.
func (vm *VM) emptyReferenceLocatorBind(bound storage.Value) bool {
	if bound.Kind != storage.ValueID || !strings.HasPrefix(string(bound.ID), ":") {
		return false
	}
	name := strings.TrimPrefix(string(bound.ID), ":")
	if name == "" || !isIdentStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isIdentPart(name[i]) {
			return false // Member, expression and method binds keep their old path.
		}
	}
	lexical := name
	if _, present := vm.Globals[lexical]; !present {
		var present bool
		lexical, present = vm.lookupGlobalName(name)
		if !present {
			return false
		}
	}
	// Do not evaluate a getter here and again during expansion. Only an
	// explicitly declared lexical binding can opt into this candidate.
	declared := vm.VarTypes[lexical]
	element, typed := collectionElementType(declared)
	if collectionBase(declared) != "Set" || !typed || !strings.EqualFold(canonicalRuntimePlatformType(element), "Id") {
		return false
	}
	value, err := vm.lookup(name) // Reuse the existing lexical lookup unchanged.
	return err == nil && value.Kind == ValueSet && len(value.Set) == 0
}

// Admit only a grouped numeric SUM, a reference-field IN bind whose declared
// value is an empty Set<Id>, and one scalar amount comparison. This checks the
// input, never whether executing a nonempty bind happens to return no records.
// The ordinary pipeline below still validates fields, literals and syntax.
// Native construction/iteration answers for this shape remain pending.
func (vm *VM) emptyReferenceSumLocatorConstruction(query soql.Query) bool {
	if vm.Org == nil || len(query.Fields) != 2 || len(query.Aggregates) != 1 || len(query.GroupBy) != 1 ||
		!strings.EqualFold(query.Aggregates[0].Func, "SUM") || query.GroupMode != "" || query.Having != nil ||
		len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 || len(query.Order) != 0 || query.OrderBy != "" ||
		query.HasLimit || query.LimitBind != "" || query.HasOffset || query.OffsetBind != "" ||
		query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" || query.SecurityMode != "" {
		return false
	}
	where := query.Where
	if where == nil || where.Not || len(where.Or) != 0 || len(where.And) != 2 || where.Field != "" {
		return false
	}
	membership, amount := where.And[0], where.And[1]
	for _, condition := range []soql.Condition{membership, amount} {
		if condition.Not || len(condition.And) != 0 || len(condition.Or) != 0 || condition.Range || condition.Subquery != nil {
			return false
		}
	}
	if !strings.EqualFold(membership.Op, "IN") || len(membership.Values) != 1 ||
		!strings.EqualFold(membership.Field, query.GroupBy[0]) || amount.Op != ">" ||
		!strings.EqualFold(amount.Field, query.Aggregates[0].Field) ||
		(amount.Value.Kind != storage.ValueInteger && amount.Value.Kind != storage.ValueDecimal) {
		return false
	}
	if !vm.emptyReferenceLocatorBind(membership.Values[0]) {
		return false
	}
	objectName, known := storage.ResolveObjectName(*vm.Org, query.Object)
	if !known {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	reference, referenceKnown := storage.ResolveFieldName(definition, vm.Org.Namespace, membership.Field)
	numeric, numericKnown := storage.ResolveFieldName(definition, vm.Org.Namespace, amount.Field)
	return referenceKnown && numericKnown && definition.Fields[reference].Type == storage.FieldReference &&
		definition.Fields[numeric].Type == storage.FieldDecimal
}

// R131/R188-R193: positions retain the creation-time IDs, while each retrieval
// reads current stored fields and omits deleted rows. Returned rows are private.
func (vm *VM) databaseQueryHandlePage(handle Value, snapshots []Value) (Value, error) {
	page := List(append([]Value(nil), snapshots...)...)
	page.Type = "List<SObject>"
	querySource := vm.Org
	if token := handle.Fields["__querySource"]; token.Kind == ValueString {
		if source, found := vm.queryHandleSources[token.Text]; found {
			querySource = source
		}
	}
	raw, ok := handle.Fields["__expandedQuery"]
	if !ok {
		raw, ok = handle.Fields["Query"]
	}
	if !ok || raw.Kind != ValueString || raw.Text == "" || querySource == nil || len(snapshots) == 0 {
		return page, nil
	}
	query, err := vm.parseSOQLAt(raw.Text)
	if err != nil {
		return Null, err
	}
	ids := make([]storage.Value, 0, len(snapshots))
	for _, row := range snapshots {
		if _, id, ok := objectFieldValue(row, "Id"); ok && id.Kind != ValueNull {
			if recordID, ok := sObjectIDFromValue(id); ok {
				ids = append(ids, storage.IDValue(recordID))
			}
		}
	}
	if len(ids) != len(snapshots) {
		return page, nil
	}
	query.Where = &soql.Condition{Field: "Id", Op: "IN", Values: ids}
	query.Limit, query.Offset = 0, 0
	query.HasLimit, query.HasOffset = false, false
	query.LimitBind, query.OffsetBind = "", ""
	query.Order, query.OrderBy = nil, ""
	query.SecurityMode = ""
	if object, ok := vm.resolveObjectName(query.Object); ok {
		query.Object = object
	}
	fetched, err := soql.ExecuteWithCache(*querySource, query, vm.soqlExecutionCacheForOrg(querySource))
	if err != nil {
		return Null, err
	}
	byID := make(map[storage.ID]storage.Record, len(fetched.Records))
	for _, record := range fetched.Records {
		byID[record.ID] = record
	}
	page.List = nil
	for _, id := range ids {
		_, record, ok := storage.LookupRecordByID(byID, id.ID)
		if !ok {
			continue
		}
		row := vm.vmValueFromRecord(record)
		vm.applySOQLRecordProjection(&row, record, query)
		vm.hydrateQueriedRecordTypeRelationships(row)
		page.List = append(page.List, row)
	}
	return page, nil
}

func (vm *VM) databaseQueryLocatorIterator(locator Value) (Value, Value, bool, bool, error) {
	if construction := locator.Fields["__emptySumConstructionOnly"]; construction.Kind == ValueBool && construction.Bool {
		return Null, locator, false, true, newExceptionError("QueryException", "Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch")
	}
	records, ok := locator.Fields["Records"]
	if !ok || records.Kind != ValueList {
		return Null, locator, false, true, fmt.Errorf("Database.QueryLocator missing records")
	}
	// R045/R046: the first iterator acquires the locator's records; a second
	// iterator does not acquire them again, even while the first is unexhausted.
	if opened := locator.Fields["__iteratorOpened"]; opened.Kind == ValueBool && opened.Bool {
		records = List()
	}
	iterator := Object("Database.QueryLocatorIterator")
	iterator.Fields["__values"] = List(append([]Value(nil), records.List...)...)
	iterator.Fields["__index"] = Int(0)
	iterator.Fields["__queryLocator"] = locator
	locator.Fields["__iteratorOpened"] = Bool(true)
	return iterator, locator, true, true, nil
}

func (vm *VM) callQueryLocatorIterator(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if strings.EqualFold(method, "hasNext") {
		method = "hasNext"
	} else if strings.EqualFold(method, "next") {
		method = "next"
	}
	loaded := receiver.Fields["__loaded"]
	mutated := false
	if len(args) == 0 && (!loaded.Bool || loaded.Kind != ValueBool) {
		page, err := vm.databaseQueryHandlePage(receiver.Fields["__queryLocator"], receiver.Fields["__values"].List)
		if err != nil {
			return Null, receiver, false, true, err
		}
		// N007-N010: acquire/charge the first page lazily on hasNext or next.
		if err := vm.incrementLimit("queryRows", len(page.List)); err != nil {
			return Null, receiver, false, true, err
		}
		receiver.Fields["__values"] = page
		receiver.Fields["__loaded"] = Bool(true)
		mutated = true
	}
	value, updated, advanced, handled, err := callIteratorMember(receiver, method, args)
	return value, updated, mutated || advanced, handled, err
}
