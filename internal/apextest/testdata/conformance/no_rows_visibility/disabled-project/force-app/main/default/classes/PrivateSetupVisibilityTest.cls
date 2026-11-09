@IsTest public class PrivateSetupVisibilityTest {
 @TestSetup static void seed() { insert new List<PrivateProbe__c>{new PrivateProbe__c(Name='A'),new PrivateProbe__c(Name='B'),new PrivateProbe__c(Name='C')}; }
 static User makeUser(String suffix, Id role) {
  return new User(Username='probe.'+suffix+'.'+UserInfo.getOrganizationId()+'@example.invalid', Alias='prb', Email='probe@example.invalid', LastName='Probe', ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id, UserRoleId=role, TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
 }
 @IsTest static void visibility() {
  User reader=makeUser('reader',null); insert reader;
  insert new PermissionSetAssignment(AssigneeId=reader.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id);
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(reader) {
   result.put('privateCount',[SELECT COUNT() FROM PrivateProbe__c]);
   result.put('privateLocator',Database.getQueryLocator('SELECT Id FROM PrivateProbe__c LIMIT 1').iterator().hasNext());
   try { PrivateProbe__c one=[SELECT Id FROM PrivateProbe__c LIMIT 1]; result.put('singleton','found'); } catch(QueryException e){result.put('singleton',e.getTypeName());}
  }
  result.put('ownerLocator1',Database.getQueryLocator('SELECT Id FROM PrivateProbe__c ORDER BY Name LIMIT 1').iterator().hasNext());
  Database.QueryLocatorIterator it=Database.getQueryLocator('SELECT Id FROM PrivateProbe__c ORDER BY Name LIMIT 2').iterator(); Integer n=0; while(it.hasNext()){it.next(); n++;} result.put('ownerLocator2',n);
  System.assert(false,'OBSERVED|visibility|'+JSON.serialize(result));
 }
 @IsTest static void hierarchy() {
  Map<String,Id> roles=new Map<String,Id>(); for(UserRole r:[SELECT Id,DeveloperName FROM UserRole WHERE DeveloperName IN ('ProbeParent','ProbeChild')]) roles.put(r.DeveloperName,r.Id);
  User parent=makeUser('parent',roles.get('ProbeParent')); User child=makeUser('child',roles.get('ProbeChild')); User other=makeUser('other',null); insert new List<User>{parent,child,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=parent.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=child.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=other.Id,PermissionSetId=ps)};
  System.runAs(child) { insert new PrivateProbe__c(Name='ChildOwned'); }
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(parent){result.put('parent',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='ChildOwned']);}
  System.runAs(other){result.put('unrelated',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='ChildOwned']);}
  System.assert(false,'OBSERVED|hierarchy|'+JSON.serialize(result));
 }
 @IsTest static void sharingRule() {
  User owner=makeUser('ruleowner',null), recipient=makeUser('recipient',null), other=makeUser('ruleother',null); insert new List<User>{owner,recipient,other};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=owner.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=recipient.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=other.Id,PermissionSetId=ps)};
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget')]) groups.put(g.DeveloperName,g.Id);
  insert new List<GroupMember>{new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id),new GroupMember(GroupId=groups.get('ProbeTarget'),UserOrGroupId=recipient.Id)};
  System.runAs(owner){ insert new PrivateProbe__c(Name='Shared'); }
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(recipient){result.put('recipient',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Shared']);}
  System.runAs(other){result.put('other',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Shared']);}
  System.assert(false,'OBSERVED|sharingRule|'+JSON.serialize(result));
 }
 @IsTest static void membershipChange() {
  User owner=makeUser('changeowner',null), recipient=makeUser('changereader',null); insert new List<User>{owner,recipient};
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=owner.Id,PermissionSetId=ps),new PermissionSetAssignment(AssigneeId=recipient.Id,PermissionSetId=ps)};
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget')]) groups.put(g.DeveloperName,g.Id);
  GroupMember origin=new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id), target=new GroupMember(GroupId=groups.get('ProbeTarget'),UserOrGroupId=recipient.Id); insert new List<GroupMember>{origin,target};
  System.runAs(owner){insert new PrivateProbe__c(Name='Change');}
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(recipient){result.put('before',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Change']);}
  delete origin;
  System.runAs(recipient){result.put('removed',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Change']);}
  insert new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=owner.Id);
  System.runAs(recipient){result.put('restored',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Change']);}
  System.assert(false,'OBSERVED|membershipChange|'+JSON.serialize(result));
 }
}
