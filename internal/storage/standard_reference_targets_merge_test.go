package storage

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// oldMergeStandardReferenceTargets is the implementation before the in-place
// comparison, kept verbatim as the equivalence oracle.
func oldMergeStandardReferenceTargets(existing, standard []string) []string {
	merged := append([]string(nil), standard...)
	return oldAppendUniqueStringsFold(merged, existing...)
}

func oldAppendUniqueStringsFold(values []string, additions ...string) []string {
	if len(values)+len(additions) <= 8 {
		for _, addition := range additions {
			if addition == "" {
				continue
			}
			found := false
			for _, value := range values {
				if stringsEqualFold(value, addition) {
					found = true
					break
				}
			}
			if !found {
				values = append(values, addition)
			}
		}
		return values
	}
	seen := make(map[string]struct{}, len(values)+len(additions))
	for _, value := range values {
		seen[asciiFoldKey(value)] = struct{}{}
	}
	for _, addition := range additions {
		if addition == "" {
			continue
		}
		key := asciiFoldKey(addition)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, addition)
	}
	return values
}

func polymorphicTargets(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("ParentObject%03d__c", i)
	}
	return out
}

func TestMergeStandardReferenceTargetsAllocationBudget(t *testing.T) {
	standard := polymorphicTargets(300)
	upper := make([]string, len(standard))
	for i, v := range standard {
		upper[i] = strings.ToUpper(v)
	}
	subset := append([]string(nil), standard[:150]...)
	sparse := []string{standard[3], standard[40], standard[200], standard[299]}
	cases := []struct {
		name     string
		existing []string
		budget   float64
	}{
		{"empty", nil, 1},
		{"equal", append([]string(nil), standard...), 1},
		{"equal mixed case", upper, 1},
		{"prefix subset", subset, 1},
		{"in-order sparse subset", sparse, 1},
		{"disjoint", []string{"Other1", "Other2", "Other3"}, 4},
		{"mixed", []string{standard[5], "Other1", strings.ToUpper(standard[9]), "OTHER1", ""}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sink []string
			allocs := testing.AllocsPerRun(50, func() {
				sink = mergeStandardReferenceTargets(tc.existing, standard)
			})
			if allocs > tc.budget {
				t.Fatalf("allocs per merge = %v, budget %v", allocs, tc.budget)
			}
			if want := oldMergeStandardReferenceTargets(tc.existing, standard); !reflect.DeepEqual(sink, want) {
				t.Fatalf("result differs from old implementation")
			}
		})
	}
}

func TestMergeStandardReferenceTargetsResultIsFresh(t *testing.T) {
	standard := polymorphicTargets(12)
	existing := append([]string(nil), standard...)
	got := mergeStandardReferenceTargets(existing, standard)
	got[0] = "changed"
	got = append(got, "more")
	if standard[0] == "changed" || existing[0] == "changed" {
		t.Fatalf("result aliases an input")
	}
	_ = got
}

func TestMergeStandardReferenceTargetsMatchesOldRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(52))
	pool := []string{"", "Account", "ACCOUNT", "account", "Contact", "Case", "Ünïcode", "ÜNÏCODE", "Straße", "STRASSE"}
	for i := 0; i < 40; i++ {
		pool = append(pool, fmt.Sprintf("Obj%02d__c", i), fmt.Sprintf("OBJ%02d__C", i))
	}
	pick := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = pool[rng.Intn(len(pool))]
		}
		return out
	}
	for iter := 0; iter < 4000; iter++ {
		var standard []string
		switch rng.Intn(3) {
		case 0:
			standard = pick(rng.Intn(10))
		case 1:
			standard = pick(rng.Intn(80))
		default:
			standard = pick(150 + rng.Intn(100))
		}
		var existing []string
		switch rng.Intn(6) {
		case 0:
			existing = pick(rng.Intn(30))
		case 1:
			existing = append([]string(nil), standard...)
		case 2:
			for _, v := range standard {
				if rng.Intn(3) != 0 {
					existing = append(existing, strings.ToUpper(v))
				}
			}
		case 3:
			existing = append(append([]string(nil), standard...), pick(rng.Intn(5))...)
		case 4:
			existing = append([]string(nil), standard...)
			rng.Shuffle(len(existing), func(a, b int) { existing[a], existing[b] = existing[b], existing[a] })
		default:
			existing = pick(20 + rng.Intn(60))
		}
		got := mergeStandardReferenceTargets(existing, standard)
		want := oldMergeStandardReferenceTargets(existing, standard)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("iter %d differs\nexisting=%q\nstandard=%q\ngot=%q\nwant=%q", iter, existing, standard, got, want)
		}
	}
}

func TestMergeStandardReferenceTargetsMatchesOldOverStandardCatalog(t *testing.T) {
	shapes := func(standard []string) map[string][]string {
		rev := make([]string, len(standard))
		upper := make([]string, len(standard))
		for i, v := range standard {
			rev[len(standard)-1-i] = v
			upper[i] = strings.ToUpper(v)
		}
		half := append([]string(nil), standard[:len(standard)/2]...)
		return map[string][]string{
			"nil":      nil,
			"equal":    append([]string(nil), standard...),
			"reversed": rev,
			"upper":    upper,
			"half":     half,
			"extra":    append(append([]string(nil), standard...), "ExtraParent__c"),
			"disjoint": {"ExtraParent__c", "Other__c"},
		}
	}
	check := func(label string, standard []string) {
		for name, existing := range shapes(standard) {
			got := mergeStandardReferenceTargets(existing, standard)
			want := oldMergeStandardReferenceTargets(existing, standard)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s (%s) differs\ngot=%q\nwant=%q", label, name, got, want)
			}
		}
	}
	objects := 0
	for _, objectName := range KnownStandardObjectNames() {
		if relationships, ok := standardSObjectStubRelationshipsFor(objectName); ok {
			for _, relationship := range relationships {
				check(objectName+"."+relationship.Field, relationship.ParentObjects)
			}
			objects++
		}
		if fields, ok := standardSObjectStubFieldsFor(objectName); ok {
			for _, field := range fields {
				check(objectName+"."+field.APIName, field.ReferenceTo)
			}
		}
		for _, field := range standardFieldsForObject(objectName) {
			check(objectName+"."+field.APIName, field.ReferenceTo)
		}
	}
	if objects == 0 {
		t.Fatalf("no catalog relationships were enumerated")
	}
}
