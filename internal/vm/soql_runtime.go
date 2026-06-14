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

func (vm *VM) parseSOQLAt(queryText string) (soql.Query, error) {
	month := 1
	if vm != nil && vm.Org != nil {
		month = soql.FiscalYearStartMonth(*vm.Org)
	}
	return soql.ParseAtWithFiscalYearStartMonth(queryText, vm.fakeNow, month)
}

func (vm *VM) executeSOQLRowsWithExpander(raw string, execResult *Result, expand func(string) (string, error), binds Value, accessLevelMode string) ([]Value, error) {
	return vm.executeSOQLRowsWithExpanderAndScope(raw, execResult, expand, binds, accessLevelMode, "")
}

func (vm *VM) executeSOQLRowsWithExpanderAndScope(raw string, execResult *Result, expand func(string) (string, error), binds Value, accessLevelMode, permissionSetID string) ([]Value, error) {
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
	query, err := vm.parseSOQLAt(queryText)
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
	if query.ForView || query.ForReference {
		vm.recordRecentlyViewedRows(query.Object, result.Records, query.ForView, query.ForReference)
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
	appendDurationTrace(
		execResult,
		"apex.soql",
		"apex.soql",
		traceStart,
		traceDurationSince(traceStartedAt),
		map[string]any{
			"query": queryText,
			"rows":  result.Rows,
		},
	)
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
		return Null, fmt.Errorf("Search.query expects query String")
	}
	if len(args) == 2 && args[1].Kind != ValueNull && !isDatabaseAccessLevelValue(args[1]) {
		return Null, fmt.Errorf("Search.query expects AccessLevel")
	}
	accessLevel := Null
	if len(args) == 2 {
		accessLevel = args[1]
	}
	return vm.executeSOSLWithAccessLevel(args[0].Text, nil, accessLevel)
}

func (vm *VM) searchFind(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("Search.find expects query String and optional access level")
	}
	if args[0].Kind != ValueString {
		return Null, fmt.Errorf("Search.find expects query String")
	}
	if len(args) == 2 && args[1].Kind != ValueNull && !isDatabaseAccessLevelValue(args[1]) {
		return Null, fmt.Errorf("Search.find expects AccessLevel")
	}
	accessLevel := Null
	if len(args) == 2 {
		accessLevel = args[1]
	}
	queryText, err := vm.expandSOQLBinds(args[0].Text)
	if err != nil {
		return Null, newExceptionError("QueryException", fmt.Sprintf("%s in query %q", err.Error(), args[0].Text))
	}
	if err := vm.incrementLimit("soslQueries", 1); err != nil {
		return Null, err
	}
	if err := validateSOSLSpellCorrectionOption(queryText); err != nil {
		return Null, err
	}
	if err := validateSOSLHostedSearchOptions(queryText); err != nil {
		return Null, err
	}
	withSnippet := soslHasSearchOption(queryText, "snippet")
	searchTerms := parseSOSLFindTerms(queryText)
	results := Object("Search.SearchResults")
	byObject := typedMap("Map<String,List<Search.SearchResult>>")
	if vm.Org != nil {
		objects, err := parseSOSLReturningObjects(queryText)
		if err != nil {
			return Null, err
		}
		for _, spec := range objects {
			objectName := spec.ObjectName
			if canonical, ok := vm.resolveObjectName(spec.ObjectName); ok {
				objectName = canonical
			}
			records, err := vm.soslRecordsForSpec(spec, objectName, parseSOSLSearchPatterns(queryText), parseSOSLSearchScope(queryText), "", accessLevel)
			if err != nil {
				return Null, err
			}
			key := mapKey(String(objectName))
			list := byObject.Map[key]
			if list.Kind != ValueList {
				list = typedList("List<Search.SearchResult>")
			}
			values := List()
			values.Type = "List<" + objectName + ">"
			for _, record := range records {
				value := vm.vmValueFromRecord(record)
				vm.applySOSLReturningFunctionAliases(&value, record, spec)
				if len(spec.Fields) > 0 {
					value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Object, spec.Fields)
					vm.hydrateQueriedRecordTypeRelationships(value)
				}
				values.List = append(values.List, value)
			}
			sortSOSLRows(values, spec.OrderBy)
			applySOSLReturningOffset(&values, spec)
			applySOSLReturningLimit(&values, spec)
			for _, value := range values.List {
				record := storage.Record{}
				if value.Kind == ValueObject {
					if id, ok := valueIDString(value.Fields["Id"]); ok {
						if found, foundOK := vm.findOrgRecord(objectName, storage.ID(id)); foundOK {
							record = found
						}
					}
				}
				row := Object("Search.SearchResult")
				row.Fields["sObject"] = value
				snippet, snippets := soslSnippetsForRecord(record, withSnippet, searchTerms)
				row.Fields["snippet"] = String(snippet)
				row.Fields["snippets"] = snippets
				list.List = append(list.List, row)
			}
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
	if !isSearchSuggestionOptionValue(args[2]) {
		return Null, fmt.Errorf("Search.suggest expects Search.SuggestionOption")
	}
	if len(args) == 4 && args[3].Kind != ValueNull && !isDatabaseAccessLevelValue(args[3]) {
		return Null, fmt.Errorf("Search.suggest expects AccessLevel")
	}
	accessLevel := Null
	if len(args) == 4 {
		accessLevel = args[3]
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

func (vm *VM) executeSOSL(raw string, execResult *Result) (Value, error) {
	return vm.executeSOSLWithAccessLevel(raw, execResult, Null)
}

func (vm *VM) executeSOSLWithAccessLevel(raw string, execResult *Result, accessLevel Value) (Value, error) {
	queryText, err := vm.expandSOQLBinds(raw)
	if err != nil {
		return Null, newExceptionError("QueryException", fmt.Sprintf("%s in query %q", err.Error(), raw))
	}
	if err := vm.incrementLimit("soslQueries", 1); err != nil {
		return Null, err
	}
	if err := validateSOSLSpellCorrectionOption(queryText); err != nil {
		return Null, err
	}
	if err := validateSOSLHostedSearchOptions(queryText); err != nil {
		return Null, err
	}
	pricebookID := soslPricebookID(queryText)
	objects, err := parseSOSLReturningObjects(queryText)
	if err != nil {
		return Null, err
	}
	groups := make([]Value, 0, len(objects))
	rowCount := 0
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
			records, err := vm.soslRecordsForSpec(spec, specObjectName, parseSOSLSearchPatterns(queryText), parseSOSLSearchScope(queryText), pricebookID, accessLevel)
			if err != nil {
				return Null, err
			}
			for _, record := range records {
				value := vm.vmValueFromRecord(record)
				vm.applySOSLReturningFunctionAliases(&value, record, spec)
				if len(spec.Fields) > 0 {
					value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Object, spec.Fields)
					vm.hydrateQueriedRecordTypeRelationships(value)
				}
				rows.List = append(rows.List, value)
			}
		}
		sortSOSLRows(rows, spec.OrderBy)
		applySOSLReturningOffset(&rows, spec)
		applySOSLReturningLimit(&rows, spec)
		rowCount += len(rows.List)
		groups = append(groups, rows)
	}
	appendTrace(execResult, "apex.sosl", "apex.sosl", map[string]any{
		"query": queryText,
		"rows":  rowCount,
	})
	return List(groups...), nil
}

type soslReturningObject struct {
	ObjectName      string
	Fields          map[string]bool
	FunctionAliases []soslReturningFunctionAlias
	Where           soslWhere
	OrderBy         []soslOrderBy
	Offset          int
	HasOffset       bool
	Limit           int
	HasLimit        bool
}

type soslReturningFunctionAlias struct {
	Func   string
	Source string
	Alias  string
}

type soslOrderBy struct {
	Field string
	Desc  bool
}

type soslWhere struct {
	Field       string
	Operator    string
	Value       string
	ValueIsNull bool
}

type soslSearchPattern struct {
	Term   string
	Prefix bool
}

type soslSearchScope string

const (
	soslSearchScopeAll   soslSearchScope = "all"
	soslSearchScopeName  soslSearchScope = "name"
	soslSearchScopeEmail soslSearchScope = "email"
	soslSearchScopePhone soslSearchScope = "phone"
)

func (vm *VM) soslRecordsForSpec(spec soslReturningObject, objectName string, patterns []soslSearchPattern, scope soslSearchScope, pricebookID string, accessLevel Value) ([]storage.Record, error) {
	if vm == nil || vm.Org == nil {
		return nil, nil
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
	if len(vm.fixedSearchResults) > 0 {
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
			if !ok || !soslRecordMatchesWhere(record, spec.Where) || !vm.soslRecordMatchesPricebook(objectName, record, pricebookID) {
				continue
			}
			records = append(records, record)
		}
		return records, nil
	}
	ids := make([]string, 0, len(state.Records))
	for id := range state.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		record := state.Records[storage.ID(id)]
		if !soslRecordMatchesWhere(record, spec.Where) || !vm.soslRecordMatchesPricebook(objectName, record, pricebookID) {
			continue
		}
		if !vm.soslRecordMatchesSearch(objectName, record, patterns, scope, accessLevel) {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

func (vm *VM) enforceSOSLAccess(objectName string, spec soslReturningObject, accessLevel Value) error {
	if databaseAccessLevelSecurityMode(accessLevel) != "USER_MODE" {
		return nil
	}
	permissionSetID := accessLevelPermissionSetID(accessLevel)
	if !vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", permissionSetID) {
		return newExceptionError("QueryException", fmt.Sprintf("sObject type '%s' is not supported by USER_MODE", objectName))
	}
	for field := range spec.Fields {
		if projection, ok := parseSOSLReturningFunctionAlias(field); ok {
			field = projection.Source
		}
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

func (vm *VM) soslRecordMatchesSearch(objectName string, record storage.Record, patterns []soslSearchPattern, scope soslSearchScope, accessLevel Value) bool {
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

func (vm *VM) soslRecordMatchesSearchPattern(objectName string, record storage.Record, pattern soslSearchPattern, scope soslSearchScope, accessLevel Value) bool {
	if pattern.Term == "" {
		return true
	}
	if scope == soslSearchScopeAll && soslTextMatchesPattern(string(record.ID), pattern) {
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

func soslFieldMatchesScope(fieldName string, field storage.Field, scope soslSearchScope) bool {
	switch scope {
	case "", soslSearchScopeAll:
		return true
	case soslSearchScopeName:
		return soslIsNameField(fieldName, field)
	case soslSearchScopeEmail:
		return soslIsEmailField(fieldName, field)
	case soslSearchScopePhone:
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

func soslTextMatchesPattern(text string, pattern soslSearchPattern) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if pattern.Prefix {
		return strings.HasPrefix(strings.ToLower(text), strings.ToLower(pattern.Term))
	}
	return containsFold(text, pattern.Term)
}

func parseSOSLSearchScope(query string) soslSearchScope {
	upper := strings.ToUpper(query)
	switch {
	case strings.Contains(upper, " IN NAME FIELDS"):
		return soslSearchScopeName
	case strings.Contains(upper, " IN EMAIL FIELDS"):
		return soslSearchScopeEmail
	case strings.Contains(upper, " IN PHONE FIELDS"):
		return soslSearchScopePhone
	default:
		return soslSearchScopeAll
	}
}

func parseSOSLSearchPatterns(query string) []soslSearchPattern {
	raw := rawSOSLFindText(query)
	if raw == "" {
		return nil
	}
	var patterns []soslSearchPattern
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == '?' || r == '"' || r == '\'' || r == '(' || r == ')' || r == '{' || r == '}' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		part = strings.TrimSpace(part)
		if part == "" || strings.EqualFold(part, "AND") || strings.EqualFold(part, "OR") {
			continue
		}
		prefix := strings.HasSuffix(part, "*")
		part = strings.Trim(part, "*")
		if part == "" {
			continue
		}
		patterns = append(patterns, soslSearchPattern{Term: part, Prefix: prefix})
	}
	return patterns
}

func rawSOSLFindText(query string) string {
	index := indexFold(query, "find")
	if index < 0 {
		return ""
	}
	rest := strings.TrimSpace(query[index+len("find"):])
	if rest == "" {
		return ""
	}
	switch rest[0] {
	case '{':
		if end := strings.IndexByte(rest[1:], '}'); end >= 0 {
			return rest[1 : end+1]
		}
	case '\'':
		if end := strings.IndexByte(rest[1:], '\''); end >= 0 {
			return rest[1 : end+1]
		}
	default:
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}

func (vm *VM) searchSuggestionRows(query, objectName string, option Value, accessLevel Value) (Value, error) {
	out := typedList("List<Search.SuggestionResult>")
	if vm == nil || vm.Org == nil {
		return out, nil
	}
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	state, ok := vm.Org.Objects[objectName]
	if !ok {
		return out, nil
	}
	spec := soslReturningObject{ObjectName: objectName, Fields: map[string]bool{"id": true, "name": true}}
	if err := vm.enforceSOSLAccess(objectName, spec, accessLevel); err != nil {
		return Null, err
	}
	limit := 10
	if option.Fields != nil {
		if value, ok := option.Fields["limit"]; ok && value.Kind == ValueInt && value.Int > 0 {
			limit = int(value.Int)
		}
	}
	pattern := soslSearchPattern{Term: strings.TrimSpace(query), Prefix: true}
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
		value := vm.vmValueFromRecord(record)
		value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Object, spec.Fields)
		row := Object("Search.SuggestionResult")
		row.Fields["sObject"] = value
		out.List = append(out.List, row)
	}
	return out, nil
}

func (vm *VM) soslRecordMatchesSuggestion(objectName string, record storage.Record, pattern soslSearchPattern, accessLevel Value) bool {
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

func soslHasSearchOption(query, option string) bool {
	for _, clause := range findSOSLWithClauses(query) {
		if strings.EqualFold(firstSOSLWord(clause), option) {
			return true
		}
	}
	return false
}

func validateSOSLSpellCorrectionOption(query string) error {
	for _, clause := range findSOSLWithClauses(query) {
		word := firstSOSLWord(clause)
		if !strings.EqualFold(word, "SPELL_CORRECTION") {
			continue
		}
		value := strings.TrimSpace(clause[len(word):])
		if strings.HasPrefix(value, "=") {
			value = strings.TrimSpace(value[1:])
		}
		if value == "" || strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
			continue
		}
		return newExceptionError("QueryException", "SOSL WITH SPELL_CORRECTION expects true or false")
	}
	return nil
}

func validateSOSLHostedSearchOptions(query string) error {
	unsupported := []struct {
		pattern string
		message string
	}{
		{`(?i)\bWITH\s+DATA\s+CATEGORY\b`, "Search.query SOSL WITH DATA CATEGORY hosted search service"},
		{`(?i)\bWITH\s+DIVISIONFILTER\b`, "Search.query SOSL WITH DivisionFilter hosted search service"},
		{`(?i)\bWITH\s+METADATA\b`, "Search.query SOSL WITH METADATA hosted search service"},
		{`(?i)\bUSING\s+LISTVIEW\b`, "Search.query SOSL USING ListView hosted search service"},
		{`(?i)\bUPDATE\s+TRACKING\b`, "Search.query SOSL UPDATE TRACKING hosted search analytics"},
		{`(?i)\bUPDATE\s+VIEWSTAT\b`, "Search.query SOSL UPDATE VIEWSTAT hosted search analytics"},
	}
	for _, candidate := range unsupported {
		if regexp.MustCompile(candidate.pattern).MatchString(query) {
			return unsupportedCallError(candidate.message)
		}
	}
	return nil
}

func findSOSLWithClauses(query string) []string {
	var clauses []string
	for i := 0; i < len(query); i++ {
		if !hasWordAtFold(query, i, "WITH") {
			continue
		}
		start := i + len("WITH")
		for start < len(query) && query[start] == ' ' {
			start++
		}
		end := len(query)
		for j := start; j < len(query); j++ {
			if hasWordAtFold(query, j, "WITH") || hasWordAtFold(query, j, "LIMIT") || hasWordAtFold(query, j, "OFFSET") {
				end = j
				break
			}
		}
		clauses = append(clauses, strings.TrimSpace(query[start:end]))
		i = end
	}
	return clauses
}

func hasWordAtFold(text string, index int, word string) bool {
	if index < 0 || index+len(word) > len(text) || !strings.EqualFold(text[index:index+len(word)], word) {
		return false
	}
	if index > 0 && isSOSLWordByte(text[index-1]) {
		return false
	}
	end := index + len(word)
	return end >= len(text) || !isSOSLWordByte(text[end])
}

func isSOSLWordByte(ch byte) bool {
	return ch == '_' || ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

func firstSOSLWord(text string) string {
	text = strings.TrimSpace(text)
	for i := 0; i < len(text); i++ {
		if !isSOSLWordByte(text[i]) {
			return text[:i]
		}
	}
	return text
}

func soslPricebookID(query string) string {
	for _, clause := range findSOSLWithClauses(query) {
		word := firstSOSLWord(clause)
		if !strings.EqualFold(word, "PricebookId") {
			continue
		}
		value := strings.TrimSpace(clause[len(word):])
		if strings.HasPrefix(value, "=") {
			value = strings.TrimSpace(value[1:])
		}
		if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			value = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
		}
		return value
	}
	return ""
}

func parseSOSLFindTerms(query string) []string {
	index := indexFold(query, "find")
	if index < 0 {
		return nil
	}
	rest := strings.TrimSpace(query[index+len("find"):])
	if rest == "" {
		return nil
	}
	var raw string
	switch rest[0] {
	case '{':
		if end := strings.IndexByte(rest[1:], '}'); end >= 0 {
			raw = rest[1 : end+1]
		}
	case '\'':
		if end := strings.IndexByte(rest[1:], '\''); end >= 0 {
			raw = rest[1 : end+1]
		}
	default:
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			raw = fields[0]
		}
	}
	if raw == "" {
		return nil
	}
	var terms []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == '*' || r == '?' || r == '"' || r == '\'' || r == '(' || r == ')' || r == '{' || r == '}' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		part = strings.TrimSpace(part)
		if part != "" && !strings.EqualFold(part, "AND") && !strings.EqualFold(part, "OR") {
			terms = append(terms, part)
		}
	}
	return terms
}

func soslSnippetsForRecord(record storage.Record, enabled bool, terms []string) (string, Value) {
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

func soslSnippetMatches(text string, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	for _, term := range terms {
		if containsFold(text, term) {
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

func parseSOSLReturningObjects(query string) ([]soslReturningObject, error) {
	match := regexp.MustCompile(`(?is)\bRETURNING\s+(.+?)(?:\s+LIMIT\s+\d+\s*)?$`).FindStringSubmatch(query)
	if len(match) != 2 {
		return nil, unsupportedCallError("Search.query SOSL RETURNING clause")
	}
	parts := splitTopLevelComma(trimSOSLReturningObjectsText(match[1]))
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
		spec.Where = parseSOSLReturningWhere(fields)
		spec.OrderBy = parseSOSLReturningOrderBy(fields)
		spec.Offset, spec.HasOffset = parseSOSLReturningOffset(fields)
		spec.Limit, spec.HasLimit = parseSOSLReturningLimit(fields)
		fields = trimSOSLReturningFieldList(fields)
		for _, field := range splitTopLevelComma(fields) {
			field = strings.TrimSpace(field)
			if field != "" {
				spec.Fields[strings.ToLower(field)] = true
				if projection, ok := parseSOSLReturningFunctionAlias(field); ok {
					spec.FunctionAliases = append(spec.FunctionAliases, projection)
					spec.Fields[strings.ToLower(projection.Alias)] = true
				}
			}
		}
		out = append(out, spec)
	}
	return out, nil
}

func trimSOSLReturningObjectsText(text string) string {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && hasWordAtFold(text, i, "WITH") {
				return strings.TrimSpace(text[:i])
			}
		}
	}
	return strings.TrimSpace(text)
}

func parseSOSLReturningWhere(fields string) soslWhere {
	index := lastIndexFoldOutsideQuotes(fields, " where ")
	if index < 0 {
		return soslWhere{}
	}
	whereText := fields[index+len(" where "):]
	end := len(whereText)
	for _, marker := range []string{" order by ", " offset ", " limit "} {
		if markerIndex := indexFoldOutsideQuotes(whereText, marker); markerIndex >= 0 && markerIndex < end {
			end = markerIndex
		}
	}
	whereText = strings.TrimSpace(whereText[:end])
	operator, operatorIndex := soslWhereOperator(whereText)
	if operatorIndex <= 0 {
		return soslWhere{}
	}
	field := strings.TrimSpace(whereText[:operatorIndex])
	value := strings.TrimSpace(whereText[operatorIndex+len(operator):])
	valueIsNull := strings.EqualFold(value, "null")
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		value = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return soslWhere{Field: field, Operator: operator, Value: value, ValueIsNull: valueIsNull}
}

func soslWhereOperator(whereText string) (string, int) {
	for i := 0; i < len(whereText); i++ {
		if soslQuotedAt(whereText, i) {
			i = skipSOSLQuoted(whereText, i)
			continue
		}
		if i+1 < len(whereText) && whereText[i] == '!' && whereText[i+1] == '=' {
			return "!=", i
		}
		if whereText[i] == '=' {
			return "=", i
		}
	}
	return "", -1
}

func soslRecordMatchesWhere(record storage.Record, where soslWhere) bool {
	if strings.TrimSpace(where.Field) == "" {
		return true
	}
	value, ok := record.GetField(where.Field)
	matches := false
	if where.ValueIsNull {
		matches = !ok || value.Kind == storage.ValueNull
	} else if ok {
		matches = strings.EqualFold(storageValueText(value), where.Value)
	}
	if where.Operator == "!=" {
		return !matches
	}
	return matches
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

func (vm *VM) applySOSLReturningFunctionAliases(value *Value, record storage.Record, spec soslReturningObject) {
	if value == nil || value.Kind != ValueObject || len(spec.FunctionAliases) == 0 {
		return
	}
	for _, alias := range spec.FunctionAliases {
		stored, found := record.GetField(alias.Source)
		if !found {
			continue
		}
		switch alias.Func {
		case "FORMAT":
			value.Fields[alias.Alias] = String(storageValueText(stored))
		case "CONVERTCURRENCY":
			value.Fields[alias.Alias] = vmValueFromStorage(stored)
		case "TOLABEL":
			value.Fields[alias.Alias] = String(vm.soslToLabel(record.Object, alias.Source, stored))
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

func parseSOSLReturningOrderBy(fields string) []soslOrderBy {
	lowered := strings.ToLower(fields)
	index := strings.Index(lowered, " order by ")
	if index < 0 {
		return nil
	}
	orderText := fields[index+len(" order by "):]
	orderLowered := strings.ToLower(orderText)
	end := len(orderText)
	for _, marker := range []string{" offset ", " limit "} {
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

func parseSOSLReturningLimit(fields string) (int, bool) {
	return parseSOSLReturningIntegerClause(fields, " limit ")
}

func parseSOSLReturningOffset(fields string) (int, bool) {
	return parseSOSLReturningIntegerClause(fields, " offset ")
}

func parseSOSLReturningIntegerClause(fields, marker string) (int, bool) {
	index := lastIndexFold(fields, marker)
	if index < 0 {
		return 0, false
	}
	start := index + len(marker)
	for start < len(fields) && fields[start] == ' ' {
		start++
	}
	end := start
	for end < len(fields) && fields[end] >= '0' && fields[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	limit, err := strconv.Atoi(fields[start:end])
	if err != nil {
		return 0, false
	}
	return limit, true
}

func lastIndexFold(s, substr string) int {
	if substr == "" {
		return len(s)
	}
	for i := len(s) - len(substr); i >= 0; i-- {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}

func lastIndexFoldOutsideQuotes(s, substr string) int {
	if substr == "" {
		return len(s)
	}
	last := -1
	for i := 0; i <= len(s)-len(substr); i++ {
		if soslQuotedAt(s, i) {
			i = skipSOSLQuoted(s, i)
			continue
		}
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			last = i
		}
	}
	return last
}

func indexFold(s, substr string) int {
	if substr == "" {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}

func indexFoldOutsideQuotes(s, substr string) int {
	if substr == "" {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if soslQuotedAt(s, i) {
			i = skipSOSLQuoted(s, i)
			continue
		}
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}

func soslQuotedAt(text string, index int) bool {
	return index >= 0 && index < len(text) && text[index] == '\''
}

func skipSOSLQuoted(text string, index int) int {
	for i := index + 1; i < len(text); i++ {
		if text[i] != '\'' {
			continue
		}
		if i+1 < len(text) && text[i+1] == '\'' {
			i++
			continue
		}
		return i
	}
	return len(text) - 1
}

func trimSOSLReturningFieldList(fields string) string {
	lowered := strings.ToLower(fields)
	end := len(fields)
	for _, marker := range []string{" where ", " order by ", " offset ", " limit "} {
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

func applySOSLReturningOffset(rows *Value, spec soslReturningObject) {
	if rows == nil || rows.Kind != ValueList || !spec.HasOffset || spec.Offset <= 0 {
		return
	}
	if spec.Offset >= len(rows.List) {
		rows.List = nil
		return
	}
	rows.List = rows.List[spec.Offset:]
}

func applySOSLReturningLimit(rows *Value, spec soslReturningObject) {
	if rows == nil || rows.Kind != ValueList || !spec.HasLimit || spec.Limit >= len(rows.List) {
		return
	}
	rows.List = rows.List[:spec.Limit]
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
	projection, ok := parseSOSLReturningFunctionAlias(field)
	if ok {
		return []string{projection.Source, projection.Alias}
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

func parseSOSLReturningFunctionAlias(field string) (soslReturningFunctionAlias, bool) {
	text := strings.TrimSpace(field)
	parts := strings.Fields(text)
	if len(parts) != 2 {
		return soslReturningFunctionAlias{}, false
	}
	raw := parts[0]
	open := strings.IndexByte(raw, '(')
	if open <= 0 || !strings.HasSuffix(raw, ")") {
		return soslReturningFunctionAlias{}, false
	}
	if !isSelectedSOQLFieldFunction(raw[:open]) {
		return soslReturningFunctionAlias{}, false
	}
	argsText := raw[open+1 : len(raw)-1]
	if strings.TrimSpace(argsText) == "" || strings.Contains(argsText, ",") {
		return soslReturningFunctionAlias{}, false
	}
	fieldArg := selectedSOQLFunctionFieldArg(argsText)
	if fieldArg == "" || strings.ContainsAny(fieldArg, " (),") {
		return soslReturningFunctionAlias{}, false
	}
	return soslReturningFunctionAlias{Func: strings.ToUpper(strings.TrimSpace(raw[:open])), Source: fieldArg, Alias: parts[1]}, true
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
		for _, fieldName := range vm.securityFieldNames(objectName, field) {
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
		for _, fieldName := range vm.securityFieldNames(objectName, order.Field) {
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
	if !vm.soqlRelationshipSecurityTargetsAllowed(targets, parts[1:], permissionSetID) {
		return newExceptionError("QueryException", fmt.Sprintf("No such column '%s' on entity '%s' for %s", expression, objectName, mode))
	}
	return nil
}

func (vm *VM) soqlRelationshipSecurityTargetsAllowed(targets []string, parts []string, permissionSetID string) bool {
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
		if !vm.soqlRelationshipSecurityTargetsAllowed(nestedTargets, parts[1:], permissionSetID) {
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
	if err := vm.incrementQueryLocatorRows(value); err != nil {
		return Null, err
	}
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
		if collectionBase(typeName) == "List" {
			if value.Runtime == "" && value.Type != "" && !strings.EqualFold(value.Type, typeName) {
				value.Runtime = value.Type
			}
			value.Type = typeName
		}
		return value, nil
	}
	if strings.EqualFold(typeName, "Integer") || strings.EqualFold(typeName, "Long") {
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
