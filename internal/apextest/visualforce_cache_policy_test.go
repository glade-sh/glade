package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestVisualforceDigestCacheRespectsRunPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"no disk cache", Options{NoDiskCache: true, Parallelism: 1}},
		{"parallel bypass", Options{ParallelMethods: true, Parallelism: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
			writeFile(t, filepath.Join(root, "src", "pages", "Home.page"), `<apex:page/>`)
			writeFile(t, filepath.Join(root, "src", "classes", "CachePolicyTest.cls"), `@IsTest private class CachePolicyTest { @IsTest static void passes() { System.assertEquals(2, 1 + 1); } }`)
			restore := EnableDiskCacheForTesting()
			defer restore()
			resetFileDigestTrust()
			defer resetFileDigestTrust()
			run := Run(loadTestIndex(t, root), tc.opts)
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Status != testreport.StatusPass {
				t.Fatalf("test run did not pass: %+v", run.Suites)
			}
			if _, err := os.Stat(visualforceDigestCachePath(root)); !os.IsNotExist(err) {
				t.Fatalf("disabled disk cache created digest table: %v", err)
			}
		})
	}
}
