package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/testutil/enumvaluefixture"
	"testing"
)

func TestEnumValueMethodsNativeCheckNamedIsTestRoutes(t *testing.T) {
	fixture, rows := enumvaluefixture.Load(t)
	for _, row := range rows {
		if row.Case.Route == "anonymous" {
			continue
		}
		t.Run(row.API+"/"+row.Case.ID, func(t *testing.T) {
			root := t.TempDir()
			enumvaluefixture.WriteProject(t, root, fixture, row)
			assertEnumValueCheck(t, row, root, "--no-cache")
		})
	}
}

// The cached twin runs check cold, then warm from the semantic caches, and
// requires the same native diagnostic both times.
func TestEnumValueMethodsNativeCheckCachedTwins(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restoreDisk := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	checkSemanticResults.Reset()
	t.Cleanup(checkSemanticResults.Reset)
	fixture, rows := enumvaluefixture.Load(t)
	for _, row := range rows {
		if row.Case.Route == "anonymous" {
			continue
		}
		t.Run(row.API+"/"+row.Case.ID, func(t *testing.T) {
			root := t.TempDir()
			enumvaluefixture.WriteProject(t, root, fixture, row)
			assertEnumValueCheck(t, row, root, "")
			assertEnumValueCheck(t, row, root, "")
		})
	}
}

func assertEnumValueCheck(t *testing.T, row enumvaluefixture.Row, root, extra string) {
	t.Helper()
	args := []string{"check", "--project", root, "--json"}
	if extra != "" {
		args = append(args, extra)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, &stdout, &stderr)
	var report struct {
		Diagnostics []diagnostic.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("check exit=%d: %v stdout=%s stderr=%s", code, err, &stdout, &stderr)
	}
	// The CLI JSON envelope exposes Message, while sema and LSP retain NativeMessage.
	for i := range report.Diagnostics {
		report.Diagnostics[i].NativeMessage = report.Diagnostics[i].Message
	}
	enumvaluefixture.AssertDiagnostics(t, row, report.Diagnostics)
	if (code == 0) != row.Compiled {
		t.Fatalf("org.tsv:%d check exit=%d stderr=%s", row.Line, code, &stderr)
	}
}
