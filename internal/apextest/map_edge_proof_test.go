package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact Salesforce-admitted source and component versions.
func TestRunMapEdgeDecimalIntegerLookup(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2DecimalRemoveTest.cls"), "@IsTest private class GladeB2DecimalRemoveTest {\n    @IsTest static void getIntegerLookupDoesNotMatchDecimalKey() {\n        Decimal key = 1.0;\n        Integer lookup = 1;\n        Map<Decimal, String> values = new Map<Decimal, String>{key => 'FIRST', 2.0 => 'SECOND'};\n        System.assertEquals('FIRST', values.get(key));\n        System.assertEquals(true, values.containsKey(key));\n        System.assertEquals(null, values.get(lookup));\n        System.assertEquals(2, values.size());\n        System.assertEquals('FIRST', values.get(key));\n        System.assertEquals('SECOND', values.get(2.0));\n        System.assertEquals('FIRST', values.remove(key));\n        System.assertEquals(1, values.size());\n        System.assertEquals(false, values.containsKey(key));\n        System.assertEquals(null, values.get(key));\n    }\n    @IsTest static void containsKeyIntegerLookupDoesNotMatchDecimalKey() {\n        Decimal key = 1.0;\n        Integer lookup = 1;\n        Map<Decimal, String> values = new Map<Decimal, String>{key => 'FIRST', 2.0 => 'SECOND'};\n        System.assertEquals('FIRST', values.get(key));\n        System.assertEquals(true, values.containsKey(key));\n        System.assertEquals(false, values.containsKey(lookup));\n        System.assertEquals(2, values.size());\n        System.assertEquals('FIRST', values.get(key));\n        System.assertEquals('SECOND', values.get(2.0));\n        System.assertEquals('FIRST', values.remove(key));\n        System.assertEquals(1, values.size());\n        System.assertEquals(false, values.containsKey(key));\n        System.assertEquals(null, values.get(key));\n    }\n    @IsTest static void removeIntegerLookupDoesNotRemoveDecimalKey() {\n        Decimal key = 1.0;\n        Integer lookup = 1;\n        Map<Decimal, String> values = new Map<Decimal, String>{key => 'FIRST', 2.0 => 'SECOND'};\n        System.assertEquals('FIRST', values.get(key));\n        System.assertEquals(true, values.containsKey(key));\n        System.assertEquals(null, values.remove(lookup));\n        System.assertEquals(2, values.size());\n        System.assertEquals('FIRST', values.get(key));\n        System.assertEquals('SECOND', values.get(2.0));\n        System.assertEquals('FIRST', values.remove(key));\n        System.assertEquals(1, values.size());\n        System.assertEquals(false, values.containsKey(key));\n        System.assertEquals(null, values.get(key));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2DecimalRemoveTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>60.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"65.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact Salesforce-admitted source and component versions.
func TestRunMapEdgeEqualMapHashes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2Key.cls"), "public class GladeB2Key {\n    public String value;\n    public GladeB2Key(String value) { this.value = value; }\n    public Boolean equals(Object other) { return other instanceof GladeB2Key && value == ((GladeB2Key)other).value; }\n    public Integer hashCode() { return 7; }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2Key.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2EqualHashTest.cls"), "@IsTest private class GladeB2EqualHashTest {\n    private static Map<GladeB2Key, String> forward() {\n        return new Map<GladeB2Key, String>{new GladeB2Key('first') => 'FIRST', new GladeB2Key('second') => 'SECOND'};\n    }\n    private static Map<GladeB2Key, String> reverse() {\n        return new Map<GladeB2Key, String>{new GladeB2Key('second') => 'SECOND', new GladeB2Key('first') => 'FIRST'};\n    }\n    @IsTest static void equalEntriesHaveEqualSystemHashCodes() {\n        Map<GladeB2Key, String> first = forward();\n        Map<GladeB2Key, String> second = reverse();\n        System.assertEquals(2, first.size()); System.assertEquals(2, second.size());\n        System.assertEquals('FIRST', first.get(new GladeB2Key('first')));\n        System.assertEquals('SECOND', first.get(new GladeB2Key('second')));\n        System.assertEquals('FIRST', second.get(new GladeB2Key('first')));\n        System.assertEquals('SECOND', second.get(new GladeB2Key('second')));\n        System.assertEquals(System.hashCode(first), System.hashCode(second));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB2EqualHashTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>60.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"65.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}
