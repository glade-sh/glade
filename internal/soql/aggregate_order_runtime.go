package soql

import "fmt"

// Keep the parsed query and its projection intact. A grouped query can sort by
// an aggregate that it does not select, just as HAVING can use an unselected
// aggregate. Unlike HAVING values, sort values must survive until after sorting.
func prepareAggregateOrder(query Query) (Query, error) {
	if len(query.GroupBy) == 0 || len(query.Order) == 0 {
		return query, nil
	}
	query.Order = append([]OrderSpec(nil), query.Order...)
	query.orderAggregates = nil
	aliases := map[string]string{}
	for i := range query.Order {
		aggregate, ok, err := parseAggregateField(query.Order[i].Field)
		if err != nil {
			return Query{}, err
		}
		if !ok {
			continue
		}
		expression := aggregateExpression(aggregate)
		alias, exists := aliases[expression]
		if !exists {
			alias = fmt.Sprintf("\x00orderAggregate%d", len(query.orderAggregates))
			aggregate.Alias = alias
			query.orderAggregates = append(query.orderAggregates, aggregate)
			aliases[expression] = alias
		}
		query.Order[i].Field = alias
		query.Order[i].RewrittenAggregate = true
	}
	// Apply the same grouping contracts to selected and sort-only aggregates.
	validation := query
	validation.Aggregates = append(append([]Aggregate(nil), query.Aggregates...), query.orderAggregates...)
	if err := validateAggregateQuery(validation); err != nil {
		return Query{}, err
	}
	query.OrderBy = query.Order[0].Field
	return query, nil
}
