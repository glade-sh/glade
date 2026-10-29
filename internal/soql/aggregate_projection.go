package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Plain grouping fields remain available to HAVING and ORDER BY, then SELECT
// determines which are exposed. Function-key naming retains its existing
// contract until its populated-map shape is captured separately.
func removeUnselectedGroupFields(fields map[string]storage.Value, query Query, org storage.OrgState) {
	if len(query.GroupBy) == 0 {
		return
	}
	selected := make(map[string]bool, len(query.Fields))
	var selectedFields []string
	for _, field := range query.Fields {
		if expr, ok := parseSelectFieldExpression(field); ok {
			selected[strings.ToLower(expr.outputName())] = true
		} else if _, alias, ok := splitSelectFieldAlias(field); ok {
			selected[strings.ToLower(alias)] = true
		} else {
			selected[strings.ToLower(field)] = true
			selectedFields = append(selectedFields, field)
			if alias := aggregateGroupFieldImplicitAlias(field); alias != "" {
				selected[strings.ToLower(alias)] = true
			}
		}
	}
	for _, aggregate := range query.Aggregates {
		if aggregate.Alias != "" {
			selected[strings.ToLower(aggregate.Alias)] = true
		}
	}
	for _, group := range query.GroupBy {
		if _, function := parseSelectFieldExpression(group); function {
			continue
		}
		keepGroup := selected[strings.ToLower(group)]
		for _, field := range selectedFields {
			// Restore the earlier projection only for a namespace-qualified
			// custom field selected against its simple unqualified group name.
			// The prefix is the current namespace; when the project declares
			// none, it is any single package prefix. Other equivalent spellings
			// have no captured projection evidence.
			if !strings.Contains(group, ".") && !strings.Contains(field, ".") &&
				strings.Count(group, "__") == 1 && strings.HasSuffix(strings.ToLower(group), "__c") &&
				groupSelectedWithNamespace(org.Namespace, field, group) {
				keepGroup = true
				break
			}
		}
		if !keepGroup {
			delete(fields, group)
		}
		if alias := aggregateGroupFieldImplicitAlias(group); alias != "" && !selected[strings.ToLower(alias)] {
			delete(fields, alias)
		}
	}
}

func groupSelectedWithNamespace(namespace, field, group string) bool {
	if namespace != "" {
		return strings.EqualFold(field, namespace+"__"+group)
	}
	if len(field) <= len(group)+2 || !strings.EqualFold(field[len(field)-len(group):], group) {
		return false
	}
	prefix := field[:len(field)-len(group)]
	return strings.HasSuffix(prefix, "__") && len(prefix) > 2 && !strings.Contains(prefix[:len(prefix)-2], "__")
}
