package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type interfaceCrossCell struct {
	ID, API, Route, Context, Actor, Status string
	SourceJSON, SourceSHA256               string
	ObservationJSON, ObservationSHA256     string
}

type interfaceCrossSource struct {
	Classes, Objects    map[string]string
	ProviderClasses     map[string]string `json:"provider_classes"`
	ProviderObjects     map[string]string `json:"provider_objects"`
	ProviderNamespace   string            `json:"provider_namespace"`
	SubscriberNamespace string            `json:"subscriber_namespace"`
	OriginalApex        string            `json:"original_apex"`
	OriginalApexSHA256  string            `json:"original_apex_sha256"`
}

// CD136 is a partial capture: 27 actual answers, one fixture error. Source and
// observation JSON strings retain the exact TSV field bytes and their hashes.
// The consumer's OPEN cleanup state remains outside this credential-free replay.
func interfaceDispatchNamespaceCross62(t *testing.T, seen map[string]bool) {
	t.Helper()
	var data struct {
		Capture, API, Route, Context, ConsumerCleanup string
		ObservedCells, AccountedCells                 int
		NativeComplete                                bool
		Inputs                                        map[string]string
		Runs                                          map[string]struct {
			Namespace             string
			Complete, CleanupOpen bool
		}
		Cells, FixtureErrors []interfaceCrossCell
	}
	raw, err := os.ReadFile("testdata/conformance/interface_dispatch_namespace_cross62.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Capture != "CD-136" || data.API != "62.0" || data.Route != "istest" || data.Context != "cross" || data.NativeComplete || data.ObservedCells != 27 || data.AccountedCells != 28 || len(data.Cells) != 27 || len(data.FixtureErrors) != 1 || !strings.HasPrefix(data.ConsumerCleanup, "OPEN;") {
		t.Fatal("cross-62 partial capture boundary changed")
	}
	if len(data.Inputs) != 4 || len(data.Runs) != 2 || data.Runs["A"].Namespace != "gladens002" || data.Runs["B"].Namespace != "gladens004" || data.Runs["A"].Complete || data.Runs["B"].Complete || data.Runs["A"].CleanupOpen || !data.Runs["B"].CleanupOpen {
		t.Fatal("cross-62 source/run provenance missing")
	}
	keys := map[string]bool{}
	for _, cell := range data.Cells {
		interfaceCrossValidate(t, cell)
		key := cell.Actor + "/" + cell.ID
		if keys[key] || cell.Status == "FIXTURE_ERROR" || cell.ID == "V19" || (cell.Actor == "A" && !strings.HasPrefix(cell.ID, "CTRL_")) {
			t.Fatalf("invalid observed cross cell: %s", key)
		}
		keys[key] = true
		if strings.HasPrefix(cell.ID, "V") {
			seen[cell.ID+"/62.0/istest/cross"] = true
		}
		t.Run("CD136/"+key+"/62.0/istest/cross", func(t *testing.T) {
			var source interfaceCrossSource
			if err := json.Unmarshal([]byte(cell.SourceJSON), &source); err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256([]byte(source.OriginalApex))) != source.OriginalApexSHA256 {
				t.Fatal("original Apex hash changed")
			}
			namespace := data.Runs[cell.Actor].Namespace
			replay := interfaceCrossReplay(t, cell, source, &namespace)
			interfaceDispatchNamespaceCell(t, namespace, cell.API, replay)
		})
	}
	for _, actor := range []string{"A", "B"} {
		for _, id := range []string{"CTRL_POS", "CTRL_CAUGHT", "CTRL_UNCAUGHT", "CTRL_COMPILE"} {
			if !keys[actor+"/"+id] {
				t.Fatal("cross transport control omitted")
			}
		}
	}
	for i := 1; i <= 20; i++ {
		if i != 19 && !keys[fmt.Sprintf("B/V%02d", i)] {
			t.Fatal("observed cross subject omitted")
		}
	}
	fixture := data.FixtureErrors[0]
	interfaceCrossValidate(t, fixture)
	var native struct{ Outcome, Signature string }
	if err := json.Unmarshal([]byte(fixture.ObservationJSON), &native); err != nil {
		t.Fatal(err)
	}
	if fixture.ID != "V19" || fixture.Actor != "B" || fixture.Status != "FIXTURE_ERROR" || native.Outcome != "FIXTURE_ERROR" || native.Signature != "producer-dependency-unavailable" {
		t.Fatal("V19 must remain fixture error, never a semantic answer")
	}
	t.Log("CD136: 27 observed/28 accounted; V19 fixture error, other APIs/routes and consumer cleanup remain carried")
}

func interfaceCrossValidate(t *testing.T, cell interfaceCrossCell) {
	t.Helper()
	var native struct{ API, Route, Context, Actor, Outcome string }
	if err := json.Unmarshal([]byte(cell.ObservationJSON), &native); err != nil {
		t.Fatal(err)
	}
	if native.API != cell.API || native.Route != cell.Route || native.Context != cell.Context || native.Actor != cell.Actor || native.Outcome != cell.Status {
		t.Fatal("native observation axes/outcome differ from captured cell")
	}
	if cell.API != "62.0" || cell.Route != "istest" || cell.Context != "cross" || (cell.Actor != "A" && cell.Actor != "B") || fmt.Sprintf("%x", sha256.Sum256([]byte(cell.SourceJSON))) != cell.SourceSHA256 || fmt.Sprintf("%x", sha256.Sum256([]byte(cell.ObservationJSON))) != cell.ObservationSHA256 {
		t.Fatal("captured axes or exact source/answer bytes changed")
	}
}

func interfaceCrossReplay(t *testing.T, cell interfaceCrossCell, source interfaceCrossSource, namespace *string) interfaceDispatchCell {
	t.Helper()
	replay := interfaceDispatchCell{ID: cell.ID, Classes: source.Classes, Objects: source.Objects, ProviderNamespace: source.ProviderNamespace, ProviderClasses: source.ProviderClasses, ProviderObjects: source.ProviderObjects}
	if err := json.Unmarshal([]byte(cell.ObservationJSON), &replay.Native); err != nil {
		t.Fatal(err)
	}
	var observation struct {
		Stage       string
		Diagnostics []struct{ Name, Problem string }
		Compile     []struct{ FullName, Problem string }
	}
	if err := json.Unmarshal([]byte(cell.ObservationJSON), &observation); err != nil {
		t.Fatal(err)
	}
	if observation.Stage == "A-provider-declaration" {
		// V15/V16 rejected the provider declaration; no subscriber invocation
		// was observed. Replay the producer alone, not the hypothetical caller.
		*namespace = source.ProviderNamespace
		replay.Classes, replay.Objects = source.ProviderClasses, source.ProviderObjects
		replay.ProviderNamespace = ""
		for _, diagnostic := range observation.Compile {
			if strings.HasSuffix(diagnostic.FullName, "ToneChannel") {
				replay.PrimaryClass, replay.PrimaryProblem = diagnostic.FullName, diagnostic.Problem
			}
			replay.Native.Diagnostics = append(replay.Native.Diagnostics, nativeBoundaryDiagnostic{Problem: diagnostic.Problem})
		}
		replay.TestClass = replay.PrimaryClass
		return replay
	}
	for name, code := range replay.Classes {
		if len(replay.Classes) == 1 || strings.HasSuffix(name, "Proof") {
			replay.TestClass = name
			if strings.Contains(code, "static void testPart00(){") {
				parts := strings.Split(code, "@IsTest static void testPart")[1:]
				body := ""
				for _, part := range parts {
					_, tail, found := strings.Cut(part, "(){")
					prefix, _, framed := strings.Cut(tail, "String captureJson=")
					if !found || !framed || (body != "" && prefix != body) {
						t.Fatal("transport parts do not share the same captured logical body")
					}
					body = prefix
				}
				header, _, _ := strings.Cut(code, "@IsTest static void testPart")
				_, suffix, closed := strings.Cut(parts[len(parts)-1], "System.assert(false,envelope);}")
				if !closed {
					t.Fatal("captured final transport method is not closed")
				}
				// Collapse only the repeated chunk-envelope transport. Every
				// statement that produces the native payload remains unchanged.
				replay.Classes[name] = header + "@IsTest static void observe(){" + body + "System.assert(false,'C126_RESULT|" + cell.ID + "|'+JSON.serialize(payload));}" + suffix
			}
		}
	}
	if replay.Native.Outcome == "uncaught_test_failure" {
		replay.Native.TestFailures = []nativeObservationFailure{{ExceptionType: replay.Native.ExceptionType, Message: replay.Native.ExceptionMessage}}
	}
	if replay.Native.Outcome == "compile-rejected" {
		for _, diagnostic := range observation.Diagnostics {
			if replay.PrimaryClass == "" || (cell.ID == "V14" && strings.HasSuffix(diagnostic.Name, "SubscriberTone")) {
				replay.PrimaryClass, replay.PrimaryProblem = diagnostic.Name, diagnostic.Problem
			}
		}
		if cell.ID == "CTRL_COMPILE" {
			replay.Comparison = "rejectionOnly"
			replay.CarryReason = "Native missing-type rejection is retained; editor diagnostic wording remains carried as in the existing control."
		}
	}
	return replay
}
