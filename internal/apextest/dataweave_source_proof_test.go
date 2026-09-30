package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// These source and metadata payloads are unchanged Salesforce-admitted packets.
// The acceptance run must configure an explicitly installed DataWeave toolchain.
func TestRunDataWeaveSourceSourceNames(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5SourceProof.cls"), "@IsTest\nprivate class GladeC5SourceProof {\n    @IsTest static void sourceControlsResults() {\n        Map<String, Object> inputs = new Map<String, Object>{'payload' => '{\"left\":3,\"right\":4}'};\n        String first = DataWeave.Script.createScript('helloWorld').execute(inputs).getValueAsString();\n        String renamed = DataWeave.Script.createScript('gladeC5Renamed').execute(inputs).getValueAsString();\n        System.assertEquals(first, renamed);\n        Map<String, Object> decoded = (Map<String, Object>)JSON.deserializeUntyped(first);\n        System.assertEquals(1, decoded.size());\n        System.assertEquals(7, decoded.get('total'));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5SourceProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/helloWorld.dwl"), "%dw 2.0\ninput payload application/json\noutput application/json indent=false\n---\n{ total: payload.left + payload.right }\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/helloWorld.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Renamed.dwl"), "%dw 2.0\ninput payload application/json\noutput application/json indent=false\n---\n{ total: payload.left + payload.right }\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Renamed.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

func TestRunDataWeaveSourceChangedSource(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5SourceProof.cls"), "@IsTest\nprivate class GladeC5SourceProof {\n    @IsTest static void sourceControlsResults() {\n        Map<String, Object> inputs = new Map<String, Object>{'payload' => '{\"left\":3,\"right\":4}'};\n        String first = DataWeave.Script.createScript('helloWorld').execute(inputs).getValueAsString();\n        String renamed = DataWeave.Script.createScript('gladeC5Renamed').execute(inputs).getValueAsString();\n        System.assertEquals(first, renamed);\n        Map<String, Object> decoded = (Map<String, Object>)JSON.deserializeUntyped(first);\n        System.assertEquals(1, decoded.size());\n        System.assertEquals(12, decoded.get('total'));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5SourceProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/helloWorld.dwl"), "%dw 2.0\ninput payload application/json\noutput application/json indent=false\n---\n{ total: payload.left * payload.right }\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/helloWorld.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Renamed.dwl"), "%dw 2.0\ninput payload application/json\noutput application/json indent=false\n---\n{ total: payload.left * payload.right }\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Renamed.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

func TestRunDataWeaveSourceBuiltinImports(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5ImportProof.cls"), "@IsTest\nprivate class GladeC5ImportProof {\n    @IsTest static void builtInImportUsesInput() {\n        String first = DataWeave.Script.createScript('gladeC5Import').execute(\n            new Map<String, Object>{'payload' => '{\"text\":\"first\",\"number\":1.25}'}).getValueAsString();\n        String second = DataWeave.Script.createScript('gladeC5Import').execute(\n            new Map<String, Object>{'payload' => '{\"text\":\"next\",\"number\":2.75}'}).getValueAsString();\n        Map<String, Object> a = (Map<String, Object>)JSON.deserializeUntyped(first);\n        Map<String, Object> b = (Map<String, Object>)JSON.deserializeUntyped(second);\n        System.assertEquals(3, a.size());\n        System.assertEquals('FIRST', a.get('text'));\n        System.assertEquals('NEXT', b.get('text'));\n        System.assertEquals(1.25, a.get('number'));\n        System.assertEquals(2.75, b.get('number'));\n        System.assertEquals(true, a.containsKey('missing'));\n        System.assertEquals(null, a.get('missing'));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5ImportProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Import.dwl"), "%dw 2.0\nimport upper from dw::Core\ninput payload application/json\noutput application/json indent=false\n---\n{ text: upper(payload.text), missing: payload.missing default null, number: payload.number }\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Import.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

func TestRunDataWeaveSourceRestrictedWordsInLiterals(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5LiteralProof.cls"), "@IsTest\nprivate class GladeC5LiteralProof {\n    @IsTest static void sourceControlPreservesValues() {\n        String result = DataWeave.Script.createScript('gladeC5Literal').execute(new Map<String,Object>{'payload' => '{\"left\":3,\"right\":4}'}).getValueAsString();\n        Map<String,Object> value = (Map<String,Object>)JSON.deserializeUntyped(result); System.assertEquals(7, value.get('total')); System.assertEquals('readUrl envVar java!', value.get('words'));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5LiteralProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Literal.dwl"), "%dw 2.0\ninput payload application/json\noutput application/json indent=false\n---\n{total:payload.left + payload.right, words:\"readUrl envVar java!\"}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Literal.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

func TestRunDataWeaveSourceLocalFunctionShadow(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5ShadowProof.cls"), "@IsTest\nprivate class GladeC5ShadowProof {\n    @IsTest static void sourceControlPreservesValues() {\n        String result = DataWeave.Script.createScript('gladeC5Shadow').execute(new Map<String,Object>{'payload' => '{\"left\":3,\"right\":4}'}).getValueAsString();\n        System.assertEquals(12, JSON.deserializeUntyped(result));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5ShadowProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Shadow.dwl"), "%dw 2.0\nfun readUrl(value) = value + 5\ninput payload application/json\noutput application/json indent=false\n---\nreadUrl(payload.left + payload.right)\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Shadow.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

func TestRunDataWeaveSourceReadURLDeadBranch(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5ReadDeadProof.cls"), "@IsTest\nprivate class GladeC5ReadDeadProof {\n    @IsTest static void observesCreationAndExecution() {\n        String phase = 'create';\n        String observed = 'NO_EXCEPTION';\n        try {\n            DataWeave.Script script = DataWeave.Script.createScript('gladeC5ReadDead');\n            phase = 'execute';\n            String value = script.execute(new Map<String,Object>{'payload' => '{\"left\":3,\"right\":4}'}).getValueAsString();\n            phase = 'complete';\n            observed = 'VALUE=' + value;\n        } catch (Exception error) {\n            observed = error.getTypeName() + ': ' + error.getMessage();\n        }\n        System.assertEquals('complete', phase);\n        System.assertEquals('VALUE=7', observed);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5ReadDeadProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5ReadDead.dwl"), "%dw 2.0\ninput payload application/json\noutput application/json indent=false\n---\nif (false) readUrl(\"https://example.invalid/glade-owned-never-read\", \"text/plain\") else payload.left + payload.right\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5ReadDead.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

func TestRunDataWeaveSourceEnvironmentDeadBranch(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5EnvDeadProof.cls"), "@IsTest\nprivate class GladeC5EnvDeadProof {\n    @IsTest static void observesCreationAndExecution() {\n        String phase = 'create';\n        String observed = 'NO_EXCEPTION';\n        try {\n            DataWeave.Script script = DataWeave.Script.createScript('gladeC5EnvDead');\n            phase = 'execute';\n            String value = script.execute(new Map<String,Object>{'payload' => '{\"left\":3,\"right\":4}'}).getValueAsString();\n            phase = 'complete';\n            observed = 'VALUE=' + value;\n        } catch (Exception error) {\n            observed = error.getTypeName() + ': ' + error.getMessage();\n        }\n        System.assertEquals('complete', phase);\n        System.assertEquals('VALUE=7', observed);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5EnvDeadProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5EnvDead.dwl"), "%dw 2.0\nimport envVar from dw::System\ninput payload application/json\noutput application/json indent=false\n---\nif (false) envVar(\"GLADE_OWNED_NEVER_READ\") else payload.left + payload.right\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5EnvDead.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

func TestRunDataWeaveSourceReadURLAlias(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5ReadAliasProof.cls"), "@IsTest\nprivate class GladeC5ReadAliasProof {\n    @IsTest static void observesCreationAndExecution() {\n        String phase = 'create';\n        String observed = 'NO_EXCEPTION';\n        try {\n            DataWeave.Script script = DataWeave.Script.createScript('gladeC5ReadAlias');\n            phase = 'execute';\n            String value = script.execute(new Map<String,Object>{'payload' => '{\"left\":3,\"right\":4}'}).getValueAsString();\n            phase = 'complete';\n            observed = 'VALUE=' + value;\n        } catch (Exception error) {\n            observed = error.getTypeName() + ': ' + error.getMessage();\n        }\n        System.assertEquals('complete', phase);\n        System.assertEquals('VALUE=7', observed);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5ReadAliasProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5ReadAlias.dwl"), "%dw 2.0\nvar ownedReader = readUrl\ninput payload application/json\noutput application/json indent=false\n---\nif (false) ownedReader(\"https://example.invalid/glade-owned-never-read\", \"text/plain\") else payload.left + payload.right\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5ReadAlias.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

func TestRunDataWeaveSourceEnvironmentAlias(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5EnvAliasProof.cls"), "@IsTest\nprivate class GladeC5EnvAliasProof {\n    @IsTest static void observesCreationAndExecution() {\n        String phase = 'create';\n        String observed = 'NO_EXCEPTION';\n        try {\n            DataWeave.Script script = DataWeave.Script.createScript('gladeC5EnvAlias');\n            phase = 'execute';\n            String value = script.execute(new Map<String,Object>{'payload' => '{\"left\":3,\"right\":4}'}).getValueAsString();\n            phase = 'complete';\n            observed = 'VALUE=' + value;\n        } catch (Exception error) {\n            observed = error.getTypeName() + ': ' + error.getMessage();\n        }\n        System.assertEquals('complete', phase);\n        System.assertEquals('VALUE=7', observed);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5EnvAliasProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5EnvAlias.dwl"), "%dw 2.0\nimport envVar as ownedEnvironment from dw::System\ninput payload application/json\noutput application/json indent=false\n---\nif (false) ownedEnvironment(\"GLADE_OWNED_NEVER_READ\") else payload.left + payload.right\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5EnvAlias.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}
func TestRunDataWeaveSourceProjectModule(t *testing.T) {
	home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME")
	if home == "" {
		t.Skip("requires explicitly installed DataWeave toolchain")
	}
	t.Setenv("GLADE_HOME", home)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5CustomProof.cls"), "@IsTest\nprivate class GladeC5CustomProof {\n    @IsTest static void observesCreationAndExecution() {\n        String phase = 'create';\n        String observed = 'NO_EXCEPTION';\n        try {\n            DataWeave.Script script = DataWeave.Script.createScript('gladeC5Custom');\n            phase = 'execute';\n            String value = script.execute(new Map<String,Object>{'payload' => '{\"left\":3,\"right\":4}'}).getValueAsString();\n            phase = 'complete';\n            observed = 'VALUE=' + value;\n        } catch (Exception error) {\n            observed = error.getTypeName() + ': ' + error.getMessage();\n        }\n        System.assertEquals('complete', phase);\n        System.assertEquals('VALUE=7', observed);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC5CustomProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Custom.dwl"), "%dw 2.0\nimport ownedSum from gladeC5OwnedModule\ninput payload application/json\noutput application/json indent=false\n---\nownedSum(payload.left, payload.right)\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5Custom.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5OwnedModule.dwl"), "%dw 2.0\nfun ownedSum(left, right) = left + right\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/gladeC5OwnedModule.dwl-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DataWeaveResource xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><isGlobal>false</isGlobal></DataWeaveResource>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}
