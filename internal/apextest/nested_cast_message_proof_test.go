package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact four API63 assertions admitted by Salesforce SF151.
func TestNestedCastExceptionMessageAPI63Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeNestedCast63Proof.cls"), "@IsTest private class GladeNestedCast63Proof {\n    private class Missing {}\n    private virtual class Base {}\n    private class Child extends Base {}\n\n    private static String nestedMessage(Object obj) {\n        try {\n            Missing shouldThrow = (Missing) obj;\n            System.assert(false, 'Incompatible cast must throw');\n        } catch (System.TypeException expectedException) {\n            return expectedException.getMessage();\n        }\n        return null;\n    }\n    private static void assertNestedSource(Object obj, String expectedSource) {\n        String message = nestedMessage(obj).toLowerCase();\n        System.assertEquals('invalid conversion from runtime type ' + expectedSource + ' to gladenestedcast63proof.missing', message);\n        String extracted = message.substringBetween('invalid conversion from runtime type ', ' to gladenestedcast63proof.missing');\n        System.assertEquals(expectedSource, extracted);\n        System.assertEquals(expectedSource, extracted.toLowerCase());\n    }\n    @IsTest static void nestedTargetFromTopLevelSource() {\n        assertNestedSource(new GladeCastSource63(), 'gladecastsource63');\n    }\n    @IsTest static void nestedTargetFromSObject() {\n        assertNestedSource(new Account(), 'account');\n    }\n    @IsTest static void nestedRuntimeSource() {\n        Base source = new Child();\n        assertNestedSource(source, 'gladenestedcast63proof.child');\n    }\n    @IsTest static void topLevelTargetControl() {\n        Object obj = new GladeCastSource63();\n        try {\n            GladeCastTarget63 shouldThrow = (GladeCastTarget63) obj;\n            System.assert(false, 'Incompatible top-level cast must throw');\n        } catch (System.TypeException expectedException) {\n            System.assertEquals('invalid conversion from runtime type gladecastsource63 to gladecasttarget63', expectedException.getMessage().toLowerCase());\n        }\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeNestedCast63Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCastSource63.cls"), "@IsTest public class GladeCastSource63 {}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCastSource63.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCastTarget63.cls"), "@IsTest public class GladeCastTarget63 {}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCastTarget63.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"63.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("report: %s", data)
	if got := run.Summary(); got.Total != 4 || got.Passed != 4 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		t.Fatalf("nested cast proof: %s", data)
	}
}
