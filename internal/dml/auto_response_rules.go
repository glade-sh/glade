package dml

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

var capturedResponseEqualityFormula = regexp.MustCompile(`(?i)^\s*[a-z_][a-z0-9_.]*\s*=\s*"[^"]*"\s*$`)

// Response-rule predicates are accepted only in shapes covered by the
// captured response-rule matrix. Other automation retains its own evaluator.
func capturedResponseCriteria(entry storage.AutoResponseRuleEntry) bool {
	if strings.TrimSpace(entry.Formula) != "" {
		return len(entry.Criteria) == 0 && strings.TrimSpace(entry.BooleanFilter) == "" && capturedResponseEqualityFormula.MatchString(entry.Formula)
	}
	if len(entry.Criteria) < 1 || len(entry.Criteria) > 2 {
		return false
	}
	filter := strings.ToUpper(strings.Join(strings.Fields(entry.BooleanFilter), " "))
	if len(entry.Criteria) == 1 && filter != "" || len(entry.Criteria) == 2 && filter != "1 AND 2" {
		return false
	}
	for _, criterion := range entry.Criteria {
		if !strings.EqualFold(strings.TrimSpace(criterion.Operation), "equals") || strings.TrimSpace(criterion.SourceField) != "" {
			return false
		}
	}
	return true
}

// AutoResponseSubject selects the first matching entry of the active rule.
func (e *Engine) AutoResponseSubject(input storage.Record) (string, bool, error) {
	state, ok := e.Org.Objects[input.Object]
	if !ok {
		return "", false, nil
	}
	_, record, ok := storage.LookupRecordByID(state.Records, input.ID)
	if !ok {
		return "", false, nil
	}
	for _, rule := range state.Definition.AutoResponseRules {
		if !rule.Active {
			continue
		}
		for _, entry := range rule.Entries {
			if !capturedResponseCriteria(entry) {
				return "", false, fmt.Errorf("dml: unsupported auto-response rule criteria in %s", rule.Name)
			}
			matches, supported := e.assignmentEntryMatches(storage.AssignmentRuleEntry{Formula: entry.Formula, BooleanFilter: entry.BooleanFilter, Criteria: entry.Criteria}, record, state.Definition)
			if !supported {
				return "", false, fmt.Errorf("dml: unsupported auto-response rule criteria in %s", rule.Name)
			}
			if !matches {
				continue
			}
			if strings.TrimSpace(entry.Template) == "" || strings.TrimSpace(entry.Subject) == "" {
				return "", false, fmt.Errorf("dml: unsupported auto-response template in %s", rule.Name)
			}
			return entry.Subject, true, nil
		}
		return "", false, nil
	}
	return "", false, nil
}
