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
		got, ok := loadStandardAPI68Profiles()[name]
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
	for object, fields := range doc.Objects {
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
		if got := standardStubRestrictedPicklistFields(object); !reflect.DeepEqual(got, want) {
			t.Fatalf("object %q differs", object)
		}
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
		standardStubRestrictedPicklists = sync.OnceValue(func() map[string]map[string]standardStubRestrictedPicklist {
			var doc struct {
				Objects map[string]map[string]standardStubRestrictedPicklist `json:"objects"`
			}
			if err := json.Unmarshal(standardStubRestrictedPicklistsData, &doc); err != nil {
				panic(err)
			}
			out := make(map[string]map[string]standardStubRestrictedPicklist, len(doc.Objects))
			for object, fields := range doc.Objects {
				lowered := make(map[string]standardStubRestrictedPicklist, len(fields))
				for name, field := range fields {
					lowered[strings.ToLower(name)] = field
				}
				out[strings.ToLower(object)] = lowered
			}
			return out
		})
		standardStubRestrictedPicklistFields("Account")
	}
}
