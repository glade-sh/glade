package schema

import "strings"

// memberIndex is scoped to schema assembly while the caller controls mutations.
// Non-ASCII names require EqualFold's full scan, including cross-ASCII folds.
type memberIndex struct {
	first    map[string]int
	fallback bool
}

func (m *memberIndex) observe(name string, position int) {
	if m.fallback {
		return
	}
	for i := 0; i < len(name); i++ {
		if name[i] >= 0x80 {
			m.fallback = true
			m.first = nil
			return
		}
	}
	if m.first == nil {
		m.first = make(map[string]int)
	}
	key := strings.ToLower(name)
	if _, ok := m.first[key]; !ok {
		m.first[key] = position
	}
}
func (m *memberIndex) find(name string, scan func() int) int {
	if m.fallback {
		return scan()
	}
	for i := 0; i < len(name); i++ {
		if name[i] >= 0x80 {
			return scan()
		}
	}
	if i, ok := m.first[strings.ToLower(name)]; ok {
		return i
	}
	return -1
}

type memberIndexes struct{ fields, records, rules memberIndex }

func newMemberIndexes(object *Object) *memberIndexes {
	index := &memberIndexes{}
	for i, f := range object.Fields {
		index.fields.observe(f.Name, i)
	}
	for i, r := range object.RecordTypes {
		index.records.observe(r.DeveloperName, i)
	}
	for i, r := range object.ValidationRules {
		index.rules.observe(r.Name, i)
	}
	return index
}
