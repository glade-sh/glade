package vm_test

import (
	"encoding/json"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"testing"
)

// Local namespace composition regression. It does not claim managed-org parity.
func TestNamespaceStaticAccessorRetainsDeclaringOwner(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, pkg := range []string{"consumer", "dependency"} {
		ns := "localns"
		marker := "local"
		if pkg == "dependency" {
			ns = "dep"
			marker = "dependency"
		}
		write(pkg+"/sfdx-project.json", `{"namespace":"`+ns+`","sourceApiVersion":"64.0","packageDirectories":[{"path":"src","default":true}]}`)
		write(pkg+"/src/Registry.cls", `public class Registry {
    private static Map<String,Registry> cache { get { if(cache==null) cache=new Map<String,Registry>(); return cache; } }
    public static Registry getInstance(){Registry value=cache.get('key');if(value==null){value=new Registry();cache.put('key',value);}return value;}
    public String marker(){return '`+marker+`';}
  }`)
		write(pkg+"/src/Registry.cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>`)
	}
	write("consumer/glade.yml", "project:\n  managedPackageDependencies: [\"dep:../dependency\"]\n")
	write("dependency/src/Gateway.cls", `global class Gateway {global static String read(){return Registry.getInstance().marker();}}`)
	write("dependency/src/Gateway.cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>`)
	write("consumer/src/RegistryTest.cls", `@IsTest private class RegistryTest {@IsTest static void keepsOwners(){System.assertEquals('local',Registry.getInstance().marker());System.assertEquals('dependency',dep.Gateway.read());System.assertEquals('local',Registry.getInstance().marker());System.assertEquals('dependency',dep.Gateway.read());}}`)
	write("consumer/src/RegistryTest.cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>`)
	p, err := project.Load(filepath.Join(root, "consumer"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	run := apextest.Run(typesys.Build(p, s), apextest.Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("namespace cache owner: %s", b)
	}
}
