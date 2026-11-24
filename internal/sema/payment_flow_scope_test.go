package sema

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPaymentFlowScopedMixedVersionsAndRoutes(t *testing.T) {
	root := t.TempDir()
	body := `commercepayments.SaleRequest req = new commercepayments.SaleRequest(7.25);
 commercepayments.PaymentGatewayContext ctx = new commercepayments.PaymentGatewayContext(req, commercepayments.RequestType.Sale);
 commercepayments.SaleResponse res = new commercepayments.SaleResponse();
 commercepayments.GatewayResponse gateway = res;`
	cases := []struct {
		name, api             string
		testClass, wantErrors bool
	}{
		{"Captured62", "62.0", true, false}, {"Captured67", "67.0", true, false},
		{"Other63", "63.0", true, true}, {"Named62", "62.0", false, true},
	}
	paths := make([]string, 0, len(cases))
	for _, tc := range cases {
		path := filepath.Join(root, tc.name+".cls")
		annotation, method := "public", "public"
		if tc.testClass {
			annotation, method = "@IsTest private", "@IsTest"
		}
		writeSemaFile(t, path, fmt.Sprintf("%s class %s { %s static void run() { %s } }", annotation, tc.name, method, body))
		writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+tc.api+"</apiVersion></ApexClass>")
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "62.0", ApexFiles: paths}, schema.Schema{})
	result := Analyze(index)
	for _, tc := range cases {
		var messages []string
		for _, d := range result.Diagnostics {
			if strings.HasSuffix(d.File, tc.name+".cls") {
				messages = append(messages, d.Message)
			}
		}
		if (len(messages) != 0) != tc.wantErrors {
			t.Errorf("%s API %s diagnostics = %v, want errors %v", tc.name, tc.api, messages, tc.wantErrors)
		}
	}
}

func TestPaymentFlowUncapturedUseShapes(t *testing.T) {
	for _, api := range []string{"62.0", "63.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			source := `@IsTest private class Probe {
    static void take(commercepayments.GatewayResponse response) {}
    @IsTest static void run() {
      commercepayments.SaleResponse response = new commercepayments.SaleResponse();
      take(response);
    }
   }`
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, api)
			if !result.HasErrors() {
				t.Fatalf("uncaptured method argument unexpectedly accepted: %+v", result.Diagnostics)
			}
		})
	}
}

func TestPaymentFlowUncapturedInitializerAndContextShapes(t *testing.T) {
	cases := map[string]string{
		"direct response": `commercepayments.GatewayResponse response = new commercepayments.SaleResponse();`,
		"wrong enum":      `commercepayments.SaleRequest req = new commercepayments.SaleRequest(1.0); commercepayments.PaymentGatewayContext ctx = new commercepayments.PaymentGatewayContext(req, commercepayments.RequestType.AuthorizationReversal);`,
		"nested context":  `commercepayments.SaleRequest req = new commercepayments.SaleRequest(1.0); Object ctx = new commercepayments.PaymentGatewayContext(req, commercepayments.RequestType.Sale);`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			src := `@IsTest private class Probe { @IsTest static void run() { ` + body + ` } }`
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": src}, "62.0")
			if !result.HasErrors() {
				t.Fatalf("uncaptured shape accepted: %+v", result.Diagnostics)
			}
		})
	}
}
