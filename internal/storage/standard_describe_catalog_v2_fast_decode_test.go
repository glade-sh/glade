package storage

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// Every generated catalog member must take the fast path and decode to exactly
// what encoding/json produces. A regenerated pack that leaves the fast path's
// shape fails here instead of silently paying the slow decode.
func TestStandardDescribeCatalogV2FastDecodeMatchesJSON(t *testing.T) {
	var fields, picklists, children, recordTypes int
	for _, indexEntry := range standardDescribeCatalogV2Index {
		member, err := decodeStandardDescribeCatalogV2MemberBytes(standardDescribeCatalogV2Pack, indexEntry, standardDescribeCatalogV2Magic, len(standardDescribeCatalogV2Index))
		if err != nil {
			t.Fatal(err)
		}
		var want standardDescribeObject
		if err := json.Unmarshal(member, &want); err != nil {
			t.Fatalf("%s: encoding/json: %v", indexEntry.Name, err)
		}
		got, ok := fastDecodeStandardDescribeObject(member)
		if !ok {
			t.Fatalf("%s: fast decode declined a generated member", indexEntry.Name)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: fast decode differs from encoding/json", indexEntry.Name)
		}
		viaCatalog, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, indexEntry)
		if err != nil || !reflect.DeepEqual(viaCatalog, want) {
			t.Fatalf("%s: catalog member decode differs from encoding/json: %v", indexEntry.Name, err)
		}
		fields += len(want.Fields)
		children += len(want.ChildRelationships)
		recordTypes += len(want.RecordTypeInfos)
		for _, field := range want.Fields {
			picklists += len(field.PicklistValues)
		}
	}
	if len(standardDescribeCatalogV2Index) == 0 || fields == 0 || picklists == 0 || children == 0 || recordTypes == 0 {
		t.Fatalf("catalog coverage objects/fields/picklists/children/recordTypes = %d/%d/%d/%d/%d",
			len(standardDescribeCatalogV2Index), fields, picklists, children, recordTypes)
	}
}

// Concurrent member decodes share pooled gzip readers; each must still decode
// exactly what a serial decode does.
func TestStandardDescribeCatalogV2ConcurrentMemberDecodesMatchSerial(t *testing.T) {
	entries := standardDescribeCatalogV2Index
	if len(entries) > 64 {
		entries = entries[:64]
	}
	want := make([]standardDescribeObject, len(entries))
	for index, entry := range entries {
		describe, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry)
		if err != nil {
			t.Fatal(err)
		}
		want[index] = describe
	}
	const workers = 8
	var wait sync.WaitGroup
	errs := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for offset := range entries {
				index := (offset + worker*len(entries)/workers) % len(entries)
				got, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entries[index])
				if err != nil {
					errs <- err
					return
				}
				if !reflect.DeepEqual(got, want[index]) {
					errs <- fmt.Errorf("%s: concurrent decode differs from serial decode", entries[index].Name)
					return
				}
			}
		}(worker)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

// standardDescribeFastDecodeSeeds covers the shapes the fast path accepts and
// the ones it must hand to encoding/json.
var standardDescribeFastDecodeSeeds = []string{
	`{}`,
	` {"name":"A"} `,
	`null`,
	`[]`,
	`"x"`,
	``,
	`{"name":"A"}x`,
	`{"name":"A",}`,
	`{"name":"A" "label":"B"}`,
	`{"name":"A","name":"B"}`,
	`{"Name":"A"}`,
	`{"NAME":"A","name":"B"}`,
	`{"name":"A"}`,
	`{"námé":"A"}`,
	`{"name":null,"label":null,"keyPrefix":"001","mergeable":null,"triggerable":true}`,
	`{"name":1}`,
	`{"mergeable":"true"}`,
	`{"name":"café \"q\" \\ \/ \n\t"}`,
	"{\"name\":\"caf\xc3\xa9\"}",
	"{\"name\":\"bad\xff utf8\"}",
	"{\"name\":\"ctl\x01\"}",
	`{"name":"😀 \ud800"}`,
	`{"name":"\x"}`,
	`{"name":"\u12"}`,
	`{"fields":null,"childRelationships":null,"recordTypeInfos":null}`,
	`{"fields":[],"childRelationships":[],"recordTypeInfos":[]}`,
	`{"fields":[null]}`,
	`{"fields":["x"]}`,
	`{"fields":[{}],"fields":[{"name":"B"}]}`,
	`{"fields":[{"name":"Id","type":"id","length":18,"precision":0,"scale":-0,"calculated":false,"nillable":false,"createable":null,"referenceTo":[],"picklistValues":[],"relationshipName":null,"compoundFieldName":null}]}`,
	`{"fields":[{"length":1.5}]}`,
	`{"fields":[{"length":1e2}]}`,
	`{"fields":[{"length":99999999999999999999}]}`,
	`{"fields":[{"length":-1}]}`,
	`{"fields":[{"length":01}]}`,
	`{"fields":[{"length":"1"}]}`,
	`{"fields":[{"defaultValue":null}]}`,
	`{"fields":[{"defaultValue":"x"}]}`,
	`{"fields":[{"defaultValue":true}]}`,
	`{"fields":[{"defaultValue":12.50}]}`,
	`{"fields":[{"defaultValue":1e999}]}`,
	`{"fields":[{"defaultValue":{"a":[1,"b",null]}}]}`,
	`{"fields":[{"defaultValue":[1,2]}]}`,
	`{"fields":[{"defaultValueFormula":"TODAY()"},{"defaultValueFormula":null}]}`,
	`{"fields":[{"referenceTo":["Account","Contact"],"polymorphicForeignKey":true}]}`,
	`{"fields":[{"referenceTo":[null]}]}`,
	`{"fields":[{"referenceTo":[1]}]}`,
	`{"fields":[{"picklistValues":[{"value":"A","label":"a","active":true,"defaultValue":false,"validFor":null}]}]}`,
	`{"fields":[{"picklistValues":[{"Value":"A"}]}]}`,
	`{"fields":[{"Type":"id"}]}`,
	`{"fields":[{"unknown":{"deep":[[[{"x":[true,false,null,-0.5e-3]}]]]}}]}`,
	`{"fields":[{"unknown":{"deep":[tru]}}]}`,
	`{"fields":[{"unknown":[1,]}]}`,
	`{"fields":[{"unknown":01}]}`,
	`{"fields":[{"unknown":-}]}`,
	`{"fields":[{"unknown":1.}]}`,
	`{"fields":[{"unknown":{"a" 1}}]}`,
	`{"fields":[{"unknown":{1:1}}]}`,
	`{"childRelationships":[{"childSObject":"Contact","field":"AccountId","relationshipName":"Contacts","cascadeDelete":true,"restrictedDelete":false,"deprecatedAndHidden":false,"junctionIdListNames":[]}]}`,
	`{"childRelationships":[{"ChildSObject":"Contact"}]}`,
	`{"recordTypeInfos":[{"recordTypeId":"012000000000000AAA","developerName":"Master","name":"Master","active":true,"available":true,"defaultRecordTypeMapping":true,"master":true,"urls":{}}]}`,
	`{"recordTypeInfos":[{"DeveloperName":"Master"}]}`,
	"{\n\t\"name\" : \"A\" ,\r\n\"fields\" : [ { \"name\" : \"Id\" } ] }",
	`{"name":"A","urls":{"a":"b"},"supportedScopes":[{"label":"x","name":"y"}],"actionOverrides":[]}`,
}

func TestStandardDescribeFastDecodeEdgeCasesMatchJSON(t *testing.T) {
	for _, seed := range standardDescribeFastDecodeSeeds {
		checkStandardDescribeFastDecodeMatchesJSON(t, []byte(seed))
	}
}

func FuzzStandardDescribeFastDecodeMatchesJSON(f *testing.F) {
	for _, seed := range standardDescribeFastDecodeSeeds {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		checkStandardDescribeFastDecodeMatchesJSON(t, data)
	})
}

func checkStandardDescribeFastDecodeMatchesJSON(t *testing.T, data []byte) {
	t.Helper()
	var want standardDescribeObject
	wantErr := json.Unmarshal(data, &want)
	if got, ok := fastDecodeStandardDescribeObject(data); ok {
		if wantErr != nil {
			t.Fatalf("%q: fast decode accepted input encoding/json rejects: %v", data, wantErr)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: fast decode = %#v, encoding/json = %#v", data, got, want)
		}
	}
	got, gotErr := decodeStandardDescribeObjectJSON(data)
	if (gotErr == nil) != (wantErr == nil) || gotErr != nil && gotErr.Error() != wantErr.Error() {
		t.Fatalf("%q: error = %v, encoding/json = %v", data, gotErr, wantErr)
	}
	if wantErr == nil && !reflect.DeepEqual(got, want) {
		t.Fatalf("%q: decode = %#v, encoding/json = %#v", data, got, want)
	}
}
