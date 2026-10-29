package vm

import (
	"encoding/json"
	"testing"
)

// Whole-record native rows address, mailingAddress and location at API62/67
// include these compound values without SObject attributes. Address nulls also
// survive both serialize overloads (sobject_json2/org.tsv).
func TestSObjectJSON2CompoundValuesNative(t *testing.T) {
	for _, tc := range []struct {
		name, typ, runtime, want string
		fields                   map[string]Value
	}{
		{"address", "System.Address", "", `{"city":"Town","country":"US","geocodeAccuracy":null,"latitude":12.5,"longitude":-45.25,"postalCode":"94000","state":"CA","street":"1 Main"}`, map[string]Value{"city": String("Town"), "country": String("US"), "latitude": Decimal(12.5), "longitude": Decimal(-45.25), "postalCode": String("94000"), "state": String("CA"), "street": String("1 Main")}},
		{"mailingAddress", "System.Address", "", `{"city":"Village","country":null,"geocodeAccuracy":null,"latitude":null,"longitude":null,"postalCode":null,"state":null,"street":"2 Main"}`, map[string]Value{"city": String("Village"), "street": String("2 Main")}},
		{"runtimeAddress", "Address", "System.Address", `{"city":"Town","country":"US","geocodeAccuracy":null,"latitude":12.5,"longitude":-45.25,"postalCode":"94000","state":"CA","street":"1 Main"}`, map[string]Value{"city": String("Town"), "country": String("US"), "latitude": Decimal(12.5), "longitude": Decimal(-45.25), "postalCode": String("94000"), "state": String("CA"), "street": String("1 Main")}},
		{"runtimeMailingAddress", "Address", "System.Address", `{"city":"Village","country":null,"geocodeAccuracy":null,"latitude":null,"longitude":null,"postalCode":null,"state":null,"street":"2 Main"}`, map[string]Value{"city": String("Village"), "street": String("2 Main")}},
		{"location", "System.Location", "", `{"latitude":12.5,"longitude":-45.25}`, map[string]Value{"latitude": Decimal(12.5), "longitude": Decimal(-45.25)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := Object(tc.typ)
			value.Runtime = tc.runtime
			value.Fields = tc.fields
			for _, suppress := range []bool{false, true} {
				machine := New(nil)
				got, err := jsonMarshalNoEscape(machine.jsonFromValueForSerialize(value, suppress))
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != tc.want {
					t.Errorf("serialize suppress=%t: native=%s; got=%s", suppress, tc.want, got)
				}
				// The low-level writer also serves JSONGenerator and must retain the same compound representation.
				out, err := jsonMarshalNoEscape(jsonFromValue(value, suppress))
				if err != nil {
					t.Fatal(err)
				}
				if string(out) != tc.want {
					t.Errorf("writer suppress=%t: native=%s; got=%s", suppress, tc.want, out)
				}
				var decoded map[string]any
				if err := json.Unmarshal(got, &decoded); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// Address and Location also name ordinary standard SObjects. Their fields must
// remain record fields; only explicitly tagged System values are compounds.
func TestSObjectJSON2CompoundNamesPreserveRecords(t *testing.T) {
	for _, name := range []string{"Address", "Location"} {
		record := Object(name)
		record.Fields["Name"] = String("Owned record")
		machine := New(nil)
		for _, encoded := range []any{machine.jsonFromValueForSerialize(record, false), jsonFromValue(record, false)} {
			raw, err := jsonMarshalNoEscape(encoded)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			attrs, ok := got["attributes"].(map[string]any)
			if !ok || attrs["type"] != name || got["Name"] != "Owned record" {
				t.Errorf("record fields lost: %s", raw)
			}
		}
	}
}
