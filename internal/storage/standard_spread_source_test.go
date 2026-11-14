package storage

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
)

// The native rows sample both present and absent objects on all three routes.
func TestPlainDeveloperSpreadNativeConformance(t *testing.T) {
	f, err := os.Open("testdata/plain_developer_spread/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	features := []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}
	orgs := map[string]OrgState{"eager": sourceSpreadEagerOrg(features), "source": sourceSpreadLazyOrg(features), "legacy": lazySpreadOrg(features)}
	for route, org := range orgs {
		if route != "eager" {
			MaterializeStandardObjectSpread(&org)
		}
		for _, row := range rows[1:] {
			_, got := org.Objects[row[2]]
			if want := row[3] == "true"; got != want {
				t.Errorf("%s API %s %s: present=%t want %t", route, row[0], row[2], got, want)
			}
		}
	}
}

func BenchmarkPlainDeveloperEagerSpread(b *testing.B) {
	features := []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}
	for i := 0; i < b.N; i++ {
		org := sourceSpreadEagerOrg(features)
		if len(org.Objects) == 0 {
			b.Fatal("empty spread")
		}
	}
}

func TestPlainDeveloperSpreadPreservesProjectObject(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "AIFeatureExtractor")
	object := org.Objects["AIFeatureExtractor"]
	object.Definition.Fields["ProjectField__c"] = Field{APIName: "ProjectField__c", Type: FieldString}
	object.Records[ID("a00000000000001")] = Record{Fields: map[string]Value{"ProjectField__c": StringValue("kept")}}
	org.Objects["AIFeatureExtractor"] = object
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	object, ok := org.Objects["AIFeatureExtractor"]
	if !ok || len(object.Records) != 1 || object.Definition.Fields["ProjectField__c"].APIName == "" {
		t.Fatal("shape removed project metadata or records")
	}
}

func TestPlainDeveloperSpreadPreservesProjectSchemaWithoutRecords(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "AIFeatureExtractor")
	object := org.Objects["AIFeatureExtractor"]
	object.Definition.Fields["ProjectField__c"] = Field{APIName: "ProjectField__c", Type: FieldString}
	org.Objects["AIFeatureExtractor"] = object
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	object, ok := org.Objects["AIFeatureExtractor"]
	if !ok || object.Definition.Fields["ProjectField__c"].APIName == "" {
		t.Fatal("shape removed project schema")
	}
}

func TestPlainDeveloperShapePreservesExplicitObjectWithoutSpread(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "AIFeatureExtractor")
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := org.Objects["AIFeatureExtractor"]; !ok {
		t.Fatal("shape removed an explicitly installed object")
	}
}

func TestPlainDeveloperSpreadRetainsModifiedCatalogObjects(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObjectSpread(&org)
	object := org.Objects["AIFeatureExtractor"]
	object.Definition.Fields["ProjectField__c"] = Field{APIName: "ProjectField__c", Type: FieldString}
	org.Objects["AIFeatureExtractor"] = object
	object = org.Objects["AIResearchPromptResult"]
	object.Records[ID("a00000000000001")] = Record{Fields: map[string]Value{"Value__c": StringValue("kept")}}
	org.Objects["AIResearchPromptResult"] = object
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if org.Objects["AIFeatureExtractor"].Definition.Fields["ProjectField__c"].APIName == "" {
		t.Error("shape removed project schema from the spread")
	}
	if len(org.Objects["AIResearchPromptResult"].Records) != 1 {
		t.Error("shape removed project records from the spread")
	}
}

func TestPlainDeveloperSpreadShapeTransition(t *testing.T) {
	org := sourceSpreadEagerOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored OrgState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	clone := restored.CloneRuntime()
	ApplyProjectOrgShape(&clone, []string{project.UnspecifiedStandardFieldShape})
	if _, ok := clone.Objects["AIFeatureExtractor"]; !ok {
		t.Fatal("leaving the plain shape did not restore the eager spread")
	}
}

func sourceSpreadEagerOrg(features []string) OrgState {
	org := NewOrgState()
	org.OrgID = DefaultOrgID
	EnsureStandardObjectSpread(&org)
	ApplyProjectOrgShape(&org, features)
	return org
}

func sourceSpreadLazyOrg(features []string) OrgState {
	org := NewOrgState()
	org.OrgID = DefaultOrgID
	org.EnableStandardSpreadSource()
	ApplyProjectOrgShape(&org, features)
	return org
}

func compareCompleteOrgs(t *testing.T, eager, lazy OrgState) {
	t.Helper()
	eagerNames, lazyNames := sortedObjectNames(eager), sortedObjectNames(lazy)
	if !reflect.DeepEqual(eagerNames, lazyNames) {
		t.Fatalf("object sets differ: eager %d, lazy %d; first only-eager/only-lazy: %v", len(eagerNames), len(lazyNames), symmetricDifference(eagerNames, lazyNames))
	}
	for _, name := range eagerNames {
		want := definitionJSON(t, eager.Objects[name].Definition)
		got := definitionJSON(t, lazy.Objects[name].Definition)
		if want != got {
			t.Errorf("%s: definition differs %s", name, firstDifference(want, got))
		}
		if len(eager.Objects[name].Records) != len(lazy.Objects[name].Records) {
			t.Errorf("%s: record counts differ: eager %d, lazy %d", name, len(eager.Objects[name].Records), len(lazy.Objects[name].Records))
		}
	}
	if !reflect.DeepEqual(eager.Features, lazy.Features) {
		t.Errorf("features differ: eager %v, lazy %v", eager.Features, lazy.Features)
	}
	for id, record := range eager.Objects["RecordType"].Records {
		got, ok := lazy.Objects["RecordType"].Records[id]
		if !ok || !reflect.DeepEqual(record.Fields, got.Fields) {
			t.Errorf("RecordType row %s differs", id)
		}
	}
}

func symmetricDifference(a, b []string) []string {
	in := map[string]int{}
	for _, n := range a {
		in[n] |= 1
	}
	for _, n := range b {
		in[n] |= 2
	}
	var out []string
	for n, bits := range in {
		if bits != 3 {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

// A source-backed lazy org that is completed holds exactly what the eager
// spread then shape holds, whichever objects were touched first.
func TestStandardSpreadSourceMatchesEagerAfterCompletion(t *testing.T) {
	for name, features := range lazySpreadShapes {
		t.Run(name, func(t *testing.T) {
			eager := sourceSpreadEagerOrg(features)
			lazy := sourceSpreadLazyOrg(features)
			EnsureUniqueKeyPrefixes(&eager)
			EnsureUniqueKeyPrefixes(&lazy)
			if len(lazy.Objects) >= len(eager.Objects)/4 {
				t.Fatalf("lazy org starts with %d objects, eager has %d", len(lazy.Objects), len(eager.Objects))
			}
			stampBefore := lazy.RuntimeSchemaStamp
			for _, objectName := range []string{"opportunity", "AsyncApexJob", "Case"} {
				if resolved, ok := ResolveObjectName(lazy, objectName); !ok {
					t.Fatalf("%s did not resolve on a lazy org (resolved %q)", objectName, resolved)
				}
			}
			EnsureStandardObject(&lazy, "Entitlement")
			if lazy.RuntimeSchemaStamp != stampBefore {
				t.Fatal("installing from the source changed the runtime schema stamp")
			}
			if !MaterializeStandardObjectSpread(&lazy) {
				t.Fatal("materialize reported no change")
			}
			if lazy.LazyStandardSpreadEnabled() || MaterializeStandardObjectSpread(&lazy) {
				t.Fatal("org still lazy after materialize")
			}
			compareCompleteOrgs(t, eager, lazy)
		})
	}
}

// Each object resolved on demand carries the definition the eager org holds.
func TestStandardSpreadSourceObjectByObject(t *testing.T) {
	for name, features := range lazySpreadShapes {
		t.Run(name, func(t *testing.T) {
			eager := sourceSpreadEagerOrg(features)
			lazy := sourceSpreadLazyOrg(features)
			EnsureUniqueKeyPrefixes(&eager)
			EnsureUniqueKeyPrefixes(&lazy)
			for _, objectName := range sortedObjectNames(eager) {
				if _, ok := ResolveObjectName(lazy, objectName); !ok {
					t.Errorf("%s: not resolved on the lazy org", objectName)
					continue
				}
				want := definitionJSON(t, eager.Objects[objectName].Definition)
				got := definitionJSON(t, lazy.Objects[objectName].Definition)
				if want != got {
					t.Errorf("%s: definition differs %s", objectName, firstDifference(want, got))
				}
			}
			for _, objectName := range []string{"Network", "NetworkMember", "Quote", "CampaignInfluence", "NoSuchObject__c"} {
				if _, eagerHas := eager.Objects[objectName]; eagerHas {
					continue
				}
				if _, ok := ResolveObjectName(lazy, objectName); ok {
					t.Errorf("%s: resolves on the lazy org but not on the eager org", objectName)
				}
			}
		})
	}
}

// The prefix assignment depends on the objects outside the catalog too: a
// custom object takes the next free prefix in name order. A lazy org that has
// custom objects must assign what the eager org assigns.
func TestStandardSpreadSourceKeyPrefixesWithCustomObjects(t *testing.T) {
	addCustom := func(org *OrgState) {
		for _, name := range []string{"Alpha__c", "Zeta__c", "Mid__mdt"} {
			org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name, Label: name, KeyPrefix: "a00"}, Records: map[ID]Record{}}
		}
	}
	eager := sourceSpreadEagerOrg(nil)
	lazy := sourceSpreadLazyOrg(nil)
	addCustom(&eager)
	addCustom(&lazy)
	EnsureUniqueKeyPrefixes(&eager)
	EnsureUniqueKeyPrefixes(&lazy)
	for _, name := range []string{"CaseOwnerSharingRule", "Entitlement"} {
		if _, ok := ResolveObjectName(lazy, name); !ok {
			t.Fatalf("%s did not resolve", name)
		}
	}
	MaterializeStandardObjectSpread(&lazy)
	compareCompleteOrgs(t, eager, lazy)
}

// Completing a prefix-validated lazy org must not rebuild the key prefix plan
// for every object it installs: that made completion quadratic in the number
// of objects held.
func TestStandardSpreadSourceMaterializeBuildsPlanOnce(t *testing.T) {
	lazy := sourceSpreadLazyOrg(nil)
	EnsureUniqueKeyPrefixes(&lazy)
	if !lazy.keyPrefixesValidated {
		t.Fatal("org prefixes are not validated")
	}
	before := spreadPlanKeyBuilds.Load()
	started := time.Now()
	if !MaterializeStandardObjectSpread(&lazy) {
		t.Fatal("materialize reported no change")
	}
	if builds := spreadPlanKeyBuilds.Load() - before; builds > 2 {
		t.Fatalf("materialize looked up the key prefix plan %d times; want at most 2", builds)
	}
	if elapsed := time.Since(started); elapsed > 20*time.Second {
		t.Fatalf("materialize took %v", elapsed)
	}
	eager := sourceSpreadEagerOrg(nil)
	EnsureUniqueKeyPrefixes(&eager)
	compareCompleteOrgs(t, eager, lazy)
}

// The plan memo names a seed map by address, so an entry must keep its seed
// alive and a lookup must match that seed's identity: a later map at a reused
// address cannot read another seed's plan.
func TestStandardSpreadPlanMemoHoldsSeedIdentity(t *testing.T) {
	lazy := sourceSpreadLazyOrg(nil)
	EnsureUniqueKeyPrefixes(&lazy)
	if lazy.spreadPrefixSeed == nil {
		t.Fatal("validated lazy org has no prefix seed")
	}
	plan := spreadKeyPrefixPlan(&lazy)
	spreadPrefixPlans.mu.Lock()
	var held bool
	for _, entry := range spreadPrefixPlans.plans {
		if reflect.ValueOf(entry.seed).Pointer() == reflect.ValueOf(lazy.spreadPrefixSeed).Pointer() {
			held = true
		}
	}
	spreadPrefixPlans.mu.Unlock()
	if !held {
		t.Fatal("plan memo does not hold the seed map its key names")
	}
	// An org with a different seed map never reads that entry, even if the
	// two keys were equal.
	other := lazy
	other.spreadPrefixSeed = map[string]string{}
	if reflect.ValueOf(spreadKeyPrefixPlan(&other)).Pointer() == reflect.ValueOf(plan).Pointer() {
		t.Fatal("an org with another seed read this seed's plan")
	}
}

// The fast check is exact only while every recorded spread definition equals
// an isolated catalog build of that object.
func TestPlainDeveloperSpreadRecordedDefinitionsMatchCatalogBuild(t *testing.T) {
	org := NewOrgState()
	org.OrgID = DefaultOrgID
	EnsureStandardObjectSpread(&org)
	if len(org.spreadInstalled) == 0 {
		t.Fatal("eager spread recorded no definitions")
	}
	for name, installed := range org.spreadInstalled {
		if !sameDefinitionInstance(org.Objects[name].Definition, installed) {
			t.Fatalf("%s: recorded definition is not the installed one", name)
		}
		baseline := NewOrgState()
		ensureStandardObjectWithProfile(&baseline, name, nil)
		if !reflect.DeepEqual(installed.definition, baseline.Objects[name].Definition) {
			t.Errorf("%s: spread definition differs from an isolated catalog build", name)
		}
	}
}

func TestSameDefinitionInstanceSeesEveryField(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "Account")
	base := org.Objects["Account"].Definition
	recorded := recordInstalledSpreadDefinition(base)
	if !sameDefinitionInstance(base, recorded) {
		t.Fatal("a definition is not the same instance as itself")
	}
	typ := reflect.TypeOf(base)
	for i := 0; i < typ.NumField(); i++ {
		changed := base
		field := reflect.ValueOf(&changed).Elem().Field(i)
		switch field.Kind() {
		case reflect.String:
			field.SetString(field.String() + "x")
		case reflect.Bool:
			field.SetBool(!field.Bool())
		case reflect.Map:
			field.Set(reflect.MakeMapWithSize(field.Type(), field.Len()))
			iter := reflect.ValueOf(base).Field(i).MapRange()
			for iter.Next() {
				field.SetMapIndex(iter.Key(), iter.Value())
			}
		case reflect.Slice:
			field.Set(reflect.MakeSlice(field.Type(), field.Len()+1, field.Len()+1))
		default:
			t.Fatalf("%s: unhandled kind %s", typ.Field(i).Name, field.Kind())
		}
		if sameDefinitionInstance(changed, recorded) {
			t.Errorf("%s: change not detected", typ.Field(i).Name)
		}
	}
	grown := base.Clone()
	org.Objects["Account"] = ObjectState{Definition: grown}
	recorded = recordInstalledSpreadDefinition(grown)
	grown.Fields["Project__c"] = Field{APIName: "Project__c", Type: FieldString}
	grown.Metadata["project"] = "true"
	if sameDefinitionInstance(grown, recorded) {
		t.Error("entries added in place were not detected")
	}
}

func TestPlainDeveloperSpreadRetainsCopiedCatalogDefinition(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObjectSpread(&org)
	object := org.Objects["AIFeatureExtractor"]
	object.Definition = object.Definition.Clone()
	id := object.Definition.Fields["Id"]
	id.Label = "Project ID"
	object.Definition.Fields["Id"] = id
	org.Objects["AIFeatureExtractor"] = object
	object = org.Objects["AIResearchPromptResult"]
	object.Definition = object.Definition.Clone()
	org.Objects["AIResearchPromptResult"] = object
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if org.Objects["AIFeatureExtractor"].Definition.Fields["Id"].Label != "Project ID" {
		t.Error("shape removed a copied and changed catalog definition")
	}
	if _, ok := org.Objects["AIResearchPromptResult"]; ok {
		t.Error("shape kept a copied but unchanged catalog definition")
	}
}

// The recorded definitions only skip the rebuild; the shaped org is the same.
func TestPlainDeveloperSpreadRecordedDefinitionsKeepOutcome(t *testing.T) {
	features := []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}
	fast := sourceSpreadEagerOrg(features)
	full := NewOrgState()
	full.OrgID = DefaultOrgID
	EnsureStandardObjectSpread(&full)
	full.spreadInstalled = nil
	ApplyProjectOrgShape(&full, features)
	if len(fast.Objects) != len(full.Objects) {
		t.Fatalf("objects: fast %d, full %d", len(fast.Objects), len(full.Objects))
	}
	fastJSON, err := json.Marshal(fast)
	if err != nil {
		t.Fatal(err)
	}
	fullJSON, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	if string(fastJSON) != string(fullJSON) {
		t.Fatal("recorded definitions changed the shaped org")
	}
}
