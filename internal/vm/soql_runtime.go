package vm

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) executeSOQLRowsWithExpander(raw string, execResult *Result, expand func(string) (string, error), binds Value, accessLevelMode string) ([]Value, error) {
	if soql.IsSOSLFind(raw) {
		return nil, unsupportedCallError("SOSL/FIND local search surface")
	}
	queryText, err := expand(raw)
	if err != nil {
		return nil, newExceptionError("QueryException", fmt.Sprintf("%s in query %q", err.Error(), raw))
	}
	if soql.IsSOSLFind(queryText) {
		return nil, unsupportedCallError("SOSL/FIND local search surface")
	}
	query, err := soql.ParseAt(queryText, vm.fakeNow)
	if err != nil {
		var unsupported *soql.UnsupportedFeatureError
		if errors.As(err, &unsupported) {
			if strings.Contains(unsupported.Message, "unsupported SOQL token") {
				return nil, newExceptionError("QueryException", fmt.Sprintf("%s in generated SOQL %q", unsupported.Message, queryText))
			}
			return nil, &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
		}
		return nil, newExceptionError("QueryException", fmt.Sprintf("%s in generated SOQL %q", err.Error(), queryText))
	}
	if strings.TrimSpace(query.SecurityMode) == "" {
		query.SecurityMode = accessLevelMode
	}
	countsQueryLimit := vm.soqlCountsQueryLimit(query)
	if countsQueryLimit {
		if err := vm.incrementLimit("queries", 1); err != nil {
			return nil, err
		}
	}
	if values, handled, err := vm.executeSoqlStub(query, queryText, binds, execResult); handled || err != nil {
		return values, err
	}
	if vm.Org == nil {
		return nil, fmt.Errorf("SOQL requires org state")
	}
	if err := vm.enforceSOQLSecurity(query); err != nil {
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
	if err := vm.validateSOQLIDLiteralConditions(executeQuery); err != nil {
		return nil, err
	}
	vm.normalizeSOQLRelationshipGroupBy(&executeQuery)
	executeOrg := vm.Org
	if strings.EqualFold(executeQuery.Object, "UserRecordAccess") {
		syntheticOrg := vm.orgWithSyntheticUserRecordAccess()
		executeOrg = &syntheticOrg
	} else if strings.EqualFold(executeQuery.Object, "RecentlyViewed") {
		syntheticOrg := vm.orgWithSyntheticRecentlyViewed()
		executeOrg = &syntheticOrg
	}
	result, err := soql.ExecuteWithCache(*executeOrg, executeQuery, vm.soqlExecutionCacheForOrg(executeOrg))
	if err != nil {
		var unsupported *soql.UnsupportedFeatureError
		if errors.As(err, &unsupported) {
			return nil, &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
		}
		return nil, newExceptionError("QueryException", fmt.Sprintf("%s in generated SOQL %q", err.Error(), queryText))
	}
	result = vm.applySOQLSharing(query, result)
	if query.ForView {
		vm.recordRecentlyViewedRows(query.Object, result.Records)
	}
	limitRows := soqlLimitRows(result)
	if countsQueryLimit {
		if err := vm.incrementLimit("queryRows", limitRows); err != nil {
			return nil, err
		}
	}
	if err := vm.incrementLimit("cpuTime", limitRows); err != nil {
		return nil, err
	}
	values := make([]Value, 0, len(result.Records))
	queriedFields := vm.queriedSObjectFields(queryText)
	for _, record := range result.Records {
		value := vm.vmValueFromRecord(record)
		if len(queriedFields) > 0 && value.Kind == ValueObject {
			value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Object, queriedFields)
			vm.hydrateQueriedRecordTypeRelationships(value)
			vm.applyQueriedParentRelationshipFieldMarkers(&value, queryText)
		}
		values = append(values, value)
	}
	appendTrace(execResult, "apex.soql", "apex.soql", map[string]any{
		"query": queryText,
		"rows":  result.Rows,
	})
	return values, nil
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
	return !storage.IsCustomMetadataObject(*vm.Org, query.Object)
}

func (vm *VM) recordRecentlyViewedRows(queryObject string, records []storage.Record) {
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
		views[id] = recentlyViewedEntry{
			ID:         id,
			ObjectName: objectName,
			Name:       recentlyViewedName(record, id),
			ViewedAt:   viewedAt,
		}
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
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal, storage.ValueBlob:
		return value.String
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
		object.Records[id] = storage.Record{
			ID:     id,
			Object: "RecentlyViewed",
			Fields: map[string]storage.Value{
				"Name":               storage.StringValue(view.Name),
				"Type":               storage.StringValue(localSchemaName(view.ObjectName)),
				"LastViewedDate":     storage.DateTimeValue(view.ViewedAt),
				"LastReferencedDate": storage.DateTimeValue(view.ViewedAt),
			},
		}
	}
	org.Objects["RecentlyViewed"] = object
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
			accessID := storage.ID(fmt.Sprintf("0UR%012d", sequence))
			sequence++
			accessObject.Records[accessID] = storage.Record{
				ID:     accessID,
				Object: "UserRecordAccess",
				Fields: map[string]storage.Value{
					"RecordId":          storage.IDValue(recordID),
					"UserId":            storage.IDValue(storage.ID(userID)),
					"HasReadAccess":     storage.BooleanValue(true),
					"HasEditAccess":     storage.BooleanValue(true),
					"HasDeleteAccess":   storage.BooleanValue(true),
					"HasTransferAccess": storage.BooleanValue(true),
					"HasAllAccess":      storage.BooleanValue(true),
					"MaxAccessLevel":    storage.StringValue("All"),
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
	if len(query.ChildQueries) > 0 || len(query.Typeofs) > 0 || soqlQueryHasConditionSubquery(query.Where) {
		return nil, true, unsupportedCallError("Test.createSoqlStub relationship query local stub surface")
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
	appendTrace(execResult, "apex.soql.stub", "apex.soql", map[string]any{
		"query":  queryText,
		"object": objectName,
		"rows":   len(rows),
	})
	return rows, true, nil
}

func soqlQueryHasConditionSubquery(condition *soql.Condition) bool {
	if condition == nil {
		return false
	}
	if condition.Subquery != nil {
		return true
	}
	for i := range condition.And {
		if soqlQueryHasConditionSubquery(&condition.And[i]) {
			return true
		}
	}
	for i := range condition.Or {
		if soqlQueryHasConditionSubquery(&condition.Or[i]) {
			return true
		}
	}
	return false
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
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("Test.setFixedSearchResults expects List<Id>")
	}
	if err := vm.requireTestContext("Test.setFixedSearchResults"); err != nil {
		return Null, err
	}
	vm.fixedSearchResults = append([]Value(nil), args[0].List...)
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
		return Null, fmt.Errorf("Search.query expects query String")
	}
	return vm.executeSOSL(args[0].Text, nil)
}

func (vm *VM) searchFind(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("Search.find expects query String and optional access level")
	}
	if args[0].Kind != ValueString {
		return Null, fmt.Errorf("Search.find expects query String")
	}
	results := Object("Search.SearchResults")
	byObject := typedMap("Map<String,List<Search.SearchResult>>")
	if vm.Org != nil {
		for _, idValue := range vm.fixedSearchResults {
			id, ok := valueIDString(idValue)
			if !ok {
				continue
			}
			objectName, ok := vm.sObjectNameForIDPrefix(idPrefix(id))
			if !ok {
				objectName, ok = vm.sObjectNameForExistingID(id)
			}
			if !ok {
				continue
			}
			record, ok := vm.findOrgRecord(objectName, storage.ID(id))
			if !ok {
				continue
			}
			key := mapKey(String(objectName))
			list := byObject.Map[key]
			if list.Kind != ValueList {
				list = typedList("List<Search.SearchResult>")
			}
			row := Object("Search.SearchResult")
			row.Fields["sObject"] = vm.vmValueFromRecord(record)
			row.Fields["snippet"] = String("")
			row.Fields["snippets"] = typedMap("Map<String,String>")
			list.List = append(list.List, row)
			byObject.Map[key] = list
			byObject.MapKeys[key] = String(objectName)
		}
	}
	results.Fields["results"] = byObject
	return results, nil
}

func (vm *VM) searchSuggest(args []Value) (Value, error) {
	if len(args) != 3 && len(args) != 4 {
		return Null, fmt.Errorf("Search.suggest expects query, sObjectType, options, and optional access level")
	}
	if args[0].Kind != ValueString || args[1].Kind != ValueString {
		return Null, fmt.Errorf("Search.suggest expects query and sObjectType Strings")
	}
	results := Object("Search.SuggestionResults")
	results.Fields["suggestionResults"] = typedList("List<Search.SuggestionResult>")
	results.Fields["hasMoreResults"] = Bool(false)
	return results, nil
}

func (vm *VM) executeSOSL(raw string, execResult *Result) (Value, error) {
	queryText, err := vm.expandSOQLBinds(raw)
	if err != nil {
		return Null, newExceptionError("QueryException", fmt.Sprintf("%s in query %q", err.Error(), raw))
	}
	objects, err := parseSOSLReturningObjects(queryText)
	if err != nil {
		return Null, err
	}
	groups := make([]Value, 0, len(objects))
	for _, spec := range objects {
		specObjectName := spec.ObjectName
		if vm.Org != nil {
			if canonical, ok := vm.resolveObjectName(spec.ObjectName); ok {
				specObjectName = canonical
			}
		}
		rows := List()
		rows.Type = "List<" + specObjectName + ">"
		if vm.Org != nil {
			for _, idValue := range vm.fixedSearchResults {
				id, ok := valueIDString(idValue)
				if !ok {
					continue
				}
				objectName, ok := vm.sObjectNameForIDPrefix(idPrefix(id))
				if !ok {
					objectName, ok = vm.sObjectNameForExistingID(id)
				}
				if !ok || !strings.EqualFold(objectName, specObjectName) {
					continue
				}
				record, ok := vm.findOrgRecord(objectName, storage.ID(id))
				if !ok {
					continue
				}
				value := vm.vmValueFromRecord(record)
				if len(spec.Fields) > 0 {
					value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Object, spec.Fields)
					vm.hydrateQueriedRecordTypeRelationships(value)
				}
				rows.List = append(rows.List, value)
			}
		}
		sortSOSLRows(rows, spec.OrderBy)
		groups = append(groups, rows)
	}
	appendTrace(execResult, "apex.sosl", "apex.sosl", map[string]any{
		"query": queryText,
		"rows":  len(vm.fixedSearchResults),
	})
	return List(groups...), nil
}

type soslReturningObject struct {
	ObjectName string
	Fields     map[string]bool
	OrderBy    []soslOrderBy
}

type soslOrderBy struct {
	Field string
	Desc  bool
}

func parseSOSLReturningObjects(query string) ([]soslReturningObject, error) {
	match := regexp.MustCompile(`(?is)\bRETURNING\s+(.+?)(?:\s+LIMIT\s+\d+\s*)?$`).FindStringSubmatch(query)
	if len(match) != 2 {
		return nil, unsupportedCallError("Search.query SOSL RETURNING clause")
	}
	parts := splitTopLevelComma(match[1])
	out := make([]soslReturningObject, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		open := strings.IndexByte(part, '(')
		close := strings.LastIndexByte(part, ')')
		if open <= 0 || close <= open {
			return nil, unsupportedCallError("Search.query SOSL RETURNING object clause")
		}
		spec := soslReturningObject{ObjectName: strings.TrimSpace(part[:open]), Fields: make(map[string]bool)}
		fields := strings.TrimSpace(part[open+1 : close])
		spec.OrderBy = parseSOSLReturningOrderBy(fields)
		fields = trimSOSLReturningFieldList(fields)
		for _, field := range splitTopLevelComma(fields) {
			field = strings.TrimSpace(field)
			if field != "" {
				spec.Fields[strings.ToLower(field)] = true
			}
		}
		out = append(out, spec)
	}
	return out, nil
}

func parseSOSLReturningOrderBy(fields string) []soslOrderBy {
	lowered := strings.ToLower(fields)
	index := strings.Index(lowered, " order by ")
	if index < 0 {
		return nil
	}
	orderText := fields[index+len(" order by "):]
	orderLowered := strings.ToLower(orderText)
	end := len(orderText)
	for _, marker := range []string{" limit "} {
		if markerIndex := strings.Index(orderLowered, marker); markerIndex >= 0 && markerIndex < end {
			end = markerIndex
		}
	}
	orderText = orderText[:end]
	var out []soslOrderBy
	for _, clause := range splitTopLevelComma(orderText) {
		parts := strings.Fields(strings.TrimSpace(clause))
		if len(parts) == 0 {
			continue
		}
		order := soslOrderBy{Field: parts[0]}
		for _, part := range parts[1:] {
			if strings.EqualFold(part, "DESC") {
				order.Desc = true
				break
			}
		}
		out = append(out, order)
	}
	return out
}

func trimSOSLReturningFieldList(fields string) string {
	lowered := strings.ToLower(fields)
	end := len(fields)
	for _, marker := range []string{" where ", " order by ", " limit "} {
		if index := strings.Index(lowered, marker); index >= 0 && index < end {
			end = index
		}
	}
	return fields[:end]
}

func sortSOSLRows(rows Value, orderBy []soslOrderBy) {
	if rows.Kind != ValueList || len(rows.List) < 2 || len(orderBy) == 0 {
		return
	}
	sort.SliceStable(rows.List, func(i, j int) bool {
		return compareSOSLRows(rows.List[i], rows.List[j], orderBy) < 0
	})
}

func compareSOSLRows(left, right Value, orderBy []soslOrderBy) int {
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
	query, err := soql.Parse(queryText)
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
			if projectedField, ok := selectedSOQLFunctionField(field); ok {
				field = projectedField
			} else {
				continue
			}
		}
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

func selectedSOQLFunctionField(field string) (string, bool) {
	text := strings.TrimSpace(field)
	lower := strings.ToLower(text)
	const prefix = "tolabel("
	if !strings.HasPrefix(lower, prefix) {
		return "", false
	}
	close := strings.IndexByte(text[len(prefix):], ')')
	if close < 0 {
		return "", false
	}
	inner := strings.TrimSpace(text[len(prefix) : len(prefix)+close])
	if inner == "" || strings.ContainsAny(inner, " (),") {
		return "", false
	}
	return inner, true
}

func (vm *VM) applyQueriedParentRelationshipFieldMarkers(value *Value, queryText string) {
	if vm == nil || vm.Org == nil || value == nil || value.Kind != ValueObject {
		return
	}
	query, err := soql.Parse(queryText)
	if err != nil || query.Count || len(query.Aggregates) > 0 || len(query.GroupBy) > 0 {
		return
	}
	objectName := query.Object
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	for _, field := range query.Fields {
		if strings.Contains(field, "(") {
			continue
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

func (vm *VM) enforceSOQLSecurity(query soql.Query) error {
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
	if !vm.currentUserObjectPermission(objectName, "isAccessible") {
		return newExceptionError("QueryException", fmt.Sprintf("sObject type '%s' is not supported by %s", objectName, mode))
	}
	for _, field := range query.Fields {
		if err := vm.enforceSOQLRelationshipSecurity(objectName, field, mode); err != nil {
			return err
		}
		for _, fieldName := range vm.securityFieldNames(objectName, field) {
			if !vm.currentUserFieldPermission(objectName, fieldName, "isAccessible") {
				return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s'.", fieldName, objectName))
			}
		}
	}
	for _, field := range soqlConditionFields(query.Where) {
		if err := vm.enforceSOQLRelationshipSecurity(objectName, field, mode); err != nil {
			return err
		}
	}
	for _, order := range query.Order {
		if err := vm.enforceSOQLRelationshipSecurity(objectName, order.Field, mode); err != nil {
			return err
		}
		for _, fieldName := range vm.securityFieldNames(objectName, order.Field) {
			if !vm.currentUserFieldPermission(objectName, fieldName, "isAccessible") {
				return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s'.", fieldName, objectName))
			}
		}
	}
	for _, child := range query.ChildQueries {
		if err := vm.enforceSOQLSecurity(child.Query); err != nil {
			return err
		}
	}
	return nil
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
			return newExceptionError("QueryException", strings.TrimPrefix(err.Error(), "System.StringException: "))
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
	fieldName := parts[1]
	for _, target := range targets {
		targetName := target
		if canonical, ok := vm.resolveObjectName(target); ok {
			targetName = canonical
		}
		object, ok := vm.Org.Objects[targetName]
		if !ok {
			return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s' for %s", expression, objectName, mode))
		}
		canonicalField, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, fieldName)
		if !ok || !vm.currentUserFieldPermission(targetName, canonicalField, "isAccessible") {
			return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s' for %s", expression, objectName, mode))
		}
	}
	return nil
}

func soqlSecurityExpressionBeforeAlias(expression string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(expression))
	if len(fields) < 2 {
		return "", false
	}
	return fields[0], true
}

func (vm *VM) parentRelationshipTargets(objectName, relationshipName string) ([]string, bool) {
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" || strings.TrimSpace(relationshipName) == "" {
		return nil, false
	}
	canonicalObject, ok := vm.resolveObjectName(objectName)
	if !ok {
		canonicalObject = objectName
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return nil, false
	}
	for _, relation := range object.Definition.Relations {
		if vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) ||
			vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			return append([]string(nil), relation.ParentObjects...), true
		}
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		return append([]string(nil), relation.ParentObjects...), true
	}
	return nil, false
}

func (vm *VM) applySOQLSharing(query soql.Query, result soql.Result) soql.Result {
	if vm.testContext != nil && vm.testContext.RunAsDepth == 0 {
		return result
	}
	if !vm.currentClassHasSharingMode("with sharing") {
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
		visibleQuery.SecurityMode = ""
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
		if record.System.OwnerID == "" || storage.IDsEqual(record.System.OwnerID, storage.ID(userID)) || vm.currentUserCanSeeSharedRecord(query.Object, record, userID) {
			records = append(records, record)
		}
	}
	result.Records = records
	result.Rows = len(records)
	return result
}

func (vm *VM) currentUserBypassesRecordSharing() bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if objectBoolField(user, "PermissionsViewAllData") || objectBoolField(user, "PermissionsModifyAllData") {
		return true
	}
	if userHasPermission(user, "ViewAllData") || userHasPermission(user, "ModifyAllData") {
		return true
	}
	profileID := stringField(user, "ProfileId")
	if vm.currentProfileIsSystemAdministrator(profileID) {
		return true
	}
	if vm.recordHasAnyBooleanPermission("Profile", profileID, "PermissionsViewAllData", "PermissionsModifyAllData") {
		return true
	}
	for _, permissionSetID := range vm.assignedPermissionSetIDs(stringField(user, "Id")) {
		if vm.recordHasAnyBooleanPermission("PermissionSet", permissionSetID, "PermissionsViewAllData", "PermissionsModifyAllData") {
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

func (vm *VM) currentUserCanSeeSharedRecord(objectName string, record storage.Record, userID string) bool {
	if strings.EqualFold(objectName, "User") || strings.EqualFold(record.Object, "User") {
		return storage.IDsEqual(record.ID, storage.ID(userID))
	}
	if strings.EqualFold(objectName, "Account") || strings.EqualFold(record.Object, "Account") {
		return storage.IDsEqual(record.ID, storage.ID(vm.currentUserContactAccountID()))
	}
	return false
}

func (vm *VM) currentUserContactAccountID() string {
	if vm == nil || vm.Org == nil {
		return ""
	}
	contactID := strings.TrimSpace(vm.currentUserInfoField("ContactId", ""))
	if contactID == "" {
		return ""
	}
	contacts, ok := vm.Org.Objects["Contact"]
	if !ok {
		return ""
	}
	contact, ok := contacts.Records[storage.ID(contactID)]
	if !ok {
		return ""
	}
	value, ok := contact.GetField("AccountId")
	if !ok || value.Kind != storage.ValueID {
		return ""
	}
	return string(value.ID)
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

func standardObjectDefaultsToPublicRead(objectName string) bool {
	name := strings.TrimSpace(objectName)
	switch {
	case strings.EqualFold(name, "Campaign"):
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
	if strings.EqualFold(mode, "with sharing") && hasSuffixFold(vm.currentClass, ".withsharing") {
		return true
	}
	if stackMode, ok := vm.nearestCallStackSharingMode(); ok {
		return strings.EqualFold(stackMode, mode)
	}
	class, ok := vm.lookupClass(vm.currentClass)
	return ok && methodHasModifier(class.Modifiers, mode)
}

func (vm *VM) nearestCallStackSharingMode() (string, bool) {
	for i := len(vm.callStack) - 1; i >= 0; i-- {
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

func (vm *VM) securityFieldNames(objectName, expression string) []string {
	expression = strings.TrimSpace(expression)
	if expression == "" || strings.Contains(expression, "(") {
		return nil
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
	return vm.expandSOQLBindsWith(raw, vm.lookup, func(name string) (Value, error) {
		return vm.call(name, nil, nil, resultForLookup())
	})
}

func (vm *VM) expandSOQLBindsFromMap(raw string, binds Value) (string, error) {
	if binds.Kind != ValueMap {
		return "", fmt.Errorf("queryWithBinds bind values must be a Map")
	}
	return vm.expandSOQLBindsWith(raw, func(name string) (Value, error) {
		if strings.Contains(name, ".") {
			return Null, fmt.Errorf("queryWithBinds does not support dotted bind path %q", name)
		}
		value, ok := binds.Map[mapKey(String(name))]
		if !ok {
			return Null, fmt.Errorf("missing bind value %q", name)
		}
		return value, nil
	}, nil)
}

func (vm *VM) expandSOQLBindsWith(raw string, lookup func(string) (Value, error), call func(string) (Value, error)) (string, error) {
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
			writeSOQLBindExpansion(&out, value, raw[valueStart:valueStart+end])
			i = valueStart + end
			continue
		}
		if isCall && call != nil {
			value, err := call(nameString)
			if err == nil {
				if value.Kind == ValueList || value.Kind == ValueSet {
					rewriteTrailingSOQLEqualsToIn(&out)
				}
				out.WriteString(soqlLiteral(value))
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
				writeSOQLBindExpansion(&out, value, raw[valueStart:valueStart+end])
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
		out.WriteString(soqlLiteral(value))
		if isCall {
			i = callEnd
		} else {
			i = j
		}
	}
	return out.String(), nil
}

func (vm *VM) queryLocatorFromSOQL(query string, result *Result) (Value, error) {
	value, err := vm.executeSOQL(query, result)
	if err != nil {
		return Null, err
	}
	locator := Object("Database.QueryLocator")
	locator.Fields["Records"] = value
	locator.Fields["Query"] = String(query)
	return locator, nil
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
	return out, nil
}
func (vm *VM) executeInlineSOQL(raw string, execResult *Result) (Value, error) {
	value, err := vm.executeSOQL(raw, execResult)
	if err != nil {
		return Null, err
	}
	if value.Kind == ValueList {
		if value.Fields == nil {
			value.Fields = make(map[string]Value)
		}
		value.Fields["__soqlQuery"] = String(raw)
	}
	if vm.inlineSOQLMayReturnScalarCount(raw) {
		if count, ok := aggregateCount(value); ok {
			return count, nil
		}
	}
	return value, nil
}
func (vm *VM) inlineSOQLMayReturnScalarCount(raw string) bool {
	query, err := soql.ParseAt(raw, vm.fakeNow)
	if err != nil {
		return !strings.Contains(strings.ToLower(raw), " group by ")
	}
	if len(query.GroupBy) > 0 || query.Having != nil {
		return false
	}
	return query.Count || len(query.Aggregates) == 1
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
	return out, nil
}
func (vm *VM) executeSOQLWithBindMapAccessLevel(raw string, binds Value, accessLevel Value, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRowsWithExpander(raw, execResult, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	}, binds, databaseAccessLevelSecurityMode(accessLevel))
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
	return out, nil
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
	query, err := soql.ParseAt(raw, vm.fakeNow)
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
		if collectionBase(typeName) == "List" {
			if value.Runtime == "" && value.Type != "" && !strings.EqualFold(value.Type, typeName) {
				value.Runtime = value.Type
			}
			value.Type = typeName
		}
		return value, nil
	}
	if typeName == "Integer" || typeName == "Long" {
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
	return vm.executeSOQLRowsWithExpander(raw, execResult, vm.expandSOQLBinds, typedMap("Map<String,Object>"), databaseAccessLevelSecurityMode(accessLevel))
}
func writeSOQLBindExpansion(out *strings.Builder, value Value, consumed string) {
	out.WriteString(soqlLiteral(value))
	if strings.TrimRight(consumed, " \t\n\r") != consumed {
		out.WriteByte(' ')
	}
}
func shouldEvaluateSOQLBindExpression(raw string, pos, callEnd int, isCall bool) bool {
	if isCall {
		pos = callEnd
	}
	for pos < len(raw) && (raw[pos] == ' ' || raw[pos] == '\t' || raw[pos] == '\n' || raw[pos] == '\r') {
		pos++
	}
	return pos < len(raw) && (raw[pos] == '[' || raw[pos] == '(' || raw[pos] == '.' || raw[pos] == '+')
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
func rewriteTrailingSOQLEqualsToIn(out *strings.Builder) {
	text := out.String()
	trimmed := strings.TrimRight(text, " \t\n\r")
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
	case "LAST_N_DAYS", "NEXT_N_DAYS", "N_DAYS_AGO", "LAST_N_WEEKS", "NEXT_N_WEEKS", "LAST_N_MONTHS", "NEXT_N_MONTHS", "LAST_N_YEARS", "NEXT_N_YEARS":
		return true
	default:
		return false
	}
}
