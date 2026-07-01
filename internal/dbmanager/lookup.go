package dbmanager

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type LookupOptions struct {
	Object string
	Field  string
	Query  string
	Limit  int
}

type LookupResult struct {
	Records []LookupRow `json:"records"`
}

type LookupRow struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Object   string `json:"object"`
	Subtitle string `json:"subtitle,omitempty"`
}

func (m Manager) Lookup(opts LookupOptions) (LookupResult, error) {
	resolved, object, ok := m.object(opts.Object)
	if !ok {
		return LookupResult{}, fmt.Errorf("unknown object %s", opts.Object)
	}
	fieldName, ok := storage.ResolveFieldName(object.Definition, m.Org.Namespace, opts.Field)
	if !ok {
		return LookupResult{}, fmt.Errorf("unknown field %s", opts.Field)
	}
	field := object.Definition.Fields[fieldName]
	if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
		return LookupResult{}, fmt.Errorf("field %s.%s is not a lookup", resolved, fieldName)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	query := strings.ToLower(strings.TrimSpace(opts.Query))
	out := LookupResult{Records: make([]LookupRow, 0, limit)}
	for _, target := range field.ReferenceTo {
		list, err := m.ListRecords(target, ListRecordsOptions{Query: query, Limit: limit})
		if err != nil {
			continue
		}
		for _, row := range list.Records {
			out.Records = append(out.Records, LookupRow{ID: row.ID, Title: row.Title, Object: row.Object, Subtitle: row.Object})
			if len(out.Records) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}
