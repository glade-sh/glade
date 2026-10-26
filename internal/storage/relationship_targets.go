package storage

import (
	"slices"
	"strings"

	"github.com/glade-sh/glade/internal/apexlang"
)

const (
	parentExact uint8 = 1 << iota
	parentAnyNamespace
	parentStandardNamespace
)

type relationshipParentLookup struct {
	parents []string
	source  []string
	keys    map[string]uint8
	ascii   bool
}

// PrepareRelationshipTargets derives immutable lookup keys when definitions are
// loaded or reinstalled. Copy the relationship slice before attaching metadata:
// definitions may still share their input slices with a frozen runtime template.
func PrepareRelationshipTargets(definition *ObjectDefinition) {
	if definition == nil {
		return
	}
	copied := false
	for i := range definition.Relations {
		relation := &definition.Relations[i]
		if relation.parentLookup != nil && sameRelationshipParentSlice(relation.ParentObjects, relation.parentLookup.parents) && slices.Equal(relation.ParentObjects, relation.parentLookup.source) {
			continue
		}
		if !copied {
			definition.Relations = slices.Clone(definition.Relations)
			copied = true
			relation = &definition.Relations[i]
		}
		lookup := &relationshipParentLookup{parents: relation.ParentObjects, source: slices.Clone(relation.ParentObjects), keys: make(map[string]uint8), ascii: true}
		for _, parent := range relation.ParentObjects {
			parent = strings.TrimSpace(parent)
			if parent == "" {
				continue
			}
			if !relationshipNameASCII(parent) {
				lookup.ascii = false
				continue
			}
			parent = strings.ToLower(parent)
			lookup.keys[parent] |= parentExact
			lookup.keys[stripRelationshipAnyNamespace(parent)] |= parentAnyNamespace
			lookup.keys[stripRelationshipStandardNamespace(parent)] |= parentStandardNamespace
		}
		relation.parentLookup = lookup
	}
}

// PreparedParentObjectMatch reports both the match and whether derived metadata
// can answer it. Unicode names retain the caller's EqualFold path; lowercasing
// alone does not preserve all of EqualFold's equivalence classes.
func (r Relationship) PreparedParentObjectMatch(objectName string) (match, prepared bool) {
	lookup := r.parentLookup
	if lookup == nil || !lookup.ascii || !sameRelationshipParentSlice(r.ParentObjects, lookup.parents) {
		return false, false
	}
	objectName = strings.TrimSpace(objectName)
	if !relationshipNameASCII(objectName) {
		return false, false
	}
	if objectName == "" {
		return false, true
	}
	key := apexlang.LowerName(objectName)
	if lookup.keys[key]&parentExact != 0 {
		return true, true
	}
	if lookup.keys[stripRelationshipAnyNamespace(key)]&parentAnyNamespace != 0 {
		return true, true
	}
	return lookup.keys[stripRelationshipStandardNamespace(key)]&parentStandardNamespace != 0, true
}

func relationshipNameASCII(name string) bool {
	for i := 0; i < len(name); i++ {
		if name[i] >= 0x80 {
			return false
		}
	}
	return true
}

func stripRelationshipAnyNamespace(name string) string {
	first := strings.Index(name, "__")
	if first <= 0 || first+2 >= len(name) {
		return name
	}
	rest := name[first+2:]
	if strings.Contains(rest, "__") {
		return rest
	}
	return name
}

func stripRelationshipStandardNamespace(name string) string {
	first := strings.Index(name, "__")
	if first <= 0 || first+2 >= len(name) {
		return name
	}
	rest := name[first+2:]
	if strings.Contains(rest, "__") {
		return stripRelationshipAnyNamespace(name)
	}
	switch rest {
	case "c", "r", "mdt", "e", "b", "x", "kav", "ka":
		return name
	}
	return rest
}

// A copied or replaced parent slice must be prepared before sharing its keys.
// Definitions with in-place edits are refreshed at their load/install boundary.
func sameRelationshipParentSlice(a, b []string) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}
