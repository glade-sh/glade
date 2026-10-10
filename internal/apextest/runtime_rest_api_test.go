package apextest

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// This is a local runner-context test, not new Salesforce parity evidence.
func TestRuntimeRESTAPIContextAcrossSetupStaticAndCaches(t *testing.T) {
	restore := EnableDiskCacheForTesting()
	defer restore()
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"53.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/RuntimeContext.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/RuntimeContext.cls"), `@IsTest private class RuntimeContext {
 private static String initial = JSON.serialize(new Account(Id='001000000000001'));
 @TestSetup static void setup() { insert new Account(Name='context setup',Description=initial); }
 private static void verify(String version) {
  Account stored = [SELECT Description FROM Account WHERE Name='context setup' LIMIT 1];
  System.assert(stored.Description.contains('/services/data/v'+version+'/sobjects/Account/'),stored.Description);
  System.assert(initial.contains('/services/data/v'+version+'/sobjects/Account/'),initial);
  String current=JSON.serialize(new Account(Id='001000000000001'));
  System.assert(current.contains('/services/data/v'+version+'/sobjects/Account/'),current);
 }
 @IsTest static void defaultContext() { verify('65.0'); }
 @IsTest static void explicitContext() { verify('67.0'); }
}`)
	index := loadTestIndex(t, root)
	if index.Project.SourceAPIVersion != "53.0" {
		t.Fatalf("project API drift: %q", index.Project.SourceAPIVersion)
	}
	for _, typ := range index.Types {
		if typ.EffectiveAPIVersion != "53.0" {
			t.Fatalf("component API drift: %q", typ.EffectiveAPIVersion)
		}
	}
	key := RuntimeContentKey(index, nil)
	for _, step := range []struct {
		version, method string
		clear           bool
	}{
		{"", "defaultContext", false}, {"67.0", "explicitContext", false}, {"", "defaultContext", false},
		{"67.0", "explicitContext", true}, {"", "defaultContext", false},
	} {
		if step.clear {
			InvalidateRuntimeCaches()
			if _, ok := tryLoadDiskRuntime(index); !ok {
				t.Fatal("disk restore prerequisite not established")
			}
		}
		run := Run(index, Options{RuntimeRESTAPIVersion: step.version, SelectedMethod: step.method, Parallelism: 1, PerfCounters: step.clear})
		if step.clear && SnapshotPerfCounters().Phases.DiskCacheHits != 1 {
			t.Fatalf("expected actual disk restore: %#v", SnapshotPerfCounters().Phases)
		}
		expected := step.version
		if expected == "" {
			expected = storage.DefaultRESTAPIVersion
		}
		if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || run.RuntimeRESTAPIVersion != expected {
			body, _ := json.Marshal(run)
			t.Fatalf("version %q method %s: %s", step.version, step.method, body)
		}
		if got := RuntimeContentKey(index, nil); got != key {
			t.Fatal("request context changed source compilation identity")
		}
		_, entry := runtimeFromIndex(index, newSourceCache())
		if got := entry.restored.CloneOrg().APIVersion; got != storage.DefaultRESTAPIVersion {
			t.Fatalf("cached template contaminated: %q", got)
		}
	}
	run := Run(index, Options{RuntimeRESTAPIVersion: "999.0", SelectedMethod: "explicitContext"})
	if summary := run.Summary(); summary.Total != 1 || summary.CompileErrors != 1 || summary.Passed != 0 {
		t.Fatalf("unsupported option ran methods: %#v", summary)
	}
	if !strings.Contains(run.Suites[0].Cases[0].Problem.Message, "unsupported REST API version") {
		t.Fatalf("wrong rejection: %#v", run)
	}
}
