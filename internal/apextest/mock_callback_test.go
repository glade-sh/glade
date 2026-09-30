package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact owned Salesforce-admitted payload; original component APIs retained.
func TestRunHTTPInstalledMock(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"51.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3Http51.cls"), "@IsTest private class GladeA3Http51 {\n    private class RecordingMock implements HttpCalloutMock {\n        public Integer calls = 0;\n        public String prefix;\n        public RecordingMock(String value) { prefix = value; }\n        public HttpResponse respond(HttpRequest request) {\n            calls++;\n            System.assertEquals('POST', request.getMethod());\n            System.assertEquals('https://glade-proof.example.invalid/' + prefix + '/' + calls, request.getEndpoint());\n            System.assertEquals(prefix + '-body-' + calls, request.getBody());\n            System.assertEquals(prefix + '-header-' + calls, request.getHeader('X-Glade-Proof'));\n            HttpResponse response = new HttpResponse();\n            response.setStatusCode(200 + calls);\n            response.setStatus('Glade ' + calls);\n            response.setHeader('X-Glade-Reply', prefix);\n            response.setBody(prefix + '-reply-' + calls);\n            return response;\n        }\n    }\n    private static void callTwice(String prefix) {\n        for (Integer n = 1; n <= 2; n++) {\n            HttpRequest request = new HttpRequest();\n            request.setMethod('POST');\n            request.setEndpoint('https://glade-proof.example.invalid/' + prefix + '/' + n);\n            request.setBody(prefix + '-body-' + n);\n            request.setHeader('X-Glade-Proof', prefix + '-header-' + n);\n            HttpResponse response = new Http().send(request);\n            System.assertEquals(200 + n, response.getStatusCode());\n            System.assertEquals('Glade ' + n, response.getStatus());\n            System.assertEquals(prefix, response.getHeader('X-Glade-Reply'));\n            System.assertEquals(prefix + '-reply-' + n, response.getBody());\n        }\n    }\n    @IsTest static void installedMockReceivesTwoCallsAndReplacementIsIndependent() {\n        RecordingMock first = new RecordingMock('first');\n        Test.setMock(HttpCalloutMock.class, first);\n        callTwice('first');\n        System.assertEquals(2, first.calls);\n        RecordingMock second = new RecordingMock('second');\n        Test.setMock(HttpCalloutMock.class, second);\n        callTwice('second');\n        System.assertEquals(2, first.calls);\n        System.assertEquals(2, second.calls);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3Http51.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>51.0</apiVersion><status>Active</status></ApexClass>\n")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("mock contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted payload; original component APIs retained.
func TestRunHTTPCaseVariantMock(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"56.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3Http56.cls"), "@IsTest private class GladeA3Http56 {\n    private class RecordingMock implements HttpCalloutMock {\n        public Integer calls = 0;\n        public String prefix;\n        public RecordingMock(String value) { prefix = value; }\n        public HttpResponse respond(HttpRequest request) {\n            calls++;\n            System.assertEquals('POST', request.getMethod());\n            System.assertEquals('https://glade-proof.example.invalid/' + prefix + '/' + calls, request.getEndpoint());\n            System.assertEquals(prefix + '-body-' + calls, request.getBody());\n            System.assertEquals(prefix + '-header-' + calls, request.getHeader('X-Glade-Proof'));\n            HttpResponse response = new HttpResponse();\n            response.setStatusCode(200 + calls);\n            response.setStatus('Glade ' + calls);\n            response.setHeader('X-Glade-Reply', prefix);\n            response.setBody(prefix + '-reply-' + calls);\n            return response;\n        }\n    }\n    private static void callTwice(String prefix) {\n        for (Integer n = 1; n <= 2; n++) {\n            HttpRequest request = new HttpRequest();\n            request.setMethod('POST');\n            request.setEndpoint('https://glade-proof.example.invalid/' + prefix + '/' + n);\n            request.setBody(prefix + '-body-' + n);\n            request.setHeader('X-Glade-Proof', prefix + '-header-' + n);\n            HttpResponse response = new Http().send(request);\n            System.assertEquals(200 + n, response.getStatusCode());\n            System.assertEquals('Glade ' + n, response.getStatus());\n            System.assertEquals(prefix, response.getHeader('X-Glade-Reply'));\n            System.assertEquals(prefix + '-reply-' + n, response.getBody());\n        }\n    }\n    @IsTest static void installedMockReceivesTwoCallsAndReplacementIsIndependent() {\n        RecordingMock first = new RecordingMock('first');\n        Test.setMock(httpCalloutMock.class, first);\n        callTwice('first');\n        System.assertEquals(2, first.calls);\n        RecordingMock second = new RecordingMock('second');\n        Test.setMock(httpCalloutMock.class, second);\n        callTwice('second');\n        System.assertEquals(2, first.calls);\n        System.assertEquals(2, second.calls);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3Http56.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>56.0</apiVersion><status>Active</status></ApexClass>\n")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("mock contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted payload; original component APIs retained.
func TestRunStubProviderInvocation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"55.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubTarget55.cls"), "public class GladeA3StubTarget55 {\n    public String format(String label, Integer amount) { return 'REAL_IMPLEMENTATION'; }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubTarget55.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubProof55.cls"), "@IsTest private class GladeA3StubProof55 {\n    private class Provider implements System.StubProvider {\n        public Integer calls = 0;\n        public String prefix;\n        public Object expected;\n        public Provider(String value) { prefix = value; }\n        public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType,\n            List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {\n            calls++;\n            System.assert(stubbedObject === expected);\n            System.assertEquals('format', stubbedMethodName);\n            System.assertEquals(String.class, returnType);\n            System.assertEquals(new List<Type>{String.class, Integer.class}, listOfParamTypes);\n            System.assertEquals(new List<String>{'label', 'amount'}, listOfParamNames);\n            System.assertEquals(new List<Object>{prefix + '-input-' + calls, calls * 10}, listOfArgs);\n            return prefix + '-result-' + calls;\n        }\n    }\n    @IsTest static void methodMetadataArgumentsAndTwoIndependentStubs() {\n        Provider firstProvider = new Provider('first');\n        Provider secondProvider = new Provider('second');\n        GladeA3StubTarget55 first = (GladeA3StubTarget55)Test.createStub(GladeA3StubTarget55.class, firstProvider);\n        GladeA3StubTarget55 second = (GladeA3StubTarget55)Test.createStub(GladeA3StubTarget55.class, secondProvider);\n        System.assert(first !== second);\n        firstProvider.expected = first;\n        secondProvider.expected = second;\n        System.assertEquals('first-result-1', first.format('first-input-1', 10));\n        System.assertEquals('second-result-1', second.format('second-input-1', 10));\n        System.assertEquals('first-result-2', first.format('first-input-2', 20));\n        System.assertEquals('second-result-2', second.format('second-input-2', 20));\n        System.assertEquals(2, firstProvider.calls);\n        System.assertEquals(2, secondProvider.calls);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubProof55.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>\n")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("mock contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted payload; original component APIs retained.
func TestRunStubProviderTypedNull(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"55.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubTarget55.cls"), "public class GladeA3StubTarget55 {\n    public String format(String label, Integer amount) { return 'REAL_IMPLEMENTATION'; }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubTarget55.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubNull55.cls"), "@IsTest private class GladeA3StubNull55 {\n    private class Provider implements System.StubProvider {\n        public Integer calls = 0;\n        public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType,\n            List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {\n            calls++;\n            System.assertEquals('format', stubbedMethodName);\n            System.assertEquals(String.class, returnType);\n            String empty = null;\n            return empty;\n        }\n    }\n    @IsTest static void typedNullRemainsNullAcrossTwoCalls() {\n        Provider provider = new Provider();\n        GladeA3StubTarget55 stub = (GladeA3StubTarget55)Test.createStub(GladeA3StubTarget55.class, provider);\n        String first = stub.format('first', 1);\n        String second = stub.format('second', 2);\n        System.assertEquals(null, first);\n        System.assertEquals(null, second);\n        System.assertEquals(2, provider.calls);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3StubNull55.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>\n")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("mock contract failed: %s", data)
	}
}

func TestRunStubProviderPreservesCallSiteCaseForNestedList(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"55.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3CaseDrive.cls"), `public interface GladeA3CaseDrive {
    List<List<String>> getSpreadsheetData(String id);
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3CaseDrive.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?><ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3CaseProof.cls"), `@IsTest private class GladeA3CaseProof {
    private class Provider implements System.StubProvider {
        public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType,
            List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
            System.assertEquals('GetSpreadsheetData', stubbedMethodName);
            return new List<List<String>>{ new List<String>{ 'nested-value' } };
        }
    }
    @IsTest static void preservesCallSiteCaseAndNestedReturn() {
        GladeA3CaseDrive drive = (GladeA3CaseDrive)Test.createStub(GladeA3CaseDrive.class, new Provider());
        List<List<String>> rows = drive.GetSpreadsheetData('sheet');
        System.assertEquals('nested-value', rows[0][0]);
    }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeA3CaseProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?><ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("case-preserving nested stub contract failed: %s", data)
	}
}
