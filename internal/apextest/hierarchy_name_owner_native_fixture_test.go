package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This validates native evidence, not local runtime conformance. In particular,
// the omitted-owner Name observation must not be replaced by the explicit-owner
// control, nor can a metadata rejection supply an Apex runtime answer.
func TestHierarchyNameOwnerNativeFixture(t *testing.T) {
	var data struct {
		SourceAPIs []string
		Fixtures   map[string]struct {
			Sources, SHA256 map[string]string
		}
		Cells []struct {
			ID, API, TransportAPI, Route, Stage, Fixture string
			DeploymentID, Status, RuntimeCarry, Reason   string
			RawAnswerSHA256, RawPayloadSHA256            string
			OrgRunSHA256, OrgTSVSHA256                   string
			ProjectedNativeSHA256                        string
			CheckOnly                                    bool
			ChunkCount                                   int
			ExpectedNative                               map[string]any
		}
	}
	raw, err := os.ReadFile(filepath.Join(hierarchyNameOwnerFixtureDir, "native-core.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.SourceAPIs, ",") != "62.0,67.0,68.0" || len(data.Fixtures) != 9 || len(data.Cells) != 18 {
		t.Fatal("incomplete native core evidence")
	}
	checkHash := func(t *testing.T, value, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(value))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("native fixture bytes changed")
		}
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.ID + "/" + cell.API + "/" + cell.Route
		if seen[key] {
			t.Fatal("duplicate native cell", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			if cell.Route == "anonymous" {
				if cell.Stage != "pending" || cell.ExpectedNative != nil || cell.Reason == "" || cell.DeploymentID != "" {
					t.Fatal("uncaptured route acquired an inferred answer")
				}
				return
			}
			if cell.Route != "istest" || cell.TransportAPI != cell.API || !cell.CheckOnly || cell.DeploymentID == "" || cell.Status != "Failed" {
				t.Fatal("missing check-only route provenance")
			}
			for _, hash := range []string{cell.RawAnswerSHA256, cell.OrgRunSHA256, cell.OrgTSVSHA256} {
				if decoded, err := hex.DecodeString(hash); err != nil || len(decoded) != sha256.Size {
					t.Fatal("missing raw native artifact digest")
				}
			}
			fixture, ok := data.Fixtures[cell.Fixture]
			if !ok || len(fixture.Sources) != 6 || len(fixture.SHA256) != 6 {
				t.Fatal("missing exact native fixture")
			}
			var object string
			for name, source := range fixture.Sources {
				checkHash(t, source, fixture.SHA256[name])
				if strings.HasSuffix(name, ".object-meta.xml") {
					object = source
				}
			}
			encoded, err := json.Marshal(cell.ExpectedNative)
			if err != nil {
				t.Fatal(err)
			}
			checkHash(t, string(encoded), cell.ProjectedNativeSHA256)
			if cell.ID == "HNO04" {
				if cell.Stage != "metadata-rejection" || strings.Contains(object, "<label>") || cell.ExpectedNative["problem"] != "Must specify a non-empty label for the CustomObject" || cell.ExpectedNative["runtimeObserved"] != false || cell.ChunkCount != 0 {
					t.Fatal("label-absent setup rejection was converted to runtime evidence")
				}
				return
			}
			if cell.Stage != "runtime" || cell.ChunkCount != 32 || len(cell.RawPayloadSHA256) != 64 || !strings.Contains(object, "<label>Owner name preference</label>") {
				t.Fatal("unbound runtime observation")
			}
			native := cell.ExpectedNative
			if native["stage"] != "complete" || native["nameWasSupplied"] != false {
				t.Fatal("changed runtime shape")
			}
			dml, ok := native["dml"].(map[string]any)
			if !ok || dml["success"] != true || len(dml["errors"].([]any)) != 0 {
				t.Fatal("native DML failed")
			}
			wantName := "Owner name preference (Organization)"
			var callerOwner any = "[ORG_ID]"
			if cell.ID == "HNO01" {
				wantName, callerOwner = "[RECORD_ID15]", nil
				if native["insertedId"] != "[RECORD_ID18]" || native["ownerWasSupplied"] != false || cell.RuntimeCarry == "" {
					t.Fatal("omitted-owner discriminator or local carry missing")
				}
			} else if cell.ID != "HNO02" || native["ownerWasSupplied"] != true {
				t.Fatal("unexpected owner control")
			}
			for view, owner := range map[string]any{
				"input": callerOwner, "callerAfterDml": callerOwner,
				"soql": "[ORG_ID]", "orgDefaults": "[ORG_ID]", "instanceOrganization": "[ORG_ID]",
				"instance": "[USER_ID]", "instanceUser": "[USER_ID]", "instanceProfile": "[PROFILE_ID]",
			} {
				observation, ok := native[view].(map[string]any)
				if !ok || observation["present"] != true || observation["SetupOwnerId"] != owner {
					t.Fatal("missing independent owner observation", view)
				}
				var name any = wantName
				if view == "input" || view == "callerAfterDml" {
					name = nil
				}
				if observation["Name"] != name {
					t.Fatal("native Name distinction changed", view)
				}
			}
		})
	}
	for _, row := range []string{"HNO01", "HNO02", "HNO04"} {
		for _, api := range data.SourceAPIs {
			for _, route := range []string{"anonymous", "istest"} {
				if !seen[row+"/"+api+"/"+route] {
					t.Fatal("missing core cell", row, api, route)
				}
			}
		}
	}
}
