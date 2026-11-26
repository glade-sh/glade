package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPaymentFlowCapturedLocalAssignmentScope(t *testing.T) {
	source := `commercepayments.SaleResponse response = new commercepayments.SaleResponse();
 commercepayments.GatewayResponse gateway = response;`
	for _, tc := range []struct {
		api     string
		wantErr bool
	}{{"62.0", false}, {"63.0", true}, {"67.0", false}} {
		t.Run(tc.api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: tc.api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = New(nil).Execute(program)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantErr && !strings.Contains(strings.ToLower(err.Error()), "cannot assign commercepayments.saleresponse to commercepayments.gatewayresponse") {
				t.Fatalf("expected response assignment mismatch, got %v", err)
			}
		})
	}
}

func TestPaymentFlowUncapturedTypeQueriesKeepBase(t *testing.T) {
	for _, api := range []string{"62.0", "63.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			machine.currentMethod.APIVersion = api
			for _, pair := range [][2]string{
				{"commercepayments.SaleResponse", "commercepayments.GatewayResponse"},
				{"commercepayments.AuthorizationReversalResponse", "commercepayments.GatewayResponse"},
				{"commercepayments.SaleRequest", "commercepayments.PaymentGatewayRequest"},
				{"commercepayments.AuthorizationReversalRequest", "commercepayments.PaymentGatewayRequest"},
			} {
				registerPaymentFlowPlatformClass(t, machine, pair[0])
				assignable := machine.typeAssignableTo(pair[0], pair[1])
				matches := machine.typeMatches(pair[0], pair[1], map[string]bool{})
				if assignable || matches {
					t.Errorf("%s -> %s: assignable = %t, matches = %t, want both false", pair[0], pair[1], assignable, matches)
				}
			}
		})
	}
}

// Register the actual standard symbol's hierarchy so this also exercises the
// linked-platform route and fails against the former global overlays.
func registerPaymentFlowPlatformClass(t *testing.T, machine *VM, name string) {
	t.Helper()
	generated, ok := lookupGeneratedPlatformType(strings.ToLower(name))
	if !ok {
		t.Fatalf("missing generated platform type %s", name)
	}
	for _, symbol := range typesys.StandardPlatformSymbolView() {
		full := symbol.Namespace + "." + symbol.Name
		if strings.EqualFold(full, name) {
			constructors := append([]Method(nil), generated.Constructors...)
			for i := range constructors {
				constructors[i].Program = ir.Program{}
				constructors[i].Modifiers = nil
			}
			err := machine.RegisterClass(Class{Name: name, PlatformType: true, Interfaces: append([]string(nil), symbol.Interfaces...), Fields: generated.Fields, Constructors: constructors})
			if err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("missing platform symbol %s", name)
}

func runPaymentFlowNamed(t *testing.T, api string, testClass bool, source string) error {
	t.Helper()
	machine := New(nil)
	registerPaymentFlowPlatformClass(t, machine, "commercepayments.SaleResponse")
	registerPaymentFlowPlatformClass(t, machine, "commercepayments.SaleRequest")
	registerPaymentFlowPlatformClass(t, machine, "commercepayments.AuthorizationReversalRequest")
	if err := machine.RegisterClass(Class{Name: "FlowProbe", APIVersion: api, IsTest: testClass}); err != nil {
		t.Fatal(err)
	}
	method, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	modifiers := []string{"public"}
	if testClass {
		machine.EnableTestContext()
		modifiers = append(modifiers, "isTest")
	}
	if err = machine.RegisterMethod(Method{Name: "FlowProbe.run", ClassName: "FlowProbe", APIVersion: api, IsStatic: true, Access: "public", Modifiers: modifiers, Program: method}); err != nil {
		t.Fatal(err)
	}
	entry, err := CompileAnonymousWithOptions("FlowProbe.run();", CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(entry)
	return err
}

func TestPaymentFlowUncapturedNamedRouteKeepsBase(t *testing.T) {
	err := runPaymentFlowNamed(t, "62.0", false, `commercepayments.SaleResponse response = new commercepayments.SaleResponse(); commercepayments.GatewayResponse gateway = response;`)
	if err == nil {
		t.Fatal("uncaptured named class assignment unexpectedly succeeded")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cannot assign commercepayments.saleresponse to commercepayments.gatewayresponse") {
		t.Fatalf("expected response assignment mismatch, got %v", err)
	}
}

func TestPaymentFlowContextDeclarationShapes(t *testing.T) {
	cases := []struct {
		name, source string
		wantErr      bool
	}{
		{"captured", `commercepayments.PaymentGatewayContext ctx = new commercepayments.PaymentGatewayContext(req, commercepayments.RequestType.Sale);`, false},
		{"object declaration", `Object ctx = new commercepayments.PaymentGatewayContext(req, commercepayments.RequestType.Sale);`, true},
		{"variable enum", `commercepayments.RequestType kind = commercepayments.RequestType.Sale; commercepayments.PaymentGatewayContext ctx = new commercepayments.PaymentGatewayContext(req, kind);`, true},
		{"wrong enum", `commercepayments.PaymentGatewayContext ctx = new commercepayments.PaymentGatewayContext(req, commercepayments.RequestType.AuthorizationReversal);`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runPaymentFlowNamed(t, "62.0", true, `commercepayments.SaleRequest req = new commercepayments.SaleRequest(7.25); `+tc.source)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantErr && !strings.Contains(strings.ToLower(err.Error()), "commercepayments.paymentgatewaycontext constructor") {
				t.Fatalf("expected context constructor mismatch, got %v", err)
			}
		})
	}
}

func TestPaymentFlowContextTypedRequestGetter(t *testing.T) {
	cases := []struct {
		request, args, kind string
	}{
		{"SaleRequest", "amount", "Sale"},
		{"AuthorizationReversalRequest", "amount, ref", "AuthorizationReversal"},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.kind, func(t *testing.T) {
				source := fmt.Sprintf(`
Double amount = 7.25;
String ref = 'synthetic-ref';
commercepayments.%[1]s req = new commercepayments.%[1]s(%[2]s);
commercepayments.PaymentGatewayContext ctx = new commercepayments.PaymentGatewayContext(req, commercepayments.RequestType.%[3]s);
commercepayments.PaymentGatewayRequest returned = ctx.getPaymentRequest();
System.assert(returned == req);
`, tc.request, tc.args, tc.kind)
				if err := runPaymentFlowNamed(t, api, true, source); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
