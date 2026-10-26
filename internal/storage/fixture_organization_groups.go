package storage

import "strings"

// Owned setup_object_read captures establish the plain Developer group and
// the additional customer and partner groups exposed by Communities.
func ensureOrganizationGroups(org *OrgState) {
	EnsureStandardObject(org, "Group")
	groups := []struct{ name, kind string }{{"AllInternalUsers", "Organization"}}
	if org.HasFeature("Communities") {
		groups = append(groups,
			struct{ name, kind string }{"AllCustomerPortalUsers", "AllCustomerPortal"},
			struct{ name, kind string }{"AllPartnerUsers", "PRMOrganization"},
		)
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	state := org.Objects["Group"]
	if state.Records == nil {
		state.Records = make(map[ID]Record)
	}
	next := org.IDSequences["Group"]
	for _, seed := range groups {
		found := false
		for _, record := range state.Records {
			if strings.EqualFold(record.Fields["DeveloperName"].String, seed.name) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		mutable, _ := EnsureMutableObjectRecords(org, "Group")
		state = *mutable
		var id ID
		for {
			next++
			id = ID(state.Definition.KeyPrefix + leftPadBase36(next, 12))
			if _, _, used := LookupRecordByID(state.Records, id); !used {
				break
			}
		}
		record := Record{ID: id, Object: "Group", Fields: map[string]Value{
			"DeveloperName": StringValue(seed.name), "Type": StringValue(seed.kind),
		}}
		stampSeedSystemFields(&record)
		state.Records[id] = record
	}
	org.Objects["Group"] = state
	org.IDSequences["Group"] = next
}
