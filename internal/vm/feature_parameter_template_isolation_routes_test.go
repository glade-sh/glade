package vm

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFeatureParameterLockedSetterAnonymousAndIsTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				org.Metadata.FeatureParameters = []storage.FeatureParameterMetadata{
					{Name: "Locked", Type: "Boolean", DataflowDirection: "LmoToSubscriber", Value: "false"},
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "isTest" {
					machine.EnableTestContext()
				}
				program, err := CompileAnonymousWithOptions(`
System.assertEquals(false, System.FeatureManagement.checkPackageBooleanValue('Locked'));
System.FeatureManagement.setPackageBooleanValue('Locked', true);
System.assertEquals(true, System.FeatureManagement.checkPackageBooleanValue('Locked'));
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(program)
				if route == "isTest" {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					var runtimeErr *RuntimeError
					if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.UnexpectedException" {
						t.Fatalf("anonymous locked setter error=%v, want System.UnexpectedException", err)
					}
					if got := org.Metadata.FeatureParameters[0].Value; got != "false" {
						t.Fatalf("rejected anonymous setter changed metadata: %q", got)
					}
				}
			})
		}
	}
}

func TestFeatureParameterNullSetterAnonymousAndIsTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				machine := New(nil)
				if route == "isTest" {
					machine.EnableTestContext()
				}
				program, err := CompileAnonymousWithOptions(`System.FeatureManagement.setPackageIntegerValue(null, 7);`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(program)
				var runtimeErr *RuntimeError
				if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.UnexpectedException" {
					t.Fatalf("null setter error=%v, want System.UnexpectedException", err)
				}
			})
		}
	}
}

func TestFeatureParameterRuntimeTemplateSetterIsolationRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"SetOrg", "SetRuntimeTemplateOrg"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				org.Metadata.FeatureParameters = []storage.FeatureParameterMetadata{
					{Name: "Enabled", Type: "Boolean", DataflowDirection: "SubscriberToLmo", Value: "true"},
					{Name: "Quota", Type: "Integer", DataflowDirection: "SubscriberToLmo", Value: "11"},
					{Name: "StartDate", Type: "Date", DataflowDirection: "SubscriberToLmo", Value: "2020-02-02"},
				}
				want := append([]storage.FeatureParameterMetadata(nil), org.Metadata.FeatureParameters...)
				template := storage.NewRuntimeTemplate(org)
				PrimeRuntimeTemplateSchema(&template)
				first, sibling := template.CloneRuntimeOrg(), template.CloneRuntimeOrg()
				machine := New(nil)
				if route == "SetOrg" {
					machine.SetOrg(&first)
				} else {
					machine.PrimeMetadataSchema(&template.Org)
					machine.SetRuntimeTemplateOrg(&first)
				}
				program, err := CompileAnonymousWithOptions(`
System.FeatureManagement.setPackageBooleanValue('eNaBlEd', false);
System.FeatureManagement.setPackageIntegerValue('qUoTa', 17);
System.FeatureManagement.setPackageDateValue('sTaRtDaTe', Date.newInstance(2020, 3, 4));
System.assertEquals(false, System.FeatureManagement.checkPackageBooleanValue('ENABLED'));
System.assertEquals(17, System.FeatureManagement.checkPackageIntegerValue('QUOTA'));
System.assertEquals(Date.newInstance(2020, 3, 4), System.FeatureManagement.checkPackageDateValue('STARTDATE'));
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				for name, state := range map[string]storage.OrgState{"source": org, "template": template.Org, "sibling": sibling, "later": template.CloneRuntimeOrg()} {
					if !reflect.DeepEqual(state.Metadata.FeatureParameters, want) {
						t.Errorf("%s feature parameters = %v, want %v", name, state.Metadata.FeatureParameters, want)
					}
				}
			})
		}
	}
}
