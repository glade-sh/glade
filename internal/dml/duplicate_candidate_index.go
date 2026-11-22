package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// duplicateCandidateIndex belongs to a single operation. It stores only
// candidate keys, not scorer verdicts or access decisions. Before each use it
// rereads the indexed field on every live record: record tokens alone do not
// cover in-place changes to shared Fields maps.
type duplicateCandidateIndex struct {
	fields   map[duplicateIndexFieldKey]*duplicateIndexedField
	selected []storage.ID
}

type duplicateIndexFieldKey struct {
	object  string
	field   string
	nameKey bool
}

type duplicateIndexedField struct {
	entries  map[storage.ID]duplicateIndexedValue
	groups   map[string]map[storage.ID]struct{}
	unknown  map[storage.ID]struct{}
	disabled bool
}

type duplicateIndexedValue struct {
	raw     string
	key     string
	indexed bool
}

// DuplicateMatchScan reuses candidate keys during one host operation. Its
// callbacks must not mutate the records being scanned. Access decisions and
// scores are evaluated afresh for every call. A scan is not concurrent-safe.
type DuplicateMatchScan struct {
	index duplicateCandidateIndex
	calls int
	busy  bool
}

// Find evaluates the same rule as FindDuplicateMatchesAmong. The first call
// scans directly so a single-record operation does not build an index.
func (scan *DuplicateMatchScan) Find(org *storage.OrgState, rule storage.DuplicateRule, record storage.Record, exclude func(storage.ID) bool, access func(storage.Record) (bool, bool), among map[storage.ID]storage.Record) (DuplicateResult, bool) {
	if scan.busy {
		return findDuplicateMatchesAmong(org, rule, record, exclude, access, among, nil)
	}
	scan.busy = true
	defer func() { scan.busy = false }()
	var index *duplicateCandidateIndex
	if among == nil {
		scan.calls++
		if scan.calls > 1 {
			index = &scan.index
		}
	}
	return findDuplicateMatchesAmong(org, rule, record, exclude, access, among, index)
}

// candidates uses the necessary key in fuzzyLastName, or groups other
// required terms by their actual values and asks the existing scorer about
// each group. Values that are not directly readable as nonblank strings always
// reach the normal scan. The groups are refreshed on every call.
func (index *duplicateCandidateIndex) candidates(object string, prepared *preparedCustomMatch, term int, nameKey bool, records map[storage.ID]storage.Record) []storage.ID {
	if index.fields == nil {
		index.fields = make(map[duplicateIndexFieldKey]*duplicateIndexedField)
	}
	item := prepared.items[term]
	field := item.field
	fieldKey := duplicateIndexFieldKey{object: object, field: field, nameKey: nameKey}
	indexed := index.fields[fieldKey]
	if indexed == nil {
		indexed = &duplicateIndexedField{
			entries: make(map[storage.ID]duplicateIndexedValue, len(records)),
			groups:  make(map[string]map[storage.ID]struct{}),
			unknown: make(map[storage.ID]struct{}),
		}
		index.fields[fieldKey] = indexed
	}
	if indexed.disabled {
		return nil
	}
	for id, record := range records {
		// The normal scan sorts by Record.ID. A malformed map can contain
		// repeated or missing Record IDs under different keys, for which that
		// sort does not define an order. Leave those maps on the scan path.
		if record.ID != id {
			indexed.disabled = true
			indexed.entries = nil
			indexed.groups = nil
			indexed.unknown = nil
			return nil
		}
		value, ok := record.Fields[field]
		usable := ok && value.Kind == storage.ValueString && strings.TrimSpace(value.String) != ""
		previous, exists := indexed.entries[id]
		if exists && previous.indexed == usable && (!usable || previous.raw == value.String) {
			continue
		}
		if exists {
			if previous.indexed {
				group := indexed.groups[previous.key]
				delete(group, id)
				if len(group) == 0 {
					delete(indexed.groups, previous.key)
				}
			} else {
				delete(indexed.unknown, id)
			}
		}
		current := duplicateIndexedValue{indexed: usable}
		if usable {
			current.raw = value.String
			current.key = value.String
			if nameKey {
				current.key = lastNameWordKey(foldText(capFuzzyInput(strings.TrimSpace(value.String))))
			}
			group := indexed.groups[current.key]
			if group == nil {
				// Highly distinct values make a per-value index more expensive
				// than the existing scan; keep that scan for this operation.
				if !nameKey && len(indexed.groups) >= maxDuplicateTermMemoValues {
					indexed.disabled = true
					indexed.entries = nil
					indexed.groups = nil
					indexed.unknown = nil
					return nil
				}
				group = make(map[storage.ID]struct{})
				indexed.groups[current.key] = group
			}
			group[id] = struct{}{}
		} else {
			indexed.unknown[id] = struct{}{}
		}
		indexed.entries[id] = current
	}
	if index.selected == nil {
		index.selected = make([]storage.ID, 0)
	}
	selected := index.selected[:0]
	appendLive := func(ids map[storage.ID]struct{}) {
		for id := range ids {
			if _, ok := records[id]; ok {
				selected = append(selected, id)
			} else {
				delete(ids, id)
				delete(indexed.entries, id)
			}
		}
	}
	if nameKey {
		appendLive(indexed.groups[lastNameWordKey(foldText(item.scored))])
	} else {
		for value, ids := range indexed.groups {
			sample := storage.Record{Fields: map[string]storage.Value{field: storage.StringValue(value)}}
			if matches, _ := prepared.verdict(term, sample); matches {
				appendLive(ids)
			}
		}
	}
	appendLive(indexed.unknown)
	index.selected = selected
	return selected
}
