package storage

import (
	"sort"
	"strings"
	"sync"
	"testing"
)

type standardOverlayCheckVerdicts struct {
	needWrite, refresh, readOnly bool
}

func standardOverlayCheckVerdictsFor(definition ObjectDefinition, signature string) standardOverlayCheckVerdicts {
	return standardOverlayCheckVerdicts{
		needWrite: standardObjectFieldsNeedWrite(definition, signature),
		refresh:   standardFieldsOverlayNeedsRefresh(definition, signature),
		readOnly:  standardReadOnlyFlagsNeedRepair(&definition),
	}
}

func legacyStandardOverlayCheckVerdictsFor(definition ObjectDefinition, signature string) standardOverlayCheckVerdicts {
	return standardOverlayCheckVerdicts{
		needWrite: legacyStandardObjectFieldsNeedWrite(definition, signature),
		refresh:   legacyStandardFieldsOverlayNeedsRefresh(definition, signature),
		readOnly:  legacyStandardReadOnlyFlagsNeedRepair(&definition),
	}
}

// standardOverlayFieldMutations are the in-place field writes the check must
// observe. Each returns the mutated field and whether it still exists.
var standardOverlayFieldMutations = []struct {
	name   string
	mutate func(Field) Field
}{
	{"type cleared", func(f Field) Field { f.Type = ""; return f }},
	{"type any", func(f Field) Field { f.Type = FieldAny; return f }},
	{"display type cleared", func(f Field) Field { f.DisplayType = ""; return f }},
	{"display type blank", func(f Field) Field { f.DisplayType = "  "; return f }},
	{"picklist cleared", func(f Field) Field { f.PicklistValues = nil; return f }},
	{"auto number cleared", func(f Field) Field { f.AutoNumber = false; return f }},
	{"time reference", func(f Field) Field { f.Type = FieldReference; f.ReferenceTo = []string{"Time"}; return f }},
	{"read-only flags cleared", func(f Field) Field { f.Createable = nil; f.Updateable = nil; return f }},
}

func checkStandardOverlayParity(t *testing.T, label string, definition ObjectDefinition, signature string) standardOverlayCheckVerdicts {
	t.Helper()
	want := legacyStandardOverlayCheckVerdictsFor(definition, signature)
	if got := standardOverlayCheckVerdictsFor(definition, signature); got != want {
		t.Fatalf("%s: verdicts = %+v, want %+v", label, got, want)
	}
	return want
}

// sweepStandardOverlayMutations applies every field mutation, deletion and
// case rename to the named fields in place, checks parity, and restores.
func sweepStandardOverlayMutations(t *testing.T, label string, definition ObjectDefinition, signature string, fieldNames []string, flips map[string]int) {
	t.Helper()
	base := legacyStandardOverlayCheckVerdictsFor(definition, signature)
	note := func(kind string, got standardOverlayCheckVerdicts) {
		if flips != nil && got != base {
			flips[kind]++
		}
	}
	for _, name := range fieldNames {
		original, ok := definition.Fields[name]
		if !ok {
			continue
		}
		for _, mutation := range standardOverlayFieldMutations {
			definition.Fields[name] = mutation.mutate(original)
			note(mutation.name, checkStandardOverlayParity(t, label+"/"+name+"/"+mutation.name, definition, signature))
		}
		delete(definition.Fields, name)
		note("deleted", checkStandardOverlayParity(t, label+"/"+name+"/deleted", definition, signature))
		renamed := strings.ToLower(name)
		if renamed == name {
			renamed = strings.ToUpper(name)
		}
		definition.Fields[renamed] = original
		note("case renamed", checkStandardOverlayParity(t, label+"/"+name+"/case renamed", definition, signature))
		delete(definition.Fields, renamed)
		definition.Fields[name] = original
	}
	enableSearch := definition.EnableSearch
	definition.EnableSearch = !enableSearch
	note("enable search flipped", checkStandardOverlayParity(t, label+"/enable search flipped", definition, signature))
	definition.EnableSearch = enableSearch
	if definition.Metadata != nil {
		marker, applied := definition.Metadata[standardFieldsOverlayMarker]
		definition.Metadata[standardFieldsOverlayMarker] = marker + ",Other"
		note("marker changed", checkStandardOverlayParity(t, label+"/marker changed", definition, signature))
		delete(definition.Metadata, standardFieldsOverlayMarker)
		note("marker removed", checkStandardOverlayParity(t, label+"/marker removed", definition, signature))
		if applied {
			definition.Metadata[standardFieldsOverlayMarker] = marker
		}
	}
	apiName := definition.APIName
	for _, renamed := range []string{strings.ToUpper(apiName), strings.ToLower(apiName), " " + apiName} {
		definition.APIName = renamed
		note("object name respelled", checkStandardOverlayParity(t, label+"/object "+renamed, definition, signature))
	}
	definition.APIName = apiName
}

func standardOverlayTestFeatures() []string {
	seen := map[string]bool{}
	for _, name := range KnownStandardObjectNames() {
		entry, ok := standardObjectCatalogEntryForName(name)
		if !ok {
			continue
		}
		for feature := range entry.FeatureFields {
			seen[feature] = true
		}
	}
	for _, feature := range []string{"PersonAccounts", "StateAndCountryPicklist", "MultiCurrency"} {
		seen[feature] = true
	}
	out := make([]string, 0, len(seen))
	for feature := range seen {
		out = append(out, feature)
	}
	sort.Strings(out)
	return out
}

func enrichedStandardOverlayDefinition(name string, features []string) ObjectDefinition {
	definition := ObjectDefinition{APIName: name, Fields: map[string]Field{
		"Name": {APIName: "Name", Type: FieldString},
	}}
	EnsureStandardObjectFieldsForFeatures(&definition, features)
	return definition
}

func sortedFieldNames(definition ObjectDefinition) []string {
	names := make([]string, 0, len(definition.Fields))
	for name := range definition.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestStandardOverlayCheckPlanMatchesLegacyForEveryStandardObject compares the
// plan-backed check with the unmemoized check for every known standard object
// under several feature signatures, enriched and mutated.
func TestStandardOverlayCheckPlanMatchesLegacyForEveryStandardObject(t *testing.T) {
	allFeatures := standardOverlayTestFeatures()
	featureSets := [][]string{nil, {"PersonAccounts"}, {"StateAndCountryPicklist"}, allFeatures}
	rawSignatures := []string{",PersonAccounts,,StateAndCountryPicklist,", "Unknown", strings.Join(allFeatures, ",") + ","}
	for _, name := range KnownStandardObjectNames() {
		for _, features := range featureSets {
			signature := canonicalFeatureSignature(features)
			definition := enrichedStandardOverlayDefinition(name, features)
			label := name + "/" + signature
			checkStandardOverlayParity(t, label, definition, signature)
			for _, other := range featureSets {
				checkStandardOverlayParity(t, label+" checked as "+canonicalFeatureSignature(other), definition, canonicalFeatureSignature(other))
			}
			for _, raw := range rawSignatures {
				checkStandardOverlayParity(t, label+" checked as "+raw, definition, raw)
			}
			// A small field sample per object keeps the sweep bounded; the
			// focused test below mutates every field of the hot objects.
			var sample []string
			fields := sortedFieldNames(definition)
			if len(fields) > 0 {
				sample = append(sample, fields[0], fields[len(fields)-1])
			}
			for _, field := range fields {
				if len(definition.Fields[field].PicklistValues) > 0 || strings.TrimSpace(definition.Fields[field].DisplayType) != "" {
					sample = append(sample, field)
					break
				}
			}
			sweepStandardOverlayMutations(t, label, definition, signature, sample, nil)
		}
	}
}

// TestStandardOverlayCheckPlanObservesEveryFieldMutation mutates every field
// of the objects the runtime checks most and of every object whose stub
// overlay has catalog time fields. Each mutation kind must flip some verdict,
// which proves the plan holds no per-definition answer.
func TestStandardOverlayCheckPlanObservesEveryFieldMutation(t *testing.T) {
	names := []string{"Account", "Contact", "Case", "Contract", "Order", "Lead", "Opportunity", "User",
		"EntityDefinition", "FieldDefinition", "Group", "Probe__c", "Probe__mdt", "Probe__e"}
	for _, name := range KnownStandardObjectNames() {
		if _, ok := standardSObjectStubFieldsFor(name); ok && len(standardStubTimeCatalogFields(name)) > 0 {
			names = append(names, name)
		}
	}
	flips := map[string]int{}
	for _, name := range names {
		for _, features := range [][]string{nil, {"PersonAccounts", "StateAndCountryPicklist"}} {
			signature := canonicalFeatureSignature(features)
			definition := enrichedStandardOverlayDefinition(name, features)
			sweepStandardOverlayMutations(t, name+"/"+signature, definition, signature, sortedFieldNames(definition), flips)
		}
	}
	for _, kind := range []string{"type cleared", "type any", "display type cleared", "display type blank", "picklist cleared",
		"auto number cleared", "time reference", "read-only flags cleared", "deleted", "enable search flipped",
		"marker changed", "marker removed"} {
		if flips[kind] == 0 {
			t.Errorf("mutation %q never changed a verdict; the sweep does not exercise it", kind)
		}
	}
}

// TestStandardOverlayCheckPlanFollowsInstalledDescribeCache proves the only
// plan invalidation path: installing a different describe cache.
func TestStandardOverlayCheckPlanFollowsInstalledDescribeCache(t *testing.T) {
	const object = "CareProgram"
	if _, ok := standardObjectCatalogData[object]; ok {
		t.Skip("CareProgram moved into the generated catalog; pick another V2-only object")
	}
	production := standardOverlayCheckPlanFor(object)
	definition := ObjectDefinition{APIName: object, EnableSearch: true, Fields: map[string]Field{
		"Status": {APIName: "Status", Type: FieldPicklist, DisplayType: "picklist"},
	}, Metadata: map[string]string{standardFieldsOverlayMarker: ""}}
	injected := newStandardDescribeCatalogV2Cache(func(standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
		return standardObjectCatalogEntry{Definition: ObjectDefinition{APIName: object, Fields: map[string]Field{
			"Status": {APIName: "Status", Type: FieldPicklist, DisplayType: "picklist", PicklistValues: []PicklistValue{{Value: "Open"}}},
		}}}, nil
	})
	previous := standardDescribeCatalogV2ProductionCache
	standardDescribeCatalogV2ProductionCache = injected
	func() {
		defer func() { standardDescribeCatalogV2ProductionCache = previous }()
		if standardOverlayCheckPlanFor(object) == production {
			t.Fatal("plan survived a describe cache swap")
		}
		if !standardFieldsOverlayNeedsRefresh(definition, "") {
			t.Fatal("plan did not read the installed describe cache's picklist values")
		}
		if got, want := standardFieldsOverlayNeedsRefresh(definition, ""), legacyStandardFieldsOverlayNeedsRefresh(definition, ""); got != want {
			t.Fatalf("refresh = %v, legacy %v under the injected cache", got, want)
		}
	}()
	if got, want := standardFieldsOverlayNeedsRefresh(definition, ""), legacyStandardFieldsOverlayNeedsRefresh(definition, ""); got != want {
		t.Fatalf("refresh = %v, legacy %v after restoring the production cache", got, want)
	}
}

// TestStandardOverlayCheckPlanBuildsOncePerObject fails if the plan cache is
// removed: repeated checks must reuse one plan per object name.
func TestStandardOverlayCheckPlanBuildsOncePerObject(t *testing.T) {
	definition := enrichedStandardOverlayDefinition("Account", nil)
	standardObjectFieldsNeedWrite(definition, "")
	set := currentStandardOverlayCheckPlans()
	before := set.builds.Load()
	plan := standardOverlayCheckPlanFor("Account")
	for i := 0; i < 10; i++ {
		standardObjectFieldsNeedWrite(definition, "")
		standardFieldsOverlayNeedsRefresh(definition, "")
		standardReadOnlyFlagsNeedRepair(&definition)
	}
	if got := set.builds.Load(); got != before {
		t.Fatalf("plan builds grew from %d to %d on repeated checks", before, got)
	}
	if standardOverlayCheckPlanFor("Account") != plan {
		t.Fatal("repeated lookups returned a different plan")
	}
}

// TestStandardOverlayCheckPlanAllocationBudget is the allocation budget for
// the steady-state check on enriched definitions.
func TestStandardOverlayCheckPlanAllocationBudget(t *testing.T) {
	for _, name := range []string{"Account", "Contact", "Case", "Opportunity"} {
		definition := enrichedStandardOverlayDefinition(name, nil)
		if standardObjectFieldsNeedWrite(definition, "") {
			t.Fatalf("%s: enriched definition still needs a write", name)
		}
		got := testing.AllocsPerRun(20, func() { standardObjectFieldsNeedWrite(definition, "") })
		legacy := testing.AllocsPerRun(20, func() { legacyStandardObjectFieldsNeedWrite(definition, "") })
		if got > 0 {
			t.Errorf("%s: check allocates %.0f per call, want 0 (legacy %.0f)", name, got, legacy)
		}
		t.Logf("%s: allocs per check %.0f, legacy %.0f", name, got, legacy)
	}
}

func TestStandardOverlayCheckPlanConcurrentUse(t *testing.T) {
	names := []string{"Account", "Contact", "Case", "Order"}
	for _, name := range KnownStandardObjectNames() {
		if _, ok := standardSObjectStubFieldsFor(name); ok && len(standardStubTimeCatalogFields(name)) > 0 {
			names = append(names, name)
			break
		}
	}
	definitions := make([]ObjectDefinition, len(names))
	for i, name := range names {
		definitions[i] = enrichedStandardOverlayDefinition(name, nil)
		if timeFields := standardStubTimeCatalogFields(name); len(timeFields) > 0 {
			if field, ok := ResolveFieldName(definitions[i], "", timeFields[0]); ok {
				stale := definitions[i].Fields[field]
				stale.Type, stale.ReferenceTo = FieldReference, []string{"Time"}
				definitions[i].Fields[field] = stale
			}
		}
	}
	want := make([]standardOverlayCheckVerdicts, len(definitions))
	for i, definition := range definitions {
		want[i] = legacyStandardOverlayCheckVerdictsFor(definition, "")
	}
	// Start from an empty plan set so goroutines race on plan creation and
	// on the lazy stub time fields.
	standardOverlayCheckPlans.Store(nil)
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i, definition := range definitions {
				if got := standardOverlayCheckVerdictsFor(definition, ""); got != want[i] {
					errs <- definition.APIName
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for name := range errs {
		t.Errorf("%s: concurrent verdict differs from legacy", name)
	}
}
