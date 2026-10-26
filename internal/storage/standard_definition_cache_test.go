package storage

import (
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// An allocated field map keeps this reference on the project-overlay route.
func uncachedCatalogDefinition(name string, features []string) ObjectDefinition {
	d := ObjectDefinition{APIName: name, Fields: map[string]Field{}}
	EnsureStandardObjectFieldsForFeatures(&d, features)
	return d
}

func TestStandardDefinitionCacheCatalogAndFeatureRoutes(t *testing.T) {
	profiles := [][]string{nil, {"PersonAccounts"}, {"MultiCurrency", "StateAndCountryPicklist"}, {"Communities", "Chatter"}, {"PersonAccounts", "HealthCloudAddOn"}}
	for _, name := range []string{"Account", "Contact", "Lead", "Task", "Event", "Opportunity", "User", "CareProgram", "UserLicense", "PermissionSet", "AccountTeamMember", "Knowledge__kav"} {
		for _, features := range profiles {
			t.Run(name+"/"+canonicalFeatureSignature(features), func(t *testing.T) {
				want := uncachedCatalogDefinition(name, features)
				for i := 0; i < 2; i++ {
					got := ObjectDefinition{APIName: name}
					EnsureStandardObjectFieldsForFeatures(&got, features)
					if !reflect.DeepEqual(got, want) {
						t.Fatal("cached feature overlay differs from uncached overlay")
					}
					before, err := json.Marshal(want)
					if err != nil {
						t.Fatal(err)
					}
					mutateCatalogValue(reflect.ValueOf(&got).Elem())
					next := ObjectDefinition{APIName: name}
					EnsureStandardObjectFieldsForFeatures(&next, features)
					after, _ := json.Marshal(next)
					if string(before) != string(after) {
						t.Fatal("caller mutation corrupted the feature overlay")
					}
				}
				raw, _ := json.Marshal(want)
				t.Logf("outcome=%x", sha256.Sum256(raw))
				if len(features) != 0 {
					return
				}
				if want.Label == "" {
					want.Label = name
				}
				if want.PluralLabel == "" {
					want.PluralLabel = name + "s"
				}
				if want.KeyPrefix == "" {
					want.KeyPrefix = StandardKeyPrefix(name)
				}
				for _, alias := range []string{name, strings.ToLower(name), strings.ToUpper(name)} {
					got, ok := StandardObjectDefinition(alias)
					if !ok || !reflect.DeepEqual(got, want) {
						t.Fatalf("catalog route %s differs from uncached definition", alias)
					}
					mutateCatalogValue(reflect.ValueOf(&got).Elem())
					next, _ := StandardObjectDefinition(name)
					if !reflect.DeepEqual(next, want) {
						t.Fatal("caller mutation corrupted the catalog cache")
					}
				}
			})
		}
	}
	if _, ok := StandardObjectDefinition("MissingCatalogProbe__c"); ok {
		t.Fatal("unknown object accepted")
	}
}

// Walk every public mutable leaf, including nested field flags, picklists, reference
// targets, relationship slices, record-type maps, and overlay metadata.
func mutateCatalogValue(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			mutateCatalogValue(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !v.Type().Field(i).IsExported() {
				continue
			}
			mutateCatalogValue(v.Field(i))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			item := reflect.New(v.Type().Elem()).Elem()
			item.Set(v.MapIndex(key))
			mutateCatalogValue(item)
			v.SetMapIndex(key, item)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			mutateCatalogValue(v.Index(i))
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString("mutated")
		}
	case reflect.Bool:
		if v.CanSet() {
			v.SetBool(!v.Bool())
		}
	case reflect.Int:
		if v.CanSet() {
			v.SetInt(v.Int() + 1)
		}
	}
}

var catalogDefinitionSink ObjectDefinition

func TestStandardDefinitionCacheWarmAllocations(t *testing.T) {
	for _, route := range []string{"catalog", "feature_overlay"} {
		t.Run(route, func(t *testing.T) {
			features := []string(nil)
			if route == "feature_overlay" {
				features = []string{"PersonAccounts", "StateAndCountryPicklist"}
			}
			read := func() {
				if route == "catalog" {
					catalogDefinitionSink, _ = StandardObjectDefinition("Account")
				} else {
					catalogDefinitionSink = ObjectDefinition{APIName: "Account"}
					EnsureStandardObjectFieldsForFeatures(&catalogDefinitionSink, features)
				}
			}
			read()
			cached := testing.AllocsPerRun(5, read)
			uncached := testing.AllocsPerRun(5, func() { catalogDefinitionSink = uncachedCatalogDefinition("Account", features) })
			t.Logf("allocations cached=%.0f uncached=%.0f", cached, uncached)
			if cached >= uncached*0.75 {
				t.Fatalf("warm %s still rebuilds catalog: %.0f allocations, uncached %.0f (want <75%%)", route, cached, uncached)
			}
		})
	}
}

func TestStandardDefinitionCacheConcurrentCatalogAndFeatureRoutes(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			for j := 0; j < 3; j++ {
				d, ok := StandardObjectDefinition("account")
				if !ok || d.APIName != "Account" {
					t.Error("canonical catalog lookup failed")
					return
				}
				d.Fields["Name"] = Field{APIName: "LocalMutation"}
				overlay := ObjectDefinition{APIName: "Account"}
				EnsureStandardObjectFieldsForFeatures(&overlay, []string{"PersonAccounts"})
				if overlay.Fields["Name"].APIName != "Name" {
					t.Error("concurrent caller corrupted overlay")
				}
			}
		})
	}
	wg.Wait()
}

func BenchmarkStandardDefinitionWarmCatalog(b *testing.B) {
	_, _ = StandardObjectDefinition("Account")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		catalogDefinitionSink, _ = StandardObjectDefinition("Account")
	}
}

func BenchmarkStandardDefinitionWarmFeatureOverlay(b *testing.B) {
	features := []string{"PersonAccounts", "StateAndCountryPicklist"}
	catalogDefinitionSink = ObjectDefinition{APIName: "Account"}
	EnsureStandardObjectFieldsForFeatures(&catalogDefinitionSink, features)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		catalogDefinitionSink = ObjectDefinition{APIName: "Account"}
		EnsureStandardObjectFieldsForFeatures(&catalogDefinitionSink, features)
	}
}

func TestStandardDefinitionCacheProjectOverlayRoute(t *testing.T) {
	for _, name := range []string{"Account", "account", "Probe__c"} {
		d := ObjectDefinition{APIName: name, Label: "Project label", Fields: map[string]Field{
			"Owned__c": {APIName: "Owned__c", Type: FieldString, Length: 27},
		}, Metadata: map[string]string{"owned": "kept"}, RecordTypes: []RecordTypeInfo{{DeveloperName: "Owned"}}}
		EnsureStandardObjectFieldsForFeatures(&d, []string{"PersonAccounts"})
		if d.APIName != name || d.Label != "Project label" || d.Fields["Owned__c"].Length != 27 || d.Metadata["owned"] != "kept" || d.RecordTypes[0].DeveloperName != "Owned" {
			t.Fatal("catalog cache replaced project-owned metadata")
		}
	}
	for _, features := range [][]string{{"MultiCurrency", "PersonAccounts"}, {"personaccounts", "MultiCurrency", "PersonAccounts"}} {
		got := ObjectDefinition{APIName: "Account"}
		EnsureStandardObjectFieldsForFeatures(&got, features)
		want := uncachedCatalogDefinition("Account", features)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("canonical feature profile changed the overlay")
		}
	}
}

func TestStandardDefinitionCacheInstalledCatalogRoute(t *testing.T) {
	original := standardDescribeCatalogV2ProductionCache
	defer func() { standardDescribeCatalogV2ProductionCache = original }()
	_, _ = StandardObjectDefinition("CareProgram")
	for _, label := range []string{"First catalog", "Second catalog"} {
		standardDescribeCatalogV2ProductionCache = newStandardDescribeCatalogV2Cache(func(standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
			return standardObjectCatalogEntry{Definition: ObjectDefinition{APIName: "CareProgram", Label: label}}, nil
		})
		got, ok := StandardObjectDefinition("careprogram")
		if !ok || got.Label != label {
			t.Fatalf("installed catalog label=%q, want %q", got.Label, label)
		}
	}
}
