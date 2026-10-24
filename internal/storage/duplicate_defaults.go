package storage

import (
	"fmt"
	"sort"
	"strings"
)

// StandardMatchEngine is the MatchingRule.MatchEngine value native reports
// for the builtin Account, Contact and Lead matching rules (duplicate
// enforcement ST02-ST05 and ST10).
const StandardMatchEngine = "FuzzyMatchEngine"

type standardDuplicateRuleSeed struct {
	object, name, alert string
	active              bool
	order               int
	matchObject, match  string
}

// Native default rules from a fresh scratch org (duplicate enforcement MD01
// and ST01): the same-object rules are active and the cross-object
// Contact/Lead rules exist inactive. The first two keep their earlier IDs.
var standardDuplicateRuleSeeds = []standardDuplicateRuleSeed{
	{object: "Contact", name: "Standard_Contact_Duplicate_Rule", alert: "Duplicate Alert", active: true, order: 1, matchObject: "Contact", match: "Standard_Contact_Match_Rule_v1_1"},
	{object: "Contact", name: "Standard_Rule_for_Contacts_with_Duplicate_Leads", alert: "Use one of these records?", order: 2, matchObject: "Lead", match: "Standard_Lead_Match_Rule_v1_0"},
	{object: "Account", name: "Standard_Account_Duplicate_Rule", alert: "Duplicate Alert", active: true, order: 1, matchObject: "Account", match: "Standard_Account_Match_Rule_v1_0"},
	{object: "Lead", name: "Standard_Lead_Duplicate_Rule", alert: "Duplicate Alert", active: true, order: 1, matchObject: "Lead", match: "Standard_Lead_Match_Rule_v1_0"},
	{object: "Lead", name: "Standard_Rule_for_Leads_with_Duplicate_Contacts", alert: "Use one of these records?", order: 2, matchObject: "Contact", match: "Standard_Contact_Match_Rule_v1_1"},
}

// Native ST01/ST10: one active builtin matching rule per object, retrieved
// with no MatchingRuleItem rows and a null BooleanFilter.
var standardMatchingRuleSeeds = [][2]string{
	{"Account", "Standard_Account_Match_Rule_v1_0"},
	{"Contact", "Standard_Contact_Match_Rule_v1_1"},
	{"Lead", "Standard_Lead_Match_Rule_v1_0"},
}

// IsStandardMatchingRule reports the builtin matching rules whose items are
// not retrievable and whose matching is evaluated by the runtime.
func IsStandardMatchingRule(object, name string) bool {
	for _, seed := range standardMatchingRuleSeeds {
		if strings.EqualFold(seed[0], object) && strings.EqualFold(seed[1], name) {
			return true
		}
	}
	return false
}

func standardDuplicateRule(seed standardDuplicateRuleSeed, active bool) DuplicateRule {
	return DuplicateRule{
		Object: seed.object, Name: seed.name, Active: active,
		ActionOnInsert: "Allow", ActionOnUpdate: "Allow",
		OperationsOnInsert: []string{"Alert", "Report"}, OperationsOnUpdate: []string{"Report"},
		AlertText: seed.alert, SecurityOption: "EnforceSharingRules", SortOrder: seed.order,
		MatchRules: []MatchingRule{{Object: seed.matchObject, Name: seed.match, Status: "Active"}},
	}
}

// EnsureDefaultDuplicateRules seeds the native default DuplicateRule and
// MatchingRule setup rows for the default local org. Supplied rule rows and
// nondefault org identities remain authoritative.
func EnsureDefaultDuplicateRules(org *OrgState) {
	if org == nil || (org.OrgID != "" && !IDsEqual(ID(org.OrgID), DefaultOrgID)) {
		return
	}
	if len(org.Objects["DuplicateRule"].Records) == 0 {
		EnsureStandardObject(org, "DuplicateRule")
		prefix := org.Objects["DuplicateRule"].Definition.KeyPrefix
		for i, seed := range standardDuplicateRuleSeeds {
			id := ID(fmt.Sprintf("%s%012d", prefix, i+1))
			putSeedRecord(org, "DuplicateRule", Record{ID: id, Object: "DuplicateRule", Fields: map[string]Value{
				"DeveloperName": StringValue(seed.name), "SobjectType": StringValue(seed.object), "IsActive": BooleanValue(seed.active),
			}})
		}
	}
	if len(org.Objects["MatchingRule"].Records) == 0 {
		EnsureStandardObject(org, "MatchingRule")
		prefix := org.Objects["MatchingRule"].Definition.KeyPrefix
		for i, seed := range standardMatchingRuleSeeds {
			id := ID(fmt.Sprintf("%s%012d", prefix, i+1))
			putSeedRecord(org, "MatchingRule", Record{ID: id, Object: "MatchingRule", Fields: map[string]Value{
				"DeveloperName": StringValue(seed[1]), "SobjectType": StringValue(seed[0]),
				"RuleStatus": StringValue("Active"), "MatchEngine": StringValue(StandardMatchEngine),
			}})
		}
	}
}

func ensureDefaultDuplicateRules(org *OrgState) {
	EnsureDefaultDuplicateRules(org)
	ensureDefaultDuplicateRuleDefinitions(org)
}

// ensureDefaultDuplicateRuleDefinitions attaches the enforced definition of
// each seeded default rule row that project metadata did not redefine.
func ensureDefaultDuplicateRuleDefinitions(org *OrgState) {
	if org == nil {
		return
	}
	rows := org.Objects["DuplicateRule"].Records
	changed := false
	for _, seed := range standardDuplicateRuleSeeds {
		var rowID ID
		var row Record
		for id, candidate := range rows {
			if candidate.System.IsDeleted || !strings.EqualFold(candidate.Fields["DeveloperName"].String, seed.name) ||
				!strings.EqualFold(candidate.Fields["SobjectType"].String, seed.object) || candidate.Fields["SobjectSubtype"].String != "" {
				continue
			}
			if rowID == "" || id < rowID {
				rowID, row = id, candidate
			}
		}
		state, exists := org.Objects[seed.object]
		if rowID == "" || !exists {
			continue
		}
		defined := false
		for _, rule := range state.Definition.DuplicateRules {
			if strings.EqualFold(rule.Name, seed.name) && rule.Subtype == "" {
				defined = true
				break
			}
		}
		if defined {
			continue
		}
		active := row.Fields["IsActive"]
		rule := standardDuplicateRule(seed, active.Kind == ValueBoolean && active.Boolean)
		rule.ID = rowID
		rules := append(CloneDuplicateRules(state.Definition.DuplicateRules), rule)
		sort.SliceStable(rules, func(i, j int) bool {
			if rules[i].SortOrder != rules[j].SortOrder {
				return rules[i].SortOrder < rules[j].SortOrder
			}
			return rules[i].Name < rules[j].Name
		})
		state.Definition.DuplicateRules = rules
		org.Objects[seed.object] = state
		changed = true
	}
	if changed {
		org.ClearRuntimeSchemaStamp()
	}
}
