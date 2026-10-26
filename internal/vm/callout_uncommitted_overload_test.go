package vm

import (
	"errors"
	"strings"
	"testing"
)

// Native UW09 and UW11 (callout_uncommitted_work/native/api{62,67}/{anonymous,istest}/org.tsv
// lines 10 and 12) capture only System.enqueueJob(job) and the one-argument
// Messaging.sendEmail(List<SingleEmailMessage>). The overloads below have no
// native row, so they keep the base callout eligibility until one is captured.
func TestCalloutUncommittedUncapturedOverloadsKeepBaseEligibility(t *testing.T) {
	const email = `Messaging.SingleEmailMessage email=new Messaging.SingleEmailMessage();email.setToAddresses(new String[]{'probe@example.invalid'});email.setSubject('Probe');email.setPlainTextBody('Synthetic observation');`
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			for _, entry := range []string{"HTTP", "SOAP"} {
				for _, shape := range []struct{ name, setup string }{
					{"sendEmail_allOrNothing_true", email + `Messaging.sendEmail(new Messaging.SingleEmailMessage[]{email},true);`},
					{"sendEmail_allOrNothing_false", email + `Messaging.sendEmail(new Messaging.SingleEmailMessage[]{email},false);`},
					{"sendEmailMessage", `Messaging.sendEmailMessage(new List<Id>{'02s000000000001AAA'});`},
					{"sendEmailMessage_allOrNothing", `Messaging.sendEmailMessage(new List<Id>{'02s000000000001AAA'},false);`},
					{"enqueueJob_delay", `System.enqueueJob(new OverloadWorker(),5);`},
					{"enqueueJob_asyncOptions", `System.enqueueJob(new OverloadWorker(),new AsyncOptions());`},
				} {
					t.Run(api+"/"+route+"/"+entry+"/"+shape.name, func(t *testing.T) {
						call := `HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);`
						if entry == "SOAP" {
							call = `WebServiceCallout.invoke(new Account(),new Account(),new Map<String,Object>(),new String[]{'https://native.invalid','action','urn:probe','request','urn:probe','response','Account'});`
						}
						source := shape.setup + `String observed='ok';try{` + call + `}catch(Exception failure){observed=failure.getTypeName()+'|'+failure.getMessage();}System.debug(observed);`
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
							if err := machine.RegisterClass(Class{Name: "SyntheticSoapResponse", Methods: map[string]Method{"doInvoke": {
								Name: "SyntheticSoapResponse.doInvoke", ClassName: "SyntheticSoapResponse", ReturnType: "void",
								Params: []Param{{Name: "stub", Type: "Object"}, {Name: "request", Type: "Object"}, {Name: "response", Type: "Map<String,Object>"}, {Name: "endpoint", Type: "String"}, {Name: "soapAction", Type: "String"}, {Name: "requestName", Type: "String"}, {Name: "responseNS", Type: "String"}, {Name: "responseName", Type: "String"}, {Name: "responseType", Type: "String"}},
							}}}); err != nil {
								t.Fatal(err)
							}
						}
						registerQueueableWorkerForMethodTest(t, machine, "OverloadWorker", `Integer observed=1;`)
						result, err := machine.Execute(program)
						if err != nil {
							var fault *RuntimeError
							if !errors.As(err, &fault) || fault.Type != "UnsupportedFeature" {
								t.Fatalf("uncaptured overload must reach the base callout boundary: %v", err)
							}
							return
						}
						for _, line := range result.Debug {
							if strings.Contains(line, "uncommitted work pending") || strings.Contains(line, "Savepoints must be released") {
								t.Errorf("no native row backs a callout guard after %s; Glade %q", shape.name, result.Debug)
							}
						}
					})
				}
			}
		}
	}
}
