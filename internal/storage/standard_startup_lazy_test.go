package storage

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func startupFullProfiles() map[string]standardAPI68Profile {
	r, err := gzip.NewReader(bytes.NewReader(standardAPI68DescribeData))
	if err != nil {
		panic(err)
	}
	defer r.Close()
	var bundle standardAPI68Bundle
	if err := json.NewDecoder(r).Decode(&bundle); err != nil {
		panic(err)
	}
	return bundle.Profiles
}

func TestStandardStartupProfileLazyIdentity(t *testing.T) {
	want := startupFullProfiles()
	standardAPI68Once = sync.Once{}
	standardAPI68Profiles = nil
	StandardAPI68DescribeFields("P0", "Account", false)
	if len(standardAPI68Profiles) != 1 {
		t.Fatalf("decoded %d profiles on first access, want 1", len(standardAPI68Profiles))
	}
	for name, profile := range want {
		got, ok := loadStandardAPI68Profile(name)
		if !ok || !reflect.DeepEqual(got, profile) {
			t.Fatalf("profile %q differs from full decode", name)
		}
	}
}

func TestStandardStartupPicklistIdentity(t *testing.T) {
	var doc struct {
		Objects map[string]map[string]standardStubRestrictedPicklist `json:"objects"`
	}
	if err := json.Unmarshal(standardStubRestrictedPicklistsData, &doc); err != nil {
		t.Fatal(err)
	}
	objects := make(map[string]struct{}, len(doc.Objects))
	for object := range doc.Objects {
		objects[object] = struct{}{}
	}
	for _, object := range standardSObjectStubNames() {
		objects[object] = struct{}{}
	}
	for object := range objects {
		var fields map[string]standardStubRestrictedPicklist
		for name, entries := range doc.Objects {
			if strings.EqualFold(name, object) {
				fields = entries
				break
			}
		}
		stubs, _ := standardSObjectStubFieldsFor(object)
		var want map[string]standardStubRestrictedPicklist
		for name, stub := range stubs {
			for original, value := range fields {
				if strings.EqualFold(name, original) && stub.Type == FieldString {
					if want == nil {
						want = map[string]standardStubRestrictedPicklist{}
					}
					want[name] = value
				}
			}
		}
		for _, name := range []string{object, strings.ToLower(object)} {
			if got := standardStubRestrictedPicklistFields(name); !reflect.DeepEqual(got, want) {
				t.Fatalf("object %q differs", name)
			}
		}
	}
	makeDefinition := func() ObjectDefinition {
		def := ObjectDefinition{APIName: "EmailMessage", Fields: map[string]Field{"Status": {APIName: "Status", Type: FieldString}}}
		applyStandardStubRestrictedPicklists(&def)
		return def
	}
	first := makeDefinition()
	values := first.Fields["Status"].PicklistValues
	if len(values) == 0 {
		t.Fatal("expected captured picklist values")
	}
	original := values[0].Value
	values[0].Value = "changed"
	if got := makeDefinition().Fields["Status"].PicklistValues[0].Value; got != original {
		t.Fatal("returned values changed the cached catalog")
	}
	if got := standardStubRestrictedPicklistFields("not-an-object"); got != nil {
		t.Fatal("unknown object returned fields")
	}
}

func BenchmarkStandardStartupProfile(b *testing.B) {
	for i := 0; i < b.N; i++ {
		standardAPI68Once = sync.Once{}
		standardAPI68Profiles = nil
		StandardAPI68DescribeFields("P0", "Account", false)
	}
}

func BenchmarkStandardStartupPicklist(b *testing.B) {
	for i := 0; i < b.N; i++ {
		standardStubRestrictedPicklists = newStandardStubRestrictedPicklists()
		standardStubRestrictedPicklistFields("Account")
	}
}

func TestStandardStartupConcurrentLookups(t *testing.T) {
	profiles := startupFullProfiles()
	var wg sync.WaitGroup
	for name, want := range profiles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				got, ok := loadStandardAPI68Profile(name)
				if !ok || !reflect.DeepEqual(got, want) {
					t.Errorf("profile %q differs", name)
					return
				}
				standardStubRestrictedPicklistFields("Account")
				standardStubRestrictedPicklistFields("not-an-object")
			}
		}()
	}
	wg.Wait()
}
