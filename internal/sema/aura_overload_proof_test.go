package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// API53 deployment and exact named Apex calls are Salesforce-admitted.
// This test preserves the complete minimal source and metadata populations.
func TestSalesforceAdmittedAuraOverloadAPI53(t *testing.T) {
	t.Run("b1-aura-overload-api53", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "force-app/main/default/classes"), 0755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB1AuraOverload.cls"), "public class GladeB1AuraOverload {\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName) {\n        return new FieldInfo[]{};\n    }\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName, Boolean includeReferenceToObjectList) {\n        return new FieldInfo[]{};\n    }\n    public class FieldInfo { @AuraEnabled public String value; }\n}\n")
		writeSemaFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB1AuraOverload.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
		writeSemaFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB1AuraOverloadTest.cls"), "@IsTest private class GladeB1AuraOverloadTest {\n    @IsTest static void apexOverloadCallsAreLegal() {\n        System.assertEquals(0, GladeB1AuraOverload.getObjectFieldDescribes('Account').size());\n        System.assertEquals(0, GladeB1AuraOverload.getObjectFieldDescribes('Account', false).size());\n    }\n}\n")
		writeSemaFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB1AuraOverloadTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
		writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), "{\n  \"sourceApiVersion\": \"53.0\",\n  \"packageDirectories\": [\n    {\n      \"path\": \"force-app\",\n      \"default\": true\n    }\n  ]\n}\n")
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		result := Analyze(typesys.Build(p, schema.Schema{}))
		if result.HasErrors() {
			t.Fatalf("Salesforce-admitted API53 project rejected: %#v", result.Diagnostics)
		}
	})
	t.Run("b1-aura-single-api53", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "force-app/main/default/classes"), 0755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB1AuraSingle.cls"), "public class GladeB1AuraSingle {\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName) {\n        return new FieldInfo[]{};\n    }\n    public class FieldInfo { @AuraEnabled public String value; }\n}\n")
		writeSemaFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB1AuraSingle.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
		writeSemaFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB1AuraSingleTest.cls"), "@IsTest private class GladeB1AuraSingleTest {\n    @IsTest static void apexOverloadCallsAreLegal() {\n        System.assertEquals(0, GladeB1AuraSingle.getObjectFieldDescribes('Account').size());\n    }\n}\n")
		writeSemaFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB1AuraSingleTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
		writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), "{\n  \"sourceApiVersion\": \"53.0\",\n  \"packageDirectories\": [\n    {\n      \"path\": \"force-app\",\n      \"default\": true\n    }\n  ]\n}\n")
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		result := Analyze(typesys.Build(p, schema.Schema{}))
		if result.HasErrors() {
			t.Fatalf("Salesforce-admitted API53 project rejected: %#v", result.Diagnostics)
		}
	})
}

// The complete project sources preserve the API53/54 accepted contracts and
// API55 documented boundary, including opposite project/component versions.
// Salesforce admission of the boundary controls is recorded separately.
func TestAuraOverloadDeclaringComponentVersion(t *testing.T) {
	for _, tc := range []struct {
		name, fixture string
		reject        bool
	}{
		{name: "b1-aura-signatures-api53", reject: false, fixture: `{
  "name": "b1-aura-signatures-api53",
  "apiVersion": "53.0",
  "project": {
    "sourceApiVersion": "53.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures53.cls",
      "content": "public class GladeB1AuraSignatures53 {\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName) {\n        FieldInfo item = new FieldInfo(); item.value = 'one:' + objectName;\n        return new FieldInfo[]{item};\n    }\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName, Boolean includeReferenceToObjectList) {\n        FieldInfo item = new FieldInfo(); item.value = 'two:' + objectName + ':' + String.valueOf(includeReferenceToObjectList);\n        return new FieldInfo[]{item};\n    }\n    public class FieldInfo { @AuraEnabled public String value; }\n    @AuraEnabled public static String find() { return 'all'; }\n    @AuraEnabled public static String find(String query) { return query; }\n    public static Integer invocations = 0;\n    public static String last;\n    @AuraEnabled public static void run() { invocations += 1; last = 'zero'; }\n    @AuraEnabled public static void run(String value) { invocations += 10; last = value; }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures53.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures53Proof.cls",
      "content": "@IsTest private class GladeB1AuraSignatures53Proof {\n    @IsTest static void dtoOverloadsKeepParametersAndSelectedBody() {\n        GladeB1AuraSignatures53.FieldInfo[] first = GladeB1AuraSignatures53.getObjectFieldDescribes('Account');\n        System.assertEquals(1, first.size()); System.assertEquals('one:Account', first[0].value);\n        GladeB1AuraSignatures53.FieldInfo[] second = GladeB1AuraSignatures53.getObjectFieldDescribes('Contact', false);\n        System.assertEquals(1, second.size()); System.assertEquals('two:Contact:false', second[0].value);\n        System.assertEquals('two:Lead:true', GladeB1AuraSignatures53.getObjectFieldDescribes('Lead', true)[0].value);\n        System.assertEquals('one:Opportunity', GladeB1AuraSignatures53.getObjectFieldDescribes('Opportunity')[0].value);\n    }\n    @IsTest static void stringOverloadsKeepOracleBodies() {\n        System.assertEquals('all', GladeB1AuraSignatures53.find());\n        System.assertEquals('owned-query', GladeB1AuraSignatures53.find('owned-query'));\n        System.assertEquals('second-query', GladeB1AuraSignatures53.find('second-query'));\n        System.assertEquals('all', GladeB1AuraSignatures53.find());\n    }\n    @IsTest static void voidOverloadsHaveObservableDistinctState() {\n        System.assertEquals(0, GladeB1AuraSignatures53.invocations);\n        GladeB1AuraSignatures53.run(); System.assertEquals(1, GladeB1AuraSignatures53.invocations); System.assertEquals('zero', GladeB1AuraSignatures53.last);\n        GladeB1AuraSignatures53.run('owned-value'); System.assertEquals(11, GladeB1AuraSignatures53.invocations); System.assertEquals('owned-value', GladeB1AuraSignatures53.last);\n        GladeB1AuraSignatures53.run(); System.assertEquals(12, GladeB1AuraSignatures53.invocations); System.assertEquals('zero', GladeB1AuraSignatures53.last);\n    }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures53Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "test"
  },
  "expected": {
    "result": {
      "ok": true,
      "errors": 0,
      "failed": 0,
      "passed": 3,
      "total": 3
    }
  }
}
`},
		{name: "b1-aura-signatures-api54", reject: false, fixture: `{
  "name": "b1-aura-signatures-api54",
  "apiVersion": "54.0",
  "project": {
    "sourceApiVersion": "54.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures54.cls",
      "content": "public class GladeB1AuraSignatures54 {\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName) {\n        FieldInfo item = new FieldInfo(); item.value = 'one:' + objectName;\n        return new FieldInfo[]{item};\n    }\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName, Boolean includeReferenceToObjectList) {\n        FieldInfo item = new FieldInfo(); item.value = 'two:' + objectName + ':' + String.valueOf(includeReferenceToObjectList);\n        return new FieldInfo[]{item};\n    }\n    public class FieldInfo { @AuraEnabled public String value; }\n    @AuraEnabled public static String find() { return 'all'; }\n    @AuraEnabled public static String find(String query) { return query; }\n    public static Integer invocations = 0;\n    public static String last;\n    @AuraEnabled public static void run() { invocations += 1; last = 'zero'; }\n    @AuraEnabled public static void run(String value) { invocations += 10; last = value; }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures54.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>54.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures54Proof.cls",
      "content": "@IsTest private class GladeB1AuraSignatures54Proof {\n    @IsTest static void dtoOverloadsKeepParametersAndSelectedBody() {\n        GladeB1AuraSignatures54.FieldInfo[] first = GladeB1AuraSignatures54.getObjectFieldDescribes('Account');\n        System.assertEquals(1, first.size()); System.assertEquals('one:Account', first[0].value);\n        GladeB1AuraSignatures54.FieldInfo[] second = GladeB1AuraSignatures54.getObjectFieldDescribes('Contact', false);\n        System.assertEquals(1, second.size()); System.assertEquals('two:Contact:false', second[0].value);\n        System.assertEquals('two:Lead:true', GladeB1AuraSignatures54.getObjectFieldDescribes('Lead', true)[0].value);\n        System.assertEquals('one:Opportunity', GladeB1AuraSignatures54.getObjectFieldDescribes('Opportunity')[0].value);\n    }\n    @IsTest static void stringOverloadsKeepOracleBodies() {\n        System.assertEquals('all', GladeB1AuraSignatures54.find());\n        System.assertEquals('owned-query', GladeB1AuraSignatures54.find('owned-query'));\n        System.assertEquals('second-query', GladeB1AuraSignatures54.find('second-query'));\n        System.assertEquals('all', GladeB1AuraSignatures54.find());\n    }\n    @IsTest static void voidOverloadsHaveObservableDistinctState() {\n        System.assertEquals(0, GladeB1AuraSignatures54.invocations);\n        GladeB1AuraSignatures54.run(); System.assertEquals(1, GladeB1AuraSignatures54.invocations); System.assertEquals('zero', GladeB1AuraSignatures54.last);\n        GladeB1AuraSignatures54.run('owned-value'); System.assertEquals(11, GladeB1AuraSignatures54.invocations); System.assertEquals('owned-value', GladeB1AuraSignatures54.last);\n        GladeB1AuraSignatures54.run(); System.assertEquals(12, GladeB1AuraSignatures54.invocations); System.assertEquals('zero', GladeB1AuraSignatures54.last);\n    }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures54Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>54.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "test"
  },
  "expected": {
    "result": {
      "ok": true,
      "errors": 0,
      "failed": 0,
      "passed": 3,
      "total": 3
    }
  }
}
`},
		{name: "b1-aura-boundary-api55", reject: true, fixture: `{
  "name": "b1-aura-boundary-api55",
  "apiVersion": "55.0",
  "project": {
    "sourceApiVersion": "55.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures55.cls",
      "content": "public class GladeB1AuraSignatures55 {\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName) {\n        FieldInfo item = new FieldInfo(); item.value = 'one:' + objectName;\n        return new FieldInfo[]{item};\n    }\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName, Boolean includeReferenceToObjectList) {\n        FieldInfo item = new FieldInfo(); item.value = 'two:' + objectName + ':' + String.valueOf(includeReferenceToObjectList);\n        return new FieldInfo[]{item};\n    }\n    public class FieldInfo { @AuraEnabled public String value; }\n    @AuraEnabled public static String find() { return 'all'; }\n    @AuraEnabled public static String find(String query) { return query; }\n    public static Integer invocations = 0;\n    public static String last;\n    @AuraEnabled public static void run() { invocations += 1; last = 'zero'; }\n    @AuraEnabled public static void run(String value) { invocations += 10; last = value; }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures55.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures55Proof.cls",
      "content": "@IsTest private class GladeB1AuraSignatures55Proof {\n    @IsTest static void dtoOverloadsKeepParametersAndSelectedBody() {\n        GladeB1AuraSignatures55.FieldInfo[] first = GladeB1AuraSignatures55.getObjectFieldDescribes('Account');\n        System.assertEquals(1, first.size()); System.assertEquals('one:Account', first[0].value);\n        GladeB1AuraSignatures55.FieldInfo[] second = GladeB1AuraSignatures55.getObjectFieldDescribes('Contact', false);\n        System.assertEquals(1, second.size()); System.assertEquals('two:Contact:false', second[0].value);\n        System.assertEquals('two:Lead:true', GladeB1AuraSignatures55.getObjectFieldDescribes('Lead', true)[0].value);\n        System.assertEquals('one:Opportunity', GladeB1AuraSignatures55.getObjectFieldDescribes('Opportunity')[0].value);\n    }\n    @IsTest static void stringOverloadsKeepOracleBodies() {\n        System.assertEquals('all', GladeB1AuraSignatures55.find());\n        System.assertEquals('owned-query', GladeB1AuraSignatures55.find('owned-query'));\n        System.assertEquals('second-query', GladeB1AuraSignatures55.find('second-query'));\n        System.assertEquals('all', GladeB1AuraSignatures55.find());\n    }\n    @IsTest static void voidOverloadsHaveObservableDistinctState() {\n        System.assertEquals(0, GladeB1AuraSignatures55.invocations);\n        GladeB1AuraSignatures55.run(); System.assertEquals(1, GladeB1AuraSignatures55.invocations); System.assertEquals('zero', GladeB1AuraSignatures55.last);\n        GladeB1AuraSignatures55.run('owned-value'); System.assertEquals(11, GladeB1AuraSignatures55.invocations); System.assertEquals('owned-value', GladeB1AuraSignatures55.last);\n        GladeB1AuraSignatures55.run(); System.assertEquals(12, GladeB1AuraSignatures55.invocations); System.assertEquals('zero', GladeB1AuraSignatures55.last);\n    }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures55Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "test"
  },
  "expected": {
    "result": {
      "ok": true,
      "errors": 0,
      "failed": 0,
      "passed": 3,
      "total": 3
    }
  }
}
`},
		{name: "b1-aura-boundary-api67", reject: true, fixture: `{
  "name": "b1-aura-signatures-api67",
  "apiVersion": "67.0",
  "project": {
    "sourceApiVersion": "67.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures67.cls",
      "content": "public class GladeB1AuraSignatures67 {\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName) {\n        FieldInfo item = new FieldInfo(); item.value = 'one:' + objectName;\n        return new FieldInfo[]{item};\n    }\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName, Boolean includeReferenceToObjectList) {\n        FieldInfo item = new FieldInfo(); item.value = 'two:' + objectName + ':' + String.valueOf(includeReferenceToObjectList);\n        return new FieldInfo[]{item};\n    }\n    public class FieldInfo { @AuraEnabled public String value; }\n    @AuraEnabled public static String find() { return 'all'; }\n    @AuraEnabled public static String find(String query) { return query; }\n    public static Integer invocations = 0;\n    public static String last;\n    @AuraEnabled public static void run() { invocations += 1; last = 'zero'; }\n    @AuraEnabled public static void run(String value) { invocations += 10; last = value; }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures67.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures67Proof.cls",
      "content": "@IsTest private class GladeB1AuraSignatures67Proof {\n    @IsTest static void dtoOverloadsKeepParametersAndSelectedBody() {\n        GladeB1AuraSignatures67.FieldInfo[] first = GladeB1AuraSignatures67.getObjectFieldDescribes('Account');\n        System.assertEquals(1, first.size()); System.assertEquals('one:Account', first[0].value);\n        GladeB1AuraSignatures67.FieldInfo[] second = GladeB1AuraSignatures67.getObjectFieldDescribes('Contact', false);\n        System.assertEquals(1, second.size()); System.assertEquals('two:Contact:false', second[0].value);\n        System.assertEquals('two:Lead:true', GladeB1AuraSignatures67.getObjectFieldDescribes('Lead', true)[0].value);\n        System.assertEquals('one:Opportunity', GladeB1AuraSignatures67.getObjectFieldDescribes('Opportunity')[0].value);\n    }\n    @IsTest static void stringOverloadsKeepOracleBodies() {\n        System.assertEquals('all', GladeB1AuraSignatures67.find());\n        System.assertEquals('owned-query', GladeB1AuraSignatures67.find('owned-query'));\n        System.assertEquals('second-query', GladeB1AuraSignatures67.find('second-query'));\n        System.assertEquals('all', GladeB1AuraSignatures67.find());\n    }\n    @IsTest static void voidOverloadsHaveObservableDistinctState() {\n        System.assertEquals(0, GladeB1AuraSignatures67.invocations);\n        GladeB1AuraSignatures67.run(); System.assertEquals(1, GladeB1AuraSignatures67.invocations); System.assertEquals('zero', GladeB1AuraSignatures67.last);\n        GladeB1AuraSignatures67.run('owned-value'); System.assertEquals(11, GladeB1AuraSignatures67.invocations); System.assertEquals('owned-value', GladeB1AuraSignatures67.last);\n        GladeB1AuraSignatures67.run(); System.assertEquals(12, GladeB1AuraSignatures67.invocations); System.assertEquals('zero', GladeB1AuraSignatures67.last);\n    }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraSignatures67Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "test"
  },
  "expected": {
    "result": {
      "ok": true,
      "errors": 0,
      "failed": 0,
      "passed": 3,
      "total": 3
    }
  }
}
`},
		{name: "b1-aura-helper54-project67", reject: false, fixture: `{
  "name": "b1-aura-helper54-project67",
  "apiVersion": "67.0",
  "project": {
    "sourceApiVersion": "67.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeB1AuraMixed54P67.cls",
      "content": "public class GladeB1AuraMixed54P67 {\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName) {\n        FieldInfo item = new FieldInfo(); item.value = 'one:' + objectName;\n        return new FieldInfo[]{item};\n    }\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName, Boolean includeReferenceToObjectList) {\n        FieldInfo item = new FieldInfo(); item.value = 'two:' + objectName + ':' + String.valueOf(includeReferenceToObjectList);\n        return new FieldInfo[]{item};\n    }\n    public class FieldInfo { @AuraEnabled public String value; }\n    @AuraEnabled public static String find() { return 'all'; }\n    @AuraEnabled public static String find(String query) { return query; }\n    public static Integer invocations = 0;\n    public static String last;\n    @AuraEnabled public static void run() { invocations += 1; last = 'zero'; }\n    @AuraEnabled public static void run(String value) { invocations += 10; last = value; }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraMixed54P67.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>54.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraMixed54P67Proof.cls",
      "content": "@IsTest private class GladeB1AuraMixed54P67Proof {\n    @IsTest static void dtoOverloadsKeepParametersAndSelectedBody() {\n        GladeB1AuraMixed54P67.FieldInfo[] first = GladeB1AuraMixed54P67.getObjectFieldDescribes('Account');\n        System.assertEquals(1, first.size()); System.assertEquals('one:Account', first[0].value);\n        GladeB1AuraMixed54P67.FieldInfo[] second = GladeB1AuraMixed54P67.getObjectFieldDescribes('Contact', false);\n        System.assertEquals(1, second.size()); System.assertEquals('two:Contact:false', second[0].value);\n        System.assertEquals('two:Lead:true', GladeB1AuraMixed54P67.getObjectFieldDescribes('Lead', true)[0].value);\n        System.assertEquals('one:Opportunity', GladeB1AuraMixed54P67.getObjectFieldDescribes('Opportunity')[0].value);\n    }\n    @IsTest static void stringOverloadsKeepOracleBodies() {\n        System.assertEquals('all', GladeB1AuraMixed54P67.find());\n        System.assertEquals('owned-query', GladeB1AuraMixed54P67.find('owned-query'));\n        System.assertEquals('second-query', GladeB1AuraMixed54P67.find('second-query'));\n        System.assertEquals('all', GladeB1AuraMixed54P67.find());\n    }\n    @IsTest static void voidOverloadsHaveObservableDistinctState() {\n        System.assertEquals(0, GladeB1AuraMixed54P67.invocations);\n        GladeB1AuraMixed54P67.run(); System.assertEquals(1, GladeB1AuraMixed54P67.invocations); System.assertEquals('zero', GladeB1AuraMixed54P67.last);\n        GladeB1AuraMixed54P67.run('owned-value'); System.assertEquals(11, GladeB1AuraMixed54P67.invocations); System.assertEquals('owned-value', GladeB1AuraMixed54P67.last);\n        GladeB1AuraMixed54P67.run(); System.assertEquals(12, GladeB1AuraMixed54P67.invocations); System.assertEquals('zero', GladeB1AuraMixed54P67.last);\n    }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraMixed54P67Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "test"
  },
  "expected": {
    "result": {
      "ok": true,
      "errors": 0,
      "failed": 0,
      "passed": 3,
      "total": 3
    }
  }
}
`},
		{name: "b1-aura-helper55-project54", reject: true, fixture: `{
  "name": "b1-aura-helper55-project54",
  "apiVersion": "54.0",
  "project": {
    "sourceApiVersion": "54.0",
    "packageDirectories": [
      {
        "path": "force-app",
        "default": true
      }
    ]
  },
  "source": [
    {
      "path": "force-app/main/default/classes/GladeB1AuraMixed55P54.cls",
      "content": "public class GladeB1AuraMixed55P54 {\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName) {\n        FieldInfo item = new FieldInfo(); item.value = 'one:' + objectName;\n        return new FieldInfo[]{item};\n    }\n    @AuraEnabled public static FieldInfo[] getObjectFieldDescribes(String objectName, Boolean includeReferenceToObjectList) {\n        FieldInfo item = new FieldInfo(); item.value = 'two:' + objectName + ':' + String.valueOf(includeReferenceToObjectList);\n        return new FieldInfo[]{item};\n    }\n    public class FieldInfo { @AuraEnabled public String value; }\n    @AuraEnabled public static String find() { return 'all'; }\n    @AuraEnabled public static String find(String query) { return query; }\n    public static Integer invocations = 0;\n    public static String last;\n    @AuraEnabled public static void run() { invocations += 1; last = 'zero'; }\n    @AuraEnabled public static void run(String value) { invocations += 10; last = value; }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraMixed55P54.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>55.0</apiVersion><status>Active</status></ApexClass>\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraMixed55P54Proof.cls",
      "content": "@IsTest private class GladeB1AuraMixed55P54Proof {\n    @IsTest static void dtoOverloadsKeepParametersAndSelectedBody() {\n        GladeB1AuraMixed55P54.FieldInfo[] first = GladeB1AuraMixed55P54.getObjectFieldDescribes('Account');\n        System.assertEquals(1, first.size()); System.assertEquals('one:Account', first[0].value);\n        GladeB1AuraMixed55P54.FieldInfo[] second = GladeB1AuraMixed55P54.getObjectFieldDescribes('Contact', false);\n        System.assertEquals(1, second.size()); System.assertEquals('two:Contact:false', second[0].value);\n        System.assertEquals('two:Lead:true', GladeB1AuraMixed55P54.getObjectFieldDescribes('Lead', true)[0].value);\n        System.assertEquals('one:Opportunity', GladeB1AuraMixed55P54.getObjectFieldDescribes('Opportunity')[0].value);\n    }\n    @IsTest static void stringOverloadsKeepOracleBodies() {\n        System.assertEquals('all', GladeB1AuraMixed55P54.find());\n        System.assertEquals('owned-query', GladeB1AuraMixed55P54.find('owned-query'));\n        System.assertEquals('second-query', GladeB1AuraMixed55P54.find('second-query'));\n        System.assertEquals('all', GladeB1AuraMixed55P54.find());\n    }\n    @IsTest static void voidOverloadsHaveObservableDistinctState() {\n        System.assertEquals(0, GladeB1AuraMixed55P54.invocations);\n        GladeB1AuraMixed55P54.run(); System.assertEquals(1, GladeB1AuraMixed55P54.invocations); System.assertEquals('zero', GladeB1AuraMixed55P54.last);\n        GladeB1AuraMixed55P54.run('owned-value'); System.assertEquals(11, GladeB1AuraMixed55P54.invocations); System.assertEquals('owned-value', GladeB1AuraMixed55P54.last);\n        GladeB1AuraMixed55P54.run(); System.assertEquals(12, GladeB1AuraMixed55P54.invocations); System.assertEquals('zero', GladeB1AuraMixed55P54.last);\n    }\n}\n"
    },
    {
      "path": "force-app/main/default/classes/GladeB1AuraMixed55P54Proof.cls-meta.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>54.0</apiVersion><status>Active</status></ApexClass>\n"
    }
  ],
  "command": {
    "kind": "test"
  },
  "expected": {
    "result": {
      "ok": true,
      "errors": 0,
      "failed": 0,
      "passed": 3,
      "total": 3
    }
  }
}
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fixture struct {
				Source  []struct{ Path, Content string }
				Project json.RawMessage
			}
			if err := json.Unmarshal([]byte(tc.fixture), &fixture); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			for _, source := range fixture.Source {
				target := filepath.Join(root, filepath.FromSlash(source.Path))
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					t.Fatal(err)
				}
				writeSemaFile(t, target, source.Content)
			}
			writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), string(fixture.Project))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(typesys.Build(p, schema.Schema{}))
			overloads := 0
			for _, d := range result.Diagnostics {
				if d.Code == "GLADESEMA032" && strings.HasPrefix(d.Message, "AuraEnabled methods cannot be overloaded:") {
					overloads++
				}
			}
			if tc.reject {
				if overloads != 3 {
					t.Fatalf("expected three version-gated overload errors, got %d: %#v", overloads, result.Diagnostics)
				}
			} else if result.HasErrors() {
				t.Fatalf("admitted declaring component version rejected: %#v", result.Diagnostics)
			}
		})
	}
}
