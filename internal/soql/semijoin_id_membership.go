package soql

import "github.com/glade-sh/glade/internal/storage"

// Keep the scan as the oracle for small or mixed-kind subquery results.
// The index belongs to one already-resolved predicate in one execution.
type semiJoinIDMembership struct {
	exact        map[storage.ID]struct{}
	longPrefixes map[storage.ID]struct{}
}

func buildSemiJoinIDMembership(values []storage.Value) *semiJoinIDMembership {
	if len(values) < 16 {
		return nil
	}
	for _, v := range values {
		if v.Kind != storage.ValueID {
			return nil
		}
	}
	m := &semiJoinIDMembership{exact: make(map[storage.ID]struct{})}
	for _, v := range values {
		m.exact[v.ID] = struct{}{}
		if len(v.ID) == 18 {
			if m.longPrefixes == nil {
				m.longPrefixes = make(map[storage.ID]struct{})
			}
			m.longPrefixes[v.ID[:15]] = struct{}{}
		}
	}
	return m
}

func (m *semiJoinIDMembership) contains(id storage.ID) bool {
	if _, ok := m.exact[id]; ok {
		return true
	}
	// idTextEqual treats unequal 18-byte IDs as distinct even when the
	// prefixes agree. Opposite-width equality uses a case-sensitive prefix.
	switch len(id) {
	case 15:
		_, ok := m.longPrefixes[id]
		return ok
	case 18:
		_, ok := m.exact[id[:15]]
		return ok
	default:
		return false
	}
}
