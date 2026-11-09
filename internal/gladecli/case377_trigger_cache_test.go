package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

// Native: sema/testdata/case377_trigger_token_recheck/org.tsv, unknown-object rows.
func TestCase377TriggerRejectionSurvivesSemanticDiskCache(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restoreDisk := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	checkSemanticResults.Reset()
	t.Cleanup(checkSemanticResults.Reset)
	for _, version := range []string{"62.0", "67.0", "68.0"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+version+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeTestFile(t, filepath.Join(root, "force-app/main/default/triggers/Case377Probe.trigger"), `trigger Case377Probe on Account (before insert) { Schema.SObjectType token = Case377Missing.SObjectType; }`)
			writeTestFile(t, filepath.Join(root, "force-app/main/default/triggers/Case377Probe.trigger-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+version+`</apiVersion><status>Active</status></ApexTrigger>`)
			run := func(perfPath string) (int, string, string) {
				var stdout, stderr bytes.Buffer
				code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress", "--perf-json", perfPath}, &stdout, &stderr)
				return code, stdout.String(), stderr.String()
			}
			firstPerf := filepath.Join(t.TempDir(), "first.json")
			code, output, errOutput := run(firstPerf)
			if code != 1 || !strings.Contains(output, "Variable does not exist: Case377Missing") {
				t.Fatalf("cold check code=%d stdout=%q stderr=%q", code, output, errOutput)
			}
			if cache := requireJSONObject(t, readPerfJSONObject(t, firstPerf), "semanticCache"); cache["source"] != "build" {
				t.Fatalf("cold cache = %#v", cache)
			}
			checkSemanticResults.Reset()
			secondPerf := filepath.Join(t.TempDir(), "second.json")
			cachedCode, cachedOutput, cachedErr := run(secondPerf)
			if cachedCode != code || cachedOutput != output {
				t.Fatalf("warm result changed: code=%d stdout=%q stderr=%q", cachedCode, cachedOutput, cachedErr)
			}
			if cache := requireJSONObject(t, readPerfJSONObject(t, secondPerf), "semanticCache"); cache["source"] != "disk" {
				t.Fatalf("warm cache = %#v", cache)
			}
		})
	}
}
