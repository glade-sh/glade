package testdaemon

import (
	"strings"
	"testing"
)

// A daemon started in one sibling-scan mode must not serve a run requested in
// the other, and must serve a run in the same mode.
func TestServerRefusesRunInOtherSiblingScanMode(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	for _, daemonOn := range []bool{false, true} {
		daemon, err := New(root)
		if err != nil {
			t.Fatal(err)
		}
		ready := make(chan struct{})
		close(ready)
		server := &Server{daemon: daemon, ready: true, warmDone: ready, siblingScan: daemonOn}

		mismatch := exchangeProtocolWiringV1(t, server, RequestV1{
			Version: ProtocolVersionV1,
			Op:      OpRun,
			ID:      "other-mode",
			Run:     &RunRequestV1{Parallelism: 1, SiblingDependencyScan: !daemonOn},
		})
		if mismatch.OK || mismatch.Op != OpError || !strings.Contains(mismatch.Error, "sibling dependency scan") {
			t.Fatalf("daemonOn=%v: other-mode run = %#v, want refusal", daemonOn, mismatch)
		}

		same := exchangeProtocolWiringV1(t, server, RequestV1{
			Version: ProtocolVersionV1,
			Op:      OpRun,
			ID:      "same-mode",
			Run: &RunRequestV1{
				SelectedClasses:       []string{"WarmTwoTest"},
				Parallelism:           1,
				TimeoutMS:             300_000,
				SiblingDependencyScan: daemonOn,
			},
		})
		if !same.OK || same.Op != OpRunResult {
			t.Fatalf("daemonOn=%v: same-mode run = %#v, want result", daemonOn, same)
		}
	}
}

func TestRunV1SendsProcessSiblingScanMode(t *testing.T) {
	t.Setenv("GLADE_SIBLING_DEPENDENCIES", "")
	if processSiblingDependencyScan() {
		t.Fatal("mode on with the environment unset")
	}
	t.Setenv("GLADE_SIBLING_DEPENDENCIES", "1")
	if !processSiblingDependencyScan() {
		t.Fatal("mode off with GLADE_SIBLING_DEPENDENCIES=1")
	}
}
