package storage

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// referenceFieldsFromFullDescribe is the lookup as it was before the narrow
// decode: every field of the fully decoded member is inspected.
func referenceFieldsFromFullDescribe(describe standardDescribeObject) []Field {
	var fields []Field
	for _, captured := range describe.Fields {
		if !strings.EqualFold(captured.Type, "reference") || (captured.DeprecatedAndHidden != nil && *captured.DeprecatedAndHidden) {
			continue
		}
		if !standardStubReferenceTargetsAvailable(captured.ReferenceTo) {
			continue
		}
		field := describeField(captured)
		field.Required = false
		fields = append(fields, field)
	}
	return fields
}

func currencyNamesFromFullDescribe(describe standardDescribeObject) []string {
	var names []string
	for _, captured := range describe.Fields {
		if strings.EqualFold(captured.Type, "currency") {
			names = append(names, captured.Name)
		}
	}
	return names
}

// Every catalog member: the narrow scan equals what the full decode yields,
// and the production lookup (which uses the narrow path) equals the old
// full-decode computation for every feature signature the filter takes.
func TestStandardDescribeNarrowMatchesFullDecodeForEveryMember(t *testing.T) {
	signatures := []string{"", "PersonAccounts", "MultiCurrency", "PersonAccounts,MultiCurrency"}
	for _, entry := range standardDescribeCatalogV2Index {
		full, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatal(err)
		}
		wantRefs, wantCurrency := referenceFieldsFromFullDescribe(full), currencyNamesFromFullDescribe(full)
		narrow, err := decodeStandardDescribeCatalogV2Narrow(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatalf("%s: %v", entry.Name, err)
		}
		if !reflect.DeepEqual(narrow.references, wantRefs) || !reflect.DeepEqual(narrow.currency, wantCurrency) {
			t.Fatalf("%s: narrow decode differs from full decode\nrefs  got %+v\nrefs want %+v\ncurrency got %v want %v", entry.Name, narrow.references, wantRefs, narrow.currency, wantCurrency)
		}
		if !standardSObjectStubHasFields(entry.Name) {
			continue
		}
		got := standardStubReferenceFieldsFromV2Describe(entry.Name)
		if !reflect.DeepEqual(got, wantRefs) {
			t.Fatalf("%s: standardStubReferenceFieldsFromV2Describe differs from the full-decode result", entry.Name)
		}
		for _, signature := range signatures {
			var gotNames, wantNames []string
			for _, field := range got {
				if standardStubReferenceFieldEnabledForSignature(entry.Name, field.APIName, signature) {
					gotNames = append(gotNames, field.APIName)
				}
			}
			for _, field := range wantRefs {
				if standardStubReferenceFieldEnabledForSignature(entry.Name, field.APIName, signature) {
					wantNames = append(wantNames, field.APIName)
				}
			}
			if !reflect.DeepEqual(gotNames, wantNames) {
				t.Fatalf("%s with %q: enabled reference fields %v, want %v", entry.Name, signature, gotNames, wantNames)
			}
		}
		stubs, _ := standardSObjectStubFieldsFor(entry.Name)
		var wantStubCurrency []Field
		for _, captured := range full.Fields {
			stub, exists := stubs[captured.Name]
			if exists && stub.Type == FieldDecimal && strings.EqualFold(stub.DisplayType, "DOUBLE") && strings.EqualFold(captured.Type, "currency") {
				wantStubCurrency = append(wantStubCurrency, stub)
			}
		}
		if got := standardStubCurrencyCatalogFields(entry.Name); !reflect.DeepEqual(got, wantStubCurrency) {
			t.Fatalf("%s: standardStubCurrencyCatalogFields differs from the full-decode result", entry.Name)
		}
	}
}

// Input outside the plain shape makes the fast scan decline or must agree with
// encoding/json; the production path then uses encoding/json.
func TestStandardDescribeNarrowFastScanAgreesOrDeclines(t *testing.T) {
	field := func(body string) string { return `{"name":"N","fields":[` + body + `]}` }
	cases := map[string]string{
		"reference":          field(`{"name":"OwnerId","type":"reference","referenceTo":["User"],"relationshipName":"Owner","nillable":false,"createable":true,"defaultedOnCreate":true}`),
		"hidden reference":   field(`{"name":"A","type":"reference","referenceTo":["User"],"deprecatedAndHidden":true}`),
		"hidden false":       field(`{"name":"A","type":"reference","referenceTo":["User"],"deprecatedAndHidden":false}`),
		"hidden null":        field(`{"name":"A","type":"reference","referenceTo":["User"],"deprecatedAndHidden":null}`),
		"unknown target":     field(`{"name":"A","type":"reference","referenceTo":["NoSuchObjectAnywhere"]}`),
		"mixed case type":    field(`{"name":"A","type":"Reference","referenceTo":["User"]}`),
		"currency":           field(`{"name":"Amt","type":"currency"}`),
		"null type":          field(`{"name":"A","type":null}`),
		"picklist skipped":   field(`{"name":"P","type":"picklist","picklistValues":[{"value":"a","label":"a","active":true}]}`),
		"fields null":        `{"name":"N","fields":null}`,
		"no fields":          `{"name":"N"}`,
		"duplicate type":     field(`{"name":"A","type":"reference","type":"string","referenceTo":["User"]}`),
		"folded key":         field(`{"name":"A","Type":"reference","referenceTo":["User"]}`),
		"escaped key":        field(`{"name":"A","` + `\` + `u0074ype":"reference","referenceTo":["User"]}`),
		"escaped value":      field(`{"name":"A","type":"refer` + `\` + `u0065nce","referenceTo":["User"]}`),
		"duplicate fields":   `{"fields":[],"fields":[]}`,
		"null element":       field(`null`),
		"extra unknown keys": `{"zzz":{"a":[1,2,{"b":null}]},"fields":[{"x":1,"name":"C","type":"currency","y":[true]}]}`,
		"trailing garbage":   `{"fields":[]} x`,
	}
	for name, doc := range cases {
		var full standardDescribeObject
		fullErr := json.Unmarshal([]byte(doc), &full)
		fast, ok := fastDecodeStandardDescribeNarrow([]byte(doc))
		if !ok {
			continue
		}
		if fullErr != nil {
			t.Errorf("%s: fast scan accepted input that encoding/json rejects: %v", name, fullErr)
			continue
		}
		want := standardDescribeNarrowFromDescribe(full)
		if !reflect.DeepEqual(fast, want) {
			t.Errorf("%s: fast scan %+v, encoding/json %+v", name, fast, want)
		}
	}
	for _, declined := range []string{"duplicate type", "folded key", "escaped key", "escaped value", "duplicate fields", "null element", "trailing garbage"} {
		if _, ok := fastDecodeStandardDescribeNarrow([]byte(cases[declined])); ok {
			t.Errorf("%s: fast scan should decline", declined)
		}
	}
	if _, ok := fastDecodeStandardDescribeNarrow([]byte(cases["reference"])); !ok {
		t.Error("plain reference field: fast scan should accept")
	}
}

// Learning the reference fields of every stub object must not run a full
// member decode, and a later full lookup decodes each member exactly once.
func TestStandardStubReferenceLookupDefersFullDecode(t *testing.T) {
	names := largestStandardStubDescribeObjectNames(t, 40)
	standardDescribeCatalogV2RawProductionCache.release()
	fullBefore, narrowBefore := standardDescribeFullDecodes.Load(), standardDescribeNarrowDecodes.Load()
	for round := 0; round < 3; round++ {
		for _, name := range names {
			standardStubReferenceFieldsFromV2Describe(name)
			standardStubCurrencyCatalogFields(name)
		}
	}
	if got := standardDescribeFullDecodes.Load() - fullBefore; got != 0 {
		t.Fatalf("%d full decodes during narrow lookups, want 0", got)
	}
	if got := standardDescribeNarrowDecodes.Load() - narrowBefore; got != uint64(len(names)) {
		t.Fatalf("%d narrow decodes for %d objects over 3 rounds, want one each", got, len(names))
	}
	for round := 0; round < 2; round++ {
		for _, name := range names {
			if _, ok, err := lookupStandardDescribeCatalogV2(name); !ok || err != nil {
				t.Fatalf("%s: %v %v", name, ok, err)
			}
		}
	}
	if got := standardDescribeFullDecodes.Load() - fullBefore; got != uint64(len(names)) {
		t.Fatalf("%d full decodes after full lookups, want %d", got, len(names))
	}
	// Once the member is fully decoded, the narrow answer comes from it.
	narrowAfterFull := standardDescribeNarrowDecodes.Load()
	standardDescribeCatalogV2RawProductionCache.release()
	lookupStandardDescribeCatalogV2(names[0])
	got := standardStubReferenceFieldsFromV2Describe(names[0])
	if standardDescribeNarrowDecodes.Load() != narrowAfterFull {
		t.Fatal("a fully decoded member was scanned again for the narrow answer")
	}
	full, _, _ := lookupStandardDescribeCatalogV2(names[0])
	if !reflect.DeepEqual(got, referenceFieldsFromFullDescribe(full)) {
		t.Fatal("narrow answer taken from the full decode differs")
	}
}

// The narrow result is shared by the check plan and the stub merge. The merge
// must not write through it.
func TestStandardStubReferenceMergeDoesNotMutateSharedNarrowResult(t *testing.T) {
	for _, name := range largestStandardStubDescribeObjectNames(t, 80) {
		shared := standardStubReferenceFieldsFromV2Describe(name)
		if len(shared) == 0 {
			continue
		}
		want := make([]Field, len(shared))
		for i, field := range shared {
			want[i] = cloneField(field)
		}
		stubs, _ := standardSObjectStubFieldsFor(name)
		for _, features := range [][]string{nil, {"PersonAccounts", "MultiCurrency"}} {
			definition := ObjectDefinition{APIName: name, Fields: map[string]Field{}}
			for fieldName, stub := range stubs {
				definition.Fields[fieldName] = cloneField(stub)
			}
			mergeStandardSObjectStubReferencesFromV2Describe(&definition, features)
			// Mutate every slice the merge produced.
			for fieldName, field := range definition.Fields {
				for i := range field.ReferenceTo {
					field.ReferenceTo[i] = "Mutated"
				}
				definition.Fields[fieldName] = field
			}
		}
		fresh, _, _ := lookupStandardDescribeNarrowV2(name)
		if !reflect.DeepEqual(fresh.references, want) {
			t.Fatalf("%s: merging changed the shared narrow result", name)
		}
		return
	}
	t.Skip("no stub object with reference fields")
}

func TestStandardDescribeNarrowConcurrentLookups(t *testing.T) {
	names := largestStandardStubDescribeObjectNames(t, 30)
	standardDescribeCatalogV2RawProductionCache.release()
	want := make(map[string][]Field, len(names))
	for _, name := range names {
		entry, _ := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, name)
		full, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatal(err)
		}
		want[name] = referenceFieldsFromFullDescribe(full)
	}
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for i := range names {
				name := names[(i+worker)%len(names)]
				got := standardStubReferenceFieldsFromV2Describe(name)
				if worker%2 == 0 {
					lookupStandardDescribeCatalogV2(name)
				}
				if !reflect.DeepEqual(got, want[name]) {
					t.Errorf("%s: concurrent narrow lookup differs", name)
					return
				}
			}
		}(worker)
	}
	group.Wait()
}
