package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestLoadedControllerPropertyGetterSetterCapabilities(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "PropertyController.cls")
	writeFile(t, path, `public class PropertyController {
  public Integer setterCalls = 0;
  private String backing;
  public String caption { get; set { setterCalls++; caption = value; } }
  public String writeOnly { set { backing = value; } }
  public Integer getCaption() { return 99; }
  public void setCaption(Integer ignored) { setterCalls += 100; }
}`)
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{path}}, schema.Schema{})
	machine := vm.New(nil)
	if err := RegisterProjectRuntime(machine, index); err != nil {
		t.Fatal(err)
	}
	controller, err := machine.ConstructController("PropertyController")
	if err != nil {
		t.Fatal(err)
	}
	if typ, ok, err := machine.InstancePropertyType(controller, "caption"); err != nil || !ok || typ != "String" {
		t.Fatalf("loaded automatic getter/custom setter capability: type=%q found=%v err=%v", typ, ok, err)
	}
	controller, err = machine.AssignInstanceProperty(controller, "caption", vm.String("loaded-value"))
	if err != nil {
		t.Fatal(err)
	}
	if value, ok, err := machine.ReadInstanceProperty(controller, "caption"); err != nil || !ok || value.Text != "loaded-value" {
		t.Fatalf("automatic getter lost setter result: value=%#v found=%v err=%v", value, ok, err)
	}
	if calls, ok, err := machine.ReadInstanceProperty(controller, "setterCalls"); err != nil || !ok || calls.Kind != vm.ValueInt || calls.Int != 1 {
		t.Fatalf("custom setter did not execute once: value=%#v found=%v err=%v", calls, ok, err)
	}
	if _, ok, err := machine.InstancePropertyType(controller, "writeOnly"); err != nil || ok {
		t.Fatalf("set-only property was admitted as get/set: found=%v err=%v", ok, err)
	}
	if _, err := machine.AssignInstanceProperty(controller, "writeOnly", vm.String("rejected")); err == nil {
		t.Fatal("set-only assignment succeeded")
	}
}

func TestControllerPropertyGetterCapabilityCacheRoundTrip(t *testing.T) {
	restore := EnableDiskCacheForTesting()
	defer restore()
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/CachedPropertyController.cls"), `public class CachedPropertyController {
  private String backing;
  public String caption { get; set { caption = value; } }
  public String writeOnly { set { backing = value; } }
}`)
	index := loadTestIndex(t, root)
	key, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), true)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRuntimeCaches()
	restored, ok := tryLoadDiskRuntimeWithSourceDigests(index, nil, key)
	if !ok {
		t.Fatal("current getter-capability cache did not reload")
	}
	machine := restored.restored.CloneMachine(nil)
	controller, err := machine.ConstructController("CachedPropertyController")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := machine.InstancePropertyType(controller, "caption"); err != nil || !ok {
		t.Fatalf("cache lost automatic getter capability: found=%v err=%v", ok, err)
	}
	if _, ok, err := machine.InstancePropertyType(controller, "writeOnly"); err != nil || ok {
		t.Fatalf("cache invented getter capability: found=%v err=%v", ok, err)
	}
	entry, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil || entry == nil {
		t.Fatalf("read cache: entry=%v err=%v", entry, err)
	}
	if entry.RuntimeABI != "apextest-runtime-v7" {
		t.Fatalf("cache ABI = %q, want getter-capability v7", entry.RuntimeABI)
	}
	entry.RuntimeABI = "apextest-runtime-v6"
	if err := startupcache.Write(entry, startupcache.SubdirTest); err != nil {
		t.Fatal(err)
	}
	InvalidateRuntimeCaches()
	if _, ok := tryLoadDiskRuntimeWithSourceDigests(index, nil, key); ok {
		t.Fatal("pre-getter-capability v6 cache was accepted")
	}
}
