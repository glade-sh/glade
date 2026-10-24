package vm

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCalloutUncommittedHTTPAnonymousAndIsTestRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/callout_uncommitted_work/native"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				raw, err := os.ReadFile(filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				want := ""
				for _, line := range strings.Split(string(raw), "\n") {
					if strings.HasPrefix(line, "UW01\t") {
						want = strings.TrimPrefix(line, "UW01\t")
					}
				}
				if want == "" {
					t.Fatal("missing native insert/callout row")
				}
				source := `insert new Account(Name='Pending work');HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');String observed='ok';try{new Http().send(request);}catch(Exception failure){observed=failure.getTypeName()+'|'+failure.getMessage();}System.debug(observed);`
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := testDataOrg()
				org.APIVersion = api
				machine.SetOrg(&org)
				if route == "istest" {
					machine.EnableTestContext()
					machine.testContext.HTTPMock = Object("SyntheticResponse")
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if result.Limits.Callouts != 0 {
					t.Errorf("native UW14 rejects before callout accounting; Glade callouts=%d", result.Limits.Callouts)
				}
				if len(result.Debug) != 1 || result.Debug[0] != want {
					t.Errorf("native %q; Glade %q", want, result.Debug)
				}
			})
		}
	}
}

// Both callout entry points must reject pending work before transport or mocks.
func TestCalloutUncommittedHTTPAndSOAPAnonymousAndIsTestNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/callout_uncommitted_work/native", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := map[string]string{}
			for _, line := range strings.Split(string(raw), "\n") {
				if key, value, ok := strings.Cut(line, "\t"); ok {
					rows[key] = value
				}
			}
			for _, entry := range []string{"HTTP", "SOAP"} {
				for _, shape := range []struct{ name, id, setup string }{
					{"insert", "UW01", `insert new Account(Name='Pending');`},
					{"failed_database_insert", "UW16", `Database.insert(new Account(),false);`},
					{"savepoint", "UW05", `Database.setSavepoint();`},
					{"rollback_active_savepoint", "UW06", `Savepoint point=Database.setSavepoint();insert new Account(Name='Rolled back');Database.rollback(point);`},
					{"email", "UW11", `Messaging.SingleEmailMessage email=new Messaging.SingleEmailMessage();email.setToAddresses(new String[]{'probe@example.invalid'});email.setSubject('Probe');email.setPlainTextBody('Synthetic observation');Messaging.sendEmail(new Messaging.SingleEmailMessage[]{email});`},
				} {
					t.Run(api+"/"+route+"/"+entry+"/"+shape.name, func(t *testing.T) {
						call := `HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);`
						if entry == "SOAP" {
							call = `WebServiceCallout.invoke(new Account(),new Account(),new Map<String,Object>(),new String[]{'https://native.invalid','action','urn:probe','request','urn:probe','response','Account'});`
						}
						source := shape.setup + `String observed='ok';try{` + call + `}catch(Exception failure){observed=failure.getTypeName()+'|'+failure.getMessage();}System.debug(observed);System.debug(Limits.getCallouts());`
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := New(nil)
						org := testDataOrg()
						org.APIVersion = api
						machine.SetOrg(&org)
						if route == "istest" {
							machine.EnableTestContext()
							machine.testContext.HTTPMock = Object("SyntheticResponse")
							machine.testContext.WebServiceMock = Object("SyntheticSoapResponse")
						}
						result, err := machine.Execute(program)
						if err != nil {
							t.Fatal(err)
						}
						if len(result.Debug) != 2 || result.Debug[0] != rows[shape.id] || result.Debug[1] != "0" {
							t.Errorf("native %q with callouts=0; Glade %q", rows[shape.id], result.Debug)
						}
					})
				}
			}
		}
	}
}

func TestCalloutUncommittedHTTPAndSOAPAnonymousAndIsTestNewExecutionRoutes(t *testing.T) {
	for _, route := range []string{"anonymous", "istest"} {
		for _, entry := range []string{"HTTP", "SOAP"} {
			t.Run(route+"/"+entry, func(t *testing.T) {
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				if route == "istest" {
					machine.EnableTestContext()
					machine.testContext.HTTPMock = Object("SyntheticResponse")
					machine.testContext.WebServiceMock = Object("SyntheticSoapResponse")
					if err := machine.RegisterClass(Class{Name: "SyntheticSoapResponse", Methods: map[string]Method{"doInvoke": {
						Name: "SyntheticSoapResponse.doInvoke", ClassName: "SyntheticSoapResponse", ReturnType: "void",
						Params: []Param{{Name: "stub", Type: "Object"}, {Name: "request", Type: "Object"}, {Name: "response", Type: "Map<String,Object>"}, {Name: "endpoint", Type: "String"}, {Name: "soapAction", Type: "String"}, {Name: "requestName", Type: "String"}, {Name: "responseNS", Type: "String"}, {Name: "responseName", Type: "String"}, {Name: "responseType", Type: "String"}},
					}}}); err != nil {
						t.Fatal(err)
					}
				}
				first, err := CompileAnonymous(`insert new Account(Name='Earlier transaction');`)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(first); err != nil {
					t.Fatal(err)
				}
				source := `HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);`
				if entry == "SOAP" {
					source = `WebServiceCallout.invoke(new Account(),new Account(),new Map<String,Object>(),new String[]{'https://native.invalid','action','urn:probe','request','urn:probe','response','Account'});`
				}
				next, err := CompileAnonymous(source)
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(next)
				if route == "istest" {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				wantType := "UnsupportedFeature"
				if entry == "HTTP" {
					wantType = "CalloutException"
				}
				var fault *RuntimeError
				if !errors.As(err, &fault) || strings.TrimPrefix(fault.Type, "System.") != wantType {
					t.Fatalf("clean transaction must reach %s: %v", wantType, err)
				}
				if entry == "HTTP" && fault.ExceptionMessage() != "Unauthorized endpoint, please check Setup->Security->Remote site settings. endpoint = https://native.invalid" {
					t.Fatalf("clean HTTP transaction: %v", err)
				}
			})
		}
	}
}

func TestCalloutUncommittedFutureLexicalAndQualifiedAnonymousAndIsTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/callout_uncommitted_work/native", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			for _, line := range strings.Split(string(raw), "\n") {
				if strings.HasPrefix(line, "UW10\t") {
					want = strings.TrimPrefix(line, "UW10\t")
				}
			}
			if want == "" {
				t.Fatal("missing native future submission row")
			}
			for _, invocation := range []string{"later();", "FutureWorker.later();"} {
				t.Run(api+"/"+route+"/"+invocation, func(t *testing.T) {
					machine := New(nil)
					org := testDataOrg()
					org.APIVersion = api
					machine.SetOrg(&org)
					if route == "istest" {
						machine.EnableTestContext()
						machine.testContext.HTTPMock = Object("SyntheticResponse")
					}
					if err := machine.RegisterClass(Class{Name: "FutureWorker", Methods: map[string]Method{"later": {Name: "FutureWorker.later", ClassName: "FutureWorker", ReturnType: "void", IsStatic: true, Modifiers: []string{"future"}, APIVersion: api}}}); err != nil {
						t.Fatal(err)
					}
					source := invocation + `HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');String observed='ok';try{new Http().send(request);}catch(Exception failure){observed=failure.getTypeName()+'|'+failure.getMessage();}System.debug(observed);`
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.ExecuteInClass(program, "FutureWorker")
					if err != nil {
						t.Fatal(err)
					}
					if len(result.Debug) != 1 || result.Debug[0] != want {
						t.Errorf("native %q; Glade %q", want, result.Debug)
					}
				})
			}
		}
	}
}
