package vm

import "testing"

// Case-insensitive System.Address recognition was supported by the legacy
// address writer. Preserve it while sharing the compound serialization path.
func TestJSONAddressCaseInsensitiveWriter(t *testing.T) {
	value := Object("system.address")
	value.Fields["city"] = String("Town")
	want := `{"city":"Town","country":null,"geocodeAccuracy":null,"latitude":null,"longitude":null,"postalCode":null,"state":null,"street":null}`
	for _, suppress := range []bool{false, true} {
		for _, got := range []any{jsonFromValue(value, suppress), New(nil).jsonFromValueForSerialize(value, suppress)} {
			raw, err := jsonMarshalNoEscape(got)
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != want {
				t.Errorf("suppress=%t: got %s, want %s", suppress, raw, want)
			}
		}
	}
}
