package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFeatureParameterMetadataLazyConstructorRoutes(t *testing.T) {
	template := New(nil)
	frozen := New(nil)
	frozen.FreezeClassLookup()
	for route, create := range map[string]func() *VM{
		"New":                              func() *VM { return New(nil) },
		"CloneRuntime":                     func() *VM { return template.CloneRuntime(nil) },
		"CloneRuntimeFrozenShared":         func() *VM { return frozen.CloneRuntimeFrozenShared(nil) },
		"CloneRuntimeFrozenSharedFallback": func() *VM { return template.CloneRuntimeFrozenShared(nil) },
	} {
		t.Run(route, func(t *testing.T) {
			machine := create()
			if machine.managedFeatureParameters != nil {
				t.Fatal("constructor eagerly allocated feature parameter metadata without an org")
			}
			for _, api := range []string{"62.0", "67.0"} {
				for _, isTest := range []bool{false, true} {
					machine = create()
					if isTest {
						machine.EnableTestContext()
					}
					program, err := CompileAnonymousWithOptions(`
try {
    System.FeatureManagement.checkPackageIntegerValue('Missing');
    System.assert(false, 'expected missing feature parameter');
} catch (System.NoDataFoundException e) {
    System.assertEquals('Unable to retrieve feature parameter Missing. No results found.', e.getMessage());
}
`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatalf("API %s isTest=%t: %v", api, isTest, err)
					}
				}
			}
			if machine.managedFeatureParameters != nil {
				t.Fatal("missing parameter lookup allocated feature parameter metadata")
			}
			machine.SetOrg(&storage.OrgState{Metadata: storage.MetadataRegistry{
				FeatureParameters: []storage.FeatureParameterMetadata{{Name: "Enabled", Type: "Boolean", Value: "true"}},
			}})
			value, err := machine.managedFeatureValue("Boolean", []Value{String("Enabled")})
			if err != nil || value.Kind != ValueBool || !value.Bool {
				t.Fatalf("org installation after lazy construction: value=%v error=%v", value, err)
			}
		})
	}
}
