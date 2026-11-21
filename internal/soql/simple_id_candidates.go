package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// simpleIDCandidates retains physical row keys, including distinct keys whose
// stored IDs compare equal. It uses the same comparators as equalValues. The
// ordinary execution path still applies the predicate, permissions and ordering.
func simpleIDCandidates(object storage.ObjectState, where *Condition) ([]string, bool) {
	if where == nil || where.Not || where.Range || where.Formula != nil || where.Subquery != nil ||
		where.RewrittenAggregate || len(where.And) != 0 || len(where.Or) != 0 ||
		where.Op != "=" || !strings.EqualFold(where.Field, "Id") {
		return nil, false
	}
	var target string
	typed := where.Value.Kind == storage.ValueID
	if typed {
		target = string(where.Value.ID)
	} else if where.Value.Kind == storage.ValueString {
		target = where.Value.String
	} else {
		return nil, false
	}
	if len(target) != 15 && len(target) != 18 {
		return nil, false
	}
	keys := make([]string, 0, 1)
	for key, row := range object.Records {
		id := string(row.ID)
		if typed && idTextEqual(id, target) || !typed && idStringEqual(id, target) {
			keys = append(keys, string(key))
		}
	}
	return keys, true
}
