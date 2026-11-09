package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native observations: testdata/conformance/commerce_enabled_surface/org.tsv,
// commerce-enabled and contract-pricing-enabled at 67.0 and 68.0.
func TestCommerceContractPricingAPI68NativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, enabled := range []bool{false, true} {
			for _, name := range []string{"ContractResolutionRequest", "ContractResolutionResponse"} {
				t.Run(api+"/"+name+map[bool]string{false: "/disabled", true: "/enabled"}[enabled], func(t *testing.T) {
					root := t.TempDir()
					if err := os.MkdirAll(filepath.Join(root, "config"), 0755); err != nil {
						t.Fatal(err)
					}
					setting := "false"
					if enabled {
						setting = "true"
					}
					writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["B2BCommerce","Communities"],"settings":{"commerceSettings":{"commerceEnabled":true,"commerceContractPricing":`+setting+`}}}`)
					source := "commercestorepricing." + name + " value;"
					path := filepath.Join(root, "Probe.cls")
					writeSemaFile(t, path, "public class Probe { "+source+" }")
					idx := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
					wantErrors := api != "68.0" || !enabled
					if got := Analyze(idx).HasErrors(); got != wantErrors {
						t.Errorf("named errors=%t want=%t", got, wantErrors)
					}
					if got := AnalyzeAnonymous(idx, source, api).HasErrors(); got != wantErrors {
						t.Errorf("anonymous errors=%t want=%t", got, wantErrors)
					}
					testPath := filepath.Join(root, "TestProbe.cls")
					writeSemaFile(t, testPath, "@IsTest private class TestProbe { @IsTest static void sample() { "+source+" System.assertEquals(null,value); } }")
					testIndex := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{testPath}}, schema.Schema{})
					if got := Analyze(testIndex).HasErrors(); got != wantErrors {
						t.Errorf("istest errors=%t want=%t", got, wantErrors)
					}
				})
			}
		}
	}
}

func TestCommerceContractPricingWarmIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config/project-scratch-def.json")
	setPricing := func(value string) {
		writeSemaFile(t, config, `{"edition":"Developer","features":["B2BCommerce"],"settings":{"commerceSettings":{"commerceEnabled":true,"commerceContractPricing":`+value+`}}}`)
	}
	path := filepath.Join(root, "Probe.cls")
	writeSemaFile(t, path, `public class Probe { commercestorepricing.ContractResolutionRequest value; }`)
	p := project.Project{Root: root, SourceAPIVersion: "68.0", ApexFiles: []string{path}}
	setPricing("false")
	old := typesys.Build(p, schema.Schema{})
	if !Analyze(old).HasErrors() {
		t.Fatal("disabled setting must reject")
	}
	setPricing("true")
	if !Analyze(old).HasErrors() {
		t.Fatal("captured index changed after config edit")
	}
	if typesys.MatchesProjectIdentity(old, p) {
		t.Fatal("setting change reused index identity")
	}
	fresh := typesys.Build(p, schema.Schema{})
	if typesys.SameProjectIdentity(old, fresh) {
		t.Fatal("different settings share index identity")
	}
	if result := Analyze(fresh); result.HasErrors() {
		t.Fatalf("enabled setting must accept: %+v", result.Diagnostics)
	}
}
