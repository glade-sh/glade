package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Native Lightning Out rows (aura_overload_determinism_ranking/org.tsv) show
// that, for overloaded @AuraEnabled methods that still deploy (Apex API before
// 55), Salesforce picks one fixed overload per set of equal-arity single
// parameter types. The choice did not change with declaration order, deploy
// order, class name, Apex or component API, repeat, or argument value. The
// order below is the strict total order those observations form, best first.
// Parameters are converted after the choice, so a value the chosen overload
// rejects fails instead of falling back to the other overload.
var historicalLightningTypeRank = map[string]int{
	"list<string>": 0, "datetime": 1, "decimal": 2, "string": 3, "object": 4, "long": 5,
	"integer": 6, "double": 7, "boolean": 8, "account": 9, "date": 10, "id": 11,
}

// historicalLightningRankedCandidates narrows candidates to the one overload
// the native rows prove Salesforce selects. Anything the rows do not cover
// (other arities, other parameter types, API 55 or later, unknown source
// versions) returns candidates unchanged.
func historicalLightningRankedCandidates(candidates []Method) []Method {
	best := -1
	bestRank := len(historicalLightningTypeRank)
	signatures := map[string]bool{}
	ranks := map[int]bool{}
	for i, method := range candidates {
		if !method.IsStatic || !methodHasAuraEnabled(method.Modifiers) {
			continue
		}
		if strings.TrimSpace(method.APIVersion) == "" || len(method.Params) != 1 {
			return candidates
		}
		version, err := apexversion.PreserveSource(method.APIVersion)
		if err != nil || !apexversion.Before(version, 55) {
			return candidates
		}
		rank, ok := historicalLightningTypeRank[strings.ToLower(strings.Join(strings.Fields(method.Params[0].Type), ""))]
		if !ok || ranks[rank] {
			return candidates
		}
		ranks[rank] = true
		signatures[uiMethodSignature(method)] = true
		if rank < bestRank {
			best, bestRank = i, rank
		}
	}
	if best < 0 || len(signatures) < 2 {
		return candidates
	}
	return []Method{candidates[best]}
}
