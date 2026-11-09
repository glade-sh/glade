@IsTest public class SameGroupProbeTest {
 @IsTest static void observe() {
  Id pid=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id;
  List<User> users=new List<User>();
  for(Integer i=0;i<3;i++) users.add(new User(Username='same.'+i+'.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='same',Email='same@example.invalid',LastName='Same',ProfileId=pid,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'));
  insert users; Id permission=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1].Id;
  insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=users[0].Id,PermissionSetId=permission),new PermissionSetAssignment(AssigneeId=users[1].Id,PermissionSetId=permission),new PermissionSetAssignment(AssigneeId=users[2].Id,PermissionSetId=permission)};
  Id groupId=[SELECT Id FROM Group WHERE DeveloperName='ProbeOrigin' LIMIT 1].Id;
  GroupMember readerMember=new GroupMember(GroupId=groupId,UserOrGroupId=users[1].Id);
  insert new List<GroupMember>{new GroupMember(GroupId=groupId,UserOrGroupId=users[0].Id),readerMember};
  Id recordId; System.runAs(users[0]){PrivateProbe__c p=new PrivateProbe__c(Name='Same Group Record');insert p;recordId=p.Id;}
  Map<String,Integer> result=new Map<String,Integer>();
  System.runAs(users[1]){result.put('recipient',[SELECT COUNT() FROM PrivateProbe__c WHERE Id=:recordId]);}
  System.runAs(users[2]){result.put('other',[SELECT COUNT() FROM PrivateProbe__c WHERE Id=:recordId]);}
  result.put('share',[SELECT COUNT() FROM PrivateProbe__Share WHERE ParentId=:recordId AND UserOrGroupId=:groupId AND RowCause='Rule']);
  delete readerMember; System.runAs(users[1]){result.put('removed',[SELECT COUNT() FROM PrivateProbe__c WHERE Id=:recordId]);}
  System.assert(false,'OBSERVED|sameGroup|'+JSON.serialize(result));
 }
}
