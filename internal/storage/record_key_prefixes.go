package storage

import (
	"sort"
	"strings"
)

// AdoptRecordKeyPrefixes aligns custom object key prefixes with the record IDs
// the org already holds. Salesforce assigns custom object key prefixes per
// org, so records imported with their real IDs can carry a prefix that the
// local schema gave to another object. Every ID-to-object lookup (LDS record
// reads, Id.getSObjectType, polymorphic references) then resolves to the wrong
// object.
//
// An object adopts a prefix only when all of its records share that one
// prefix, no standard object owns it, and no other object holds records under
// it. An object that loses its local prefix to an adopter receives a fresh
// unused one. It reports whether any definition changed.
func AdoptRecordKeyPrefixes(org *OrgState) bool {
	if org == nil || len(org.Objects) == 0 {
		return false
	}
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)

	// Prefixes each object's records actually use.
	recordPrefixes := make(map[string]map[string]bool, len(names))
	holders := make(map[string][]string)
	for _, name := range names {
		prefixes := make(map[string]bool)
		for id := range org.Objects[name].Records {
			text := string(id)
			if len(text) < 3 {
				continue
			}
			prefixes[text[:3]] = true
		}
		recordPrefixes[name] = prefixes
		for prefix := range prefixes {
			holders[prefix] = append(holders[prefix], name)
		}
	}

	adopt := make(map[string]string)
	claimed := make(map[string]int)
	for _, name := range names {
		if standardKeyPrefixForObject(name) != "" {
			continue
		}
		prefixes := recordPrefixes[name]
		if len(prefixes) != 1 {
			continue
		}
		var prefix string
		for candidate := range prefixes {
			prefix = candidate
		}
		if _, valid := keyPrefixBitsetIndex(prefix); !valid {
			continue
		}
		if strings.TrimSpace(org.Objects[name].Definition.KeyPrefix) == prefix {
			continue
		}
		if standardKeyPrefixOwner(prefix) != "" || len(holders[prefix]) != 1 {
			continue
		}
		adopt[name] = prefix
		claimed[prefix]++
	}
	if len(adopt) == 0 {
		return false
	}

	adopted := make(map[string]bool, len(adopt))
	for name, prefix := range adopt {
		if claimed[prefix] != 1 {
			continue
		}
		state := org.Objects[name]
		state.Definition.KeyPrefix = prefix
		org.Objects[name] = state
		adopted[prefix] = true
	}
	if len(adopted) == 0 {
		return false
	}
	// Objects that held an adopted prefix locally have no records under it
	// (checked above), so they can move to a fresh prefix.
	for _, name := range names {
		state := org.Objects[name]
		prefix := strings.TrimSpace(state.Definition.KeyPrefix)
		if !adopted[prefix] || adopt[name] == prefix {
			continue
		}
		state.Definition.KeyPrefix = ""
		org.Objects[name] = state
	}
	org.keyPrefixesValidated = false
	EnsureUniqueKeyPrefixes(org)
	org.ClearRuntimeSchemaStamp()
	return true
}
