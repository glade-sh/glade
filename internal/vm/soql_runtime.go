package vm

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/sosl"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) parseSOQLAt(queryText string) (soql.Query, error) {
	month := 1
	if vm != nil && vm.Org != nil {
		month = soql.FiscalYearStartMonth(*vm.Org)
	}
	return soql.ParseAtWithFiscalYearStartMonthAndTimeZone(queryText, vm.fakeNow, month, vm.currentUserTimeZoneID())
}

func (vm *VM) countQueryParseError(err error, queryText string) error {
	// Named API 62-66 and anonymous callers retain the ordinary query message.
	// API 67 named countQuery exposes the lexer position in the original query,
	// before expansion changes the length of any preceding bind expression.
	if vm.currentClass == "" || !apexversion.AtLeast(vm.currentMethod.APIVersion, 67) {
		return nil
	}
	var queryErr *soql.QueryError
	if errors.As(err, &queryErr) && queryErr.DateTimeLexerMessage != "" {
		return newExceptionErrorWithContext("QueryException", queryErr.DateTimeLexerMessage, fmt.Sprintf("generated SOQL %q", queryText))
	}
	return nil
}

func (vm *VM) executeSOQLRowsWithExpander(raw string, execResult *Result, expand func(string) (string, error), binds Value, accessLevelMode string) ([]Value, error) {
	if strings.TrimSpace(accessLevelMode) == "" {
		accessLevelMode = vm.defaultAccessLevelMode()
	}
	return vm.executeSOQLRowsWithExpanderAndScope(raw, execResult, expand, binds, accessLevelMode, "")
}

func (vm *VM) executeSOQLRowsWithExpanderAndScope(raw string, execResult *Result, expand func(string) (string, error), binds Value, accessLevelMode, permissionSetID string) ([]Value, error) {
	return vm.executeSOQLRowsWithAccounting(raw, execResult, expand, binds, accessLevelMode, permissionSetID, soqlOrdinaryAccounting)
}

// Cursor creation uses the same query/security/sharing pipeline, but reserves
// cursor rows rather than spending the query and query-row budgets of fetch.
func (vm *VM) executeSOQLRowsWithAccounting(raw string, execResult *Result, expand func(string) (string, error), binds Value, accessLevelMode, permissionSetID string, accounting soqlHandleAccounting) ([]Value, error) {
	return vm.executeSOQLRowsWithAccountingAndSource(raw, execResult, expand, binds, accessLevelMode, permissionSetID, accounting, nil)
}

func (vm *VM) executeSOQLRowsWithAccountingAndSource(raw string, execResult *Result, expand func(string) (string, error), binds Value, accessLevelMode, permissionSetID string, accounting soqlHandleAccounting, captureSource func(*storage.OrgState)) ([]Value, error) {
	if soql.IsSOSLFind(raw) {
		return nil, unsupportedCallError("SOSL/FIND local search surface")
	}
	if query, err := vm.parseSOQLAt(raw); err == nil {
		if err := vm.validateSOQLSelection(query, true); err != nil {
			return nil, err
		}
	}
	queryText, err := expand(raw)
	if err != nil {
		var thrown *apexThrowError
		if errors.As(err, &thrown) {
			return nil, err
		}
		return nil, newExceptionError("QueryException", fmt.Sprintf("%s in query %q", err.Error(), raw))
	}
	if soql.IsSOSLFind(queryText) {
		return nil, unsupportedCallError("SOSL/FIND local search surface")
	}
	query, err := vm.parseSOQLAt(queryText)
	if err != nil {
		var queryErr *soql.QueryError
		if errors.As(err, &queryErr) {
			// N014/N017: the captured handle rejection omits generated-query
			// details. Ordinary-query controls yielded no native answer, so keep
			// their existing diagnostic and the compiler's parser text intact.
			if queryErr.NonGroupedAggregateLimit && accounting != soqlOrdinaryAccounting {
				return nil, newExceptionError("QueryException", "Non-grouped query that uses overall aggregate functions cannot also use LIMIT")
			}
			if queryErr.Message != "" {
				return nil, soqlQueryException(queryErr, queryText)
			}
		}
		var unsupported *soql.UnsupportedFeatureError
		if errors.As(err, &unsupported) {
			if strings.Contains(unsupported.Message, "unsupported SOQL token") {
				return nil, newExceptionError("QueryException", fmt.Sprintf("%s in generated SOQL %q", unsupported.Message, queryText))
			}
			return nil, &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
		}
		return nil, newExceptionError("QueryException", fmt.Sprintf("%s in generated SOQL %q", err.Error(), queryText))
	}
	if err := vm.validateSOQLSelection(query, false); err != nil {
		return nil, err
	}
	// R007-R012/R062: OFFSET query handles contain no records. Ordinary SOQL
	// still paginates normally. Validation above retains its existing errors.
	if accounting != soqlOrdinaryAccounting && query.HasOffset {
		query.Limit, query.HasLimit = 0, true
	}
	if strings.TrimSpace(query.SecurityMode) == "" {
		query.SecurityMode = accessLevelMode
	}
	if strings.EqualFold(query.SecurityMode, "USER_MODE") && apexversion.Before(vm.currentMethod.APIVersion, 66) && strings.EqualFold(vm.executionUser.Fields["UserType"].String(), "AutomatedProcess") {
		return nil, newExceptionError("QueryException", "Automated Process User requires API version 66.0 or later for WITH USER_MODE")
	}
	countsQueryLimit := vm.soqlCountsQueryLimit(query)
	if countsQueryLimit && accounting != soqlCursorAccounting {
		if err := vm.incrementLimit("queries", 1); err != nil {
			return nil, err
		}
		// A child relationship selection spends the
		// aggregate-query budget even when the parent query returns no rows.
		if err := vm.incrementLimit("aggregateQueries", len(query.ChildQueries)); err != nil {
			return nil, err
		}
	}
	if values, handled, err := vm.executeSoqlStub(query, queryText, binds, execResult); handled || err != nil {
		if handled && err == nil && accounting == soqlCursorAccounting {
			err = vm.incrementLimit("apexCursorRows", len(values))
		}
		return values, err
	}
	traceStart, traceStartedAt := traceSpanStart(execResult)
	if vm.Org == nil {
		return nil, fmt.Errorf("SOQL requires org state")
	}
	if err := vm.enforceSOQLSecurity(query, permissionSetID); err != nil {
		return nil, err
	}
	if err := vm.enforceOrgShapeObjectAvailability(query); err != nil {
		return nil, err
	}
	executeQuery := query
	executeQuery.SecurityMode = ""
	if resolved, ok := vm.resolveObjectName(executeQuery.Object); ok {
		executeQuery.Object = resolved
	}
	if validationQuery, ok := vm.soqlIDLiteralValidationQuery(raw, executeQuery); ok {
		if err := vm.validateSOQLIDLiteralConditions(validationQuery); err != nil {
			return nil, err
		}
	}
	vm.normalizeSOQLRelationshipGroupBy(&executeQuery)
	executeOrg := vm.Org
	// Native Document isolation is captured for ordinary, direct SOQL. Query
	// handles retain their live source org and must not receive a filtered copy.
	if accounting == soqlOrdinaryAccounting && strings.EqualFold(executeQuery.Object, "Document") {
		executeOrg = vm.orgWithTestDocumentVisibility(executeQuery.Object)
	}
	if strings.EqualFold(executeQuery.Object, "UserRecordAccess") {
		syntheticOrg := vm.orgWithSyntheticUserRecordAccess()
		executeOrg = &syntheticOrg
	} else if strings.EqualFold(executeQuery.Object, "RecentlyViewed") {
		syntheticOrg := vm.orgWithSyntheticRecentlyViewed()
		executeOrg = &syntheticOrg
	} else if strings.EqualFold(executeQuery.Object, "FlowDefinitionView") || strings.EqualFold(executeQuery.Object, "FlowVariableView") {
		syntheticOrg := vm.orgWithSyntheticFlowMetadata()
		executeOrg = &syntheticOrg
	}
	if captureSource != nil {
		captureSource(executeOrg)
	}
	result, err := soql.ExecuteWithCache(*executeOrg, executeQuery, vm.soqlExecutionCacheForOrg(executeOrg))
	if err != nil {
		var queryErr *soql.QueryError
		if errors.As(err, &queryErr) && queryErr.Message != "" {
			return nil, soqlQueryException(queryErr, queryText)
		}
		var unsupported *soql.UnsupportedFeatureError
		if errors.As(err, &unsupported) {
			return nil, &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
		}
		return nil, newExceptionError("QueryException", fmt.Sprintf("%s in generated SOQL %q", err.Error(), queryText))
	}
	result = vm.applySOQLSharing(query, result)
	if query.ForView || query.ForReference {
		vm.recordRecentlyViewedRows(query.Object, result.Records, query.ForView, query.ForReference)
	}
	limitRows := soqlLimitRows(result)
	// Custom metadata exempts ordinary projections from query count, not rows.
	if accounting == soqlCursorAccounting {
		if err := vm.incrementLimit("apexCursorRows", len(result.Records)); err != nil {
			return nil, err
		}
	} else if accounting == soqlOrdinaryAccounting && (countsQueryLimit || vm.triggerDepth == 0) {
		if err := vm.incrementLimit("queryRows", limitRows); err != nil {
			return nil, err
		}
	}
	if err := vm.incrementLimit("cpuTime", limitRows); err != nil {
		return nil, err
	}
	values := make([]Value, 0, len(result.Records))
	for _, record := range result.Records {
		value := vm.vmValueFromRecord(record)
		if record.Object == "AggregateResult" {
			value = vm.aggregateResultFromRecord(record, query)
		}
		if value.Kind == ValueObject {
			vm.applySOQLRecordProjection(&value, record, query)
			vm.hydrateQueriedRecordTypeRelationships(value)
		}
		values = append(values, value)
	}
	appendTraceLazy(execResult, "apex.soql", "apex.soql", func() map[string]any {
		return vm.traceSOQLArgs(queryText, query.Object, result.Rows)
	})
	appendDurationTraceLazy(
		execResult,
		"apex.soql",
		"apex.soql",
		traceStart,
		traceDurationSince(traceStartedAt),
		func() map[string]any {
			return vm.traceSOQLArgs(queryText, query.Object, result.Rows)
		},
	)
	return values, nil
}

func (vm *VM) orgWithTestDocumentVisibility(objectName string) *storage.OrgState {
	if vm.testContext == nil || vm.testContext.SeeAllData || len(vm.testDocumentBaseline) == 0 {
		return vm.Org
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return vm.Org
	}
	records := make(map[storage.ID]storage.Record, len(object.Records))
	hidden := false
	for id, record := range object.Records {
		if _, baseline := vm.testDocumentBaseline[id]; baseline {
			hidden = true
			continue
		}
		records[id] = record
	}
	if !hidden {
		return vm.Org
	}
	// The org and journal keep the complete records. This query gets fresh
	// record/index containers, so setup inserts and later mutations stay visible
	// without changing another method's SeeAllData view or cached runtime.
	org := *vm.Org
	org.Objects = make(map[string]storage.ObjectState, len(vm.Org.Objects))
	for name, state := range vm.Org.Objects {
		org.Objects[name] = state
	}
	object.Records = records
	object.Indexes = nil
	object.RecordsShared, object.IndexesShared = false, false
	org.Objects[objectName] = object
	return &org
}

// A query view includes only its projection. Record conversion also serves DML
// and may hydrate lookups; those stored fields are not selected by SOQL.
func (vm *VM) applySOQLRecordProjection(value *Value, record storage.Record, query soql.Query) {
	if value == nil || value.Kind != ValueObject || query.Count || len(query.Aggregates) > 0 || len(query.GroupBy) > 0 {
		return
	}
	fields := map[string]bool{"id": true}
	var paths []string
	for _, field := range query.Fields {
		if strings.EqualFold(strings.TrimSpace(field), "FIELDS(STANDARD)") {
			if vm.Org != nil {
				if object, known := vm.Org.Objects[record.Object]; known {
					if selected, err := soql.ExpandFieldsFunctions(object.Definition, []string{field}); err == nil {
						paths = append(paths, selected...)
						continue
					}
				}
			}
			for selected := range record.Fields {
				paths = append(paths, selected)
			}
		} else if strings.Contains(field, "(") {
			paths = append(paths, selectedSOQLFunctionFields(field)...)
		} else {
			paths = append(paths, field)
		}
	}
	for _, spec := range query.Typeofs {
		fields[strings.ToLower(spec.Relationship)] = true
		if lookup, ok := vm.parentRelationshipField(record.Object, spec.Relationship); ok {
			fields[strings.ToLower(lookup)] = true
		}
		actual, parent, ok := objectFieldValue(*value, spec.Relationship)
		if !ok || parent.Kind != ValueObject {
			continue
		}
		vm.resolveSOQLParentRuntimeType(&parent, record.Object, spec.Relationship)
		value.Fields[actual] = parent
		selected := spec.Else
		for name, branch := range spec.When {
			if strings.EqualFold(name, parent.Type) {
				selected = branch
				break
			}
		}
		paths = append(paths, spec.Relationship+".Id")
		for _, field := range selected {
			paths = append(paths, spec.Relationship+"."+field)
		}
	}
	for _, field := range paths {
		if strings.Contains(field, "(") {
			for _, selected := range selectedSOQLFunctionFields(field) {
				vm.addQueriedSObjectField(fields, record.Object, selected)
			}
			continue
		}
		vm.addQueriedSObjectField(fields, record.Object, field)
	}
	for _, child := range query.ChildQueries {
		fields[strings.ToLower(child.Relationship)] = true
		actual, rows, ok := objectFieldValue(*value, child.Relationship)
		if !ok || rows.Kind != ValueList {
			continue
		}
		for i := range rows.List {
			for relationship, records := range record.Children {
				if strings.EqualFold(relationship, child.Relationship) && i < len(records) {
					vm.applySOQLRecordProjection(&rows.List[i], records[i], child.Query)
					break
				}
			}
		}
		value.Fields[actual] = rows
	}
	value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Object, fields)
	for field := range value.Fields {
		if isInternalSObjectField(field) || vm.queriedSObjectFieldsIncludes(*value, field) {
			continue
		}
		// Restrict cleanup to lookup hydration; ordinary fields already come
		// from the executor's projection, including FIELDS and function aliases.
		if _, definition, ok := vm.sObjectFieldDefinition(record.Object, field); ok && definition.Type == storage.FieldReference {
			delete(value.Fields, field)
		}
	}
	vm.markSOQLLoadedReferences(value, record)
	for _, field := range paths {
		if strings.Contains(field, "(") {
			continue
		}
		parts := splitSOQLRelationshipFieldPath(field)
		if len(parts) > 1 && !vm.soqlFieldQualifierMatchesObject(record.Object, parts[0]) {
			vm.markQueriedParentRelationshipPath(value, record.Object, parts)
		}
	}
}

func (vm *VM) resolveSOQLParentRuntimeType(parent *Value, objectName, relationship string) {
	relation, ok := vm.parentRelationshipMetadata(objectName, relationship)
	if !ok || !relation.Polymorphic || parent == nil || parent.Kind != ValueObject {
		return
	}
	id := sObjectIDFromFields(parent.Fields)
	if id == "" {
		return
	}
	for _, target := range relation.ParentObjects {
		if canonical, ok := vm.resolveObjectName(target); ok {
			target = canonical
		}
		if _, ok := vm.findOrgRecord(target, id); ok {
			parent.Type = target
			return
		}
	}
}

func soqlQueryException(err *soql.QueryError, queryText string) error {
	if err.Uncatchable {
		return &RuntimeError{Type: "System.UnexpectedException", Message: err.Message}
	}
	// S001/S002 retain native getMessage() text. Uncaught runtime diagnostics
	// still need the generated query (including the Apex REST error path).
	return newExceptionErrorWithContext("QueryException", err.Message, fmt.Sprintf("generated SOQL %q", queryText))
}

func (vm *VM) aggregateResultFromRecord(record storage.Record, query soql.Query) Value {
	// R175/R184 and N017/N018: exprN names exist only for unnamed
	// aggregates. Internal ordinal keys remain available to SOQL HAVING/ORDER BY.
	fields := make(map[string]storage.Value, len(record.Fields))
	for name, value := range record.Fields {
		fields[name] = value
	}
	for i := range query.Aggregates {
		delete(fields, fmt.Sprintf("expr%d", i))
	}
	unnamed := 0
	for i, aggregate := range query.Aggregates {
		name := aggregate.Alias
		if name == "" {
			name = fmt.Sprintf("expr%d", unnamed)
			unnamed++
		}
		fields[name] = record.Fields[fmt.Sprintf("expr%d", i)]
	}
	out := Object("AggregateResult")
	for name, value := range fields {
		fieldValue := vmValueFromStorage(value)
		// R127/R192/N019/Q001-Q003: get() returns Object. Preserve the
		// runtime payload and use the existing boxed formatting/cast paths.
		fieldValue.Static = "Object"
		out.Fields[name] = fieldValue
	}
	return out
}

// Selection checks belong to the shared Apex query route. The underlying
// SOQL executor also serves callers that support unbounded FIELDS expansion.
func (vm *VM) validateSOQLSelection(query soql.Query, beforeExpansion bool) error {
	if strings.EqualFold(query.SecurityMode, "SECURITY_ENFORCED") && apexversion.Enabled(vm.currentMethod.APIVersion, apexversion.SecureDefaults) {
		return newExceptionError("QueryException", "WITH SECURITY_ENFORCED is no longer supported, use WITH USER_MODE instead.")
	}
	if query.ForUpdate && (len(query.Order) > 0 || query.OrderBy != "") {
		return newExceptionError("QueryException", "Explicit ORDER BY not allowed when locking rows (Id order is implied)")
	}
	if query.ForUpdate && query.HasOffset {
		return newExceptionError("QueryException", "FOR UPDATE cannot be used in conjunction with the OFFSET clause")
	}
	if query.Offset > 2000 {
		return newExceptionError("QueryException", "Maximum SOQL offset allowed for apiName "+query.Object+" is 2000")
	}
	if vm.Org != nil {
		if objectName, ok := storage.ResolveObjectName(*vm.Org, query.Object); ok {
			definition := vm.Org.Objects[objectName].Definition
			for _, order := range query.Order {
				name, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, order.Field)
				if !ok {
					continue
				}
				field := definition.Fields[name]
				if field.Sortable != nil && !*field.Sortable {
					return &RuntimeError{Type: "System.UnexpectedException", Message: "field '" + name + "' can not be sorted in a query call"}
				}
			}
		}
	}
	seen := map[string]bool{}
	for _, field := range query.Fields {
		upper := strings.ToUpper(strings.TrimSpace(field))
		if upper == "FIELDS(ALL)" || upper == "FIELDS(CUSTOM)" {
			return newExceptionError("QueryException", "The SOQL FIELDS function is not supported with an unbounded set of fields in this API.")
		}
		if strings.HasPrefix(upper, "FIELDS(") && strings.HasSuffix(upper, ")") {
			if seen[upper] {
				return newExceptionError("QueryException", "Invalid field: '"+upper[len("FIELDS("):len(upper)-1]+"'")
			}
			seen[upper] = true
		}
	}
	var check func(*soql.Condition) error
	check = func(condition *soql.Condition) error {
		if condition == nil {
			return nil
		}
		for i := range condition.And {
			child := &condition.And[i]
			if condition.SourceAnd && child.Not && !child.Parenthesized {
				return newExceptionError("QueryException", "unexpected token: 'NOT'")
			}
			if err := check(child); err != nil {
				return err
			}
		}
		for i := range condition.Or {
			child := &condition.Or[i]
			if child.SourceAnd && !child.Parenthesized {
				return newExceptionError("QueryException", "unexpected token: 'OR'")
			}
			if err := check(child); err != nil {
				return err
			}
		}
		if condition.Subquery != nil {
			return vm.validateSOQLSelection(*condition.Subquery, beforeExpansion)
		}
		if beforeExpansion {
			if (condition.Op == "IN" || condition.Op == "NOT IN") && len(condition.Values) == 0 {
				return newExceptionError("QueryException", "unexpected token: '"+condition.Field+" "+condition.Op+" ()'")
			}
			if condition.Op == "LIKE" && condition.Value.Kind == storage.ValueString && condition.Value.String == "" {
				return newExceptionError("QueryException", "invalid LIKE value: ")
			}
			if err := vm.validateSOQLSelectionLiteral(query.Object, condition); err != nil {
				return err
			}
		}
		return nil
	}
	return check(query.Where)
}

func (vm *VM) validateSOQLSelectionLiteral(objectName string, condition *soql.Condition) error {
	if vm.Org == nil {
		return nil
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, objectName)
	if !ok {
		return nil
	}
	definition := vm.Org.Objects[objectName].Definition
	name, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, condition.Field)
	if !ok {
		return nil
	}
	field := definition.Fields[name]
	values := []storage.Value{condition.Value}
	if condition.Op == "IN" || condition.Op == "NOT IN" {
		values = condition.Values
	}
	for _, value := range values {
		if value.Kind == storage.ValueDecimal && strings.ContainsAny(value.Decimal, "eE") {
			return newExceptionError("QueryException", "missing value at '<EOF>'")
		}
		if field.Type == storage.FieldDecimal && value.Kind == storage.ValueString {
			return newExceptionError("QueryException", "value of filter criterion for field '"+condition.Field+"' must be of type double and should not be enclosed in quotes")
		}
		if field.Type == storage.FieldDate && value.Kind == storage.ValueID {
			text := string(value.ID)
			if len(text) == 10 && text[4] == '-' && text[7] == '-' {
				if _, err := time.Parse("2006-01-02", text); err != nil {
					return newExceptionError("QueryException", "Invalid date: "+text)
				}
			}
		}
	}
	return nil
}

// Dynamic binds retain their value types until the query has been checked.
// Serializing first would turn incompatible values into harmless zero matches.
func (vm *VM) validateDynamicSOQLBinds(raw string, binds Value) error {
	query, err := vm.parseSOQLAt(raw)
	if err != nil {
		return nil // The execution parser supplies the syntax diagnostic.
	}
	lookup := vm.lookup
	if binds.Kind == ValueMap {
		lookup = func(name string) (Value, error) { return soqlBindMapValue(binds, name) }
	}
	window := func(name, clause string) error {
		if name == "" {
			return nil
		}
		value, err := lookup(name)
		if err != nil {
			return nil // Expression binds are evaluated by the expander.
		}
		if value.Kind == ValueNull {
			return newExceptionError("NullPointerException", "Attempt to de-reference a null object")
		}
		if value.Kind != ValueInt {
			label := "Limit"
			if clause == "OFFSET" {
				label = "Offset"
			}
			return newExceptionError("QueryException", label+" expression must be of type Integer")
		}
		return nil
	}
	if err := window(query.LimitBind, "LIMIT"); err != nil {
		return err
	}
	if err := window(query.OffsetBind, "OFFSET"); err != nil {
		return err
	}
	var check func(soql.Query, *soql.Condition) error
	check = func(query soql.Query, condition *soql.Condition) error {
		if condition == nil {
			return nil
		}
		for i := range condition.And {
			if err := check(query, &condition.And[i]); err != nil {
				return err
			}
		}
		for i := range condition.Or {
			if err := check(query, &condition.Or[i]); err != nil {
				return err
			}
		}
		if condition.Subquery != nil {
			return check(*condition.Subquery, condition.Subquery.Where)
		}
		values := []storage.Value{condition.Value}
		collection := condition.Op == "IN" || condition.Op == "NOT IN"
		if collection {
			values = condition.Values
		}
		for _, bind := range values {
			if bind.Kind != storage.ValueID || !strings.HasPrefix(string(bind.ID), ":") {
				continue
			}
			name := string(bind.ID)[1:]
			value, err := lookup(name)
			if err != nil {
				continue // Preserve expression lookup and missing-name diagnostics.
			}
			if value.Kind == ValueNull {
				if collection {
					return newExceptionError("NullPointerException", "Attempt to de-reference a null object")
				}
				continue
			}
			if vm.Org == nil {
				continue
			}
			objectName, ok := storage.ResolveObjectName(*vm.Org, query.Object)
			if !ok {
				continue
			}
			definition := vm.Org.Objects[objectName].Definition
			fieldName, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, condition.Field)
			if !ok {
				continue
			}
			fieldType := definition.Fields[fieldName].Type
			// Native R023-R028 and collection-view R005/R006 capture nonempty Object
			// containers used with IN. Retain the concrete collection type
			// behind an erased Iterable/Object view; scalar Object stays valid.
			if condition.Op == "IN" && fieldType == storage.FieldString &&
				(value.Kind == ValueList || value.Kind == ValueSet) && (len(value.List) != 0 || len(value.Set) != 0) {
				bindType := value.Type
				if base := collectionBase(value.Runtime); base == "List" || base == "Set" {
					bindType = value.Runtime
				}
				if element, typed := collectionElementType(bindType); typed && strings.EqualFold(canonicalRuntimePlatformType(element), "Object") {
					return newExceptionError("QueryException", "Invalid bind expression type of ANY for column of type String")
				}
			}
			items := []Value{value}
			if value.Kind == ValueList {
				items = value.List
			} else if value.Kind == ValueSet {
				items = value.Set
			}
			for _, item := range items {
				if item.Kind == ValueNull {
					continue
				}
				columnType := ""
				switch fieldType {
				case storage.FieldDecimal:
					if item.Kind != ValueInt && item.Kind != ValueDecimal {
						columnType = "Decimal"
					}
				case storage.FieldString:
					// Native R001-R004/R015-R021 accept typed Ids to text.
					boxedID := item.Kind == ValueObject && strings.EqualFold(item.Type, "Id")
					if item.Kind != ValueString && !boxedID {
						columnType = "String"
					}
				}
				if columnType != "" {
					return newExceptionError("QueryException", "Invalid bind expression type of "+valueTypeName(item)+" for column of type "+columnType)
				}
			}
		}
		return nil
	}
	return check(query, query.Where)
}

func soqlBindMapValue(binds Value, name string) (Value, error) {
	if value, ok := binds.Map[mapKey(String(name))]; ok {
		return value, nil
	}
	for _, key := range orderedValueMapKeys(binds) {
		if strings.EqualFold(mapStoredKey(binds, key).Text, name) {
			return binds.Map[key], nil
		}
	}
	return Null, newExceptionError("QueryException", "Key '"+name+"' does not exist in the bindMap")
}

func (vm *VM) soqlExecutionCacheForOrg(org *storage.OrgState) *soql.ExecutionCache {
	if vm == nil || org == nil || vm.Org == nil || org != vm.Org {
		return nil
	}
	if vm.soqlExecutionCache == nil {
		vm.soqlExecutionCache = soql.NewExecutionCache()
	}
	return vm.soqlExecutionCache
}

func (vm *VM) soqlCountsQueryLimit(query soql.Query) bool {
	if vm == nil || vm.Org == nil {
		return true
	}
	if vm.triggerDepth > 0 {
		return false
	}
	if !storage.IsCustomMetadataObject(*vm.Org, query.Object) {
		return true
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, query.Object)
	if !ok {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	for _, selected := range query.Fields {
		fieldName, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, selected)
		if !ok {
			continue
		}
		field := definition.Fields[fieldName]
		// Metadata loading represents LongTextArea as TEXTAREA with its length.
		if strings.EqualFold(field.DisplayType, "TEXTAREA") && field.Length > 255 {
			return true
		}
	}
	return false
}

func (vm *VM) recordRecentlyViewedRows(queryObject string, records []storage.Record, markViewed bool, markReferenced bool) {
	if vm == nil || len(records) == 0 {
		return
	}
	if vm.recentlyViewed == nil {
		vm.recentlyViewed = make(map[string]map[storage.ID]recentlyViewedEntry)
	}
	userID := vm.currentUserInfoField("Id", "005000000000001")
	if strings.TrimSpace(userID) == "" {
		userID = "005000000000001"
	}
	views := vm.recentlyViewed[userID]
	if views == nil {
		views = make(map[storage.ID]recentlyViewedEntry)
		vm.recentlyViewed[userID] = views
	}
	viewedAt := vm.fakeNow.UTC().Format(time.RFC3339)
	for _, record := range records {
		id := record.ID
		if id == "" {
			continue
		}
		objectName := record.Object
		if strings.TrimSpace(objectName) == "" {
			objectName = queryObject
		}
		if resolved, ok := vm.resolveObjectName(objectName); ok {
			objectName = resolved
		}
		entry := views[id]
		entry.ID = id
		entry.ObjectName = objectName
		entry.Name = recentlyViewedName(record, id)
		if markViewed {
			entry.ViewedAt = viewedAt
		}
		if markReferenced {
			entry.ReferencedAt = viewedAt
		}
		views[id] = entry
	}
}

func recentlyViewedName(record storage.Record, id storage.ID) string {
	if value, ok := record.GetField("Name"); ok {
		if text := storageValueText(value); strings.TrimSpace(text) != "" {
			return text
		}
	}
	return string(id)
}

func storageValueText(value storage.Value) string {
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueInteger:
		return strconv.FormatInt(value.Integer, 10)
	case storage.ValueBoolean:
		if value.Boolean {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func (vm *VM) orgWithSyntheticRecentlyViewed() storage.OrgState {
	org := cloneRuntimeOrgState(*vm.Org)
	storage.EnsureStandardObject(&org, "RecentlyViewed")
	object := org.Objects["RecentlyViewed"]
	object.Records = make(map[storage.ID]storage.Record)
	userID := vm.currentUserInfoField("Id", "005000000000001")
	views := vm.recentlyViewed[userID]
	ids := make([]string, 0, len(views))
	for id := range views {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, idText := range ids {
		id := storage.ID(idText)
		view := views[id]
		lastViewedDate := storage.NullValue()
		if strings.TrimSpace(view.ViewedAt) != "" {
			lastViewedDate = storage.DateTimeValue(view.ViewedAt)
		}
		lastReferencedDate := storage.NullValue()
		if strings.TrimSpace(view.ReferencedAt) != "" {
			lastReferencedDate = storage.DateTimeValue(view.ReferencedAt)
		}
		object.Records[id] = storage.Record{
			ID:     id,
			Object: "RecentlyViewed",
			Fields: map[string]storage.Value{
				"Name":               storage.StringValue(view.Name),
				"Type":               storage.StringValue(localSchemaName(view.ObjectName)),
				"LastViewedDate":     lastViewedDate,
				"LastReferencedDate": lastReferencedDate,
			},
		}
	}
	org.Objects["RecentlyViewed"] = object
	return org
}

// orgWithSyntheticFlowMetadata exposes the Flow metadata already parsed into
// the local org through the standard FlowDefinitionView and FlowVariableView
// query surfaces. Salesforce exposes these rows even though they are not
// ordinary mutable records; keeping them query-backed lets Apex that discovers
// and invokes an autolaunched Flow follow the same local execution path.
func (vm *VM) orgWithSyntheticFlowMetadata() storage.OrgState {
	org := cloneRuntimeOrgState(*vm.Org)
	storage.EnsureStandardObject(&org, "FlowDefinitionView")
	storage.EnsureStandardObject(&org, "FlowVariableView")
	definitions := org.Objects["FlowDefinitionView"]
	variables := org.Objects["FlowVariableView"]
	definitions.Records = make(map[storage.ID]storage.Record)
	variables.Records = make(map[storage.ID]storage.Record)
	rules := append([]storage.FlowRule(nil), vm.Org.Metadata.Flows...)
	sort.Slice(rules, func(i, j int) bool { return strings.ToLower(rules[i].Name) < strings.ToLower(rules[j].Name) })
	for index, rule := range rules {
		if strings.TrimSpace(rule.Name) == "" {
			continue
		}
		definitionID := storage.ID(fmt.Sprintf("3FD%012d", index+1))
		versionID := storage.ID(fmt.Sprintf("3FV%012d", index+1))
		definitions.Records[definitionID] = storage.Record{
			ID:     definitionID,
			Object: "FlowDefinitionView",
			Fields: map[string]storage.Value{
				"ApiName":         storage.StringValue(rule.Name),
				"ActiveVersionId": storage.IDValue(versionID),
				"LatestVersionId": storage.IDValue(versionID),
				"IsActive":        storage.BooleanValue(rule.Active),
				"Label":           storage.StringValue(rule.Name),
				"ProcessType":     storage.StringValue(rule.ProcessType),
				"TriggerType":     storage.StringValue(rule.TriggerType),
			},
		}
		for variableIndex, variable := range rule.Variables {
			if strings.TrimSpace(variable.Name) == "" {
				continue
			}
			variableID := storage.ID(fmt.Sprintf("3FW%09d%03d", index+1, variableIndex+1))
			variables.Records[variableID] = storage.Record{
				ID:     variableID,
				Object: "FlowVariableView",
				Fields: map[string]storage.Value{
					"ApiName":           storage.StringValue(variable.Name),
					"DataType":          storage.StringValue(variable.DataType),
					"Description":       storage.StringValue(variable.Description),
					"FlowVersionViewId": storage.IDValue(versionID),
					"IsCollection":      storage.BooleanValue(variable.IsCollection),
					"IsInput":           storage.BooleanValue(variable.IsInput),
					"IsOutput":          storage.BooleanValue(variable.IsOutput),
					"ObjectType":        storage.StringValue(variable.ObjectType),
				},
			}
		}
	}
	org.Objects["FlowDefinitionView"] = definitions
	org.Objects["FlowVariableView"] = variables
	return org
}

func (vm *VM) orgWithSyntheticUserRecordAccess() storage.OrgState {
	org := cloneRuntimeOrgState(*vm.Org)
	storage.EnsureStandardObject(&org, "UserRecordAccess")
	accessObject := org.Objects["UserRecordAccess"]
	accessObject.Records = make(map[storage.ID]storage.Record)
	userID := vm.currentUserInfoField("Id", "005-local-user")
	objectNames := make([]string, 0, len(vm.Org.Objects))
	for objectName := range vm.Org.Objects {
		if strings.EqualFold(objectName, "UserRecordAccess") {
			continue
		}
		objectNames = append(objectNames, objectName)
	}
	sort.Strings(objectNames)
	sequence := 1
	for _, objectName := range objectNames {
		object := vm.Org.Objects[objectName]
		recordIDs := make([]string, 0, len(object.Records))
		for id, record := range object.Records {
			recordID := record.ID
			if recordID == "" {
				recordID = id
			}
			if recordID == "" || record.System.IsDeleted {
				continue
			}
			text := string(recordID)
			recordIDs = append(recordIDs, text)
		}
		sort.Strings(recordIDs)
		for _, recordIDText := range recordIDs {
			recordID := storage.ID(recordIDText)
			_, record, _ := storage.LookupRecordByID(object.Records, recordID)
			read := vm.currentUserObjectPermission(objectName, "isAccessible") && vm.userModeRecordVisible(objectName, record, userID)
			edit := vm.currentUserCanWriteRecord(objectName, record, userID, "update") && vm.currentUserObjectPermission(objectName, "isUpdateable")
			remove := vm.currentUserCanWriteRecord(objectName, record, userID, "delete") && vm.currentUserObjectPermission(objectName, "isDeletable")
			all := read && edit && remove
			level := "None"
			if read {
				level = "Read"
			}
			if edit {
				level = "Edit"
			}
			if all {
				level = "All"
			}
			accessID := storage.ID(fmt.Sprintf("0UR%012d", sequence))
			sequence++
			accessObject.Records[accessID] = storage.Record{
				ID:     accessID,
				Object: "UserRecordAccess",
				Fields: map[string]storage.Value{
					"RecordId":          storage.IDValue(recordID),
					"UserId":            storage.IDValue(storage.ID(userID)),
					"HasReadAccess":     storage.BooleanValue(read),
					"HasEditAccess":     storage.BooleanValue(edit),
					"HasDeleteAccess":   storage.BooleanValue(remove),
					"HasTransferAccess": storage.BooleanValue(vm.currentUserCanWriteRecord(objectName, record, userID, "transfer")),
					"HasAllAccess":      storage.BooleanValue(all),
					"MaxAccessLevel":    storage.StringValue(level),
				},
			}
		}
	}
	org.Objects["UserRecordAccess"] = accessObject
	return org
}

func (vm *VM) executeSoqlStub(query soql.Query, queryText string, binds Value, execResult *Result) ([]Value, bool, error) {
	if vm.testContext == nil || len(vm.testContext.SoqlStubs) == 0 {
		return nil, false, nil
	}
	objectName := query.Object
	if vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(query.Object); ok {
			objectName = resolved
		}
	}
	provider, ok := vm.testContext.SoqlStubs[strings.ToLower(objectName)]
	if !ok {
		return nil, false, nil
	}
	if query.Count || len(query.Aggregates) > 0 || len(query.HavingAggregates) > 0 || len(query.GroupBy) > 0 || query.Having != nil {
		return nil, true, unsupportedCallError("Test.createSoqlStub aggregate query local stub surface")
	}
	if provider.Kind != ValueObject {
		return nil, true, fmt.Errorf("Test.createSoqlStub provider for %s is not an object", query.Object)
	}
	if binds.Kind == "" || binds.Kind == ValueNull {
		binds = typedMap("Map<String,Object>")
	}
	args := []Value{sObjectTypeToken(objectName), String(queryText), binds}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(provider.Type, "handleSoqlQuery", args)
	if ambiguous {
		return nil, true, vm.ambiguousOverloadError(provider.Type+".handleSoqlQuery", args)
	}
	if !ok {
		return nil, true, fmt.Errorf("SoqlStubProvider %s must implement handleSoqlQuery", provider.Type)
	}
	value, err := vm.callMethodWithReceiver(target, provider, args, execResult)
	if err != nil {
		return nil, true, err
	}
	if value.Kind != ValueList {
		return nil, true, fmt.Errorf("SoqlStubProvider %s handleSoqlQuery must return List<SObject>", provider.Type)
	}
	rows := append([]Value(nil), value.List...)
	appendTraceLazy(execResult, "apex.soql.stub", "apex.soql", func() map[string]any {
		return vm.traceSOQLArgs(queryText, objectName, len(rows))
	})
	return rows, true, nil
}

func (vm *VM) enforceOrgShapeObjectAvailability(query soql.Query) error {
	if vm == nil || vm.Org == nil {
		return nil
	}
	objectName := query.Object
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		objectName = resolved
	}
	if strings.EqualFold(objectName, "DatedConversionRate") && !vm.orgBool("Organization", "IsMultiCurrencyEnabled", false) {
		return newExceptionError("QueryException", "sObject type 'DatedConversionRate' is not supported")
	}
	for _, child := range query.ChildQueries {
		if err := vm.enforceOrgShapeObjectAvailability(child.Query); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) testSetFixedSearchResults(args []Value) (Value, error) {
	// Context validation also precedes a null list.
	if err := vm.requireTestContext("Test.setFixedSearchResults"); err != nil {
		return Null, newExceptionError("System.StringException", "Test.setFixedSearchResults() can only be called from testMethods")
	}
	if len(args) != 1 || args[0].Kind != ValueList && args[0].Kind != ValueNull {
		return Null, fmt.Errorf("Test.setFixedSearchResults expects List<Id>")
	}
	vm.fixedSearchResults = append([]Value(nil), args[0].List...)
	vm.fixedSearchResultsSet = true
	return Null, nil
}

func (vm *VM) testSetCreatedDate(args []Value) (Value, error) {
	if err := vm.requireTestContext("Test.setCreatedDate"); err != nil {
		return Null, err
	}
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.setCreatedDate expects record Id and Datetime")
	}
	idText, ok := idTextFromValue(args[0])
	if !ok || idText == "" {
		return Null, fmt.Errorf("Test.setCreatedDate expects record Id")
	}
	createdDate, err := platformScalarText(args[1], "Datetime")
	if err != nil {
		return Null, fmt.Errorf("Test.setCreatedDate expects Datetime")
	}
	if vm.Org == nil {
		return Null, fmt.Errorf("Test.setCreatedDate requires org state")
	}
	id := storage.ID(idText)
	for objectName := range vm.Org.Objects {
		object := vm.Org.Objects[objectName]
		storedID, record, ok := storage.LookupRecordByID(object.Records, id)
		if !ok {
			continue
		}
		if mutable, _ := storage.EnsureMutableObjectRecords(vm.Org, objectName); mutable != nil {
			object = *mutable
		}
		vm.recordIsolationJournalMutation(objectName, storedID, record, true)
		record.System.CreatedDate = createdDate
		object.Records[storedID] = record
		vm.Org.Objects[objectName] = object
		return Null, nil
	}
	return Null, newExceptionError("DmlException", "record not found: "+idText)
}

func (vm *VM) searchQuery(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("Search.query expects query String and optional access level")
	}
	if args[0].Kind != ValueString {
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("NullPointerException", "Argument 1 cannot be null")
		}
		return Null, fmt.Errorf("Search.query expects query String")
	}
	if len(args) == 2 && args[1].Kind != ValueNull && !isDatabaseAccessLevelValue(args[1]) {
		return Null, fmt.Errorf("Search.query expects AccessLevel")
	}
	accessLevel := Null
	if len(args) == 2 {
		if args[1].Kind == ValueNull {
			return Null, newExceptionError("NullPointerException", "Argument 2 cannot be null")
		}
		accessLevel = args[1]
	}
	return vm.executeSOSLWithAccessLevel(args[0].Text, nil, accessLevel)
}

func (vm *VM) searchFind(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("Search.find expects query String and optional access level")
	}
	if args[0].Kind != ValueString {
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("NullPointerException", "Argument 1 cannot be null")
		}
		return Null, fmt.Errorf("Search.find expects query String")
	}
	if len(args) == 2 && args[1].Kind != ValueNull && !isDatabaseAccessLevelValue(args[1]) {
		return Null, fmt.Errorf("Search.find expects AccessLevel")
	}
	accessLevel := Null
	if len(args) == 2 {
		if args[1].Kind == ValueNull {
			return Null, newExceptionError("NullPointerException", "Argument 2 cannot be null")
		}
		accessLevel = args[1]
	}
	if !isDatabaseAccessLevelValue(accessLevel) {
		accessLevel = vm.defaultAccessLevel()
	}
	if err := vm.incrementLimit("soslQueries", 1); err != nil {
		return Null, err
	}
	_, query, err := vm.parseSOSLQuery(args[0].Text)
	if err != nil {
		return Null, err
	}
	if query.AccessMode != "" {
		accessLevel = soslAccessLevel(query.AccessMode)
	}
	groups, err := vm.executeSOSLQuery(query, accessLevel)
	if err != nil {
		return Null, err
	}
	results := Object("Search.SearchResults")
	byObject := typedMap("Map<String,List<Search.SearchResult>>")
	for _, group := range groups {
		key := mapKey(String(group.ObjectName))
		list := byObject.Map[key]
		if list.Kind != ValueList {
			list = typedList("List<Search.SearchResult>")
		}
		for _, value := range group.Rows.List {
			record := storage.Record{}
			if value.Kind == ValueObject {
				if id, ok := valueIDString(value.Fields["Id"]); ok {
					if found, foundOK := vm.findOrgRecord(group.ObjectName, storage.ID(id)); foundOK {
						record = found
					}
				}
			}
			row := Object("Search.SearchResult")
			row.Fields["sObject"] = value
			snippet, snippets := soslSnippetsForRecord(record, query.WithSnippet, query.Terms)
			row.Fields["snippet"] = String(snippet)
			row.Fields["snippets"] = snippets
			list.List = append(list.List, row)
		}
		byObject.Map[key] = list
		byObject.MapKeys[key] = String(group.ObjectName)
	}
	results.Fields["results"] = byObject
	return results, nil
}

func (vm *VM) searchSuggest(args []Value) (Value, error) {
	if len(args) != 3 && len(args) != 4 {
		return Null, fmt.Errorf("Search.suggest expects query, sObjectType, options, and optional access level")
	}
	if args[0].Kind != ValueString || args[1].Kind != ValueString {
		for i := 0; i < 2; i++ {
			if args[i].Kind == ValueNull {
				return Null, newExceptionError("NullPointerException", fmt.Sprintf("Argument %d cannot be null", i+1))
			}
		}
		return Null, fmt.Errorf("Search.suggest expects query and sObjectType Strings")
	}
	if args[2].Kind != ValueNull && !isSearchSuggestionOptionValue(args[2]) {
		return Null, newExceptionError("TypeException", "Third parameters should be an object of type Search.SuggestionOption")
	}
	if len(args) == 4 && args[3].Kind != ValueNull && !isDatabaseAccessLevelValue(args[3]) {
		return Null, fmt.Errorf("Search.suggest expects AccessLevel")
	}
	accessLevel := Null
	if len(args) == 4 {
		if args[3].Kind == ValueNull {
			return Null, newExceptionError("NullPointerException", "Argument 4 cannot be null")
		}
		accessLevel = args[3]
	}
	if len([]rune(strings.TrimSpace(args[0].Text))) < 3 {
		return Null, newExceptionError("SearchException", "Your search term must have 3 or more characters")
	}
	if strings.TrimSpace(args[1].Text) == "" {
		return Null, newExceptionError("SearchException", "sobject has an invalid value")
	}
	if vm.Org != nil {
		if _, ok := vm.resolveObjectName(args[1].Text); !ok {
			return Null, newExceptionError("SearchException", "sobject has an invalid value")
		}
	} else if _, ok := storage.ResolveKnownStandardObjectName(args[1].Text); !ok {
		return Null, newExceptionError("SearchException", "sobject has an invalid value")
	}
	if value, ok := args[2].Fields["limit"]; ok && value.Kind == ValueInt && value.Int < 1 {
		return Null, newExceptionError("SearchException", "limit must be greater than or equal to 1")
	}
	results := Object("Search.SuggestionResults")
	suggestions, err := vm.searchSuggestionRows(args[0].Text, args[1].Text, args[2], accessLevel)
	if err != nil {
		return Null, err
	}
	results.Fields["suggestionResults"] = suggestions
	results.Fields["hasMoreResults"] = Bool(false)
	return results, nil
}

func isSearchSuggestionOptionValue(value Value) bool {
	return value.Kind == ValueObject && strings.EqualFold(value.Type, "Search.SuggestionOption")
}

func (vm *VM) soslParseError(err error) error {
	var contract *sosl.ContractError
	if errors.As(err, &contract) {
		return newExceptionError(contract.Type, contract.Message)
	}
	var unsupported *sosl.UnsupportedFeatureError
	if errors.As(err, &unsupported) {
		return &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
	}
	return newExceptionError("QueryException", err.Error())
}

func validateSOSLRuntimeFeatures(query sosl.Query) error {
	if query.UpdateViewStat || query.UpdateTracking {
		option := "VIEWSTAT"
		if !query.UpdateViewStat {
			option = "TRACKING"
		}
		for _, spec := range query.Returning {
			if !strings.EqualFold(spec.Object, "KnowledgeArticle") && !strings.EqualFold(spec.Object, "KnowledgeArticleVersion") {
				return &sosl.ContractError{Type: "QueryException", Message: "Only KnowledgeArticle and KnowledgeArticleVersion support UPDATE " + option + "."}
			}
		}
		return &sosl.UnsupportedFeatureError{Message: "SOSL UPDATE " + option + " hosted search analytics"}
	}
	if query.DivisionSpecified {
		return &sosl.ContractError{Type: "QueryException", Message: "divisions are not enabled"}
	}
	if query.NetworkNull {
		return &sosl.ContractError{Type: "QueryException", Message: "NULL cannot be used for WITH NETWORK clause"}
	}
	if query.NetworkSpecified {
		return &sosl.UnsupportedFeatureError{Message: "SOSL WITH NETWORK hosted search service"}
	}
	if query.DataCategoryGroup != "" {
		for _, spec := range query.Returning {
			if !strings.EqualFold(spec.Object, "KnowledgeArticle") && !strings.EqualFold(spec.Object, "KnowledgeArticleVersion") {
				return &sosl.ContractError{Type: "SearchException", Message: "The category group name is not valid for the entities you are searching: " + strings.TrimSuffix(query.DataCategoryGroup, "__c")}
			}
		}
		return &sosl.UnsupportedFeatureError{Message: "SOSL WITH DATA CATEGORY hosted search service"}
	}
	if query.PricebookID != "" {
		for _, spec := range query.Returning {
			if !strings.EqualFold(spec.Object, "Product2") {
				return &sosl.ContractError{Type: "SearchException", Message: "PricebookId filter can only be applied to 'Product2' search"}
			}
		}
	}
	return nil
}

func (vm *VM) executeSOSL(raw string, execResult *Result) (Value, error) {
	return vm.executeSOSLSource(raw, execResult, Null, true)
}

func (vm *VM) parseSOSLQuery(raw string) (string, sosl.Query, error) {
	return vm.parseSOSLQuerySource(raw, false)
}

func (vm *VM) parseSOSLQuerySource(raw string, inline bool) (string, sosl.Query, error) {
	original, originalErr := sosl.Parse(raw)
	if originalErr == nil && original.SearchBind != "" {
		value, lookupErr := vm.lookup(original.SearchBind)
		if lookupErr == nil && value.Kind == ValueNull {
			return "", sosl.Query{}, newExceptionError("NullPointerException", "Attempt to de-reference a null object")
		}
	}
	queryText, err := vm.expandSOSLBinds(raw)
	if err != nil {
		return "", sosl.Query{}, newExceptionError("QueryException", fmt.Sprintf("%s in query %q", err.Error(), raw))
	}
	query, err := sosl.Parse(queryText)
	if err != nil {
		return "", sosl.Query{}, vm.soslParseError(err)
	}
	// C011/N001/N002 accept inline negative literals only with empty results.
	// Bind values and dynamic Search.query/find keep their captured range check.
	if inline && originalErr == nil {
		for i, spec := range original.Returning {
			if i < len(query.Returning) && spec.Offset.HasValue && spec.Offset.Value < 0 {
				query.Returning[i].EmptyNegativeOffset = true
			}
		}
	}
	if len([]rune(query.SearchText)) < 2 || query.Expression != nil && query.Expression.Operator == "INVALID_SEARCH_TERM" {
		return "", sosl.Query{}, newExceptionError("SearchException", "search term must be longer than one character: "+query.SearchText)
	}
	switch query.Scope {
	case sosl.SearchScopeAll, sosl.SearchScopeName, sosl.SearchScopeEmail, sosl.SearchScopePhone, sosl.SearchScopeSidebar:
	default:
		return "", sosl.Query{}, newExceptionError("SearchException", "Invalid IN fields group: "+strings.TrimSuffix(string(query.Scope), " FIELDS"))
	}
	if err := vm.validateSOSLSelections(query); err != nil {
		return "", sosl.Query{}, err
	}
	if err := validateSOSLRuntimeFeatures(query); err != nil {
		return "", sosl.Query{}, vm.soslParseError(err)
	}
	return queryText, query, nil
}

func (vm *VM) executeSOSLWithAccessLevel(raw string, execResult *Result, accessLevel Value) (Value, error) {
	return vm.executeSOSLSource(raw, execResult, accessLevel, false)
}

func (vm *VM) executeSOSLSource(raw string, execResult *Result, accessLevel Value, inline bool) (Value, error) {
	if !isDatabaseAccessLevelValue(accessLevel) {
		accessLevel = vm.defaultAccessLevel()
	}
	if err := vm.incrementLimit("soslQueries", 1); err != nil {
		return Null, err
	}
	queryText, query, err := vm.parseSOSLQuerySource(raw, inline)
	if err != nil {
		return Null, err
	}
	if query.AccessMode != "" {
		accessLevel = soslAccessLevel(query.AccessMode)
	}
	groups, err := vm.executeSOSLQuery(query, accessLevel)
	if err != nil {
		return Null, err
	}
	rowCount := 0
	values := make([]Value, 0, len(groups))
	for _, group := range groups {
		rowCount += len(group.Rows.List)
		values = append(values, group.Rows)
	}
	appendTraceLazy(execResult, "apex.sosl", "apex.sosl", func() map[string]any {
		return map[string]any{
			"query": queryText,
			"rows":  rowCount,
		}
	})
	result := typedList("List<List<SObject>>")
	result.List = values
	return result, nil
}

func (vm *VM) executeSOSLQuery(query sosl.Query, accessLevel Value) ([]soslResultGroup, error) {
	if !isDatabaseAccessLevelValue(accessLevel) {
		accessLevel = vm.defaultAccessLevel()
	}
	groups := make([]soslResultGroup, 0, len(query.Returning))
	remaining := -1
	if query.Limit.HasValue {
		remaining = query.Limit.Value
	}
	for _, spec := range query.Returning {
		specObjectName := spec.Object
		if vm.Org != nil {
			if canonical, ok := vm.resolveObjectName(spec.Object); ok {
				specObjectName = canonical
			}
		}
		rows := List()
		rows.Type = "List<" + specObjectName + ">"
		if vm.Org != nil {
			matcherBoundary := query.MatchUnsupported
			if query.SpellCorrection != nil && *query.SpellCorrection {
				matcherBoundary = "SOSL WITH SPELL_CORRECTION local runtime"
			}
			records, err := vm.soslRecordsForSpec(spec, specObjectName, query.Terms, query.Expression, query.Scope, query.PricebookID, accessLevel, matcherBoundary)
			if err != nil {
				return nil, err
			}
			for _, record := range records {
				value := vm.vmValueFromRecord(record)
				vm.applySOSLReturningFunctionAliases(&value, record, spec)
				if len(spec.Fields) > 0 {
					value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Object, soslFieldSet(spec))
					vm.hydrateQueriedRecordTypeRelationships(value)
				}
				rows.List = append(rows.List, value)
			}
		}
		if len(rows.List) > 0 && spec.EmptyNegativeOffset {
			return nil, unsupportedCallError("SOSL inline negative OFFSET with nonempty results")
		}
		sortSOSLRows(rows, spec.OrderBy)
		applySOSLReturningOffset(&rows, spec)
		applySOSLReturningLimit(&rows, spec)
		if remaining >= 0 {
			if remaining < len(rows.List) {
				rows.List = rows.List[:remaining]
			}
			remaining -= len(rows.List)
		}
		// R141/R143/R144/R164 prove empty-result acceptance. Captured rows
		// do not establish the index's highlight or target-length formatting.
		if len(rows.List) > 0 && query.WithHighlight {
			return nil, unsupportedCallError("SOSL WITH HIGHLIGHT result formatting")
		}
		if len(rows.List) > 0 && query.SnippetTargetLength {
			return nil, unsupportedCallError("SOSL WITH SNIPPET TARGET_LENGTH result formatting")
		}
		groups = append(groups, soslResultGroup{ObjectName: specObjectName, Rows: rows})
	}
	return groups, nil
}

type soslWhere = sosl.Condition

type soslResultGroup struct {
	ObjectName string
	Rows       Value
}

func soslFieldSet(spec sosl.ReturningObject) map[string]bool {
	fields := make(map[string]bool, len(spec.Fields))
	for _, field := range spec.Fields {
		if field.Field != "" {
			fields[strings.ToLower(field.Field)] = true
		}
		if field.Alias != "" {
			fields[strings.ToLower(field.Alias)] = true
		}
	}
	return fields
}

func (vm *VM) soslRecordsForSpec(spec sosl.ReturningObject, objectName string, patterns []sosl.SearchTerm, expression *sosl.SearchExpression, scope sosl.SearchScope, pricebookID string, accessLevel Value, matcherBoundary string) ([]storage.Record, error) {
	if vm == nil || vm.Org == nil {
		return nil, nil
	}
	if !isDatabaseAccessLevelValue(accessLevel) {
		accessLevel = vm.defaultAccessLevel()
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(objectName)), "__x") {
		return nil, unsupportedCallError("Search.query SOSL external indexes")
	}
	state, ok := vm.Org.Objects[objectName]
	if !ok {
		return nil, nil
	}
	if err := vm.enforceSOSLAccess(objectName, spec, accessLevel); err != nil {
		return nil, err
	}
	var records []storage.Record
	if vm.fixedSearchResultsSet {
		// One result per record, in first-occurrence order.
		seen := make(map[storage.ID]bool)
		for _, idValue := range vm.fixedSearchResults {
			id, ok := valueIDString(idValue)
			if !ok {
				continue
			}
			foundObject, ok := vm.sObjectNameForIDPrefix(idPrefix(id))
			if !ok {
				foundObject, ok = vm.sObjectNameForExistingID(id)
			}
			if !ok || !strings.EqualFold(foundObject, objectName) {
				continue
			}
			record, ok := vm.findOrgRecord(objectName, storage.ID(id))
			if !ok {
				continue
			}
			recordID := record.ID
			if recordID == "" {
				recordID = storage.ID(id)
			}
			if seen[recordID] {
				continue
			}
			seen[recordID] = true
			matches, err := vm.soslRecordMatchesWhere(record, spec.Where)
			if (err == nil && !matches) || !vm.soslRecordMatchesPricebook(objectName, record, pricebookID) {
				continue
			}
			if vm.recordSharingApplies(accessLevel) && !vm.userModeRecordVisible(objectName, record, vm.currentUserID()) {
				continue
			}
			if err != nil {
				return nil, err
			}
			records = append(records, record)
		}
		return records, nil
	}
	// Apex test-context SOSL is empty unless the test explicitly supplies
	// Test.setFixedSearchResults. Keep the org-backed index for normal runtime
	// execution, but do not let inserted test data leak into a default search.
	if vm.testContext != nil {
		return nil, nil
	}
	ids := make([]string, 0, len(state.Records))
	for id := range state.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		record := state.Records[storage.ID(id)]
		matchesWhere, whereErr := vm.soslRecordMatchesWhere(record, spec.Where)
		if (whereErr == nil && !matchesWhere) || !vm.soslRecordMatchesPricebook(objectName, record, pricebookID) {
			continue
		}
		if matcherBoundary != "" {
			return nil, &RuntimeError{Type: "UnsupportedFeature", Message: matcherBoundary}
		}
		matched, err := vm.soslRecordMatchesExpression(objectName, record, patterns, expression, scope, accessLevel)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		if vm.recordSharingApplies(accessLevel) && !vm.userModeRecordVisible(objectName, record, vm.currentUserID()) {
			continue
		}
		if whereErr != nil {
			return nil, whereErr
		}
		records = append(records, record)
	}
	return records, nil
}

func (vm *VM) enforceSOSLAccess(objectName string, spec sosl.ReturningObject, accessLevel Value) error {
	if databaseAccessLevelSecurityMode(accessLevel) != "USER_MODE" {
		return nil
	}
	permissionSetID := accessLevelPermissionSetID(accessLevel)
	if !vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", permissionSetID) {
		return newExceptionError("QueryException", fmt.Sprintf("sObject type '%s' is not supported by USER_MODE", objectName))
	}
	for _, projection := range spec.Fields {
		field := projection.Field
		if strings.EqualFold(field, "Id") {
			continue
		}
		canonical := field
		if vm.Org != nil {
			if object, ok := vm.Org.Objects[objectName]; ok {
				if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
					canonical = resolved
				}
			}
		}
		if !vm.currentUserFieldPermissionWithScope(objectName, canonical, "isAccessible", permissionSetID) {
			return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s'.", canonical, objectName))
		}
	}
	return nil
}

func (vm *VM) soslRecordMatchesExpression(objectName string, record storage.Record, terms []sosl.SearchTerm, expression *sosl.SearchExpression, scope sosl.SearchScope, accessLevel Value) (bool, error) {
	if expression == nil {
		return vm.soslRecordMatchesSearch(objectName, record, terms, scope, accessLevel), nil
	}
	switch expression.Operator {
	case "TERM":
		if expression.Term.NeedsMatcher {
			return false, unsupportedCallError("SOSL wildcard index matching")
		}
		return vm.soslRecordMatchesSearchPattern(objectName, record, expression.Term, scope, accessLevel), nil
	case "AND", "OR", "AND NOT":
		left, err := vm.soslRecordMatchesExpression(objectName, record, terms, expression.Left, scope, accessLevel)
		if err != nil {
			return false, err
		}
		if (expression.Operator == "AND" || expression.Operator == "AND NOT") && !left {
			return false, nil
		}
		if expression.Operator == "OR" && left {
			return true, nil
		}
		right, err := vm.soslRecordMatchesExpression(objectName, record, terms, expression.Right, scope, accessLevel)
		if err != nil {
			return false, err
		}
		if expression.Operator == "AND NOT" {
			return !right, nil
		}
		return right, nil
	default:
		return false, unsupportedCallError("SOSL unknown search expression operator")
	}
}

func (vm *VM) soslRecordMatchesSearch(objectName string, record storage.Record, patterns []sosl.SearchTerm, scope sosl.SearchScope, accessLevel Value) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if vm.soslRecordMatchesSearchPattern(objectName, record, pattern, scope, accessLevel) {
			return true
		}
	}
	return false
}

func (vm *VM) soslRecordMatchesSearchPattern(objectName string, record storage.Record, pattern sosl.SearchTerm, scope sosl.SearchScope, accessLevel Value) bool {
	if pattern.Text == "" {
		return true
	}
	if scope == sosl.SearchScopeAll && soslTextMatchesPattern(string(record.ID), pattern) {
		return true
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	permissionSetID := accessLevelPermissionSetID(accessLevel)
	userMode := databaseAccessLevelSecurityMode(accessLevel) == "USER_MODE"
	fields := make([]string, 0, len(record.Fields))
	for field := range record.Fields {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		canonical, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field)
		if !ok {
			canonical = field
		}
		definition, hasDefinition := object.Definition.Fields[canonical]
		if !hasDefinition || !soslSearchableField(definition) || !soslFieldMatchesScope(canonical, definition, scope) {
			continue
		}
		if userMode && !vm.currentUserFieldPermissionWithScope(objectName, canonical, "isAccessible", permissionSetID) {
			continue
		}
		value, ok := record.GetField(canonical)
		if !ok {
			continue
		}
		if soslTextMatchesPattern(storageValueText(value), pattern) {
			return true
		}
	}
	return false
}

func soslSearchableField(field storage.Field) bool {
	switch field.Type {
	case storage.FieldID, storage.FieldString, storage.FieldPicklist, storage.FieldMultiPicklist, storage.FieldReference:
		return true
	default:
		return false
	}
}

func soslFieldMatchesScope(fieldName string, field storage.Field, scope sosl.SearchScope) bool {
	switch scope {
	case "", sosl.SearchScopeAll:
		return true
	case sosl.SearchScopeName:
		return soslIsNameField(fieldName, field)
	case sosl.SearchScopeEmail:
		return soslIsEmailField(fieldName, field)
	case sosl.SearchScopePhone:
		return soslIsPhoneField(fieldName, field)
	default:
		return true
	}
}

func soslIsNameField(fieldName string, field storage.Field) bool {
	name := strings.ToLower(strings.TrimSpace(fieldName))
	return name == "name" || name == "firstname" || name == "lastname" || name == "subject" || field.NamePointing
}

func soslIsEmailField(fieldName string, field storage.Field) bool {
	name := strings.ToLower(strings.TrimSpace(fieldName))
	label := strings.ToLower(strings.TrimSpace(field.Label))
	return name == "email" || strings.HasSuffix(name, "email") || strings.Contains(label, "email")
}

func soslIsPhoneField(fieldName string, field storage.Field) bool {
	name := strings.ToLower(strings.TrimSpace(fieldName))
	label := strings.ToLower(strings.TrimSpace(field.Label))
	return name == "phone" || strings.HasSuffix(name, "phone") || strings.Contains(label, "phone")
}

func soslTextMatchesPattern(text string, pattern sosl.SearchTerm) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if pattern.Prefix {
		return strings.HasPrefix(strings.ToLower(text), strings.ToLower(pattern.Text))
	}
	return containsFold(text, pattern.Text)
}

func (vm *VM) searchSuggestionRows(query, objectName string, option Value, accessLevel Value) (Value, error) {
	out := typedList("List<Search.SuggestionResult>")
	if vm == nil || vm.Org == nil {
		return out, nil
	}
	if !isDatabaseAccessLevelValue(accessLevel) {
		accessLevel = vm.defaultAccessLevel()
	}
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	state, ok := vm.Org.Objects[objectName]
	if !ok {
		return out, nil
	}
	spec := sosl.ReturningObject{Object: objectName, Fields: []sosl.SelectExpr{{Field: "Id"}, {Field: "Name"}}}
	if err := vm.enforceSOSLAccess(objectName, spec, accessLevel); err != nil {
		return Null, err
	}
	limit := 10
	if option.Fields != nil {
		if value, ok := option.Fields["limit"]; ok && value.Kind == ValueInt && value.Int > 0 {
			limit = int(value.Int)
		}
	}
	pattern := sosl.SearchTerm{Text: strings.TrimSpace(query), Prefix: true}
	ids := make([]string, 0, len(state.Records))
	for id := range state.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		if len(out.List) >= limit {
			break
		}
		record := state.Records[storage.ID(id)]
		if !vm.soslRecordMatchesSuggestion(objectName, record, pattern, accessLevel) {
			continue
		}
		if vm.recordSharingApplies(accessLevel) && !vm.userModeRecordVisible(objectName, record, vm.currentUserID()) {
			continue
		}
		value := vm.vmValueFromRecord(record)
		value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Object, soslFieldSet(spec))
		row := Object("Search.SuggestionResult")
		row.Fields["sObject"] = value
		out.List = append(out.List, row)
	}
	return out, nil
}

func (vm *VM) soslRecordMatchesSuggestion(objectName string, record storage.Record, pattern sosl.SearchTerm, accessLevel Value) bool {
	for _, field := range []string{"Name", "LastName", "FirstName", "Subject"} {
		canonical := vm.resolveSObjectFieldName(objectName, field)
		if databaseAccessLevelSecurityMode(accessLevel) == "USER_MODE" && !vm.currentUserFieldPermissionWithScope(objectName, canonical, "isAccessible", accessLevelPermissionSetID(accessLevel)) {
			continue
		}
		value, ok := record.GetField(canonical)
		if ok && soslTextMatchesPattern(storageValueText(value), pattern) {
			return true
		}
	}
	return false
}

func soslSnippetsForRecord(record storage.Record, enabled bool, terms []sosl.SearchTerm) (string, Value) {
	snippets := typedMap("Map<String,String>")
	if !enabled {
		return "", snippets
	}
	fields := make([]string, 0, len(record.Fields))
	for field := range record.Fields {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		value := record.Fields[field]
		text := storageValueText(value)
		if text == "" || !soslSnippetMatches(text, terms) {
			continue
		}
		snippet := String(text)
		key := mapKey(String(field))
		snippets.Map[key] = snippet
		snippets.MapKeys[key] = String(field)
	}
	for _, field := range fields {
		key := mapKey(String(field))
		if value, ok := snippets.Map[key]; ok && value.Kind == ValueString {
			return value.Text, snippets
		}
	}
	return "", snippets
}

func soslSnippetMatches(text string, terms []sosl.SearchTerm) bool {
	if len(terms) == 0 {
		return true
	}
	for _, term := range terms {
		if containsFold(text, term.Text) {
			return true
		}
	}
	return false
}

func containsFold(text, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i <= len(text)-len(needle); i++ {
		if strings.EqualFold(text[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

func soslRecordMatchesWhere(record storage.Record, where *sosl.Condition) bool {
	matches, _ := soslWhereResult(record, where, nil)
	return matches
}

func (vm *VM) soslRecordMatchesWhere(record storage.Record, where *sosl.Condition) (bool, error) {
	var boundary error
	matches, known := soslWhereResult(record, where, func(field string) storage.FieldType {
		_, definition, ok := vm.sObjectFieldDefinition(record.Object, field)
		if !ok {
			boundary = unsupportedCallError("SOSL ordered null predicate without field metadata")
			return storage.FieldAny
		}
		// The new null-predicate controls cover text, integer and decimal
		// fields. Keep other nullable operand types explicit until captured.
		switch definition.Type {
		case storage.FieldString, storage.FieldInteger, storage.FieldDecimal:
		default:
			boundary = unsupportedCallError("SOSL ordered null predicate for " + string(definition.Type))
		}
		return definition.Type
	})
	if known {
		return matches, nil
	}
	return matches, boundary
}

// Q004/Q021-Q023/Q034-Q036/Q043-Q044 preserve unknown numeric null
// comparisons through NOT/AND/OR. Equality, LIKE and IN retain their own rules.
func soslWhereResult(record storage.Record, where *sosl.Condition, fieldType func(string) storage.FieldType) (matches, known bool) {
	if where != nil && where.Not {
		positive := *where
		positive.Not = false
		matches, known := soslWhereResult(record, &positive, fieldType)
		return known && !matches, known
	}
	if where == nil {
		return true, true
	}
	if len(where.And) > 0 {
		known := true
		for i := range where.And {
			matches, childKnown := soslWhereResult(record, &where.And[i], fieldType)
			if childKnown && !matches {
				return false, true
			}
			known = known && childKnown
		}
		return known, known
	}
	if len(where.Or) > 0 {
		known := true
		for i := range where.Or {
			matches, childKnown := soslWhereResult(record, &where.Or[i], fieldType)
			if childKnown && matches {
				return true, true
			}
			known = known && childKnown
		}
		return false, known
	}
	if strings.TrimSpace(where.Field) == "" {
		return true, true
	}
	value, ok := record.GetField(where.Field)
	text := storageValueText(value)
	if strings.EqualFold(where.Field, "Id") {
		text, ok = string(record.ID), record.ID != ""
		value = storage.StringValue(text)
	}
	matches = false
	notIn := strings.EqualFold(where.Operator, "NOT IN")
	if strings.EqualFold(where.Operator, "IN") || notIn {
		if ok {
			for _, candidate := range where.Values {
				if strings.EqualFold(where.Field, "Id") {
					if apexIDTextEqual(text, candidate) {
						matches = true
						break
					}
				} else if strings.EqualFold(text, candidate) {
					matches = true
					break
				}
			}
		}
		return matches != notIn, true
	} else if where.Operator == "<" || where.Operator == "<=" || where.Operator == ">" || where.Operator == ">=" {
		if where.ValueIsNull || where.Bind != "" {
			return false, true
		}
		if !ok || value.Kind == storage.ValueNull {
			// Q024/Q025/Q041/Q042/Q045: null text sorts like empty text in
			// ordered predicates. Use metadata so dates/numbers stay distinct.
			if fieldType == nil || fieldType(where.Field) != storage.FieldString {
				return false, false
			}
			value = storage.StringValue("")
		}
		comparison, comparable := compareSOSLWhereValue(value, where.Value)
		if !comparable {
			return false, true
		}
		switch where.Operator {
		case "<":
			return comparison < 0, true
		case "<=":
			return comparison <= 0, true
		case ">":
			return comparison > 0, true
		default:
			return comparison >= 0, true
		}
	} else if where.ValueIsNull {
		matches = !ok || value.Kind == storage.ValueNull
	} else if where.Bind != "" {
		// Binds are expanded before parsing for the runtime path. A residual bind
		// is therefore not a local match and must not fabricate a hit.
		matches = false
	} else if ok {
		if strings.EqualFold(where.Operator, "LIKE") {
			matches = strings.Contains(strings.ToLower(text), strings.ToLower(strings.Trim(where.Value, "%")))
		} else if strings.EqualFold(where.Field, "Id") {
			matches = apexIDTextEqual(text, where.Value)
		} else {
			matches = strings.EqualFold(text, where.Value)
		}
	}
	if where.Operator == "!=" {
		return !matches, true
	}
	return matches, true
}

func compareSOSLWhereValue(value storage.Value, literal string) (int, bool) {
	switch value.Kind {
	case storage.ValueInteger, storage.ValueDecimal:
		var left *big.Rat
		if value.Kind == storage.ValueInteger {
			left = new(big.Rat).SetInt64(value.Integer)
		} else {
			var ok bool
			left, ok = new(big.Rat).SetString(value.Decimal)
			if !ok {
				return 0, false
			}
		}
		right, ok := new(big.Rat).SetString(literal)
		if !ok {
			return 0, false
		}
		return left.Cmp(right), true
	case storage.ValueNull:
		return 0, false
	default:
		return strings.Compare(strings.ToLower(storageValueText(value)), strings.ToLower(literal)), true
	}
}

func (vm *VM) soslRecordMatchesPricebook(objectName string, record storage.Record, pricebookID string) bool {
	if strings.TrimSpace(pricebookID) == "" || vm == nil || vm.Org == nil {
		return true
	}
	if strings.EqualFold(objectName, "PricebookEntry") {
		value, ok := record.GetField("Pricebook2Id")
		return ok && apexIDTextEqual(storageValueText(value), pricebookID)
	}
	if !strings.EqualFold(objectName, "Product2") {
		return true
	}
	entries, ok := vm.Org.Objects["PricebookEntry"]
	if !ok {
		return false
	}
	for _, entry := range entries.Records {
		entryPricebook, ok := entry.GetField("Pricebook2Id")
		if !ok || !apexIDTextEqual(storageValueText(entryPricebook), pricebookID) {
			continue
		}
		entryProduct, ok := entry.GetField("Product2Id")
		if ok && apexIDTextEqual(storageValueText(entryProduct), string(record.ID)) {
			return true
		}
	}
	return false
}

func (vm *VM) applySOSLReturningFunctionAliases(value *Value, record storage.Record, spec sosl.ReturningObject) {
	if value == nil || value.Kind != ValueObject {
		return
	}
	for _, alias := range spec.Fields {
		if alias.Func == "" {
			continue
		}
		stored, found := record.GetField(alias.Field)
		if !found {
			continue
		}
		switch strings.ToUpper(alias.Func) {
		case "FORMAT":
			value.Fields[alias.Alias] = String(storageValueText(stored))
		case "CONVERTCURRENCY":
			value.Fields[alias.Alias] = vmValueFromStorage(stored)
		case "TOLABEL":
			value.Fields[alias.Alias] = String(vm.soslToLabel(record.Object, alias.Field, stored))
		default:
			continue
		}
	}
}

func (vm *VM) soslToLabel(objectName, fieldName string, value storage.Value) string {
	text := storageValueText(value)
	if vm == nil || vm.Org == nil {
		return text
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return text
	}
	field, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, fieldName)
	if !ok {
		field = fieldName
	}
	fieldDef, ok := object.Definition.Fields[field]
	if !ok {
		return text
	}
	for _, option := range fieldDef.PicklistValues {
		if option.Value == text {
			if option.Label != "" {
				return option.Label
			}
			return option.Value
		}
	}
	return text
}

func sortSOSLRows(rows Value, orderBy []sosl.OrderSpec) {
	if rows.Kind != ValueList || len(rows.List) < 2 || len(orderBy) == 0 {
		return
	}
	sort.SliceStable(rows.List, func(i, j int) bool {
		return compareSOSLRows(rows.List[i], rows.List[j], orderBy) < 0
	})
}

func applySOSLReturningOffset(rows *Value, spec sosl.ReturningObject) {
	if rows == nil || rows.Kind != ValueList || !spec.Offset.HasValue || spec.Offset.Value <= 0 {
		return
	}
	if spec.Offset.Value >= len(rows.List) {
		rows.List = nil
		return
	}
	rows.List = rows.List[spec.Offset.Value:]
}

func applySOSLReturningLimit(rows *Value, spec sosl.ReturningObject) {
	if rows == nil || rows.Kind != ValueList || !spec.Limit.HasValue || spec.Limit.Value >= len(rows.List) {
		return
	}
	rows.List = rows.List[:spec.Limit.Value]
}

func compareSOSLRows(left, right Value, orderBy []sosl.OrderSpec) int {
	for _, order := range orderBy {
		leftValue := Null
		if _, value, ok := objectFieldValue(left, order.Field); ok {
			leftValue = value
		}
		rightValue := Null
		if _, value, ok := objectFieldValue(right, order.Field); ok {
			rightValue = value
		}
		cmp := compareSOSLOrderValues(leftValue, rightValue)
		// Q009-Q020: default NULLS FIRST is independent of ASC/DESC, as
		// are explicit FIRST/LAST. Direction only reverses nonnull values.
		if (leftValue.Kind == ValueNull || rightValue.Kind == ValueNull) && leftValue.Kind != rightValue.Kind {
			first := order.Nulls != "LAST"
			if (leftValue.Kind == ValueNull) == first {
				return -1
			}
			return 1
		}
		if cmp == 0 {
			continue
		}
		if order.Desc {
			return -cmp
		}
		return cmp
	}
	return 0
}

func compareSOSLOrderValues(left, right Value) int {
	if cmp, ok := compareNullSortValues(left, right); ok {
		return cmp
	}
	if isSortablePlatformValue(left) && isSortablePlatformValue(right) {
		return comparePlatformSortValues(left, right)
	}
	switch {
	case left.Kind == ValueInt && right.Kind == ValueInt:
		if left.Int < right.Int {
			return -1
		}
		if left.Int > right.Int {
			return 1
		}
		return 0
	case left.Kind == ValueDecimal && right.Kind == ValueDecimal:
		if left.Decimal < right.Decimal {
			return -1
		}
		if left.Decimal > right.Decimal {
			return 1
		}
		return 0
	case left.Kind == ValueBool && right.Kind == ValueBool:
		if left.Bool == right.Bool {
			return 0
		}
		if !left.Bool {
			return -1
		}
		return 1
	default:
		return strings.Compare(left.String(), right.String())
	}
}

func splitTopLevelComma(text string) []string {
	var parts []string
	start := 0
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, text[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, text[start:])
	return parts
}

func valueIDString(value Value) (string, bool) {
	if value.Kind == ValueString && value.Text != "" {
		return value.Text, true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		text, err := platformScalarText(value, "Id")
		return text, err == nil && text != ""
	}
	return "", false
}

func isApexIDLikeValue(value Value) bool {
	if value.Kind == ValueString && value.Text != "" {
		return true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		_, ok := valueIDString(value)
		return ok
	}
	return false
}

func (vm *VM) queriedSObjectFields(queryText string) map[string]bool {
	query, err := vm.parseSOQLAt(queryText)
	if err != nil || query.Count || len(query.Aggregates) > 0 || len(query.GroupBy) > 0 {
		return nil
	}
	objectName := query.Object
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			objectName = canonical
		}
	}
	fields := make(map[string]bool)
	for _, field := range query.Fields {
		if strings.Contains(field, "(") {
			projectedFields := selectedSOQLFunctionFields(field)
			if len(projectedFields) == 0 {
				continue
			}
			for _, projectedField := range projectedFields {
				vm.addQueriedSObjectField(fields, objectName, projectedField)
			}
			continue
		}
		vm.addQueriedSObjectField(fields, objectName, field)
	}
	for _, childQuery := range query.ChildQueries {
		if relationship := strings.TrimSpace(childQuery.Relationship); relationship != "" {
			fields[strings.ToLower(relationship)] = true
		}
	}
	if len(fields) == 0 {
		return nil
	}
	fields["id"] = true
	return fields
}

func (vm *VM) addQueriedSObjectField(fields map[string]bool, objectName, field string) {
	originalField := field
	if dot := strings.IndexByte(field, '.'); dot >= 0 {
		relationship := field[:dot]
		qualifiedField := strings.TrimSpace(field[dot+1:])
		if vm.soqlFieldQualifierMatchesObject(objectName, relationship) {
			fields[strings.ToLower(originalField)] = true
			field = qualifiedField
		} else {
			fields[strings.ToLower(originalField)] = true
			if lookupField, ok := vm.parentRelationshipField(objectName, relationship); ok {
				fields[strings.ToLower(lookupField)] = true
			}
			field = relationship
		}
	}
	if vm.Org != nil {
		if object, ok := vm.Org.Objects[objectName]; ok {
			if canonical, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
				field = canonical
			}
		}
	}
	fields[strings.ToLower(field)] = true
}

func selectedSOQLFunctionFields(field string) []string {
	projection, ok := parseSelectedFunctionAlias(field)
	if ok {
		if projection.Func == "TOLABEL" {
			if dot := strings.LastIndexByte(projection.Field, '.'); dot >= 0 {
				return []string{projection.Field[:dot+1] + projection.Alias}
			}
			return []string{projection.Alias}
		}
		return []string{projection.Field, projection.Alias}
	}
	text := strings.TrimSpace(field)
	parts := strings.Fields(text)
	if len(parts) == 0 || len(parts) > 2 {
		return nil
	}
	raw := parts[0]
	open := strings.IndexByte(raw, '(')
	if open <= 0 || !strings.HasSuffix(raw, ")") {
		return nil
	}
	if !isSelectedSOQLFieldFunction(raw[:open]) {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(raw[:open]), "DISTANCE") {
		if len(parts) == 2 {
			return []string{parts[1]}
		}
		return nil
	}
	argsText := raw[open+1 : len(raw)-1]
	if strings.TrimSpace(argsText) == "" || strings.Contains(argsText, ",") {
		return nil
	}
	fieldArg := selectedSOQLFunctionFieldArg(argsText)
	if fieldArg == "" || strings.ContainsAny(fieldArg, " (),") {
		return nil
	}
	fields := []string{fieldArg}
	if len(parts) == 2 {
		fields = append(fields, parts[1])
	}
	return fields
}

func parseSelectedFunctionAlias(field string) (sosl.SelectExpr, bool) {
	text := strings.TrimSpace(field)
	parts := strings.Fields(text)
	if len(parts) != 2 {
		return sosl.SelectExpr{}, false
	}
	raw := parts[0]
	open := strings.IndexByte(raw, '(')
	if open <= 0 || !strings.HasSuffix(raw, ")") {
		return sosl.SelectExpr{}, false
	}
	if !isSelectedSOQLFieldFunction(raw[:open]) {
		return sosl.SelectExpr{}, false
	}
	argsText := raw[open+1 : len(raw)-1]
	if strings.TrimSpace(argsText) == "" || strings.Contains(argsText, ",") {
		return sosl.SelectExpr{}, false
	}
	fieldArg := selectedSOQLFunctionFieldArg(argsText)
	if fieldArg == "" || strings.ContainsAny(fieldArg, " (),") {
		return sosl.SelectExpr{}, false
	}
	return sosl.SelectExpr{Func: strings.ToUpper(strings.TrimSpace(raw[:open])), Field: fieldArg, Alias: parts[1]}, true
}

func isSelectedSOQLFieldFunction(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "TOLABEL", "FORMAT", "CONVERTCURRENCY",
		"DISTANCE",
		"CALENDAR_MONTH", "CALENDAR_QUARTER", "CALENDAR_YEAR",
		"DAY_IN_MONTH", "DAY_IN_WEEK", "DAY_IN_YEAR", "DAY_ONLY",
		"FISCAL_MONTH", "FISCAL_QUARTER", "FISCAL_YEAR",
		"HOUR_IN_DAY", "WEEK_IN_MONTH", "WEEK_IN_YEAR":
		return true
	default:
		return false
	}
}

func selectedSOQLFunctionFieldArg(arg string) string {
	arg = strings.TrimSpace(arg)
	open := strings.IndexByte(arg, '(')
	if open <= 0 || !strings.HasSuffix(arg, ")") {
		return arg
	}
	if !strings.EqualFold(strings.TrimSpace(arg[:open]), "convertTimezone") {
		return arg
	}
	inner := strings.TrimSpace(arg[open+1 : len(arg)-1])
	if inner == "" || strings.ContainsAny(inner, ",()") {
		return arg
	}
	return inner
}

func (vm *VM) applyQueriedParentRelationshipFieldMarkers(value *Value, queryText string) {
	if vm == nil || vm.Org == nil || value == nil || value.Kind != ValueObject {
		return
	}
	query, err := vm.parseSOQLAt(queryText)
	if err != nil || query.Count || len(query.Aggregates) > 0 || len(query.GroupBy) > 0 {
		return
	}
	objectName := query.Object
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	for _, field := range query.Fields {
		if strings.Contains(field, "(") {
			projection, ok := parseSelectedFunctionAlias(field)
			if !ok || projection.Func != "TOLABEL" {
				continue
			}
			field = selectedSOQLFunctionFields(field)[0]
		}
		parts := splitSOQLRelationshipFieldPath(field)
		if len(parts) < 2 {
			continue
		}
		if vm.soqlFieldQualifierMatchesObject(objectName, parts[0]) {
			continue
		}
		vm.markQueriedParentRelationshipPath(value, objectName, parts)
	}
}

func splitSOQLRelationshipFieldPath(field string) []string {
	rawParts := strings.Split(field, ".")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil
		}
		parts = append(parts, part)
	}
	return parts
}

func (vm *VM) markQueriedParentRelationshipPath(value *Value, objectName string, parts []string) {
	if vm == nil || value == nil || value.Kind != ValueObject || len(parts) < 2 {
		return
	}
	relationship := parts[0]
	parentObject, ok := vm.parentRelationshipObjectType(objectName, relationship)
	if !ok {
		return
	}
	actualName, relationshipValue, ok := objectFieldValue(*value, relationship)
	if !ok || relationshipValue.Kind != ValueObject {
		return
	}
	vm.resolveSOQLParentRuntimeType(&relationshipValue, objectName, relationship)
	if vm.isSObjectLikeType(relationshipValue.Type) {
		parentObject = relationshipValue.Type
	}
	if relationshipValue.Type == "" || !vm.isSObjectLikeType(relationshipValue.Type) {
		relationshipValue.Type = parentObject
	}
	if relationshipValue.Fields == nil {
		relationshipValue.Fields = make(map[string]Value)
	}
	relationshipValue.Fields[sobjectParentProjectionField] = Bool(true)
	vm.ensureQueriedSObjectFieldMarker(&relationshipValue, parentObject)
	markQueriedSObjectField(&relationshipValue, parts[1])
	if len(parts) > 2 {
		vm.markQueriedParentRelationshipPath(&relationshipValue, parentObject, parts[1:])
	}
	value.Fields[actualName] = relationshipValue
}

func (vm *VM) ensureQueriedSObjectFieldMarker(value *Value, objectName string) {
	if value == nil || value.Kind != ValueObject {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	selected, ok := value.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(objectName, map[string]bool{"id": true})
		return
	}
	if _, ok := selected.Map[mapKey(String("object"))]; !ok {
		selected.Map[mapKey(String("object"))] = String(objectName)
		if selected.MapKeys == nil {
			selected.MapKeys = make(map[string]Value)
		}
		selected.MapKeys[mapKey(String("object"))] = String("object")
	}
	selected.Map[mapKey(String("id"))] = Bool(true)
	value.Fields[sobjectQueriedFieldsField] = selected
}

func (vm *VM) soqlFieldQualifierMatchesObject(objectName, qualifier string) bool {
	objectName = strings.TrimSpace(objectName)
	qualifier = strings.TrimSpace(qualifier)
	if objectName == "" || qualifier == "" {
		return false
	}
	canonicalQualifier := qualifier
	if vm != nil && vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(qualifier); ok {
			canonicalQualifier = resolved
		}
	}
	return strings.EqualFold(canonicalQualifier, objectName)
}

func (vm *VM) parentRelationshipField(objectName, relationshipName string) (string, bool) {
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" || strings.TrimSpace(relationshipName) == "" {
		return "", false
	}
	canonicalObject, ok := vm.resolveObjectName(objectName)
	if !ok {
		canonicalObject = objectName
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return "", false
	}
	for _, relation := range object.Definition.Relations {
		if vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) && strings.TrimSpace(relation.Field) != "" {
			return relation.Field, true
		}
	}
	for name, field := range object.Definition.Fields {
		apiName := field.APIName
		if apiName == "" {
			apiName = name
		}
		if !vmFieldIsReference(field) {
			continue
		}
		if vmParentRelationshipNameMatches(vm.Org.Namespace, apiName, relationshipName) {
			return apiName, true
		}
	}
	return "", false
}

func vmParentRelationshipNameMatches(namespace, fieldName, relationshipName string) bool {
	candidates := []string(nil)
	if strings.HasSuffix(fieldName, "__c") {
		candidates = append(candidates, strings.TrimSuffix(fieldName, "__c")+"__r")
	} else if strings.HasSuffix(fieldName, "Id") && len(fieldName) > len("Id") {
		candidates = append(candidates, strings.TrimSuffix(fieldName, "Id"))
	}
	for _, candidate := range candidates {
		if vmRelationshipNameMatches(namespace, candidate, relationshipName) {
			return true
		}
	}
	return false
}

func (vm *VM) parentRelationshipNameForField(definition storage.ObjectDefinition, fieldName string) (string, bool) {
	if vm == nil || strings.TrimSpace(fieldName) == "" {
		return "", false
	}
	for _, relation := range definition.Relations {
		if strings.EqualFold(relation.Field, fieldName) && strings.TrimSpace(relation.ParentRelationship) != "" {
			return relation.ParentRelationship, true
		}
	}
	return "", false
}

func (vm *VM) parentRelationshipNameForReferenceField(definition storage.ObjectDefinition, field storage.Field) string {
	describeName := vm.describeFieldName(field.APIName)
	if strings.HasSuffix(describeName, "__c") {
		return strings.TrimSuffix(describeName, "__c") + "__r"
	}
	if parentRelationship, ok := vm.parentRelationshipNameForField(definition, field.APIName); ok {
		return parentRelationship
	}
	if strings.HasSuffix(describeName, "Id") && len(describeName) > len("Id") {
		return strings.TrimSuffix(describeName, "Id")
	}
	return field.RelationshipName
}

func lookupFieldRelationshipName(field string) string {
	if strings.HasSuffix(field, "__c") {
		return strings.TrimSuffix(field, "__c") + "__r"
	}
	if strings.HasSuffix(field, "Id") && len(field) > len("Id") {
		return strings.TrimSuffix(field, "Id")
	}
	return ""
}

func (vm *VM) enforceSOQLSecurity(query soql.Query, permissionSetID string) error {
	mode := strings.ToUpper(strings.TrimSpace(query.SecurityMode))
	if mode == "" || mode == "SYSTEM_MODE" {
		return nil
	}
	objectName := query.Object
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			objectName = canonical
		}
	}
	if !vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", permissionSetID) {
		return newExceptionError("QueryException", fmt.Sprintf("sObject type '%s' is not supported by %s", objectName, mode))
	}
	for _, field := range query.Fields {
		if err := vm.enforceSOQLRelationshipSecurityWithScope(objectName, field, mode, permissionSetID); err != nil {
			return err
		}
		for _, fieldName := range vm.securityFieldNames(objectName, field, mode) {
			if !vm.currentUserFieldPermissionWithScope(objectName, fieldName, "isAccessible", permissionSetID) {
				return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s'.", fieldName, objectName))
			}
		}
	}
	for _, field := range soqlConditionFields(query.Where) {
		if err := vm.enforceSOQLRelationshipSecurityWithScope(objectName, field, mode, permissionSetID); err != nil {
			return err
		}
	}
	for _, order := range query.Order {
		if err := vm.enforceSOQLRelationshipSecurityWithScope(objectName, order.Field, mode, permissionSetID); err != nil {
			return err
		}
		for _, fieldName := range vm.securityFieldNames(objectName, order.Field, mode) {
			if !vm.currentUserFieldPermissionWithScope(objectName, fieldName, "isAccessible", permissionSetID) {
				return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s'.", fieldName, objectName))
			}
		}
	}
	for _, child := range query.ChildQueries {
		childQuery := child.Query
		if strings.TrimSpace(childQuery.SecurityMode) == "" {
			childQuery.SecurityMode = mode
		}
		if childObject, ok := vm.soqlChildRelationshipObject(objectName, child.Relationship); ok {
			childQuery.Object = childObject
		}
		if err := vm.enforceSOQLSecurity(childQuery, permissionSetID); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) soqlChildRelationshipObject(parentObject, relationshipName string) (string, bool) {
	listType, ok := vm.jsonSObjectChildRelationshipType(parentObject, relationshipName)
	if !ok || !strings.HasPrefix(listType, "List<") || !strings.HasSuffix(listType, ">") {
		return "", false
	}
	childObject := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(listType, "List<"), ">"))
	return childObject, childObject != ""
}

func soqlConditionFields(condition *soql.Condition) []string {
	if condition == nil {
		return nil
	}
	out := []string(nil)
	if condition.Not {
		nested := *condition
		nested.Not = false
		return soqlConditionFields(&nested)
	}
	for i := range condition.And {
		out = append(out, soqlConditionFields(&condition.And[i])...)
	}
	for i := range condition.Or {
		out = append(out, soqlConditionFields(&condition.Or[i])...)
	}
	if strings.TrimSpace(condition.Field) != "" {
		out = append(out, condition.Field)
	}
	return out
}

func (vm *VM) validateSOQLIDLiteralConditions(query soql.Query) error {
	if vm == nil || vm.Org == nil || query.Where == nil {
		return nil
	}
	object, ok := vm.Org.Objects[query.Object]
	if !ok {
		return nil
	}
	return vm.validateSOQLIDLiteralCondition(object.Definition, query.Where)
}

func (vm *VM) validateSOQLIDLiteralCondition(definition storage.ObjectDefinition, condition *soql.Condition) error {
	if condition == nil {
		return nil
	}
	if condition.Not {
		nested := *condition
		nested.Not = false
		return vm.validateSOQLIDLiteralCondition(definition, &nested)
	}
	for i := range condition.And {
		if err := vm.validateSOQLIDLiteralCondition(definition, &condition.And[i]); err != nil {
			return err
		}
	}
	for i := range condition.Or {
		if err := vm.validateSOQLIDLiteralCondition(definition, &condition.Or[i]); err != nil {
			return err
		}
	}
	if strings.TrimSpace(condition.Field) == "" || strings.Contains(condition.Field, ".") {
		return nil
	}
	canonicalField, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, condition.Field)
	if !ok {
		if !strings.EqualFold(strings.TrimSpace(condition.Field), "Id") {
			return nil
		}
		canonicalField = "Id"
	}
	field, hasField := definition.Fields[canonicalField]
	if !strings.EqualFold(canonicalField, "Id") && (!hasField || field.Type != storage.FieldID) {
		return nil
	}
	validate := func(value storage.Value) error {
		if value.Kind != storage.ValueString || strings.TrimSpace(value.String) == "" {
			return nil
		}
		if err := validateApexIDShape(value.String); err != nil {
			return newExceptionError("QueryException", "invalid ID field: "+value.String)
		}
		return nil
	}
	switch condition.Op {
	case "=":
		return validate(condition.Value)
	case "IN":
		for _, value := range condition.Values {
			if err := validate(value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (vm *VM) enforceSOQLRelationshipSecurity(objectName, expression, mode string) error {
	return vm.enforceSOQLRelationshipSecurityWithScope(objectName, expression, mode, "")
}

func (vm *VM) enforceSOQLRelationshipSecurityWithScope(objectName, expression, mode, permissionSetID string) error {
	if vm == nil || vm.Org == nil {
		return nil
	}
	expression = strings.TrimSpace(expression)
	if expression == "" || strings.Contains(expression, "(") {
		return nil
	}
	if raw, ok := soqlSecurityExpressionBeforeAlias(expression); ok {
		expression = raw
	}
	if strings.EqualFold(mode, "USER_MODE") {
		if relation, ok := vm.polymorphicSOQLNameFieldRelationship(objectName, expression); ok {
			fieldName, found := vm.soqlRelationshipReferenceFieldName(objectName, relation)
			if !found {
				fieldName = relation.Field
			}
			if !found || !vm.currentUserFieldPermissionWithScope(objectName, fieldName, "isAccessible", permissionSetID) {
				return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s'.", fieldName, objectName))
			}
			return nil
		}
	}
	parts := strings.Split(expression, ".")
	if len(parts) < 2 {
		return nil
	}
	if vm.soqlFieldQualifierMatchesObject(objectName, parts[0]) && len(parts) >= 3 {
		parts = parts[1:]
	}
	if len(parts) < 2 || strings.EqualFold(parts[len(parts)-1], "Id") {
		return nil
	}
	targets, ok := vm.parentRelationshipTargets(objectName, parts[0])
	if !ok || len(targets) == 0 {
		return nil
	}
	if !vm.soqlRelationshipSecurityTargetsAllowed(targets, parts[1:], mode, permissionSetID) {
		if len(targets) == 1 {
			if field, entity, missing := vm.soqlRelationshipMissingColumn(targets, parts[1:]); missing {
				return newExceptionError("QueryException", soql.MissingColumnMessage(field, entity))
			}
		}
		return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s' for %s", expression, objectName, mode))
	}
	return nil
}

func (vm *VM) soqlRelationshipMissingColumn(targets, parts []string) (string, string, bool) {
	for _, target := range targets {
		if canonical, ok := vm.resolveObjectName(target); ok {
			target = canonical
		}
		object, ok := vm.Org.Objects[target]
		if !ok || len(parts) == 0 {
			continue
		}
		if len(parts) == 1 {
			if _, known := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, parts[0]); !known {
				return parts[0], target, true
			}
			continue
		}
		if nested, ok := vm.parentRelationshipTargets(target, parts[0]); ok {
			if field, entity, missing := vm.soqlRelationshipMissingColumn(nested, parts[1:]); missing {
				return field, entity, true
			}
		}
	}
	return "", "", false
}

func (vm *VM) soqlRelationshipSecurityTargetsAllowed(targets []string, parts []string, mode, permissionSetID string) bool {
	if len(targets) == 0 || len(parts) == 0 {
		return false
	}
	for _, target := range targets {
		targetName := target
		if canonical, ok := vm.resolveObjectName(target); ok {
			targetName = canonical
		}
		object, ok := vm.Org.Objects[targetName]
		if !ok {
			return false
		}
		if strings.EqualFold(mode, "USER_MODE") && len(parts) == 2 && strings.EqualFold(parts[1], "Type") {
			if relation, ok := vm.parentRelationshipMetadata(targetName, parts[0]); ok && relation.Polymorphic && len(relation.ParentObjects) > 0 {
				fieldName, found := vm.soqlRelationshipReferenceFieldName(targetName, relation)
				if !found || !vm.currentUserFieldPermissionWithScope(targetName, fieldName, "isAccessible", permissionSetID) {
					return false
				}
				continue
			}
		}
		if len(parts) == 1 {
			canonicalField, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, parts[0])
			if !ok || !vm.currentUserFieldPermissionWithScope(targetName, canonicalField, "isAccessible", permissionSetID) {
				return false
			}
			continue
		}
		nestedTargets, ok := vm.parentRelationshipTargets(targetName, parts[0])
		if !ok || len(nestedTargets) == 0 {
			return false
		}
		if !vm.soqlRelationshipSecurityTargetsAllowed(nestedTargets, parts[1:], mode, permissionSetID) {
			return false
		}
	}
	return true
}

func soqlSecurityExpressionBeforeAlias(expression string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(expression))
	if len(fields) < 2 {
		return "", false
	}
	return fields[0], true
}

func (vm *VM) parentRelationshipTargets(objectName, relationshipName string) ([]string, bool) {
	relation, ok := vm.parentRelationshipMetadata(objectName, relationshipName)
	if !ok {
		return nil, false
	}
	return append([]string(nil), relation.ParentObjects...), true
}

func (vm *VM) parentRelationshipMetadata(objectName, relationshipName string) (storage.Relationship, bool) {
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" || strings.TrimSpace(relationshipName) == "" {
		return storage.Relationship{}, false
	}
	canonicalObject, ok := vm.resolveObjectName(objectName)
	if !ok {
		canonicalObject = objectName
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return storage.Relationship{}, false
	}
	for _, relation := range object.Definition.Relations {
		if vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) ||
			vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			return relation, true
		}
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		return relation, true
	}
	return storage.Relationship{}, false
}

func (vm *VM) soqlRelationshipReferenceFieldName(objectName string, relation storage.Relationship) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	if canonicalObject, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonicalObject
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return "", false
	}
	return storage.ResolveFieldName(object.Definition, vm.Org.Namespace, relation.Field)
}

func (vm *VM) polymorphicSOQLNameFieldRelationship(objectName, expression string) (storage.Relationship, bool) {
	if vm == nil || vm.Org == nil {
		return storage.Relationship{}, false
	}
	expression = strings.TrimSpace(expression)
	if raw, ok := soqlSecurityExpressionBeforeAlias(expression); ok {
		expression = raw
	}
	parts := strings.Split(expression, ".")
	if vm.soqlFieldQualifierMatchesObject(objectName, parts[0]) && len(parts) >= 3 {
		parts = parts[1:]
	}
	// R186–R189: Name and Type use the common polymorphic parent shape.
	// Their security check uses the reference field, not every target schema.
	if len(parts) != 2 || !(strings.EqualFold(parts[1], "Name") || strings.EqualFold(parts[1], "Type")) {
		return storage.Relationship{}, false
	}
	relation, ok := vm.parentRelationshipMetadata(objectName, parts[0])
	return relation, ok && relation.Polymorphic && len(relation.ParentObjects) > 0
}

func (vm *VM) applySOQLSharing(query soql.Query, result soql.Result) soql.Result {
	userMode := strings.EqualFold(strings.TrimSpace(query.SecurityMode), "USER_MODE")
	if !userMode && vm.testContext != nil && vm.testContext.RunAsDepth == 0 {
		return result
	}
	if !userMode && !vm.currentClassHasSharingMode("with sharing") {
		return result
	}
	if vm.soqlObjectHasPublicReadSharing(query.Object) {
		return result
	}
	if vm.currentUserBypassesRecordSharing() {
		return result
	}
	userID := vm.currentUserID()
	if userID == "" {
		return result
	}
	if soqlSharingCountQuery(query) {
		visibleQuery := query
		visibleQuery.Count = false
		visibleQuery.Aggregates = nil
		visibleQuery.Fields = []string{"Id"}
		if len(query.Aggregates) == 1 && query.Aggregates[0].Field != "" {
			// R038/R040/R044/R046/R050/R052/R088: COUNT(field) counts
			// non-null field values, including when sharing recounts the rows.
			nonNull := soql.Condition{Field: query.Aggregates[0].Field, Op: "!=", Value: storage.NullValue()}
			if visibleQuery.Where != nil {
				nonNull = soql.Condition{And: []soql.Condition{*visibleQuery.Where, nonNull}}
			}
			visibleQuery.Where = &nonNull
		}
		if !userMode {
			visibleQuery.SecurityMode = ""
		}
		visibleResult, err := soql.ExecuteWithCache(*vm.Org, visibleQuery, vm.soqlExecutionCacheForOrg(vm.Org))
		if err == nil {
			visibleResult = vm.applySOQLSharing(visibleQuery, visibleResult)
			count := storage.IntegerValue(int64(len(visibleResult.Records)))
			if len(result.Records) == 0 {
				result.Records = []storage.Record{{Object: "AggregateResult", Fields: map[string]storage.Value{"expr0": count}}}
			} else {
				if result.Records[0].Fields == nil {
					result.Records[0].Fields = make(map[string]storage.Value)
				}
				result.Records[0].Fields["expr0"] = count
			}
			result.Rows = len(result.Records)
		}
		return result
	}
	records := result.Records[:0]
	for _, record := range result.Records {
		if vm.userModeRecordVisible(query.Object, record, userID) {
			records = append(records, record)
		}
	}
	result.Records = records
	result.Rows = len(records)
	return result
}

func (vm *VM) currentUserBypassesRecordSharing() bool {
	return vm.currentUserBypassesRecordSharingWithPermissions("PermissionsViewAllData", "PermissionsModifyAllData")
}

func (vm *VM) currentUserBypassesRecordSharingForWrite() bool {
	return vm.currentUserBypassesRecordSharingWithPermissions("PermissionsModifyAllData")
}

func (vm *VM) currentUserBypassesRecordSharingWithPermissions(permissions ...string) bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	for _, permission := range permissions {
		if objectBoolField(user, permission) || userHasPermission(user, strings.TrimPrefix(permission, "Permissions")) {
			return true
		}
	}
	profileID := stringField(user, "ProfileId")
	if vm.currentUserProfileName(user) == "System Administrator" {
		return true
	}
	if vm.recordHasAnyBooleanPermission("Profile", profileID, permissions...) {
		return true
	}
	for _, permissionSetID := range vm.assignedPermissionSetIDs(stringField(user, "Id")) {
		if vm.recordHasAnyBooleanPermission("PermissionSet", permissionSetID, permissions...) {
			return true
		}
	}
	return false
}

func (vm *VM) recordHasAnyBooleanPermission(objectName, recordID string, fields ...string) bool {
	if recordID == "" || vm == nil || vm.Org == nil {
		return false
	}
	state, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	record, ok := state.Records[storage.ID(recordID)]
	if !ok {
		return false
	}
	for _, field := range fields {
		value, ok := record.GetField(field)
		if ok && value.Kind == storage.ValueBoolean && value.Boolean {
			return true
		}
	}
	return false
}

func (vm *VM) soqlObjectHasPublicReadSharing(objectName string) bool {
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" {
		return false
	}
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok {
		return false
	}
	model := strings.TrimSpace(vm.Org.Objects[canonical].Definition.SharingModel)
	if model == "" && standardObjectDefaultsToPublicRead(canonical) {
		return true
	}
	switch strings.ToLower(model) {
	case "readwrite", "publicreadwrite", "read", "readonly", "publicreadonly", "publicread":
		return true
	default:
		return false
	}
}

func soqlSharingCountQuery(query soql.Query) bool {
	if len(query.GroupBy) > 0 || query.Having != nil {
		return false
	}
	if query.Count && len(query.Aggregates) == 0 {
		return true
	}
	if len(query.Aggregates) != 1 {
		return false
	}
	return strings.EqualFold(query.Aggregates[0].Func, "COUNT")
}

func (vm *VM) normalizeSOQLRelationshipGroupBy(query *soql.Query) {
	if vm == nil || vm.Org == nil || query == nil || len(query.GroupBy) == 0 {
		return
	}
	for i, field := range query.GroupBy {
		if normalized, ok := vm.normalizeSOQLRelationshipField(query.Object, field); ok {
			query.GroupBy[i] = normalized
		}
	}
}

func (vm *VM) normalizeSOQLRelationshipField(objectName, field string) (string, bool) {
	field = strings.TrimSpace(field)
	if field == "" || !strings.Contains(field, ".") {
		return "", false
	}
	if before, after, ok := strings.Cut(field, "."); ok && strings.EqualFold(before, objectName) {
		field = after
	}
	relationship, leaf, ok := strings.Cut(field, ".")
	if !ok || !strings.EqualFold(leaf, "Id") {
		return "", false
	}
	canonicalObject := objectName
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		canonicalObject = resolved
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if ok {
		for _, relation := range object.Definition.Relations {
			if strings.EqualFold(relation.ParentRelationship, relationship) {
				return relation.Field, true
			}
		}
	}
	if strings.HasSuffix(relationship, "__r") {
		return strings.TrimSuffix(relationship, "__r") + "__c", true
	}
	return relationship + "Id", true
}

func (vm *VM) currentClassHasSharingMode(mode string) bool {
	return strings.EqualFold(vm.currentSharingMode(), mode)
}

func (vm *VM) nearestCallStackSharingMode() (string, bool) {
	for i := len(vm.callStack) - 1; i >= 0; i-- {
		if mode := strings.TrimSpace(vm.callStack[i].SharingMode); mode != "" {
			return mode, true
		}
		className := classNameFromMethod(vm.callStack[i].Symbol)
		if hasSuffixFold(className, ".withsharing") {
			return "with sharing", true
		}
		if class, ok := vm.lookupClass(className); ok {
			if methodHasModifier(class.Modifiers, "with sharing") {
				return "with sharing", true
			}
			if methodHasModifier(class.Modifiers, "without sharing") {
				return "without sharing", true
			}
		}
	}
	return "", false
}

func (vm *VM) currentUserID() string {
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if id := stringField(user, "Id"); id != "" {
		return id
	}
	if user.Kind == ValueObject {
		return "__run_as_user_without_id__"
	}
	return vm.currentUserInfoField("Id", "")
}

func (vm *VM) securityFieldNames(objectName, expression, mode string) []string {
	expression = strings.TrimSpace(expression)
	if expression == "" || strings.Contains(expression, "(") {
		return nil
	}
	if strings.EqualFold(mode, "USER_MODE") {
		if relation, ok := vm.polymorphicSOQLNameFieldRelationship(objectName, expression); ok {
			return []string{relation.Field}
		}
	}
	if before, after, ok := strings.Cut(expression, "."); ok {
		if strings.EqualFold(before, objectName) {
			expression = after
		}
	}
	if strings.Contains(expression, ".") {
		parts := strings.Split(expression, ".")
		if len(parts) >= 2 && strings.EqualFold(parts[len(parts)-1], "Id") {
			return nil
		}
		expression = parts[0]
	}
	if dot := strings.IndexByte(expression, '.'); dot >= 0 {
		expression = expression[:dot]
	}
	if vm.Org != nil {
		if object, ok := vm.Org.Objects[objectName]; ok {
			if canonical, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, expression); ok {
				expression = canonical
			}
		}
	}
	if strings.EqualFold(expression, "Id") {
		return nil
	}
	return []string{expression}
}

func soqlLimitRows(result soql.Result) int {
	rows := len(result.Records)
	for _, record := range result.Records {
		for _, children := range record.Children {
			rows += len(children)
		}
	}
	return rows
}

func aggregateCount(value Value) (Value, bool) {
	if value.Kind != ValueList || len(value.List) != 1 {
		return Null, false
	}
	row := value.List[0]
	if row.Kind != ValueObject || row.Type != "AggregateResult" {
		return Null, false
	}
	count, ok := row.Fields["expr0"]
	return count, ok && count.Kind == ValueInt
}

func (vm *VM) expandSOQLBinds(raw string) (string, error) {
	return vm.expandSOQLBindsWith(raw, func(name string) (Value, error) {
		value, err := vm.lookup(name)
		if err != nil && strings.HasPrefix(err.Error(), "unknown variable ") {
			return Null, newExceptionError("QueryException", "Variable does not exist: "+name)
		}
		return value, err
	}, func(name string) (Value, error) {
		return vm.call(name, nil, nil, resultForLookup())
	})
}

func (vm *VM) expandSOSLBinds(raw string) (string, error) {
	// A colon inside a literal FIND term is search text, including escaped
	// punctuation. Only the remaining clauses contain dynamic Apex binds.
	end := soslLiteralTermEnd(raw)
	prefix := raw[:end]
	expanded, err := vm.expandSOQLBindsWithLiteral(raw[end:], vm.lookup, func(name string) (Value, error) {
		return vm.call(name, nil, nil, resultForLookup())
	}, soslLiteral)
	return prefix + expanded, err
}

func (vm *VM) expandSOQLBindsFromMap(raw string, binds Value) (string, error) {
	if binds.Kind != ValueMap {
		return "", fmt.Errorf("queryWithBinds bind values must be a Map")
	}
	return vm.expandSOQLBindsWith(raw, func(name string) (Value, error) {
		return soqlBindMapValue(binds, name)
	}, nil)
}

func (vm *VM) expandSOQLBindsWith(raw string, lookup func(string) (Value, error), call func(string) (Value, error)) (string, error) {
	columns, _ := soql.BindColumns(raw)
	return vm.expandSOQLBindsWithContextLiteral(raw, lookup, call, func(value Value, offset int) string {
		if column, ok := columns[offset]; ok && vm.Org != nil {
			if object, ok := storage.ResolveObjectName(*vm.Org, column.Object); ok {
				definition := vm.Org.Objects[object].Definition
				if field, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, column.Field); ok && definition.Fields[field].Type == storage.FieldString {
					return soqlTextBindLiteral(value)
				}
			}
		}
		return soqlLiteral(value)
	})
}

// Native R013/R015/R031: typed Id text binds use the 18-character
// String form, including IN and LIKE. Id/reference columns retain Id literals.
func soqlTextBindLiteral(value Value) string {
	if text, ok := typedIDValueText(value); ok {
		text = displayIDText(text)
		return soqlQuotedStringLiteral(text)
	}
	switch value.Kind {
	case ValueList:
		items := make([]string, len(value.List))
		for i, item := range value.List {
			items[i] = soqlTextBindLiteral(item)
		}
		return "(" + strings.Join(items, ", ") + ")"
	case ValueSet:
		items := make([]string, len(value.Set))
		for i, item := range value.Set {
			items[i] = soqlTextBindLiteral(item)
		}
		return "(" + strings.Join(items, ", ") + ")"
	}
	return soqlLiteral(value)
}

func (vm *VM) expandSOQLBindsWithLiteral(raw string, lookup func(string) (Value, error), call func(string) (Value, error), literal func(Value) string) (string, error) {
	return vm.expandSOQLBindsWithContextLiteral(raw, lookup, call, func(value Value, _ int) string { return literal(value) })
}

func (vm *VM) expandSOQLBindsWithContextLiteral(raw string, lookup func(string) (Value, error), call func(string) (Value, error), literal func(Value, int) string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(raw); {
		if raw[i] == '\'' {
			out.WriteByte(raw[i])
			i++
			for i < len(raw) {
				out.WriteByte(raw[i])
				if raw[i] == '\'' {
					if i+1 < len(raw) && raw[i+1] == '\'' {
						i++
						out.WriteByte(raw[i])
						i++
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}
		if raw[i] != ':' {
			out.WriteByte(raw[i])
			i++
			continue
		}
		bindLiteral := func(value Value) string { return literal(value, i) }
		valueStart := i + 1
		for valueStart < len(raw) && (raw[valueStart] == ' ' || raw[valueStart] == '\t' || raw[valueStart] == '\n' || raw[valueStart] == '\r') {
			valueStart++
		}
		if isSOQLDateLiteralBind(raw, i) {
			trimmed := strings.TrimRight(out.String(), " \t\n\r")
			if len(trimmed) != out.Len() {
				out.Reset()
				out.WriteString(trimmed)
			}
			out.WriteByte(':')
			i++
			if valueStart < len(raw) && valueStart != i && raw[valueStart] >= '0' && raw[valueStart] <= '9' {
				for valueStart < len(raw) && raw[valueStart] >= '0' && raw[valueStart] <= '9' {
					out.WriteByte(raw[valueStart])
					valueStart++
				}
				i = valueStart
			}
			continue
		}
		if valueStart < len(raw) && raw[valueStart] == '(' && call != nil {
			value, end, err := vm.evalSOQLBindExpression(raw[valueStart:], resultForLookup())
			if err != nil {
				return "", err
			}
			if value.Kind == ValueList || value.Kind == ValueSet {
				rewriteTrailingSOQLEqualsToIn(&out)
			}
			writeSOQLBindExpansion(&out, value, raw[valueStart:valueStart+end], bindLiteral)
			i = valueStart + end
			continue
		}
		if valueStart >= len(raw) || !isIdentStart(raw[valueStart]) {
			out.WriteByte(raw[i])
			i++
			continue
		}
		nameStart := valueStart
		j := nameStart
		var name strings.Builder
		for j < len(raw) {
			if isIdentPart(raw[j]) {
				name.WriteByte(raw[j])
				j++
				continue
			}
			dot := j
			for dot < len(raw) && (raw[dot] == ' ' || raw[dot] == '\t' || raw[dot] == '\n' || raw[dot] == '\r') {
				dot++
			}
			if dot < len(raw) && raw[dot] == '.' {
				next := dot + 1
				for next < len(raw) && (raw[next] == ' ' || raw[next] == '\t' || raw[next] == '\n' || raw[next] == '\r') {
					next++
				}
				if next < len(raw) && isIdentStart(raw[next]) {
					name.WriteByte('.')
					j = next
					name.WriteByte(raw[j])
					j++
					for j < len(raw) && isIdentPart(raw[j]) {
						name.WriteByte(raw[j])
						j++
					}
					continue
				}
			}
			if raw[j] == '.' && j+1 < len(raw) && isIdentStart(raw[j+1]) {
				name.WriteByte('.')
				name.WriteByte(raw[j+1])
				j += 2
				for j < len(raw) && isIdentPart(raw[j]) {
					name.WriteByte(raw[j])
					j++
				}
				continue
			}
			break
		}
		callEnd, isCall := consumeEmptyCallSuffix(raw, j)
		nameString := name.String()
		if isSOQLLiteralBind(nameString) {
			out.WriteString(strings.ToLower(nameString))
			i = j
			continue
		}
		if call != nil && shouldEvaluateSOQLBindExpression(raw, j, callEnd, isCall) {
			value, end, err := vm.evalSOQLBindExpression(raw[valueStart:], resultForLookup())
			if err != nil {
				return "", err
			}
			if value.Kind == ValueList || value.Kind == ValueSet {
				rewriteTrailingSOQLEqualsToIn(&out)
			}
			writeSOQLBindExpansion(&out, value, raw[valueStart:valueStart+end], bindLiteral)
			i = valueStart + end
			continue
		}
		if isCall && call != nil {
			value, err := call(nameString)
			if err == nil {
				if value.Kind == ValueList || value.Kind == ValueSet {
					rewriteTrailingSOQLEqualsToIn(&out)
				}
				out.WriteString(bindLiteral(value))
				i = callEnd
				continue
			}
		}
		value, err := lookup(nameString)
		if err != nil && isCall && call != nil {
			value, err = call(nameString)
		}
		if err != nil && call != nil {
			value, end, evalErr := vm.evalSOQLBindExpression(raw[valueStart:], resultForLookup())
			if evalErr == nil {
				if value.Kind == ValueList || value.Kind == ValueSet {
					rewriteTrailingSOQLEqualsToIn(&out)
				}
				writeSOQLBindExpansion(&out, value, raw[valueStart:valueStart+end], bindLiteral)
				i = valueStart + end
				continue
			}
		}
		if err != nil {
			return "", err
		}
		if value.Kind == ValueList || value.Kind == ValueSet {
			rewriteTrailingSOQLEqualsToIn(&out)
		}
		out.WriteString(bindLiteral(value))
		if isCall {
			i = callEnd
		} else {
			i = j
		}
	}
	return out.String(), nil
}

func (vm *VM) executeSOQL(raw string, execResult *Result) (Value, error) {
	if soql.IsSOSLFind(raw) {
		return vm.executeSOSL(raw, execResult)
	}
	values, err := vm.executeSOQLRows(raw, execResult)
	if err != nil {
		return Null, err
	}
	out := List(values...)
	if len(values) > 0 && values[0].Type != "" {
		out.Type = "List<" + values[0].Type + ">"
	} else if objectName := vm.soqlResultObjectNameWithExpander(raw, vm.expandSOQLBinds); objectName != "" {
		out.Type = "List<" + objectName + ">"
	}
	tagSOQLQueryList(&out, raw)
	return out, nil
}

func (vm *VM) executeCursorSOQL(raw string, binds Value, execResult *Result) (Value, error) {
	expand := vm.expandSOQLBinds
	if binds.Kind == ValueMap {
		expand = func(query string) (string, error) { return vm.expandSOQLBindsFromMap(query, binds) }
	} else {
		binds = typedMap("Map<String,Object>")
	}
	values, err := vm.executeSOQLRowsWithAccounting(raw, execResult, expand, binds, vm.defaultAccessLevelMode(), "", soqlCursorAccounting)
	if err != nil {
		return Null, err
	}
	out := List(values...)
	if len(values) > 0 && values[0].Type != "" {
		out.Type = "List<" + values[0].Type + ">"
	} else if objectName := vm.soqlResultObjectNameWithExpander(raw, expand); objectName != "" {
		out.Type = "List<" + objectName + ">"
	}
	tagSOQLQueryList(&out, raw)
	return out, nil
}
func (vm *VM) executeSOQLWithAccessLevel(raw string, accessLevel Value, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRowsWithAccessLevel(raw, execResult, accessLevel)
	if err != nil {
		return Null, err
	}
	out := List(values...)
	if len(values) > 0 && values[0].Type != "" {
		out.Type = "List<" + values[0].Type + ">"
	} else if objectName := vm.soqlResultObjectNameWithExpander(raw, vm.expandSOQLBinds); objectName != "" {
		out.Type = "List<" + objectName + ">"
	}
	tagSOQLQueryList(&out, raw)
	return out, nil
}
func (vm *VM) executeInlineSOQL(raw string, execResult *Result) (Value, error) {
	value, err := vm.executeSOQL(raw, execResult)
	if err != nil {
		return Null, err
	}
	tagSOQLQueryList(&value, raw)
	if vm.inlineSOQLMayReturnScalarCount(raw) {
		if count, ok := aggregateCount(value); ok {
			return count, nil
		}
	}
	return value, nil
}
func (vm *VM) inlineSOQLMayReturnScalarCount(raw string) bool {
	query, err := vm.parseSOQLAt(raw)
	if err != nil {
		// R006: indexed binds are Apex expressions and become ordinary
		// SOQL literals before the query parser identifies scalar COUNT().
		if expanded, expandErr := vm.expandSOQLBinds(raw); expandErr == nil {
			query, err = vm.parseSOQLAt(expanded)
		}
	}
	if err != nil {
		return false
	}
	if len(query.GroupBy) > 0 || query.Having != nil {
		return false
	}
	return query.Count
}
func inlineSOQLQueryText(value Value) string {
	if value.Kind != ValueList || value.Fields == nil {
		return ""
	}
	query, ok := value.Fields["__soqlQuery"]
	if !ok || query.Kind != ValueString {
		return ""
	}
	return query.Text
}
func (vm *VM) executeSOQLWithBindMap(raw string, binds Value, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRowsWithExpander(raw, execResult, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	}, binds, "")
	if err != nil {
		return Null, err
	}
	out := List(values...)
	if len(values) > 0 && values[0].Type != "" {
		out.Type = "List<" + values[0].Type + ">"
	} else if objectName := vm.soqlResultObjectNameWithExpander(raw, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	}); objectName != "" {
		out.Type = "List<" + objectName + ">"
	}
	tagSOQLQueryList(&out, raw)
	return out, nil
}
func (vm *VM) executeSOQLWithBindMapAccessLevel(raw string, binds Value, accessLevel Value, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRowsWithExpanderAndScope(raw, execResult, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	}, binds, databaseAccessLevelSecurityMode(accessLevel), accessLevelPermissionSetID(accessLevel))
	if err != nil {
		return Null, err
	}
	out := List(values...)
	if len(values) > 0 && values[0].Type != "" {
		out.Type = "List<" + values[0].Type + ">"
	} else if objectName := vm.soqlResultObjectNameWithExpander(raw, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	}); objectName != "" {
		out.Type = "List<" + objectName + ">"
	}
	tagSOQLQueryList(&out, raw)
	return out, nil
}

func tagSOQLQueryList(value *Value, raw string) {
	if value == nil || value.Kind != ValueList {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	value.Fields["__soqlQuery"] = String(raw)
}
func (vm *VM) soqlResultObjectNameWithExpander(raw string, expand func(string) (string, error)) string {
	queryText := raw
	if expand != nil {
		if expanded, err := expand(raw); err == nil {
			queryText = expanded
		}
	}
	return vm.soqlResultObjectName(queryText)
}
func (vm *VM) soqlResultObjectName(raw string) string {
	query, err := vm.parseSOQLAt(raw)
	if err != nil || strings.TrimSpace(query.Object) == "" {
		return ""
	}
	if query.Count || len(query.Aggregates) > 0 || len(query.HavingAggregates) > 0 || len(query.GroupBy) > 0 || query.Having != nil {
		return "AggregateResult"
	}
	objectName := query.Object
	if vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(query.Object); ok {
			objectName = resolved
		}
	}
	return objectName
}
func (vm *VM) executeSOQLForType(raw, typeName string, result *Result) (Value, error) {
	value, err := vm.executeSOQL(raw, result)
	if err != nil {
		return Null, err
	}
	if collectionBase(typeName) == "List" || typeName == "Object" {
		if typeName == "Object" && vm.inlineSOQLMayReturnScalarCount(raw) {
			if count, ok := aggregateCount(value); ok {
				return count, nil
			}
		}
		if collectionBase(typeName) == "List" {
			if value.Runtime == "" && value.Type != "" && !strings.EqualFold(value.Type, typeName) {
				value.Runtime = value.Type
			}
			value.Type = typeName
		}
		return value, nil
	}
	if strings.EqualFold(typeName, "Integer") || strings.EqualFold(typeName, "Long") ||
		(vm.inlineSOQLMayReturnScalarCount(raw) && (strings.EqualFold(typeName, "Decimal") || strings.EqualFold(typeName, "Double"))) {
		if count, ok := aggregateCount(value); ok {
			return count, nil
		}
	}
	if len(value.List) == 0 {
		return Null, newExceptionError("QueryException", "List has no rows for assignment to SObject")
	}
	if len(value.List) > 1 {
		return Null, newExceptionError("QueryException", "List has more than 1 row for assignment to SObject")
	}
	return value.List[0], nil
}
func (vm *VM) executeSOQLRows(raw string, execResult *Result) ([]Value, error) {
	return vm.executeSOQLRowsWithExpander(raw, execResult, vm.expandSOQLBinds, typedMap("Map<String,Object>"), "")
}
func (vm *VM) executeSOQLRowsWithAccessLevel(raw string, execResult *Result, accessLevel Value) ([]Value, error) {
	return vm.executeSOQLRowsWithExpanderAndScope(raw, execResult, vm.expandSOQLBinds, typedMap("Map<String,Object>"), databaseAccessLevelSecurityMode(accessLevel), accessLevelPermissionSetID(accessLevel))
}
func writeSOQLBindExpansion(out *strings.Builder, value Value, consumed string, literal func(Value) string) {
	out.WriteString(literal(value))
	if strings.TrimRight(consumed, " \t\n\r") != consumed {
		out.WriteByte(' ')
	}
}

func soslQuotedStringLiteral(text string) string {
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = strings.ReplaceAll(text, "'", "''")
	return "'" + text + "'"
}

func soslLiteral(value Value) string {
	switch value.Kind {
	case ValueList:
		items := make([]string, 0, len(value.List))
		for _, item := range value.List {
			items = append(items, soslLiteral(item))
		}
		return "(" + strings.Join(items, ", ") + ")"
	case ValueSet:
		items := make([]string, 0, len(value.Set))
		for _, item := range value.Set {
			items = append(items, soslLiteral(item))
		}
		return "(" + strings.Join(items, ", ") + ")"
	case ValueString:
		if strings.EqualFold(value.Type, "Id") {
			return "'" + strings.ReplaceAll(value.Text, "'", "''") + "'"
		}
		return soslQuotedStringLiteral(value.Text)
	case ValueObject:
		if strings.EqualFold(value.Type, "Id") {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return "'" + strings.ReplaceAll(raw.Text, "'", "''") + "'"
			}
		}
		if strings.EqualFold(value.Type, "String") {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return soslQuotedStringLiteral(raw.Text)
			}
		}
	}
	return soqlLiteral(value)
}
func shouldEvaluateSOQLBindExpression(raw string, pos, callEnd int, isCall bool) bool {
	if isCall {
		pos = callEnd
	}
	for pos < len(raw) && (raw[pos] == ' ' || raw[pos] == '\t' || raw[pos] == '\n' || raw[pos] == '\r') {
		pos++
	}
	return pos < len(raw) && (raw[pos] == '[' || raw[pos] == '(' || raw[pos] == '.' || raw[pos] == '+' || raw[pos] == '-')
}
func (vm *VM) evalSOQLBindExpression(source string, result *Result) (Value, int, error) {
	expr, end, err := compileExpressionPrefix(source)
	if err != nil {
		return Null, 0, err
	}
	value, err := vm.eval(expr, result)
	if err != nil {
		return Null, 0, err
	}
	return value, end, nil
}
func isSOQLLiteralBind(name string) bool {
	return strings.EqualFold(name, "true") || strings.EqualFold(name, "false") || strings.EqualFold(name, "null")
}

func soqlHasBindExpression(raw string) bool {
	for i := 0; i < len(raw); {
		if raw[i] == '\'' {
			i++
			for i < len(raw) {
				if raw[i] == '\'' {
					i++
					if i < len(raw) && raw[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
			continue
		}
		if raw[i] != ':' {
			i++
			continue
		}
		valueStart := i + 1
		for valueStart < len(raw) && (raw[valueStart] == ' ' || raw[valueStart] == '\t' || raw[valueStart] == '\n' || raw[valueStart] == '\r') {
			valueStart++
		}
		if isSOQLDateLiteralBind(raw, i) {
			i++
			continue
		}
		if valueStart < len(raw) && raw[valueStart] == '(' {
			return true
		}
		if valueStart >= len(raw) || !isIdentStart(raw[valueStart]) {
			i++
			continue
		}
		j := valueStart + 1
		for j < len(raw) && isIdentPart(raw[j]) {
			j++
		}
		if isSOQLLiteralBind(raw[valueStart:j]) {
			i = j
			continue
		}
		return true
	}
	return false
}

func (vm *VM) soqlIDLiteralValidationQuery(raw string, fallback soql.Query) (soql.Query, bool) {
	if !soqlHasBindExpression(raw) {
		return fallback, true
	}
	queryText := soqlLiteralValidationQueryText(raw)
	query, err := vm.parseSOQLAt(queryText)
	if err != nil {
		return soql.Query{}, false
	}
	query.SecurityMode = ""
	if resolved, ok := vm.resolveObjectName(query.Object); ok {
		query.Object = resolved
	}
	return query, true
}

func soqlLiteralValidationQueryText(raw string) string {
	var out strings.Builder
	for i := 0; i < len(raw); {
		if raw[i] == '\'' {
			out.WriteByte(raw[i])
			i++
			for i < len(raw) {
				out.WriteByte(raw[i])
				if raw[i] == '\'' {
					if i+1 < len(raw) && raw[i+1] == '\'' {
						i++
						out.WriteByte(raw[i])
						i++
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}
		if raw[i] != ':' {
			out.WriteByte(raw[i])
			i++
			continue
		}
		valueStart := i + 1
		for valueStart < len(raw) && (raw[valueStart] == ' ' || raw[valueStart] == '\t' || raw[valueStart] == '\n' || raw[valueStart] == '\r') {
			valueStart++
		}
		if isSOQLDateLiteralBind(raw, i) {
			out.WriteByte(raw[i])
			i++
			continue
		}
		end, name, ok := consumeSOQLBindForLiteralValidation(raw, valueStart)
		if !ok {
			out.WriteByte(raw[i])
			i++
			continue
		}
		if isSOQLLiteralBind(name) {
			out.WriteString(strings.ToLower(name))
		} else {
			out.WriteString("null")
		}
		i = end
	}
	return out.String()
}

func consumeSOQLBindForLiteralValidation(raw string, start int) (int, string, bool) {
	if start < len(raw) && raw[start] == '(' {
		if _, end, err := compileExpressionPrefix(raw[start:]); err == nil && end > 0 {
			return start + end, raw[start : start+end], true
		}
	}
	if start >= len(raw) || !isIdentStart(raw[start]) {
		return 0, "", false
	}
	j := start
	var name strings.Builder
	for j < len(raw) {
		if isIdentPart(raw[j]) {
			name.WriteByte(raw[j])
			j++
			continue
		}
		dot := j
		for dot < len(raw) && (raw[dot] == ' ' || raw[dot] == '\t' || raw[dot] == '\n' || raw[dot] == '\r') {
			dot++
		}
		if dot < len(raw) && raw[dot] == '.' {
			next := dot + 1
			for next < len(raw) && (raw[next] == ' ' || raw[next] == '\t' || raw[next] == '\n' || raw[next] == '\r') {
				next++
			}
			if next < len(raw) && isIdentStart(raw[next]) {
				name.WriteByte('.')
				j = next
				continue
			}
		}
		break
	}
	callEnd, isCall := consumeEmptyCallSuffix(raw, j)
	if shouldEvaluateSOQLBindExpression(raw, j, callEnd, isCall) {
		if _, end, err := compileExpressionPrefix(raw[start:]); err == nil && end > 0 {
			return start + end, name.String(), true
		}
	}
	if isCall {
		return callEnd, name.String(), true
	}
	return j, name.String(), true
}

func rewriteTrailingSOQLEqualsToIn(out *strings.Builder) {
	text := out.String()
	trimmed := strings.TrimRight(text, " \t\n\r")
	if strings.HasSuffix(trimmed, "!=") {
		out.Reset()
		out.WriteString(strings.TrimRight(trimmed[:len(trimmed)-2], " \t\n\r"))
		out.WriteString(" NOT IN ")
		return
	}
	if !strings.HasSuffix(trimmed, "=") {
		return
	}
	out.Reset()
	out.WriteString(strings.TrimRight(trimmed[:len(trimmed)-1], " \t\n\r"))
	out.WriteString(" IN ")
}
func consumeEmptyCallSuffix(raw string, index int) (int, bool) {
	j := index
	for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
		j++
	}
	if j >= len(raw) || raw[j] != '(' {
		return index, false
	}
	j++
	for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
		j++
	}
	if j >= len(raw) || raw[j] != ')' {
		return index, false
	}
	return j + 1, true
}
func isSOQLDateLiteralBind(raw string, colon int) bool {
	start := colon - 1
	for start >= 0 && (raw[start] == ' ' || raw[start] == '\t' || raw[start] == '\n' || raw[start] == '\r') {
		start--
	}
	end := start + 1
	for start >= 0 && (raw[start] == '_' || raw[start] >= 'A' && raw[start] <= 'Z' || raw[start] >= 'a' && raw[start] <= 'z') {
		start--
	}
	prefix := strings.ToUpper(raw[start+1 : end])
	switch prefix {
	case "LAST_N_DAYS", "NEXT_N_DAYS", "N_DAYS_AGO",
		"LAST_N_WEEKS", "NEXT_N_WEEKS", "N_WEEKS_AGO",
		"LAST_N_MONTHS", "NEXT_N_MONTHS", "N_MONTHS_AGO",
		"LAST_N_QUARTERS", "NEXT_N_QUARTERS", "N_QUARTERS_AGO",
		"LAST_N_YEARS", "NEXT_N_YEARS", "N_YEARS_AGO",
		"LAST_N_FISCAL_QUARTERS", "NEXT_N_FISCAL_QUARTERS", "N_FISCAL_QUARTERS_AGO",
		"LAST_N_FISCAL_YEARS", "NEXT_N_FISCAL_YEARS", "N_FISCAL_YEARS_AGO":
		return true
	default:
		return false
	}
}
