package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Replay the captured legacy-source hash sources at their native class API.
// Each route is backed by its own native observation, including direct/nested source52/61 calls
// and concrete-singleton, primitive and null controls.
func TestSystemHashLegacySourceOrgConformance(t *testing.T) {
	const file = "system_hash_legacy_callers.json"
	var data struct {
		APIVersions            []string            `json:"apiVersions"`
		ShapeCount             int                 `json:"shapeCount"`
		NativeObservationCount int                 `json:"nativeObservationCount"`
		Rows                   []nativeBoundaryRow `json:"rows"`
		Cells                  []struct {
			API              string `json:"api"`
			Route            string `json:"route"`
			ID               string `json:"id"`
			Native           string `json:"native"`
			NativeSHA256     string `json:"nativeSHA256"`
			RawReceiptSHA256 string `json:"rawReceiptSHA256"`
		} `json:"cells"`
		ClassMetadataByAPI  map[string]map[string]string `json:"classMetadataByAPI"`
		ClassMetadataSHA256 map[string]map[string]string `json:"classMetadataSHA256"`
	}
	raw, err := os.ReadFile("testdata/conformance/" + file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.ShapeCount != 1 || data.NativeObservationCount != 6 || len(data.Rows) != 1 || len(data.Cells) != 6 || len(data.ClassMetadataByAPI) != 3 || len(data.ClassMetadataSHA256) != 3 {
		t.Fatal("incomplete legacy-source hash native matrix")
	}
	row := data.Rows[0]
	if row.ID != "LH01" || row.Expected == nil || row.Expected.Outcome != "runtime" || row.Expected.Value == "" || len(row.ExpectedByAPI) != 0 || len(row.Routes) != 2 {
		t.Fatal("invalid captured legacy-source hash row")
	}
	fixed := map[string]string{"LegacyHashKey": "52.0", "LegacyHashCaller": "52.0", "LegacyHashBox": "52.0", "FloorHashCaller": "61.0"}
	for _, route := range row.Routes {
		if len(route.FixedClassAPIVersions) != len(fixed) {
			t.Fatal("missing fixed source API controls")
		}
		for name, api := range fixed {
			if route.FixedClassAPIVersions[name] != api {
				t.Fatalf("incorrect source API: %s", name)
			}
		}
	}
	checkHash := func(label, source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatalf("captured bytes changed: %s", label)
		}
	}
	wantCells := make(map[string]bool, 6)
	for _, api := range data.APIVersions {
		for route, id := range map[string]string{"anonymous": "LH01A", "isTest": "LH01T"} {
			if source, ok := row.Routes[route]; !ok || source.ID != id || source.Stage != "runtime" {
				t.Fatalf("invalid captured route: %s", route)
			}
			wantCells[api+"/"+route+"/"+id] = false
		}
		metadata := data.ClassMetadataByAPI[api]
		hashes := data.ClassMetadataSHA256[api]
		classes := row.Routes["isTest"].Classes
		if len(metadata) != 6 || len(hashes) != 6 || len(classes) != 6 {
			t.Fatalf("incomplete captured class metadata: API%s", api)
		}
		for name := range classes {
			text, ok := metadata[name]
			classAPI := api
			if fixed := row.Routes["isTest"].FixedClassAPIVersions[name]; fixed != "" {
				classAPI = fixed
			}
			if !ok || !strings.Contains(text, "<apiVersion>"+classAPI+"</apiVersion>") {
				t.Fatalf("captured class API mismatch: API%s/%s", api, name)
			}
			checkHash(api+"/"+name+" metadata", text, hashes[name])
		}
	}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Route + "/" + cell.ID
		if seen, ok := wantCells[key]; !ok || seen || cell.Native != row.Expected.Value {
			t.Fatalf("unknown, duplicate or misbound native cell: %s", key)
		}
		checkHash(key, cell.Native, cell.NativeSHA256)
		if digest, err := hex.DecodeString(cell.RawReceiptSHA256); err != nil || len(digest) != sha256.Size {
			t.Fatalf("invalid raw native receipt digest: %s", key)
		}
		wantCells[key] = true
	}
	for key, seen := range wantCells {
		if !seen {
			t.Fatalf("missing native cell: %s", key)
		}
	}
	runNativeBoundaryConformance(t, file, "SystemHashLegacySource", 1)
}
