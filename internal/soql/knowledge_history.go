package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Archived-version queries implicitly select the latest version unless the
// caller selects a specific record or asks explicitly for version history.
func knowledgeArchivedLatestFilter(object string, where *Condition) bool {
	if where == nil || !strings.EqualFold(object, "Knowledge__kav") {
		return false
	}
	archived, explicit, complex := false, false, false
	var visit func(Condition)
	visit = func(c Condition) {
		if c.Not || len(c.Or) > 0 {
			complex = true
		}
		field := c.Field
		if i := strings.LastIndex(field, "."); i >= 0 {
			field = field[i+1:]
		}
		if strings.EqualFold(field, "Id") || strings.EqualFold(field, "IsLatestVersion") {
			explicit = true
		}
		if strings.EqualFold(field, "PublishStatus") && c.Op == "=" && c.Value.Kind == storage.ValueString && strings.EqualFold(c.Value.String, "Archived") {
			archived = true
		}
		for _, child := range c.And {
			visit(child)
		}
		for _, child := range c.Or {
			visit(child)
		}
	}
	visit(*where)
	return archived && !explicit && !complex
}
