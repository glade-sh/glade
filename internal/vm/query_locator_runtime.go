package vm

import (
	"fmt"
	"strconv"
	"strings"

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
		// N032/N035/N036 yielded no framed native answer for unknown tokens.
		// Remote token resolution is an explicit boundary.
		return Null, unsupportedCallError("Database.Cursor JSON queryId resolution outside the local execution")
	}
	return cursor, nil
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
			return Null, newExceptionError("QueryException", "use countQuery() for [select count()...] queries")
		}
		if len(parsed.Aggregates) > 0 || len(parsed.GroupBy) > 0 {
			message := "Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch"
			if cursor {
				message = "Query contains aggregate expressions, these are not permitted in cursor queries"
			}
			return Null, newExceptionError("QueryException", message)
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
