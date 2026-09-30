package vm_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Exact SF139 accepted API42/37 assertions, project43.
func TestBlankSObjectAmountAndNullListNameProof(t *testing.T) {
	t.Run("blank-opportunity-amount-api42-assertions", func(t *testing.T) {
		root := t.TempDir()
		files := map[string]string{
			"force-app/main/default/classes/GladeBlankAmount42Proof.cls":          "@IsTest private class GladeBlankAmount42Proof {\n @IsTest static void blankStringZero(){Opportunity row=(Opportunity)JSON.deserialize('{\"Amount\":\"\"}',Opportunity.class);System.assertEquals(0,row.Amount);Decimal total=0;total+=row.Amount;System.assertEquals(0,total);}\n @IsTest static void numericZero(){Opportunity row=(Opportunity)JSON.deserialize('{\"Amount\":0}',Opportunity.class);System.assertEquals(0,row.Amount);}\n @IsTest static void numericString(){Opportunity row=(Opportunity)JSON.deserialize('{\"Amount\":\"12.5\"}',Opportunity.class);System.assertEquals(12.5,row.Amount);}\n @IsTest static void explicitNull(){Opportunity row=(Opportunity)JSON.deserialize('{\"Amount\":null}',Opportunity.class);System.assertEquals(null,row.Amount);}\n @IsTest static void missingAmount(){Opportunity row=(Opportunity)JSON.deserialize('{}',Opportunity.class);System.assertEquals(null,row.Amount);}\n}\n",
			"force-app/main/default/classes/GladeBlankAmount42Proof.cls-meta.xml": "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>42.0</apiVersion><status>Active</status></ApexClass>\n",
			"sfdx-project.json": "{\"sourceApiVersion\": \"43.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}",
		}
		for name, content := range files {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		s, err := schema.LoadProject(p)
		if err != nil {
			t.Fatal(err)
		}
		run := apextest.Run(typesys.Build(p, s), apextest.Options{})
		if got := run.Summary(); got.Total != 5 || got.Passed != 5 {
			b, _ := json.Marshal(run)
			t.Fatalf("proof: %s", b)
		}
	})
	t.Run("list-setting-null-name-api37-one-row-assertion", func(t *testing.T) {
		root := t.TempDir()
		files := map[string]string{
			"force-app/main/default/classes/GladeNullList37Proof.cls":                            "@IsTest private class GladeNullList37Proof {\n @IsTest static void oneNamedRow(){GladeListProbe__c inserted=new GladeListProbe__c(Name='MyDownloads',Source__c='MyDownloadsDataSource');insert inserted;String missing;GladeListProbe__c row=GladeListProbe__c.getInstance(missing);System.assertNotEquals(null,row);System.assertEquals(inserted.Id,row.Id);System.assertEquals('MyDownloads',row.Name);System.assertEquals('MyDownloadsDataSource',row.Source__c);}\n}\n",
			"force-app/main/default/classes/GladeNullList37Proof.cls-meta.xml":                   "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>37.0</apiVersion><status>Active</status></ApexClass>\n",
			"force-app/main/default/objects/GladeListProbe__c/GladeListProbe__c.object-meta.xml": "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><customSettingsType>List</customSettingsType><label>Glade List Probe</label><visibility>Public</visibility></CustomObject>\n",
			"force-app/main/default/objects/GladeListProbe__c/fields/Source__c.field-meta.xml":   "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Source__c</fullName><label>Source</label><length>80</length><type>Text</type></CustomField>\n",
			"sfdx-project.json": "{\"sourceApiVersion\": \"43.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}",
		}
		for name, content := range files {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		s, err := schema.LoadProject(p)
		if err != nil {
			t.Fatal(err)
		}
		run := apextest.Run(typesys.Build(p, s), apextest.Options{})
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			b, _ := json.Marshal(run)
			t.Fatalf("proof: %s", b)
		}
	})
}

// SF142 accepted API37 assertions and observed backdated-second contract.
// Deleted-row and getValues controls are local regression coverage.
func TestListSettingNullNameInsertionOrderAPI37(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"force-app/main/default/classes/GladeNullList37OrderProof.cls":                       "@IsTest private class GladeNullList37OrderProof {\nprivate static void assertFirst(String name,String source){String key;GladeListProbe__c row=GladeListProbe__c.getInstance(key);System.assertNotEquals(null,row);System.assertEquals(name,row.Name);System.assertEquals(source,row.Source__c);}\n@IsTest static void reverseInsert(){insert new List<GladeListProbe__c>{new GladeListProbe__c(Name='Beta',Source__c='B'),new GladeListProbe__c(Name='Alpha',Source__c='A')};assertFirst('Beta','B');}\n@IsTest static void separateReverseInsert(){insert new GladeListProbe__c(Name='Zulu',Source__c='Z');insert new GladeListProbe__c(Name='Alpha',Source__c='A');assertFirst('Zulu','Z');}\n@IsTest static void backdatedSecond(){GladeListProbe__c first=new GladeListProbe__c(Name='Beta',Source__c='B');insert first;GladeListProbe__c second=new GladeListProbe__c(Name='Alpha',Source__c='A');insert second;Test.setCreatedDate(second.Id,Datetime.newInstance(2000,1,1,0,0,0));assertFirst('Beta','B');}\n@IsTest static void deletedFirst(){GladeListProbe__c first=new GladeListProbe__c(Name='Beta',Source__c='B');insert first;insert new GladeListProbe__c(Name='Alpha',Source__c='A');delete first;assertFirst('Alpha','A');}\n@IsTest static void nullGetValues(){insert new GladeListProbe__c(Name='Beta',Source__c='B');String key;GladeListProbe__c row=GladeListProbe__c.getValues(key);System.assertEquals(null,row.Id);System.assertEquals(null,row.Source__c);}\n@IsTest static void empty(){String key;System.assertEquals(null,GladeListProbe__c.getInstance(key));}\n@IsTest static void namedMissing(){insert new GladeListProbe__c(Name='Alpha',Source__c='A');System.assertEquals(null,GladeListProbe__c.getInstance('Missing'));}\n}",
		"force-app/main/default/classes/GladeNullList37OrderProof.cls-meta.xml":              "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>37.0</apiVersion><status>Active</status></ApexClass>\n",
		"force-app/main/default/objects/GladeListProbe__c/GladeListProbe__c.object-meta.xml": "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><customSettingsType>List</customSettingsType><label>Glade List Probe</label><visibility>Public</visibility></CustomObject>\n",
		"force-app/main/default/objects/GladeListProbe__c/fields/Source__c.field-meta.xml":   "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Source__c</fullName><label>Source</label><length>80</length><type>Text</type></CustomField>\n",
		"sfdx-project.json": "{\"sourceApiVersion\": \"43.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	run := apextest.Run(typesys.Build(p, s), apextest.Options{})
	if got := run.Summary(); got.Total != 7 || got.Passed != 7 {
		b, _ := json.Marshal(run)
		t.Fatalf("proof: %s", b)
	}
}
