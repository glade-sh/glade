package soql

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// case332OriginalAggregateRecords retains the original grouping algorithm.
func case332OriginalAggregateRecords(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, query Query) ([]storage.Record, error) {
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
				// R127/N019: grouped dates retain their field type, including
				// records loaded from fixtures that store the date as text.
				if value.Kind == storage.ValueString {
					if definitions, known := fieldDefinitionsForReference(org, definition, field); known && len(definitions) == 1 && definitions[0].Type == storage.FieldDate {
						value = storage.DateValue(value.String)
					}
				}
				values[field] = value
				parts = append(parts, "0:"+aggregateGroupValueKey(org, definition, field, value))
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
			value := group.values[field].Clone()
			fields[field] = value
			if alias := aggregateGroupFieldImplicitAlias(field); alias != "" {
				if _, exists := fields[alias]; !exists {
					fields[alias] = value.Clone()
				}
			}
		}
		for _, field := range query.Fields {
			if expr, ok := parseSelectFieldExpression(field); ok && expr.Alias != "" {
				if value, ok := fields[expr.Raw]; ok {
					fields[expr.Alias] = value.Clone()
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
		if err := addHiddenAggregateFields(fields, org, definition, group.records, query.orderAggregates, group.grouping); err != nil {
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

// Native NPG API62/67/68: a lookup retains one group across equivalent
// 15/18-character Id representations. Keep the projected representative intact.

func TestCase332AggregateAgainstOriginal(t *testing.T) {
	org := storage.NewOrgState()
	def := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
		"Name":   {APIName: "Name", Type: storage.FieldString},
		"Day__c": {APIName: "Day__c", Type: storage.FieldDate},
		"Id":     {APIName: "Id", Type: storage.FieldID},
	}}
	records := []storage.Record{
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("A"), "Day__c": storage.StringValue("2020-01-01")}},
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("A"), "Day__c": storage.StringValue("2020-01-01")}},
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.NullValue(), "Day__c": storage.StringValue("2020-02-01")}},
	}
	for _, mode := range []string{"", "ROLLUP", "CUBE"} {
		for _, fields := range [][]string{{"Name"}, {"Name", "Day__c"}, {"Name", "Name"}} {
			query := Query{GroupBy: fields, GroupMode: mode}
			got, gotErr := aggregateRecords(org, def, records, query)
			want, wantErr := case332OriginalAggregateRecords(org, def, records, query)
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("mode %q fields %v: got %#v %v want %#v %v", mode, fields, got, gotErr, want, wantErr)
			}
		}
	}
}
