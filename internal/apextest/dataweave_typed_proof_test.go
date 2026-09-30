package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The source, metadata and assertions below are unchanged Salesforce-admitted packets.
// Acceptance requires an explicitly installed DataWeave toolchain.
func TestRunDataWeaveTypedAdmitted(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	t.Run("01-c5-typed-output-json-exact", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedOutJsonProof.cls"), "@IsTest\nprivate class GladeC5TypedOutJsonProof {\n    @IsTest static void observesJsonOutputTypeAndValue() {\n        DataWeave.Result result = DataWeave.Script.createScript('gladeC5TypedOutJson').execute(new Map<String,Object>());\n        Object value = result.getValue();\n        System.assertEquals(true, value instanceof String);\n        System.assertEquals('{\"text\": \"owned\",\"number\": 7}', (String)value);\n        System.assertEquals('{\"text\": \"owned\",\"number\": 7}', result.getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedOutJsonProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedOutJson.dwl"), "%dw 2.0\noutput application/json indent=false\n---\n{text:\"owned\", number:7}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedOutJson.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-output-json-exact\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("02-c5-typed-output-text-exact", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedOutTextProof.cls"), "@IsTest\nprivate class GladeC5TypedOutTextProof {\n    @IsTest static void observesTextOutputTypeAndValue() {\n        DataWeave.Result result = DataWeave.Script.createScript('gladeC5TypedOutText').execute(new Map<String,Object>());\n        Object value = result.getValue();\n        System.assertEquals(true, value instanceof String);\n        System.assertEquals('owned-text', (String)value);\n        System.assertEquals('owned-text', result.getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedOutTextProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedOutText.dwl"), "%dw 2.0\noutput text/plain\n---\n\"owned-text\"\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedOutText.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-output-text-exact\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("03-c5-typed-output-csv-exact", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedOutCsvProof.cls"), "@IsTest\nprivate class GladeC5TypedOutCsvProof {\n    @IsTest static void observesCsvOutputTypeAndValue() {\n        DataWeave.Result result = DataWeave.Script.createScript('gladeC5TypedOutCsv').execute(new Map<String,Object>());\n        Object value = result.getValue();\n        System.assertEquals(true, value instanceof String);\n        System.assertEquals('label,count\\nowned,7\\n', (String)value);\n        System.assertEquals('label,count\\nowned,7\\n', result.getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedOutCsvProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedOutCsv.dwl"), "%dw 2.0\noutput application/csv lineSeparator=\"\\n\"\n---\n[{label:\"owned\", count:7}]\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedOutCsv.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-output-csv-exact\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("04-c5-typed-output-blob-exact", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedOutBlobProof.cls"), "@IsTest\nprivate class GladeC5TypedOutBlobProof {\n    @IsTest static void observesBlobOutputTypeAndValue() {\n        DataWeave.Result result = DataWeave.Script.createScript('gladeC5TypedOutBlob').execute(new Map<String,Object>());\n        Object value = result.getValue();\n        System.assertEquals(true, value instanceof String);\n        System.assertEquals('owned-bytes', (String)value);\n        System.assertEquals('owned-bytes', result.getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedOutBlobProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedOutBlob.dwl"), "%dw 2.0\noutput application/octet-stream\n---\n\"owned-bytes\" as Binary {encoding:\"UTF-8\"}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedOutBlob.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-output-blob-exact\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("05-c5-typed-missing-resource-exact", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedMissingProof.cls"), "@IsTest\nprivate class GladeC5TypedMissingProof {\n    @IsTest static void observesMissingNamespace() {\n        Boolean caught = false;\n        try { DataWeave.Script.createScript('ownedmissing', 'gladeC5OwnedDefinitelyMissing'); } catch (Exception error) {\n            caught = true;\n            System.assertEquals('System.NoDataFoundException', error.getTypeName());\n            System.assertEquals('Could not find DataWeave script gladeC5OwnedDefinitelyMissing', error.getMessage());\n        }\n        System.assertEquals(true, caught);\n    }\n    @IsTest static void observesMissingScript() {\n        Boolean caught = false;\n        try { DataWeave.Script.createScript('gladeC5OwnedDefinitelyMissing'); } catch (Exception error) {\n            caught = true;\n            System.assertEquals('System.NoDataFoundException', error.getTypeName());\n            System.assertEquals('Could not find DataWeave script gladeC5OwnedDefinitelyMissing', error.getMessage());\n        }\n        System.assertEquals(true, caught);\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedMissingProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-missing-resource-exact\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("01-c5-typed-input-integer", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedIntegerProof.cls"), "@IsTest\nprivate class GladeC5TypedIntegerProof {\n    @IsTest static void transformsTwoIntegerInputs() {\n        Integer first = 5;\n        Integer second = 10;\n        DataWeave.Script script = DataWeave.Script.createScript('gladeC5TypedInteger');\n        System.assertEquals('7', script.execute(new Map<String,Object>{'payload'=>first}).getValueAsString());\n        System.assertEquals('12', script.execute(new Map<String,Object>{'payload'=>second}).getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedIntegerProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedInteger.dwl"), "%dw 2.0\ninput payload application/java\noutput application/json indent=false\n---\npayload + 2\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedInteger.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-input-integer\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("02-c5-typed-input-decimal", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedDecimalProof.cls"), "@IsTest\nprivate class GladeC5TypedDecimalProof {\n    @IsTest static void transformsTwoDecimalInputs() {\n        Decimal first = Decimal.valueOf('1.25');\n        Decimal second = Decimal.valueOf('2.375');\n        DataWeave.Script script = DataWeave.Script.createScript('gladeC5TypedDecimal');\n        System.assertEquals('2.5', script.execute(new Map<String,Object>{'payload'=>first}).getValueAsString());\n        System.assertEquals('4.75', script.execute(new Map<String,Object>{'payload'=>second}).getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedDecimalProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedDecimal.dwl"), "%dw 2.0\ninput payload application/java\noutput application/json indent=false\n---\npayload * 2\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedDecimal.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-input-decimal\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("03-c5-typed-input-boolean", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedBooleanProof.cls"), "@IsTest\nprivate class GladeC5TypedBooleanProof {\n    @IsTest static void transformsTwoBooleanInputs() {\n        Boolean first = true;\n        Boolean second = false;\n        DataWeave.Script script = DataWeave.Script.createScript('gladeC5TypedBoolean');\n        System.assertEquals('false', script.execute(new Map<String,Object>{'payload'=>first}).getValueAsString());\n        System.assertEquals('true', script.execute(new Map<String,Object>{'payload'=>second}).getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedBooleanProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedBoolean.dwl"), "%dw 2.0\ninput payload application/java\noutput application/json indent=false\n---\nnot payload\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedBoolean.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-input-boolean\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("04-c5-typed-input-null", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedNullProof.cls"), "@IsTest\nprivate class GladeC5TypedNullProof {\n    @IsTest static void transformsTwoNullInputs() {\n        Object first = null;\n        String second = 'owned';\n        DataWeave.Script script = DataWeave.Script.createScript('gladeC5TypedNull');\n        System.assertEquals('\"fallback\"', script.execute(new Map<String,Object>{'payload'=>first}).getValueAsString());\n        System.assertEquals('\"owned\"', script.execute(new Map<String,Object>{'payload'=>second}).getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedNullProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedNull.dwl"), "%dw 2.0\ninput payload application/java\noutput application/json indent=false\n---\npayload default \"fallback\"\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedNull.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-input-null\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("05-c5-typed-input-list", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedListProof.cls"), "@IsTest\nprivate class GladeC5TypedListProof {\n    @IsTest static void transformsTwoListInputs() {\n        List<Integer> first = new List<Integer>{2,3};\n        List<Integer> second = new List<Integer>{4,5};\n        DataWeave.Script script = DataWeave.Script.createScript('gladeC5TypedList');\n        System.assertEquals('10', script.execute(new Map<String,Object>{'payload'=>first}).getValueAsString());\n        System.assertEquals('18', script.execute(new Map<String,Object>{'payload'=>second}).getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedListProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedList.dwl"), "%dw 2.0\ninput payload application/java\noutput application/json indent=false\n---\nsum(payload map ($ * 2))\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedList.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-input-list\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("06-c5-typed-input-map", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedMapProof.cls"), "@IsTest\nprivate class GladeC5TypedMapProof {\n    @IsTest static void transformsTwoMapInputs() {\n        Map<String,Object> first = new Map<String,Object>{'left'=>3,'right'=>4};\n        Map<String,Object> second = new Map<String,Object>{'left'=>8,'right'=>4};\n        DataWeave.Script script = DataWeave.Script.createScript('gladeC5TypedMap');\n        System.assertEquals('7', script.execute(new Map<String,Object>{'payload'=>first}).getValueAsString());\n        System.assertEquals('12', script.execute(new Map<String,Object>{'payload'=>second}).getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedMapProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedMap.dwl"), "%dw 2.0\ninput payload application/java\noutput application/json indent=false\n---\npayload.left + payload.right\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedMap.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-input-map\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("07-c5-typed-input-account", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedAccountProof.cls"), "@IsTest\nprivate class GladeC5TypedAccountProof {\n    @IsTest static void transformsTwoAccountInputs() {\n        Account first = new Account(Name='first owned');\n        Account second = new Account(Name='second owned');\n        DataWeave.Script script = DataWeave.Script.createScript('gladeC5TypedAccount');\n        System.assertEquals('\"FIRST OWNED\"', script.execute(new Map<String,Object>{'payload'=>first}).getValueAsString());\n        System.assertEquals('\"SECOND OWNED\"', script.execute(new Map<String,Object>{'payload'=>second}).getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedAccountProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedAccount.dwl"), "%dw 2.0\ninput payload application/java\noutput application/json indent=false\n---\nupper(payload.Name)\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedAccount.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-input-account\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("12-c5-typed-output-apex-accounts", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedApexProof.cls"), "@IsTest\nprivate class GladeC5TypedApexProof {\n    @IsTest static void returnsTypedAccountList() {\n        DataWeave.Script script = DataWeave.Script.createScript('gladeC5TypedApex');\n        List<Account> values = (List<Account>)script.execute(new Map<String,Object>{'records'=>'name,revenue\\nFirst Owned,1.25\\nSecond Owned,2.5'}).getValue();\n        System.assertEquals(2, values.size());\n        System.assertEquals('First Owned', values[0].Name);\n        System.assertEquals(Decimal.valueOf('1.25'), values[0].AnnualRevenue);\n        System.assertEquals('Second Owned', values[1].Name);\n        System.assertEquals(Decimal.valueOf('2.5'), values[1].AnnualRevenue);\n        System.assertEquals(Account.SObjectType, values[0].getSObjectType());\n        System.assertEquals(null, values[0].Id);\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedApexProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedApex.dwl"), "%dw 2.0\ninput records application/csv\noutput application/apex\n---\nrecords map(record) -> {Name: record.name, AnnualRevenue: record.revenue as Number} as Object {class: \"Account\"}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedApex.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-output-apex-accounts\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("13-c5-typed-generated-resource-dispatch", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedGeneratedProof.cls"), "@IsTest\nprivate class GladeC5TypedGeneratedProof {\n    @IsTest static void executesGeneratedResourceTwice() {\n        DataWeave.Script script = new DataWeaveScriptResource.gladeC5TypedGenerated();\n        System.assertEquals('7', script.execute(new Map<String,Object>{'payload'=>'{\"left\":3,\"right\":4}'}).getValueAsString());\n        System.assertEquals('12', script.execute(new Map<String,Object>{'payload'=>'{\"left\":8,\"right\":4}'}).getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedGeneratedProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedGenerated.dwl"), "%dw 2.0\ninput payload application/json\noutput application/json indent=false\n---\npayload.left + payload.right\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedGenerated.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-generated-resource-dispatch\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
	t.Run("14-c5-typed-namespace-current-org", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedNamespaceProof.cls"), "@IsTest\nprivate class GladeC5TypedNamespaceProof {\n    @IsTest static void nullNamespaceUsesCaller() {\n        DataWeave.Script script = DataWeave.Script.createScript(null, 'gladeC5TypedNamespace');\n        System.assertEquals('7', script.execute(new Map<String,Object>{'payload'=>'{\"left\":3,\"right\":4}'}).getValueAsString());\n        System.assertEquals('12', script.execute(new Map<String,Object>{'payload'=>'{\"left\":8,\"right\":4}'}).getValueAsString());\n    }\n    @IsTest static void emptyNamespaceUsesOrg() {\n        DataWeave.Script script = DataWeave.Script.createScript('', 'gladeC5TypedNamespace');\n        System.assertEquals('7', script.execute(new Map<String,Object>{'payload'=>'{\"left\":3,\"right\":4}'}).getValueAsString());\n        System.assertEquals('12', script.execute(new Map<String,Object>{'payload'=>'{\"left\":8,\"right\":4}'}).getValueAsString());\n    }\n}\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5TypedNamespaceProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedNamespace.dwl"), "%dw 2.0\ninput payload application/json\noutput application/json indent=false\n---\npayload.left + payload.right\n")
		writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5TypedNamespace.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
		writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c5-typed-namespace-current-org\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
		run := Run(loadTestIndex(t, root), Options{})
		if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
			data, _ := json.Marshal(run)
			t.Fatalf("admitted contract failed: %s", data)
		}
	})
}
