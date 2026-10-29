package storage

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestFeatureProfileSerializationPresence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		features   []string
		jsonMember string
	}{
		{"legacy", nil, ""},
		{"empty", []string{}, `"features":[]`},
		{"populated", []string{"Chatter"}, `"features":["Chatter"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			org.Features = tc.features
			data, err := json.Marshal(org)
			if err != nil {
				t.Fatal(err)
			}
			if tc.jsonMember == "" {
				if bytes.Contains(data, []byte(`"features"`)) {
					t.Fatalf("legacy JSON: %s", data)
				}
			} else if !bytes.Contains(data, []byte(tc.jsonMember)) {
				t.Fatalf("org JSON: %s", data)
			}
			var decoded OrgState
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if (decoded.Features == nil) != (tc.features == nil) || len(decoded.Features) != len(tc.features) {
				t.Fatalf("org profile: %#v", decoded.Features)
			}
			fixture := FixtureFromOrg(org)
			if (fixture.Org.Features == nil) != (tc.features == nil) {
				t.Fatalf("fixture profile: %#v", fixture.Org.Features)
			}
			var buf bytes.Buffer
			if err := WriteFixture(&buf, fixture); err != nil {
				t.Fatal(err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
				t.Fatal(err)
			}
			var rawOrg map[string]json.RawMessage
			if err := json.Unmarshal(raw["org"], &rawOrg); err != nil {
				t.Fatal(err)
			}
			member, present := rawOrg["features"]
			if tc.features == nil {
				if present {
					t.Fatalf("legacy fixture: %s", buf.Bytes())
				}
			} else {
				var values []string
				if !present || json.Unmarshal(member, &values) != nil || values == nil || len(values) != len(tc.features) {
					t.Fatalf("fixture JSON: %s", buf.Bytes())
				}
			}
			decodedFixture, err := ReadFixture(&buf)
			if err != nil {
				t.Fatal(err)
			}
			if (decodedFixture.Org.Features == nil) != (tc.features == nil) || len(decodedFixture.Org.Features) != len(tc.features) {
				t.Fatalf("decoded fixture profile: %#v", decodedFixture.Org.Features)
			}
		})
	}
}

func TestFixtureEmptyFeatureProfileDoesNotRestoreLegacyOverlay(t *testing.T) {
	for _, tc := range []struct {
		name        string
		features    []string
		wantChatter bool
	}{
		{"missing", nil, true}, {"explicit empty", []string{}, false}, {"populated", []string{"Chatter"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := NewOrgState()
			source.Features = tc.features
			var encoded bytes.Buffer
			if err := WriteFixture(&encoded, FixtureFromOrg(source)); err != nil {
				t.Fatal(err)
			}
			fixture, err := ReadFixture(&encoded)
			if err != nil {
				t.Fatal(err)
			}
			target := NewOrgState()
			target.Features = []string{}
			target.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", Metadata: map[string]string{standardFieldsOverlayMarker: "Chatter"}}, Records: map[ID]Record{}}
			if err := ApplyFixture(&target, fixture); err != nil {
				t.Fatal(err)
			}
			if got := target.HasFeature("Chatter"); got != tc.wantChatter {
				t.Fatalf("Chatter = %v; want %v (profile %#v)", got, tc.wantChatter, target.Features)
			}
		})
	}
}
