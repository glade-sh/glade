package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/vm"
)

func TestRegisterProjectRuntimeCompilesUnicodeStaticMapFieldInitializer(t *testing.T) {
	root := t.TempDir()
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	if err := os.MkdirAll(classes, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `public class Countries {
    public static String lookupCountry(String country) {
        return country == null ? null : CountryMap.get(country.toUpperCase());
    }

    private static final Map<String, String> CountryMap = new Map<String, String> {
        'ÅLAND'=>'AX',
        'L\'ANDORRE'=>'AD',
        'UNITED STATES'=>'US',
        'US'=>'US'
    };
}`
	if err := os.WriteFile(filepath.Join(classes, "Countries.cls"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	proj, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	loadedSchema, err := schema.LoadProject(proj)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(proj, loadedSchema)
	if index.HasErrors() {
		t.Fatalf("index diagnostics: %#v", index.Diagnostics)
	}
	machine := vm.New(nil)
	if err := RegisterProjectRuntimeForRequest(machine, index); err != nil {
		t.Fatal(err)
	}
	value, err := machine.CallStatic("Countries.lookupCountry", []vm.Value{vm.String("US")})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != vm.ValueString || value.Text != "US" {
		t.Fatalf("lookupCountry = %#v, want US", value)
	}
}
