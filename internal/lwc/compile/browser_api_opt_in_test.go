package compile_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/testutil/browserapi"
)

func TestBrowserAPIDefaultIsSequential(t *testing.T) {
	t.Setenv("GLADE_LWC_BROWSER_API_PARALLEL", "")
	t.Setenv("GLADE_LWC_BROWSER_CONCURRENCY", "")
	for _, passes := range []int{2, 3, 5} {
		if got := browserAPIParallelism(passes); got != 1 {
			t.Errorf("default for %d API passes = %d, want 1", passes, got)
		}
	}
}

// Exercise complete temporary projects, compiler, server, Chromium and native
// tab-removal observations. API reports must retain their original row order.
func TestBrowserAPIDefaultProjectConformance(t *testing.T) {
	t.Setenv("GLADE_LWC_BROWSER_API_PARALLEL", "")
	t.Setenv("GLADE_LWC_BROWSER_CONCURRENCY", "")
	browserAPIProjectConformance(t, true)
}

// Run with GLADE_LWC_BROWSER_API_PARALLEL=1 and =2 to compare lane time
// and aggregate process-tree RSS while asserting the same native row bytes.
func TestBrowserAPIProjectMode(t *testing.T) {
	browserAPIProjectConformance(t, browserAPIParallelism(2) == 1)
}

func TestBrowserAPIPreparedProjectMode(t *testing.T) {
	browserAPIProjectConformance(t, browserAPIParallelism(2) == 1, true)
}

func browserAPIProjectConformance(t *testing.T, sequential bool, prepared ...bool) {
	t.Helper()
	data, err := os.ReadFile("testdata/l21_tab_removal_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var table l21Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	root, err := gladehome.EnsureRoot()
	if err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	started := time.Now()
	var active, peak atomic.Int32
	observe := func(t *testing.T, pass int) l21DOMObservation {
		n := active.Add(1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		defer active.Add(-1)
		return l21ObserveDOM(t, table, versions[pass], root)
	}
	var results []l21DOMObservation
	if len(prepared) > 0 && prepared[0] {
		observeAPI := browserapi.Prepare(t, versions, func(t *testing.T, api string) l21DOMObservation {
			for pass, version := range versions {
				if version == api {
					return observe(t, pass)
				}
			}
			t.Fatalf("unexpected API %s", api)
			return l21DOMObservation{}
		})
		for _, api := range versions {
			results = append(results, observeAPI(api))
		}
	} else {
		results = runBrowserPasses(t, versions, browserAPIParallelism(len(versions)), observe)
	}
	wantPeak := int32(2)
	if sequential {
		wantPeak = 1
	}
	if peak.Load() != wantPeak {
		t.Errorf("ran %d API browsers concurrently, want %d", peak.Load(), wantPeak)
	}
	t.Logf("API passes=%d peak=%d observation time=%s", wantPeak, peak.Load(), time.Since(started))
	var actual, expected strings.Builder
	for pass, api := range versions {
		result := results[pass]
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		if result.CompileErr != nil {
			t.Fatal(result.CompileErr)
		}
		for _, c := range table.Cases {
			got, err := l21BrowserText(result.Values[c.ID])
			if err != nil {
				t.Fatalf("%s API %s: %v; %s", c.ID, api, err, result.Errors[c.ID])
			}
			if table.OriginalDOMSurface {
				got, err = l21OriginalDOMText(got)
				if err != nil {
					t.Fatal(err)
				}
			}
			fmt.Fprintf(&actual, "%s\t%s\t%s\n", api, c.ID, got)
			fmt.Fprintf(&expected, "%s\t%s\t%s\n", api, c.ID, c.Expected[api])
		}
	}
	if path := os.Getenv("GLADE_LWC_BROWSER_API_REPORT"); path != "" {
		if len(prepared) > 0 && prepared[0] {
			path += ".prepared"
		}
		if err := os.WriteFile(path, []byte(actual.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if actual.String() != expected.String() {
		t.Errorf("browser observations differ from native rows:\n%s\nwant:\n%s", actual.String(), expected.String())
	}
}
