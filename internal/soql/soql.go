package soql

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

var parsedQueryCache sync.Map

const virtualSchemaHydrationStampKey = "__glade_virtual_schema_hydration_stamp"

func cachedParsedQuery(input string, now time.Time, fiscalYearStartMonth int) (Query, bool) {
	return cachedParsedQueryWithTimeZone(input, now, fiscalYearStartMonth, "UTC")
}

func cachedParsedQueryWithTimeZone(input string, now time.Time, fiscalYearStartMonth int, timeZoneID string) (Query, bool) {
	value, ok := parsedQueryCache.Load(parsedQueryCacheKey(input, now, fiscalYearStartMonth, timeZoneID))
	if !ok {
		return Query{}, false
	}
	query, ok := value.(Query)
	if !ok {
		return Query{}, false
	}
	return cloneQueryForCacheHit(query), true
}

func storeParsedQuery(input string, now time.Time, fiscalYearStartMonth int, query Query) {
	storeParsedQueryWithTimeZone(input, now, fiscalYearStartMonth, "UTC", query)
}

func storeParsedQueryWithTimeZone(input string, now time.Time, fiscalYearStartMonth int, timeZoneID string, query Query) {
	parsedQueryCache.Store(parsedQueryCacheKey(input, now, fiscalYearStartMonth, timeZoneID), cloneQuery(query))
}

func parsedQueryCacheKey(input string, now time.Time, fiscalYearStartMonth int, timeZoneID string) string {
	return now.UTC().Truncate(time.Minute).Format("2006-01-02T15:04") + "\x00" + strconv.Itoa(normalizeFiscalYearStartMonth(fiscalYearStartMonth)) + "\x00" + timeZoneID + "\x00" + input
}

func cloneQuery(query Query) Query {
	query.Fields = append([]string(nil), query.Fields...)
	query.ChildQueries = append([]ChildQuery(nil), query.ChildQueries...)
	for i := range query.ChildQueries {
		query.ChildQueries[i].Query = cloneQuery(query.ChildQueries[i].Query)
	}
	query.Typeofs = append([]TypeofSpec(nil), query.Typeofs...)
	for i := range query.Typeofs {
		query.Typeofs[i].When = cloneStringSliceMap(query.Typeofs[i].When)
		query.Typeofs[i].Else = append([]string(nil), query.Typeofs[i].Else...)
	}
	query.Order = append([]OrderSpec(nil), query.Order...)
	query.GroupBy = append([]string(nil), query.GroupBy...)
	query.Aggregates = append([]Aggregate(nil), query.Aggregates...)
	query.HavingAggregates = append([]Aggregate(nil), query.HavingAggregates...)
	if query.Where != nil {
		where := cloneCondition(*query.Where)
		query.Where = &where
	}
	if query.Having != nil {
		having := cloneCondition(*query.Having)
		query.Having = &having
	}
	return query
}

func cloneQueryForCacheHit(query Query) Query {
	query.Fields = append([]string(nil), query.Fields...)
	query.ChildQueries = append([]ChildQuery(nil), query.ChildQueries...)
	for i := range query.ChildQueries {
		query.ChildQueries[i].Query = cloneQueryForCacheHit(query.ChildQueries[i].Query)
	}
	query.Typeofs = append([]TypeofSpec(nil), query.Typeofs...)
	for i := range query.Typeofs {
		query.Typeofs[i].When = cloneStringSliceMap(query.Typeofs[i].When)
		query.Typeofs[i].Else = append([]string(nil), query.Typeofs[i].Else...)
	}
	query.Order = append([]OrderSpec(nil), query.Order...)
	query.GroupBy = append([]string(nil), query.GroupBy...)
	query.Aggregates = append([]Aggregate(nil), query.Aggregates...)
	query.HavingAggregates = append([]Aggregate(nil), query.HavingAggregates...)
	if query.Where != nil {
		where := *query.Where
		if conditionContainsSubquery(where) || conditionHasFormula(&where) {
			where = cloneCondition(where)
		}
		query.Where = &where
	}
	if query.Having != nil {
		having := *query.Having
		if conditionContainsSubquery(having) || conditionHasFormula(&having) {
			having = cloneCondition(having)
		}
		query.Having = &having
	}
	return query
}

func cloneCondition(condition Condition) Condition {
	if condition.Formula != nil {
		formula := *condition.Formula
		condition.Formula = &formula
	}
	condition.And = append([]Condition(nil), condition.And...)
	for i := range condition.And {
		condition.And[i] = cloneCondition(condition.And[i])
	}
	condition.Or = append([]Condition(nil), condition.Or...)
	for i := range condition.Or {
		condition.Or[i] = cloneCondition(condition.Or[i])
	}
	condition.Values = append([]storage.Value(nil), condition.Values...)
	if condition.Subquery != nil {
		subquery := cloneQuery(*condition.Subquery)
		condition.Subquery = &subquery
	}
	return condition
}

func conditionContainsSubquery(condition Condition) bool {
	if condition.Subquery != nil {
		return true
	}
	for _, child := range condition.And {
		if conditionContainsSubquery(child) {
			return true
		}
	}
	for _, child := range condition.Or {
		if conditionContainsSubquery(child) {
			return true
		}
	}
	return false
}

func cloneStringSliceMap(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, value := range in {
		out[key] = append([]string(nil), value...)
	}
	return out
}

func NewExecutionCache() *ExecutionCache {
	return &ExecutionCache{}
}

func (e *UnsupportedFeatureError) Error() string {
	return e.Message
}

func unsupportedSOQLErrorf(format string, args ...any) error {
	return &UnsupportedFeatureError{Message: fmt.Sprintf("soql: "+format, args...)}
}

func Parse(input string) (Query, error) {
	return ParseAt(input, time.Now().UTC())
}

func IsSOSLFind(input string) bool {
	return firstQueryWord(input) == "FIND"
}

func firstQueryWord(input string) string {
	input = strings.TrimLeft(input, " \t\r\n\f")
	if input == "" {
		return ""
	}
	end := 0
	for end < len(input) {
		ch := input[end]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' {
			end++
			continue
		}
		break
	}
	return strings.ToUpper(input[:end])
}

func ParseAt(input string, now time.Time) (Query, error) {
	return ParseAtWithFiscalYearStartMonth(input, now, 1)
}

func ParseAtWithFiscalYearStartMonth(input string, now time.Time, fiscalYearStartMonth int) (Query, error) {
	return ParseAtWithFiscalYearStartMonthAndTimeZone(input, now, fiscalYearStartMonth, "UTC")
}

// ParseAtWithFiscalYearStartMonthAndTimeZone parses a query using the
// execution user's timezone for date-literal ranges. Date literals are
// calendar ranges in that timezone when matched against Datetime fields.
func ParseAtWithFiscalYearStartMonthAndTimeZone(input string, now time.Time, fiscalYearStartMonth int, timeZoneID string) (Query, error) {
	return ParseAtWithFiscalYearStartMonthTimeZoneAndWeekStart(input, now, fiscalYearStartMonth, timeZoneID, time.Sunday)
}

// ParseAtWithFiscalYearStartMonthTimeZoneAndWeekStart also sets the weekday
// that begins the week date literals (the user locale's first day of week).
func ParseAtWithFiscalYearStartMonthTimeZoneAndWeekStart(input string, now time.Time, fiscalYearStartMonth int, timeZoneID string, firstWeekday time.Weekday) (Query, error) {
	now = now.UTC()
	fiscalYearStartMonth = normalizeFiscalYearStartMonth(fiscalYearStartMonth)
	if strings.TrimSpace(timeZoneID) == "" {
		timeZoneID = "UTC"
	}
	cacheZone := timeZoneID
	if firstWeekday != time.Sunday {
		cacheZone += "\x01" + firstWeekday.String()
	}
	if query, ok := cachedParsedQueryWithTimeZone(input, now, fiscalYearStartMonth, cacheZone); ok {
		return query, nil
	}
	tokens, err := lex(input)
	if err != nil {
		return Query{}, err
	}
	p := parser{tokens: tokens, now: dateLiteralNow(now, timeZoneID), fiscalYearStartMonth: fiscalYearStartMonth, dateLiteralTimeZoneID: timeZoneID, firstWeekday: firstWeekday}
	query, err := p.parseQuery()
	if err != nil {
		return Query{}, err
	}
	query.TimeZoneID = timeZoneID
	storeParsedQueryWithTimeZone(input, now, fiscalYearStartMonth, cacheZone, query)
	return query, nil
}

func Execute(org storage.OrgState, query Query) (Result, error) {
	return ExecuteWithCache(org, query, nil)
}

func ExecuteWithCache(org storage.OrgState, query Query, cache *ExecutionCache) (Result, error) {
	return ExecuteWithCacheAndContext(org, query, cache, ExecutionContext{})
}

// ExecuteWithCacheAndContext binds execution state independently of cached query
// syntax and schema resolution.
func ExecuteWithCacheAndContext(org storage.OrgState, query Query, cache *ExecutionCache, context ExecutionContext) (Result, error) {
	org = org.WithFormulaContext(context.formulaUserID(), context.PermissionUserFields)
	if context.APIVersion != "" {
		org.APIVersion = context.APIVersion
	}
	if err := ValidateFormulaVersion(query, org.APIVersion); err != nil {
		return Result{}, err
	}
	if err := validateSemiAntiQuery(query); err != nil {
		return Result{}, err
	}
	var err error
	query, err = prepareAggregateOrder(query)
	if err != nil {
		return Result{}, err
	}
	if err := validateAggregateQuery(query); err != nil {
		return Result{}, err
	}
	query = withTextCollation(query, context.Locale)
	if strings.EqualFold(query.Object, "PlatformCachePartition") {
		if _, ok := org.Objects["PlatformCachePartition"]; !ok {
			org = org.Clone()
			storage.EnsureStandardObject(&org, "PlatformCachePartition")
		}
	}
	if strings.EqualFold(query.Object, "KnowledgeArticleVersion") {
		org = *storage.KnowledgeArticleVersionReadView(&org)
	}
	objectName, ok := storage.ResolveObjectName(org, query.Object)
	if !ok && capturedVersionedRootIDQuery(query) {
		org, objectName, ok = orgWithLazyChildObject(org, query.Object)
	}
	if !ok && shouldHydrateVirtualSchemaForQuery(query.Object) {
		org = virtualSchemaQueryOrg(org, query, cache)
		objectName, ok = storage.ResolveObjectName(org, query.Object)
	} else if ok && shouldHydrateVirtualSchemaForQuery(objectName) {
		org = virtualSchemaQueryOrg(org, query, cache)
		objectName, ok = storage.ResolveObjectName(org, query.Object)
	}
	if !ok {
		// Retain the executor's diagnostic while
		// supplying the native QueryException text to Apex runtime callers.
		return Result{}, queryError(MissingObjectMessage(query.Object), "unknown object "+query.Object)
	}
	org = orgWithTypeofActivityTargets(org, objectName, query)
	object := org.Objects[objectName]
	if err := validateSemiAntiJoinReferences(org, objectName, query, object.Definition); err != nil {
		return Result{}, err
	}
	if len(query.Fields) == 0 && len(query.ChildQueries) == 0 && len(query.Typeofs) == 0 {
		return Result{}, fmt.Errorf("soql: SELECT requires at least one field")
	}
	fields, err := ExpandFieldsFunctions(object.Definition, query.Fields)
	if err != nil {
		return Result{}, err
	}
	query.Fields = fields
	if err := validateAggregateQuery(query); err != nil {
		return Result{}, err
	}
	if len(query.GroupBy) > 0 && len(query.Aggregates) == 0 && len(query.HavingAggregates) == 0 {
		if err := validateGroupedSelectedFields(query, false); err != nil {
			return Result{}, err
		}
	}
	var childCache *childRelationshipQueryCache
	if len(query.ChildQueries) > 0 {
		childCache = newChildRelationshipQueryCache(cache)
	}
	if err := validateQueryReferencesWithChildCache(org, object.Definition, query, query.SecurityMode, childCache); err != nil {
		return Result{}, err
	}
	if len(query.ChildQueries) > 0 && queryHasAggregates(query) {
		return Result{}, unsupportedSOQLErrorf("child relationship subqueries are not supported in aggregate queries")
	}
	if query.ForUpdate && queryHasAggregates(query) {
		return Result{}, unsupportedSOQLErrorf("FOR UPDATE is not supported with aggregate queries")
	}
	scopeOwnerField, scopeUserID, err := usingScopeOwnerFilter(object.Definition, query.UsingScope, context)
	if err != nil {
		return Result{}, err
	}
	if query.Where != nil && conditionContainsSubquery(*query.Where) {
		condition, err := resolveSubqueries(org, cloneCondition(*query.Where), context)
		if err != nil {
			return Result{}, err
		}
		query.Where = &condition
	}
	if query.Having != nil && conditionContainsSubquery(*query.Having) {
		condition, err := resolveSubqueries(org, cloneCondition(*query.Having), context)
		if err != nil {
			return Result{}, err
		}
		query.Having = &condition
	}

	ids := candidateRecordIDs(object, query.Where, query.AllRows)
	sort.Strings(ids)
	filterKnowledgeHistory := knowledgeArchivedLatestFilter(objectName, query.Where)

	predicateScratch := &predicateScratch{}
	matchedRecords := make([]storage.Record, 0, len(ids))
	for _, idText := range ids {
		record := object.Records[storage.ID(idText)]
		if recordHiddenFromSOQL(object.Definition, record) {
			continue
		}
		if record.System.IsDeleted && !query.AllRows {
			continue
		}
		if filterKnowledgeHistory {
			latest, ok := record.GetField("IsLatestVersion")
			if ok && latest.Kind == storage.ValueBoolean && !latest.Boolean {
				continue
			}
		}
		record.Object = objectName
		if query.DataCategoryGroup != "" && !matchesKnowledgeDataCategory(org, record, query) {
			continue
		}
		if scopeOwnerField != "" {
			owner, _ := recordValue(org, object.Definition, record, scopeOwnerField)
			if !storage.IDsEqual(idFromValue(owner), scopeUserID) {
				continue
			}
		}
		if matchesWithScratch(org, object.Definition, record, query.Where, context, predicateScratch) {
			if context.WindowRecordVisible != nil && (query.HasLimit || query.HasOffset) && !queryHasAggregates(query) && !context.WindowRecordVisible(objectName, callbackRecord(objectName, record)) {
				continue
			}
			matchedRecords = append(matchedRecords, record)
		}
	}
	if queryHasAggregates(query) {
		// Scalar COUNT() windows the source rows, while
		// grouped and field aggregates window the aggregate result rows.
		scalarCount := query.Count && len(query.GroupBy) == 0 && query.Having == nil
		if scalarCount {
			matchedRecords = applyWindow(matchedRecords, query.Offset, query.Limit, query.HasLimit)
		}
		records, err := aggregateRecords(org, object.Definition, matchedRecords, query, context.formulaActor())
		if err != nil {
			return Result{}, err
		}
		if len(query.Order) > 0 {
			order := collateOrder(query.Order, query.textCollation)
			sort.SliceStable(records, func(i, j int) bool {
				return aggregateOrderedBefore(records[i], records[j], order)
			})
		}
		if !scalarCount {
			records = applyWindow(records, query.Offset, query.Limit, query.HasLimit)
		}
		for _, record := range records {
			removeHiddenAggregateFields(record.Fields, query.orderAggregates)
			removeUnselectedGroupFields(record.Fields, query, org)
		}
		return Result{
			Records: records, Rows: len(records),
			AggregateInputRows: aggregateQueryInputRows(org, object.Definition, matchedRecords, query, context.formulaActor()),
		}, nil
	}
	if len(query.Order) > 0 {
		sortRecordsByOrder(org, object.Definition, matchedRecords, collateOrder(query.Order, query.textCollation), query.LegacyPicklistNullOrder, query.LegacyRestrictedPicklistNullOrder, context.formulaActor())
	}
	matchedRecords = applyWindow(matchedRecords, query.Offset, query.Limit, query.HasLimit)
	if query.ForUpdate {
		for i := range matchedRecords {
			matchedRecords[i].System.Locked = true
		}
	}
	records := make([]storage.Record, 0, len(matchedRecords))
	var projection []projectionField
	if len(matchedRecords) > 0 {
		projection = prepareProjectionFields(query.Fields)
	}
	for _, record := range matchedRecords {
		projected, err := projectRecord(org, object.Definition, record, projection, query.ChildQueries, query.Typeofs, childCache, context)
		if err != nil {
			return Result{}, err
		}
		if query.ForUpdate {
			projected.System.Locked = true
		}
		records = append(records, projected)
	}
	projectEventSubscriberRows(object.Definition.APIName, matchedRecords, records)
	return Result{Records: records, Rows: len(records)}, nil
}

func validateSemiAntiJoinReferences(org storage.OrgState, outerObjectName string, query Query, outer storage.ObjectDefinition) error {
	joins, _, err := semiAntiConditions(query.Where)
	if err != nil {
		return err
	}
	for _, join := range joins {
		outerTargets, ok := semiAntiReferenceTargets(org, outerObjectName, outer, join.Field)
		if !ok {
			if rejection := semiJoinRuntimeOperandRejection(org, outerObjectName, outer, join.Field); rejection != nil {
				return rejection
			}
			return fmt.Errorf("soql: semi-join main operand %s must be an ID or reference field", join.Field)
		}
		subquery := join.Subquery
		subqueryObject, ok := storage.ResolveObjectName(org, subquery.Object)
		if !ok {
			continue // The ordinary object validator reports the unknown subquery object.
		}
		if strings.EqualFold(outerObjectName, subqueryObject) {
			return fmt.Errorf("soql: semi-join and anti-join subqueries cannot query the same object as the outer query")
		}
		subqueryDefinition := org.Objects[subqueryObject].Definition
		subqueryTargets, ok := semiAntiReferenceTargets(org, subqueryObject, subqueryDefinition, subquery.Fields[0])
		if !ok {
			return fmt.Errorf("soql: semi-join subquery field %s must be an ID or reference field", subquery.Fields[0])
		}
		if !semiAntiTargetsOverlap(outerTargets, subqueryTargets) {
			return fmt.Errorf("soql: semi-join subquery field %s must reference the outer object", subquery.Fields[0])
		}
	}
	return nil
}

func semiAntiReferenceTargets(org storage.OrgState, objectName string, definition storage.ObjectDefinition, fieldName string) ([]string, bool) {
	fieldName = SemiJoinOperandField(objectName, fieldName)
	if strings.Contains(fieldName, ".") {
		return nil, false
	}
	if strings.EqualFold(fieldName, "Id") {
		return []string{semiAntiCanonicalObjectTarget(org, objectName)}, true
	}
	_, field, ok := storage.ResolveFieldDefinition(definition, org.Namespace, fieldName)
	if !ok {
		field, ok = fieldDefinitionsForReferenceField(definition, fieldName)
	}
	if !ok || field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
		return nil, false
	}
	targets := make([]string, 0, len(field.ReferenceTo))
	for _, target := range field.ReferenceTo {
		targets = append(targets, semiAntiCanonicalObjectTarget(org, target))
	}
	return targets, true
}

func semiAntiCanonicalObjectTarget(org storage.OrgState, target string) string {
	if resolved, ok := storage.ResolveObjectName(org, target); ok {
		return strings.ToLower(resolved)
	}
	return strings.ToLower(strings.TrimSpace(target))
}

func semiAntiTargetsOverlap(left, right []string) bool {
	for _, leftTarget := range left {
		for _, rightTarget := range right {
			if strings.EqualFold(leftTarget, rightTarget) {
				return true
			}
		}
	}
	return false
}

func recordHiddenFromSOQL(definition storage.ObjectDefinition, record storage.Record) bool {
	if strings.EqualFold(definition.APIName, "Knowledge__DataCategorySelection") && record.System.IsDeleted {
		return true
	}
	if !strings.EqualFold(definition.APIName, "AsyncApexJob") {
		return false
	}
	return record.System.HiddenFromSOQL
}

// VirtualSchemaView returns a copy of org with the virtual schema objects
// (EntityDefinition, FieldDefinition and the access views) populated, the same
// view an ordinary SOQL read of those objects uses.
func VirtualSchemaView(org storage.OrgState) storage.OrgState {
	view := org.Clone()
	hydrateVirtualSchemaObjects(&view)
	return view
}

func shouldHydrateVirtualSchemaForQuery(objectName string) bool {
	return isVirtualSchemaObjectName(objectName) || isUserAccessVirtualSchemaObjectName(objectName)
}

// callbackRecord is the record handed to an ExecutionContext callback. A
// virtual schema row may belong to the generation's shared view, so a caller
// gets its own copy; every other record is the caller's own org state.
func callbackRecord(objectName string, record storage.Record) storage.Record {
	if shouldHydrateVirtualSchemaForQuery(objectName) {
		return record.Clone()
	}
	return record
}

var virtualSchemaObjectNames = [...]string{"EntityDefinition", "EntityParticle", "FieldDefinition", "RelationshipDomain", "UserEntityAccess", "UserFieldAccess"}

// sharedVirtualSchema is one runtime schema generation's virtual schema
// objects. Their rows derive only from the generation's object definitions,
// which are frozen while the org carries its RuntimeSchemaStamp (every
// definition writer clears it), so they can be shared read-only by every
// query of every VM on that generation. definitions holds the six object
// states as EnsureStandardObject prepares them, with no rows; tables holds the
// hydrated states, built one table at a time as queries need them, so a run
// that only reads EntityDefinition never materialises the per-field tables.
// tables is replaced, never mutated, so a reader keeps a consistent snapshot.
type sharedVirtualSchema struct {
	runtimeStamp   string
	hydrationStamp string
	definitions    map[string]storage.ObjectState
	tables         map[string]storage.ObjectState
}

// virtualSchemaBuilds counts hydrated virtual schema tables; tests read it to
// prove that concurrent first queries build each table once.
var virtualSchemaBuilds atomic.Int64

// virtualSchemaQueryOrg returns the org a virtual schema query executes
// against: the caller's objects plus the six virtual schema objects hydrated
// from the caller's definitions. A frozen generation hydrates each table once
// per execution cache and reuses it; an org without a runtime stamp, or one
// that already holds records for a virtual schema object (explicit records
// win over hydration), hydrates a private view as before.
func virtualSchemaQueryOrg(org storage.OrgState, query Query, cache *ExecutionCache) storage.OrgState {
	runtimeStamp := strings.TrimSpace(org.RuntimeSchemaStamp)
	if cache == nil || runtimeStamp == "" || virtualSchemaObjectsHaveRecords(org) {
		return hydratedVirtualSchemaView(org)
	}
	hydrationStamp := virtualSchemaHydrationStamp(org)
	shared := cache.sharedVirtualSchema(org, runtimeStamp, hydrationStamp)
	base := canonicalVirtualSchemaObjectName(query.Object)
	want := virtualSchemaTablesForQuery(org, shared.definitions[base].Definition, base, query)
	tables := cache.hydratedVirtualSchemaTables(org, shared, want)
	out := org.CloneObjectsShallow(len(virtualSchemaObjectNames))
	for _, name := range virtualSchemaObjectNames {
		if table, ok := tables[name]; ok && want[name] {
			out.Objects[name] = table
			continue
		}
		definition := shared.definitions[name]
		definition.Records = map[storage.ID]storage.Record{}
		definition.Indexes = map[string]storage.IndexSet{}
		out.Objects[name] = definition
	}
	return out
}

// hydratedVirtualSchemaView hydrates the virtual schema objects on a private
// view of org. Hydration reads every ordinary object and writes only the six
// virtual object states, so the view shares the ordinary states and owns a
// deep copy of any virtual state the caller already holds (explicit records
// win over hydration, and the hydration stamp lands on the copy's definition).
func hydratedVirtualSchemaView(org storage.OrgState) storage.OrgState {
	out := org.CloneObjectsShallow(len(virtualSchemaObjectNames))
	for _, name := range virtualSchemaObjectNames {
		if object, ok := out.Objects[name]; ok {
			object = object.Clone()
			object.RecordsShared, object.IndexesShared = false, false
			out.Objects[name] = object
		}
	}
	hydrateVirtualSchemaObjects(&out)
	return out
}

func virtualSchemaObjectsHaveRecords(org storage.OrgState) bool {
	for _, name := range virtualSchemaObjectNames {
		if object, ok := org.Objects[name]; ok && len(object.Records) > 0 {
			return true
		}
	}
	return false
}

func canonicalVirtualSchemaObjectName(name string) string {
	for _, candidate := range virtualSchemaObjectNames {
		if strings.EqualFold(candidate, strings.TrimSpace(name)) {
			return candidate
		}
	}
	return name
}

func allVirtualSchemaTables() map[string]bool {
	all := make(map[string]bool, len(virtualSchemaObjectNames))
	for _, name := range virtualSchemaObjectNames {
		all[name] = true
	}
	return all
}

// virtualSchemaTablesForQuery returns the virtual schema tables a query can
// read: the queried table and the parents its one-hop relationship paths
// reach. Anything the walk does not fully understand (child queries, TYPEOF,
// subqueries, formula predicates, multi-hop or unresolved paths, polymorphic
// relationships) answers every table, which is what the eager hydration gave.
func virtualSchemaTablesForQuery(org storage.OrgState, definition storage.ObjectDefinition, base string, query Query) map[string]bool {
	want := map[string]bool{base: true}
	if len(query.ChildQueries) > 0 || len(query.Typeofs) > 0 || definition.APIName == "" {
		return allVirtualSchemaTables()
	}
	complete := true
	var path func(field string)
	path = func(field string) {
		if !complete {
			return
		}
		field = strings.TrimSpace(field)
		if field == "" {
			return
		}
		if aggregate, ok, err := parseAggregateField(field); err == nil && ok {
			field = aggregate.Field
			if field == "" {
				return
			}
		}
		if expr, ok := parseSelectFieldExpression(field); ok {
			for _, arg := range expr.Args {
				path(selectFunctionFieldArg(arg))
			}
			return
		}
		if stripped, ok := stripQualifiedCurrentObjectField(org, definition, field); ok {
			field = stripped
		}
		if !strings.Contains(field, ".") {
			return
		}
		parts := strings.SplitN(field, ".", 2)
		if strings.Contains(parts[1], ".") {
			complete = false
			return
		}
		relations := matchingParentRelations(org.Namespace, definition, parts[0])
		if len(relations) == 0 {
			complete = false
			return
		}
		for _, relation := range relations {
			if relation.Polymorphic || len(relation.ParentObjects) == 0 {
				complete = false
				return
			}
			for _, parent := range relation.ParentObjects {
				if shouldHydrateVirtualSchemaForQuery(parent) {
					want[canonicalVirtualSchemaObjectName(parent)] = true
				}
			}
		}
	}
	var condition func(c *Condition)
	condition = func(c *Condition) {
		if c == nil || !complete {
			return
		}
		if c.Subquery != nil || c.Formula != nil {
			complete = false
			return
		}
		path(c.Field)
		for i := range c.And {
			condition(&c.And[i])
		}
		for i := range c.Or {
			condition(&c.Or[i])
		}
	}
	for _, field := range query.Fields {
		path(field)
	}
	for _, field := range query.GroupBy {
		path(field)
	}
	path(query.OrderBy)
	for _, order := range query.Order {
		path(order.Field)
	}
	for _, list := range [][]Aggregate{query.Aggregates, query.HavingAggregates, query.orderAggregates} {
		for _, aggregate := range list {
			path(aggregate.Field)
		}
	}
	condition(query.Where)
	condition(query.Having)
	if !complete {
		return allVirtualSchemaTables()
	}
	return want
}

// sharedVirtualSchema returns the generation's entry, preparing the six
// definitions (no rows) on the first query of a generation.
func (c *ExecutionCache) sharedVirtualSchema(org storage.OrgState, runtimeStamp, hydrationStamp string) *sharedVirtualSchema {
	c.mu.RLock()
	shared := c.virtualSchema
	c.mu.RUnlock()
	if shared != nil && shared.runtimeStamp == runtimeStamp && shared.hydrationStamp == hydrationStamp {
		return shared
	}
	c.virtualSchemaBuildMu.Lock()
	defer c.virtualSchemaBuildMu.Unlock()
	c.mu.RLock()
	shared = c.virtualSchema
	c.mu.RUnlock()
	if shared != nil && shared.runtimeStamp == runtimeStamp && shared.hydrationStamp == hydrationStamp {
		return shared
	}
	view := org.CloneObjectsShallow(len(virtualSchemaObjectNames))
	hydrateVirtualSchemaTables(&view, nil)
	definitions := make(map[string]storage.ObjectState, len(virtualSchemaObjectNames))
	for _, name := range virtualSchemaObjectNames {
		object := view.Objects[name]
		object.Records, object.Indexes = nil, nil
		definitions[name] = object
	}
	shared = &sharedVirtualSchema{runtimeStamp: runtimeStamp, hydrationStamp: hydrationStamp, definitions: definitions, tables: map[string]storage.ObjectState{}}
	c.mu.Lock()
	c.virtualSchema = shared
	c.mu.Unlock()
	return shared
}

// hydratedVirtualSchemaTables returns the generation's hydrated tables after
// building the wanted ones that are missing. One build runs at a time per
// cache, so concurrent first queries wait for a single hydration instead of
// each building their own. Record and index maps are marked shared so any
// writer clones them first.
func (c *ExecutionCache) hydratedVirtualSchemaTables(org storage.OrgState, shared *sharedVirtualSchema, want map[string]bool) map[string]storage.ObjectState {
	c.mu.RLock()
	tables := shared.tables
	c.mu.RUnlock()
	missing := virtualSchemaMissingTables(tables, want)
	if len(missing) == 0 {
		return tables
	}
	c.virtualSchemaBuildMu.Lock()
	defer c.virtualSchemaBuildMu.Unlock()
	c.mu.RLock()
	tables = shared.tables
	c.mu.RUnlock()
	missing = virtualSchemaMissingTables(tables, want)
	if len(missing) == 0 {
		return tables
	}
	view := org.CloneObjectsShallow(len(virtualSchemaObjectNames))
	hydrateVirtualSchemaTables(&view, missing)
	next := make(map[string]storage.ObjectState, len(tables)+len(missing))
	for name, table := range tables {
		next[name] = table
	}
	for name := range missing {
		object := view.Objects[name]
		object.RecordsShared, object.IndexesShared = true, true
		next[name] = object
		virtualSchemaBuilds.Add(1)
	}
	c.mu.Lock()
	shared.tables = next
	c.mu.Unlock()
	return next
}

func virtualSchemaMissingTables(tables map[string]storage.ObjectState, want map[string]bool) map[string]bool {
	var missing map[string]bool
	for name := range want {
		if _, ok := tables[name]; !ok {
			if missing == nil {
				missing = map[string]bool{}
			}
			missing[name] = true
		}
	}
	return missing
}

// hydrateVirtualSchemaObjects hydrates every virtual schema table on org,
// once per hydration stamp.
func hydrateVirtualSchemaObjects(org *storage.OrgState) {
	if org == nil {
		return
	}
	stamp := virtualSchemaHydrationStamp(*org)
	if virtualSchemaAlreadyHydrated(*org, stamp) {
		return
	}
	hydrateVirtualSchemaTables(org, allVirtualSchemaTables())
	setVirtualSchemaHydrationStamp(org, stamp)
}

// hydrateVirtualSchemaTables prepares the six virtual schema objects on org
// and fills the rows of the wanted tables from org's ordinary definitions.
// Rows a table already holds are kept (explicit records win).
func hydrateVirtualSchemaTables(org *storage.OrgState, want map[string]bool) {
	if org == nil {
		return
	}
	for _, objectName := range virtualSchemaObjectNames {
		storage.EnsureStandardObject(org, objectName)
		storage.EnsureMutableObjectRecords(org, objectName)
	}
	if len(want) == 0 {
		return
	}
	entityDefinitions := org.Objects["EntityDefinition"]
	entityParticles := org.Objects["EntityParticle"]
	fieldDefinitions := org.Objects["FieldDefinition"]
	relationshipDomains := org.Objects["RelationshipDomain"]
	userEntityAccess := org.Objects["UserEntityAccess"]
	userFieldAccess := org.Objects["UserFieldAccess"]
	if entityDefinitions.Records == nil {
		entityDefinitions.Records = make(map[storage.ID]storage.Record)
	}
	if entityParticles.Records == nil {
		entityParticles.Records = make(map[storage.ID]storage.Record)
	}
	if fieldDefinitions.Records == nil {
		fieldDefinitions.Records = make(map[storage.ID]storage.Record)
	}
	if relationshipDomains.Records == nil {
		relationshipDomains.Records = make(map[storage.ID]storage.Record)
	}
	if userEntityAccess.Records == nil {
		userEntityAccess.Records = make(map[storage.ID]storage.Record)
	}
	if userFieldAccess.Records == nil {
		userFieldAccess.Records = make(map[storage.ID]storage.Record)
	}
	wantEntity, wantParticle, wantField := want["EntityDefinition"], want["EntityParticle"], want["FieldDefinition"]
	wantDomain, wantEntityAccess, wantFieldAccess := want["RelationshipDomain"], want["UserEntityAccess"], want["UserFieldAccess"]
	// EntityDefinition describes itself even though the virtual schema objects
	// do not participate in ordinary field and access-record hydration.
	selfID := storage.ID(entityDefinitions.Definition.APIName)
	if _, exists := entityDefinitions.Records[selfID]; wantEntity && !exists {
		entityDefinitions.Records[selfID] = virtualEntityDefinitionRecord(entityDefinitions.Definition)
	}
	for objectName, object := range org.Objects {
		if isVirtualSchemaObjectName(objectName) {
			continue
		}
		if object.Definition.APIName == "" {
			continue
		}
		entityID := storage.ID(object.Definition.APIName)
		if _, exists := entityDefinitions.Records[entityID]; wantEntity && !exists {
			entityDefinitions.Records[entityID] = virtualEntityDefinitionRecord(object.Definition)
		}
		if _, exists := userEntityAccess.Records[entityID]; wantEntityAccess && !exists {
			userEntityAccess.Records[entityID] = virtualUserEntityAccessRecord(object.Definition.APIName)
		}
		if wantParticle || wantField || wantFieldAccess {
			for fieldName, field := range object.Definition.Fields {
				if field.APIName == "" {
					field.APIName = fieldName
				}
				particleID := storage.ID(object.Definition.APIName + "." + field.APIName)
				if _, exists := entityParticles.Records[particleID]; wantParticle && !exists {
					entityParticles.Records[particleID] = virtualEntityParticleRecord(object.Definition, field)
				}
				if _, exists := fieldDefinitions.Records[particleID]; wantField && !exists {
					fieldDefinitions.Records[particleID] = virtualFieldDefinitionRecord(object.Definition, field)
				}
				if _, exists := userFieldAccess.Records[particleID]; wantFieldAccess && !exists {
					userFieldAccess.Records[particleID] = virtualUserFieldAccessRecord(particleID, object.Definition.APIName)
				}
			}
		}
		if wantDomain {
			for _, relation := range object.Definition.Relations {
				if relation.ChildRelationship == "" || relation.Field == "" {
					continue
				}
				for _, parentName := range relation.ParentObjects {
					if parentName == "" {
						continue
					}
					domainID := storage.ID(parentName + "." + object.Definition.APIName + "." + relation.Field)
					if _, exists := relationshipDomains.Records[domainID]; !exists {
						relationshipDomains.Records[domainID] = virtualRelationshipDomainRecord(domainID, parentName, object.Definition.APIName, relation)
					}
				}
			}
		}
	}
	org.Objects["EntityDefinition"] = entityDefinitions
	org.Objects["EntityParticle"] = entityParticles
	org.Objects["FieldDefinition"] = fieldDefinitions
	org.Objects["RelationshipDomain"] = relationshipDomains
	org.Objects["UserEntityAccess"] = userEntityAccess
	org.Objects["UserFieldAccess"] = userFieldAccess
}

func virtualSchemaHydrationStamp(org storage.OrgState) string {
	objectCount := 0
	fieldCount := 0
	for objectName, object := range org.Objects {
		if isVirtualSchemaObjectName(objectName) || isUserAccessVirtualSchemaObjectName(objectName) {
			continue
		}
		if object.Definition.APIName == "" {
			continue
		}
		objectCount++
		fieldCount += len(object.Definition.Fields)
	}
	return strconv.Itoa(objectCount) + ":" + strconv.Itoa(fieldCount)
}

func virtualSchemaAlreadyHydrated(org storage.OrgState, stamp string) bool {
	entityDefinitions, ok := org.Objects["EntityDefinition"]
	if !ok {
		return false
	}
	if entityDefinitions.Definition.Metadata == nil {
		return false
	}
	return entityDefinitions.Definition.Metadata[virtualSchemaHydrationStampKey] == stamp
}

func setVirtualSchemaHydrationStamp(org *storage.OrgState, stamp string) {
	if org == nil {
		return
	}
	entityDefinitions, ok := org.Objects["EntityDefinition"]
	if !ok {
		return
	}
	storage.EnsureMutableObjectDefinition(org, "EntityDefinition")
	entityDefinitions = org.Objects["EntityDefinition"]
	if entityDefinitions.Definition.Metadata == nil {
		entityDefinitions.Definition.Metadata = make(map[string]string)
	}
	entityDefinitions.Definition.Metadata[virtualSchemaHydrationStampKey] = stamp
	org.Objects["EntityDefinition"] = entityDefinitions
}

func isVirtualSchemaObjectName(objectName string) bool {
	return strings.EqualFold(objectName, "EntityDefinition") ||
		strings.EqualFold(objectName, "EntityParticle") ||
		strings.EqualFold(objectName, "FieldDefinition") ||
		strings.EqualFold(objectName, "RelationshipDomain")
}

func isUserAccessVirtualSchemaObjectName(objectName string) bool {
	return strings.EqualFold(objectName, "UserEntityAccess") ||
		strings.EqualFold(objectName, "UserFieldAccess")
}

func virtualEntityDefinitionRecord(definition storage.ObjectDefinition) storage.Record {
	label := definition.Label
	if label == "" {
		label = definition.APIName
	}
	record := storage.Record{ID: storage.ID(definition.APIName), Object: "EntityDefinition", Fields: map[string]storage.Value{
		"DurableId":                 storage.StringValue(definition.APIName),
		"QualifiedApiName":          storage.StringValue(definition.APIName),
		"DeveloperName":             storage.StringValue(strings.TrimSuffix(definition.APIName, "__c")),
		"Label":                     storage.StringValue(label),
		"MasterLabel":               storage.StringValue(label),
		"NamespacePrefix":           storage.NullValue(),
		"IsCustomSetting":           storage.BooleanValue(false),
		"IsLayoutable":              storage.BooleanValue(true),
		"IsQueryable":               storage.BooleanValue(true),
		"IsTriggerable":             storage.BooleanValue(true),
		"IsDeprecatedAndHidden":     storage.BooleanValue(false),
		"RunningUserEntityAccessId": storage.StringValue(definition.APIName),
	}}
	for key, field := range map[string]string{"customizable": "IsCustomizable", "triggerable": "IsTriggerable"} {
		if value, known := virtualEntityDefinitionCapability(definition, key); known {
			record.Fields[field] = storage.BooleanValue(value)
		}
	}
	// Org capture setup_entity_queries: custom objects, custom settings and
	// custom metadata types are customizable, settings and metadata types drop
	// their suffix from DeveloperName, and settings are flagged IsCustomSetting.
	if storage.IsCustomSettingDefinition(definition) {
		record.Fields["IsCustomSetting"] = storage.BooleanValue(true)
	}
	if storage.IsCustomMetadataDefinition(definition) {
		record.Fields["DeveloperName"] = storage.StringValue(strings.TrimSuffix(definition.APIName, "__mdt"))
	}
	// IsApexTriggerable follows IsTriggerable except for custom settings and
	// custom metadata types, which cannot carry triggers.
	apexTriggerable := record.Fields["IsTriggerable"]
	if storage.IsCustomSettingDefinition(definition) || storage.IsCustomMetadataDefinition(definition) {
		apexTriggerable = storage.BooleanValue(false)
	}
	record.Fields["IsApexTriggerable"] = apexTriggerable
	if _, known := record.Fields["IsCustomizable"]; !known && (strings.HasSuffix(strings.ToLower(definition.APIName), "__c") || storage.IsCustomMetadataDefinition(definition)) {
		record.Fields["IsCustomizable"] = storage.BooleanValue(true)
	}
	return record
}

// Org capture setup_entity_queries: an EntityDefinition.QualifiedApiName
// greater than a null bind matches every entity, which is how paging loops that
// start from an unset cursor see the first page.
func entityDefinitionNullLowerBound(definition storage.ObjectDefinition, field string, bound storage.Value) bool {
	return bound.Kind == storage.ValueNull && strings.EqualFold(definition.APIName, "EntityDefinition") && strings.EqualFold(field, "QualifiedApiName")
}

func virtualEntityDefinitionCapability(definition storage.ObjectDefinition, key string) (bool, bool) {
	if raw, explicit := definition.Metadata[key]; explicit {
		if value, err := strconv.ParseBool(raw); err == nil {
			return value, true
		}
	}
	if key == "triggerable" {
		if value, known := storage.StandardObjectTriggerable(definition.APIName); known {
			return value, true
		}
	}
	if raw, known := storage.StandardObjectMetadata(definition.APIName, key); known {
		value, err := strconv.ParseBool(raw)
		return value, err == nil
	}
	return false, false
}

func virtualEntityParticleRecord(definition storage.ObjectDefinition, field storage.Field) storage.Record {
	label := field.Label
	if label == "" {
		label = field.APIName
	}
	dataType := field.DisplayType
	if dataType == "" {
		dataType = storageFieldDisplayType(field.Type)
	}
	length := int64(field.Length)
	if length == 0 {
		length = 255
	}
	fieldID := definition.APIName + "." + field.APIName
	return storage.Record{ID: storage.ID(fieldID), Object: "EntityParticle", Fields: map[string]storage.Value{
		"DurableId":          storage.StringValue(fieldID),
		"QualifiedApiName":   storage.StringValue(field.APIName),
		"DeveloperName":      storage.StringValue(strings.TrimSuffix(field.APIName, "__c")),
		"Label":              storage.StringValue(label),
		"MasterLabel":        storage.StringValue(label),
		"Name":               storage.StringValue(field.APIName),
		"DataType":           storage.StringValue(dataType),
		"IsCalculated":       storage.BooleanValue(field.Type == storage.FieldCalculated || strings.TrimSpace(field.Formula) != ""),
		"Length":             storage.IntegerValue(length),
		"EntityDefinitionId": storage.StringValue(definition.APIName),
		"FieldDefinitionId":  storage.StringValue(fieldID),
	}}
}

func virtualFieldDefinitionRecord(definition storage.ObjectDefinition, field storage.Field) storage.Record {
	label := field.Label
	if label == "" {
		label = field.APIName
	}
	dataType := field.DisplayType
	if dataType == "" {
		dataType = storageFieldDisplayType(field.Type)
	}
	length := int64(field.Length)
	if length == 0 {
		length = 255
	}
	fieldID := definition.APIName + "." + field.APIName
	record := storage.Record{ID: storage.ID(fieldID), Object: "FieldDefinition", Fields: map[string]storage.Value{
		"DurableId":                storage.StringValue(fieldID),
		"QualifiedApiName":         storage.StringValue(field.APIName),
		"DeveloperName":            storage.StringValue(strings.TrimSuffix(field.APIName, "__c")),
		"Label":                    storage.StringValue(label),
		"MasterLabel":              storage.StringValue(label),
		"DataType":                 storage.StringValue(dataType),
		"IsCalculated":             storage.BooleanValue(field.Type == storage.FieldCalculated || strings.TrimSpace(field.Formula) != ""),
		"Length":                   storage.IntegerValue(length),
		"EntityDefinitionId":       storage.StringValue(definition.APIName),
		"RunningUserFieldAccessId": storage.StringValue(fieldID),
	}}
	if nameField, known := storage.StandardFieldNameFlag(definition, field.APIName); known {
		record.Fields["IsNameField"] = storage.BooleanValue(nameField)
	}
	return record
}

func virtualRelationshipDomainRecord(id storage.ID, parentName, childName string, relation storage.Relationship) storage.Record {
	return storage.Record{ID: id, Object: "RelationshipDomain", Fields: map[string]storage.Value{
		"DurableId":             storage.StringValue(string(id)),
		"ParentSobjectId":       storage.StringValue(parentName),
		"ChildSobjectId":        storage.StringValue(childName),
		"FieldId":               storage.StringValue(childName + "." + relation.Field),
		"RelationshipName":      storage.StringValue(relation.ChildRelationship),
		"IsCascadeDelete":       storage.BooleanValue(false),
		"IsRestrictedDelete":    storage.BooleanValue(false),
		"IsDeprecatedAndHidden": storage.BooleanValue(false),
		"JunctionIdListNames":   storage.NullValue(),
	}}
}

func virtualUserEntityAccessRecord(objectName string) storage.Record {
	return storage.Record{ID: storage.ID(objectName), Object: "UserEntityAccess", Fields: map[string]storage.Value{
		"EntityDefinitionId": storage.StringValue(objectName),
		"IsCreatable":        storage.BooleanValue(true),
		"IsDeletable":        storage.BooleanValue(true),
		"IsUpdatable":        storage.BooleanValue(true),
		"IsReadable":         storage.BooleanValue(true),
	}}
}

func virtualUserFieldAccessRecord(id storage.ID, objectName string) storage.Record {
	return storage.Record{ID: id, Object: "UserFieldAccess", Fields: map[string]storage.Value{
		"EntityDefinitionId": storage.StringValue(objectName),
		"FieldDefinitionId":  storage.StringValue(string(id)),
		"IsAccessible":       storage.BooleanValue(true),
		"IsCreatable":        storage.BooleanValue(true),
		"IsUpdatable":        storage.BooleanValue(true),
	}}
}

func storageFieldDisplayType(fieldType storage.FieldType) string {
	switch fieldType {
	case storage.FieldBoolean:
		return "BOOLEAN"
	case storage.FieldInteger:
		return "INTEGER"
	case storage.FieldDecimal, storage.FieldCalculated, storage.FieldSummary:
		return "DOUBLE"
	case storage.FieldDate:
		return "DATE"
	case storage.FieldDateTime:
		return "DATETIME"
	case storage.FieldReference, storage.FieldID:
		return "REFERENCE"
	case storage.FieldPicklist:
		return "PICKLIST"
	case storage.FieldMultiPicklist:
		return "MULTIPICKLIST"
	default:
		return "STRING"
	}
}

func candidateRecordIDs(object storage.ObjectState, where *Condition, allRows bool) []string {
	if ids, ok := indexedCandidateIDs(object, where, allRows); ok {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			out = append(out, string(id))
		}
		return out
	}
	out := make([]string, 0, len(object.Records))
	for id := range object.Records {
		out = append(out, string(id))
	}
	return out
}

func indexedCandidateIDs(object storage.ObjectState, where *Condition, allRows bool) ([]storage.ID, bool) {
	if allRows || where == nil || where.Not {
		return nil, false
	}
	if len(where.Or) != 0 {
		return unionIndexedCandidateIDs(object, where.Or, allRows)
	}
	if len(where.And) != 0 {
		var best []storage.ID
		for i := range where.And {
			ids, ok := indexedCandidateIDs(object, &where.And[i], allRows)
			if !ok {
				continue
			}
			if best == nil || len(ids) < len(best) {
				best = ids
			}
		}
		if best != nil {
			return best, true
		}
		return nil, false
	}
	if where.Range || where.Subquery != nil || where.Op != "=" {
		if where.Range || where.Subquery != nil || where.Op != "IN" {
			return nil, false
		}
		if strings.Contains(where.Field, ".") {
			return nil, false
		}
		return unionLookupIndexValues(object, where.Field, where.Values)
	}
	if strings.Contains(where.Field, ".") {
		return nil, false
	}
	return storage.LookupIndex(object, where.Field, where.Value)
}

func unionIndexedCandidateIDs(object storage.ObjectState, conditions []Condition, allRows bool) ([]storage.ID, bool) {
	if len(conditions) == 0 {
		return nil, false
	}
	seen := make(map[storage.ID]struct{})
	out := make([]storage.ID, 0)
	for i := range conditions {
		ids, ok := indexedCandidateIDs(object, &conditions[i], allRows)
		if !ok {
			return nil, false
		}
		for _, id := range ids {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out, true
}

func unionLookupIndexValues(object storage.ObjectState, field string, values []storage.Value) ([]storage.ID, bool) {
	seen := make(map[storage.ID]struct{})
	out := make([]storage.ID, 0)
	for _, value := range values {
		ids, ok := storage.LookupIndex(object, field, value)
		if !ok {
			return nil, false
		}
		for _, id := range ids {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out, true
}

func applyWindow[T any](records []T, offset, limit int, hasLimit bool) []T {
	if offset > 0 {
		if offset >= len(records) {
			return nil
		}
		records = records[offset:]
	}
	if hasLimit && limit <= 0 {
		return nil
	}
	if hasLimit && limit < len(records) {
		return records[:limit]
	}
	return records
}

func aggregateRecords(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, query Query, userIDs ...storage.ID) ([]storage.Record, error) {
	if len(query.GroupBy) == 0 {
		fields, err := aggregateFields(org, definition, records, query.Aggregates, nil, userIDs...)
		if err != nil {
			return nil, err
		}
		if err := addHiddenAggregateFields(fields, org, definition, records, query.HavingAggregates, nil, userIDs...); err != nil {
			return nil, err
		}
		if err := addAggregateToLabelFields(fields, org, definition, records, query, userIDs...); err != nil {
			return nil, err
		}
		record := storage.Record{Object: "AggregateResult", Fields: fields}
		if query.Having != nil && !matchesWithContext(org, storage.ObjectDefinition{}, record, query.Having, ExecutionContext{Locale: query.textLocale}) {
			return nil, nil
		}
		removeHiddenAggregateFields(record.Fields, query.HavingAggregates)
		return []storage.Record{record}, nil
	}

	type group struct {
		key      string
		values   map[string]storage.Value
		grouping map[string]bool
		records  []storage.Record
	}
	groups := map[string]*group{}
	var order []string
	sets := groupingSets(query.GroupBy, query.GroupMode)
	scratch := make([]storage.Value, len(query.GroupBy))
	// The declared kind of each grouped field is fixed for the scan; resolve it
	// once instead of copying its definition per record.
	kinds := make([]groupFieldKind, len(query.GroupBy))
	for i, field := range query.GroupBy {
		kinds[i] = declaredGroupFieldKind(org, definition, field)
	}
	for _, record := range records {
		for setIndex, active := range sets {
			parts := []string{strconv.Itoa(setIndex)}
			for i, field := range query.GroupBy {
				if !active[field] {
					scratch[i] = storage.NullValue()
					parts = append(parts, "1:")
					continue
				}
				var value storage.Value
				var ok bool
				if expr, function := parseSelectFieldExpression(field); function && query.TimeZoneID != "" {
					value, ok = convertedSelectFieldExpressionValue(org, definition, record, expr, query.TimeZoneID, userIDs...)
				} else {
					value, ok = recordValue(org, definition, record, field, userIDs...)
				}
				if !ok {
					value = storage.NullValue()
				}
				// R127/N019: grouped dates retain their field type, including
				// records loaded from fixtures that store the date as text.
				if value.Kind == storage.ValueString && kinds[i].date {
					value = storage.DateValue(value.String)
				}
				scratch[i] = value
				if query.textCollation && value.Kind == storage.ValueString {
					parts = append(parts, "0:"+string(storage.ValueString)+":"+textGroupKey(value.String))
				} else {
					parts = append(parts, "0:"+groupValueKey(value, kinds[i]))
				}
			}
			key := strings.Join(parts, "\x00")
			current, ok := groups[key]
			if !ok {
				values := make(map[string]storage.Value, len(query.GroupBy))
				grouping := make(map[string]bool, len(query.GroupBy))
				for i, field := range query.GroupBy {
					values[field] = scratch[i]
					if !active[field] {
						grouping[field] = true
					}
				}
				current = &group{key: key, values: values, grouping: grouping}
				groups[key] = current
				order = append(order, key)
			}
			if query.textCollation {
				// The captured A/a and Z/z groups report the binary-smallest
				// spelling. Using that representative for other merged spellings
				// is provisional; these rows do not prove a universal rule.
				for i, field := range query.GroupBy {
					if held := current.values[field]; active[field] && scratch[i].Kind == storage.ValueString && held.Kind == storage.ValueString && scratch[i].String < held.String {
						current.values[field] = scratch[i]
					}
				}
			}
			current.records = append(current.records, record)
		}
	}
	sort.Strings(order)
	out := make([]storage.Record, 0, len(order))
	for _, key := range order {
		group := groups[key]
		fields := make(map[string]storage.Value, len(group.values)+len(query.Aggregates))
		for _, field := range query.GroupBy {
			value := group.values[field].Clone()
			fields[field] = value
			if alias := aggregateGroupFieldImplicitAlias(field); alias != "" {
				if _, exists := fields[alias]; !exists {
					fields[alias] = value.Clone()
				}
			}
		}
		if err := addAggregateToLabelFields(fields, org, definition, group.records, query, userIDs...); err != nil {
			return nil, err
		}
		for _, field := range query.Fields {
			if _, _, toLabel := aggregateToLabelField(field); toLabel {
				continue
			}
			if expr, ok := parseSelectFieldExpression(field); ok && expr.Alias != "" {
				if value, ok := fields[expr.Raw]; ok {
					fields[expr.Alias] = value.Clone()
				} else if len(group.records) > 0 {
					// Native Q8 rollover: a date function over a grouped Date
					// field is exposed under its alias for each group.
					if value, ok := convertedSelectFieldExpressionValue(org, definition, group.records[0], expr, query.TimeZoneID, userIDs...); ok {
						fields[expr.Alias] = value
					}
				}
				continue
			}
			if raw, alias, ok := splitSelectFieldAlias(field); ok {
				if value, ok := fields[raw]; ok {
					fields[alias] = value.Clone()
					continue
				}
				if value, ok := findAggregateFieldByComparableName(fields, raw); ok {
					fields[alias] = value.Clone()
				}
			}
		}
		aggregateFields, err := aggregateFields(org, definition, group.records, query.Aggregates, group.grouping, userIDs...)
		if err != nil {
			return nil, err
		}
		for field, value := range aggregateFields {
			fields[field] = value
		}
		if err := addHiddenAggregateFields(fields, org, definition, group.records, query.HavingAggregates, group.grouping, userIDs...); err != nil {
			return nil, err
		}
		if err := addHiddenAggregateFields(fields, org, definition, group.records, query.orderAggregates, group.grouping, userIDs...); err != nil {
			return nil, err
		}
		record := storage.Record{Object: "AggregateResult", Fields: fields}
		if query.Having != nil && !matchesWithContext(org, storage.ObjectDefinition{}, record, query.Having, ExecutionContext{Locale: query.textLocale}) {
			continue
		}
		removeHiddenAggregateFields(record.Fields, query.HavingAggregates)
		out = append(out, record)
	}
	return out, nil
}

// addAggregateToLabelFields projects toLabel(field) select items of an
// aggregate query (flow_tolabel_residuals, API 62.0 and 67.0). The value is the
// label of the group's MIN non-null value: the group value when the field is
// grouped. An aliased item is exposed under its alias; an unaliased item
// replaces its grouped field's value under the grouped field's key.
func addAggregateToLabelFields(fields map[string]storage.Value, org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, query Query, userIDs ...storage.ID) error {
	for _, field := range query.Fields {
		arg, alias, ok := aggregateToLabelField(field)
		if !ok {
			continue
		}
		key := alias
		if key == "" {
			for _, group := range query.GroupBy {
				if containsName([]string{group}, arg) {
					key = group
					break
				}
			}
		}
		if key == "" {
			continue
		}
		value, err := aggregateValue(org, definition, records, Aggregate{Func: "MIN", Field: arg}, userIDs...)
		if err != nil {
			return err
		}
		if value.Kind != storage.ValueNull {
			value = toLabelValue(org, definition, arg, value)
		}
		fields[key] = value
	}
	return nil
}

// groupFieldKind is what the grouping loop needs to know about a grouped
// field's single declaration: whether it is a Date (text values group as
// dates) and whether it is an Id or reference (18-character Ids group with
// their 15-character form).
type groupFieldKind struct {
	date bool
	id   bool
}

func declaredGroupFieldKind(org storage.OrgState, definition storage.ObjectDefinition, field string) groupFieldKind {
	fields, known := fieldDefinitionsForReference(org, definition, field)
	if !known || len(fields) != 1 {
		return groupFieldKind{}
	}
	return groupFieldKind{
		date: fields[0].Type == storage.FieldDate,
		id:   fields[0].Type == storage.FieldID || fields[0].Type == storage.FieldReference,
	}
}

// Native NPG API62/67/68: a lookup retains one group across equivalent
// 15/18-character Id representations. Keep the projected representative intact.
func aggregateGroupValueKey(org storage.OrgState, definition storage.ObjectDefinition, field string, value storage.Value) string {
	return groupValueKey(value, declaredGroupFieldKind(org, definition, field))
}

func groupValueKey(value storage.Value, kind groupFieldKind) string {
	if value.Kind == storage.ValueID && len(value.ID) == 18 && kind.id {
		value.ID = value.ID[:15]
	}
	return valueKey(value)
}

func groupingSets(fields []string, mode string) []map[string]bool {
	switch strings.ToUpper(mode) {
	case "ROLLUP":
		sets := make([]map[string]bool, 0, len(fields)+1)
		for activeCount := len(fields); activeCount >= 0; activeCount-- {
			active := make(map[string]bool, activeCount)
			for i := 0; i < activeCount; i++ {
				active[fields[i]] = true
			}
			sets = append(sets, active)
		}
		return sets
	case "CUBE":
		if len(fields) == 0 {
			return []map[string]bool{{}}
		}
		total := 1 << len(fields)
		sets := make([]map[string]bool, 0, total)
		for mask := total - 1; mask >= 0; mask-- {
			active := make(map[string]bool, len(fields))
			for i, field := range fields {
				if mask&(1<<i) != 0 {
					active[field] = true
				}
			}
			sets = append(sets, active)
		}
		return sets
	default:
		active := make(map[string]bool, len(fields))
		for _, field := range fields {
			active[field] = true
		}
		return []map[string]bool{active}
	}
}

func aggregateGroupFieldImplicitAlias(field string) string {
	field = strings.TrimSpace(field)
	if field == "" || strings.ContainsAny(field, "()") || !strings.Contains(field, ".") {
		return ""
	}
	parts := strings.Split(field, ".")
	return strings.TrimSpace(parts[len(parts)-1])
}

func aggregateRecordValue(record storage.Record, field string) storage.Value {
	if value, ok := record.GetField(field); ok {
		return value
	}
	return storage.NullValue()
}

func aggregateFields(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, aggregates []Aggregate, grouping map[string]bool, userIDs ...storage.ID) (map[string]storage.Value, error) {
	fields := make(map[string]storage.Value, len(aggregates)*2)
	for i, aggregate := range aggregates {
		value, err := aggregateResultValue(org, definition, records, aggregate, grouping, userIDs...)
		if err != nil {
			return nil, err
		}
		fields[fmt.Sprintf("expr%d", i)] = value
		if aggregate.Alias != "" {
			fields[aggregate.Alias] = value.Clone()
		}
	}
	return fields, nil
}

func addHiddenAggregateFields(fields map[string]storage.Value, org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, aggregates []Aggregate, grouping map[string]bool, userIDs ...storage.ID) error {
	for _, aggregate := range aggregates {
		if aggregate.Alias == "" {
			continue
		}
		value, err := aggregateResultValue(org, definition, records, aggregate, grouping, userIDs...)
		if err != nil {
			return err
		}
		fields[aggregate.Alias] = value
	}
	return nil
}

func removeHiddenAggregateFields(fields map[string]storage.Value, aggregates []Aggregate) {
	for _, aggregate := range aggregates {
		if aggregate.Alias != "" {
			delete(fields, aggregate.Alias)
		}
	}
}

func aggregateResultValue(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, aggregate Aggregate, grouping map[string]bool, userIDs ...storage.ID) (storage.Value, error) {
	if aggregate.Func == "GROUPING" {
		if grouping[aggregate.Field] {
			return storage.IntegerValue(1), nil
		}
		return storage.IntegerValue(0), nil
	}
	return aggregateValue(org, definition, records, aggregate, userIDs...)
}

func aggregateValue(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, aggregate Aggregate, userIDs ...storage.ID) (storage.Value, error) {
	switch aggregate.Func {
	case "COUNT":
		if aggregate.Field == "" {
			return storage.IntegerValue(int64(len(records))), nil
		}
		var count int64
		for _, record := range records {
			value, ok := recordValue(org, definition, record, aggregate.Field, userIDs...)
			if ok && value.Kind != storage.ValueNull {
				count++
			}
		}
		return storage.IntegerValue(count), nil
	case "COUNT_DISTINCT":
		seen := map[string]bool{}
		for _, record := range records {
			value, ok := recordValue(org, definition, record, aggregate.Field, userIDs...)
			if ok && value.Kind != storage.ValueNull {
				seen[valueKey(value)] = true
			}
		}
		return storage.IntegerValue(int64(len(seen))), nil
	case "SUM", "AVG":
		sum := new(big.Rat)
		var count int64
		for _, record := range records {
			value, ok := recordValue(org, definition, record, aggregate.Field, userIDs...)
			if !ok || value.Kind == storage.ValueNull {
				continue
			}
			number, ok := numericValue(value)
			if !ok {
				return storage.Value{}, fmt.Errorf("soql: %s requires numeric field %s", aggregate.Func, aggregate.Field)
			}
			sum.Add(sum, number)
			count++
		}
		if count == 0 {
			return storage.NullValue(), nil
		}
		if aggregate.Func == "AVG" {
			sum.Quo(sum, new(big.Rat).SetInt64(count))
		}
		// R072/R075/R079/R109/R135/R142: aggregate SUM/AVG return a
		// Decimal carrying the query engine's double precision and minimum scale.
		number, _ := sum.Float64()
		text := strconv.FormatFloat(number, 'f', -1, 64)
		if !strings.Contains(text, ".") {
			text += ".0"
		}
		return storage.DecimalValue(text), nil
	case "MIN", "MAX":
		var best storage.Value
		found := false
		// Native Q13 extrema: MIN and MAX over a picklist follow setup order,
		// like ORDER BY. A value outside the configured list keeps text order.
		var ranks map[string]int
		if fields, ok := fieldDefinitionsForReference(org, definition, aggregate.Field); ok && len(fields) == 1 && fields[0].Type == storage.FieldPicklist && len(fields[0].PicklistValues) > 0 {
			ranks = make(map[string]int, len(fields[0].PicklistValues))
			for position, option := range fields[0].PicklistValues {
				ranks[option.Value] = position
			}
		}
		if ranks != nil {
			for _, record := range records {
				value, ok := recordValue(org, definition, record, aggregate.Field, userIDs...)
				if !ok || value.Kind == storage.ValueNull {
					continue
				}
				if _, known := ranks[value.String]; value.Kind != storage.ValueString || !known {
					ranks = nil
					break
				}
			}
		}
		for _, record := range records {
			value, ok := recordValue(org, definition, record, aggregate.Field, userIDs...)
			if !ok || value.Kind == storage.ValueNull {
				continue
			}
			if !found {
				best = value.Clone()
				found = true
				continue
			}
			var cmp int
			if ranks != nil {
				cmp = ranks[value.String] - ranks[best.String]
			} else {
				cmp = compareValuesCollated(value, best, aggregate.textCollation)
			}
			if aggregate.Func == "MIN" && cmp < 0 {
				best = value.Clone()
			}
			if aggregate.Func == "MAX" && cmp > 0 {
				best = value.Clone()
			}
		}
		if !found {
			return storage.NullValue(), nil
		}
		return best, nil
	default:
		return storage.Value{}, fmt.Errorf("soql: unsupported aggregate %s", aggregate.Func)
	}
}

func ParseAndExecute(org storage.OrgState, input string) (Result, error) {
	return ParseAndExecuteAt(org, input, time.Now().UTC())
}

func ParseAndExecuteAt(org storage.OrgState, input string, now time.Time) (Result, error) {
	query, err := ParseAtWithFiscalYearStartMonth(input, now, FiscalYearStartMonth(org))
	if err != nil {
		return Result{}, err
	}
	return Execute(org, query)
}

func FiscalYearStartMonth(org storage.OrgState) int {
	month, _ := fiscalYearSettings(org)
	return month
}

func usesStartDateAsFiscalYearName(org storage.OrgState) bool {
	_, useStart := fiscalYearSettings(org)
	return useStart
}

func fiscalYearSettings(org storage.OrgState) (int, bool) {
	if org.OrgID != "" {
		if month, useStart, ok := fiscalYearSettingsFromRecord(org, storage.ID(org.OrgID)); ok {
			return month, useStart
		}
	}
	for _, record := range org.Objects["Organization"].Records {
		if month, useStart, ok := fiscalYearSettingsFromFields(record.Fields); ok {
			return month, useStart
		}
	}
	return 1, false
}

func fiscalYearStartMonthFromRecord(org storage.OrgState, id storage.ID) (int, bool) {
	month, _, ok := fiscalYearSettingsFromRecord(org, id)
	return month, ok
}

func usesStartDateAsFiscalYearNameFromRecord(org storage.OrgState, id storage.ID) (bool, bool) {
	_, useStart, ok := fiscalYearSettingsFromRecord(org, id)
	return useStart, ok
}

func fiscalYearSettingsFromRecord(org storage.OrgState, id storage.ID) (int, bool, bool) {
	object, ok := org.Objects["Organization"]
	if !ok {
		return 0, false, false
	}
	record, ok := object.Records[id]
	if !ok {
		return 0, false, false
	}
	return fiscalYearSettingsFromFields(record.Fields)
}

func fiscalYearSettingsFromFields(fields map[string]storage.Value) (int, bool, bool) {
	month, ok := fiscalYearStartMonthFromValue(fields["FiscalYearStartMonth"])
	if !ok {
		return 0, false, false
	}
	useStart, _ := usesStartDateAsFiscalYearNameFromValue(fields["UsesStartDateAsFiscalYearName"])
	return month, useStart, true
}

func fiscalYearStartMonthFromValue(value storage.Value) (int, bool) {
	if value.Kind != storage.ValueInteger {
		return 0, false
	}
	month := int(value.Integer)
	if month < 1 || month > 12 {
		return 0, false
	}
	return month, true
}

func usesStartDateAsFiscalYearNameFromValue(value storage.Value) (bool, bool) {
	if value.Kind != storage.ValueBoolean {
		return false, false
	}
	return value.Boolean, true
}

func matches(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, condition *Condition) bool {
	return matchesWithContext(org, definition, record, condition, ExecutionContext{})
}

type literalBoundsKey struct {
	start, end, zone string
	utc              *time.Location
}
type literalBounds struct {
	start, end time.Time
	ok         bool
}
type predicateScratch struct {
	bounds    map[literalBoundsKey]literalBounds
	patterns  map[string]*compiledLikePattern
	textKinds map[textEqualityKindKey]textEqualityKind
}

// textEqualityKind records how equality on one declared field treats text
// operands: whether every declared field is text, and whether every declared
// text field compares exactly (unique and case-sensitive).
type textEqualityKind struct {
	text  bool
	exact bool
}

type textEqualityKindKey struct {
	object string
	field  string
}

// declaredTextEqualityKind classifies a field for text equality. It is the
// declaration walk equalFieldValuesInOrg performs; the predicate scratch
// remembers the answer so a WHERE clause resolves each field once per query
// instead of once per candidate record.
func declaredTextEqualityKind(org storage.OrgState, definition storage.ObjectDefinition, field string) textEqualityKind {
	fields, known := fieldDefinitionsForReference(org, definition, field)
	text := known && len(fields) > 0
	exact := text
	for _, declared := range fields {
		if declared.Type != storage.FieldString {
			text = false
			break
		}
		exact = exact && declared.Unique && declared.CaseSensitive
	}
	return textEqualityKind{text: text, exact: text && exact}
}

func (s *predicateScratch) textEqualityKind(org storage.OrgState, definition storage.ObjectDefinition, field string) textEqualityKind {
	if s == nil {
		return declaredTextEqualityKind(org, definition, field)
	}
	key := textEqualityKindKey{object: definition.APIName, field: field}
	if kind, ok := s.textKinds[key]; ok {
		return kind
	}
	kind := declaredTextEqualityKind(org, definition, field)
	if s.textKinds == nil {
		s.textKinds = make(map[textEqualityKindKey]textEqualityKind)
	}
	s.textKinds[key] = kind
	return kind
}

func matchesWithContext(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, condition *Condition, context ExecutionContext) bool {
	return matchesWithScratch(org, definition, record, condition, context, nil)
}

func matchesWithScratch(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, condition *Condition, context ExecutionContext, scratch *predicateScratch) bool {
	collate := englishTextLocale(context.Locale)
	if condition == nil {
		return true
	}
	if condition.Not {
		return !matchesWithScratch(org, definition, record, &Condition{
			RewrittenAggregate: condition.RewrittenAggregate,
			And:                condition.And, Or: condition.Or,
			Field: condition.Field, Formula: condition.Formula, Op: condition.Op,
			Value: condition.Value, Value2: condition.Value2, Range: condition.Range,
			DateLiteralTimeZoneID: condition.DateLiteralTimeZoneID,
			Values:                condition.Values, Subquery: condition.Subquery,
		}, context, scratch)
	}
	if len(condition.And) > 0 {
		for _, c := range condition.And {
			if !matchesWithScratch(org, definition, record, &c, context, scratch) {
				return false
			}
		}
		return true
	}
	if len(condition.Or) > 0 {
		for _, c := range condition.Or {
			if matchesWithScratch(org, definition, record, &c, context, scratch) {
				return true
			}
		}
		return false
	}
	var left storage.Value
	var ok bool
	if condition.Formula != nil {
		left, ok = whereFormulaValue(org, definition, record, condition.Formula, context.formulaActor()), true
	} else {
		left, ok = recordValue(org, definition, record, condition.Field, context.formulaActor())
	}
	if !ok {
		left = storage.NullValue()
	}
	if expr, parsed := parseSelectFieldExpression(condition.Field); parsed && expr.Func == "TOLABEL" && len(expr.Args) == 1 {
		left = translatedPredicateLabel(org, definition, expr.Args[0], left, context.Language)
	}
	if condition.Range && condition.DateLiteralTimeZoneID != "" {
		if matched, handled := matchesDateLiteralRangeWithScratch(left, condition, scratch); handled {
			return matched
		}
	}
	// R164 and N001-N014: a null relational operand omits that HAVING
	// aggregate predicate. Equality still tests the aggregate's raw null.
	if record.Object == "AggregateResult" && condition.RewrittenAggregate && !condition.Range && condition.Value.Kind == storage.ValueNull {
		switch condition.Op {
		case ">", ">=", "<", "<=":
			return true
		}
	}
	switch condition.Op {
	case "=":
		if condition.Range {
			return compareValuesCollated(left, condition.Value, collate) >= 0 && compareValuesCollated(left, condition.Value2, collate) < 0
		}
		return equalFieldValuesInOrgWithScratch(org, definition, condition.Field, left, condition.Value, context.Locale, scratch)
	case "!=":
		if condition.Range {
			return compareValuesCollated(left, condition.Value, collate) < 0 || compareValuesCollated(left, condition.Value2, collate) >= 0
		}
		return !equalFieldValuesInOrgWithScratch(org, definition, condition.Field, left, condition.Value, context.Locale, scratch)
	case ">":
		bound := condition.Value
		if condition.Range {
			bound = condition.Value2
		}
		if entityDefinitionNullLowerBound(definition, condition.Field, bound) {
			return left.Kind != storage.ValueNull
		}
		if left.Kind == storage.ValueNull || bound.Kind == storage.ValueNull {
			return false
		}
		return compareValuesCollated(left, bound, collate) > 0
	case ">=":
		if entityDefinitionNullLowerBound(definition, condition.Field, condition.Value) {
			return left.Kind != storage.ValueNull
		}
		if left.Kind == storage.ValueNull || condition.Value.Kind == storage.ValueNull {
			return false
		}
		return compareValuesCollated(left, condition.Value, collate) >= 0
	case "<":
		if left.Kind == storage.ValueNull || condition.Value.Kind == storage.ValueNull {
			return false
		}
		return compareValuesCollated(left, condition.Value, collate) < 0
	case "<=":
		bound := condition.Value
		if condition.Range {
			bound = condition.Value2
		}
		if left.Kind == storage.ValueNull || bound.Kind == storage.ValueNull {
			return false
		}
		if condition.Range {
			return compareValuesCollated(left, bound, collate) < 0
		}
		return compareValuesCollated(left, bound, collate) <= 0
	case "LIKE":
		return likeMatchWithScratch(left, condition.Value, scratch)
	case "NOT LIKE":
		return !likeMatchWithScratch(left, condition.Value, scratch)
	case "INCLUDES":
		return includesConditionMatch(org, left, condition)
	case "EXCLUDES":
		return excludesConditionMatch(org, left, condition)
	case "IN":
		if condition.Subquery != nil {
			return false
		}
		for _, v := range condition.Values {
			if equalFieldValuesInOrgWithScratch(org, definition, condition.Field, left, v, context.Locale, scratch) {
				return true
			}
		}
		return false
	case "NOT IN":
		if condition.Subquery != nil {
			return false
		}
		for _, v := range condition.Values {
			if equalFieldValuesInOrgWithScratch(org, definition, condition.Field, left, v, context.Locale, scratch) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// dateLiteralNow projects the execution instant's local calendar fields into
// a UTC location. The parser stores date-literal bounds as calendar labels,
// so this preserves the running user's date without changing arithmetic.
func dateLiteralNow(now time.Time, timeZoneID string) time.Time {
	local := now.UTC().In(dateLiteralLocation(timeZoneID))
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
}

// MatchesDateLiteralRange shares parsed calendar-range semantics with SOSL.
func MatchesDateLiteralRange(left storage.Value, condition *Condition) (bool, bool) {
	if condition == nil || !condition.Range {
		return false, false
	}
	return matchesDateLiteralRange(left, condition)
}

// matchesDateLiteralRange applies a date-literal range to the stored field
// type. Date fields use the calendar labels in the literal. Datetime fields
// compare against UTC instants for local midnight in the execution user's
// timezone. The parser keeps the range values as Date values so existing Date
// field behavior remains unchanged.
func matchesDateLiteralRange(left storage.Value, condition *Condition) (bool, bool) {
	return matchesDateLiteralRangeWithScratch(left, condition, nil)
}

func matchesDateLiteralRangeWithScratch(left storage.Value, condition *Condition, scratch *predicateScratch) (bool, bool) {
	if left.Kind == storage.ValueNull || condition.Value.Kind != storage.ValueDate || condition.Value2.Kind != storage.ValueDate {
		return false, false
	}
	compareStart, compareEnd := 0, 0
	if left.Kind == storage.ValueDate {
		compareStart = strings.Compare(left.String, condition.Value.String)
		compareEnd = strings.Compare(left.String, condition.Value2.String)
	} else if left.Kind == storage.ValueDateTime {
		zone := strings.TrimSpace(condition.DateLiteralTimeZoneID)
		cacheable := scratch != nil && len(condition.Value.String) <= 64 && len(condition.Value2.String) <= 64 && len(condition.DateLiteralTimeZoneID) <= 64 && (zone == "" || strings.EqualFold(zone, "UTC") || strings.EqualFold(zone, "GMT"))
		key := literalBoundsKey{start: condition.Value.String, end: condition.Value2.String, zone: condition.DateLiteralTimeZoneID, utc: time.UTC}
		var bounds literalBounds
		var found bool
		if cacheable {
			bounds, found = scratch.bounds[key]
		}
		if !found {
			bounds.start, bounds.end, bounds.ok = dateLiteralUTCBounds(key.start, key.end, key.zone)
			if cacheable {
				if scratch.bounds == nil {
					scratch.bounds = make(map[literalBoundsKey]literalBounds)
				}
				clear(scratch.bounds)
				scratch.bounds[key] = bounds
			}
		}
		start, end, ok := bounds.start, bounds.end, bounds.ok
		if !ok {
			return false, false
		}
		value, ok := parseISODateTime(left.String)
		if !ok {
			return false, false
		}
		compareStart = compareTime(value, start)
		compareEnd = compareTime(value, end)
	} else {
		return false, false
	}

	switch condition.Op {
	case "=":
		return compareStart >= 0 && compareEnd < 0, true
	case "!=":
		return compareStart < 0 || compareEnd >= 0, true
	case ">":
		return compareEnd > 0, true
	case ">=":
		return compareStart >= 0, true
	case "<":
		return compareStart < 0, true
	case "<=":
		return compareEnd < 0, true
	default:
		return false, false
	}
}

func dateLiteralUTCBounds(startText, endText, timeZoneID string) (time.Time, time.Time, bool) {
	location := dateLiteralLocation(timeZoneID)
	start, err := time.ParseInLocation("2006-01-02", startText, location)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	end, err := time.ParseInLocation("2006-01-02", endText, location)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return start.UTC(), end.UTC(), true
}

func dateLiteralLocation(timeZoneID string) *time.Location {
	trimmed := strings.TrimSpace(timeZoneID)
	if trimmed == "" || strings.EqualFold(trimmed, "UTC") || strings.EqualFold(trimmed, "GMT") {
		return time.UTC
	}
	if location, err := time.LoadLocation(trimmed); err == nil {
		return location
	}
	if offset, ok := parseDateLiteralFixedOffset(trimmed); ok {
		return time.FixedZone(trimmed, offset)
	}
	return time.UTC
}

func parseDateLiteralFixedOffset(value string) (int, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if strings.HasPrefix(upper, "UTC") || strings.HasPrefix(upper, "GMT") {
		upper = upper[3:]
	} else {
		return 0, false
	}
	if upper == "" {
		return 0, true
	}
	sign := 1
	if upper[0] == '+' {
		upper = upper[1:]
	} else if upper[0] == '-' {
		sign = -1
		upper = upper[1:]
	}
	parts := strings.Split(upper, ":")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" {
		return 0, false
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours > 23 {
		return 0, false
	}
	minutes := 0
	if len(parts) == 2 {
		if len(parts[1]) != 2 {
			return 0, false
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil || minutes > 59 {
			return 0, false
		}
	}
	return sign * (hours*int(time.Hour) + minutes*int(time.Minute)), true
}

func compareTime(left, right time.Time) int {
	if left.Before(right) {
		return -1
	}
	if left.After(right) {
		return 1
	}
	return 0
}

func resolveSubqueries(org storage.OrgState, condition Condition, context ExecutionContext) (Condition, error) {
	if condition.Subquery != nil {
		groupedNegation := groupedNegatedSemiAntiCondition(&condition)
		if len(condition.Subquery.Fields) != 1 || condition.Subquery.Count || len(condition.Subquery.Aggregates) > 0 {
			return Condition{}, fmt.Errorf("soql: semi-join subquery must select exactly one field")
		}
		// Preserve the prior actor-free semi-join boundary while sharing the
		// language needed for translated predicates.
		result, err := ExecuteWithCacheAndContext(org, *condition.Subquery, nil, ExecutionContext{Language: context.Language, FormulaUserID: context.formulaUserID(), PermissionUserFields: context.PermissionUserFields, SetupUserID: context.formulaActor()})
		if err != nil {
			return Condition{}, err
		}
		field := condition.Subquery.Fields[0]
		values := make([]storage.Value, 0, len(result.Records))
		for _, record := range result.Records {
			value, ok := subqueryRecordValue(org, record, field)
			if !ok {
				// References were validated before execution. An omitted value
				// in a projected row is null, not an unknown selected field.
				value = storage.NullValue()
			}
			if value.Kind == storage.ValueNull {
				continue
			}
			values = append(values, value)
		}
		condition.Values = values
		condition.Subquery = nil
		if groupedNegation {
			// Validation limits this form to a top-level AND conjunct.
			// Native matches no rows even when the inner result is empty.
			condition.Not = false
			condition.Op = "IN"
			condition.Values = nil
		}
	}
	for i := range condition.And {
		resolved, err := resolveSubqueries(org, condition.And[i], context)
		if err != nil {
			return Condition{}, err
		}
		condition.And[i] = resolved
	}
	for i := range condition.Or {
		resolved, err := resolveSubqueries(org, condition.Or[i], context)
		if err != nil {
			return Condition{}, err
		}
		condition.Or[i] = resolved
	}
	return condition, nil
}

func subqueryRecordValue(org storage.OrgState, record storage.Record, field string) (storage.Value, bool) {
	if strings.EqualFold(field, "Id") {
		return storage.IDValue(record.ID), true
	}
	if object, ok := org.Objects[record.Object]; ok {
		return recordValue(org, object.Definition, record, field)
	}
	value, ok := record.GetField(field)
	return value, ok
}

func newChildRelationshipQueryCache(execution *ExecutionCache) *childRelationshipQueryCache {
	return &childRelationshipQueryCache{
		indexes:   make(map[string]map[storage.ID][]string),
		prepared:  make(map[string]preparedChildRelationshipQuery),
		execution: execution,
	}
}

type projectionField struct {
	raw    string
	expr   selectFieldExpression
	isExpr bool
}

func prepareProjectionFields(fields []string) []projectionField {
	plan := make([]projectionField, len(fields))
	for i, field := range fields {
		expr, ok := parseSelectFieldExpression(field)
		plan[i] = projectionField{raw: field, expr: expr, isExpr: ok}
	}
	return plan
}

// Subscriber rows have a local storage identity and a distinct visible Id.
// Test the object once per result set so ordinary record projection stays on
// its original path, including projects without platform-event subscribers.
func projectEventSubscriberRows(objectName string, source, projected []storage.Record) {
	if !strings.EqualFold(objectName, "EventBusSubscriber") {
		return
	}
	for i, record := range source {
		if !storage.IsProjectEventSubscriber(record) {
			continue
		}
		projected[i].MetadataSourceUnit = record.MetadataSourceUnit
		if _, selected := projected[i].Fields["Id"]; selected {
			projected[i].Fields["Id"] = storage.IDValue(storage.EventSubscriberVisibleID)
		}
	}
}

func projectRecord(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, fields []projectionField, childQueries []ChildQuery, typeofs []TypeofSpec, childCache *childRelationshipQueryCache, context ExecutionContext) (storage.Record, error) {
	out := storage.Record{
		ID:            record.ID,
		Object:        record.Object,
		Fields:        make(map[string]storage.Value),
		Children:      make(map[string][]storage.Record),
		ExplicitNulls: make(map[string]bool),
		System:        record.System,
	}
	for _, selected := range fields {
		field := selected.raw
		if expr, ok := selected.expr, selected.isExpr; ok {
			if value, ok := selectFieldExpressionValue(org, definition, record, expr, context.formulaActor()); ok {
				if expr.Func == "FORMAT" && context.FormatDatetime != nil && len(expr.Args) == 1 {
					// Native Q7 formatFunction: Datetime FORMAT uses the user's zone and locale.
					if source, found := recordValue(org, definition, record, selectFunctionFieldArg(expr.Args[0]), context.formulaActor()); found && source.Kind == storage.ValueDateTime {
						if text, formatted := context.FormatDatetime(source.String); formatted {
							value = storage.StringValue(text)
						}
					}
				}
				output := expr.outputName()
				if expr.Func == "TOLABEL" && expr.Alias != "" && len(expr.Args) == 1 {
					source := selectFunctionFieldArg(expr.Args[0])
					if dot := strings.LastIndexByte(source, '.'); dot >= 0 {
						qualifier, qualified := storage.ResolveObjectName(org, source[:dot])
						if !qualified || qualifier != definition.APIName {
							if relationship, missing := relationshipLookupMissing(org, record, source); missing {
								out.Fields[relationship] = storage.NullValue()
								continue
							}
							output = source[:dot+1] + expr.Alias
							projectRelationshipIDs(org, record, source, out.Fields)
						}
					}
				}
				out.Fields[output] = value.Clone()
			}
			continue
		}
		if strings.Contains(field, ".") {
			if base, qualified := stripQualifiedCurrentObjectField(org, definition, field); qualified {
				field = base
			}
		}
		canonicalField, ok := storage.ResolveFieldName(definition, org.Namespace, field)
		if !ok {
			canonicalField, ok = canonicalSystemFieldName(field)
		}
		if ok && canonicalField == "Id" {
			out.Fields[canonicalField] = storage.IDValue(record.ID)
			continue
		}
		if strings.Contains(field, ".") {
			if key, value, handled := userRecordAccessProjectionValue(org, record, field, context); handled {
				out.Fields[key] = value
				projectRelationshipIDs(org, record, field, out.Fields)
				continue
			}
			if relationship, missing := relationshipLookupMissing(org, record, field); missing {
				out.Fields[relationship] = storage.NullValue()
				continue
			}
			if value, ok := relationshipValue(org, record, field, context.formulaActor()); ok {
				out.Fields[field] = value
				projectRelationshipIDs(org, record, field, out.Fields)
			}
			continue
		}
		if !ok {
			canonicalField = field
		}
		if record.HasExplicitNull(canonicalField) {
			out.ExplicitNulls[canonicalField] = true
			continue
		}
		if value, ok := calculatedFieldValue(org, definition, record, canonicalField, context.formulaActor()); ok {
			out.Fields[canonicalField] = value.Clone()
			continue
		}
		if value, ok := recordFieldValue(org, record, canonicalField); ok {
			out.Fields[canonicalField] = value.Clone()
			continue
		}
		if value, ok := recordValue(org, definition, record, field, context.formulaActor()); ok {
			out.Fields[canonicalField] = value.Clone()
		}
	}
	for _, childQuery := range childQueries {
		records, err := executeChildRelationshipQuery(org, definition, record, childQuery, childCache, context)
		if err != nil {
			return storage.Record{}, err
		}
		out.Children[childQuery.Relationship] = records
	}
	for _, spec := range typeofs {
		typeName, ok := polymorphicParentObject(org, definition, record, spec.Relationship)
		if !ok {
			continue
		}
		var selected []string
		for name, fields := range spec.When {
			// Native Q6 lowercase: WHEN operands match object names case-insensitively.
			if resolved, ok := storage.ResolveObjectName(org, name); strings.EqualFold(name, typeName) || (ok && resolved == typeName) {
				selected = fields
				break
			}
		}
		if len(selected) == 0 {
			selected = spec.Else
		}
		for _, field := range selected {
			value, ok := relationshipValue(org, record, spec.Relationship+"."+field, context.formulaActor())
			if ok {
				out.Fields[spec.Relationship+"."+field] = value.Clone()
				projectRelationshipIDs(org, record, spec.Relationship+"."+field, out.Fields)
			}
		}
	}
	if len(out.Children) == 0 {
		out.Children = nil
	}
	return out, nil
}

func executeChildRelationshipQuery(org storage.OrgState, parentDefinition storage.ObjectDefinition, parent storage.Record, childQuery ChildQuery, childCache *childRelationshipQueryCache, context ExecutionContext) ([]storage.Record, error) {
	if records, handled, err := executeContentChildAttachedQuery(org, parentDefinition, parent, childQuery, childCache, context); handled {
		return records, err
	}
	prepared, err := prepareChildRelationshipQuery(org, parentDefinition, childQuery, childCache, context)
	if err != nil {
		return nil, err
	}
	childObjectName := prepared.childObjectName
	org, childObjectName, _ = orgWithLazyChildObject(org, childObjectName)
	relation := prepared.relation
	query := prepared.query
	childObject := org.Objects[childObjectName]
	var ids []string
	if indexed, ok := storage.LookupIndex(childObject, relation.Field, storage.IDValue(parent.ID)); ok && !isSystemRelationshipField(relation.Field) {
		ids = make([]string, 0, len(indexed))
		for _, id := range indexed {
			ids = append(ids, string(id))
		}
	} else if !isSystemRelationshipField(relation.Field) {
		index := childRelationshipIndex(org, childObjectName, childObject, relation.Field, childCache)
		ids = append([]string(nil), index[storage.IDIndexKey(parent.ID)]...)
	} else {
		ids = make([]string, 0, len(childObject.Records))
		for id := range childObject.Records {
			ids = append(ids, string(id))
		}
	}
	sort.Strings(ids)
	predicateScratch := &predicateScratch{}
	matched := make([]storage.Record, 0, len(ids))
	for _, idText := range ids {
		child := childObject.Records[storage.ID(idText)]
		if child.System.IsDeleted && !query.AllRows {
			continue
		}
		child.Object = childObjectName
		parentID, ok := recordValue(org, childObject.Definition, child, relation.Field)
		if !ok || !equalValues(parentID, storage.IDValue(parent.ID)) {
			continue
		}
		if matchesWithScratch(org, childObject.Definition, child, query.Where, context, predicateScratch) {
			if context.ChildRecordVisible != nil && !query.HasLimit && !query.HasOffset && !context.ChildRecordVisible(childObjectName, callbackRecord(childObjectName, child)) {
				continue
			}
			matched = append(matched, child)
		}
	}
	if len(query.Order) > 0 {
		sortRecordsByOrder(org, childObject.Definition, matched, collateOrder(query.Order, englishTextLocale(context.Locale)), query.LegacyPicklistNullOrder, query.LegacyRestrictedPicklistNullOrder, context.formulaActor())
	}
	matched = applyWindow(matched, query.Offset, query.Limit, query.HasLimit)
	out := make([]storage.Record, 0, len(matched))
	if len(matched) > 0 && prepared.projection == nil {
		prepared.projection = prepareProjectionFields(query.Fields)
		if childCache != nil {
			key := strings.ToLower(parentDefinition.APIName + "." + childQuery.Relationship)
			childCache.prepared[key] = prepared
		}
	}
	for _, child := range matched {
		projected, err := projectRecord(org, childObject.Definition, child, prepared.projection, query.ChildQueries, query.Typeofs, childCache, context)
		if err != nil {
			return nil, err
		}
		// Native child projections expose the readable parent lookup in the
		// populated-field map even when the child SELECT omits it.
		if context.ChildFieldReadable != nil && !query.HasLimit && !query.HasOffset && context.ChildFieldReadable(childObjectName, relation.Field) {
			if value, found := recordValue(org, childObject.Definition, child, relation.Field); found {
				projected.Fields[relation.Field] = value.Clone()
			}
		}
		out = append(out, projected)
	}
	projectEventSubscriberRows(childObject.Definition.APIName, matched, out)
	return out, nil
}

func prepareChildRelationshipQuery(org storage.OrgState, parentDefinition storage.ObjectDefinition, childQuery ChildQuery, childCache *childRelationshipQueryCache, context ExecutionContext) (preparedChildRelationshipQuery, error) {
	key := strings.ToLower(parentDefinition.APIName + "." + childQuery.Relationship)
	if childCache != nil {
		if prepared, ok := childCache.prepared[key]; ok {
			return prepared, nil
		}
	}
	childObjectName, relation, ok := contentChildRelationshipForQuery(org, parentDefinition, childQuery, childCache)
	if !ok {
		return preparedChildRelationshipQuery{}, fmt.Errorf("soql: unknown child relationship %s on %s", childQuery.Relationship, parentDefinition.APIName)
	}
	org, childObjectName, ok = orgWithLazyChildObject(org, childObjectName)
	if !ok {
		return preparedChildRelationshipQuery{}, childRelationshipUnavailableError(childQuery.Relationship, "")
	}
	childObject := org.Objects[childObjectName]
	query := childQuery.Query
	query.Object = childObjectName
	if query.Count || len(query.Aggregates) > 0 || len(query.GroupBy) > 0 || query.Having != nil {
		return preparedChildRelationshipQuery{}, fmt.Errorf("soql: aggregate child relationship subqueries are not supported")
	}
	query.Fields = normalizeChildRelationshipSelectFields(org, childObjectName, childObject.Definition, query.Fields, childCache)
	fields, err := ExpandFieldsFunctions(childObject.Definition, query.Fields)
	if err != nil {
		return preparedChildRelationshipQuery{}, err
	}
	query.Fields = fields
	if err := validateSemiAntiJoinReferences(org, childObjectName, query, childObject.Definition); err != nil {
		return preparedChildRelationshipQuery{}, err
	}
	if err := validateQueryReferencesWithChildCache(org, childObject.Definition, query, query.SecurityMode, childCache); err != nil {
		return preparedChildRelationshipQuery{}, err
	}
	if query.Where != nil && conditionContainsSubquery(*query.Where) {
		condition, err := resolveSubqueries(org, cloneCondition(*query.Where), context)
		if err != nil {
			return preparedChildRelationshipQuery{}, err
		}
		query.Where = &condition
	}
	prepared := preparedChildRelationshipQuery{childObjectName: childObjectName, relation: relation, query: query}
	if childCache != nil {
		childCache.prepared[key] = prepared
	}
	return prepared, nil
}

func normalizeChildRelationshipSelectFields(org storage.OrgState, childObjectName string, definition storage.ObjectDefinition, fields []string, childCache *childRelationshipQueryCache) []string {
	out := make([]string, len(fields))
	for i, field := range fields {
		out[i] = normalizeChildRelationshipSelectField(org, childObjectName, definition, field, childCache)
	}
	return out
}

func normalizeChildRelationshipSelectField(org storage.OrgState, childObjectName string, definition storage.ObjectDefinition, field string, childCache *childRelationshipQueryCache) string {
	prefix, rest, ok := strings.Cut(field, ".")
	if !ok || strings.TrimSpace(prefix) == "" || strings.TrimSpace(rest) == "" {
		return field
	}
	if childRelationshipFieldQualifierMatchesCached(org, childObjectName, definition, prefix, childCache) {
		return rest
	}
	return field
}

func childRelationshipFieldQualifierMatchesCached(org storage.OrgState, childObjectName string, definition storage.ObjectDefinition, prefix string, childCache *childRelationshipQueryCache) bool {
	if childCache == nil || childCache.execution == nil {
		return childRelationshipFieldQualifierMatches(org, childObjectName, definition, prefix)
	}
	cache := childCache.execution
	key := childRelationshipFieldQualifierCacheKey(org.Namespace, childObjectName, definition.APIName, prefix)
	cache.mu.RLock()
	if cache.childFieldQualifierMatches != nil {
		if cached, ok := cache.childFieldQualifierMatches[key]; ok {
			cache.mu.RUnlock()
			return cached
		}
	}
	cache.mu.RUnlock()

	matched := childRelationshipFieldQualifierMatches(org, childObjectName, definition, prefix)
	cache.mu.Lock()
	if cache.childFieldQualifierMatches == nil {
		cache.childFieldQualifierMatches = make(map[string]bool)
	}
	cache.childFieldQualifierMatches[key] = matched
	cache.mu.Unlock()
	return matched
}

func childRelationshipFieldQualifierCacheKey(namespace, childObjectName, definitionName, prefix string) string {
	return strings.ToLower(namespace) + "\x00" + strings.ToLower(childObjectName) + "\x00" + strings.ToLower(definitionName) + "\x00" + strings.ToLower(strings.TrimSpace(prefix))
}

func childRelationshipFieldQualifierMatches(org storage.OrgState, childObjectName string, definition storage.ObjectDefinition, prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return false
	}
	if strings.EqualFold(prefix, childObjectName) || strings.EqualFold(prefix, definition.APIName) {
		return true
	}
	if resolved, ok := storage.ResolveObjectName(org, prefix); ok {
		return strings.EqualFold(resolved, childObjectName) || strings.EqualFold(resolved, definition.APIName)
	}
	return strings.EqualFold(storage.StripAnyNamespaceToken(prefix), storage.StripAnyNamespaceToken(childObjectName)) ||
		strings.EqualFold(storage.StripAnyNamespaceToken(prefix), storage.StripAnyNamespaceToken(definition.APIName))
}

func childRelationshipIndex(org storage.OrgState, childObjectName string, childObject storage.ObjectState, field string, childCache *childRelationshipQueryCache) map[storage.ID][]string {
	if childCache == nil {
		childCache = newChildRelationshipQueryCache(nil)
	}
	key := strings.ToLower(childObjectName + "." + field)
	if index, ok := childCache.indexes[key]; ok {
		return index
	}
	index := make(map[storage.ID][]string)
	for id, record := range childObject.Records {
		if record.System.IsDeleted {
			continue
		}
		value, ok := recordValue(org, childObject.Definition, record, field)
		if !ok {
			continue
		}
		parentID := idFromValue(value)
		if parentID == "" {
			continue
		}
		parentID = storage.IDIndexKey(parentID)
		index[parentID] = append(index[parentID], string(id))
	}
	for parentID := range index {
		sort.Strings(index[parentID])
	}
	childCache.indexes[key] = index
	return index
}

func childRelationshipCached(org storage.OrgState, parentDefinition storage.ObjectDefinition, relationship string, childCache *childRelationshipQueryCache) (string, storage.Relationship, bool) {
	if childCache == nil || childCache.execution == nil {
		return childRelationship(org, parentDefinition, relationship)
	}
	cache := childCache.execution
	key := childRelationshipCacheKey(org.Namespace, parentDefinition.APIName, relationship) + "\x00" + org.APIVersion
	cache.mu.RLock()
	cached, found := cache.childRelationships[key]
	cache.mu.RUnlock()
	if found {
		return cached.childObjectName, cached.relation, cached.ok
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cached, ok := cache.childRelationships[key]; ok {
		return cached.childObjectName, cached.relation, cached.ok
	}
	if cache.childRelationships == nil {
		cache.childRelationships = make(map[string]childRelationshipResolution)
	}
	childObjectName, relation, ok := childRelationship(org, parentDefinition, relationship)
	cache.childRelationships[key] = childRelationshipResolution{childObjectName: childObjectName, relation: relation, ok: ok}
	return childObjectName, relation, ok
}

func childRelationshipCacheKey(namespace, parentName, relationship string) string {
	return strings.ToLower(namespace) + "\x00" + strings.ToLower(parentName) + "\x00" + strings.ToLower(relationship)
}

func childRelationship(org storage.OrgState, parentDefinition storage.ObjectDefinition, relationship string) (string, storage.Relationship, bool) {
	if name, relation, ok := loadedChildRelationship(org, parentDefinition, relationship); ok {
		return name, relation, true
	}
	child, relation, ok := storage.LookupPlatformChildRelationshipAtVersion(parentDefinition, relationship, org.APIVersion)
	if ok && strings.HasSuffix(strings.ToLower(child), "__share") {
		if _, loaded := storage.ResolveObjectName(org, child); !loaded {
			return "", storage.Relationship{}, false
		}
	}
	return child, relation, ok
}

func loadedChildRelationship(org storage.OrgState, parentDefinition storage.ObjectDefinition, relationship string) (string, storage.Relationship, bool) {
	parentName := parentDefinition.APIName
	bestRank := 99
	var bestObject string
	var bestRelation storage.Relationship
	for childObjectName, childObject := range org.Objects {
		childDefinition := childObject.Definition
		consider := func(relation storage.Relationship) {
			if !storage.PlatformChildRelationshipAvailableAtVersion(parentName, childObjectName, relation.Field, org.APIVersion) {
				return
			}
			rank, matched := childRelationshipMatchRank(org.Namespace, relation, childDefinition, relationship)
			if !matched || !childRelationshipCandidatePreferred(org, childObjectName, rank, relation, bestObject, bestRank, bestRelation) {
				return
			}
			for _, candidate := range relation.ParentObjects {
				if candidate == "*" {
					bestRank = rank
					bestObject = childObjectName
					bestRelation = relation
					break
				}
				resolved, ok := storage.ResolveObjectName(org, candidate)
				if !ok {
					resolved = candidate
				}
				if strings.EqualFold(resolved, parentName) ||
					strings.EqualFold(storage.StripNamespaceToken(org.Namespace, resolved), storage.StripNamespaceToken(org.Namespace, parentName)) ||
					strings.EqualFold(storage.StripAnyNamespaceToken(resolved), storage.StripAnyNamespaceToken(parentName)) {
					bestRank = rank
					bestObject = childObjectName
					bestRelation = relation
					break
				}
			}
		}
		for _, relation := range childDefinition.Relations {
			consider(relation)
		}
		storage.VisitStandardObjectRelationships(childDefinition.APIName, nil, consider)
		for _, relation := range syntheticContentDocumentLinkRelationship(childDefinition) {
			consider(relation)
		}
		visitSyntheticStandardSystemChildRelationships(childDefinition, consider)
		visitSyntheticSystemChildRelationships(childDefinition, consider)
		if relation, ok := syntheticPrefixedCustomChildRelationship(org, parentDefinition, childDefinition, relationship); ok {
			consider(relation)
		}
	}
	if bestRank != 99 {
		return bestObject, bestRelation, true
	}
	return "", storage.Relationship{}, false
}

func syntheticPrefixedCustomChildRelationship(org storage.OrgState, parentDefinition, childDefinition storage.ObjectDefinition, queryName string) (storage.Relationship, bool) {
	parentName := strings.TrimSpace(parentDefinition.APIName)
	childName := strings.TrimSpace(childDefinition.APIName)
	if !customObjectLikeSOQLName(parentName) || !customObjectLikeSOQLName(childName) {
		return storage.Relationship{}, false
	}
	parentBase := strings.TrimSuffix(parentName, "__c")
	childBase := strings.TrimSuffix(childName, "__c")
	if parentBase == "" || childBase == "" || !hasPrefixFold(childBase, parentBase) || strings.EqualFold(parentBase, childBase) {
		return storage.Relationship{}, false
	}
	childRelationship := childBase + "s__r"
	if !childRelationshipNameMatches(org.Namespace, childRelationship, queryName) {
		return storage.Relationship{}, false
	}
	return storage.Relationship{
		Field:              parentName,
		ParentObjects:      []string{parentName},
		ParentRelationship: parentBase + "__r",
		ChildRelationship:  childRelationship,
	}, true
}

func childRelationshipCandidatePreferred(org storage.OrgState, childObjectName string, rank int, relation storage.Relationship, bestObjectName string, bestRank int, bestRelation storage.Relationship) bool {
	if bestRank == 99 {
		return true
	}
	leftObjectPriority := childRelationshipObjectPriority(org, childObjectName)
	rightObjectPriority := childRelationshipObjectPriority(org, bestObjectName)
	if leftObjectPriority != rightObjectPriority {
		return leftObjectPriority < rightObjectPriority
	}
	if rank < bestRank {
		return true
	}
	if rank > bestRank {
		return false
	}
	leftFieldPriority := childRelationshipFieldPriority(relation.Field)
	rightFieldPriority := childRelationshipFieldPriority(bestRelation.Field)
	if leftFieldPriority != rightFieldPriority {
		return leftFieldPriority < rightFieldPriority
	}
	return childRelationshipObjectPreferred(org, childObjectName, bestObjectName)
}

func childRelationshipFieldPriority(field string) int {
	canonical, ok := canonicalSystemFieldName(field)
	if !ok {
		return 0
	}
	switch canonical {
	case "CreatedById":
		return 10
	case "LastModifiedById":
		return 11
	case "OwnerId":
		return 12
	default:
		return 20
	}
}

func childRelationshipObjectPreferred(org storage.OrgState, left, right string) bool {
	leftPriority := childRelationshipObjectPriority(org, left)
	rightPriority := childRelationshipObjectPriority(org, right)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	leftState, leftOK := org.Objects[left]
	rightState, rightOK := org.Objects[right]
	if leftOK && rightOK {
		leftScore := len(leftState.Definition.Fields) + len(leftState.Definition.Relations)
		rightScore := len(rightState.Definition.Fields) + len(rightState.Definition.Relations)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
	}
	return left < right
}

func childRelationshipObjectPriority(org storage.OrgState, objectName string) int {
	if org.Namespace != "" && hasPrefixFold(objectName, org.Namespace+"__") && customObjectLikeSOQLName(objectName) {
		return 0
	}
	if customObjectLikeSOQLName(objectName) {
		return 1
	}
	if storage.IsKnownStandardObject(objectName) {
		return 2
	}
	return 3
}

func syntheticContentDocumentLinkRelationship(definition storage.ObjectDefinition) []storage.Relationship {
	if !strings.EqualFold(definition.APIName, "ContentDocumentLink") {
		return nil
	}
	field, ok := fieldDefinitionsForReferenceField(definition, "LinkedEntityId")
	if !ok || field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
		return nil
	}
	return []storage.Relationship{{
		Field:              field.APIName,
		ParentObjects:      []string{"*"},
		ParentRelationship: "LinkedEntity",
		ChildRelationship:  "ContentDocumentLinks",
		Polymorphic:        true,
	}}
}

func isSystemRelationshipField(field string) bool {
	canonical, ok := canonicalSystemFieldName(field)
	if !ok {
		return false
	}
	switch canonical {
	case "CreatedById", "LastModifiedById", "OwnerId":
		return true
	default:
		return false
	}
}

func visitSyntheticStandardSystemChildRelationships(definition storage.ObjectDefinition, visit func(storage.Relationship)) {
	fields := []struct {
		name         string
		relationship string
	}{
		{name: "CreatedById", relationship: "CreatedBy"},
		{name: "LastModifiedById", relationship: "LastModifiedBy"},
	}
	if storage.IsOwnerBackedObject(definition.APIName) {
		fields = append(fields, struct {
			name         string
			relationship string
		}{name: "OwnerId", relationship: "Owner"})
	}
	for _, field := range fields {
		if hasSOQLRelationForField(definition.Relations, field.name) {
			continue
		}
		if _, ok := fieldDefinitionsForReferenceField(definition, field.name); ok {
			continue
		}
		visit(storage.Relationship{
			Field:              field.name,
			ParentObjects:      []string{"User"},
			ParentRelationship: field.relationship,
			ChildRelationship:  derivedChildRelationshipName(definition),
		})
	}
}

func syntheticSystemChildRelationships(definition storage.ObjectDefinition) []storage.Relationship {
	fields := []string{"CreatedById", "LastModifiedById", "OwnerId"}
	relations := make([]storage.Relationship, 0, len(fields))
	visitSyntheticSystemChildRelationships(definition, func(relation storage.Relationship) {
		relations = append(relations, relation)
	})
	return relations
}

func visitSyntheticSystemChildRelationships(definition storage.ObjectDefinition, visit func(storage.Relationship)) {
	fields := []string{"CreatedById", "LastModifiedById", "OwnerId"}
	for _, fieldName := range fields {
		if hasSOQLRelationForField(definition.Relations, fieldName) {
			continue
		}
		field, ok := fieldDefinitionsForReferenceField(definition, fieldName)
		if !ok || field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		visit(storage.Relationship{
			Field:              field.APIName,
			ParentObjects:      append([]string(nil), field.ReferenceTo...),
			ParentRelationship: strings.TrimSuffix(field.APIName, "Id"),
			ChildRelationship:  derivedChildRelationshipName(definition),
			Polymorphic:        len(field.ReferenceTo) > 1,
		})
	}
}

func hasSOQLRelationForField(relations []storage.Relationship, fieldName string) bool {
	for _, relation := range relations {
		if strings.EqualFold(relation.Field, fieldName) {
			return true
		}
	}
	return false
}

func childRelationshipMatchRank(namespace string, relation storage.Relationship, definition storage.ObjectDefinition, queryName string) (int, bool) {
	if relation.ChildRelationship != "" {
		if rank, ok := childRelationshipNameMatchRank(namespace, relation.ChildRelationship, queryName); ok {
			return rank, true
		}
	}
	derived := derivedChildRelationshipName(definition)
	if rank, ok := childRelationshipNameMatchRank(namespace, derived, queryName); ok {
		return rank + 10, true
	}
	return 99, false
}

func childRelationshipNameFromObject(objectName string) string {
	if idx := strings.LastIndex(objectName, "."); idx >= 0 && idx+1 < len(objectName) {
		return objectName[idx+1:]
	}
	return objectName
}

func childRelationshipNameMatches(namespace, metadataName, queryName string) bool {
	_, ok := childRelationshipNameMatchRank(namespace, metadataName, queryName)
	return ok
}

func childRelationshipNameMatchRank(namespace, metadataName, queryName string) (int, bool) {
	if rank, ok := relationshipNameMatchRank(namespace, metadataName, queryName); ok {
		return rank, true
	}
	strippedQuery := storage.StripNamespaceToken(namespace, queryName)
	if hasSuffixFold(strippedQuery, "__r") {
		if rank, ok := relationshipNameMatchRank(namespace, metadataName+"__r", strippedQuery); ok {
			return rank + 1, true
		}
	}
	if hasSuffixFold(metadataName, "__r") {
		base := strings.TrimSuffix(metadataName, metadataName[len(metadataName)-3:])
		if rank, ok := relationshipNameMatchRank(namespace, base, strippedQuery); ok {
			return rank + 1, true
		}
	}
	return 99, false
}

func relationshipNameMatchRank(namespace, canonical, candidate string) (int, bool) {
	if canonical == candidate || strings.EqualFold(canonical, candidate) {
		return 0, true
	}
	strippedCanonical := canonical
	strippedCandidate := candidate
	if namespace != "" {
		strippedCanonical = storage.StripNamespaceToken(namespace, canonical)
		strippedCandidate = storage.StripNamespaceToken(namespace, candidate)
		if canonical == strippedCandidate ||
			strings.EqualFold(canonical, strippedCandidate) ||
			strippedCanonical == candidate ||
			strings.EqualFold(strippedCanonical, candidate) ||
			strippedCanonical == strippedCandidate ||
			strings.EqualFold(strippedCanonical, strippedCandidate) {
			return 1, true
		}
	}
	anyCanonical := storage.StripAnyNamespaceToken(canonical)
	anyCandidate := storage.StripAnyNamespaceToken(candidate)
	if anyCanonical == anyCandidate || strings.EqualFold(anyCanonical, anyCandidate) {
		return 4, true
	}
	return 99, false
}

func derivedChildRelationshipName(definition storage.ObjectDefinition) string {
	if strings.TrimSpace(definition.PluralLabel) != "" {
		return normalizeDerivedChildRelationshipName(definition.PluralLabel)
	}
	if strings.TrimSpace(definition.Label) != "" {
		return normalizeDerivedChildRelationshipName(definition.Label)
	}
	if definition.APIName != "" {
		return normalizeDerivedChildRelationshipName(definition.APIName)
	}
	return ""
}

func normalizeDerivedChildRelationshipName(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, "ys") && len(name) > 2 {
		return strings.TrimSuffix(name, "ys") + "ies"
	}
	if strings.HasSuffix(name, "s") {
		return name
	}
	if strings.HasSuffix(name, "y") && len(name) > 1 {
		return strings.TrimSuffix(name, "y") + "ies"
	}
	return name + "s"
}

// ExpandFieldsFunctions resolves FIELDS projections from the schema, including
// selected fields whose values are absent from a returned record.
func ExpandFieldsFunctions(definition storage.ObjectDefinition, fields []string) ([]string, error) {
	out := make([]string, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	hasExpansion := false
	for _, field := range fields {
		if _, ok := fieldsFunctionMode(field); ok {
			hasExpansion = true
		}
	}
	appendField := func(field string) error {
		key := strings.ToLower(field)
		if seen[key] {
			if hasExpansion {
				return queryError("duplicate field selected: "+field, "duplicate field selected: "+field)
			}
			return nil
		}
		seen[key] = true
		out = append(out, field)
		return nil
	}
	names := make([]string, 0, len(definition.Fields))
	for name := range definition.Fields {
		if strings.EqualFold(name, "Id") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, field := range fields {
		mode, ok := fieldsFunctionMode(field)
		if !ok {
			if err := appendField(field); err != nil {
				return nil, err
			}
			continue
		}
		switch mode {
		case "ALL":
			if err := appendField("Id"); err != nil {
				return nil, err
			}
			for _, name := range names {
				if err := appendField(name); err != nil {
					return nil, err
				}
			}
		case "STANDARD":
			if err := appendField("Id"); err != nil {
				return nil, err
			}
			for _, name := range names {
				if !isCustomFieldName(name) {
					if err := appendField(name); err != nil {
						return nil, err
					}
				}
			}
		case "CUSTOM":
			for _, name := range names {
				if isCustomFieldName(name) {
					if err := appendField(name); err != nil {
						return nil, err
					}
				}
			}
		default:
			return nil, queryError("The SOQL FIELDS function only supports arguments of (all), (custom), (standard); ("+mode+") is not supported.", "unsupported FIELDS("+mode+")")
		}
	}
	return out, nil
}

func fieldsFunctionMode(field string) (string, bool) {
	upper := strings.ToUpper(field)
	if !strings.HasPrefix(upper, "FIELDS(") || !strings.HasSuffix(upper, ")") {
		return "", false
	}
	return strings.TrimSpace(upper[len("FIELDS(") : len(upper)-1]), true
}

func validateQueryReferences(org storage.OrgState, definition storage.ObjectDefinition, query Query, mode string) error {
	return validateQueryReferencesWithChildCache(org, definition, query, mode, nil)
}

// ValidateReferences shares field and relationship resolution with consumers
// such as SOSL, without executing a SELECT or spending query governor limits.
func ValidateReferences(org storage.OrgState, definition storage.ObjectDefinition, query Query) error {
	if err := validateQueryReferences(org, definition, query, ""); err != nil {
		// R081/R112: SOSL reports the same native column text for a plain
		// missing projection, filter or sort field, even with no candidates.
		// Existing SELECT callers retain their own diagnostic adapters.
		var fields []string
		fields = append(fields, query.Fields...)
		var conditions func(*Condition)
		conditions = func(condition *Condition) {
			if condition == nil {
				return
			}
			if condition.Field != "" {
				fields = append(fields, condition.Field)
			}
			for i := range condition.And {
				conditions(&condition.And[i])
			}
			for i := range condition.Or {
				conditions(&condition.Or[i])
			}
		}
		conditions(query.Where)
		for _, order := range query.Order {
			fields = append(fields, order.Field)
		}
		for _, field := range fields {
			if !strings.ContainsAny(field, ".() ") && !fieldKnown(org, definition, field) {
				message := MissingColumnMessage(field, definition.APIName)
				return queryError(message, message)
			}
		}
		return err
	}
	return nil
}

func validateQueryReferencesWithChildCache(org storage.OrgState, definition storage.ObjectDefinition, query Query, mode string, childCache *childRelationshipQueryCache) error {
	if err := UserRecordAccessQueryError(query); err != nil {
		return err
	}
	if err := validateDateFunctionGrouping(query, org, &definition); err != nil {
		return err
	}
	if err := validateAggregateAliases(query); err != nil {
		return err
	}
	// record_type_visibility groupId rows: an absent RecordTypeId in GROUP BY
	// is reported before the SELECT list, as an invalid field.
	if mode == "" {
		for _, field := range query.GroupBy {
			if !strings.Contains(field, ".") && absentRecordTypeReference(org, definition, field) {
				message := "Invalid field: '" + strings.TrimSpace(field) + "'"
				return queryError(message, message)
			}
		}
	}
	for _, field := range query.Fields {
		if expr, ok := parseSelectFieldExpression(field); ok {
			if err := validateSelectFieldExpression(org, definition, expr, mode); err != nil {
				return err
			}
			continue
		}
		aggregate, ok, err := parseAggregateField(field)
		if err != nil {
			return err
		}
		if ok {
			if err := validateAggregateReference(org, definition, aggregate, mode); err != nil {
				return err
			}
			continue
		}
		if raw, _, ok := splitSelectFieldAlias(field); ok {
			field = raw
		}
		if err := validateFieldReference(org, definition, field, mode); err != nil {
			return err
		}
	}
	for _, aggregate := range query.Aggregates {
		if err := validateAggregateReference(org, definition, aggregate, mode); err != nil {
			return err
		}
	}
	for _, aggregate := range query.HavingAggregates {
		if err := validateAggregateReference(org, definition, aggregate, mode); err != nil {
			return err
		}
	}
	for _, aggregate := range query.orderAggregates {
		if err := validateAggregateReference(org, definition, aggregate, mode); err != nil {
			return err
		}
	}
	for _, field := range query.GroupBy {
		if err := validateFieldReference(org, definition, field, mode); err != nil {
			return err
		}
		if fields, known := fieldDefinitionsForReference(org, definition, field); known {
			for _, fieldDef := range fields {
				if fieldDef.Groupable != nil && !*fieldDef.Groupable {
					message := fmt.Sprintf("field '%s' can not be grouped in a query call", field)
					return unexpectedQueryError(message, message)
				}
			}
		}
	}
	if err := validateConditionReferences(org, definition, query.Where, mode); err != nil {
		return err
	}
	if err := validateHavingReferences(org, definition, query, mode); err != nil {
		return err
	}
	for _, spec := range query.Order {
		if queryHasAggregates(query) {
			if aggregateOrderFieldKnown(query, spec.Field) {
				continue
			}
			if !fieldKnown(org, definition, spec.Field) {
				return fieldUnavailableError(spec.Field, mode)
			}
			return fmt.Errorf("soql: aggregate ORDER BY field %s must be grouped or aggregated", spec.Field)
		}
		if expr, ok := parseSelectFieldExpression(spec.Field); ok {
			if err := validateSelectFieldExpression(org, definition, expr, mode); err != nil {
				return err
			}
			continue
		}
		if err := validateFieldReference(org, definition, spec.Field, mode); err != nil {
			return err
		}
	}
	for _, spec := range query.Typeofs {
		prefix := strings.ToLower(spec.Relationship) + "."
		for _, field := range query.Fields {
			if strings.HasPrefix(strings.ToLower(field), prefix) {
				// Native Q7 overlap: a top-level path through the TYPEOF relationship.
				name := strings.Fields(field)[0]
				message := "TYPEOF operand '" + spec.Relationship + "' and top-level SELECT list element '" + name + "' may not reference the same relationship field"
				return queryError(message, message)
			}
		}
		if err := validateTypeofReference(org, definition, spec, mode); err != nil {
			return err
		}
	}
	for _, childQuery := range query.ChildQueries {
		childName, _, ok := contentChildRelationshipForQuery(org, definition, childQuery, childCache)
		if !ok {
			return childRelationshipUnavailableError(childQuery.Relationship, mode)
		}
		org, childName, ok = orgWithLazyChildObject(org, childName)
		if !ok {
			return childRelationshipUnavailableError(childQuery.Relationship, mode)
		}
		child := org.Objects[childName]
		normalizedFields := normalizeChildRelationshipSelectFields(org, childName, child.Definition, childQuery.Query.Fields, childCache)
		childFields, err := ExpandFieldsFunctions(child.Definition, normalizedFields)
		if err != nil {
			return err
		}
		nested := childQuery.Query
		nested.Fields = childFields
		nested.SecurityMode = mode
		if err := validateQueryReferencesWithChildCache(org, child.Definition, nested, mode, childCache); err != nil {
			return err
		}
	}
	return nil
}

func validateAggregateReference(org storage.OrgState, definition storage.ObjectDefinition, aggregate Aggregate, mode string) error {
	if aggregate.Field == "" || aggregate.Field == "*" {
		return nil
	}
	if err := validateFieldReference(org, definition, aggregate.Field, mode); err != nil {
		return err
	}
	if fields, known := fieldDefinitionsForReference(org, definition, aggregate.Field); known {
		for _, field := range fields {
			if field.Type == storage.FieldTime || field.Aggregatable != nil && !*field.Aggregatable {
				message := fmt.Sprintf("field %s does not support aggregate operator %s", aggregate.Field, aggregate.Func)
				return queryError(message, message)
			}
		}
	}
	if (aggregate.Func == "SUM" || aggregate.Func == "AVG") && !aggregateFieldMayBeNumeric(org, definition, aggregate.Field) {
		message := fmt.Sprintf("field %s does not support aggregate operator %s", aggregate.Field, aggregate.Func)
		return queryError(message, fmt.Sprintf("%s requires numeric field %s", aggregate.Func, aggregate.Field))
	}
	if aggregate.Func == "MIN" || aggregate.Func == "MAX" {
		if fields, known := fieldDefinitionsForReference(org, definition, aggregate.Field); known {
			for _, field := range fields {
				if field.Type == storage.FieldBoolean {
					message := fmt.Sprintf("field %s does not support aggregate operator %s", aggregate.Field, aggregate.Func)
					return queryError(message, message)
				}
			}
		}
	}
	return nil
}

func aggregateFieldMayBeNumeric(org storage.OrgState, definition storage.ObjectDefinition, field string) bool {
	if field == "" || field == "*" {
		return true
	}
	fields, ok := fieldDefinitionsForReference(org, definition, field)
	if !ok || len(fields) == 0 {
		return true
	}
	for _, field := range fields {
		switch field.Type {
		case storage.FieldAny, storage.FieldInteger, storage.FieldDecimal, storage.FieldCalculated, storage.FieldSummary:
			continue
		default:
			return false
		}
	}
	return true
}

func fieldDefinitionsForReference(org storage.OrgState, definition storage.ObjectDefinition, field string) ([]storage.Field, bool) {
	if canonical, ok := resolveSOQLFieldName(definition, org.Namespace, field); ok && canonical == "Id" {
		return []storage.Field{{APIName: "Id", Type: storage.FieldID}}, true
	}
	if systemField, ok := systemFieldDefinition(field); ok {
		return []storage.Field{systemField}, true
	}
	if strings.Contains(field, ".") {
		parts := strings.SplitN(field, ".", 2)
		if strings.EqualFold(parts[1], "Id") && isSystemParentRelationship(parts[0]) {
			return []storage.Field{{APIName: parts[1], Type: storage.FieldID}}, true
		}
		for _, relation := range matchingParentRelations(org.Namespace, definition, parts[0]) {
			if relation.Polymorphic && strings.EqualFold(parts[1], "Type") {
				return []storage.Field{{APIName: "Type", Type: storage.FieldString}}, true
			}
			if len(relation.ParentObjects) == 0 {
				return nil, false
			}
			var out []storage.Field
			for _, parentName := range relation.ParentObjects {
				if strings.EqualFold(parentName, "EntityDefinition") || strings.EqualFold(parentName, "FieldDefinition") {
					if entityDefinitionFieldKnown(parts[1]) {
						out = append(out, entityDefinitionRelationshipField(parts[1]))
						continue
					}
				}
				canonical, ok := storage.ResolveObjectName(org, parentName)
				if !ok {
					var orgWithStandard storage.OrgState
					orgWithStandard, canonical, ok = orgWithLazyStandardObject(org, parentName)
					if !ok {
						return nil, false
					}
					org = orgWithStandard
				}
				fields, ok := fieldDefinitionsForReference(org, org.Objects[canonical].Definition, parts[1])
				if !ok {
					return nil, false
				}
				out = append(out, fields...)
			}
			return out, len(out) > 0
		}
		return nil, false
	}
	canonicalField, ok := resolveSOQLFieldName(definition, org.Namespace, field)
	if !ok {
		return nil, false
	}
	return []storage.Field{definition.Fields[canonicalField]}, true
}

func isSystemParentRelationship(relationship string) bool {
	switch {
	case strings.EqualFold(relationship, "CreatedBy"):
		return true
	case strings.EqualFold(relationship, "LastModifiedBy"):
		return true
	case strings.EqualFold(relationship, "Owner"):
		return true
	case strings.EqualFold(relationship, "RecordType"):
		return true
	default:
		return false
	}
}

func orgWithLazyStandardObject(org storage.OrgState, objectName string) (storage.OrgState, string, bool) {
	if canonical, ok := storage.ResolveObjectName(org, objectName); ok {
		return org, canonical, true
	}
	canonical, known := storage.ResolveKnownStandardObjectName(objectName)
	if !known {
		return org, "", false
	}
	objectName = canonical
	orgWithStandard := org
	orgWithStandard.Objects = make(map[string]storage.ObjectState, len(org.Objects)+1)
	for name, state := range org.Objects {
		orgWithStandard.Objects[name] = state
	}
	storage.EnsureStandardObject(&orgWithStandard, objectName)
	canonical, ok := storage.ResolveObjectName(orgWithStandard, objectName)
	return orgWithStandard, canonical, ok
}

// capturedVersionedRootIDQuery recognizes the independently captured root
// Id-only boundary without generalizing to unobserved fields or query shapes.
func capturedVersionedRootIDQuery(query Query) bool {
	return len(query.Fields) == 1 && strings.EqualFold(query.Fields[0], "Id") &&
		len(query.ChildQueries) == 0 && len(query.Typeofs) == 0 && query.Where == nil && query.Having == nil &&
		len(query.Order) == 0 && query.OrderBy == "" && query.HasLimit && query.Limit == 1 && query.LimitBind == "" &&
		!query.HasOffset && !query.Count && !query.ForUpdate && !query.ForView && !query.ForReference && !query.AllRows &&
		query.UsingScope == "" && len(query.Aggregates) == 0 && len(query.HavingAggregates) == 0 && len(query.GroupBy) == 0
}

// orgWithLazyChildObject materializes a captured Id projection for relationship
// queries and the separately observed root Id-only query, not general hydration.
func orgWithLazyChildObject(org storage.OrgState, objectName string) (storage.OrgState, string, bool) {
	if next, canonical, ok := orgWithLazyStandardObject(org, objectName); ok {
		return next, canonical, true
	}
	definition, ok := storage.VersionedPlatformChildObject(objectName, org.APIVersion)
	if !ok {
		return org, "", false
	}
	next := org
	next.Objects = make(map[string]storage.ObjectState, len(org.Objects)+1)
	for name, state := range org.Objects {
		next.Objects[name] = state
	}
	next.Objects[definition.APIName] = storage.ObjectState{Definition: definition, Records: make(map[storage.ID]storage.Record)}
	return next, definition.APIName, true
}

func systemFieldDefinition(field string) (storage.Field, bool) {
	canonical, ok := canonicalSystemFieldName(field)
	if !ok {
		return storage.Field{}, false
	}
	switch canonical {
	case "CreatedDate", "LastModifiedDate", "SystemModstamp":
		return storage.Field{APIName: canonical, Type: storage.FieldDateTime}, true
	case "CreatedById", "LastModifiedById", "OwnerId", "RecordTypeId":
		return storage.Field{APIName: canonical, Type: storage.FieldID}, true
	case "IsDeleted":
		return storage.Field{APIName: canonical, Type: storage.FieldBoolean}, true
	default:
		return storage.Field{}, false
	}
}

func validateHavingReferences(org storage.OrgState, definition storage.ObjectDefinition, query Query, mode string) error {
	if query.Having == nil {
		return nil
	}
	return validateAggregateConditionReferences(org, definition, query, *query.Having, mode)
}

// AggregateHavingColumnMessage is the native rejection text for an unknown
// HAVING column (R214/C013). Ordinary field validation keeps its base behavior.
func AggregateHavingColumnMessage(field, object string) string {
	return "No such column '" + field + "' on entity '" + object + "'. If you are attempting to use a custom field, be sure to append the '__c' after the custom field name. Please reference your WSDL or the describe call for the appropriate names."
}

func validateAggregateConditionReferences(org storage.OrgState, definition storage.ObjectDefinition, query Query, condition Condition, mode string) error {
	if condition.Not {
		condition.Not = false
		return validateAggregateConditionReferences(org, definition, query, condition, mode)
	}
	for i := range condition.And {
		if err := validateAggregateConditionReferences(org, definition, query, condition.And[i], mode); err != nil {
			return err
		}
	}
	for i := range condition.Or {
		if err := validateAggregateConditionReferences(org, definition, query, condition.Or[i], mode); err != nil {
			return err
		}
	}
	if condition.Field == "" {
		return nil
	}
	if condition.RewrittenAggregate && aggregateOrderFieldKnown(query, condition.Field) {
		return nil
	}
	aggregate, ok, err := parseAggregateField(condition.Field)
	if err != nil {
		return err
	}
	if ok {
		if err := validateAggregateReference(org, definition, aggregate, mode); err != nil {
			return err
		}
		return fmt.Errorf("soql: HAVING aggregate field %s must be selected or aliased", condition.Field)
	}
	if containsName(query.GroupBy, condition.Field) {
		return validateFieldReference(org, definition, condition.Field, mode)
	}
	if fieldKnown(org, definition, condition.Field) {
		return fmt.Errorf("soql: HAVING field %s must be grouped or aggregated", condition.Field)
	}
	message := AggregateHavingColumnMessage(condition.Field, definition.APIName)
	return queryError(message, message)
}

func validateConditionReferences(org storage.OrgState, definition storage.ObjectDefinition, condition *Condition, mode string) error {
	if condition == nil {
		return nil
	}
	if condition.Not {
		nested := *condition
		nested.Not = false
		return validateConditionReferences(org, definition, &nested, mode)
	}
	for i := range condition.And {
		if err := validateConditionReferences(org, definition, &condition.And[i], mode); err != nil {
			return err
		}
	}
	for i := range condition.Or {
		if err := validateConditionReferences(org, definition, &condition.Or[i], mode); err != nil {
			return err
		}
	}
	if condition.Formula != nil {
		return validateWhereFormula(org, definition, *condition, mode)
	}
	if condition.Field != "" {
		if expr, ok := parseSelectFieldExpression(condition.Field); ok {
			if err := validateSelectFieldExpression(org, definition, expr, mode); err != nil {
				return err
			}
		} else if err := validateFieldReference(org, definition, condition.Field, mode); err != nil {
			return err
		}
		if condition.Op == "INCLUDES" || condition.Op == "EXCLUDES" {
			if err := validateMultiSelectPicklistCondition(org, definition, *condition, mode); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMultiSelectPicklistCondition(org storage.OrgState, definition storage.ObjectDefinition, condition Condition, mode string) error {
	fields, ok := fieldDefinitionsForReference(org, definition, condition.Field)
	if !ok || len(fields) == 0 {
		return fieldUnavailableError(condition.Field, mode)
	}
	for _, field := range fields {
		if field.Type != storage.FieldMultiPicklist {
			return fmt.Errorf("soql: %s requires multi-select picklist field %s", condition.Op, condition.Field)
		}
	}
	return nil
}

func validateTypeofReference(org storage.OrgState, definition storage.ObjectDefinition, spec TypeofSpec, mode string) error {
	allowedTargets := map[string]bool{}
	relationshipKnown := false
	polymorphic := false
	for _, relation := range matchingParentRelations(org.Namespace, definition, spec.Relationship) {
		relationshipKnown = true
		polymorphic = polymorphic || relation.Polymorphic || len(relation.ParentObjects) > 1
		for _, parentName := range relation.ParentObjects {
			canonical, ok := storage.ResolveObjectName(org, parentName)
			if ok {
				allowedTargets[canonical] = true
			}
		}
	}
	if !relationshipKnown {
		message := MissingColumnMessage(spec.Relationship, definition.APIName)
		message = strings.Replace(message, "No such column", "No such relation", 1)
		return queryError(message, message)
	}
	if !polymorphic {
		message := "TYPEOF operand '" + spec.Relationship + "' is not a polymorphic relationship field"
		return queryError(message, message)
	}
	for typeName, fields := range spec.When {
		parentName, ok := storage.ResolveObjectName(org, typeName)
		if !ok || !allowedTargets[parentName] {
			// The captured diagnostics are for ordinary queries; retain the
			// existing security-mode behavior until that route has evidence.
			if mode != "" {
				return typeofTargetUnavailableError(typeName, mode)
			}
			if !ok {
				message := MissingObjectMessage(typeName)
				return queryError(message, message)
			}
			message := fmt.Sprintf("WHEN operand '%s' is not a valid entity type for polymorphic field '%s'", typeName, spec.Relationship)
			return queryError(message, message)
		}
		parent := org.Objects[parentName]
		for _, field := range fields {
			if err := validateFieldReference(org, parent.Definition, field, mode); err != nil {
				message := MissingColumnMessage(field, parent.Definition.APIName)
				return queryError(message, message)
			}
		}
	}
	for _, field := range spec.Else {
		// ELSE has the common polymorphic Name shape, not the union of the
		// concrete WHEN objects' fields.
		if !strings.EqualFold(field, "Id") && !strings.EqualFold(field, "Name") && !strings.EqualFold(field, "Type") {
			message := MissingColumnMessage(field, "Name")
			return queryError(message, message)
		}
	}
	return nil
}

func validateFieldReference(org storage.OrgState, definition storage.ObjectDefinition, field string, mode string) error {
	if handled, err := validateUserRecordAccessProjection(org, definition, field); handled {
		return err
	}
	if relationship, absent := absentRecordTypeRelationship(org, definition, field); absent && mode == "" {
		message := MissingRelationshipMessage(relationship, "field path")
		return queryError(message, message)
	}
	if strings.Contains(field, ".") {
		if base, ok := stripQualifiedCurrentObjectField(org, definition, field); ok {
			field = base
		}
	}
	if expr, ok := parseSelectFieldExpression(field); ok {
		return validateSelectFieldExpression(org, definition, expr, mode)
	}
	if fieldKnownForMode(org, definition, field, mode != "") {
		return nil
	}
	if strings.Contains(field, ".") {
		parts := strings.Split(field, ".")
		current := definition
		for _, relationship := range parts[:len(parts)-1] {
			relations := matchingParentRelations(org.Namespace, current, relationship)
			// R221/R222 back the missing leaf diagnostic on resolved,
			// nonpolymorphic parent paths. Do not choose the first target of
			// an ambiguous relationship or invent a target for missing metadata.
			if len(relations) != 1 || relations[0].Polymorphic || len(relations[0].ParentObjects) != 1 {
				return fieldUnavailableError(field, mode)
			}
			parent, ok := storage.ResolveObjectName(org, relations[0].ParentObjects[0])
			if !ok {
				return fieldUnavailableError(field, mode)
			}
			current = org.Objects[parent].Definition
		}
		leaf := parts[len(parts)-1]
		if _, known := storage.ResolveFieldName(current, org.Namespace, leaf); known {
			return fieldUnavailableError(field, mode)
		}
		message := MissingColumnMessage(leaf, current.APIName)
		return queryError(message, message)
	}
	if mode == "" {
		// Unqualified fields use the captured missing-column diagnostic.
		// Relationship and access-mode failures retain their existing paths.
		return queryError(MissingColumnMessage(field, definition.APIName), "unknown field "+field)
	}
	return fieldUnavailableError(field, mode)
}

func MissingObjectMessage(object string) string {
	return "sObject type '" + object + "' is not supported. If you are attempting to use a custom object, be sure to append the '__c' after the entity name. Please reference your WSDL or the describe call for the appropriate names."
}

func MissingColumnMessage(field, object string) string {
	return "No such column '" + field + "' on entity '" + object + "'. If you are attempting to use a custom field, be sure to append the '__c' after the custom field name. Please reference your WSDL or the describe call for the appropriate names."
}

func MissingRelationshipMessage(relationship, part string) string {
	return "Didn't understand relationship '" + relationship + "' in " + part + ". If you are attempting to use a custom relationship, be sure to append the '__r' after the custom relationship name. Please reference your WSDL or the describe call for the appropriate names."
}

func fieldKnown(org storage.OrgState, definition storage.ObjectDefinition, field string) bool {
	return fieldKnownForMode(org, definition, field, false)
}

// absentRecordTypeReference: RecordTypeId and RecordType.<field> on an object
// without a record type (dml.RecordTypeFieldAbsent) raise "No such column" and
// "Didn't understand relationship 'RecordType' in field path" in SELECT, WHERE
// and ORDER BY (record_type_visibility and record_type_shape rows).
func absentRecordTypeReference(org storage.OrgState, definition storage.ObjectDefinition, field string) bool {
	field = strings.TrimSpace(field)
	if len(field) < len("RecordTypeId") || !strings.EqualFold(field[:len("RecordType")], "RecordType") {
		return false
	}
	if !strings.EqualFold(field, "RecordTypeId") && field[len("RecordType")] != '.' {
		return false
	}
	return dml.RecordTypeFieldAbsent(&org, definition)
}

func absentRecordTypeRelationship(org storage.OrgState, definition storage.ObjectDefinition, field string) (string, bool) {
	relationship, _, ok := strings.Cut(strings.TrimSpace(field), ".")
	if !ok || !strings.EqualFold(relationship, "RecordType") || !absentRecordTypeReference(org, definition, field) {
		return "", false
	}
	return relationship, true
}

func fieldKnownForMode(org storage.OrgState, definition storage.ObjectDefinition, field string, requireAllParents bool) bool {
	if absentRecordTypeReference(org, definition, field) {
		return false
	}
	if strings.EqualFold(field, "Id") || isSystemField(field) {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(field), "QualifiedApiName") {
		return true
	}
	if storage.IsCustomMetadataDefinition(definition) && customMetadataSystemFieldKnown(field) {
		return true
	}
	if storage.IsCustomMetadataDefinition(definition) && customFieldLikeSOQLName(field) {
		return true
	}
	if customObjectLikeSOQLName(definition.APIName) && customFieldLikeSOQLName(field) {
		if len(definition.Fields) == 0 {
			return true
		}
		_, ok := storage.ResolveFieldName(definition, org.Namespace, field)
		return ok
	}
	if namespacedCustomFieldTolerated(org, field) {
		return true
	}
	if len(definition.Fields) == 0 && !strings.Contains(field, ".") {
		return true
	}
	if strings.Contains(field, ".") {
		parts := strings.SplitN(field, ".", 2)
		if strings.EqualFold(parts[1], "Id") && isSystemParentRelationship(parts[0]) {
			return true
		}
		for _, relation := range matchingParentRelations(org.Namespace, definition, parts[0]) {
			if relation.Polymorphic && strings.EqualFold(parts[1], "Type") {
				return true
			}
			if len(relation.ParentObjects) == 0 {
				return false
			}
			found := false
			for _, parentName := range relation.ParentObjects {
				if strings.EqualFold(parentName, "EntityDefinition") || strings.EqualFold(parentName, "FieldDefinition") {
					if entityDefinitionFieldKnown(parts[1]) {
						if !requireAllParents {
							return true
						}
						found = true
						continue
					}
					if requireAllParents {
						return false
					}
					continue
				}
				canonical, ok := storage.ResolveObjectName(org, parentName)
				if !ok {
					var orgWithStandard storage.OrgState
					orgWithStandard, canonical, ok = orgWithLazyStandardObject(org, parentName)
					if !ok {
						return false
					}
					org = orgWithStandard
				}
				if fieldKnownForMode(org, org.Objects[canonical].Definition, parts[1], requireAllParents) {
					if !requireAllParents {
						return true
					}
					found = true
					continue
				}
				if requireAllParents {
					return false
				}
			}
			return found
		}
		return false
	}
	_, ok := storage.ResolveFieldName(definition, org.Namespace, field)
	return ok
}

func customMetadataSystemFieldKnown(field string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "developername", "masterlabel", "label", "language", "namespaceprefix", "qualifiedapiname", "name":
		return true
	default:
		return false
	}
}

func entityDefinitionFieldKnown(field string) bool {
	if strings.Contains(field, ".") {
		parts := strings.SplitN(field, ".", 2)
		switch strings.ToLower(strings.TrimSpace(parts[0])) {
		case "runninguserfieldaccess", "runninguserentityaccess":
			return entityDefinitionFieldKnown(parts[1])
		}
	}
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "qualifiedapiname", "developername", "durableid", "label", "masterlabel",
		"datatype", "length", "namespaceprefix", "iskeyprefix", "keyprefix",
		"iscustomsetting", "islayoutable", "isqueryable", "istriggerable",
		"isdeprecatedandhidden", "iscreatable", "isdeletable", "isupdatable",
		"isaccessible", "iscreateable":
		return true
	default:
		return false
	}
}

func entityDefinitionRelationshipField(field string) storage.Field {
	name := field
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		name = parts[len(parts)-1]
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "isaccessible", "iscreatable", "iscreateable", "isdeletable", "isupdatable",
		"iscustomsetting", "islayoutable", "isqueryable", "istriggerable", "isdeprecatedandhidden":
		return storage.Field{APIName: name, Type: storage.FieldBoolean}
	case "length":
		return storage.Field{APIName: name, Type: storage.FieldInteger}
	default:
		return storage.Field{APIName: name, Type: storage.FieldString}
	}
}

func queryHasAggregates(query Query) bool {
	return len(query.Aggregates) > 0 || len(query.GroupBy) > 0 || query.Having != nil
}

// Non-COUNT aggregates charge their shared source rows. This covers non-null
// inputs with plain grouping; contributor selection for windows, HAVING,
// subtotal groups and null inputs retains the existing result-row accounting.
func aggregateQueryInputRows(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, query Query, userIDs ...storage.ID) *int {
	if len(records) == 0 || query.HasLimit || query.HasOffset || query.Offset != 0 || query.Having != nil || len(query.HavingAggregates) > 0 || query.GroupMode != "" {
		return nil
	}
	sourceRows := false
	for _, aggregate := range query.Aggregates {
		if strings.EqualFold(aggregate.Func, "COUNT") {
			continue
		}
		switch strings.ToUpper(aggregate.Func) {
		case "SUM", "AVG", "MIN", "MAX", "COUNT_DISTINCT":
		default:
			return nil
		}
		for _, record := range records {
			value, ok := recordValue(org, definition, record, aggregate.Field, userIDs...)
			if !ok || value.Kind == storage.ValueNull {
				return nil
			}
		}
		sourceRows = true
	}
	if !sourceRows {
		return nil
	}
	inputRows := len(records)
	return &inputRows
}

func aggregateOrderFieldKnown(query Query, field string) bool {
	if containsName(query.GroupBy, field) {
		return true
	}
	for _, selected := range query.Fields {
		if raw, alias, ok := splitSelectFieldAlias(selected); ok && containsName(query.GroupBy, raw) && strings.EqualFold(alias, field) {
			return true
		}
	}
	for i, aggregate := range query.Aggregates {
		if strings.EqualFold(field, fmt.Sprintf("expr%d", i)) || (aggregate.Alias != "" && strings.EqualFold(field, aggregate.Alias)) {
			return true
		}
	}
	for _, aggregate := range query.HavingAggregates {
		if aggregate.Alias != "" && strings.EqualFold(field, aggregate.Alias) {
			return true
		}
	}
	for _, aggregate := range query.orderAggregates {
		if strings.EqualFold(field, aggregate.Alias) {
			return true
		}
	}
	return false
}

func fieldUnavailableError(field, mode string) error {
	if mode != "" {
		return fmt.Errorf("soql: field %s is not available in %s mode", field, mode)
	}
	return fmt.Errorf("soql: unknown field %s", field)
}

func childRelationshipUnavailableError(relationship, mode string) error {
	message := MissingRelationshipMessage(relationship, "FROM part of query call")
	return queryError(message, message)
}

func typeofTargetUnavailableError(typeName, mode string) error {
	if mode != "" {
		return fmt.Errorf("soql: unknown TYPEOF target %s in %s mode", typeName, mode)
	}
	return fmt.Errorf("soql: unknown TYPEOF target %s", typeName)
}

func isSystemField(field string) bool {
	_, ok := canonicalSystemFieldName(field)
	return ok
}

func canonicalSystemFieldName(field string) (string, bool) {
	base := storage.StripAnyNamespaceToken(strings.TrimSpace(field))
	for _, candidate := range []string{"CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SystemModstamp", "OwnerId", "RecordTypeId", "IsDeleted"} {
		if strings.EqualFold(field, candidate) || strings.EqualFold(base, candidate) {
			return candidate, true
		}
	}
	return "", false
}

func isCustomFieldName(name string) bool {
	return hasSuffixFold(name, "__c")
}

func polymorphicParentObject(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, relationship string) (string, bool) {
	for _, relation := range matchingParentRelations(org.Namespace, definition, relationship) {
		parentID, ok := recordValue(org, definition, record, relation.Field)
		if !ok || parentID.Kind == storage.ValueNull {
			return "", false
		}
		id := idFromValue(parentID)
		for _, parentObjectName := range relation.ParentObjects {
			canonicalParent, ok := storage.ResolveObjectName(org, parentObjectName)
			if !ok {
				continue
			}
			parentObject := org.Objects[canonicalParent]
			if _, ok := parentObject.Records[id]; ok {
				return canonicalParent, true
			}
			if parentObject.Definition.KeyPrefix != "" && strings.HasPrefix(string(id), parentObject.Definition.KeyPrefix) {
				return canonicalParent, true
			}
		}
	}
	return "", false
}

func relationshipValue(org storage.OrgState, record storage.Record, field string, userIDs ...storage.ID) (storage.Value, bool) {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return storage.Value{}, false
	}
	objectName := record.Object
	if canonical, resolved := storage.ResolveObjectName(org, objectName); resolved {
		objectName = canonical
	}
	if automatedProcessProfileRelationshipIsNull(objectName, record, parts[0]) {
		return storage.NullValue(), true
	}
	object, ok := org.Objects[objectName]
	if !ok {
		return storage.Value{}, false
	}
	for _, relation := range matchingParentRelations(org.Namespace, object.Definition, parts[0]) {
		parentID, ok := recordValue(org, object.Definition, record, relation.Field, userIDs...)
		if !ok || parentID.Kind == storage.ValueNull {
			return storage.NullValue(), true
		}
		if strings.EqualFold(parts[1], "Id") {
			return parentID.Clone(), true
		}
		for _, parentObjectName := range relation.ParentObjects {
			if strings.EqualFold(parentObjectName, "EntityDefinition") || strings.EqualFold(parentObjectName, "FieldDefinition") {
				if strings.EqualFold(parts[1], "QualifiedApiName") {
					if value, ok := ownedMetadataRelationshipName(org, object.Definition, record, relation.Field, parentObjectName, parentID); ok {
						return value, true
					}
				}
				if value, ok := entityDefinitionRelationshipValue(parentObjectName, parentID, parts[1]); ok {
					return value, true
				}
				continue
			}
			canonicalParent, ok := storage.ResolveObjectName(org, parentObjectName)
			if !ok {
				continue
			}
			parentObject := org.Objects[canonicalParent]
			_, parent, ok := storage.LookupRecordByID(parentObject.Records, idFromValue(parentID))
			if !ok || parent.System.IsDeleted {
				continue
			}
			parent.Object = canonicalParent
			// Type on a polymorphic relationship identifies the target object;
			// it is not the target's ordinary field named Type.
			if relation.Polymorphic && strings.EqualFold(parts[1], "Type") {
				return storage.StringValue(canonicalParent), true
			}
			if strings.Contains(parts[1], ".") {
				return relationshipValue(org, parent, parts[1], userIDs...)
			}
			value, ok := recordValue(org, parentObject.Definition, parent, parts[1], userIDs...)
			if !ok {
				return storage.NullValue(), true
			}
			return value.Clone(), true
		}
	}
	return storage.Value{}, false
}

// Salesforce exposes the Automated Process user's ProfileId but its Profile
// relationship is null. Keep that special relationship behavior while
// retaining the profile id for direct field and permission checks.
func automatedProcessProfileRelationshipIsNull(objectName string, record storage.Record, relationship string) bool {
	if !strings.EqualFold(objectName, "User") || !strings.EqualFold(relationship, "Profile") {
		return false
	}
	value, ok := record.GetField("UserType")
	return ok && value.Kind == storage.ValueString && strings.EqualFold(strings.TrimSpace(value.String), "AutomatedProcess")
}

func lookupRecordByIDInOrg(org storage.OrgState, id storage.ID) (string, storage.ObjectState, storage.Record, bool) {
	if id == "" {
		return "", storage.ObjectState{}, storage.Record{}, false
	}
	for objectName, object := range org.Objects {
		_, record, ok := storage.LookupRecordByID(object.Records, id)
		if ok {
			return objectName, object, record, true
		}
	}
	return "", storage.ObjectState{}, storage.Record{}, false
}

func relationshipLookupMissing(org storage.OrgState, record storage.Record, field string) (string, bool) {
	parts := strings.Split(field, ".")
	if len(parts) < 2 {
		return "", false
	}
	for i := 1; i < len(parts); i++ {
		relationship := strings.Join(parts[:i], ".")
		if relationship == "" {
			continue
		}
		if !relationshipLookupPathKnown(org, record, relationship) {
			continue
		}
		value, ok := relationshipValue(org, record, relationship+".Id")
		if ok && value.Kind == storage.ValueNull {
			return relationship, true
		}
		if ok && !relationshipRecordExists(org, record, relationship) {
			return relationship, true
		}
	}
	return "", false
}

func relationshipRecordExists(org storage.OrgState, record storage.Record, relationshipPath string) bool {
	parts := strings.Split(relationshipPath, ".")
	current := record
	for _, relationship := range parts {
		if strings.TrimSpace(relationship) == "" {
			return false
		}
		if virtualParentRelationshipExists(org, current, relationship) {
			continue
		}
		_, parent, ok := parentRelationshipRecord(org, current, relationship)
		if !ok {
			return false
		}
		current = parent
	}
	return true
}

func virtualParentRelationshipExists(org storage.OrgState, record storage.Record, relationship string) bool {
	objectName := record.Object
	if canonical, resolved := storage.ResolveObjectName(org, objectName); resolved {
		objectName = canonical
	}
	object, ok := org.Objects[objectName]
	if !ok {
		return false
	}
	for _, relation := range matchingParentRelations(org.Namespace, object.Definition, relationship) {
		virtualParent := false
		for _, parentObjectName := range relation.ParentObjects {
			if strings.EqualFold(parentObjectName, "EntityDefinition") || strings.EqualFold(parentObjectName, "FieldDefinition") {
				virtualParent = true
				break
			}
		}
		if !virtualParent {
			continue
		}
		parentID, ok := recordValue(org, object.Definition, record, relation.Field)
		if !ok || parentID.Kind == storage.ValueNull {
			continue
		}
		switch parentID.Kind {
		case storage.ValueString:
			if strings.TrimSpace(parentID.String) != "" {
				return true
			}
		case storage.ValueID:
			if strings.TrimSpace(string(parentID.ID)) != "" {
				return true
			}
		}
	}
	return false
}

func relationshipLookupPathKnown(org storage.OrgState, record storage.Record, relationship string) bool {
	first, _, _ := strings.Cut(relationship, ".")
	objectName := record.Object
	if canonical, ok := storage.ResolveObjectName(org, objectName); ok {
		objectName = canonical
	}
	object, ok := org.Objects[objectName]
	if !ok {
		return false
	}
	if len(matchingParentRelations(org.Namespace, object.Definition, first)) > 0 {
		return true
	}
	return false
}

// ownedMetadataRelationshipName resolves the project-owned custom schema names
// used by metadata relationship projections. Other ownership and unresolved
// references retain the existing fallback.
func ownedMetadataRelationshipName(org storage.OrgState, source storage.ObjectDefinition, record storage.Record, relationField, target string, value storage.Value) (storage.Value, bool) {
	if org.Namespace == "" || !storage.IsCustomMetadataDefinition(source) {
		return storage.Value{}, false
	}
	owner, ok := record.Fields["NamespacePrefix"]
	if !ok || owner.Kind != storage.ValueString || !strings.EqualFold(owner.String, org.Namespace) {
		return storage.Value{}, false
	}
	var text string
	switch value.Kind {
	case storage.ValueString:
		text = value.String
	case storage.ValueID:
		text = string(value.ID)
	default:
		return storage.Value{}, false
	}
	text = strings.TrimSpace(text)
	if strings.EqualFold(target, "FieldDefinition") && !strings.Contains(text, ".") {
		// A field relationship with a controlling entity field stores the bare
		// field name; the controlling field's value names the owning object.
		if entity, ok := controllingMetadataEntity(org.Namespace, source, record, relationField); ok {
			text = entity + "." + text
		}
	}
	objectName, fieldName, hasField := strings.Cut(text, ".")
	if strings.EqualFold(target, "FieldDefinition") != hasField || !projectCustomSchemaName(org.Namespace, objectName) {
		return storage.Value{}, false
	}
	if hasField && !projectCustomSchemaName(org.Namespace, fieldName) {
		return storage.Value{}, false
	}
	canonical, ok := storage.ResolveObjectName(org, objectName)
	if !ok {
		return storage.Value{}, false
	}
	definition := org.Objects[canonical].Definition
	qualifiedObject := storage.NamespaceTokenName(org.Namespace, definition.APIName)
	if !strings.HasPrefix(strings.ToLower(qualifiedObject), strings.ToLower(org.Namespace)+"__") {
		return storage.Value{}, false
	}
	if !hasField {
		return storage.StringValue(qualifiedObject), true
	}
	_, field, ok := storage.ResolveFieldDefinition(definition, org.Namespace, fieldName)
	if !ok || !projectCustomSchemaName(org.Namespace, field.APIName) {
		return storage.Value{}, false
	}
	return storage.StringValue(storage.NamespaceTokenName(org.Namespace, field.APIName)), true
}

func controllingMetadataEntity(namespace string, source storage.ObjectDefinition, record storage.Record, relationField string) (string, bool) {
	_, field, ok := storage.ResolveFieldDefinition(source, namespace, relationField)
	if !ok {
		return "", false
	}
	controller := strings.TrimSpace(field.MetadataRelationshipControllingField)
	if dot := strings.LastIndexByte(controller, '.'); dot >= 0 {
		controller = controller[dot+1:]
	}
	if controller == "" {
		return "", false
	}
	controller = storage.StripNamespaceToken(namespace, controller)
	for key, value := range record.Fields {
		if strings.EqualFold(storage.StripNamespaceToken(namespace, key), controller) && value.Kind == storage.ValueString && strings.TrimSpace(value.String) != "" {
			return strings.TrimSpace(value.String), true
		}
	}
	return "", false
}

// Only simple custom object and field names with no explicit foreign namespace
// are eligible for the captured project-owned qualification rule.
func projectCustomSchemaName(namespace, name string) bool {
	if !strings.HasSuffix(strings.ToLower(name), "__c") {
		return false
	}
	unqualified := storage.StripAnyNamespaceToken(name)
	return unqualified == name || strings.EqualFold(storage.StripNamespaceToken(namespace, name), unqualified)
}

func entityDefinitionRelationshipValue(objectName string, value storage.Value, field string) (storage.Value, bool) {
	text := ""
	switch value.Kind {
	case storage.ValueString:
		text = value.String
	case storage.ValueID:
		text = string(value.ID)
	default:
		return storage.Value{}, false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return storage.NullValue(), true
	}
	// A field relationship can store Object.Field; its qualified API name
	// identifies the field alone, while its durable identifier retains both.
	if strings.EqualFold(objectName, "FieldDefinition") && strings.EqualFold(field, "QualifiedApiName") {
		if dot := strings.LastIndexByte(text, '.'); dot >= 0 {
			text = text[dot+1:]
		}
	}
	switch {
	case strings.Contains(field, "."):
		parts := strings.SplitN(field, ".", 2)
		switch strings.ToLower(strings.TrimSpace(parts[0])) {
		case "runninguserfieldaccess", "runninguserentityaccess":
			return entityDefinitionRelationshipValue(objectName, value, parts[1])
		}
	case strings.EqualFold(field, "QualifiedApiName"), strings.EqualFold(field, "DurableId"):
		return storage.StringValue(text), true
	case strings.EqualFold(field, "DeveloperName"):
		return storage.StringValue(strings.TrimSuffix(text, "__c")), true
	case strings.EqualFold(field, "Label"), strings.EqualFold(field, "MasterLabel"):
		return storage.StringValue(strings.TrimSuffix(text, "__c")), true
	case strings.EqualFold(field, "DataType"):
		return storage.StringValue("STRING"), true
	case strings.EqualFold(field, "Length"):
		return storage.IntegerValue(255), true
	case strings.EqualFold(field, "NamespacePrefix"), strings.EqualFold(field, "KeyPrefix"):
		return storage.NullValue(), true
	case hasPrefixFold(field, "is"):
		switch strings.ToLower(field) {
		case "isdeprecatedandhidden", "iscustomsetting":
			return storage.BooleanValue(false), true
		default:
			return storage.BooleanValue(true), true
		}
	}
	return storage.Value{}, false
}

func projectRelationshipIDs(org storage.OrgState, record storage.Record, field string, fields map[string]storage.Value) {
	parts := strings.Split(field, ".")
	if len(parts) < 2 {
		return
	}
	currentRecord := record
	currentPath := ""
	for _, relationship := range parts[:len(parts)-1] {
		parentID, parent, ok := parentRelationshipRecord(org, currentRecord, relationship)
		if !ok {
			return
		}
		if currentPath == "" {
			currentPath = relationship
		} else {
			currentPath += "." + relationship
		}
		fields[currentPath+".Id"] = parentID.Clone()
		currentRecord = parent
	}
}

func parentRelationshipRecord(org storage.OrgState, record storage.Record, relationship string) (storage.Value, storage.Record, bool) {
	objectName := record.Object
	if canonical, resolved := storage.ResolveObjectName(org, objectName); resolved {
		objectName = canonical
	}
	object, ok := org.Objects[objectName]
	if !ok {
		return storage.Value{}, storage.Record{}, false
	}
	for _, relation := range matchingParentRelations(org.Namespace, object.Definition, relationship) {
		parentID, ok := recordValue(org, object.Definition, record, relation.Field)
		if !ok || parentID.Kind == storage.ValueNull {
			return storage.Value{}, storage.Record{}, false
		}
		for _, parentObjectName := range relation.ParentObjects {
			canonicalParent, ok := storage.ResolveObjectName(org, parentObjectName)
			if !ok {
				continue
			}
			parentObject := org.Objects[canonicalParent]
			_, parent, ok := storage.LookupRecordByID(parentObject.Records, idFromValue(parentID))
			if !ok || parent.System.IsDeleted {
				continue
			}
			parent.Object = canonicalParent
			return parentID, parent, true
		}
	}
	return storage.Value{}, storage.Record{}, false
}

func relationshipNameMatches(namespace, canonical, candidate string) bool {
	if canonical == candidate || strings.EqualFold(canonical, candidate) {
		return true
	}
	strippedCanonical := canonical
	stripped := candidate
	if namespace != "" {
		strippedCanonical = storage.StripNamespaceToken(namespace, canonical)
		stripped = storage.StripNamespaceToken(namespace, candidate)
	}
	anyCanonical := storage.StripAnyNamespaceToken(canonical)
	anyCandidate := storage.StripAnyNamespaceToken(candidate)
	return anyCanonical == anyCandidate ||
		strings.EqualFold(anyCanonical, anyCandidate) ||
		canonical == stripped ||
		strings.EqualFold(canonical, stripped) ||
		strippedCanonical == candidate ||
		strings.EqualFold(strippedCanonical, candidate) ||
		strippedCanonical == stripped ||
		strings.EqualFold(strippedCanonical, stripped)
}

func parentRelationshipNameMatches(namespace string, relation storage.Relationship, candidate string) bool {
	if relationshipNameMatches(namespace, relation.ParentRelationship, candidate) {
		return true
	}
	field := strings.TrimSpace(relation.Field)
	if field == "" {
		return false
	}
	switch {
	case strings.HasSuffix(field, "__c"):
		return relationshipNameMatches(namespace, strings.TrimSuffix(field, "__c")+"__r", candidate)
	case strings.HasSuffix(field, "Id") && len(field) > len("Id"):
		return relationshipNameMatches(namespace, strings.TrimSuffix(field, "Id"), candidate)
	default:
		return false
	}
}

func matchingParentRelations(namespace string, definition storage.ObjectDefinition, candidate string) []storage.Relationship {
	var matches []storage.Relationship
	for _, relation := range definition.Relations {
		if parentRelationshipNameMatches(namespace, relation, candidate) {
			matches = append(matches, relation)
		}
	}
	if len(matches) > 0 {
		return matches
	}
	for name, field := range definition.Fields {
		apiName := field.APIName
		if apiName == "" {
			apiName = name
		}
		if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		parentRelationship := field.RelationshipName
		if parentRelationship == "" {
			parentRelationship = derivedParentRelationshipName(apiName)
		}
		relation := storage.Relationship{
			Field:              apiName,
			ParentObjects:      append([]string(nil), field.ReferenceTo...),
			ParentRelationship: parentRelationship,
			Polymorphic:        len(field.ReferenceTo) > 1,
		}
		if parentRelationshipNameMatches(namespace, relation, candidate) {
			matches = append(matches, relation)
		}
	}
	if len(matches) > 0 {
		return matches
	}
	if relation, ok := systemParentRelationship(definition, candidate); ok {
		return []storage.Relationship{relation}
	}
	return nil
}

func derivedParentRelationshipName(fieldName string) string {
	fieldName = strings.TrimSpace(fieldName)
	if strings.HasSuffix(fieldName, "__c") {
		return strings.TrimSuffix(fieldName, "__c") + "__r"
	}
	if strings.HasSuffix(fieldName, "Id") && len(fieldName) > len("Id") {
		return strings.TrimSuffix(fieldName, "Id")
	}
	return ""
}

func systemParentRelationship(definition storage.ObjectDefinition, candidate string) (storage.Relationship, bool) {
	if storage.IsCustomSettingDefinition(definition) && strings.EqualFold(candidate, "SetupOwner") {
		return storage.Relationship{
			Field:              "SetupOwnerId",
			ParentObjects:      []string{"Organization", "Profile", "User"},
			ParentRelationship: "SetupOwner",
			Polymorphic:        true,
		}, true
	}
	if strings.EqualFold(definition.APIName, "RelationshipDomain") {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case "childsobject":
			return storage.Relationship{Field: "ChildSobjectId", ParentObjects: []string{"EntityDefinition"}, ParentRelationship: "ChildSobject"}, true
		case "field":
			return storage.Relationship{Field: "FieldId", ParentObjects: []string{"FieldDefinition"}, ParentRelationship: "Field"}, true
		}
	}
	fieldName := candidate + "Id"
	field, ok := fieldDefinitionsForReferenceField(definition, fieldName)
	if !ok || field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
		return storage.Relationship{}, false
	}
	return storage.Relationship{Field: field.APIName, ParentObjects: append([]string(nil), field.ReferenceTo...), ParentRelationship: candidate, Polymorphic: len(field.ReferenceTo) > 1}, true
}

func fieldDefinitionsForReferenceField(definition storage.ObjectDefinition, fieldName string) (storage.Field, bool) {
	if field, ok := definition.Fields[fieldName]; ok {
		return field, true
	}
	for candidate, field := range definition.Fields {
		if strings.EqualFold(candidate, fieldName) || strings.EqualFold(field.APIName, fieldName) {
			return field, true
		}
	}
	if canonical, ok := canonicalSystemFieldName(fieldName); ok {
		switch canonical {
		case "CreatedById", "LastModifiedById", "OwnerId":
			return storage.Field{APIName: canonical, Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: strings.TrimSuffix(canonical, "Id")}, true
		case "RecordTypeId":
			return storage.Field{APIName: canonical, Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}, true
		}
	}
	return storage.Field{}, false
}

func idFromValue(value storage.Value) storage.ID {
	if value.Kind == storage.ValueID {
		return value.ID
	}
	if value.Kind == storage.ValueString {
		return storage.ID(value.String)
	}
	return ""
}

func recordValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, field string, userIDs ...storage.ID) (storage.Value, bool) {
	if expr, ok := parseSelectFieldExpression(field); ok {
		return selectFieldExpressionValue(org, definition, record, expr, userIDs...)
	}
	if strings.Contains(field, ".") {
		if base, ok := stripQualifiedCurrentObjectField(org, definition, field); ok {
			field = base
		} else {
			return relationshipValue(org, record, field, userIDs...)
		}
	}
	if canonical, ok := resolveSOQLFieldName(definition, org.Namespace, field); ok && canonical == "Id" {
		return storage.IDValue(record.ID), true
	}
	if strings.Contains(field, ".") {
		return relationshipValue(org, record, field, userIDs...)
	}
	if canonical, ok := canonicalSystemFieldName(field); ok {
		field = canonical
	}
	if strings.EqualFold(record.Object, "Organization") && strings.EqualFold(field, "TimeZoneSidKey") {
		return storage.StringValue("UTC"), true
	}
	switch field {
	case "CreatedDate":
		if record.System.CreatedDate != "" {
			return storage.DateTimeValue(record.System.CreatedDate), true
		}
		if value, ok := record.GetField(field); ok {
			return value, true
		}
		return storage.NullValue(), true
	case "CreatedById":
		if record.System.CreatedByID != "" {
			return storage.IDValue(record.System.CreatedByID), true
		}
		if value, ok := record.GetField(field); ok {
			return value, true
		}
		return storage.NullValue(), true
	case "LastModifiedDate":
		if record.System.LastModifiedDate != "" {
			return storage.DateTimeValue(record.System.LastModifiedDate), true
		}
		if value, ok := record.GetField(field); ok {
			return value, true
		}
		return storage.NullValue(), true
	case "LastModifiedById":
		if record.System.LastModifiedByID != "" {
			return storage.IDValue(record.System.LastModifiedByID), true
		}
		if value, ok := record.GetField(field); ok {
			return value, true
		}
		return storage.NullValue(), true
	case "SystemModstamp":
		if record.System.SystemModstamp != "" {
			return storage.DateTimeValue(record.System.SystemModstamp), true
		}
		if value, ok := record.GetField(field); ok {
			return value, true
		}
		return storage.NullValue(), true
	case "OwnerId":
		if record.System.OwnerID != "" {
			return storage.IDValue(record.System.OwnerID), true
		}
		if value, ok := record.GetField(field); ok {
			return value, true
		}
		return storage.NullValue(), true
	case "IsDeleted":
		return storage.BooleanValue(record.System.IsDeleted), true
	}
	canonicalField, ok := resolveSOQLFieldName(definition, org.Namespace, field)
	if !ok {
		canonicalField = field
	}
	if record.HasExplicitNull(canonicalField) {
		return storage.NullValue(), true
	}
	if value, ok := calculatedFieldValue(org, definition, record, canonicalField, userIDs...); ok {
		return value, true
	}
	value, ok := recordFieldValue(org, record, canonicalField)
	if ok && value.Kind == storage.ValueString {
		if fieldDef, known := fieldDefinitionsForReferenceField(definition, canonicalField); known &&
			(fieldDef.Type == storage.FieldID || fieldDef.Type == storage.FieldReference) {
			// Imported/defaulted reference fields may have string-backed IDs.
			// Their schema, rather than their storage representation, determines
			// Id comparison semantics.
			value = idFieldValue(value)
		}
	}
	if ok && strings.EqualFold(canonicalField, "NamespacePrefix") && value.Kind == storage.ValueString && strings.TrimSpace(value.String) == "" {
		return storage.NullValue(), true
	}
	if !ok && strings.EqualFold(definition.APIName, "Contact") && strings.EqualFold(canonicalField, "Name") {
		return contactNameValue(record)
	}
	if !ok && strings.EqualFold(canonicalField, "QualifiedApiName") {
		return storage.NullValue(), true
	}
	if !ok && customObjectLikeSOQLName(definition.APIName) && customFieldLikeSOQLName(canonicalField) {
		return storage.NullValue(), true
	}
	if !ok && namespacedCustomFieldTolerated(org, canonicalField) {
		return storage.NullValue(), true
	}
	if !ok && storage.IsCustomMetadataDefinition(definition) {
		if value, customMetadataOK := customMetadataSystemValue(definition, record, canonicalField); customMetadataOK {
			return value, true
		}
	}
	if !ok {
		if fieldDef, fieldOK := definition.Fields[canonicalField]; fieldOK && strings.TrimSpace(fieldDef.Formula) != "" {
			if value, formulaOK := calculatedRecordValue(org, definition, record, fieldDef, userIDs...); formulaOK {
				return value, true
			}
		}
	}
	return value, ok
}

func idFieldValue(value storage.Value) storage.Value {
	if value.Kind == storage.ValueString && value.String != "" {
		return storage.IDValue(storage.ID(value.String))
	}
	return value
}

func customObjectLikeSOQLName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, "__c") || strings.HasSuffix(lower, "__e") || strings.HasSuffix(lower, "__mdt")
}

func customFieldLikeSOQLName(name string) bool {
	if strings.Contains(name, ".") {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, "__c") || strings.HasSuffix(lower, "__pc")
}

// namespacedCustomFieldTolerated accepts an undefined ns__X__c name on a
// standard object. Native rows (namespaced_standard_field_soql) accept it only
// when that managed package is installed and has the field. Glade knows
// installed packages from the project's declared dependencies: when the
// project declares none, or one with no namespace binding, every such name
// stays accepted; otherwise only a declared namespace whose source is absent.
func namespacedCustomFieldTolerated(org storage.OrgState, name string) bool {
	if !namespacedCustomFieldLikeSOQLName(name) {
		return false
	}
	namespace := strings.TrimSpace(name)[:strings.Index(strings.TrimSpace(name), "__")]
	return storage.PackageFieldNamespaceAllowed(org.Metadata.PackageFieldsNarrow, org.Metadata.SourcelessPackageNamespaces, namespace)
}

func namespacedCustomFieldLikeSOQLName(name string) bool {
	name = strings.TrimSpace(name)
	if strings.Contains(name, ".") {
		return false
	}
	lower := strings.ToLower(name)
	return strings.Count(lower, "__") >= 2 && (strings.HasSuffix(lower, "__c") || strings.HasSuffix(lower, "__pc"))
}

func customMetadataSystemValue(definition storage.ObjectDefinition, record storage.Record, field string) (storage.Value, bool) {
	if value, ok := record.GetField(field); ok {
		return value, true
	}
	developerName := firstSOQLString(record.Fields, "DeveloperName", "Name")
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "developername":
		if developerName != "" {
			return storage.StringValue(developerName), true
		}
	case "qualifiedapiname":
		if value, ok := record.GetField("QualifiedApiName"); ok {
			return value, true
		}
		if developerName != "" {
			return storage.StringValue(developerName), true
		}
	case "masterlabel", "label", "name":
		if label := firstSOQLString(record.Fields, "MasterLabel", "Label", "Name", "DeveloperName"); label != "" {
			return storage.StringValue(label), true
		}
	case "language", "namespaceprefix":
		return storage.NullValue(), true
	}
	return storage.Value{}, false
}

func firstSOQLString(fields map[string]storage.Value, names ...string) string {
	for _, name := range names {
		value, ok := fields[name]
		if !ok || value.Kind != storage.ValueString {
			continue
		}
		if text := strings.TrimSpace(value.String); text != "" {
			return text
		}
	}
	return ""
}

func resolveSOQLFieldName(definition storage.ObjectDefinition, namespace, field string) (string, bool) {
	return storage.ResolveFieldName(definition, namespace, field)
}

func stripQualifiedCurrentObjectField(org storage.OrgState, definition storage.ObjectDefinition, field string) (string, bool) {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	objectToken := strings.TrimSpace(parts[0])
	fieldToken := strings.TrimSpace(parts[1])
	if objectToken == "" || fieldToken == "" {
		return "", false
	}
	canonicalObject := definition.APIName
	if canonicalObject == "" {
		return "", false
	}
	if strings.EqualFold(objectToken, canonicalObject) {
		return fieldToken, true
	}
	if strings.EqualFold(storage.StripAnyNamespaceToken(objectToken), storage.StripAnyNamespaceToken(canonicalObject)) {
		return fieldToken, true
	}
	if org.Namespace != "" {
		localCanonical := storage.StripNamespaceToken(org.Namespace, canonicalObject)
		localObject := storage.StripNamespaceToken(org.Namespace, objectToken)
		if strings.EqualFold(localObject, localCanonical) {
			return fieldToken, true
		}
	}
	return "", false
}

func recordFieldValue(org storage.OrgState, record storage.Record, field string) (storage.Value, bool) {
	if value, ok := record.GetField(field); ok {
		return value, true
	}
	if stripped := storage.StripNamespaceToken(org.Namespace, field); stripped != field {
		if value, ok := record.GetField(stripped); ok {
			return value, true
		}
	}
	if prefixed := storage.NamespaceTokenName(org.Namespace, field); prefixed != field {
		if value, ok := record.GetField(prefixed); ok {
			return value, true
		}
	}
	if unqualified := storage.StripAnyNamespaceToken(field); unqualified != field {
		if value, ok := record.GetField(unqualified); ok {
			return value, true
		}
	}
	return storage.Value{}, false
}

func calculatedFieldValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, field string, userIDs ...storage.ID) (storage.Value, bool) {
	if canonical, ok := storage.ResolveFieldName(definition, org.Namespace, field); ok {
		field = canonical
	}
	fieldDef, ok := definition.Fields[field]
	if !ok {
		return storage.Value{}, false
	}
	if strings.TrimSpace(fieldDef.Formula) == "" {
		if fieldDef.Type == storage.FieldSummary {
			return calculatedSummaryFieldValue(org, definition, record, fieldDef)
		}
		return storage.Value{}, false
	}
	return calculatedRecordValue(org, definition, record, fieldDef, userIDs...)
}

func calculatedSummaryFieldValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, field storage.Field) (storage.Value, bool) {
	return dml.EvaluateRecordSummaryValueInOrg(field, &org, definition, record)
}

func calculatedRecordValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, field storage.Field, userIDs ...storage.ID) (storage.Value, bool) {
	options := dml.FormulaEvaluationOptions{}
	if len(userIDs) != 0 {
		options.SetupUserID = userIDs[0]
	}
	if value, _, ok := dml.EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, &org, definition, record, options); ok {
		return value, true
	}
	return storage.Value{}, false
}

func contactNameValue(record storage.Record) (storage.Value, bool) {
	first, hasFirst := record.GetField("FirstName")
	last, hasLast := record.GetField("LastName")
	parts := make([]string, 0, 2)
	if hasFirst && first.Kind == storage.ValueString && strings.TrimSpace(first.String) != "" {
		parts = append(parts, strings.TrimSpace(first.String))
	}
	if hasLast && last.Kind == storage.ValueString && strings.TrimSpace(last.String) != "" {
		parts = append(parts, strings.TrimSpace(last.String))
	}
	if len(parts) == 0 {
		return storage.Value{}, false
	}
	return storage.StringValue(strings.Join(parts, " ")), true
}

func equalValues(left, right storage.Value) bool {
	if (left.Kind == storage.ValueNull && right.Kind == storage.ValueString && right.String == "") ||
		(right.Kind == storage.ValueNull && left.Kind == storage.ValueString && left.String == "") {
		return true
	}
	if leftNumber, rightNumber, ok := numericValues(left, right); ok {
		return leftNumber.Cmp(rightNumber) == 0
	}
	if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
		return idStringEqual(string(left.ID), right.String)
	}
	if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
		// Text fields compare the complete 18-character typed Id bind. The
		// checksum distinguishes IDs whose case-sensitive portions differ
		// only by case; dropping it would merge those distinct identities.
		return strings.EqualFold(left.String, string(right.ID))
	}
	if left.Kind == storage.ValueID && right.Kind == storage.ValueInteger {
		return idEqualsInteger(left.ID, right.Integer)
	}
	if left.Kind == storage.ValueInteger && right.Kind == storage.ValueID {
		return idEqualsInteger(right.ID, left.Integer)
	}
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString:
		// SOQL text equality is case-insensitive. Apex VM String == remains
		// case-sensitive in internal/vm/value.go.
		return strings.EqualFold(left.String, right.String)
	case storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return left.String == right.String
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueDecimal:
		return left.Decimal == right.Decimal
	case storage.ValueID:
		return idTextEqual(string(left.ID), string(right.ID))
	default:
		return false
	}
}

// Text predicates compare complete text, even when both strings resemble Ids.
// Resolve the declared field type so Id/reference predicates retain their
// existing 15/18-character comparison, including relationship fields.
func equalFieldValuesInOrg(org storage.OrgState, definition storage.ObjectDefinition, field string, left, right storage.Value, locale string) bool {
	return equalFieldValuesInOrgWithScratch(org, definition, field, left, right, locale, nil)
}

func equalFieldValuesInOrgWithScratch(org storage.OrgState, definition storage.ObjectDefinition, field string, left, right storage.Value, locale string, scratch *predicateScratch) bool {
	// Custom metadata record identities compare their full qualified name;
	// the current namespace must not make an unqualified name an alias.
	if storage.IsCustomMetadataDefinition(definition) && strings.EqualFold(field, "QualifiedApiName") {
		return equalValues(left, right)
	}
	// Captured namespaced org: ProcessDefinition.DeveloperName is stored
	// without the namespace and the prefixed form matches no row.
	if strings.EqualFold(definition.APIName, "ProcessDefinition") && strings.EqualFold(field, "DeveloperName") {
		return equalValues(left, right)
	}
	if left.Kind == storage.ValueString && right.Kind == storage.ValueString {
		kind := scratch.textEqualityKind(org, definition, field)
		if kind.text && kind.exact {
			// Native U6: equality on a case-sensitive unique text field is exact.
			return left.String == right.String
		}
		if kind.text && turkishTextLocale(locale) {
			// Native U5 tr_TR: text equality folds by the user's Turkish
			// casing, so I pairs with a dotless i and a dotted capital I with i.
			return foldTurkish(left.String) == foldTurkish(right.String) || equalNamespacedTextValues(org, left, right)
		}
		if kind.text {
			return strings.EqualFold(left.String, right.String) || equalNamespacedTextValues(org, left, right)
		}
	}
	return equalValuesInOrg(org, left, right)
}

func equalValuesInOrg(org storage.OrgState, left, right storage.Value) bool {
	return equalValues(left, right) || equalNamespacedTextValues(org, left, right)
}

func equalNamespacedTextValues(org storage.OrgState, left, right storage.Value) bool {
	if org.Namespace == "" || left.Kind != storage.ValueString || right.Kind != storage.ValueString {
		return false
	}
	leftText := strings.TrimSpace(left.String)
	rightText := strings.TrimSpace(right.String)
	if leftText == "" || rightText == "" {
		return false
	}
	if !strings.Contains(leftText, "__") && !strings.Contains(rightText, "__") {
		return false
	}
	return strings.EqualFold(storage.StripNamespaceToken(org.Namespace, leftText), storage.StripNamespaceToken(org.Namespace, rightText))
}

func idTextEqual(left, right string) bool {
	// The 15-character portion of a Salesforce Id is case-sensitive. The
	// 18-character form is a case-safe representation of that same value, so
	// compare the canonical 15-character portions exactly when widths differ.
	// Do not use EqualFold here: custom-object key prefixes can differ only by
	// case, and those are distinct Salesforce objects.
	if left == right {
		return true
	}
	if len(left) == 15 && len(right) == 18 {
		return left == right[:15]
	}
	if len(left) == 18 && len(right) == 15 {
		return left[:15] == right
	}
	return false
}

func idStringEqual(left, right string) bool {
	if idTextEqual(left, right) {
		return true
	}
	if len(left) == 15 && len(right) == 18 {
		return strings.EqualFold(left, right[:15])
	}
	if len(left) == 18 && len(right) == 15 {
		return strings.EqualFold(left[:15], right)
	}
	return strings.EqualFold(left, right)
}

func compareIDText(left, right string) int {
	if idTextEqual(left, right) {
		return 0
	}
	return strings.Compare(left, right)
}

func idEqualsInteger(id storage.ID, value int64) bool {
	if value < 0 {
		return false
	}
	text := strconv.FormatInt(value, 10)
	if len(text) > len(id) {
		return false
	}
	return strings.Repeat("0", len(id)-len(text))+text == string(id)
}

func compareValues(left, right storage.Value) int {
	return compareValuesCollated(left, right, false)
}

// compareValuesCollated is compareValues with the captured en_* text collation
// when collate is set; see compareSOQLTextCollated.
func compareValuesCollated(left, right storage.Value, collate bool) int {
	if left.Kind == storage.ValueNull || right.Kind == storage.ValueNull {
		if left.Kind == right.Kind {
			return 0
		}
		if left.Kind == storage.ValueNull {
			return -1
		}
		return 1
	}
	if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
		return compareIDText(string(left.ID), right.String)
	}
	if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
		return strings.Compare(left.String, string(right.ID))
	}
	if leftNumber, rightNumber, ok := numericValues(left, right); ok {
		return leftNumber.Cmp(rightNumber)
	}
	if isTemporalKind(left.Kind) && isTemporalKind(right.Kind) {
		return strings.Compare(temporalCompareString(left), temporalCompareString(right))
	}
	if left.Kind != right.Kind {
		return strings.Compare(string(left.Kind), string(right.Kind))
	}
	switch left.Kind {
	case storage.ValueString:
		// SOQL ORDER BY uses folded text ordering with the original text as a stable
		// tie-breaker; this intentionally differs from Apex String comparisons.
		if collate {
			return compareSOQLTextCollated(left.String, right.String)
		}
		return compareSOQLText(left.String, right.String)
	case storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return strings.Compare(left.String, right.String)
	case storage.ValueInteger:
		if left.Integer < right.Integer {
			return -1
		}
		if left.Integer > right.Integer {
			return 1
		}
		return 0
	case storage.ValueBoolean:
		if !left.Boolean && right.Boolean {
			return -1
		}
		if left.Boolean && !right.Boolean {
			return 1
		}
		return 0
	case storage.ValueDecimal:
		return strings.Compare(left.Decimal, right.Decimal)
	case storage.ValueID:
		return compareIDText(string(left.ID), string(right.ID))
	default:
		return 0
	}
}

// compareSOQLText orders text case-insensitively. Strings that differ only in
// case tie, so a following ORDER BY key (or record Id) decides, as native
// Q-gapsoql ORDER BY Name,Id rows show; the unkeyed tie order is unspecified.
func compareSOQLText(left, right string) int {
	return compareSOQLTextFolded(left, right)
}

func compareSOQLTextFolded(left, right string) int {
	for len(left) > 0 && len(right) > 0 {
		leftRune, leftSize := utf8.DecodeRuneInString(left)
		rightRune, rightSize := utf8.DecodeRuneInString(right)
		leftFolded := unicode.ToLower(leftRune)
		rightFolded := unicode.ToLower(rightRune)
		if leftFolded < rightFolded {
			return -1
		}
		if leftFolded > rightFolded {
			return 1
		}
		left = left[leftSize:]
		right = right[rightSize:]
	}
	if len(left) == len(right) {
		return 0
	}
	if len(left) == 0 {
		return -1
	}
	return 1
}

func sortRecordsByOrder(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, order []OrderSpec, legacyPicklistNullOrder, legacyRestrictedPicklistNullOrder bool, userIDs ...storage.ID) {
	// picklist_order_native.json observes setup order and one unlisted value
	// after configured values. Keep unobserved incomplete shapes on their
	// existing whole-column fallback so comparisons retain a total order.
	picklistRanks := make([]map[string]int, len(order))
	legacyNulls := make([]bool, len(order))
	for i, spec := range order {
		fields, ok := fieldDefinitionsForReference(org, definition, spec.Field)
		if !ok || len(fields) != 1 || fields[0].Type != storage.FieldPicklist || len(fields[0].PicklistValues) == 0 {
			continue
		}
		ranks := make(map[string]int, len(fields[0].PicklistValues))
		for position, option := range fields[0].PicklistValues {
			ranks[option.Value] = position
		}
		unknown, hasUnknown := "", false
		for _, record := range records {
			value, ok := recordValue(org, definition, record, spec.Field, userIDs...)
			if !ok || value.Kind == storage.ValueNull {
				continue
			}
			if value.Kind != storage.ValueString {
				ranks = nil
				break
			}
			if _, known := ranks[value.String]; !known {
				if fields[0].RestrictedPicklist || value.String == "" || (hasUnknown && unknown != value.String) {
					ranks = nil
					break
				}
				unknown, hasUnknown = value.String, true
			}
		}
		if ranks != nil && hasUnknown {
			ranks[unknown] = len(fields[0].PicklistValues)
		}
		picklistRanks[i] = ranks
		// Restricted picklists follow the legacy policy only where captured
		// (source API 62.0), when every stored value has configured rank metadata.
		legacyNulls[i] = ranks != nil && legacyPicklistNullOrder && (!fields[0].RestrictedPicklist || legacyRestrictedPicklistNullOrder)
	}
	nullsIgnored := make([]bool, len(order))
	for i, spec := range order {
		field := spec.Field
		if prefix := definition.APIName + "."; len(field) > len(prefix) && strings.EqualFold(field[:len(prefix)], prefix) {
			field = field[len(prefix):]
		}
		nullsIgnored[i] = !strings.Contains(field, ".") && StandardLookupIgnoresNullsClause(definition.APIName, field)
	}
	sort.SliceStable(records, func(i, j int) bool {
		for column, spec := range order {
			left, ok := recordValue(org, definition, records[i], spec.Field, userIDs...)
			if !ok {
				left = storage.NullValue()
			}
			right, ok := recordValue(org, definition, records[j], spec.Field, userIDs...)
			if !ok {
				right = storage.NullValue()
			}
			if ranks := picklistRanks[column]; ranks != nil && left.Kind != storage.ValueNull && right.Kind != storage.ValueNull {
				left = storage.IntegerValue(int64(ranks[left.String]))
				right = storage.IntegerValue(int64(ranks[right.String]))
			}
			if legacyNulls[column] || nullsIgnored[column] {
				spec.Nulls = "FIRST"
				if spec.Desc {
					spec.Nulls = "LAST"
				}
			}
			cmp := orderCompare(left, right, spec)
			if cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})
}

func aggregateOrderedBefore(leftRecord, rightRecord storage.Record, order []OrderSpec) bool {
	for _, spec := range order {
		left := aggregateRecordValue(leftRecord, spec.Field)
		right := aggregateRecordValue(rightRecord, spec.Field)
		cmp := orderCompare(left, right, spec)
		if cmp == 0 {
			continue
		}
		return cmp < 0
	}
	return false
}

func orderCompare(left, right storage.Value, spec OrderSpec) int {
	if left.Kind == storage.ValueNull || right.Kind == storage.ValueNull {
		if left.Kind == right.Kind {
			return 0
		}
		if spec.Nulls == "" || spec.Nulls == "FIRST" {
			if left.Kind == storage.ValueNull {
				return -1
			}
			return 1
		}
		if left.Kind == storage.ValueNull {
			return 1
		}
		return -1
	}
	cmp := compareValuesCollated(left, right, spec.textCollation)
	if spec.Desc {
		return -cmp
	}
	return cmp
}

func isTemporalKind(kind storage.ValueKind) bool {
	return kind == storage.ValueDate || kind == storage.ValueDateTime
}

func temporalCompareString(value storage.Value) string {
	if value.Kind == storage.ValueDate {
		return value.String + "T00:00:00Z"
	}
	return value.String
}

func numericValues(left, right storage.Value) (*big.Rat, *big.Rat, bool) {
	leftNumber, ok := numericValue(left)
	if !ok {
		return nil, nil, false
	}
	rightNumber, ok := numericValue(right)
	if !ok {
		return nil, nil, false
	}
	return leftNumber, rightNumber, true
}

func numericValue(value storage.Value) (*big.Rat, bool) {
	switch value.Kind {
	case storage.ValueInteger:
		return new(big.Rat).SetInt64(value.Integer), true
	case storage.ValueDecimal:
		parsed, ok := new(big.Rat).SetString(value.Decimal)
		return parsed, ok
	default:
		return nil, false
	}
}

func decimalString(value *big.Rat) string {
	if value.IsInt() {
		return value.Num().String()
	}
	return strings.TrimRight(strings.TrimRight(value.FloatString(10), "0"), ".")
}

func valueKey(value storage.Value) string {
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return string(value.Kind) + ":" + value.String
	case storage.ValueDecimal:
		if rat, ok := new(big.Rat).SetString(value.Decimal); ok {
			return string(value.Kind) + ":" + rat.RatString()
		}
		return string(value.Kind) + ":" + value.Decimal
	case storage.ValueInteger:
		return string(value.Kind) + ":" + strconv.FormatInt(value.Integer, 10)
	case storage.ValueBoolean:
		return string(value.Kind) + ":" + strconv.FormatBool(value.Boolean)
	case storage.ValueID:
		return string(value.Kind) + ":" + string(value.ID)
	default:
		return string(value.Kind)
	}
}

func likeMatch(left, right storage.Value) bool { return likeMatchWithScratch(left, right, nil) }

func likeMatchWithScratch(left, right storage.Value, scratch *predicateScratch) bool {
	if right.Kind == storage.ValueID {
		// Typed Id binds carry their complete 18-character text into LIKE.
		right = storage.StringValue(string(right.ID))
	}
	if left.Kind != storage.ValueString || right.Kind != storage.ValueString {
		return false
	}
	if scratch == nil || len(right.String) > 4096 {
		return matchLikePattern(left.String, right.String)
	}
	if scratch.patterns == nil {
		scratch.patterns = make(map[string]*compiledLikePattern)
	}
	pattern, found := scratch.patterns[right.String]
	if !found {
		pattern = compileLikePattern(right.String)
		clear(scratch.patterns)
		scratch.patterns[right.String] = pattern
	}
	return pattern.match(left.String)
}

func includesMatch(org storage.OrgState, left, right storage.Value) bool {
	if left.Kind != storage.ValueString || right.Kind != storage.ValueString {
		return false
	}
	wants := splitMultiPicklistValue(right.String)
	if len(wants) == 0 {
		return false
	}
	haves := splitMultiPicklistValue(left.String)
	if len(haves) == 0 {
		return false
	}
	for _, want := range wants {
		found := false
		for _, have := range haves {
			if equalValuesInOrg(org, storage.StringValue(have), storage.StringValue(want)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func includesConditionMatch(org storage.OrgState, left storage.Value, condition *Condition) bool {
	if condition == nil {
		return false
	}
	if len(condition.Values) == 0 {
		return includesMatch(org, left, condition.Value)
	}
	for _, value := range condition.Values {
		if includesMatch(org, left, value) {
			return true
		}
	}
	return false
}

func excludesMatch(org storage.OrgState, left, right storage.Value) bool {
	if right.Kind != storage.ValueString {
		return false
	}
	wants := splitMultiPicklistValue(right.String)
	if len(wants) == 0 {
		return false
	}
	if left.Kind != storage.ValueString {
		return left.Kind == storage.ValueNull
	}
	haves := splitMultiPicklistValue(left.String)
	if len(haves) == 0 {
		return true
	}
	for _, want := range wants {
		for _, have := range haves {
			if equalValuesInOrg(org, storage.StringValue(have), storage.StringValue(want)) {
				return false
			}
		}
	}
	return true
}

func excludesConditionMatch(org storage.OrgState, left storage.Value, condition *Condition) bool {
	if condition == nil {
		return false
	}
	if len(condition.Values) == 0 {
		return excludesMatch(org, left, condition.Value)
	}
	for _, value := range condition.Values {
		if !excludesMatch(org, left, value) {
			return false
		}
	}
	return true
}

func splitMultiPicklistValue(text string) []string {
	parts := strings.Split(text, ";")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

type patternToken struct{ kind, value byte }
type runePatternToken struct {
	kind  byte
	value rune
}
type compiledLikePattern struct {
	tokens     []patternToken
	prev, curr []bool
	// runeTokens is set only when the pattern has non-ASCII text; matching
	// then proceeds per Unicode code point, as native LIKE does.
	runeTokens []runePatternToken
	runes      []rune
}

func compileLikePattern(pattern string) *compiledLikePattern {
	ascii := true
	for i := 0; i < len(pattern); i++ {
		if pattern[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if !ascii {
		var tokens []runePatternToken
		chars := []rune(pattern)
		for i := 0; i < len(chars); i++ {
			if chars[i] == '\\' && i+1 < len(chars) && (chars[i+1] == '\\' || chars[i+1] == '_' || chars[i+1] == '%') {
				i++
				tokens = append(tokens, runePatternToken{value: chars[i]})
				continue
			}
			switch chars[i] {
			case '%', '_':
				tokens = append(tokens, runePatternToken{kind: byte(chars[i])})
			default:
				tokens = append(tokens, runePatternToken{value: chars[i]})
			}
		}
		return &compiledLikePattern{runeTokens: tokens, prev: make([]bool, len(tokens)+1), curr: make([]bool, len(tokens)+1)}
	}
	tokens := make([]patternToken, 0, len(pattern))
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) && (pattern[i+1] == '\\' || pattern[i+1] == '_' || pattern[i+1] == '%') {
			i++
			tokens = append(tokens, patternToken{value: pattern[i]})
			continue
		}
		switch pattern[i] {
		case '%', '_':
			tokens = append(tokens, patternToken{kind: pattern[i]})
		default:
			tokens = append(tokens, patternToken{value: pattern[i]})
		}
	}
	return &compiledLikePattern{tokens: tokens, prev: make([]bool, len(tokens)+1), curr: make([]bool, len(tokens)+1)}
}
func matchLikePattern(text, pattern string) bool { return compileLikePattern(pattern).match(text) }
func (p *compiledLikePattern) match(text string) bool {
	if p.runeTokens != nil || !isASCIIText(text) {
		return p.matchRunes(text)
	}
	// Dynamic programming over text bytes and the tokenized LIKE pattern.
	m, n := len(text), len(p.tokens)
	tokens := p.tokens
	// dp[i][j] = true if text[i:] matches pattern[j:]
	// Use two rows to keep O(n) space.
	prev, curr := p.prev, p.curr
	clear(prev)
	clear(curr)
	prev[n] = true
	for j := n - 1; j >= 0; j-- {
		if tokens[j].kind == '%' {
			prev[j] = prev[j+1]
		} else {
			prev[j] = false
		}
	}
	for i := m - 1; i >= 0; i-- {
		curr[n] = false
		for j := n - 1; j >= 0; j-- {
			switch tokens[j].kind {
			case '%':
				curr[j] = curr[j+1] || prev[j]
			case '_':
				curr[j] = prev[j+1]
			default:
				curr[j] = (asciiLower(text[i]) == asciiLower(tokens[j].value)) && prev[j+1]
			}
		}
		prev, curr = curr, prev
	}
	return prev[0]
}

func asciiLower(ch byte) byte {
	if ch >= 'A' && ch <= 'Z' {
		return ch + ('a' - 'A')
	}
	return ch
}

func isASCIIText(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// likeRuneEqual compares code points case-insensitively (simple Unicode folding).
func likeRuneEqual(a, b rune) bool {
	if a == b {
		return true
	}
	return unicode.ToLower(a) == unicode.ToLower(b) || unicode.ToUpper(a) == unicode.ToUpper(b)
}

func (p *compiledLikePattern) matchRunes(text string) bool {
	tokens := p.runeTokens
	if p.runeTokens == nil {
		// ASCII pattern against non-ASCII text.
		tokens = make([]runePatternToken, len(p.tokens))
		for i, t := range p.tokens {
			tokens[i] = runePatternToken{kind: t.kind, value: rune(t.value)}
		}
		p.runeTokens = tokens
	}
	p.runes = p.runes[:0]
	for _, r := range text {
		p.runes = append(p.runes, r)
	}
	chars := p.runes
	m, n := len(chars), len(tokens)
	if len(p.prev) < n+1 {
		p.prev = make([]bool, n+1)
		p.curr = make([]bool, n+1)
	}
	prev, curr := p.prev[:n+1], p.curr[:n+1]
	clear(prev)
	clear(curr)
	prev[n] = true
	for j := n - 1; j >= 0; j-- {
		prev[j] = tokens[j].kind == '%' && prev[j+1]
	}
	for i := m - 1; i >= 0; i-- {
		curr[n] = false
		for j := n - 1; j >= 0; j-- {
			switch tokens[j].kind {
			case '%':
				curr[j] = curr[j+1] || prev[j]
			case '_':
				curr[j] = prev[j+1]
			default:
				curr[j] = likeRuneEqual(chars[i], tokens[j].value) && prev[j+1]
			}
		}
		prev, curr = curr, prev
	}
	return prev[0]
}

func turkishTextLocale(locale string) bool {
	return locale == "tr_TR" || locale == "tr"
}

func foldTurkish(text string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case 'I':
			return '\u0131'
		case '\u0130':
			return 'i'
		}
		return unicode.ToLower(r)
	}, text)
}
