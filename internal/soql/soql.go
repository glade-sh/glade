package soql

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/storage"
)

type Query struct {
	Fields           []string
	ChildQueries     []ChildQuery
	Typeofs          []TypeofSpec
	Object           string
	Where            *Condition
	Having           *Condition
	OrderBy          string
	OrderDesc        bool
	Order            []OrderSpec
	Limit            int
	Offset           int
	Count            bool
	ForUpdate        bool
	AllRows          bool
	SecurityMode     string
	Aggregates       []Aggregate
	HavingAggregates []Aggregate
	GroupBy          []string
	GroupMode        string
}

type OrderSpec struct {
	Field string
	Desc  bool
	Nulls string
}

type ChildQuery struct {
	Relationship string
	Query        Query
}

type TypeofSpec struct {
	Relationship string
	When         map[string][]string
	Else         []string
}

type Aggregate struct {
	Func  string
	Field string
	Alias string
}

type Condition struct {
	Not      bool
	And      []Condition
	Or       []Condition
	Field    string
	Op       string
	Value    storage.Value
	Value2   storage.Value
	Range    bool
	Values   []storage.Value
	Subquery *Query
}

type Result struct {
	Records []storage.Record `json:"records"`
	Rows    int              `json:"rows"`
}

type UnsupportedFeatureError struct {
	Message string
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
	tokens, err := lex(input)
	if err != nil {
		return Query{}, err
	}
	p := parser{tokens: tokens, now: now.UTC()}
	return p.parseQuery()
}

func Execute(org storage.OrgState, query Query) (Result, error) {
	objectName, ok := storage.ResolveObjectName(org, query.Object)
	if !ok {
		return Result{}, fmt.Errorf("soql: unknown object %s", query.Object)
	}
	object := org.Objects[objectName]
	if len(query.Fields) == 0 && len(query.ChildQueries) == 0 && len(query.Typeofs) == 0 {
		return Result{}, fmt.Errorf("soql: SELECT requires at least one field")
	}
	fields, err := expandFieldsFunctions(object.Definition, query.Fields)
	if err != nil {
		return Result{}, err
	}
	query.Fields = fields
	if err := validateQueryReferences(org, object.Definition, query, query.SecurityMode); err != nil {
		return Result{}, err
	}
	if len(query.ChildQueries) > 0 && queryHasAggregates(query) {
		return Result{}, unsupportedSOQLErrorf("child relationship subqueries are not supported in aggregate queries")
	}
	if query.ForUpdate && queryHasAggregates(query) {
		return Result{}, unsupportedSOQLErrorf("FOR UPDATE is not supported with aggregate queries")
	}
	if query.Where != nil {
		condition, err := resolveSubqueries(org, *query.Where)
		if err != nil {
			return Result{}, err
		}
		query.Where = &condition
	}
	if query.Having != nil {
		condition, err := resolveSubqueries(org, *query.Having)
		if err != nil {
			return Result{}, err
		}
		query.Having = &condition
	}

	ids := candidateRecordIDs(object, query.Where, query.AllRows)
	sort.Strings(ids)

	matchedRecords := make([]storage.Record, 0, len(ids))
	for _, idText := range ids {
		record := object.Records[storage.ID(idText)]
		if record.System.IsDeleted && !query.AllRows {
			continue
		}
		record.Object = objectName
		if matches(org, object.Definition, record, query.Where) {
			matchedRecords = append(matchedRecords, record)
		}
	}
	if queryHasAggregates(query) {
		records, err := aggregateRecords(org, object.Definition, matchedRecords, query)
		if err != nil {
			return Result{}, err
		}
		if len(query.Order) > 0 {
			sort.SliceStable(records, func(i, j int) bool {
				return aggregateOrderedBefore(records[i], records[j], query.Order)
			})
		}
		records = applyWindow(records, query.Offset, query.Limit)
		return Result{Records: records, Rows: len(records)}, nil
	}
	if len(query.Order) > 0 {
		sort.SliceStable(matchedRecords, func(i, j int) bool {
			return recordsOrderedBefore(org, object.Definition, matchedRecords[i], matchedRecords[j], query.Order)
		})
	}
	matchedRecords = applyWindow(matchedRecords, query.Offset, query.Limit)
	records := make([]storage.Record, 0, len(matchedRecords))
	for _, record := range matchedRecords {
		if query.ForUpdate && record.System.Locked {
			return Result{}, fmt.Errorf("soql: unable to lock row %s", record.ID)
		}
		projected, err := projectRecord(org, object.Definition, record, query.Fields, query.ChildQueries, query.Typeofs)
		if err != nil {
			return Result{}, err
		}
		if query.ForUpdate {
			projected.System.Locked = true
		}
		records = append(records, projected)
	}
	return Result{Records: records, Rows: len(records)}, nil
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
	if allRows || where == nil || where.Not || len(where.And) != 0 || len(where.Or) != 0 || where.Range || where.Subquery != nil || where.Op != "=" {
		return nil, false
	}
	if strings.Contains(where.Field, ".") {
		return nil, false
	}
	return storage.LookupIndex(object, where.Field, where.Value)
}

func applyWindow[T any](records []T, offset, limit int) []T {
	if offset > 0 {
		if offset >= len(records) {
			return nil
		}
		records = records[offset:]
	}
	if limit > 0 && limit < len(records) {
		return records[:limit]
	}
	return records
}

func aggregateRecords(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, query Query) ([]storage.Record, error) {
	if len(query.GroupBy) == 0 {
		fields, err := aggregateFields(org, definition, records, query.Aggregates, nil)
		if err != nil {
			return nil, err
		}
		if err := addHiddenAggregateFields(fields, org, definition, records, query.HavingAggregates, nil); err != nil {
			return nil, err
		}
		record := storage.Record{Object: "AggregateResult", Fields: fields}
		if query.Having != nil && !matches(org, storage.ObjectDefinition{}, record, query.Having) {
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
	for _, record := range records {
		for setIndex, active := range sets {
			values := make(map[string]storage.Value, len(query.GroupBy))
			grouping := make(map[string]bool, len(query.GroupBy))
			parts := []string{strconv.Itoa(setIndex)}
			for _, field := range query.GroupBy {
				if !active[field] {
					values[field] = storage.NullValue()
					grouping[field] = true
					parts = append(parts, "1:")
					continue
				}
				value, ok := recordValue(org, definition, record, field)
				if !ok {
					value = storage.NullValue()
				}
				values[field] = value
				parts = append(parts, "0:"+valueKey(value))
			}
			key := strings.Join(parts, "\x00")
			current, ok := groups[key]
			if !ok {
				current = &group{key: key, values: values, grouping: grouping}
				groups[key] = current
				order = append(order, key)
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
			fields[field] = group.values[field].Clone()
		}
		aggregateFields, err := aggregateFields(org, definition, group.records, query.Aggregates, group.grouping)
		if err != nil {
			return nil, err
		}
		for field, value := range aggregateFields {
			fields[field] = value
		}
		if err := addHiddenAggregateFields(fields, org, definition, group.records, query.HavingAggregates, group.grouping); err != nil {
			return nil, err
		}
		record := storage.Record{Object: "AggregateResult", Fields: fields}
		if query.Having != nil && !matches(org, storage.ObjectDefinition{}, record, query.Having) {
			continue
		}
		removeHiddenAggregateFields(record.Fields, query.HavingAggregates)
		out = append(out, record)
	}
	return out, nil
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

func aggregateRecordValue(record storage.Record, field string) storage.Value {
	if value, ok := record.Fields[field]; ok {
		return value
	}
	return storage.NullValue()
}

func aggregateFields(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, aggregates []Aggregate, grouping map[string]bool) (map[string]storage.Value, error) {
	fields := make(map[string]storage.Value, len(aggregates)*2)
	for i, aggregate := range aggregates {
		value, err := aggregateResultValue(org, definition, records, aggregate, grouping)
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

func addHiddenAggregateFields(fields map[string]storage.Value, org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, aggregates []Aggregate, grouping map[string]bool) error {
	for _, aggregate := range aggregates {
		if aggregate.Alias == "" {
			continue
		}
		value, err := aggregateResultValue(org, definition, records, aggregate, grouping)
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

func aggregateResultValue(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, aggregate Aggregate, grouping map[string]bool) (storage.Value, error) {
	if aggregate.Func == "GROUPING" {
		if grouping[aggregate.Field] {
			return storage.IntegerValue(1), nil
		}
		return storage.IntegerValue(0), nil
	}
	return aggregateValue(org, definition, records, aggregate)
}

func aggregateValue(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, aggregate Aggregate) (storage.Value, error) {
	switch aggregate.Func {
	case "COUNT":
		if aggregate.Field == "" {
			return storage.IntegerValue(int64(len(records))), nil
		}
		var count int64
		for _, record := range records {
			value, ok := recordValue(org, definition, record, aggregate.Field)
			if ok && value.Kind != storage.ValueNull {
				count++
			}
		}
		return storage.IntegerValue(count), nil
	case "COUNT_DISTINCT":
		seen := map[string]bool{}
		for _, record := range records {
			value, ok := recordValue(org, definition, record, aggregate.Field)
			if ok && value.Kind != storage.ValueNull {
				seen[valueKey(value)] = true
			}
		}
		return storage.IntegerValue(int64(len(seen))), nil
	case "SUM", "AVG":
		sum := new(big.Rat)
		var count int64
		for _, record := range records {
			value, ok := recordValue(org, definition, record, aggregate.Field)
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
		return storage.DecimalValue(decimalString(sum)), nil
	case "MIN", "MAX":
		var best storage.Value
		found := false
		for _, record := range records {
			value, ok := recordValue(org, definition, record, aggregate.Field)
			if !ok || value.Kind == storage.ValueNull {
				continue
			}
			if !found {
				best = value.Clone()
				found = true
				continue
			}
			cmp := compareValues(value, best)
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
	query, err := ParseAt(input, now)
	if err != nil {
		return Result{}, err
	}
	return Execute(org, query)
}

func matches(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, condition *Condition) bool {
	if condition == nil {
		return true
	}
	if condition.Not {
		return !matches(org, definition, record, &Condition{
			And: condition.And, Or: condition.Or,
			Field: condition.Field, Op: condition.Op,
			Value: condition.Value, Value2: condition.Value2, Range: condition.Range, Values: condition.Values, Subquery: condition.Subquery,
		})
	}
	if len(condition.And) > 0 {
		for _, c := range condition.And {
			if !matches(org, definition, record, &c) {
				return false
			}
		}
		return true
	}
	if len(condition.Or) > 0 {
		for _, c := range condition.Or {
			if matches(org, definition, record, &c) {
				return true
			}
		}
		return false
	}
	left, ok := recordValue(org, definition, record, condition.Field)
	if !ok {
		left = storage.NullValue()
	}
	switch condition.Op {
	case "=":
		if condition.Range {
			return compareValues(left, condition.Value) >= 0 && compareValues(left, condition.Value2) < 0
		}
		return equalValues(left, condition.Value)
	case "!=":
		if condition.Range {
			return compareValues(left, condition.Value) < 0 || compareValues(left, condition.Value2) >= 0
		}
		return !equalValues(left, condition.Value)
	case ">":
		return compareValues(left, condition.Value) > 0
	case ">=":
		return compareValues(left, condition.Value) >= 0
	case "<":
		return compareValues(left, condition.Value) < 0
	case "<=":
		return compareValues(left, condition.Value) <= 0
	case "LIKE":
		return likeMatch(left, condition.Value)
	case "NOT LIKE":
		return !likeMatch(left, condition.Value)
	case "IN":
		if condition.Subquery != nil {
			return false
		}
		for _, v := range condition.Values {
			if equalValues(left, v) {
				return true
			}
		}
		return false
	case "NOT IN":
		if condition.Subquery != nil {
			return false
		}
		for _, v := range condition.Values {
			if equalValues(left, v) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func resolveSubqueries(org storage.OrgState, condition Condition) (Condition, error) {
	if condition.Subquery != nil {
		if len(condition.Subquery.Fields) != 1 || condition.Subquery.Count || len(condition.Subquery.Aggregates) > 0 {
			return Condition{}, fmt.Errorf("soql: semi-join subquery must select exactly one field")
		}
		result, err := Execute(org, *condition.Subquery)
		if err != nil {
			return Condition{}, err
		}
		field := condition.Subquery.Fields[0]
		values := make([]storage.Value, 0, len(result.Records))
		for _, record := range result.Records {
			value, ok := subqueryRecordValue(org, record, field)
			if !ok {
				return Condition{}, fmt.Errorf("soql: unknown subquery field %s", field)
			}
			values = append(values, value)
		}
		condition.Values = values
		condition.Subquery = nil
	}
	for i := range condition.And {
		resolved, err := resolveSubqueries(org, condition.And[i])
		if err != nil {
			return Condition{}, err
		}
		condition.And[i] = resolved
	}
	for i := range condition.Or {
		resolved, err := resolveSubqueries(org, condition.Or[i])
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
	value, ok := record.Fields[field]
	return value, ok
}

func projectRecord(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, fields []string, childQueries []ChildQuery, typeofs []TypeofSpec) (storage.Record, error) {
	out := storage.Record{
		ID:            record.ID,
		Object:        record.Object,
		Fields:        make(map[string]storage.Value),
		Children:      make(map[string][]storage.Record),
		ExplicitNulls: make(map[string]bool),
		System:        record.System,
	}
	for _, field := range fields {
		canonicalField, ok := storage.ResolveFieldName(definition, org.Namespace, field)
		if ok && canonicalField == "Id" {
			out.Fields[canonicalField] = storage.IDValue(record.ID)
			continue
		}
		if strings.Contains(field, ".") {
			if value, ok := relationshipValue(org, record, field); ok {
				out.Fields[field] = value
			}
			continue
		}
		if !ok {
			canonicalField = field
		}
		if record.ExplicitNulls[canonicalField] {
			out.ExplicitNulls[canonicalField] = true
			continue
		}
		if value, ok := record.Fields[canonicalField]; ok {
			out.Fields[canonicalField] = value.Clone()
			continue
		}
		if value, ok := recordValue(org, definition, record, field); ok {
			out.Fields[canonicalField] = value.Clone()
		}
	}
	for _, childQuery := range childQueries {
		records, err := executeChildRelationshipQuery(org, definition, record, childQuery)
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
		selected := spec.When[typeName]
		if len(selected) == 0 {
			selected = spec.Else
		}
		for _, field := range selected {
			value, ok := relationshipValue(org, record, spec.Relationship+"."+field)
			if ok {
				out.Fields[spec.Relationship+"."+field] = value.Clone()
			}
		}
	}
	if len(out.Children) == 0 {
		out.Children = nil
	}
	return out, nil
}

func executeChildRelationshipQuery(org storage.OrgState, parentDefinition storage.ObjectDefinition, parent storage.Record, childQuery ChildQuery) ([]storage.Record, error) {
	childObjectName, relation, ok := childRelationship(org, parentDefinition, childQuery.Relationship)
	if !ok {
		return nil, fmt.Errorf("soql: unknown child relationship %s on %s", childQuery.Relationship, parentDefinition.APIName)
	}
	childObject := org.Objects[childObjectName]
	query := childQuery.Query
	query.Object = childObjectName
	if len(query.ChildQueries) > 0 {
		return nil, fmt.Errorf("soql: nested child relationship subqueries are not supported")
	}
	if query.Count || len(query.Aggregates) > 0 || len(query.GroupBy) > 0 || query.Having != nil {
		return nil, fmt.Errorf("soql: aggregate child relationship subqueries are not supported")
	}
	fields, err := expandFieldsFunctions(childObject.Definition, query.Fields)
	if err != nil {
		return nil, err
	}
	query.Fields = fields
	if err := validateQueryReferences(org, childObject.Definition, query, query.SecurityMode); err != nil {
		return nil, err
	}
	if query.Where != nil {
		condition, err := resolveSubqueries(org, *query.Where)
		if err != nil {
			return nil, err
		}
		query.Where = &condition
	}
	ids := make([]string, 0, len(childObject.Records))
	for id := range childObject.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
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
		if matches(org, childObject.Definition, child, query.Where) {
			matched = append(matched, child)
		}
	}
	if len(query.Order) > 0 {
		sort.SliceStable(matched, func(i, j int) bool {
			return recordsOrderedBefore(org, childObject.Definition, matched[i], matched[j], query.Order)
		})
	}
	matched = applyWindow(matched, query.Offset, query.Limit)
	out := make([]storage.Record, 0, len(matched))
	for _, child := range matched {
		projected, err := projectRecord(org, childObject.Definition, child, query.Fields, nil, query.Typeofs)
		if err != nil {
			return nil, err
		}
		out = append(out, projected)
	}
	return out, nil
}

func childRelationship(org storage.OrgState, parentDefinition storage.ObjectDefinition, relationship string) (string, storage.Relationship, bool) {
	parentName := parentDefinition.APIName
	for childObjectName, childObject := range org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if relation.ChildRelationship != relationship {
				continue
			}
			for _, candidate := range relation.ParentObjects {
				resolved, ok := storage.ResolveObjectName(org, candidate)
				if !ok {
					resolved = candidate
				}
				if resolved == parentName {
					return childObjectName, relation, true
				}
			}
		}
	}
	return "", storage.Relationship{}, false
}

func expandFieldsFunctions(definition storage.ObjectDefinition, fields []string) ([]string, error) {
	out := make([]string, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	appendField := func(field string) {
		if seen[field] {
			return
		}
		seen[field] = true
		out = append(out, field)
	}
	names := make([]string, 0, len(definition.Fields))
	for name := range definition.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, field := range fields {
		mode, ok := fieldsFunctionMode(field)
		if !ok {
			appendField(field)
			continue
		}
		switch mode {
		case "ALL":
			appendField("Id")
			for _, name := range names {
				appendField(name)
			}
		case "STANDARD":
			appendField("Id")
			for _, name := range names {
				if !isCustomFieldName(name) {
					appendField(name)
				}
			}
		case "CUSTOM":
			for _, name := range names {
				if isCustomFieldName(name) {
					appendField(name)
				}
			}
		default:
			return nil, fmt.Errorf("soql: unsupported FIELDS(%s)", mode)
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
	if err := validateAggregateAliases(query); err != nil {
		return err
	}
	for _, field := range query.Fields {
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
	for _, field := range query.GroupBy {
		if err := validateFieldReference(org, definition, field, mode); err != nil {
			return err
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
		if err := validateFieldReference(org, definition, spec.Field, mode); err != nil {
			return err
		}
	}
	for _, spec := range query.Typeofs {
		if err := validateTypeofReference(org, definition, spec, mode); err != nil {
			return err
		}
	}
	for _, childQuery := range query.ChildQueries {
		childName, _, ok := childRelationship(org, definition, childQuery.Relationship)
		if !ok {
			return childRelationshipUnavailableError(childQuery.Relationship, mode)
		}
		child := org.Objects[childName]
		childFields, err := expandFieldsFunctions(child.Definition, childQuery.Query.Fields)
		if err != nil {
			return err
		}
		nested := childQuery.Query
		nested.Fields = childFields
		nested.SecurityMode = mode
		if err := validateQueryReferences(org, child.Definition, nested, mode); err != nil {
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
	if (aggregate.Func == "SUM" || aggregate.Func == "AVG") && !aggregateFieldMayBeNumeric(org, definition, aggregate.Field) {
		return fmt.Errorf("soql: %s requires numeric field %s", aggregate.Func, aggregate.Field)
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
		case storage.FieldAny, storage.FieldInteger, storage.FieldDecimal, storage.FieldCalculated:
			continue
		default:
			return false
		}
	}
	return true
}

func fieldDefinitionsForReference(org storage.OrgState, definition storage.ObjectDefinition, field string) ([]storage.Field, bool) {
	if canonical, ok := storage.ResolveFieldName(definition, org.Namespace, field); ok && canonical == "Id" {
		return []storage.Field{{APIName: "Id", Type: storage.FieldID}}, true
	}
	if systemField, ok := systemFieldDefinition(field); ok {
		return []storage.Field{systemField}, true
	}
	if strings.Contains(field, ".") {
		parts := strings.SplitN(field, ".", 2)
		for _, relation := range definition.Relations {
			if relation.ParentRelationship != parts[0] {
				continue
			}
			if len(relation.ParentObjects) == 0 {
				return nil, false
			}
			var out []storage.Field
			for _, parentName := range relation.ParentObjects {
				canonical, ok := storage.ResolveObjectName(org, parentName)
				if !ok {
					return nil, false
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
	canonicalField, ok := storage.ResolveFieldName(definition, org.Namespace, field)
	if !ok {
		return nil, false
	}
	return []storage.Field{definition.Fields[canonicalField]}, true
}

func systemFieldDefinition(field string) (storage.Field, bool) {
	switch field {
	case "CreatedDate", "LastModifiedDate", "SystemModstamp":
		return storage.Field{APIName: field, Type: storage.FieldDateTime}, true
	case "CreatedById", "LastModifiedById", "OwnerId":
		return storage.Field{APIName: field, Type: storage.FieldID}, true
	case "IsDeleted":
		return storage.Field{APIName: field, Type: storage.FieldBoolean}, true
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
	if aggregateOrderFieldKnown(query, condition.Field) {
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
	return fieldUnavailableError(condition.Field, mode)
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
	if condition.Field != "" {
		if err := validateFieldReference(org, definition, condition.Field, mode); err != nil {
			return err
		}
	}
	return nil
}

func validateTypeofReference(org storage.OrgState, definition storage.ObjectDefinition, spec TypeofSpec, mode string) error {
	allowedTargets := map[string]bool{}
	relationshipKnown := false
	for _, relation := range definition.Relations {
		if relation.ParentRelationship != spec.Relationship {
			continue
		}
		relationshipKnown = true
		for _, parentName := range relation.ParentObjects {
			canonical, ok := storage.ResolveObjectName(org, parentName)
			if ok {
				allowedTargets[canonical] = true
			}
		}
	}
	if !relationshipKnown {
		return fieldUnavailableError(spec.Relationship, mode)
	}
	for typeName, fields := range spec.When {
		parentName, ok := storage.ResolveObjectName(org, typeName)
		if !ok || !allowedTargets[parentName] {
			return typeofTargetUnavailableError(typeName, mode)
		}
		parent := org.Objects[parentName]
		for _, field := range fields {
			if err := validateFieldReference(org, parent.Definition, field, mode); err != nil {
				return fieldUnavailableError(typeName+"."+field, mode)
			}
		}
	}
	for _, field := range spec.Else {
		if err := validateFieldReference(org, definition, spec.Relationship+"."+field, mode); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldReference(org storage.OrgState, definition storage.ObjectDefinition, field string, mode string) error {
	if fieldKnownForMode(org, definition, field, mode != "") {
		return nil
	}
	return fieldUnavailableError(field, mode)
}

func fieldKnown(org storage.OrgState, definition storage.ObjectDefinition, field string) bool {
	return fieldKnownForMode(org, definition, field, false)
}

func fieldKnownForMode(org storage.OrgState, definition storage.ObjectDefinition, field string, requireAllParents bool) bool {
	if field == "Id" || isSystemField(field) {
		return true
	}
	if len(definition.Fields) == 0 && !strings.Contains(field, ".") {
		return true
	}
	if strings.Contains(field, ".") {
		parts := strings.SplitN(field, ".", 2)
		for _, relation := range definition.Relations {
			if relation.ParentRelationship != parts[0] {
				continue
			}
			if len(relation.ParentObjects) == 0 {
				return false
			}
			found := false
			for _, parentName := range relation.ParentObjects {
				canonical, ok := storage.ResolveObjectName(org, parentName)
				if !ok {
					return false
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

func queryHasAggregates(query Query) bool {
	return len(query.Aggregates) > 0 || len(query.GroupBy) > 0 || query.Having != nil
}

func aggregateOrderFieldKnown(query Query, field string) bool {
	if containsName(query.GroupBy, field) {
		return true
	}
	for i, aggregate := range query.Aggregates {
		if field == fmt.Sprintf("expr%d", i) || (aggregate.Alias != "" && field == aggregate.Alias) {
			return true
		}
	}
	for _, aggregate := range query.HavingAggregates {
		if aggregate.Alias != "" && field == aggregate.Alias {
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
	if mode != "" {
		return fmt.Errorf("soql: unknown child relationship %s in %s mode", relationship, mode)
	}
	return fmt.Errorf("soql: unknown child relationship %s", relationship)
}

func typeofTargetUnavailableError(typeName, mode string) error {
	if mode != "" {
		return fmt.Errorf("soql: unknown TYPEOF target %s in %s mode", typeName, mode)
	}
	return fmt.Errorf("soql: unknown TYPEOF target %s", typeName)
}

func isSystemField(field string) bool {
	switch field {
	case "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SystemModstamp", "OwnerId", "IsDeleted":
		return true
	default:
		return false
	}
}

func isCustomFieldName(name string) bool {
	return strings.HasSuffix(name, "__c")
}

func polymorphicParentObject(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, relationship string) (string, bool) {
	for _, relation := range definition.Relations {
		if relation.ParentRelationship != relationship {
			continue
		}
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

func relationshipValue(org storage.OrgState, record storage.Record, field string) (storage.Value, bool) {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return storage.Value{}, false
	}
	object, ok := org.Objects[record.Object]
	if !ok {
		return storage.Value{}, false
	}
	for _, relation := range object.Definition.Relations {
		if relation.ParentRelationship != parts[0] {
			continue
		}
		parentID, ok := recordValue(org, object.Definition, record, relation.Field)
		if !ok || parentID.Kind == storage.ValueNull {
			return storage.NullValue(), true
		}
		for _, parentObjectName := range relation.ParentObjects {
			canonicalParent, ok := storage.ResolveObjectName(org, parentObjectName)
			if !ok {
				continue
			}
			parentObject := org.Objects[canonicalParent]
			parent, ok := parentObject.Records[idFromValue(parentID)]
			if !ok || parent.System.IsDeleted {
				continue
			}
			parent.Object = canonicalParent
			value, ok := recordValue(org, parentObject.Definition, parent, parts[1])
			if !ok {
				return storage.NullValue(), true
			}
			return value.Clone(), true
		}
	}
	return storage.Value{}, false
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

func recordValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, field string) (storage.Value, bool) {
	if canonical, ok := storage.ResolveFieldName(definition, org.Namespace, field); ok && canonical == "Id" {
		return storage.IDValue(record.ID), true
	}
	if strings.Contains(field, ".") {
		return relationshipValue(org, record, field)
	}
	switch field {
	case "CreatedDate":
		if record.System.CreatedDate != "" {
			return storage.DateTimeValue(record.System.CreatedDate), true
		}
	case "CreatedById":
		if record.System.CreatedByID != "" {
			return storage.IDValue(record.System.CreatedByID), true
		}
	case "LastModifiedDate":
		if record.System.LastModifiedDate != "" {
			return storage.DateTimeValue(record.System.LastModifiedDate), true
		}
	case "LastModifiedById":
		if record.System.LastModifiedByID != "" {
			return storage.IDValue(record.System.LastModifiedByID), true
		}
	case "SystemModstamp":
		if record.System.SystemModstamp != "" {
			return storage.DateTimeValue(record.System.SystemModstamp), true
		}
	case "OwnerId":
		if record.System.OwnerID != "" {
			return storage.IDValue(record.System.OwnerID), true
		}
	case "IsDeleted":
		return storage.BooleanValue(record.System.IsDeleted), true
	}
	canonicalField, ok := storage.ResolveFieldName(definition, org.Namespace, field)
	if !ok {
		canonicalField = field
	}
	if record.ExplicitNulls[canonicalField] {
		return storage.NullValue(), true
	}
	value, ok := record.Fields[canonicalField]
	return value, ok
}

func equalValues(left, right storage.Value) bool {
	if leftNumber, rightNumber, ok := numericValues(left, right); ok {
		return leftNumber.Cmp(rightNumber) == 0
	}
	if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
		return string(left.ID) == right.String
	}
	if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
		return left.String == string(right.ID)
	}
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return left.String == right.String
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueDecimal:
		return left.Decimal == right.Decimal
	case storage.ValueID:
		return left.ID == right.ID
	default:
		return false
	}
}

func compareValues(left, right storage.Value) int {
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
		return strings.Compare(string(left.ID), right.String)
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
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
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
		return strings.Compare(string(left.ID), string(right.ID))
	default:
		return 0
	}
}

func recordsOrderedBefore(org storage.OrgState, definition storage.ObjectDefinition, leftRecord, rightRecord storage.Record, order []OrderSpec) bool {
	for _, spec := range order {
		left, ok := recordValue(org, definition, leftRecord, spec.Field)
		if !ok {
			left = storage.NullValue()
		}
		right, ok := recordValue(org, definition, rightRecord, spec.Field)
		if !ok {
			right = storage.NullValue()
		}
		cmp := orderCompare(left, right, spec)
		if cmp == 0 {
			continue
		}
		return cmp < 0
	}
	return false
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
	if spec.Nulls != "" && (left.Kind == storage.ValueNull || right.Kind == storage.ValueNull) {
		if left.Kind == right.Kind {
			return 0
		}
		if spec.Nulls == "FIRST" {
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
	cmp := compareValues(left, right)
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
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal, storage.ValueBlob:
		return string(value.Kind) + ":" + value.String
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

func likeMatch(left, right storage.Value) bool {
	if left.Kind != storage.ValueString || right.Kind != storage.ValueString {
		return false
	}
	return matchLikePattern(left.String, right.String)
}

func matchLikePattern(text, pattern string) bool {
	// Dynamic programming approach for SQL LIKE matching.
	// % matches any sequence, _ matches any single character.
	m, n := len(text), len(pattern)
	// dp[i][j] = true if text[i:] matches pattern[j:]
	// Use two rows to keep O(n) space.
	prev := make([]bool, n+1)
	curr := make([]bool, n+1)
	prev[n] = true
	for j := n - 1; j >= 0; j-- {
		if pattern[j] == '%' {
			prev[j] = prev[j+1]
		} else {
			prev[j] = false
		}
	}
	for i := m - 1; i >= 0; i-- {
		curr[n] = false
		for j := n - 1; j >= 0; j-- {
			switch pattern[j] {
			case '%':
				curr[j] = curr[j+1] || prev[j]
			case '_':
				curr[j] = prev[j+1]
			default:
				curr[j] = (asciiLower(text[i]) == asciiLower(pattern[j])) && prev[j+1]
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

type token struct {
	text string
}

func lex(input string) ([]token, error) {
	var out []token
	for i := 0; i < len(input); {
		switch {
		case input[i] == ' ' || input[i] == '\n' || input[i] == '\t' || input[i] == '\r':
			i++
		case input[i] == '\'':
			start := i
			i++
			for i < len(input) {
				if input[i] == '\'' {
					if i+1 < len(input) && input[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					out = append(out, token{text: input[start:i]})
					goto next
				}
				i++
			}
			return nil, fmt.Errorf("soql: unterminated string literal")
		case strings.ContainsRune(",=!*()<>", rune(input[i])):
			if i+1 < len(input) && input[i:i+2] == "!=" {
				out = append(out, token{text: "!="})
				i += 2
			} else if i+1 < len(input) && input[i:i+2] == "<=" {
				out = append(out, token{text: "<="})
				i += 2
			} else if i+1 < len(input) && input[i:i+2] == ">=" {
				out = append(out, token{text: ">="})
				i += 2
			} else {
				out = append(out, token{text: input[i : i+1]})
				i++
			}
		default:
			start := i
			for i < len(input) && !strings.ContainsRune(" \n\t\r,=!*()<>", rune(input[i])) {
				i++
			}
			out = append(out, token{text: input[start:i]})
		}
	next:
	}
	out = append(out, token{text: ""})
	return out, nil
}

type parser struct {
	tokens []token
	pos    int
	now    time.Time
}

func (p *parser) parseQuery() (Query, error) {
	if !p.matchWord("SELECT") {
		return Query{}, p.errorf("expected SELECT")
	}
	fields, childQueries, typeofs, err := p.parseFields()
	if err != nil {
		return Query{}, err
	}
	if !p.matchWord("FROM") {
		return Query{}, p.errorf("expected FROM")
	}
	object := p.advance().text
	if object == "" {
		return Query{}, p.errorf("expected object name")
	}
	aggregates, err := aggregateSpecs(fields)
	if err != nil {
		return Query{}, err
	}
	q := Query{Fields: fields, ChildQueries: childQueries, Typeofs: typeofs, Object: object, Count: len(fields) == 1 && strings.EqualFold(fields[0], "COUNT()"), Aggregates: aggregates}
	for p.peek().text != "" && p.peek().text != ")" {
		switch {
		case p.matchWord("WHERE"):
			condition, err := p.parseOrCondition()
			if err != nil {
				return Query{}, err
			}
			q.Where = &condition
		case p.matchWord("GROUP"):
			if !p.matchWord("BY") {
				return Query{}, p.errorf("expected BY after GROUP")
			}
			groupMode := ""
			if p.matchWord("ROLLUP") || p.matchWord("CUBE") {
				groupMode = strings.ToUpper(p.tokens[p.pos-1].text)
				if !p.match("(") {
					return Query{}, p.errorf("expected ( after GROUP BY %s", groupMode)
				}
			}
			groupBy, err := p.parseNameList()
			if err != nil {
				return Query{}, err
			}
			if groupMode != "" && !p.match(")") {
				return Query{}, p.errorf("expected ) after GROUP BY %s", groupMode)
			}
			q.GroupBy = groupBy
			q.GroupMode = groupMode
		case p.matchWord("HAVING"):
			condition, err := p.parseOrCondition()
			if err != nil {
				return Query{}, err
			}
			condition = rewriteHavingAggregates(condition, &q)
			q.Having = &condition
		case p.matchWord("ORDER"):
			if !p.matchWord("BY") {
				return Query{}, p.errorf("expected BY after ORDER")
			}
			order, err := p.parseOrderList()
			if err != nil {
				return Query{}, err
			}
			if len(order) == 0 {
				return Query{}, p.errorf("expected ORDER BY field")
			}
			q.Order = order
			q.OrderBy = order[0].Field
			q.OrderDesc = order[0].Desc
		case p.matchWord("LIMIT"):
			limit, err := p.parseInt()
			if err != nil {
				return Query{}, err
			}
			q.Limit = limit
		case p.matchWord("OFFSET"):
			offset, err := p.parseInt()
			if err != nil {
				return Query{}, err
			}
			q.Offset = offset
		case p.matchWord("FOR"):
			if !p.matchWord("UPDATE") {
				return Query{}, p.errorf("expected UPDATE after FOR")
			}
			q.ForUpdate = true
		case p.matchWord("ALL"):
			if !p.matchWord("ROWS") {
				return Query{}, p.errorf("expected ROWS after ALL")
			}
			q.AllRows = true
		case p.matchWord("WITH"):
			mode, err := p.parseSecurityMode()
			if err != nil {
				return Query{}, err
			}
			q.SecurityMode = mode
		default:
			return Query{}, unsupportedSOQLErrorf("unsupported SOQL token %q", p.peek().text)
		}
	}
	if err := validateAggregateQuery(q); err != nil {
		return Query{}, err
	}
	return q, nil
}

func (p *parser) parseFields() ([]string, []ChildQuery, []TypeofSpec, error) {
	var fields []string
	var childQueries []ChildQuery
	var typeofs []TypeofSpec
	for {
		if p.match("(") {
			query, err := p.parseQuery()
			if err != nil {
				return nil, nil, nil, err
			}
			if !p.match(")") {
				return nil, nil, nil, p.errorf("expected ) after child relationship subquery")
			}
			if len(query.Fields) == 0 && len(query.Typeofs) == 0 {
				return nil, nil, nil, p.errorf("child relationship subquery requires at least one field")
			}
			childQueries = append(childQueries, ChildQuery{Relationship: query.Object, Query: query})
			if !p.match(",") {
				return fields, childQueries, typeofs, nil
			}
			continue
		}
		field, err := p.parseName()
		if err != nil {
			return nil, nil, nil, err
		}
		if strings.EqualFold(field, "TYPEOF") {
			spec, err := p.parseTypeofSpec()
			if err != nil {
				return nil, nil, nil, err
			}
			typeofs = append(typeofs, spec)
			if !p.match(",") {
				return fields, childQueries, typeofs, nil
			}
			continue
		}
		if strings.EqualFold(field, "FIELDS") && p.match("(") {
			arg, err := p.parseName()
			if err != nil {
				return nil, nil, nil, err
			}
			if !p.match(")") {
				return nil, nil, nil, p.errorf("expected ) after FIELDS(")
			}
			field = "FIELDS(" + strings.ToUpper(arg) + ")"
			fields = append(fields, field)
			if !p.match(",") {
				return fields, childQueries, typeofs, nil
			}
			continue
		}
		isAggregate := isAggregateFunc(field) && p.match("(")
		if isAggregate {
			if strings.EqualFold(field, "COUNT") && p.match(")") {
				field = "COUNT()"
			} else {
				arg, err := p.parseName()
				if err != nil {
					return nil, nil, nil, err
				}
				if !p.match(")") {
					return nil, nil, nil, p.errorf("expected ) after %s(", field)
				}
				field = strings.ToUpper(field) + "(" + arg + ")"
			}
			alias := ""
			if p.matchWord("AS") {
				var err error
				alias, err = p.parseName()
				if err != nil {
					return nil, nil, nil, err
				}
			} else if tok := p.peek().text; tok != "" && tok != "," && !strings.EqualFold(tok, "FROM") {
				alias = p.advance().text
			}
			if alias != "" {
				field += " " + alias
			}
		}
		fields = append(fields, field)
		if !p.match(",") {
			return fields, childQueries, typeofs, nil
		}
	}
}

func (p *parser) parseNameList() ([]string, error) {
	var names []string
	for {
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		names = append(names, name)
		if !p.match(",") {
			return names, nil
		}
	}
}

func (p *parser) parseTypeofSpec() (TypeofSpec, error) {
	relationship, err := p.parseName()
	if err != nil {
		return TypeofSpec{}, err
	}
	spec := TypeofSpec{Relationship: relationship, When: make(map[string][]string)}
	for {
		switch {
		case p.matchWord("WHEN"):
			objectName, err := p.parseName()
			if err != nil {
				return TypeofSpec{}, err
			}
			if !p.matchWord("THEN") {
				return TypeofSpec{}, p.errorf("expected THEN in TYPEOF")
			}
			fields, err := p.parseTypeofFieldList()
			if err != nil {
				return TypeofSpec{}, err
			}
			spec.When[objectName] = fields
		case p.matchWord("ELSE"):
			fields, err := p.parseTypeofFieldList()
			if err != nil {
				return TypeofSpec{}, err
			}
			spec.Else = fields
		case p.matchWord("END"):
			return spec, nil
		default:
			return TypeofSpec{}, p.errorf("expected WHEN, ELSE, or END in TYPEOF")
		}
	}
}

func (p *parser) parseTypeofFieldList() ([]string, error) {
	var fields []string
	for {
		if p.peek().text == "" || p.peek().text == "," || strings.EqualFold(p.peek().text, "WHEN") || strings.EqualFold(p.peek().text, "ELSE") || strings.EqualFold(p.peek().text, "END") {
			if len(fields) == 0 {
				return nil, p.errorf("TYPEOF branch requires at least one field")
			}
			return fields, nil
		}
		field, err := p.parseName()
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
		if !p.match(",") {
			continue
		}
	}
}

func (p *parser) parseOrderList() ([]OrderSpec, error) {
	var order []OrderSpec
	for {
		field, err := p.parseName()
		if err != nil {
			return nil, err
		}
		if field == "" {
			return nil, p.errorf("expected ORDER BY field")
		}
		spec := OrderSpec{Field: field}
		if p.matchWord("ASC") {
			spec.Desc = false
		} else if p.matchWord("DESC") {
			spec.Desc = true
		}
		if p.matchWord("NULLS") {
			switch {
			case p.matchWord("FIRST"):
				spec.Nulls = "FIRST"
			case p.matchWord("LAST"):
				spec.Nulls = "LAST"
			default:
				return nil, p.errorf("expected FIRST or LAST after NULLS")
			}
		}
		order = append(order, spec)
		if !p.match(",") {
			return order, nil
		}
	}
}

func (p *parser) parseSecurityMode() (string, error) {
	switch {
	case p.matchWord("SECURITY_ENFORCED"):
		return "SECURITY_ENFORCED", nil
	case p.matchWord("USER_MODE"):
		return "USER_MODE", nil
	case p.matchWord("SYSTEM_MODE"):
		return "SYSTEM_MODE", nil
	default:
		return "", p.errorf("expected SECURITY_ENFORCED, USER_MODE, or SYSTEM_MODE after WITH")
	}
}

func isAggregateFunc(name string) bool {
	switch strings.ToUpper(name) {
	case "COUNT", "COUNT_DISTINCT", "SUM", "MIN", "MAX", "AVG", "GROUPING":
		return true
	default:
		return false
	}
}

func aggregateSpecs(fields []string) ([]Aggregate, error) {
	var aggregates []Aggregate
	for _, field := range fields {
		aggregate, ok, err := parseAggregateField(field)
		if err != nil {
			return nil, err
		}
		if ok {
			aggregates = append(aggregates, aggregate)
		}
	}
	return aggregates, nil
}

func validateAggregateQuery(query Query) error {
	if len(query.Aggregates) == 0 && len(query.HavingAggregates) == 0 {
		if len(query.GroupBy) > 0 || query.Having != nil {
			return fmt.Errorf("soql: GROUP BY and HAVING require aggregate fields")
		}
		return nil
	}
	for _, field := range query.Fields {
		aggregate, ok, err := parseAggregateField(field)
		if err != nil {
			return err
		}
		if ok {
			if aggregate.Func == "GROUPING" {
				if len(query.GroupBy) == 0 {
					return fmt.Errorf("soql: GROUPING requires GROUP BY")
				}
				if !containsName(query.GroupBy, aggregate.Field) {
					return fmt.Errorf("soql: GROUPING field %s must be grouped", aggregate.Field)
				}
			}
			continue
		}
		if !containsName(query.GroupBy, field) {
			return fmt.Errorf("soql: field %s must be grouped or aggregated", field)
		}
	}
	return nil
}

func validateAggregateAliases(query Query) error {
	seen := map[string]bool{}
	for _, aggregate := range query.Aggregates {
		if aggregate.Alias == "" {
			continue
		}
		aliasKey := strings.ToLower(aggregate.Alias)
		if seen[aliasKey] {
			return fmt.Errorf("soql: duplicate aggregate alias %s", aggregate.Alias)
		}
		seen[aliasKey] = true
		for exprIndex := range query.Aggregates {
			if strings.EqualFold(aggregate.Alias, fmt.Sprintf("expr%d", exprIndex)) {
				return fmt.Errorf("soql: aggregate alias %s conflicts with generated aggregate field", aggregate.Alias)
			}
		}
		for _, groupField := range query.GroupBy {
			if strings.EqualFold(aggregate.Alias, groupField) {
				return fmt.Errorf("soql: aggregate alias %s conflicts with grouped field %s", aggregate.Alias, groupField)
			}
		}
	}
	return nil
}

func aggregateExprMap(aggregates []Aggregate) map[string]string {
	out := make(map[string]string, len(aggregates))
	for i, aggregate := range aggregates {
		out[aggregateExpression(aggregate)] = fmt.Sprintf("expr%d", i)
		if aggregate.Alias != "" {
			out[aggregate.Alias] = aggregate.Alias
		}
	}
	return out
}

func aggregateExpression(aggregate Aggregate) string {
	if aggregate.Field == "" {
		return aggregate.Func + "()"
	}
	return aggregate.Func + "(" + aggregate.Field + ")"
}

func rewriteHavingAggregates(condition Condition, query *Query) Condition {
	condition = rewriteConditionAggregates(condition, aggregateExprMap(query.Aggregates))
	hidden := make(map[string]string, len(query.HavingAggregates))
	for _, aggregate := range query.HavingAggregates {
		if aggregate.Alias != "" {
			hidden[aggregateExpression(aggregate)] = aggregate.Alias
		}
	}
	return rewriteUnselectedHavingAggregates(condition, &query.HavingAggregates, hidden)
}

func rewriteUnselectedHavingAggregates(condition Condition, aggregates *[]Aggregate, aliases map[string]string) Condition {
	if condition.Field != "" {
		if aggregate, ok, err := parseAggregateField(condition.Field); err == nil && ok {
			expression := aggregateExpression(aggregate)
			alias, ok := aliases[expression]
			if !ok {
				alias = fmt.Sprintf("\x00havingAggregate%d", len(*aggregates))
				aggregate.Alias = alias
				*aggregates = append(*aggregates, aggregate)
				aliases[expression] = alias
			}
			condition.Field = alias
		}
	}
	for i := range condition.And {
		condition.And[i] = rewriteUnselectedHavingAggregates(condition.And[i], aggregates, aliases)
	}
	for i := range condition.Or {
		condition.Or[i] = rewriteUnselectedHavingAggregates(condition.Or[i], aggregates, aliases)
	}
	return condition
}

func rewriteConditionAggregates(condition Condition, aliases map[string]string) Condition {
	if condition.Field != "" {
		if alias, ok := aliases[condition.Field]; ok {
			condition.Field = alias
		}
	}
	for i := range condition.And {
		condition.And[i] = rewriteConditionAggregates(condition.And[i], aliases)
	}
	for i := range condition.Or {
		condition.Or[i] = rewriteConditionAggregates(condition.Or[i], aliases)
	}
	return condition
}

func containsName(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func parseAggregateField(field string) (Aggregate, bool, error) {
	alias := ""
	parts := strings.Fields(field)
	if len(parts) > 2 {
		return Aggregate{}, false, fmt.Errorf("soql: invalid aggregate field %s", field)
	}
	if len(parts) == 2 {
		field = parts[0]
		alias = parts[1]
	}
	open := strings.Index(field, "(")
	if open < 0 || !strings.HasSuffix(field, ")") {
		return Aggregate{}, false, nil
	}
	fn := strings.ToUpper(field[:open])
	if !isAggregateFunc(fn) {
		return Aggregate{}, false, nil
	}
	arg := strings.TrimSpace(field[open+1 : len(field)-1])
	if fn == "GROUPING" {
		if arg == "" {
			return Aggregate{}, false, fmt.Errorf("soql: GROUPING requires a field")
		}
		return Aggregate{Func: fn, Field: arg, Alias: alias}, true, nil
	}
	if fn == "COUNT" && arg == "" {
		return Aggregate{Func: fn, Alias: alias}, true, nil
	}
	if arg == "" {
		return Aggregate{}, false, fmt.Errorf("soql: %s requires a field", fn)
	}
	return Aggregate{Func: fn, Field: arg, Alias: alias}, true, nil
}

func (p *parser) parseOrCondition() (Condition, error) {
	left, err := p.parseAndCondition()
	if err != nil {
		return Condition{}, err
	}
	var ors []Condition
	for p.matchWord("OR") {
		right, err := p.parseAndCondition()
		if err != nil {
			return Condition{}, err
		}
		ors = append(ors, right)
	}
	if len(ors) == 0 {
		return left, nil
	}
	return Condition{Or: append([]Condition{left}, ors...)}, nil
}

func (p *parser) parseAndCondition() (Condition, error) {
	left, err := p.parsePrimaryCondition()
	if err != nil {
		return Condition{}, err
	}
	var ands []Condition
	for p.matchWord("AND") {
		right, err := p.parsePrimaryCondition()
		if err != nil {
			return Condition{}, err
		}
		ands = append(ands, right)
	}
	if len(ands) == 0 {
		return left, nil
	}
	return Condition{And: append([]Condition{left}, ands...)}, nil
}

func (p *parser) parsePrimaryCondition() (Condition, error) {
	if p.matchWord("NOT") {
		cond, err := p.parsePrimaryCondition()
		if err != nil {
			return Condition{}, err
		}
		cond.Not = true
		return cond, nil
	}
	if p.match("(") {
		cond, err := p.parseOrCondition()
		if err != nil {
			return Condition{}, err
		}
		if !p.match(")") {
			return Condition{}, p.errorf("expected ) after condition")
		}
		return cond, nil
	}
	field, err := p.parseConditionField()
	if err != nil {
		return Condition{}, err
	}
	op, err := p.parseOperator()
	if err != nil {
		return Condition{}, err
	}
	if op == "IN" || op == "NOT IN" {
		values, subquery, err := p.parseInOperand()
		if err != nil {
			return Condition{}, err
		}
		return Condition{Field: field, Op: op, Values: values, Subquery: subquery}, nil
	}
	valueToken := p.advance().text
	if valueToken == "" {
		return Condition{}, p.errorf("expected WHERE value")
	}
	value, value2, isRange, err := literalAt(valueToken, p.now)
	if err != nil {
		return Condition{}, err
	}
	return Condition{Field: field, Op: op, Value: value, Value2: value2, Range: isRange}, nil
}

func (p *parser) parseConditionField() (string, error) {
	field, err := p.parseName()
	if err != nil {
		return "", err
	}
	if isAggregateFunc(field) && p.match("(") {
		if strings.EqualFold(field, "COUNT") && p.match(")") {
			return "COUNT()", nil
		}
		arg, err := p.parseName()
		if err != nil {
			return "", err
		}
		if !p.match(")") {
			return "", p.errorf("expected ) after %s(", field)
		}
		return strings.ToUpper(field) + "(" + arg + ")", nil
	}
	return field, nil
}

func (p *parser) parseOperator() (string, error) {
	tok := p.advance().text
	switch tok {
	case "=", "!=", ">", "<", ">=", "<=":
		return tok, nil
	}
	// Word operators
	word := tok
	if strings.EqualFold(word, "LIKE") {
		return "LIKE", nil
	}
	if strings.EqualFold(word, "IN") {
		return "IN", nil
	}
	if strings.EqualFold(word, "NOT") {
		if p.matchWord("IN") {
			return "NOT IN", nil
		}
		if p.matchWord("LIKE") {
			return "NOT LIKE", nil
		}
		return "", p.errorf("expected IN or LIKE after NOT")
	}
	return "", p.errorf("unsupported WHERE operator %q", tok)
}

func (p *parser) parseInOperand() ([]storage.Value, *Query, error) {
	if !p.match("(") {
		return nil, nil, p.errorf("expected ( after IN")
	}
	if strings.EqualFold(p.peek().text, "SELECT") {
		query, err := p.parseQuery()
		if err != nil {
			return nil, nil, err
		}
		if len(query.Fields) != 1 {
			return nil, nil, p.errorf("semi-join subquery must select one field")
		}
		if !p.match(")") {
			return nil, nil, p.errorf("expected ) after semi-join subquery")
		}
		return nil, &query, nil
	}
	var values []storage.Value
	if p.match(")") {
		return values, nil, nil
	}
	for {
		tok := p.advance().text
		if tok == "" {
			return nil, nil, p.errorf("expected value in IN list")
		}
		value, _, isRange, err := literalAt(tok, p.now)
		if err != nil {
			return nil, nil, err
		}
		if isRange {
			return nil, nil, unsupportedSOQLErrorf("date range literal %s is not supported in IN lists", tok)
		}
		values = append(values, value)
		if p.match(")") {
			break
		}
		if !p.match(",") {
			return nil, nil, p.errorf("expected , or ) in IN list")
		}
	}
	return values, nil, nil
}

func (p *parser) parseName() (string, error) {
	name := p.advance().text
	if name == "" {
		return "", p.errorf("expected name")
	}
	for p.match(".") {
		part := p.advance().text
		if part == "" {
			return "", p.errorf("expected name after .")
		}
		name += "." + part
	}
	return name, nil
}

func (p *parser) parseInt() (int, error) {
	text := p.advance().text
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 {
		return 0, p.errorf("expected non-negative integer")
	}
	return value, nil
}

func (p *parser) match(text string) bool {
	if p.peek().text != text {
		return false
	}
	p.pos++
	return true
}

func (p *parser) matchWord(text string) bool {
	if !strings.EqualFold(p.peek().text, text) {
		return false
	}
	p.pos++
	return true
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("soql: "+format, args...)
}

func literalAt(text string, now time.Time) (storage.Value, storage.Value, bool, error) {
	if start, end, ok := dateLiteral(text, now); ok {
		return start, end, true, nil
	}
	if isKnownUnsupportedDateLiteral(text) {
		return storage.Value{}, storage.Value{}, false, unsupportedSOQLErrorf("date literal %s is not supported", strings.ToUpper(text))
	}
	switch {
	case strings.EqualFold(text, "null"):
		return storage.NullValue(), storage.Value{}, false, nil
	case strings.EqualFold(text, "true"):
		return storage.BooleanValue(true), storage.Value{}, false, nil
	case strings.EqualFold(text, "false"):
		return storage.BooleanValue(false), storage.Value{}, false, nil
	case strings.HasPrefix(text, "'") && strings.HasSuffix(text, "'"):
		inner := strings.TrimSuffix(strings.TrimPrefix(text, "'"), "'")
		return storage.StringValue(strings.ReplaceAll(inner, "''", "'")), storage.Value{}, false, nil
	default:
		if t, ok := parseISODate(text); ok {
			return storage.DateValue(t.Format("2006-01-02")), storage.Value{}, false, nil
		}
		value, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return storage.IDValue(storage.ID(text)), storage.Value{}, false, nil
		}
		return storage.IntegerValue(value), storage.Value{}, false, nil
	}
}

func dateLiteral(text string, now time.Time) (storage.Value, storage.Value, bool) {
	today := dateOnly(now)
	upper := strings.ToUpper(text)
	switch upper {
	case "TODAY":
		return dateRange(today, today.AddDate(0, 0, 1))
	case "YESTERDAY":
		return dateRange(today.AddDate(0, 0, -1), today)
	case "TOMORROW":
		return dateRange(today.AddDate(0, 0, 1), today.AddDate(0, 0, 2))
	case "THIS_MONTH":
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		return dateRange(start, start.AddDate(0, 1, 0))
	case "LAST_MONTH":
		thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		return dateRange(thisMonth.AddDate(0, -1, 0), thisMonth)
	case "NEXT_MONTH":
		nextMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
		return dateRange(nextMonth, nextMonth.AddDate(0, 1, 0))
	case "THIS_YEAR":
		start := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return dateRange(start, start.AddDate(1, 0, 0))
	case "LAST_YEAR":
		thisYear := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return dateRange(thisYear.AddDate(-1, 0, 0), thisYear)
	case "NEXT_YEAR":
		nextYear := time.Date(today.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
		return dateRange(nextYear, nextYear.AddDate(1, 0, 0))
	case "THIS_QUARTER":
		start := quarterStart(today)
		return dateRange(start, start.AddDate(0, 3, 0))
	case "LAST_QUARTER":
		start := quarterStart(today).AddDate(0, -3, 0)
		return dateRange(start, start.AddDate(0, 3, 0))
	case "NEXT_QUARTER":
		start := quarterStart(today).AddDate(0, 3, 0)
		return dateRange(start, start.AddDate(0, 3, 0))
	case "LAST_90_DAYS":
		return dateRange(today.AddDate(0, 0, -90), today.AddDate(0, 0, 1))
	case "NEXT_90_DAYS":
		return dateRange(today, today.AddDate(0, 0, 91))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_DAYS:"); ok {
		return dateRange(today.AddDate(0, 0, -n), today.AddDate(0, 0, 1))
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_DAYS:"); ok {
		return dateRange(today, today.AddDate(0, 0, n+1))
	}
	if n, ok := literalNumberSuffix(upper, "N_DAYS_AGO:"); ok {
		start := today.AddDate(0, 0, -n)
		return dateRange(start, start.AddDate(0, 0, 1))
	}
	if n, ok := literalNumberSuffix(upper, "LAST_N_MONTHS:"); ok {
		thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		start := thisMonth.AddDate(0, -n, 0)
		return dateRange(start, thisMonth)
	}
	if n, ok := literalNumberSuffix(upper, "NEXT_N_MONTHS:"); ok {
		nextMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
		return dateRange(nextMonth, nextMonth.AddDate(0, n, 0))
	}
	return storage.Value{}, storage.Value{}, false
}

func isKnownUnsupportedDateLiteral(text string) bool {
	upper := strings.ToUpper(text)
	switch upper {
	case "THIS_WEEK", "LAST_WEEK", "NEXT_WEEK":
		return true
	default:
		return strings.HasPrefix(upper, "LAST_N_WEEKS:") || strings.HasPrefix(upper, "NEXT_N_WEEKS:")
	}
}

func quarterStart(day time.Time) time.Time {
	month := time.Month(((int(day.Month()) - 1) / 3 * 3) + 1)
	return time.Date(day.Year(), month, 1, 0, 0, 0, 0, time.UTC)
}

func literalNumberSuffix(text, prefix string) (int, bool) {
	if !strings.HasPrefix(text, prefix) {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimPrefix(text, prefix))
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func dateOnly(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func dateRange(start, end time.Time) (storage.Value, storage.Value, bool) {
	return storage.DateValue(start.Format("2006-01-02")), storage.DateValue(end.Format("2006-01-02")), true
}

func parseISODate(text string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", text)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
