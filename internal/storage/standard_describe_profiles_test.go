package storage

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The runtime decoder must retain every captured profile and API, including
// edition removals and the Commerce profile's Communities baseline.
func TestStandardDescribeProfileCatalogNativeDeltas(t *testing.T) {
	stream, err := gzip.NewReader(bytes.NewReader(standardFieldPolicyData))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var bundle standardFieldPolicyBundle
	if err := json.NewDecoder(stream).Decode(&bundle); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	var retained struct {
		Evidence struct {
			Profiles map[string]map[string]struct {
				Baseline string `json:"baseline_profile"`
				Fields   struct{ Added, Removed [][2]string }
				Objects  struct{ Added, Removed []string }
			}
		}
	}
	if err := json.Unmarshal(encoded, &retained); err != nil {
		t.Fatal(err)
	}
	if len(retained.Evidence.Profiles) != 13 {
		t.Fatalf("storage retained %d native profiles, want 13", len(retained.Evidence.Profiles))
	}
	const root = "../apextest/testdata/conformance/standard_describe_profiles"
	fieldRows := readDescribeProfileDeltaRows(t, filepath.Join(root, "feature-set-differences.tsv"))
	objectRows := readDescribeProfileDeltaRows(t, filepath.Join(root, "global-object-differences.tsv"))
	for profile, apis := range retained.Evidence.Profiles {
		if len(apis) != 2 {
			t.Fatalf("%s retained %d APIs, want 2", profile, len(apis))
		}
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(profile+"/"+api, func(t *testing.T) {
				delta, ok := apis[api]
				if !ok {
					t.Fatal("captured API missing from storage bundle")
				}
				baseline := "P0"
				if profile == "P10" {
					baseline = "P4"
				}
				if delta.Baseline != baseline {
					t.Fatalf("baseline=%s, want %s", delta.Baseline, baseline)
				}
				for _, change := range []string{"added", "removed"} {
					wantFields, wantObjects := map[[2]string]bool{}, map[string]bool{}
					for _, row := range fieldRows {
						if row["profile"] == profile && row["api"] == api && row["route"] == "effective-membership" && row["change"] == change {
							wantFields[[2]string{row["object"], row["field"]}] = true
						}
					}
					for _, row := range objectRows {
						if row["profile"] == profile && row["api"] == api && row["change"] == change {
							wantObjects[row["object"]] = true
						}
					}
					fields, objects := delta.Fields.Added, delta.Objects.Added
					if change == "removed" {
						fields, objects = delta.Fields.Removed, delta.Objects.Removed
					}
					gotFields, gotObjects := map[[2]string]bool{}, map[string]bool{}
					for _, field := range fields {
						gotFields[field] = true
					}
					for _, object := range objects {
						gotObjects[object] = true
					}
					if len(gotFields) != len(fields) || !reflect.DeepEqual(gotFields, wantFields) {
						t.Errorf("%s fields differ from native set differences", change)
					}
					if len(gotObjects) != len(objects) || !reflect.DeepEqual(gotObjects, wantObjects) {
						t.Errorf("%s objects differ from native set differences", change)
					}
				}
			})
		}
	}
}

func readDescribeProfileDeltaRows(t *testing.T, path string) []map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("read native differences %s: %v", path, err)
	}
	result := make([]map[string]string, 0, len(rows)-1)
	for _, values := range rows[1:] {
		row := map[string]string{}
		for i, name := range rows[0] {
			row[name] = values[i]
		}
		result = append(result, row)
	}
	return result
}

func TestStandardDescribeProfileCatalogLookupRoutes(t *testing.T) {
	stream, err := gzip.NewReader(bytes.NewReader(standardFieldPolicyData))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var captured struct {
		Evidence struct {
			Profiles map[string]map[string]StandardDescribeCatalogDelta
		}
	}
	if err := json.NewDecoder(stream).Decode(&captured); err != nil {
		t.Fatal(err)
	}
	for profile, apis := range captured.Evidence.Profiles {
		for api, want := range apis {
			t.Run(profile+"/"+api, func(t *testing.T) {
				got, ok := StandardDescribeCatalogProfileDelta(profile, api)
				if !ok || !reflect.DeepEqual(got, want) {
					t.Fatalf("captured catalog delta missing or changed: found=%t", ok)
				}
			})
		}
	}
	for _, key := range [][2]string{{"", "62.0"}, {"unknown", "67.0"}, {"P0", ""}, {"P0", "62"}, {"P0", "63.0"}, {"P0", "68.0"}} {
		t.Run("uncaptured/"+key[0]+"/"+key[1], func(t *testing.T) {
			got, ok := StandardDescribeCatalogProfileDelta(key[0], key[1])
			if ok || !reflect.DeepEqual(got, StandardDescribeCatalogDelta{}) {
				t.Fatal("uncaptured profile/API received an inferred delta")
			}
		})
	}
}

func TestStandardDescribeProfileCatalogCloneIsolation(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want, ok := StandardDescribeCatalogProfileDelta("P11", api)
			if !ok {
				t.Fatal("captured edition delta missing")
			}
			original, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			changed, _ := StandardDescribeCatalogProfileDelta("P11", api)
			if len(changed.Fields.Added) == 0 || len(changed.Fields.Removed) == 0 || len(changed.Objects.Added) == 0 || len(changed.Objects.Removed) == 0 {
				t.Fatal("edition control must exercise all four delta kinds")
			}
			changed.Fields.Added[0][0] = "Changed"
			changed.Fields.Removed[0][1] = "Changed"
			changed.Objects.Added[0] = "Changed"
			changed.Objects.Removed[0] = "Changed"
			got, _ := StandardDescribeCatalogProfileDelta("P11", api)
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded, original) {
				t.Fatal("caller mutation changed the loaded catalog")
			}
		})
	}
}
