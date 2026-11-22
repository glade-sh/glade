package vm

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestGlobalDescribeMatrixCloneMatchesFreshNeighbor(t *testing.T) {
	features := []string{"Sites", "Quotes"}
	caller := namesOnlyCaller{"owner", "nsx", "nsx"}
	source := namesOnlyProbeSource([]string{"Account", "Widget__c", "NoSuchObject"})
	for _, route := range []string{"source", "legacy"} {
		for _, eager := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/eager=%t", route, eager), func(t *testing.T) {
				base := namesOnlyDescribeOrg(route, features, caller.namespace)
				cloned := base.Clone()
				fresh := namesOnlyDescribeOrg(route, features, caller.namespace)
				want := runNamesOnlyProbe(t, &fresh, eager, "67.0", caller, false, false, source)
				got := runNamesOnlyProbe(t, &cloned, eager, "67.0", caller, false, false, source)
				if strings.HasPrefix(want, "error: ") {
					t.Fatalf("fresh probe failed: %s", want)
				}
				if diff := probeDifference(want, got); diff != "" {
					t.Fatal(diff)
				}
				if fresh.LazyStandardSpreadEnabled() != cloned.LazyStandardSpreadEnabled() {
					t.Fatal("fresh and cloned orgs differ in spread state")
				}
				again := base.Clone()
				if got := runNamesOnlyProbe(t, &again, eager, "67.0", caller, false, false, source); got != want {
					t.Fatalf("later clone differs from fresh: %s", probeDifference(want, got))
				}
			})
		}
	}
}

func BenchmarkGlobalDescribeParityMatrix(b *testing.B) {
	features := namesOnlyDescribeShapes["communities"]
	source := namesOnlyProbeSource(namesOnlyProbeNames(features))
	caller := namesOnlyCallers[0]
	for _, clone := range []bool{false, true} {
		name := "fresh"
		if clone {
			name = "clone"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				hash := sha256.New()
				var base storage.OrgState
				if clone {
					base = namesOnlyDescribeOrg("source", features, caller.namespace)
				}
				for _, api := range namesOnlyDescribeAPIs {
					for _, named := range []bool{false, true} {
						for _, testContext := range []bool{false, true} {
							if testContext && !named {
								continue
							}
							var eagerOrg, lazyOrg storage.OrgState
							if clone {
								eagerOrg, lazyOrg = base.Clone(), base.Clone()
							} else {
								eagerOrg = namesOnlyDescribeOrg("source", features, caller.namespace)
								lazyOrg = namesOnlyDescribeOrg("source", features, caller.namespace)
							}
							want := runNamesOnlyProbe(b, &eagerOrg, true, api, caller, named, testContext, source)
							got := runNamesOnlyProbe(b, &lazyOrg, false, api, caller, named, testContext, source)
							if strings.HasPrefix(want, "error: ") {
								b.Fatalf("eager probe failed: %s", want)
							}
							if want != got {
								b.Fatal("parity mismatch")
							}
							fmt.Fprintf(hash, "%d:%s", len(got), got)
						}
					}
				}
				b.Logf("output sha256 %x", hash.Sum(nil))
			}
		})
	}
}
