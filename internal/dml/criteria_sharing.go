package dml

import (
	"sort"

	"github.com/glade-sh/glade/internal/storage"
)

// ReconcileCriteriaSharing is the commit-time evaluation of criteria-based
// sharing rules. Native Salesforce grants a matching record to the rule's
// target group when the transaction commits, never inside it: the inserting
// transaction sees no access row and zero Rule share rows, the next
// transaction sees one, and the grant stays. An update into the rule grants at
// its commit; an update out revokes at its commit, so the updating transaction
// still sees the old state (grant observations, APIs 62.0 and 67.0).
// Named tests never commit, so they never reach this path.
//
// Only Case has captured rows. The reconcile adds and removes Rule share rows
// on CaseShare and leaves every other cause alone.
func ReconcileCriteriaSharing(org *storage.OrgState) error {
	if org == nil {
		return nil
	}
	cases, ok := org.Objects["Case"]
	if !ok {
		return nil
	}
	rules := cases.Definition.CriteriaSharingRules
	if len(rules) == 0 && len(org.Objects["CaseShare"].Records) == 0 {
		return nil
	}
	engine := NewEngine(org)
	return engine.reconcileCaseCriteriaShares(cases, rules)
}

// criteriaShareKey identifies a Rule share row by its parent and target group,
// compared on the 15-character Id prefix.
type criteriaShareKey struct {
	parent, group storage.ID
}

func newCriteriaShareKey(parent, group storage.ID) criteriaShareKey {
	return criteriaShareKey{parent: shortShareID(parent), group: shortShareID(group)}
}

func shortShareID(id storage.ID) storage.ID {
	if len(id) > 15 {
		return id[:15]
	}
	return id
}

func (e *Engine) reconcileCaseCriteriaShares(cases storage.ObjectState, rules []storage.CriteriaSharingRule) error {
	const shareName = "CaseShare"
	targets := make(map[string]storage.ID, len(rules))
	for _, rule := range rules {
		if rule.AccessLevel != "Read" {
			continue
		}
		if group := e.directSharingGroup(rule.TargetGroup); group != "" {
			targets[rule.TargetGroup] = group
		}
	}
	wanted := make(map[criteriaShareKey]bool)
	type grant struct{ parent, group storage.ID }
	grants := make(map[criteriaShareKey]grant)
	for id, record := range cases.Records {
		if record.System.IsDeleted {
			continue
		}
		if record.ID != "" {
			id = record.ID
		}
		for _, rule := range rules {
			group, ok := targets[rule.TargetGroup]
			if ok && criteriaRuleMatches(rule, record) {
				key := newCriteriaShareKey(id, group)
				wanted[key] = true
				grants[key] = grant{parent: id, group: group}
			}
		}
	}
	storage.EnsureStandardObject(e.Org, shareName)
	share := e.Org.Objects[shareName]
	var remove []storage.ID
	present := make(map[criteriaShareKey]bool)
	for id, row := range share.Records {
		cause, _ := row.GetField("RowCause")
		if row.System.IsDeleted || cause.String != "Rule" {
			continue
		}
		parent, _ := row.GetField("CaseId")
		recipient, _ := row.GetField("UserOrGroupId")
		key := newCriteriaShareKey(idFromStorageValue(parent), idFromStorageValue(recipient))
		if wanted[key] {
			present[key] = true
			continue
		}
		if row.CriteriaSharingOwned {
			remove = append(remove, id)
		}
	}
	for key := range present {
		delete(wanted, key)
	}
	if len(remove) == 0 && len(wanted) == 0 {
		return nil
	}
	sort.Slice(remove, func(i, j int) bool { return remove[i] < remove[j] })
	add := make([]grant, 0, len(wanted))
	for key := range wanted {
		add = append(add, grants[key])
	}
	sort.Slice(add, func(i, j int) bool {
		if add[i].parent != add[j].parent {
			return add[i].parent < add[j].parent
		}
		return add[i].group < add[j].group
	})
	storage.EnsureMutableObjectRecords(e.Org, shareName)
	share = e.Org.Objects[shareName]
	for _, id := range remove {
		if e.IsolationJournal != nil {
			e.IsolationJournal.RecordUpdate(shareName, id, share.Records[id])
		}
		share.DeleteRecord(id)
	}
	for _, key := range add {
		e.recordJournalSequence(shareName)
		var id storage.ID
		for {
			next, err := e.IDs.Next(shareName)
			if err != nil {
				return err
			}
			id = next
			if _, _, exists := storage.LookupRecordByID(share.Records, id); !exists {
				break
			}
		}
		if e.IsolationJournal != nil {
			e.IsolationJournal.RecordInsert(shareName, id)
		}
		share.PutRecord(id, storage.Record{ID: id, Object: shareName, CriteriaSharingOwned: true, Fields: map[string]storage.Value{
			"CaseId":          storage.IDValue(key.parent),
			"UserOrGroupId":   storage.IDValue(key.group),
			"CaseAccessLevel": storage.StringValue("Read"),
			"RowCause":        storage.StringValue("Rule"),
		}})
	}
	share.Indexes = nil
	e.Org.Objects[shareName] = share
	e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
	return nil
}

// criteriaRuleMatches evaluates only the captured single Subject equality shape.
func criteriaRuleMatches(rule storage.CriteriaSharingRule, record storage.Record) bool {
	if len(rule.Criteria) != 1 || rule.Criteria[0].Field != "Subject" || rule.Criteria[0].Operation != "equals" {
		return false
	}
	value, _ := record.GetField("Subject")
	return value.String == rule.Criteria[0].Value
}
