package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: scratch org, API 62.0. Declared queues appear as Group rows of Type Queue with QueueSobject rows and can own records.
func TestProjectQueuesProjectGroupAndQueueSobjectRows(t *testing.T) {
	root := t.TempDir()
	const base = "force-app/main/default/"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, base, "queues/QProbeA.queue-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Queue xmlns="http://soap.sforce.com/2006/04/metadata">
    <doesSendEmailToMembers>false</doesSendEmailToMembers>
    <name>Cola Probe Protección A</name>
    <queueMembers>
        <roles>
            <role>ProbeA</role>
        </roles>
    </queueMembers>
    <queueSobject>
        <sobjectType>Case</sobjectType>
    </queueSobject>
    <queueSobject>
        <sobjectType>Probe__c</sobjectType>
    </queueSobject>
</Queue>
`)
	writeFile(t, filepath.Join(root, base, "queues/QProbeB.queue-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Queue xmlns="http://soap.sforce.com/2006/04/metadata">
    <doesSendEmailToMembers>true</doesSendEmailToMembers>
    <email>queue@example.com</email>
    <name>QProbeB</name>
    <queueSobject>
        <sobjectType>Case</sobjectType>
    </queueSobject>
</Queue>
`)
	writeFile(t, filepath.Join(root, base, "roles/ProbeA.role-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Role xmlns="http://soap.sforce.com/2006/04/metadata">
<caseAccessLevel>Edit</caseAccessLevel><contactAccessLevel>Edit</contactAccessLevel><description>top</description><mayForecastManagerShare>false</mayForecastManagerShare><name>Probe A</name><opportunityAccessLevel>Edit</opportunityAccessLevel>
</Role>
`)
	writeFile(t, filepath.Join(root, base, "objects/Probe__c/Probe__c.object-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <deploymentStatus>Deployed</deploymentStatus>
    <enableActivities>false</enableActivities>
    <enableBulkApi>true</enableBulkApi>
    <enableFeeds>false</enableFeeds>
    <enableHistory>false</enableHistory>
    <enableLicensing>false</enableLicensing>
    <enableReports>true</enableReports>
    <enableSearch>false</enableSearch>
    <enableSharing>true</enableSharing>
    <enableStreamingApi>true</enableStreamingApi>
    <externalSharingModel>Private</externalSharingModel>
    <label>Probe</label>
    <nameField><label>Expediente</label><type>Text</type></nameField>
    <pluralLabel>Probes</pluralLabel>
    <sharingModel>Private</sharingModel>
    <visibility>Public</visibility>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, base, "classes/QueueProbe.cls"), `@isTest
public class QueueProbe {
  @isTest static void run() {
    String o = '';
    for (Group g : [SELECT Id, Name, DeveloperName, Type, Email, DoesSendEmailToMembers, DoesIncludeBosses, RelatedId FROM Group WHERE Type = 'Queue' AND DeveloperName LIKE 'QProbe%' ORDER BY DeveloperName]) {
      o += '\nQ|' + g.DeveloperName + '|name=' + g.Name + '|type=' + g.Type + '|email=' + g.Email + '|sendEmail=' + g.DoesSendEmailToMembers + '|bosses=' + g.DoesIncludeBosses + '|related=' + g.RelatedId;
    }
    for (QueueSobject qs : [SELECT Queue.DeveloperName, SobjectType FROM QueueSobject WHERE Queue.DeveloperName LIKE 'QProbe%' ORDER BY Queue.DeveloperName, SobjectType]) {
      o += '\nQS|' + qs.Queue.DeveloperName + '|' + qs.SobjectType;
    }
    Id qid = [SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='QProbeA'].Id;
    Case c = new Case(Subject='q', OwnerId=qid); insert c;
    o += '\nOWN|' + ([SELECT OwnerId FROM Case WHERE Id=:c.Id].OwnerId == qid);
    System.assert(false, o);
  }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/QueueProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{})
	var got string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				got = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: ")
			}
		}
	}
	want := `Q|QProbeA|name=Cola Probe Protección A|type=Queue|email=null|sendEmail=false|bosses=true|related=null
Q|QProbeB|name=QProbeB|type=Queue|email=queue@example.com|sendEmail=true|bosses=true|related=null
QS|QProbeA|Case
QS|QProbeA|Probe__c
QS|QProbeB|Case
OWN|true`
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("native rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}
