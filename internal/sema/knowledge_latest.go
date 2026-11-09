package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func knowledgeLatestWithoutArchived(query soql.Query) bool {
	if query.Where == nil || !strings.EqualFold(query.Object, "Knowledge__kav") {
		return false
	}
	latestFalse, archived, complex := false, false, false
	var visit func(soql.Condition)
	visit = func(c soql.Condition) {
		if c.Not || len(c.Or) > 0 {
			complex = true
		}
		if !c.Not && c.Op == "=" {
			if strings.EqualFold(c.Field, "IsLatestVersion") && c.Value.Kind == storage.ValueBoolean && !c.Value.Boolean {
				latestFalse = true
			}
			if strings.EqualFold(c.Field, "PublishStatus") && c.Value.Kind == storage.ValueString && strings.EqualFold(c.Value.String, "Archived") {
				archived = true
			}
		}
		for _, x := range c.And {
			visit(x)
		}
		for _, x := range c.Or {
			visit(x)
		}
	}
	visit(*query.Where)
	return latestFalse && !archived && !complex
}
