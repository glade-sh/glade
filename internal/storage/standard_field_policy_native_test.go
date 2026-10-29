package storage

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Native named Apex membership takes precedence over an FLS-filtered REST
// describe. Exercise both cold imports and already marked persisted overlays.
func TestStandardFieldPolicyNativePersistedRefreshRoutes(t *testing.T) {
	const dir = "../apextest/testdata/conformance/standard_describe_profiles"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Samples               map[string][]string
		HistoricalExtraFields []struct{ Object, Field string } `json:"historical_extra_fields"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	wanted := map[string]map[string]bool{}
	for _, row := range plan.HistoricalExtraFields {
		if wanted[row.Object] == nil {
			wanted[row.Object] = map[string]bool{}
		}
		wanted[row.Object][row.Field] = true
	}
	for object, fields := range plan.Samples {
		if wanted[object] == nil {
			wanted[object] = map[string]bool{}
		}
		for _, field := range fields {
			if field != "*" {
				wanted[object][field] = true
			}
		}
	}
	for _, api := range []string{"62", "67"} {
		file, err := os.Open(filepath.Join(dir, "P0", "api"+api, "granted/describes.json.gz"))
		if err != nil {
			t.Fatal(err)
		}
		stream, err := gzip.NewReader(file)
		if err != nil {
			t.Fatal(err)
		}
		var bundle struct {
			Describes map[string]struct{ Fields []struct{ Name string } }
		}
		err = json.NewDecoder(stream).Decode(&bundle)
		stream.Close()
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		present := map[string]map[string]bool{}
		for object, describe := range bundle.Describes {
			present[object] = map[string]bool{}
			for _, field := range describe.Fields {
				present[object][field.Name] = true
			}
		}
		file, err = os.Open(filepath.Join(dir, "P0", "api"+api, "default/named-fields.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(file)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows[1:] {
			if row[2] == "true" && row[1] != "" && present[row[0]] != nil {
				present[row[0]][row[1]] = true
			}
		}
		for object, fields := range wanted {
			native, observed := present[object]
			if !observed {
				continue
			}
			for field := range fields {
				t.Run(api+"/"+object+"."+field, func(t *testing.T) {
					for _, route := range []string{"cold-import", "current-marker-refresh"} {
						t.Run(route, func(t *testing.T) {
							definition := ObjectDefinition{APIName: object, Fields: map[string]Field{"Boundary__c": {APIName: "Boundary__c", Type: FieldString}}}
							if route == "current-marker-refresh" {
								EnsureStandardObjectFieldsForFeatures(&definition, nil)
							}
							if native[field] {
								delete(definition.Fields, field)
							} else if route == "current-marker-refresh" {
								definition.Fields[field] = Field{APIName: field, Type: FieldString}
							}
							if route == "current-marker-refresh" {
								RefreshStandardObjectFieldOverlay(&definition)
							} else {
								EnsureStandardObjectFieldsForFeatures(&definition, nil)
							}
							_, exists := ResolveFieldName(definition, "", field)
							if exists != native[field] {
								t.Fatalf("native membership=%t; %s membership=%t", native[field], route, exists)
							}
							if _, ok := definition.Fields["Boundary__c"]; !ok {
								t.Fatal("project field lost during refresh")
							}
						})
					}
				})
			}
		}
	}
}

// Bind the shipped policy metadata to the native input, independently of the
// runtime merge. Legacy gates preserve existing behavior and add no new facets.
func TestStandardFieldPolicyNativeSidecarProvenance(t *testing.T) {
	const dir = "../apextest/testdata/conformance/standard_describe_profiles"
	stream, err := gzip.NewReader(bytes.NewReader(standardFieldPolicyData))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var policy standardFieldPolicyBundle
	if err := json.NewDecoder(stream).Decode(&policy); err != nil {
		t.Fatal(err)
	}
	if policy.Version != 1 || len(policy.Sources) != 118 || len(policy.Gates) != 108 {
		t.Fatal("incomplete native field policy")
	}
	for _, name := range []string{"definition.json", "org.tsv", "observations.json"} {
		path := "../standard_case_source_evidence/" + name
		if _, exists := policy.Sources[path]; !exists {
			t.Fatalf("missing Case source native policy input: %s", path)
		}
	}
	// standard_constructor_complete/org.tsv:2-55 and ORG-RUN.txt qualify the
	// added API62/67 constructor rejection, boundary and dynamic-put routes.
	for _, name := range []string{"cases.json", "observations.json", "org.tsv"} {
		path := "../standard_constructor_complete/" + name
		if _, exists := policy.Sources[path]; !exists {
			t.Fatalf("missing qualified constructor policy input: %s", path)
		}
	}
	// Every captured profile and API must remain an input, even where its
	// broader field deltas are reserved for the edition/feature sidecar.
	for _, profile := range []string{"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10", "P11", "PU"} {
		for _, api := range []string{"62", "67"} {
			path := profile + "/api" + api + "/default/named-fields.tsv"
			if _, exists := policy.Sources[path]; !exists {
				t.Fatalf("missing native policy input: %s", path)
			}
		}
	}
	for path, want := range policy.Sources {
		raw, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		got := sha256.Sum256(raw)
		if hex.EncodeToString(got[:]) != want {
			t.Fatalf("policy source changed: %s", path)
		}
	}
	read := func(profile, api string) map[string]map[string]standardDescribeField {
		t.Helper()
		file, err := os.Open(filepath.Join(dir, profile, "api"+api, "granted/describes.json.gz"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		gz, err := gzip.NewReader(file)
		if err != nil {
			t.Fatal(err)
		}
		defer gz.Close()
		var bundle standardDescribeCatalogBundle
		if err := json.NewDecoder(gz).Decode(&bundle); err != nil {
			t.Fatal(err)
		}
		fields := map[string]map[string]standardDescribeField{}
		for object, describe := range bundle.Describes {
			fields[object] = map[string]standardDescribeField{}
			for _, field := range describe.Fields {
				fields[object][field.Name] = field
			}
		}
		return fields
	}
	base := read("P0", "62")
	constructorFields := map[string]bool{}
	for name, field := range base["Account"] {
		constructorFields[name] = true
		if field.RelationshipName != "" {
			constructorFields[field.RelationshipName] = true
		}
	}
	namedFile, err := os.Open(filepath.Join(dir, "P0/api62/default/named-fields.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	namedReader := csv.NewReader(namedFile)
	namedReader.Comma = '\t'
	namedRows, err := namedReader.ReadAll()
	namedFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range namedRows[1:] {
		if row[0] == "Account" && row[2] == "true" && row[1] != "" {
			constructorFields[row[1]] = true
		}
	}
	shippedConstructorFields := map[string]bool{}
	for _, name := range policy.Objects["Account"].ConstructorFields {
		shippedConstructorFields[name] = true
	}
	if !reflect.DeepEqual(shippedConstructorFields, constructorFields) {
		t.Fatal("constructor surface differs from native named and reference fields")
	}
	for object, entry := range policy.Objects {
		for _, field := range entry.Fields {
			if object == "WebStore" && field.Name == "OptionsAutoFacetingEnabled" {
				// P10 supplies the rich facets; P8 named Apex also exposes the field.
				if !reflect.DeepEqual(field, read("P10", "62")[object][field.Name]) {
					t.Fatal("WebStore metadata differs from the enabled Commerce capture")
				}
				continue
			}
			if !reflect.DeepEqual(field, base[object][field.Name]) {
				t.Fatalf("baseline metadata differs: %s.%s", object, field.Name)
			}
		}
	}
	profiles := map[string]string{"MultiCurrency": "P1", "PersonAccounts": "P2", "StateAndCountryPicklist": "P3"}
	legacy := 0
	for _, api := range []string{"62", "67"} {
		captured := map[string]map[string]map[string]standardDescribeField{}
		for feature, profile := range profiles {
			captured[feature] = read(profile, api)
		}
		captured["union"] = read("PU", api)
		captured["case-source"] = read("P11", api)
		for _, gate := range policy.Gates {
			if gate.Legacy {
				if api == "62" {
					legacy++
				}
				if !((gate.Object == "Account" && gate.Field.Name == "IsActive") || (gate.Object == "Opportunity" && gate.Field.Name == "SyncedQuoteId")) {
					t.Fatal("unexpected legacy gate")
				}
				continue
			}
			key := gate.Features[0]
			if gate.Object == "Case" && gate.Field.Name == "SourceId" {
				if !reflect.DeepEqual(gate.Features, []string{"LiveAgent", "LiveAgentEnabled"}) {
					t.Fatal("Case source availability differs from the isolated native profile")
				}
				requireCaseSourceNativeMembership(t, api)
				key = "case-source"
			} else if len(gate.Features) > 1 {
				key = "union"
			}
			field, ok := captured[key][gate.Object][gate.Field.Name]
			if !ok {
				t.Fatalf("gate lacks native positive row: %s/%s.%s", api, gate.Object, gate.Field.Name)
			}
			if (api == "62" || key == "case-source") && !reflect.DeepEqual(gate.Field, field) {
				t.Fatalf("gate metadata differs: %s.%s", gate.Object, gate.Field.Name)
			}
		}
	}
	if legacy != 2 {
		t.Fatal("legacy policy changed")
	}
}

func TestStandardFieldPolicyNativeConstructorAvailabilityRoutes(t *testing.T) {
	const dir = "../apextest/testdata/conformance/standard_describe_profiles"
	for _, api := range []string{"62", "67"} {
		for _, profile := range []struct {
			path     string
			features []string
		}{
			{"P0", nil},
			{"P10/license-only", []string{"B2BCommerce", "Communities"}},
			{"P10", []string{"B2BCommerce", "Communities", "CommerceEnabled"}},
		} {
			t.Run(api+"/"+profile.path+"/constructor-availability", func(t *testing.T) {
				file, err := os.Open(filepath.Join(dir, profile.path, "api"+api, "default/named-fields.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				reader := csv.NewReader(file)
				reader.Comma = '\t'
				rows, err := reader.ReadAll()
				if err != nil {
					t.Fatal(err)
				}
				observed, present := false, false
				for _, row := range rows[1:] {
					if row[0] == "WebStore" {
						observed = true
						present = row[2] == "true"
						break
					}
				}
				if !observed {
					t.Fatal("missing native WebStore object observation")
				}
				if got := StandardObjectConstructorAvailableForFeatures("WebStore", profile.features); got != present {
					t.Fatalf("native present=%t; constructor availability=%t", present, got)
				}
			})
		}
	}
}
