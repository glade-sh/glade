package playground

import (
	"encoding/json"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/vm"
)

// publicResultFitsBudget bounds work before encoding. A shared Apex value may
// appear repeatedly in the response, so count every occurrence, not pointers.
func publicResultFitsBudget(result RunResult) bool {
	budget := maxPublicResponseBytes - 4096 // fixed result fields and punctuation
	consume := func(n int) bool {
		if n < 0 || n > budget {
			return false
		}
		budget -= n
		return true
	}
	var stringFits func(string) bool
	stringFits = func(text string) bool {
		if !consume(2) {
			return false
		}
		for len(text) > 0 {
			r, n := utf8.DecodeRuneInString(text)
			cost := n
			switch {
			case r == '"' || r == '\\' || r == '\b' || r == '\f' || r == '\n' || r == '\r' || r == '\t':
				cost = 2
			case r < 0x20 || r == '<' || r == '>' || r == '&' || r == '\u2028' || r == '\u2029' || (r == utf8.RuneError && n == 1):
				cost = 6
			}
			if !consume(cost) {
				return false
			}
			text = text[n:]
		}
		return true
	}
	var valueFits func(vm.Value, int) bool
	valueFits = func(value vm.Value, depth int) bool {
		value = value.JSONBudgetSnapshot()
		if depth > 64 || !consume(160) || !stringFits(string(value.Kind)) || !stringFits(value.Text) || !stringFits(value.Type) {
			return false
		}
		for key, child := range value.Fields {
			if !stringFits(key) || !valueFits(child, depth+1) {
				return false
			}
		}
		for _, group := range [][]vm.Value{value.List, value.Set} {
			for _, child := range group {
				if !valueFits(child, depth+1) {
					return false
				}
			}
		}
		for key, child := range value.Map {
			if !stringFits(key) || !valueFits(child, depth+1) {
				return false
			}
		}
		return true
	}
	for _, text := range []string{result.RunID, string(result.Status), string(result.LimitMode), result.CacheKey, result.ErrorMessage} {
		if !stringFits(text) {
			return false
		}
	}
	for _, d := range result.Diagnostics {
		if !consume(80) || !stringFits(d.Severity) || !stringFits(d.Message) {
			return false
		}
	}
	for _, line := range result.Logs {
		if !consume(1) || !stringFits(line) {
			return false
		}
	}
	for _, variable := range result.Vars {
		if !consume(80) || !stringFits(variable.Name) || !stringFits(variable.Type) || !valueFits(variable.Value, 0) {
			return false
		}
	}
	for _, event := range result.Trace {
		data, err := json.Marshal(event)
		if err != nil || !consume(len(data)+1) {
			return false
		}
	}
	for _, diff := range result.OrgDiff {
		if !consume(128) || !stringFits(diff.Object) {
			return false
		}
		for _, ids := range [][]string{diff.InsertedIDs, diff.UpdatedIDs, diff.DeletedIDs} {
			for _, id := range ids {
				if !consume(1) || !stringFits(id) {
					return false
				}
			}
		}
	}
	return true
}
