package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestRunMetadataChildIteratorAcrossCachedMap(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json":                                   `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"objects/Route__mdt/Route__mdt.object-meta.xml":       `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Route</label><visibility>Public</visibility></CustomObject>`,
		"objects/Step__mdt/Step__mdt.object-meta.xml":         `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Step</label><visibility>Public</visibility></CustomObject>`,
		"objects/Route__mdt/fields/Enabled__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Enabled__c</fullName><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`,
		"objects/Step__mdt/fields/Enabled__c.field-meta.xml":  `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Enabled__c</fullName><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`,
		"objects/Step__mdt/fields/Route__c.field-meta.xml":    `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Route__c</fullName><type>MetadataRelationship</type><referenceTo>Route__mdt</referenceTo><relationshipName>Steps</relationshipName></CustomField>`,
		"customMetadata/Route.North.md-meta.xml":              `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>North</label><values><field>Enabled__c</field><value>true</value></values></CustomMetadata>`,
		"customMetadata/Step.First.md-meta.xml":               `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>First</label><values><field>Route__c</field><value>North</value></values><values><field>Enabled__c</field><value>true</value></values></CustomMetadata>`,
		"customMetadata/Step.Disabled.md-meta.xml":            `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Disabled</label><values><field>Route__c</field><value>North</value></values><values><field>Enabled__c</field><value>false</value></values></CustomMetadata>`,
		"classes/RouteCache.cls": `public class RouteCache {
    public static Map<String, Route__mdt> rows {
        get {
            if (rows == null) {
                rows = new Map<String, Route__mdt>();
                for (Route__mdt row : Database.query('SELECT DeveloperName, (SELECT DeveloperName, Enabled__c FROM Steps__r WHERE Enabled__c = true) FROM Route__mdt WHERE Enabled__c = true', AccessLevel.SYSTEM_MODE)) {
                    rows.put(row.DeveloperName, row);
                }
            }
            return rows;
        }
        private set;
    }
}`,
		"classes/RouteIteratorTest.cls": `@IsTest private class RouteIteratorTest {
    @IsTest static void directListAndMap() {
        List<Route__mdt> rows = Database.query('SELECT DeveloperName, (SELECT DeveloperName FROM Steps__r WHERE Enabled__c = true) FROM Route__mdt WHERE Enabled__c = true');
        System.assertEquals(1, rows.size(), 'parent count');
        System.assertEquals(1, rows[0].Steps__r.size(), 'direct child count');
        System.assertEquals('First', rows[0].Steps__r.iterator().next().DeveloperName, 'direct iterator');
        Map<String, Route__mdt> mapped = new Map<String, Route__mdt>();
        mapped.put(rows[0].DeveloperName, rows[0]);
        System.assertEquals(1, mapped.get('North').Steps__r.size(), 'map child count');
        System.assertEquals('First', mapped.get('North').Steps__r.iterator().next().DeveloperName, 'map iterator');
        Iterator<Step__mdt> iter = mapped.get('North').Steps__r.iterator();
        System.assert(iter.hasNext());
        System.assertEquals('First', iter.next().DeveloperName);
        System.assert(!iter.hasNext());
        Boolean exhausted = false;
        try { iter.next(); } catch (NoSuchElementException e) { exhausted = true; }
        System.assert(exhausted, 'exhaustion control');
    }
    @IsTest static void staticCachedMap() {
        System.assertEquals(1, RouteCache.rows.get('North').Steps__r.size(), 'cached child count');
        Step__mdt row = RouteCache.rows.get('North').Steps__r.iterator().next();
        System.assertEquals('First', row.DeveloperName, 'cached iterator');
        System.assertEquals('First', RouteCache.rows.get('North').Steps__r.iterator().next().DeveloperName, 'fresh iterator');
    }
}`,
	}
	for path, contents := range files {
		if path != "sfdx-project.json" {
			path = filepath.Join("force-app/main/default", path)
		}
		writeFile(t, filepath.Join(root, path), contents)
	}
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("metadata iterator boundary control: %s", data)
	}
}
