package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/soql"
)

// CD01/CD02/CASE_*/PRIVATE/ZERO/TWO/FIELDS and CD03, both native routes at
// 62/67/68. Keep the platform edges separate from declared lookup relationships.
func (c querySemanticsChecker) contentChildObjectForRelationship(parent schema.Object, child soql.ChildQuery) (schema.Object, bool) {
	if parent.CustomSettingsType != "" || !strings.HasSuffix(strings.ToLower(parent.Name), "__c") {
		return schema.Object{}, false
	}
	switch {
	case strings.EqualFold(child.Relationship, "ContentDocumentLinks"):
		return c.object("ContentDocumentLink")
	case strings.EqualFold(child.Relationship, "AttachedContentDocuments") && soql.IsContentChildAttachedIDQuery(child.Query):
		// CD03 captures only the Id projection, without child filters or windows.
		return c.object("AttachedContentDocument")
	default:
		return schema.Object{}, false
	}
}
