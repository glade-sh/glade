package soql

import "strings"

// IsContentChildAttachedIDQuery limits the custom-object sibling view to CD03's
// captured projection. Other AttachedContentDocument shapes keep their old path.
func IsContentChildAttachedIDQuery(query Query) bool {
	return len(query.Fields) == 1 && strings.EqualFold(query.Fields[0], "Id") &&
		query.Where == nil && query.Having == nil && len(query.Order) == 0 && query.OrderBy == "" &&
		!query.HasLimit && query.LimitBind == "" && !query.HasOffset && query.OffsetBind == "" &&
		!query.Count && len(query.Aggregates) == 0 && len(query.HavingAggregates) == 0 &&
		len(query.GroupBy) == 0 && query.GroupMode == "" && len(query.ChildQueries) == 0 && len(query.Typeofs) == 0 &&
		!query.ForUpdate && !query.ForView && !query.ForReference && !query.AllRows && query.UsingScope == "" && query.SecurityMode == ""
}
