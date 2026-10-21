package vm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestDMLAuditClockNativeConformance(t *testing.T) {
	var fixture struct {
		HelperSource, HelperSourceSha256, ReplaySource, ReplaySourceSha256 string
		Rows                                                               []struct {
			Context, SourceAPI, TransportAPI, DeploymentID, DeploymentStatus                  string
			ObserverSource, ObserverSourceSha256, RawResponseSha256, AnswerJSON, AnswerSha256 string
			Expected                                                                          map[string]any
		}
	}
	raw, err := os.ReadFile("testdata/dml_clock_native_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	digest := func(source string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(source))) }
	if digest(fixture.HelperSource) != fixture.HelperSourceSha256 || digest(fixture.ReplaySource) != fixture.ReplaySourceSha256 {
		t.Fatal("native helper or replay source hash changed")
	}
	if len(fixture.Rows) != 2 {
		t.Fatalf("native rows=%d, want2", len(fixture.Rows))
	}
	seen := make(map[string]bool)
	for _, row := range fixture.Rows {
		t.Run(row.Context, func(t *testing.T) {
			if seen[row.Context] || row.SourceAPI != "61.0" || row.DeploymentID == "" || row.DeploymentStatus != "Succeeded" || len(row.RawResponseSha256) != 64 {
				t.Fatal("incomplete native source boundary")
			}
			seen[row.Context] = true
			if digest(row.ObserverSource) != row.ObserverSourceSha256 || digest(row.AnswerJSON) != row.AnswerSha256 {
				t.Fatal("native observer or answer hash changed")
			}
			expectedTransport := "67.0"
			if row.Context == "anonymous" {
				expectedTransport = "61.0"
			} else if row.Context != "istest" {
				t.Fatal("unobserved context")
			}
			if row.TransportAPI != expectedTransport || len(row.Expected) != 5 {
				t.Fatal("incomplete expected native cells")
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			program, err := CompileAnonymousWithOptions(fixture.ReplaySource, CompileOptions{APIVersion: row.SourceAPI})
			if err != nil {
				t.Fatal(err)
			}
			if row.Context == "istest" {
				if _, err := machine.ExecuteTestInClass(program, "AuditClockObservation"); err != nil {
					t.Fatal(err)
				}
			} else if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			observed := machine.Globals["result"]
			if observed.Kind != ValueMap {
				t.Fatalf("missing observed map: %#v", observed)
			}
			for key, want := range row.Expected {
				actual, present := observed.Map[mapKey(String(key))]
				if !present {
					t.Fatalf("missing observed cell %s", key)
				}
				switch want := want.(type) {
				case bool:
					if actual.Kind != ValueBool || actual.Bool != want {
						t.Errorf("%s=%v; native=%v", key, actual, want)
					}
				case float64:
					if actual.Kind != ValueInt || actual.Int != int64(want) {
						t.Errorf("%s=%v; native=%v", key, actual, want)
					}
				default:
					t.Fatalf("unhandled native cell %s", key)
				}
			}
		})
	}
}
