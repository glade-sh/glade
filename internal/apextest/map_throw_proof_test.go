package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact Salesforce-admitted source and component versions.
func TestRunMapEdgeThrowingKey(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2ThrowKey.cls"), "public class GladeB2ThrowKey {\n    public class MarkerException extends Exception {}\n    public Boolean failHash = false;\n    public Boolean failEquals = false;\n    public String value;\n    public GladeB2ThrowKey(String text) { value = text; }\n    public Integer hashCode() {\n        if (failHash) { throw new MarkerException('owned hash marker'); }\n        return 7;\n    }\n    public Boolean equals(Object other) {\n        if (failEquals) { throw new MarkerException('owned equals marker'); }\n        return other instanceof GladeB2ThrowKey && value == ((GladeB2ThrowKey)other).value;\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2ThrowKey.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2ThrowObservationTest.cls"), "@IsTest private class GladeB2ThrowObservationTest {\n    private static void observe(String operation, Boolean hashFailure) {\n        GladeB2ThrowKey stored = new GladeB2ThrowKey('stored');\n        GladeB2ThrowKey lookup = new GladeB2ThrowKey('different');\n        Map<GladeB2ThrowKey, String> values = new Map<GladeB2ThrowKey, String>{stored => 'ORIGINAL'};\n        stored.failEquals = !hashFailure;\n        lookup.failEquals = !hashFailure;\n        lookup.failHash = hashFailure;\n        String observation = 'NO_EXCEPTION';\n        String observedType;\n        String observedMessage;\n        Boolean caught = false;\n        try {\n            Object result;\n            if (operation == 'put') { result = values.put(lookup, 'NEW'); }\n            else if (operation == 'get') { result = values.get(lookup); }\n            else if (operation == 'containsKey') { result = values.containsKey(lookup); }\n            else { result = values.remove(lookup); }\n            observation += ': result=' + String.valueOf(result);\n        } catch (Exception problem) {\n            caught = true;\n            observedType = problem.getTypeName();\n            observedMessage = problem.getMessage();\n            observation = problem.getTypeName() + ': ' + problem.getMessage();\n        }\n        System.assertEquals(true, caught, operation + ': exception required');\n        System.assertEquals('GladeB2ThrowKey.MarkerException', observedType);\n        System.assertEquals(hashFailure ? 'owned hash marker' : 'owned equals marker', observedMessage);\n        System.assertEquals(operation == 'put' ? 2 : 1, values.size());\n    }\n    @IsTest static void putThrowingHashObservation() { observe('put', true); }\n    @IsTest static void getThrowingHashObservation() { observe('get', true); }\n    @IsTest static void containsKeyThrowingHashObservation() { observe('containsKey', true); }\n    @IsTest static void removeThrowingHashObservation() { observe('remove', true); }\n    @IsTest static void putThrowingEqualsObservation() { observe('put', false); }\n    @IsTest static void getThrowingEqualsObservation() { observe('get', false); }\n    @IsTest static void containsKeyThrowingEqualsObservation() { observe('containsKey', false); }\n    @IsTest static void removeThrowingEqualsObservation() { observe('remove', false); }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2ThrowObservationTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>60.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"65.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 8 || got.Passed != 8 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}
