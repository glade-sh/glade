package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestRunTimeFieldConstructorAPI64(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion": "64.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB3Time64Test.cls"), `@IsTest
private class GladeB3Time64Test {
    @IsTest static void scalarDateAndTime() {
        Datetime result = Datetime.newInstanceGmt(Date.newInstance(2026, 9, 5), Time.newInstance(8, 0, 0, 0));
        Assert.areEqual(8, result.hourGmt());
    }
    @IsTest static void assignedFieldDirect() {
        ClockProbe__c row = new ClockProbe__c(Name='probe');
        row.Day__c = Date.newInstance(2026, 9, 5);
        row.Clock__c = Time.newInstance(8, 0, 0, 0);
        Assert.isNotNull(row.Day__c);
        Assert.isNotNull(row.Clock__c);
        Datetime result = Datetime.newInstanceGmt(row.Day__c, row.Clock__c);
        Assert.areEqual(8, result.hourGmt());
    }
    @IsTest static void queriedFieldDirect() {
        ClockProbe__c row = new ClockProbe__c(Name='probe');
        row.Day__c = Date.newInstance(2026, 9, 5);
        row.Clock__c = Time.newInstance(8, 0, 0, 0);
        insert row;
        ClockProbe__c loaded = [SELECT Day__c, Clock__c FROM ClockProbe__c WHERE Id = :row.Id];
        Assert.isNotNull(loaded.Day__c);
        Assert.isNotNull(loaded.Clock__c);
        Datetime result = Datetime.newInstanceGmt(loaded.Day__c, loaded.Clock__c);
        Assert.areEqual(8, result.hourGmt());
    }
    @IsTest static void queriedFieldExplicitTimeLocal() {
        ClockProbe__c row = new ClockProbe__c(Name='probe');
        row.Day__c = Date.newInstance(2026, 9, 5);
        row.Clock__c = Time.newInstance(8, 0, 0, 0);
        insert row;
        ClockProbe__c loaded = [SELECT Day__c, Clock__c FROM ClockProbe__c WHERE Id = :row.Id];
        Time typedTime = loaded.Clock__c;
        Datetime result = Datetime.newInstanceGmt(loaded.Day__c, typedTime);
        Assert.areEqual(8, result.hourGmt());
    }
    private static User gmtUser() {
        return new User(Alias='gldtime', Email='glade-time@example.invalid',
            EmailEncodingKey='UTF-8', LastName='Glade Time Proof', LanguageLocaleKey='en_US',
            LocaleSidKey='en_US', ProfileId=UserInfo.getProfileId(), TimeZoneSidKey='GMT',
            Username='glade-time-'+UserInfo.getOrganizationId()+'@example.invalid');
    }
    @IsTest static void localConstructorPreservesFieldTypes() {
        System.runAs(gmtUser()) {
            Date day = Date.newInstance(2026, 9, 5); Time clock = Time.newInstance(8, 9, 10, 123);
            Datetime expected = Datetime.newInstanceGmt(day, clock);
            System.assertEquals(expected, Datetime.newInstance(day, clock));
            ClockProbe__c row = new ClockProbe__c(Name='local', Day__c=day, Clock__c=clock);
            System.assertEquals(expected, Datetime.newInstance(row.Day__c, row.Clock__c));
            insert row;
            ClockProbe__c loaded = [SELECT Day__c, Clock__c FROM ClockProbe__c WHERE Id=:row.Id];
            System.assertEquals(expected, Datetime.newInstance(loaded.Day__c, loaded.Clock__c));
            Time typedTime = loaded.Clock__c;
            System.assertEquals(expected, Datetime.newInstance(loaded.Day__c, typedTime));
        }
    }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB3Time64Test.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>64.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ClockProbe__c/ClockProbe__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Clock Probe</label><pluralLabel>Clock Probes</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ClockProbe__c/fields/Clock__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Clock__c</fullName><label>Clock__c</label><type>Time</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ClockProbe__c/fields/Day__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Day__c</fullName><label>Day__c</label><type>Date</type></CustomField>`)
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 5 || got.Passed != 5 {
		data, _ := json.Marshal(run)
		t.Fatalf("Time packet: %s", data)
	}
}
func TestRunTimeFieldConstructorAPI66(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion": "66.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB3Time66Test.cls"), `@IsTest
private class GladeB3Time66Test {
    @IsTest static void scalarDateAndTime() {
        Datetime result = Datetime.newInstanceGmt(Date.newInstance(2026, 9, 5), Time.newInstance(8, 0, 0, 0));
        Assert.areEqual(8, result.hourGmt());
    }
    @IsTest static void assignedFieldDirect() {
        ClockProbe__c row = new ClockProbe__c(Name='probe');
        row.Day__c = Date.newInstance(2026, 9, 5);
        row.Clock__c = Time.newInstance(8, 0, 0, 0);
        Assert.isNotNull(row.Day__c);
        Assert.isNotNull(row.Clock__c);
        Datetime result = Datetime.newInstanceGmt(row.Day__c, row.Clock__c);
        Assert.areEqual(8, result.hourGmt());
    }
    @IsTest static void queriedFieldDirect() {
        ClockProbe__c row = new ClockProbe__c(Name='probe');
        row.Day__c = Date.newInstance(2026, 9, 5);
        row.Clock__c = Time.newInstance(8, 0, 0, 0);
        insert row;
        ClockProbe__c loaded = [SELECT Day__c, Clock__c FROM ClockProbe__c WHERE Id = :row.Id];
        Assert.isNotNull(loaded.Day__c);
        Assert.isNotNull(loaded.Clock__c);
        Datetime result = Datetime.newInstanceGmt(loaded.Day__c, loaded.Clock__c);
        Assert.areEqual(8, result.hourGmt());
    }
    @IsTest static void queriedFieldExplicitTimeLocal() {
        ClockProbe__c row = new ClockProbe__c(Name='probe');
        row.Day__c = Date.newInstance(2026, 9, 5);
        row.Clock__c = Time.newInstance(8, 0, 0, 0);
        insert row;
        ClockProbe__c loaded = [SELECT Day__c, Clock__c FROM ClockProbe__c WHERE Id = :row.Id];
        Time typedTime = loaded.Clock__c;
        Datetime result = Datetime.newInstanceGmt(loaded.Day__c, typedTime);
        Assert.areEqual(8, result.hourGmt());
    }
    private static User gmtUser() {
        return new User(Alias='gldtime', Email='glade-time@example.invalid',
            EmailEncodingKey='UTF-8', LastName='Glade Time Proof', LanguageLocaleKey='en_US',
            LocaleSidKey='en_US', ProfileId=UserInfo.getProfileId(), TimeZoneSidKey='GMT',
            Username='glade-time-'+UserInfo.getOrganizationId()+'@example.invalid');
    }
    @IsTest static void localConstructorPreservesFieldTypes() {
        System.runAs(gmtUser()) {
            Date day = Date.newInstance(2026, 9, 5); Time clock = Time.newInstance(8, 9, 10, 123);
            Datetime expected = Datetime.newInstanceGmt(day, clock);
            System.assertEquals(expected, Datetime.newInstance(day, clock));
            ClockProbe__c row = new ClockProbe__c(Name='local', Day__c=day, Clock__c=clock);
            System.assertEquals(expected, Datetime.newInstance(row.Day__c, row.Clock__c));
            insert row;
            ClockProbe__c loaded = [SELECT Day__c, Clock__c FROM ClockProbe__c WHERE Id=:row.Id];
            System.assertEquals(expected, Datetime.newInstance(loaded.Day__c, loaded.Clock__c));
            Time typedTime = loaded.Clock__c;
            System.assertEquals(expected, Datetime.newInstance(loaded.Day__c, typedTime));
        }
    }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB3Time66Test.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ClockProbe__c/ClockProbe__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Clock Probe</label><pluralLabel>Clock Probes</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ClockProbe__c/fields/Clock__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Clock__c</fullName><label>Clock__c</label><type>Time</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ClockProbe__c/fields/Day__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Day__c</fullName><label>Day__c</label><type>Date</type></CustomField>`)
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 5 || got.Passed != 5 {
		data, _ := json.Marshal(run)
		t.Fatalf("Time packet: %s", data)
	}
}
