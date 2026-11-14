package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

// A test runtime populated before DAP must retain the baseline cache behavior
// for an Apex class whose metadata sidecar is absent.
func TestDAPDiskCacheWithoutClassSidecar(t *testing.T) {
	testDAPDiskCacheClassVersion(t, "")
}

func TestDAPDiskCacheWithUnversionedSidecar(t *testing.T) {
	testDAPDiskCacheClassVersion(t, `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><status>Active</status></ApexClass>`)
}

func testDAPDiskCacheClassVersion(t *testing.T, sidecar string) {
	t.Helper()
	restore := apextest.EnableDiskCacheForTesting()
	defer restore()
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
	write("src/classes/NeighborTest.cls", `@IsTest private class NeighborTest { @IsTest static void passes() { System.assertEquals(2,1+1); } }`)
	if sidecar != "" {
		write("src/classes/NeighborTest.cls-meta.xml", sidecar)
	}
	write("src/pages/Home.page", `<apex:page>Home</apex:page>`)
	write("src/pages/Home.page-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>Home</label></ApexPage>`)
	var out, errout bytes.Buffer
	if rc := Run(context.Background(), []string{"test", "--project", root, "--filter", "NeighborTest", "--parallelism", "1", "--json"}, &out, &errout); rc != 0 {
		t.Fatalf("test rc=%d: %s %s", rc, out.String(), errout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "test", "startup.meta.json")); err != nil {
		t.Fatal(err)
	}
	apextest.InvalidateRuntimeCaches()
	_, runtime, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, class := range runtime.Classes {
		if class.Name == "NeighborTest" {
			if class.APIVersion != "" {
				t.Fatalf("cached class API version = %q, want baseline empty version", class.APIVersion)
			}
			return
		}
	}
	t.Fatal("NeighborTest class missing")
}
