package dml

import (
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// DuplicateMatch is one matching rule's result inside a DuplicateResult.
// Records are snapshots of the matched rows at detection time.
type DuplicateMatch struct {
	Object  string
	Rule    string
	Engine  string
	Records []storage.Record
	// Confidences holds each record's match confidence (Datacloud
	// MatchRecord.matchConfidence), parallel to Records.
	Confidences []float64
	// FieldDiffs holds each record's compared-field differences (Datacloud
	// MatchRecord.getFieldDiffs), parallel to Records. The matcher leaves it
	// nil; AttachFieldDiffs fills it for a result that is reported. Nil also
	// means the rule's field set is not pinned by a native row.
	FieldDiffs [][]DuplicateFieldDiff
}

// DuplicateFieldDiff is one Datacloud.FieldDiff: the field and whether the
// candidate's and the matched record's values are Same, Different or Null.
type DuplicateFieldDiff struct {
	Name       string
	Difference string
}

// byMatchCreation orders members by creation, with the ID breaking audit-time ties.
type byMatchCreation struct{ m *DuplicateMatch }

func (b byMatchCreation) Len() int { return len(b.m.Records) }
func (b byMatchCreation) Less(i, j int) bool {
	a, z := b.m.Records[i], b.m.Records[j]
	if a.System.CreatedDate != z.System.CreatedDate {
		at, ae := time.Parse(time.RFC3339Nano, a.System.CreatedDate)
		zt, ze := time.Parse(time.RFC3339Nano, z.System.CreatedDate)
		if ae == nil && ze == nil {
			if !at.Equal(zt) {
				return at.Before(zt)
			}
		} else {
			return a.System.CreatedDate < z.System.CreatedDate
		}
	}
	return a.ID < z.ID
}
func (b byMatchCreation) Swap(i, j int) {
	b.m.Records[i], b.m.Records[j] = b.m.Records[j], b.m.Records[i]
	b.m.Confidences[i], b.m.Confidences[j] = b.m.Confidences[j], b.m.Confidences[i]
}

// DuplicateResult mirrors Datacloud.DuplicateResult for one duplicate rule.
type DuplicateResult struct {
	// Detected remains true when a Bypass rule finds only undisclosed rows.
	Detected  bool
	Rule      string
	RuleID    storage.ID
	Object    string
	AllowSave bool
	Message   string
	Matches   []DuplicateMatch
}

// DuplicateReport is a saved duplicate that an Allow+Report rule records once
// its transaction commits (native CM01: one set and one item per record).
type DuplicateReport struct {
	Rule     storage.DuplicateRule
	Object   string
	RecordID storage.ID
	Matches  []storage.Record
}

type duplicateState struct {
	stamp *string
	batch map[storage.ID]bool
	peers []duplicatePeer // records inserted by this statement, in insert order
	// peerView holds the current stored state of the peers, keyed by id, for
	// the peer scan of this statement.
	peerView map[storage.ID]storage.Record
	reports  []DuplicateReport
	// insertMatched is set while an insert is checked when a rule found a
	// match for it (the record is inserted as an allowed duplicate).
	insertMatched bool
}

// duplicatePeer names a record inserted earlier by the current statement.
type duplicatePeer struct {
	object string
	id     storage.ID
}

// exactMatchEngine names custom matching rules whose items are all Exact.
// Native rows observe only the builtin engine name (ST02-ST05).
const exactMatchEngine = "ExactMatchEngine"

func duplicateOperationsInclude(operations []string, want string) bool {
	for _, operation := range operations {
		if strings.EqualFold(strings.TrimSpace(operation), want) {
			return true
		}
	}
	return false
}

// DuplicateRuleApplies evaluates a rule's record filter against the record.
// supported is false for filters outside the evaluated criteria shapes.
func DuplicateRuleApplies(org *storage.OrgState, rule storage.DuplicateRule, record storage.Record) (applies, supported bool) {
	if len(rule.Filter.Items) == 0 {
		return true, true
	}
	var definition storage.ObjectDefinition
	namespace := ""
	if org != nil {
		if name, ok := storage.ResolveObjectName(*org, record.Object); ok {
			definition = org.Objects[name].Definition
		}
		namespace = org.Namespace
	}
	items := append([]storage.DuplicateRuleFilterItem(nil), rule.Filter.Items...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].SortOrder < items[j].SortOrder })
	matches := make([]bool, len(items))
	for i, item := range items {
		matched, ok := evaluateWorkflowCriteria(storage.WorkflowCriteriaItem{Field: item.Field, Operation: item.Operation, Value: item.Value}, record, definition, namespace)
		if !ok {
			return false, false
		}
		matches[i] = matched
	}
	return evaluateLookupFilterLogic(rule.Filter.BooleanFilter, matches)
}

func duplicateBlank(value storage.Value, ok bool) bool {
	return !ok || value.Kind == storage.ValueNull || strings.TrimSpace(workflowValueString(value)) == ""
}

func duplicateFieldsEqual(left, right storage.Record, field string) bool {
	l, lok := left.GetField(field)
	r, rok := right.GetField(field)
	if duplicateBlank(l, lok) || duplicateBlank(r, rok) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(workflowValueString(l)), strings.TrimSpace(workflowValueString(r)))
}

// duplicateFieldText returns the trimmed comparison text of a discriminator,
// or false when the field is blank for duplicate matching.
func duplicateFieldText(record storage.Record, field string) (string, bool) {
	value, ok := record.GetField(field)
	if duplicateBlank(value, ok) {
		return "", false
	}
	return strings.TrimSpace(workflowValueString(value)), true
}

// standardDuplicateMatcher returns the pair scorer of a builtin matching rule
// for a candidate record, or nil when the candidate cannot match any record
// (no last name). ruleObject is the matching rule's object, which is not the
// candidate's object for a cross-object rule: it applies the matching rule's
// structure to the candidate's own fields. The scorer reports the verdict and
// the match confidence (standard_rules.go).
func standardDuplicateMatcher(org *storage.OrgState, ruleObject, candidateObject string, candidate storage.Record) func(existing storage.Record) (bool, float64) {
	switch strings.ToLower(ruleObject) {
	case "account":
		prepared, ok := newStandardAccountCandidate(candidate)
		if !ok {
			return nil
		}
		return func(existing storage.Record) (bool, float64) { return standardAccountMatch(prepared, existing) }
	case "contact", "lead":
		person := standardPersonOf(org, candidateObject, candidate)
		object := strings.ToLower(ruleObject)
		// Only the email path, keyed on the address alone, reaches a record
		// whose surname key differs or a candidate without a last name.
		emailPath := person.kind == object && person.email != ""
		if person.last == "" && (!emailPath || person.first == "") {
			return nil
		}
		if person.last != "" {
			person.lastKey = rawKeysOf(person.last).last
		}
		return func(existing storage.Record) (bool, float64) {
			// The last name gates the pair; the other fields are read only after it.
			last := ""
			if v, ok := existing.Fields["LastName"]; ok {
				last = strings.TrimSpace(workflowValueString(v))
			} else if v, ok := existing.GetField("LastName"); ok {
				last = strings.TrimSpace(workflowValueString(v))
			}
			if last == "" && person.last != "" {
				return false, 0
			}
			if person.last == "" || rawKeysOf(last).last != person.lastKey {
				if !emailPath {
					return false, 0
				}
				email, _ := duplicateFieldText(existing, "Email")
				if !strings.EqualFold(person.email, email) {
					return false, 0
				}
			}
			return standardPersonMatch(object, person, standardPersonOf(org, ruleObject, existing))
		}
	}
	return nil
}

// customDuplicateMatch evaluates a custom matching rule's items. Exact items
// compare text without letter case (DR10); fuzzy items score the pair with the
// field-specific method. Blank handling is shared: MatchBlanks pairs two blank
// values, NullNotAllowed never does, and one blank side never matches.
//
// The returned confidence is the matching rule's match confidence: the filter
// logic's mean of means (duplicate_filter_confidence.go). An exact or blank
// match scores 100, a fuzzy item its method score; an item that did not match
// still counts with its raw score. keys is the key of each item, a property
// of the rule (customRuleKeyModes).
func customDuplicateMatch(rule storage.MatchingRule, keys []fuzzyKeyMode, candidate, existing storage.Record) (matched, supported bool, confidence float64) {
	matches := make([]bool, len(rule.Items))
	scores := make([]float64, len(rule.Items))
	for i, item := range rule.Items {
		left, lok := candidate.GetField(item.Field)
		right, rok := existing.GetField(item.Field)
		leftBlank, rightBlank := duplicateBlank(left, lok), duplicateBlank(right, rok)
		matchBlanks := strings.EqualFold(item.BlankBehavior, "MatchBlanks")
		switch {
		case leftBlank || rightBlank:
			// DR09: MatchBlanks pairs two blank keys; NullNotAllowed never does.
			matches[i] = leftBlank && rightBlank && matchBlanks
			scores[i] = 100
			if !matches[i] {
				scores[i] = 50
			}
		case !strings.EqualFold(item.Method, "Exact"):
			var score float64
			l, r := workflowValueString(left), workflowValueString(right)
			if i < len(keys) && keys[i] == fuzzyKeyFirstInitial {
				matches[i], score = firstNameInitialKeyed(l, r)
			} else {
				matches[i], score = FuzzyScore(item.Method, l, r, matchBlanks)
			}
			if matches[i] {
				scores[i] = fuzzyItemScore(item.Method, score)
			} else {
				scores[i] = FuzzyRawConfidence(item.Method, l, r, matchBlanks)
			}
		default:
			// DR10: Exact compares text without letter case.
			l, r := strings.TrimSpace(workflowValueString(left)), strings.TrimSpace(workflowValueString(right))
			if len(l) > duplicateExactLimit {
				l = duplicateExactPrefix(l)
			}
			if len(r) > duplicateExactLimit {
				r = duplicateExactPrefix(r)
			}
			matches[i] = strings.EqualFold(l, r)
			scores[i] = 100
			if !matches[i] {
				scores[i] = 50
			}
		}
	}
	matched, supported = evaluateLookupFilterLogic(rule.BooleanFilter, matches)
	if !matched || !supported {
		return matched, supported, 0
	}
	if _, confidence, ok := filterRuleConfidence(rule.BooleanFilter, matches, scores); ok {
		return true, true, confidence
	}
	return true, true, 100
}

// fuzzyKeyMode is the match key a custom rule gives one of its items. The
// key is a property of the rule, not of the field (Salesforce Help, "How
// Match Keys Are Generated and Used in Matching Rules"): the rule's filter is
// rewritten into AND groups, and a group with a fuzzy First Name and a fuzzy
// Last Name uses the predefined key First_Name (1,1) Last_Name.
type fuzzyKeyMode uint8

const (
	// fuzzyKeyMethod is the method's own key (fuzzyScorers).
	fuzzyKeyMethod fuzzyKeyMode = iota
	// fuzzyKeyFirstInitial keys a first name on its initial
	// (firstNameInitialKeyed).
	fuzzyKeyFirstInitial
)

// customRuleKeyModes returns the key of each item of a custom matching rule.
// A First Name item takes the initial key when every AND group of the
// rule's filter that contains it also contains a Last Name item (native C1
// rows K3, K4, K7, K8, K9: "Vorkel" finds "Volkel", "Y" finds "Yancey" and
// "Kristen" no longer finds "Cristen"; alone or beside Email the item keeps
// its own key, K1 and K6). Any other item, and every item of a filter outside
// AND, OR and parentheses, keeps the method's key.
func customRuleKeyModes(rule storage.MatchingRule) []fuzzyKeyMode {
	modes := make([]fuzzyKeyMode, len(rule.Items))
	first, last := -1, false
	for i, item := range rule.Items {
		switch strings.ToLower(strings.TrimSpace(item.Method)) {
		case "firstname":
			first = i
		case "lastname":
			last = true
		}
	}
	if first < 0 || !last {
		return modes
	}
	var groups [][]int
	if strings.TrimSpace(rule.BooleanFilter) == "" {
		group := make([]int, len(rule.Items))
		for i := range group {
			group[i] = i
		}
		groups = [][]int{group}
	} else {
		node, ok := parseFilterConfidence(rule.BooleanFilter, len(rule.Items))
		if !ok {
			return modes
		}
		if groups, ok = node.andGroups(); !ok {
			return modes
		}
	}
	for i, item := range rule.Items {
		if !strings.EqualFold(strings.TrimSpace(item.Method), "FirstName") {
			continue
		}
		seen, keyed := false, true
		for _, group := range groups {
			hasFirst, hasLast := false, false
			for _, leaf := range group {
				hasFirst = hasFirst || leaf == i
				hasLast = hasLast || strings.EqualFold(strings.TrimSpace(rule.Items[leaf].Method), "LastName")
			}
			if hasFirst {
				seen = true
				keyed = keyed && hasLast
			}
		}
		if seen && keyed {
			modes[i] = fuzzyKeyFirstInitial
		}
	}
	return modes
}

// FindDuplicateMatches evaluates one duplicate rule for the record. supported
// is false when the rule needs matching semantics outside the captured rows:
// cross-object, unsupported fuzzy methods or unresolved matching rules. Callers then leave the
// rule unenforced, as before duplicate enforcement existed.
func FindDuplicateMatches(org *storage.OrgState, rule storage.DuplicateRule, record storage.Record, exclude func(storage.ID) bool) (DuplicateResult, bool) {
	return FindDuplicateMatchesWithAccess(org, rule, record, exclude, nil)
}

// FindDuplicateMatchesWithAccess shares the matcher with hosts that enforce
// record visibility. Access decides detection separately from disclosure.
func FindDuplicateMatchesWithAccess(org *storage.OrgState, rule storage.DuplicateRule, record storage.Record, exclude func(storage.ID) bool, access func(storage.Record) (detect, disclose bool)) (DuplicateResult, bool) {
	return FindDuplicateMatchesAmong(org, rule, record, exclude, access, nil)
}

// FindDuplicateMatchesAmong is FindDuplicateMatchesWithAccess limited to the
// given candidate records when among is not nil (records of another object than
// the scanned one are skipped). A nil among scans the org. The map is read only.
func FindDuplicateMatchesAmong(org *storage.OrgState, rule storage.DuplicateRule, record storage.Record, exclude func(storage.ID) bool, access func(storage.Record) (detect, disclose bool), among map[storage.ID]storage.Record) (DuplicateResult, bool) {
	result := DuplicateResult{Rule: rule.Name, RuleID: rule.ID, Object: rule.Object}
	personRule := strings.EqualFold(rule.Subtype, "PersonAccount")
	if org == nil || (rule.Subtype != "" && !personRule) || len(rule.MatchRules) == 0 {
		return result, false
	}
	for _, matching := range rule.MatchRules {
		standard, personStandard, scanObject, candidate, shaped := duplicateMatchingShape(rule, matching, record)
		if !shaped {
			return result, false
		}
		fuzzy := false
		if !standard {
			if !strings.EqualFold(matching.Status, "Active") || len(matching.Items) == 0 {
				return result, false
			}
			for _, item := range matching.Items {
				if !strings.EqualFold(item.Method, "Exact") && !FuzzyMethodSupported(item.Method) {
					return result, false
				}
				if !strings.EqualFold(item.Method, "Exact") {
					fuzzy = true
				}
			}
		}
		objectName, ok := storage.ResolveObjectName(*org, scanObject)
		if !ok {
			return result, false
		}
		match := DuplicateMatch{Object: objectName, Rule: matching.Name, Engine: exactMatchEngine}
		if fuzzy {
			// Native custom rules with a fuzzy item report the fuzzy engine.
			match.Engine = storage.StandardMatchEngine
		}
		var standardMatch func(existing storage.Record) (bool, float64)
		var keys []fuzzyKeyMode
		var prepared preparedCustomMatch
		usePrepared := false
		if !standard {
			keys = customRuleKeyModes(matching)
			prepared, usePrepared = newPreparedCustomMatch(matching, keys, candidate)
		}
		scan := true
		if standard {
			match.Engine = storage.StandardMatchEngine
			standardMatch = standardDuplicateMatcher(org, matching.Object, rule.Object, record)
			if personStandard {
				standardMatch = standardPersonAccountMatcher(org, record)
			}
			scan = standardMatch != nil
		}
		records := org.Objects[objectName].Records
		if among != nil {
			records = duplicateAmongObject(among, objectName)
		}
		for _, existing := range records {
			if !scan {
				break
			}
			if existing.System.IsDeleted || (record.ID != "" && storage.IDsEqual(existing.ID, record.ID)) || (exclude != nil && exclude(existing.ID)) {
				continue
			}
			if strings.EqualFold(scanObject, "Account") && personRule != isPersonAccountRecord(existing) {
				continue
			}
			var matched bool
			confidence := 100.0
			if standard {
				matched, confidence = standardMatch(existing)
			} else {
				var supported bool
				if usePrepared {
					matched, supported, confidence = prepared.match(existing)
				} else {
					matched, supported, confidence = customDuplicateMatch(matching, keys, candidate, existing)
				}
				if !supported {
					return result, false
				}
			}
			if matched {
				detect, disclose := true, true
				if access != nil {
					detect, disclose = access(existing)
				}
				if !detect {
					continue
				}
				result.Detected = true
				if !disclose {
					continue
				}
				match.Records = append(match.Records, existing.Clone())
				match.Confidences = append(match.Confidences, confidence)
			}
		}
		sort.Sort(byMatchCreation{&match})
		// Native account_overflow rows select the earliest 100 for every kind
		// of rule, then rank only that selected set by confidence and creation.
		if len(match.Records) > 100 {
			match.Records = match.Records[:100]
			match.Confidences = match.Confidences[:100]
		}
		sort.Stable(byDuplicateConfidence{&match})
		result.Matches = append(result.Matches, match)
	}
	return result, true
}

// duplicateAmongObject returns among itself when every record belongs to the
// object, else a copy without the others. It keeps the object test out of the
// scan loop of the org-wide path.
func duplicateAmongObject(among map[storage.ID]storage.Record, objectName string) map[storage.ID]storage.Record {
	for _, candidate := range among {
		if strings.EqualFold(candidate.Object, objectName) {
			continue
		}
		filtered := make(map[storage.ID]storage.Record, len(among))
		for id, other := range among {
			if strings.EqualFold(other.Object, objectName) {
				filtered[id] = other
			}
		}
		return filtered
	}
	return among
}

// duplicateMatchingShape classifies one matching rule of a duplicate rule for
// a record: whether it is a builtin or Person Account standard rule, the object
// it scans, and the candidate the scan compares (the record, or its objectMapping
// image for a cross-object custom rule). ok is false for a cross-object shape
// no native row pins.
func duplicateMatchingShape(rule storage.DuplicateRule, matching storage.MatchingRule, record storage.Record) (standard, personStandard bool, scanObject string, candidate storage.Record, ok bool) {
	personRule := strings.EqualFold(rule.Subtype, "PersonAccount")
	personStandard = personRule && strings.EqualFold(matching.Object, "PersonAccount") && strings.EqualFold(matching.Name, "Standard_PersonAccount_Match_Rule_v1_0") && len(matching.Items) == 0
	standard = (storage.IsStandardMatchingRule(matching.Object, matching.Name) && len(matching.Items) == 0) || personStandard
	scanObject = matching.Object
	if personRule && strings.EqualFold(matching.Object, "PersonAccount") {
		// A Person Account rule compares person accounts with each other.
		scanObject = "Account"
	}
	candidate = record
	if !strings.EqualFold(scanObject, rule.Object) && !(standard && standardCrossObject(rule.Object, matching.Object)) {
		mapped, mappedOK := mappedDuplicateCandidate(rule, matching, record)
		if standard || !mappedOK {
			return standard, personStandard, scanObject, candidate, false
		}
		candidate = mapped
	}
	return standard, personStandard, scanObject, candidate, true
}

// AttachFieldDiffs fills FieldDiffs for every match that has records. Only a
// result that is shown needs them (Find, a blocked or alerting save), so the
// matcher does not compute them for every candidate of every statement.
func (r *DuplicateResult) AttachFieldDiffs(rule storage.DuplicateRule, record storage.Record) {
	for i := range r.Matches {
		match := &r.Matches[i]
		if len(match.Records) == 0 || i >= len(rule.MatchRules) {
			continue
		}
		matching := rule.MatchRules[i]
		standard, personStandard, _, candidate, ok := duplicateMatchingShape(rule, matching, record)
		if !ok {
			continue
		}
		fields := duplicateDiffFields(matching, standard, personStandard)
		if len(fields) == 0 {
			continue
		}
		match.FieldDiffs = make([][]DuplicateFieldDiff, len(match.Records))
		for j, existing := range match.Records {
			match.FieldDiffs[j] = duplicateFieldDiffs(fields, candidate, existing)
		}
	}
}

// standardCrossObject reports the one cross-object shape the native rows pin:
// the Contact and Lead standard rules applied to a record of the other object.
func standardCrossObject(ruleObject, matchObject string) bool {
	r, m := strings.ToLower(ruleObject), strings.ToLower(matchObject)
	return (r == "contact" && m == "lead") || (r == "lead" && m == "contact")
}

// HasRecords reports whether any matching rule found a record.
func (r DuplicateResult) HasRecords() bool {
	if r.Detected {
		return true
	}
	for _, match := range r.Matches {
		if len(match.Records) > 0 {
			return true
		}
	}
	return false
}

// ActiveDuplicateRulesForRecord returns the active rules that apply to the
// record: for a person account the Person Account rules, otherwise the
// object's own. Natively a person account candidate is checked only against
// the Person Account rules (findDuplicates with no active Person Account rule
// reports "No active duplicate rules are defined for the Account object type").
func ActiveDuplicateRulesForRecord(org *storage.OrgState, object string, record storage.Record) []storage.DuplicateRule {
	if !isPersonAccountRecord(record) {
		return ActiveDuplicateRules(org, object)
	}
	return activeDuplicateRules(org, object, "PersonAccount")
}

// ActiveDuplicateRules returns the object's active rules in evaluation order.
func ActiveDuplicateRules(org *storage.OrgState, object string) []storage.DuplicateRule {
	return activeDuplicateRules(org, object, "")
}

func activeDuplicateRules(org *storage.OrgState, object, subtype string) []storage.DuplicateRule {
	if org == nil {
		return nil
	}
	name, ok := storage.ResolveObjectName(*org, object)
	if !ok {
		return nil
	}
	var rules []storage.DuplicateRule
	for _, rule := range org.Objects[name].Definition.DuplicateRules {
		if rule.Active && strings.EqualFold(rule.Subtype, subtype) && (!rule.Seeded || StandardDuplicateRulesEnabled(org)) {
			rules = append(rules, rule)
		}
	}
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].SortOrder < rules[j].SortOrder })
	return rules
}

// checkDuplicateRules runs after custom validation (native DR13) for Apex
// insert and update statements. Rules run in sort order; a permitted match
// does not bypass later blocking or alerting rules. Records inserted by the
// same statement are not candidates.
func (e *Engine) checkDuplicateRules(record storage.Record, update bool, stamp *string, prior *storage.Record) (*DuplicateReport, error) {
	if !update {
		e.duplicates.insertMatched = false
	}
	if (!e.Options.EnforceDuplicateRules && !(update && e.Options.EnforceUpsertIDInsertDuplicateRules)) || (!update && stamp == nil) {
		return nil, nil
	}
	exclude := func(id storage.ID) bool { return !update && e.duplicates.stamp == stamp && e.duplicates.batch[id] }
	var report *DuplicateReport
	rules := ActiveDuplicateRulesForRecord(e.Org, record.Object, record)
	gateBase := prior
	if update && len(rules) > 0 {
		gateBase = e.updateGateBaseline(record, prior)
	}
	for _, rule := range rules {
		if !update && e.Options.UpsertCapturedInsertRulesOnly && !upsertCapturedEmailRule(rule, record) && !standardAccountInsertRule(rule) {
			continue
		}
		gated := false
		if update {
			var changed bool
			gated, changed = customFuzzyFieldsChanged(rule, record, gateBase, e.Options.EnforceDuplicateRules)
			if !e.Options.EnforceDuplicateRules && !gated {
				continue
			}
			if gated && !changed {
				continue
			}
			if e.Options.UpsertChangedExactDuplicatesOnly && !gated && !upsertExactMatchChanged(rule, record, prior) {
				continue
			}
		}
		applies, supported := DuplicateRuleApplies(e.Org, rule, record)
		if !supported || !applies {
			continue
		}
		found, supported := e.findDuplicateMatches(rule, record, exclude)
		if !supported {
			continue
		}
		if !found.HasRecords() {
			// Native REP: records inserted together report as one set although
			// they never alert or block one another. A record that matches an
			// earlier record of its own statement is also inserted as an allowed
			// duplicate for the update gate (native GATE rows); that only matters
			// when the rule can act on an update, so other rules skip the scan.
			if !update && (report == nil && peerReportCapable(rule) || rule.ActionOnUpdate != "" && !e.duplicates.insertMatched) {
				if peers := e.peerMatches(rule, record, stamp); peers.HasRecords() {
					e.duplicates.insertMatched = true
					if report == nil && peerReportCapable(rule) {
						report = &DuplicateReport{Rule: rule, Object: record.Object, RecordID: record.ID, Matches: flattenMatches(peers)}
					}
				}
			}
			continue
		}
		action, operations := rule.ActionOnInsert, rule.OperationsOnInsert
		if update {
			action, operations = rule.ActionOnUpdate, rule.OperationsOnUpdate
		}
		found.Message = rule.AlertText
		if !update {
			e.duplicates.insertMatched = true
		}
		switch {
		case strings.EqualFold(action, "Block"):
			// DR01/DR05: Block ignores DuplicateRuleHeader.allowSave.
			found.AllowSave = false
			found.AttachFieldDiffs(rule, record)
			return nil, duplicateRuleError(found)
		case strings.EqualFold(action, "Allow"):
			found.AllowSave = true
			// DR03/DR04: an alert fails the save unless the explicit options allow it.
			if duplicateOperationsInclude(operations, "Alert") && !e.Options.AllowDuplicateSave {
				found.AttachFieldDiffs(rule, record)
				return nil, duplicateRuleError(found)
			}
			// CM01 pins insert reports; native REP also records an update into a
			// match for a custom rule whose update gate applies. Standard rules
			// and other update shapes stay unrecorded.
			if !duplicateOperationsInclude(operations, "Report") || update && !(gated && e.Options.EnforceDuplicateRules) {
				continue
			}
			if report == nil {
				matches := flattenMatches(found)
				if peerReportCapable(rule) && !update {
					if peers := e.peerMatches(rule, record, stamp); peers.HasRecords() {
						matches = append(matches, flattenMatches(peers)...)
					}
				}
				report = &DuplicateReport{Rule: rule, Object: record.Object, RecordID: record.ID, Matches: matches}
			}
			continue
		}
		return nil, nil
	}
	if report != nil && e.Options.UpsertCapturedInsertRulesOnly && !e.upsertSingleReportMatch(record, exclude) {
		return nil, nil
	}
	return report, nil
}

// peerReportCapable reports whether an insert under the rule can report
// records inserted earlier by the same statement (Allow with the Report operation,
// outside the upsert-captured route).
func peerReportCapable(rule storage.DuplicateRule) bool {
	return strings.EqualFold(rule.ActionOnInsert, "Allow") && duplicateOperationsInclude(rule.OperationsOnInsert, "Report")
}

func flattenMatches(result DuplicateResult) []storage.Record {
	var matches []storage.Record
	for _, match := range result.Matches {
		matches = append(matches, match.Records...)
	}
	return matches
}

// peerMatches finds the records inserted earlier by the same statement that
// match the record. Enforcement never sees them (they are excluded), so a
// report lists them after the matches enforcement found, then the peers: a
// design choice, since no native row pins the order inside a report set (the
// REPR rows list sorted names). Only those records can match, so the scan reads
// them directly instead of every record of the object.
func (e *Engine) peerMatches(rule storage.DuplicateRule, record storage.Record, stamp *string) DuplicateResult {
	if e.Options.UpsertCapturedInsertRulesOnly || stamp == nil || e.duplicates.stamp != stamp || len(e.duplicates.peers) == 0 {
		return DuplicateResult{}
	}
	if e.duplicates.peerView == nil {
		e.duplicates.peerView = make(map[storage.ID]storage.Record, len(e.duplicates.peers))
	}
	// The view keeps its keys across the statement, so a peer's entry is
	// allocated once and later refreshed in place with its current stored state.
	earlier := e.duplicates.peerView
	for _, peer := range e.duplicates.peers {
		if id, existing, ok := storage.LookupRecordByID(e.Org.Objects[peer.object].Records, peer.id); ok {
			earlier[id] = existing
		} else {
			delete(earlier, peer.id)
		}
	}
	if len(earlier) == 0 {
		return DuplicateResult{}
	}
	var peers DuplicateResult
	var supported bool
	if e.DuplicatePeerMatcher != nil {
		peers, supported = e.DuplicatePeerMatcher(rule, record, earlier)
	} else {
		peers, supported = FindDuplicateMatchesAmong(e.Org, rule, record, nil, nil, earlier)
	}
	if !supported {
		return DuplicateResult{}
	}
	return peers
}

// updateGateBaseline returns the record state the update gate compares with.
// Natively (RPT rows) a record inserted as an allowed duplicate in this
// transaction is compared with the values it had at its first update, so moving
// back to them does not run the rule again. The rows cover only that shape; a
// record from an earlier transaction, a record inserted without a match, or any
// other update compares with the stored values (a documented gap).
func (e *Engine) updateGateBaseline(record storage.Record, prior *storage.Record) *storage.Record {
	if prior == nil || e.RecordCreations == nil || !e.RecordCreations.Contains(record.ID) || !e.RecordCreations.InsertedAsDuplicate(record.ID) {
		return prior
	}
	if base, ok := e.RecordCreations.Baseline(record.ID); ok {
		return &base
	}
	e.RecordCreations.SetBaseline(record.ID, prior.Clone())
	return prior
}

// The committed upsert observation contains one matching report rule and one
// matching record. Leave fanout on the prior upsert path until captured.
func (e *Engine) upsertSingleReportMatch(record storage.Record, exclude func(storage.ID) bool) bool {
	rules, records := 0, 0
	for _, rule := range ActiveDuplicateRulesForRecord(e.Org, record.Object, record) {
		if !strings.EqualFold(rule.ActionOnInsert, "Allow") || !duplicateOperationsInclude(rule.OperationsOnInsert, "Report") {
			continue
		}
		applies, supported := DuplicateRuleApplies(e.Org, rule, record)
		if !supported {
			return false
		}
		if !applies {
			continue
		}
		matches, supported := e.findDuplicateMatches(rule, record, exclude)
		if !supported {
			return false
		}
		if !matches.HasRecords() {
			continue
		}
		rules++
		for _, match := range matches.Matches {
			records += len(match.Records)
		}
	}
	return rules == 1 && records == 1
}

func duplicateRuleError(result DuplicateResult) error {
	copied := result
	return dmlError{code: "DUPLICATES_DETECTED", message: result.Message, duplicate: &copied}
}

// noteDuplicateInsert records an inserted ID for statement-local exclusion and
// queues its pending report.
func (e *Engine) noteDuplicateInsert(stamp *string, report *DuplicateReport, object string, id storage.ID) {
	if !e.Options.EnforceDuplicateRules || stamp == nil {
		return
	}
	if e.duplicates.stamp != stamp {
		e.duplicates.stamp, e.duplicates.batch, e.duplicates.peers, e.duplicates.peerView = stamp, make(map[storage.ID]bool), nil, nil
	}
	e.duplicates.batch[id] = true
	e.duplicates.peers = append(e.duplicates.peers, duplicatePeer{object: object, id: id})
	if report != nil {
		report.RecordID = id
		e.duplicates.reports = append(e.duplicates.reports, *report)
	}
}

// enforceDuplicateRulesOnUpdate checks the merged record (native DR08).
func (e *Engine) enforceDuplicateRulesOnUpdate(record, prior storage.Record) error {
	report, err := e.checkDuplicateRules(record, true, nil, &prior)
	if err == nil && report != nil && e.recordTriggeredFlow == 0 {
		report.RecordID = record.ID
		e.duplicates.reports = append(e.duplicates.reports, *report)
	}
	return err
}

// The scalar external-key insert row uses the built-in business Account rule.
func standardAccountInsertRule(rule storage.DuplicateRule) bool {
	if !strings.EqualFold(rule.Object, "Account") || len(rule.MatchRules) != 1 {
		return false
	}
	match := rule.MatchRules[0]
	return len(match.Items) == 0 && ((rule.Subtype == "" && strings.EqualFold(match.Object, "Account") && storage.IsStandardMatchingRule(match.Object, match.Name)) ||
		(strings.EqualFold(rule.Subtype, "PersonAccount") && strings.EqualFold(match.Object, "PersonAccount") && strings.EqualFold(match.Name, "Standard_PersonAccount_Match_Rule_v1_0")))
}

// Custom edit rows cover exact and fuzzy items, AND and OR logic, and
// zero, one or multiple filter conditions. Filters decide scope separately.
// A nonblank matching value must change; sparse and same-value edits do not run.
//
// allowMatchBlanks admits an Exact MatchBlanks item to the gate. Only Apex DML
// (EnforceDuplicateRules) has native rows for it; the upsert-by-ID update route
// keeps skipping MatchBlanks rules.
func customFuzzyFieldsChanged(rule storage.DuplicateRule, record storage.Record, prior *storage.Record, allowMatchBlanks bool) (gated, changed bool) {
	if prior == nil || len(rule.MatchRules) != 1 {
		return false, false
	}
	match := rule.MatchRules[0]
	if !strings.EqualFold(match.Status, "Active") || len(match.Items) == 0 {
		return false, false
	}
	for _, item := range match.Items {
		matchBlanks := allowMatchBlanks && strings.EqualFold(item.BlankBehavior, "MatchBlanks") && strings.EqualFold(item.Method, "Exact")
		if !matchBlanks && !strings.EqualFold(item.BlankBehavior, "NullNotAllowed") || (!strings.EqualFold(item.Method, "Exact") && !FuzzyMethodSupported(item.Method)) {
			return false, false
		}
		before, beforeOK := prior.GetField(item.Field)
		after, ok := record.GetField(item.Field)
		if matchBlanks {
			// Native SGATE_mb/SGATE_mbBlank: an Exact MatchBlanks item runs
			// the rule when its value changes, including a change to blank.
			if duplicateBlank(before, beforeOK) != duplicateBlank(after, ok) || (!duplicateBlank(after, ok) && !storageValuesEqual(storage.Field{}, before, after)) {
				changed = true
			}
			continue
		}
		if !duplicateBlank(after, ok) && !storageValuesEqual(storage.Field{}, before, after) {
			changed = true
		}
	}
	return true, changed
}

// Limit upsert update enforcement to changed, single-field exact matches.
func upsertExactMatchChanged(rule storage.DuplicateRule, record storage.Record, prior *storage.Record) bool {
	if prior == nil || !upsertCapturedEmailRule(rule, record) {
		return false
	}
	match := rule.MatchRules[0]
	item := match.Items[0]
	before, beforeOK := prior.GetField(item.Field)
	after, afterOK := record.GetField(item.Field)
	// Native changed-update rows start from an existing nonblank match key.
	if duplicateBlank(before, beforeOK) {
		return false
	}
	return beforeOK != afterOK || !storageValuesEqual(storage.Field{}, before, after)
}

// The upsert captures cover one nonblank Contact Email exact match, not
// standard, multi-item, other-field, or MatchBlanks rules.
func upsertCapturedEmailRule(rule storage.DuplicateRule, record storage.Record) bool {
	if !strings.EqualFold(record.Object, "Contact") || !strings.EqualFold(rule.Object, "Contact") || len(rule.MatchRules) != 1 {
		return false
	}
	match := rule.MatchRules[0]
	if !strings.EqualFold(match.Object, "Contact") || storage.IsStandardMatchingRule(match.Object, match.Name) || !strings.EqualFold(match.Status, "Active") || len(match.Items) != 1 || (match.BooleanFilter != "" && match.BooleanFilter != "1") {
		return false
	}
	item := match.Items[0]
	value, ok := record.GetField(item.Field)
	return strings.EqualFold(item.Field, "Email") && strings.EqualFold(item.Method, "Exact") && strings.EqualFold(item.BlankBehavior, "NullNotAllowed") && !duplicateBlank(value, ok)
}

// TakeDuplicateReports returns and clears reports pending commit.
func (e *Engine) TakeDuplicateReports() []DuplicateReport {
	reports := e.duplicates.reports
	e.duplicates.reports = nil
	return reports
}

// StandardDuplicateRulesEnabled reports whether the seeded standard duplicate
// rules are enforced: the duplicateRules.standard setting or
// GLADE_STANDARD_DUPLICATE_RULES=on. The setting is off by default. A rule a
// project's metadata defines is never seeded and is always honored.
func StandardDuplicateRulesEnabled(org *storage.OrgState) bool {
	if org != nil && org.StandardDuplicateRules {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GLADE_STANDARD_DUPLICATE_RULES"))) {
	case "on", "1", "true":
		return true
	}
	return false
}

// Matches sort by the confidence Apex shows, the whole number
// math.Floor(c+0.5) (NATIVE fuzzy tie order
// ../apextest/testdata/conformance/duplicate_fuzzy_tie_order/org.tsv). Records
// whose shown confidences are equal keep creation order, whatever their
// unrounded scores. The sort is stable and the input is already ordered by
// creation before capping.
type byDuplicateConfidence struct{ m *DuplicateMatch }

func (b byDuplicateConfidence) Len() int { return len(b.m.Records) }
func (b byDuplicateConfidence) Less(i, j int) bool {
	return math.Floor(b.m.Confidences[i]+0.5) > math.Floor(b.m.Confidences[j]+0.5)
}
func (b byDuplicateConfidence) Swap(i, j int) { byMatchCreation{b.m}.Swap(i, j) }

// The activated Person Account rows establish an exact surname and email,
// with matching first names (including an initial). Every observed positive
// reports 100. Other Person Account matching paths remain unproven.
func standardPersonAccountMatcher(_ *storage.OrgState, record storage.Record) func(storage.Record) (bool, float64) {
	first, _ := duplicateFieldText(record, "FirstName")
	return func(existing storage.Record) (bool, float64) {
		if !duplicateFieldsEqual(record, existing, "LastName") || !duplicateFieldsEqual(record, existing, "PersonEmail") {
			return false, 0
		}
		other, _ := duplicateFieldText(existing, "FirstName")
		if item := standardCompare(first, other, standardFirstName); item.ok() || standardFirstInitial(first, other) {
			return true, 100
		}
		return false, 0
	}
}

func (e *Engine) findDuplicateMatches(rule storage.DuplicateRule, record storage.Record, exclude func(storage.ID) bool) (DuplicateResult, bool) {
	if e.DuplicateMatcher != nil {
		return e.DuplicateMatcher(rule, record, exclude)
	}
	return FindDuplicateMatches(e.Org, rule, record, exclude)
}

// Native FieldDiff rows (duplicate_runtime_behavior FDIFF, FDIFF2, MIXT) list
// the compared fields of each builtin matching rule and, for a custom rule of
// only Exact items, the item fields. Other custom rules and Person Account
// rules have no row, so they report no differences.
var duplicateDiffStandardFields = map[string][]string{
	"account": {"BillingCity", "BillingCountry", "BillingPostalCode", "BillingState", "BillingStreet", "Name", "Phone", "Website"},
	"contact": {"Account", "Email", "FirstName", "LastName", "MailingCity", "MailingPostalCode", "MailingStreet", "Phone", "Title"},
	"lead":    {"City", "Company", "Email", "FirstName", "LastName", "Phone", "PostalCode", "Street", "Title"},
}

func duplicateDiffFields(matching storage.MatchingRule, standard, personStandard bool) []string {
	if personStandard {
		return nil
	}
	if standard {
		return duplicateDiffStandardFields[strings.ToLower(matching.Object)]
	}
	var fields []string
	for _, item := range matching.Items {
		if !strings.EqualFold(item.Method, "Exact") {
			return nil
		}
		fields = append(fields, item.Field)
	}
	sort.Strings(fields)
	return fields
}

// duplicateFieldDiffs compares each field: Null when either side is blank,
// Same when the trimmed texts are equal without letter case, else Different.
func duplicateFieldDiffs(fields []string, candidate, existing storage.Record) []DuplicateFieldDiff {
	out := make([]DuplicateFieldDiff, 0, len(fields))
	for _, field := range fields {
		source := field
		if field == "Account" {
			source = "AccountId"
		}
		left, lok := duplicateFieldText(candidate, source)
		right, rok := duplicateFieldText(existing, source)
		difference := "Different"
		switch {
		case !lok || !rok:
			difference = "Null"
		case strings.EqualFold(left, right):
			difference = "Same"
		}
		out = append(out, DuplicateFieldDiff{Name: field, Difference: difference})
	}
	return out
}

// mappedDuplicateCandidate applies a custom duplicate rule's objectMapping
// (native XMAP): the matching rule object's fields read the mapped input
// fields, and every unmapped field is blank for the comparison. XMAP captured one
// mapped field, so a mapping of several fields stays unenforced.
func mappedDuplicateCandidate(rule storage.DuplicateRule, matching storage.MatchingRule, record storage.Record) (storage.Record, bool) {
	mapping := matching.Mapping
	if mapping == nil || len(mapping.Fields) != 1 ||
		(mapping.InputObject != "" && !strings.EqualFold(mapping.InputObject, rule.Object)) ||
		(mapping.OutputObject != "" && !strings.EqualFold(mapping.OutputObject, matching.Object)) {
		return storage.Record{}, false
	}
	mapped := storage.Record{ID: record.ID, Object: matching.Object, Fields: map[string]storage.Value{}}
	for _, pair := range mapping.Fields {
		if value, ok := record.GetField(pair.Input); ok {
			mapped.Fields[pair.Output] = value
		}
	}
	return mapped, true
}

// duplicateExactLimit is how many leading characters an Exact item compares
// (native X99: values equal in the first 99 characters match).
const duplicateExactLimit = 99

// duplicateExactPrefix keeps the first 99 characters of a trimmed value that
// is longer than 99 bytes (bytes bound characters, so shorter values never
// reach it). Native counts characters of an unrecorded unit (code points or
// UTF-16 units); the X99 rows are ASCII, so counting runes is an assumption
// that only differs for characters outside the Basic Multilingual Plane.
func duplicateExactPrefix(text string) string {
	if runes := []rune(text); len(runes) > duplicateExactLimit {
		return string(runes[:duplicateExactLimit])
	}
	return text
}
