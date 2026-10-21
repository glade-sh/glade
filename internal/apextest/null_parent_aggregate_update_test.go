package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Recomputing grouped child counts must use the current stored lookup values,
// including an update that assigns a previously null parent to an existing one.
func TestNullParentAggregateUpdate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for name, source := range map[string]string{
				"ContactAccountTotals":      nullParentAggregateHandlerSource,
				"ContactAccountTotalsProof": nullParentAggregateProofSource,
			} {
				writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
				writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			writeFile(t, filepath.Join(base, "triggers/ContactAccountTotalsTrigger.trigger"), `trigger ContactAccountTotalsTrigger on Contact (after insert, after update) {
 ContactAccountTotals.recompute(Trigger.new, Trigger.old);
}`)
			writeFile(t, filepath.Join(base, "triggers/ContactAccountTotalsTrigger.trigger-meta.xml"), `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 4 || got.Passed != 4 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
				data, _ := json.Marshal(run)
				t.Fatalf("null-parent grouped update: %s", data)
			}
		})
	}
}

const nullParentAggregateHandlerSource = `public class ContactAccountTotals {
 public static void recompute(List<Contact> currentRows, List<Contact> previousRows) {
  Set<Id> parentIds=new Set<Id>();
  for(Contact row:currentRows) if(row.AccountId!=null) parentIds.add(row.AccountId);
  if(previousRows!=null) {
   for(Contact row:previousRows) if(row.AccountId!=null) parentIds.add(row.AccountId);
  }
  if(parentIds.isEmpty()) return;
  Map<Id,Integer> totals=new Map<Id,Integer>();
  for(Id parentId:parentIds) totals.put(parentId,0);
  for(AggregateResult grouped:[SELECT AccountId parentId,COUNT(Id) total FROM Contact WHERE AccountId IN :parentIds GROUP BY AccountId]) {
   totals.put((Id)grouped.get('parentId'),(Integer)grouped.get('total'));
  }
  List<Account> changes=new List<Account>();
  for(Id parentId:parentIds) changes.add(new Account(Id=parentId,NumberOfEmployees=totals.get(parentId)));
  update changes;
 }
}`

const nullParentAggregateProofSource = `@IsTest private class ContactAccountTotalsProof {
 private class Fixture {
  Account first;
  Account second;
  List<Contact> children;
 }
 private static Fixture setup() {
  Fixture fixture=new Fixture();
  fixture.first=new Account(Name='First tally');
  fixture.second=new Account(Name='Second tally');
  insert new List<Account>{fixture.first,fixture.second};
  fixture.children=new List<Contact>{
   new Contact(LastName='Unassigned',AccountId=fixture.first.Id),
   new Contact(LastName='Assigned one',AccountId=fixture.first.Id),
   new Contact(LastName='Assigned two',AccountId=fixture.first.Id)
  };
  for(Integer i=0;i<fixture.children.size();i++) {
   fixture.children[i]=(Contact)JSON.deserialize(JSON.serialize(fixture.children[i]),Contact.class);
  }
  fixture.children[0].AccountId=null;
  insert fixture.children;
  Id firstId=fixture.first.Id;
  fixture.first=[SELECT NumberOfEmployees FROM Account WHERE Id=:firstId];
  assertTotal(fixture.first.Id,2);
  return fixture;
 }
 private static void assertTotal(Id parentId,Integer expected) {
  Account stored=[SELECT NumberOfEmployees FROM Account WHERE Id=:parentId];
  System.assertEquals(expected,stored.NumberOfEmployees,'stored grouped child count');
 }
 @IsTest static void assignsPreviouslyNullParent() {
  Fixture fixture=setup();
  Test.startTest();
  fixture.children[0].AccountId=fixture.first.Id;
  update fixture.children[0];
  Test.stopTest();
  assertTotal(fixture.first.Id,3);
 }
 @IsTest static void transfersExistingParent() {
  Fixture fixture=setup();
  Test.startTest();
  fixture.children[1].AccountId=fixture.second.Id;
  update fixture.children[1];
  Test.stopTest();
  assertTotal(fixture.first.Id,1);
  assertTotal(fixture.second.Id,1);
 }
 @IsTest static void clearsExistingParent() {
  Fixture fixture=setup();
  Test.startTest();
  fixture.children[1].AccountId=null;
  update fixture.children[1];
  Test.stopTest();
  assertTotal(fixture.first.Id,1);
 }
 @IsTest static void leavesUnassignedParentUnchanged() {
  Fixture fixture=setup();
  Test.startTest();
  fixture.children[0].LastName='Still unassigned';
  update fixture.children[0];
  Test.stopTest();
  assertTotal(fixture.first.Id,2);
  Id unassignedId=fixture.children[0].Id;
  System.assertEquals(null,[SELECT AccountId FROM Contact WHERE Id=:unassignedId].AccountId);
 }
}`
