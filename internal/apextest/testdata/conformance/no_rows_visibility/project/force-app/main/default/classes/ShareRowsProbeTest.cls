@IsTest public class ShareRowsProbeTest {
 @IsTest static void observe() {
  Id profileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id;
  User one=new User(Username='share.one.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='one',Email='one@example.invalid',LastName='One',ProfileId=profileId,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  User two=new User(Username='share.two.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='two',Email='two@example.invalid',LastName='Two',ProfileId=profileId,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert new List<User>{one,two};
  Id permissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id;
  insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=one.Id,PermissionSetId=permissionSetId),new PermissionSetAssignment(AssigneeId=two.Id,PermissionSetId=permissionSetId)};
  Map<String,Id> groups=new Map<String,Id>(); for(Group g:[SELECT Id,DeveloperName FROM Group WHERE DeveloperName IN ('ProbeOrigin','ProbeTarget','ProbeOriginTwo','ProbeTargetTwo')]) groups.put(g.DeveloperName,g.Id);
  insert new List<GroupMember>{new GroupMember(GroupId=groups.get('ProbeOrigin'),UserOrGroupId=one.Id),new GroupMember(GroupId=groups.get('ProbeOriginTwo'),UserOrGroupId=two.Id)};
  Id first; Id second;
  System.runAs(one){PrivateProbe__c p=new PrivateProbe__c(Name='Share row first');insert p;first=p.Id;}
  System.runAs(two){PrivateProbe__c p=new PrivateProbe__c(Name='Share row second');insert p;second=p.Id;}
  Set<Id> parents=new Set<Id>{first,second};
  Integer firstTarget=0,secondTarget=0,firstOwner=0,secondOwner=0;
  for(PrivateProbe__Share s:[SELECT ParentId,UserOrGroupId,RowCause,AccessLevel FROM PrivateProbe__Share WHERE ParentId IN :parents]) {
   if(s.ParentId==first && s.UserOrGroupId==groups.get('ProbeTarget') && s.RowCause=='Rule' && s.AccessLevel=='Read')firstTarget++;
   if(s.ParentId==second && s.UserOrGroupId==groups.get('ProbeTargetTwo') && s.RowCause=='Rule' && s.AccessLevel=='Read')secondTarget++;
   if(s.ParentId==first && s.UserOrGroupId==one.Id && s.RowCause=='Owner')firstOwner++;
   if(s.ParentId==second && s.UserOrGroupId==two.Id && s.RowCause=='Owner')secondOwner++;
  }
  System.assert(false,'OBSERVED|shareRows|'+JSON.serialize(new Map<String,Integer>{'firstTarget'=>firstTarget,'secondTarget'=>secondTarget,'firstOwner'=>firstOwner,'secondOwner'=>secondOwner}));
 }
}
