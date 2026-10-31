package dml

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// DuplicateMatch is one matching rule's result inside a DuplicateResult.
// Records are snapshots of the matched rows at detection time.
type DuplicateMatch struct {
	Object  string
	Rule    string
	Engine  string
	Records []storage.Record
}

// DuplicateResult mirrors Datacloud.DuplicateResult for one duplicate rule.
type DuplicateResult struct {
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
	stamp   *string
	batch   map[storage.ID]bool
	reports []DuplicateReport
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

// standardDuplicateMatch applies only the discriminators the native rows pin
// (ST02-ST05, ST07, ST08, ST11, DR12); builtin fuzzy scoring is not modeled.
func standardDuplicateMatch(object string, candidate, existing storage.Record) bool {
	switch strings.ToLower(object) {
	case "account":
		for _, field := range []string{"Name", "BillingStreet", "BillingCity", "BillingPostalCode"} {
			if !duplicateFieldsEqual(candidate, existing, field) {
				return false
			}
		}
		return true
	case "contact", "lead":
		if !duplicateFieldsEqual(candidate, existing, "LastName") {
			return false
		}
		first, firstOK := candidate.GetField("FirstName")
		other, otherOK := existing.GetField("FirstName")
		bothFirst := !duplicateBlank(first, firstOK) && !duplicateBlank(other, otherOK)
		if bothFirst && !duplicateFieldsEqual(candidate, existing, "FirstName") {
			return false
		}
		if duplicateFieldsEqual(candidate, existing, "Email") {
			return true
		}
		return bothFirst && duplicateFieldsEqual(candidate, existing, "Phone")
	}
	return false
}

func exactDuplicateMatch(rule storage.MatchingRule, candidate, existing storage.Record) (bool, bool) {
	matches := make([]bool, len(rule.Items))
	for i, item := range rule.Items {
		left, lok := candidate.GetField(item.Field)
		right, rok := existing.GetField(item.Field)
		leftBlank, rightBlank := duplicateBlank(left, lok), duplicateBlank(right, rok)
		switch {
		case leftBlank || rightBlank:
			// DR09: MatchBlanks pairs two blank keys; NullNotAllowed never does.
			matches[i] = leftBlank && rightBlank && strings.EqualFold(item.BlankBehavior, "MatchBlanks")
		default:
			// DR10: Exact compares text without letter case.
			matches[i] = strings.EqualFold(strings.TrimSpace(workflowValueString(left)), strings.TrimSpace(workflowValueString(right)))
		}
	}
	return evaluateLookupFilterLogic(rule.BooleanFilter, matches)
}

// FindDuplicateMatches evaluates one duplicate rule for the record. supported
// is false when the rule needs matching semantics outside the captured rows:
// cross-object, fuzzy or unresolved matching rules. Callers then leave the
// rule unenforced, as before duplicate enforcement existed.
func FindDuplicateMatches(org *storage.OrgState, rule storage.DuplicateRule, record storage.Record, exclude func(storage.ID) bool) (DuplicateResult, bool) {
	result := DuplicateResult{Rule: rule.Name, RuleID: rule.ID, Object: rule.Object}
	if org == nil || rule.Subtype != "" || len(rule.MatchRules) == 0 {
		return result, false
	}
	for _, matching := range rule.MatchRules {
		if !strings.EqualFold(matching.Object, rule.Object) {
			return result, false
		}
		standard := storage.IsStandardMatchingRule(matching.Object, matching.Name) && len(matching.Items) == 0
		if !standard {
			if !strings.EqualFold(matching.Status, "Active") || len(matching.Items) == 0 {
				return result, false
			}
			for _, item := range matching.Items {
				if !strings.EqualFold(item.Method, "Exact") {
					return result, false
				}
			}
		}
		objectName, ok := storage.ResolveObjectName(*org, matching.Object)
		if !ok {
			return result, false
		}
		match := DuplicateMatch{Object: objectName, Rule: matching.Name, Engine: exactMatchEngine}
		if standard {
			match.Engine = storage.StandardMatchEngine
		}
		for _, existing := range org.Objects[objectName].Records {
			if existing.System.IsDeleted || (record.ID != "" && storage.IDsEqual(existing.ID, record.ID)) || (exclude != nil && exclude(existing.ID)) {
				continue
			}
			var matched bool
			if standard {
				matched = standardDuplicateMatch(objectName, record, existing)
			} else {
				var supported bool
				if matched, supported = exactDuplicateMatch(matching, record, existing); !supported {
					return result, false
				}
			}
			if matched {
				match.Records = append(match.Records, existing.Clone())
			}
		}
		sort.Slice(match.Records, func(i, j int) bool { return match.Records[i].ID < match.Records[j].ID })
		result.Matches = append(result.Matches, match)
	}
	return result, true
}

// HasRecords reports whether any matching rule found a record.
func (r DuplicateResult) HasRecords() bool {
	for _, match := range r.Matches {
		if len(match.Records) > 0 {
			return true
		}
	}
	return false
}

// ActiveDuplicateRules returns the object's active rules in evaluation order.
func ActiveDuplicateRules(org *storage.OrgState, object string) []storage.DuplicateRule {
	if org == nil {
		return nil
	}
	name, ok := storage.ResolveObjectName(*org, object)
	if !ok {
		return nil
	}
	var rules []storage.DuplicateRule
	for _, rule := range org.Objects[name].Definition.DuplicateRules {
		if rule.Active && rule.Subtype == "" {
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
	if !e.Options.EnforceDuplicateRules || (!update && stamp == nil) {
		return nil, nil
	}
	exclude := func(id storage.ID) bool { return !update && e.duplicates.stamp == stamp && e.duplicates.batch[id] }
	var report *DuplicateReport
	for _, rule := range ActiveDuplicateRules(e.Org, record.Object) {
		if !update && e.Options.UpsertCapturedInsertRulesOnly && !upsertCapturedEmailRule(rule, record) {
			continue
		}
		if update && e.Options.UpsertChangedExactDuplicatesOnly && !upsertExactMatchChanged(rule, record, prior) {
			continue
		}
		applies, supported := DuplicateRuleApplies(e.Org, rule, record)
		if !supported || !applies {
			continue
		}
		found, supported := FindDuplicateMatches(e.Org, rule, record, exclude)
		if !supported || !found.HasRecords() {
			continue
		}
		action, operations := rule.ActionOnInsert, rule.OperationsOnInsert
		if update {
			action, operations = rule.ActionOnUpdate, rule.OperationsOnUpdate
		}
		found.Message = rule.AlertText
		switch {
		case strings.EqualFold(action, "Block"):
			// DR01/DR05: Block ignores DuplicateRuleHeader.allowSave.
			found.AllowSave = false
			return nil, duplicateRuleError(found)
		case strings.EqualFold(action, "Allow"):
			found.AllowSave = true
			// DR03/DR04: an alert fails the save unless the explicit options allow it.
			if duplicateOperationsInclude(operations, "Alert") && !e.Options.AllowDuplicateSave {
				return nil, duplicateRuleError(found)
			}
			// CM01 pins insert reports only; update reports stay unrecorded.
			if update || !duplicateOperationsInclude(operations, "Report") {
				continue
			}
			if report == nil {
				var matches []storage.Record
				for _, match := range found.Matches {
					matches = append(matches, match.Records...)
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

// The committed upsert observation contains one matching report rule and one
// matching record. Leave fanout on the prior upsert path until captured.
func (e *Engine) upsertSingleReportMatch(record storage.Record, exclude func(storage.ID) bool) bool {
	rules, records := 0, 0
	for _, rule := range ActiveDuplicateRules(e.Org, record.Object) {
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
		matches, supported := FindDuplicateMatches(e.Org, rule, record, exclude)
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
func (e *Engine) noteDuplicateInsert(stamp *string, report *DuplicateReport, id storage.ID) {
	if !e.Options.EnforceDuplicateRules || stamp == nil {
		return
	}
	if e.duplicates.stamp != stamp {
		e.duplicates.stamp, e.duplicates.batch = stamp, make(map[storage.ID]bool)
	}
	e.duplicates.batch[id] = true
	if report != nil {
		report.RecordID = id
		e.duplicates.reports = append(e.duplicates.reports, *report)
	}
}

// enforceDuplicateRulesOnUpdate checks the merged record (native DR08).
func (e *Engine) enforceDuplicateRulesOnUpdate(record, prior storage.Record) error {
	_, err := e.checkDuplicateRules(record, true, nil, &prior)
	return err
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
