package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB4SwitchBindTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4SwitchBindTest.cls"), "@IsTest private class GladeB4SwitchBindTest {\n    @IsTest static void outerMapRemainsVisibleInSwitchBranch() {\n        Account chosen = new Account(Name='CHOSEN'); Account excluded = new Account(Name='EXCLUDED');\n        insert new List<Account>{chosen, excluded};\n        Map<String,Set<Id>> mapIDsByObject = new Map<String,Set<Id>>{'account'=>new Set<Id>{chosen.Id}};\n        String objectName = 'account';\n        switch on objectName {\n            when 'account' {\n                List<Account> found = [SELECT Id, Name FROM Account WHERE Id IN :mapIDsByObject.get('account')];\n                System.assertEquals(1, found.size()); System.assertEquals(chosen.Id, found[0].Id);\n                System.assertEquals('CHOSEN', found[0].Name);\n            }\n            when else { System.assert(false, 'Expected account branch'); }\n        }\n    }\n    @IsTest static void branchLocalMapBindsWithinItsScope() {\n        Account chosen = new Account(Name='LOCAL'); insert chosen;\n        String objectName = 'account';\n        switch on objectName {\n            when 'account' {\n                Map<String,Set<Id>> branchIds = new Map<String,Set<Id>>{'account'=>new Set<Id>{chosen.Id}};\n                List<Account> found = [SELECT Id, Name FROM Account WHERE Id IN :branchIds.get('account')];\n                System.assertEquals(1, found.size()); System.assertEquals(chosen.Id, found[0].Id);\n            }\n            when else { System.assert(false, 'Expected account branch'); }\n        }\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4SwitchBindTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b4-switch-map-bind-api65\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"65.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB4StringSelectTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4StringSelectTest.cls"), "@IsTest private class GladeB4StringSelectTest {\n    private static Account lookup(String developerName) {\n        Boolean cached = false;\n        if (cached) { return null; }\n        else {\n            return [SELECT Id, Name FROM Account WHERE Name = :developerName WITH USER_MODE LIMIT 1];\n        }\n    }\n    @IsTest static void stringParameterRetainsTypeAfterSelect() {\n        Account chosen = new Account(Name='BIND_TARGET'); insert new List<Account>{chosen,new Account(Name='OTHER')};\n        String developerName = 'BIND_TARGET'; Account found = lookup(developerName);\n        System.assertEquals(chosen.Id, found.Id); System.assertEquals(developerName, found.Name);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4StringSelectTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b4-string-parameter-select-bind-api66\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"66.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB4StringWhereTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4StringWhereTest.cls"), "@IsTest private class GladeB4StringWhereTest {\n    private static List<Task> lookup(String subject) {\n        return [SELECT Id, Subject FROM Task WHERE Subject = :subject AND CreatedDate = TODAY ORDER BY CreatedDate DESC LIMIT 1];\n    }\n    @IsTest static void stringParameterRetainsTypeAfterWhere() {\n        Task chosen = new Task(Subject='TARGET_SUBJECT',Status='Not Started',Priority='Normal');\n        insert new List<Task>{chosen,new Task(Subject='OTHER',Status='Not Started',Priority='Normal')};\n        List<Task> found = lookup('TARGET_SUBJECT');\n        System.assertEquals(1, found.size()); System.assertEquals(chosen.Id, found[0].Id);\n        System.assertEquals('TARGET_SUBJECT', found[0].Subject); System.assertEquals(0,lookup('ABSENT').size());\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4StringWhereTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b4-string-parameter-where-bind-api63\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"63.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB4CastBindTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4CastBindTest.cls"), "@IsTest private class GladeB4CastBindTest {\n    @IsTest static void castSObjectGetProducesIdBind() {\n        Account chosen = new Account(Name='CAST_TARGET'); insert new List<Account>{chosen,new Account(Name='OTHER')};\n        SObject t = chosen;\n        Account found = [SELECT Id, Name FROM Account WHERE Id = :(Id) t.get('Id')];\n        System.assertEquals(chosen.Id, found.Id); System.assertEquals('CAST_TARGET',found.Name);\n        Id typedResult = found.Id; System.assertEquals(chosen.Id,typedResult);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4CastBindTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b4-cast-expression-bind-api65\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"65.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB4CompactInTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4CompactInTest.cls"), "@IsTest private class GladeB4CompactInTest {\n    @IsTest static void compactInColonSelectsOnlyBoundNames() {\n        insert new List<Account>{new Account(Name='RED'),new Account(Name='BLUE'),new Account(Name='GREEN')};\n        Set<String> names = new Set<String>{'RED','BLUE'};\n        List<Account> found = [SELECT Id, Name FROM Account WHERE Name in: names ORDER BY Name];\n        System.assertEquals(2,found.size()); System.assertEquals('BLUE',found[0].Name); System.assertEquals('RED',found[1].Name);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4CompactInTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>51.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b4-whitespace-free-in-bind-api51\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"52.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB4ScalarCountTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4ScalarCountTest.cls"), "@IsTest private class GladeB4ScalarCountTest {\n    @IsTest static void bareCountAssignsIntegerAndUpdatesAfterDml() {\n        Account chosen = new Account(Name='COUNT_TARGET'); insert chosen;\n        Integer before = [SELECT COUNT() FROM Contact WHERE AccountId = :chosen.Id]; System.assertEquals(0,before);\n        insert new List<Contact>{new Contact(LastName='One',AccountId=chosen.Id),new Contact(LastName='Two',AccountId=chosen.Id)};\n        Integer after = [SELECT COUNT() FROM Contact WHERE AccountId = :chosen.Id]; System.assertEquals(2,after);\n        System.assertEquals(3,after+1);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4ScalarCountTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>51.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b4-scalar-count-result-api51\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"52.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB4GroupedCountTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4GroupedCountTest.cls"), "@IsTest private class GladeB4GroupedCountTest {\n    @IsTest static void groupedCountReturnsTypedAggregateRows() {\n        Account first = new Account(Name='FIRST'); Account second = new Account(Name='SECOND'); insert new List<Account>{first,second};\n        insert new List<Contact>{new Contact(LastName='One',AccountId=first.Id),new Contact(LastName='Two',AccountId=first.Id),new Contact(LastName='Three',AccountId=second.Id)};\n        Set<Id> accountIds = new Set<Id>{first.Id,second.Id};\n        AggregateResult[] groups = [SELECT AccountId, Count(Id) FROM Contact WHERE AccountId = :accountIds GROUP BY AccountId];\n        System.assertEquals(2,groups.size()); Map<Id,Integer> counts = new Map<Id,Integer>();\n        for (AggregateResult groupRow : groups) { Id accountId=(Id)groupRow.get('AccountId'); Integer count=(Integer)groupRow.get('expr0'); counts.put(accountId,count); }\n        System.assertEquals(2,counts.get(first.Id)); System.assertEquals(1,counts.get(second.Id));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4GroupedCountTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>51.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b4-grouped-count-result-api51\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"52.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB4GroupedInlineTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4GroupedInlineTest.cls"), "@IsTest private class GladeB4GroupedInlineTest {\n    @IsTest static void groupedCountReturnsTypedAggregateRows() {\n        Account first = new Account(Name='FIRST'); Account second = new Account(Name='SECOND'); insert new List<Account>{first,second};\n        insert new List<Contact>{new Contact(LastName='One',AccountId=first.Id),new Contact(LastName='Two',AccountId=first.Id),new Contact(LastName='Three',AccountId=second.Id)};\n        Set<Id> accountIds = new Set<Id>{first.Id,second.Id};\n        AggregateResult[] groups = [SELECT AccountId, Count(Id) FROM Contact WHERE AccountId = :accountIds GROUP BY AccountId];\n        System.assertEquals(2,groups.size()); Map<Id,Integer> counts = new Map<Id,Integer>();\n        for (AggregateResult groupRow : groups) { counts.put((Id)groupRow.get('AccountId'), (Integer)groupRow.get('expr0')); }\n        System.assertEquals(2,counts.get(first.Id)); System.assertEquals(1,counts.get(second.Id));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB4GroupedInlineTest.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>51.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b4-grouped-count-inline-casts-api51\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"52.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}
