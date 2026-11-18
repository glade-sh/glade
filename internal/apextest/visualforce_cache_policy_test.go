package apextest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestVisualforceDigestCacheDisabledRunDoesNotReadExistingTable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
	page := filepath.Join(root, "src", "pages", "Home.page")
	writeFile(t, page, `<apex:page/>`)
	writeFile(t, filepath.Join(root, "src", "classes", "CachePolicyTest.cls"), `@IsTest private class CachePolicyTest { @IsTest static void passes() { System.assertEquals(2, 1 + 1); } }`)
	restore := EnableDiskCacheForTesting()
	defer restore()
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = oldNow, oldHook; resetFileDigestTrust() })
	resetFileDigestTrust()
	index := loadTestIndex(t, root)
	first := Run(index, Options{Parallelism: 1})
	if len(first.Suites) != 1 || len(first.Suites[0].Cases) != 1 || first.Suites[0].Cases[0].Status != testreport.StatusPass {
		t.Fatal("enabled test run did not pass")
	}
	before, err := os.ReadFile(visualforceDigestCachePath(root))
	if err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	reads := 0
	fileDigestReadHook = func(path string) {
		if path == page {
			reads++
		}
	}
	second := Run(index, Options{NoDiskCache: true, Parallelism: 1})
	if len(second.Suites) != 1 || len(second.Suites[0].Cases) != 1 || second.Suites[0].Cases[0].Status != testreport.StatusPass {
		t.Fatal("disabled-cache test run did not pass")
	}
	if reads == 0 {
		t.Fatal("disabled-cache run reused the persisted page digest")
	}
	after, err := os.ReadFile(visualforceDigestCachePath(root))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("disabled-cache run changed the digest table: %v", err)
	}
}
