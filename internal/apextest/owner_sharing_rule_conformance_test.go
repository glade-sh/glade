package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The captured boundary is share-row materialization, not recipient visibility.
// Owner baselines have both routes. Owner transitions are named-test-only because
// their isolated setup users require runAs; no anonymous transition is inferred.
func TestOwnerSharingRuleOrgConformance(t *testing.T) {
	const file = "owner_sharing_rule.json"
	var data struct {
		APIVersions            []string            `json:"apiVersions"`
		ShapeCount             int                 `json:"shapeCount"`
		NativeObservationCount int                 `json:"nativeObservationCount"`
		Rows                   []nativeBoundaryRow `json:"rows"`
		Cells                  []struct {
			API, Row, Route, ID, Native string
			NativeSHA256                string `json:"nativeSHA256"`
			RawReceiptSHA256            string `json:"rawReceiptSHA256"`
		} `json:"cells"`
		ClassMetadataByAPI  map[string]map[string]string `json:"classMetadataByAPI"`
		ClassMetadataSHA256 map[string]map[string]string `json:"classMetadataSHA256"`
	}
	raw, err := os.ReadFile("testdata/conformance/" + file)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ShapeCount != 4 || data.NativeObservationCount != 18 || len(data.Rows) != 4 || len(data.Cells) != 18 || len(data.APIVersions) != 3 {
		t.Fatal("incomplete captured owner-sharing matrix")
	}
	hash := func(label, source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatalf("captured bytes changed: %s", label)
		}
	}
	cells := map[string]bool{}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, row := range data.Rows {
			if row.Expected == nil || row.Expected.Value == "" {
				t.Fatal("missing observed value")
			}
			for route, source := range row.Routes {
				cells[api+"/"+row.ID+"/"+route] = false
				classes := source.Classes
				if route == "anonymous" {
					classes = source.SetupClasses
				}
				for name := range classes {
					meta := data.ClassMetadataByAPI[api][name]
					if !strings.Contains(meta, "<apiVersion>"+api+"</apiVersion>") {
						t.Fatalf("captured class API mismatch %s/%s", api, name)
					}
					hash(api+name, meta, data.ClassMetadataSHA256[api][name])
				}
			}
		}
	}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Row + "/" + cell.Route
		if seen, ok := cells[key]; !ok || seen {
			t.Fatalf("unexpected/repeated native cell %s", key)
		}
		for _, row := range data.Rows {
			if row.ID == cell.Row && (row.Expected.Value != cell.Native || row.Routes[cell.Route].ID != cell.ID) {
				t.Fatalf("misbound native cell %s", key)
			}
		}
		hash(key, cell.Native, cell.NativeSHA256)
		if digest, err := hex.DecodeString(cell.RawReceiptSHA256); err != nil || len(digest) != sha256.Size {
			t.Fatalf("invalid raw native receipt digest %s", key)
		}
		cells[key] = true
	}
	for key, seen := range cells {
		if !seen {
			t.Fatalf("uncaptured cell %s", key)
		}
	}
	runNativeBoundaryConformance(t, file, "OwnerSharingRule", 4)
}
