package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestInvocableResultOutputNativeConformance(t *testing.T) {
	var data struct {
		Captures []struct {
			SourceAPI, TransportAPI, Route string
			DeployID                       string
			CheckOnly, RollbackOnError     bool
			Classes, SourceSHA256          map[string]string
			Metadata                       map[string]string
			MetadataSHA256                 string
			Observations                   map[string]map[string]any
		}
		Uncaptured []struct{ Scope, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/invocable_result_output_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Captures) != 2 || len(data.Uncaptured) != 2 {
		t.Fatal("incomplete native Result output evidence")
	}
	for _, carry := range data.Uncaptured {
		if carry.Scope == "" || carry.Reason == "" {
			t.Fatal("uncaptured scope requires a reason")
		}
		t.Logf("UNCAPTURED %s: %s", carry.Scope, carry.Reason)
	}
	for i, capture := range data.Captures {
		t.Run(capture.SourceAPI, func(t *testing.T) {
			if capture.SourceAPI != []string{"62.0", "67.0"}[i] || capture.TransportAPI != capture.SourceAPI || capture.Route != "istest" ||
				capture.DeployID == "" || !capture.CheckOnly || !capture.RollbackOnError || len(capture.Classes) != 1 || len(capture.SourceSHA256) != 1 || len(capture.Metadata) != 1 {
				t.Fatal("unexpected native capture scope")
			}
			for name, source := range capture.Classes {
				digest := sha256.Sum256([]byte(source))
				if hex.EncodeToString(digest[:]) != capture.SourceSHA256[name] {
					t.Fatal("native source hash mismatch", name)
				}
				metadata := capture.Metadata[name+".cls-meta.xml"]
				digest = sha256.Sum256([]byte(metadata))
				if hex.EncodeToString(digest[:]) != capture.MetadataSHA256 || !strings.Contains(metadata, "<apiVersion>"+capture.SourceAPI+"</apiVersion>") {
					t.Fatal("native metadata pin mismatch", name)
				}
			}
			index := nativeBoundaryIndex(t, capture.SourceAPI, capture.Classes, nil)
			run := Run(index, Options{SelectedClasses: []string{"ResultOutputObservation"}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 3 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 3 || len(capture.Observations) != 3 {
				t.Fatalf("observation selection: %#v", run)
			}
			seen := map[string]bool{}
			for _, result := range run.Suites[0].Cases {
				if result.Problem == nil || result.Problem.Type != "System.AssertException" {
					t.Fatalf("missing native observation: %#v", result)
				}
				_, packet, ok := strings.Cut(result.Problem.Message, "RESULT_DTO_RESULT|")
				id, payload, framed := strings.Cut(packet, "|")
				want, observed := capture.Observations[id]
				var actual map[string]any
				if !ok || !framed || !observed || seen[id] || json.Unmarshal([]byte(payload), &actual) != nil {
					t.Fatalf("invalid observation frame: %s", result.Problem.Message)
				}
				seen[id] = true
				if !reflect.DeepEqual(want, actual) {
					t.Errorf("%s native=%v; local=%v", id, want, actual)
				}
			}
		})
	}
}
