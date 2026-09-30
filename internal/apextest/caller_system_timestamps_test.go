package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF221 exact API65 caller timestamps and StandardController assertions.
func TestCallerSystemTimestampsSF221(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "65.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTemplateVersion65Assertions.cls"), `@IsTest private class GladeTemplateVersion65Assertions {
 private static Object millis(Datetime value){return value==null?null:value.getTime();}
 @IsTest static void assertCallerSystemFieldsAndController() {
  Map<String,Object> result=new Map<String,Object>();
  GladeVersionPlan65Assert__c plan=new GladeVersionPlan65Assert__c(Name='Owned plan');
  insert plan;
  result.put('callerCreatedAfterInsert',millis(plan.CreatedDate));
  result.put('callerModifiedAfterInsert',millis(plan.LastModifiedDate));
  GladeVersionTemplate65Assert__c template=new GladeVersionTemplate65Assert__c(Name='Owned template');insert template;
  result.put('templateCallerModified',millis(template.LastModifiedDate));
  plan.Template__c=template.Id;update plan;
  result.put('callerCreatedAfterUpdate',millis(plan.CreatedDate));
  result.put('callerModifiedAfterUpdate',millis(plan.LastModifiedDate));
  ApexPages.StandardController controller=new ApexPages.StandardController(plan);
  GladeVersionPlan65Assert__c supplied=(GladeVersionPlan65Assert__c)controller.getRecord();
  GladeVersionTemplate65Assert__c queriedTemplate=[SELECT LastModifiedDate FROM GladeVersionTemplate65Assert__c WHERE Id=:template.Id];
  GladeVersionPlan65Assert__c queriedPlan=[SELECT CreatedDate,LastModifiedDate FROM GladeVersionPlan65Assert__c WHERE Id=:plan.Id];
  result.put('controllerIdMatches',supplied.Id==plan.Id);
  result.put('controllerTemplateMatches',supplied.Template__c==template.Id);
  result.put('controllerCreated',millis(supplied.CreatedDate));
  result.put('queriedPlanCreated',millis(queriedPlan.CreatedDate));
  result.put('queriedPlanModified',millis(queriedPlan.LastModifiedDate));
  result.put('queriedTemplateModified',millis(queriedTemplate.LastModifiedDate));
  result.put('originalComparison',queriedTemplate.LastModifiedDate>supplied.CreatedDate);
  result.put('queriedComparison',queriedTemplate.LastModifiedDate>queriedPlan.CreatedDate);
  Datetime missing;
  result.put('nullComparison',queriedTemplate.LastModifiedDate>missing);
  result.put('reverseNullComparison',missing>queriedTemplate.LastModifiedDate);
  System.assertEquals(new Set<String>{'reverseNullComparison','nullComparison','queriedComparison','originalComparison','queriedTemplateModified','queriedPlanModified','queriedPlanCreated','controllerCreated','controllerTemplateMatches','controllerIdMatches','callerModifiedAfterUpdate','callerCreatedAfterUpdate','templateCallerModified','callerModifiedAfterInsert','callerCreatedAfterInsert'},result.keySet());
  System.assertEquals(false,result.get('reverseNullComparison'),'reverseNullComparison');
  System.assertEquals(false,result.get('nullComparison'),'nullComparison');
  System.assertEquals(false,result.get('originalComparison'),'originalComparison');
  System.assertNotEquals(null,result.get('queriedTemplateModified'),'queriedTemplateModified');
  System.assertNotEquals(null,result.get('queriedPlanModified'),'queriedPlanModified');
  System.assertNotEquals(null,result.get('queriedPlanCreated'),'queriedPlanCreated');
  System.assertEquals(null,result.get('controllerCreated'),'controllerCreated');
  System.assertEquals(true,result.get('controllerTemplateMatches'),'controllerTemplateMatches');
  System.assertEquals(true,result.get('controllerIdMatches'),'controllerIdMatches');
  System.assertEquals(null,result.get('callerModifiedAfterUpdate'),'callerModifiedAfterUpdate');
  System.assertEquals(null,result.get('callerCreatedAfterUpdate'),'callerCreatedAfterUpdate');
  System.assertEquals(null,result.get('templateCallerModified'),'templateCallerModified');
  System.assertEquals(null,result.get('callerModifiedAfterInsert'),'callerModifiedAfterInsert');
  System.assertEquals(null,result.get('callerCreatedAfterInsert'),'callerCreatedAfterInsert');
  System.debug('GLADE_TEMPLATE_TIMING65='+JSON.serialize(new Map<String,Object>{'queriedPlanCreated'=>result.get('queriedPlanCreated'),'queriedPlanModified'=>result.get('queriedPlanModified'),'queriedTemplateModified'=>result.get('queriedTemplateModified'),'queriedComparison'=>result.get('queriedComparison')}));
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTemplateVersion65Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeVersionPlan65Assert__c/GladeVersionPlan65Assert__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned Version Record</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Owned Version Records</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeVersionTemplate65Assert__c/GladeVersionTemplate65Assert__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned Version Record</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Owned Version Records</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeVersionPlan65Assert__c/fields/Template__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Template__c</fullName><label>Template</label><type>Lookup</type><referenceTo>GladeVersionTemplate65Assert__c</referenceTo><relationshipName>OwnedVersionPlans65Assert</relationshipName></CustomField>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("caller timestamps: %s", b)
	}
}
