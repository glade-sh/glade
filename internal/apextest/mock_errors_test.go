package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact owned Salesforce-admitted source and component metadata.
func TestRunHTTPFreshDefaults(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3HttpDefaults51.cls"), "@IsTest private class GladeA3HttpDefaults51 {\n    @IsTest static void freshResponseDefaults() {\n        HttpResponse response = new HttpResponse();\n        System.assertEquals(0, response.getStatusCode());\n        System.assertEquals(null, response.getStatus());\n        System.assertEquals('', response.getBody());\n        System.assertEquals(null, response.getHeader('X-Glade-Unset'));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3HttpDefaults51.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>51.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"a3-http-fresh-defaults-exact-api51\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"51.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunHTTPMissingMockError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3HttpNoMock51.cls"), "@IsTest private class GladeA3HttpNoMock51 {\n    @IsTest static void noMockExceptionObservation() {\n        HttpRequest request = new HttpRequest();\n        request.setEndpoint('https://glade-proof.example.invalid/no-mock');\n        request.setMethod('GET');\n        Boolean caught = false;\n        try { new Http().send(request); }\n        catch (TypeException problem) { caught = true; System.assertEquals('Methods defined as TestMethod do not support Web service callouts', problem.getMessage()); }\n        System.assertEquals(true, caught, 'Expected TypeException for an unmocked test callout');\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3HttpNoMock51.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>51.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"a3-http-no-mock-exact-api51\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"51.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunStubInvalidReturnError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubInvalid55.cls"), "@IsTest private class GladeA3StubInvalid55 {\n    private class Provider implements System.StubProvider {\n        public Integer calls = 0;\n        public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType,\n            List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {\n            calls++;\n            System.assertEquals('format', stubbedMethodName);\n            System.assertEquals(String.class, returnType);\n            return new Account(Name='invalid String return');\n        }\n    }\n    @IsTest static void invalidReturnConversionExceptionObservation() {\n        Provider provider = new Provider();\n        GladeA3StubTarget55 stub = (GladeA3StubTarget55)Test.createStub(GladeA3StubTarget55.class, provider);\n        Boolean caught = false;\n        try { String result = stub.format('first', 1); }\n        catch (TypeException problem) { caught = true; System.assertEquals('Invalid conversion from runtime type Account to String', problem.getMessage()); }\n        System.assertEquals(1, provider.calls);\n        System.assertEquals(true, caught, 'Expected TypeException for the incompatible provider return');\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubInvalid55.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubTarget55.cls"), "public class GladeA3StubTarget55 {\n    public String format(String label, Integer amount) { return 'REAL_IMPLEMENTATION'; }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubTarget55.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"a3-stub-invalid-return-exact-api55\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"55.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}
