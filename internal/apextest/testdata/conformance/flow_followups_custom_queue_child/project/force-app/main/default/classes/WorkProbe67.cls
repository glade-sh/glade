@IsTest private with sharing class WorkProbe67 {
 static User user(String label, Id profileId) { return new User(Alias=label, Email=label+'@example.invalid', EmailEncodingKey='UTF-8', LastName=label, LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='GMT', ProfileId=profileId, Username=label+Math.abs(Crypto.getRandomInteger())+'@example.invalid'); }
 static Integer visible(Id item) { return [SELECT count() FROM SyntheticWork__c WHERE Id=:item]; }
 static Integer visibleUser(Id item) { return [SELECT count() FROM SyntheticWork__c WHERE Id=:item WITH USER_MODE]; }
 static Integer access(Id item) { Id actor=UserInfo.getUserId(); List<UserRecordAccess> rows=[SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:actor AND RecordId=:item]; return rows.isEmpty() ? -1 : (rows[0].HasReadAccess ? 1 : 0); }
 @IsTest static void probe() {
  Map<String,Object> obs=new Map<String,Object>();
  try {
   Id profileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id;
   User a=user('probea',profileId), b=user('probeb',profileId); Group emptyQ, fullQ;
   System.runAs(new User(Id=UserInfo.getUserId())) {
    Database.DMLOptions opt=new Database.DMLOptions(); opt.EmailHeader.triggerUserEmail=false; opt.EmailHeader.triggerOtherEmail=false;
    Database.insert(new List<User>{a,b},opt);
    Id permissionId=[SELECT Id FROM PermissionSet WHERE Name='SyntheticWorkAccess' LIMIT 1].Id; insert new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=a.Id,PermissionSetId=permissionId),new PermissionSetAssignment(AssigneeId=b.Id,PermissionSetId=permissionId)};
    emptyQ=new Group(Name='Synthetic Empty',DeveloperName='SyntheticEmpty',Type='Queue'); fullQ=new Group(Name='Synthetic Full',DeveloperName='SyntheticFull',Type='Queue'); insert new List<Group>{emptyQ,fullQ};
    insert new List<QueueSobject>{new QueueSobject(QueueId=emptyQ.Id,SobjectType='SyntheticWork__c'),new QueueSobject(QueueId=fullQ.Id,SobjectType='SyntheticWork__c')};
    insert new GroupMember(GroupId=fullQ.Id,UserOrGroupId=b.Id);
   }
   Id ownId, emptyId, fullId, caseId;
   System.runAs(a) {
    SyntheticWork__c own=new SyntheticWork__c(Name='Own'); SyntheticWork__c emptyWork=new SyntheticWork__c(Name='Empty',OwnerId=emptyQ.Id); SyntheticWork__c fullWork=new SyntheticWork__c(Name='Full',OwnerId=fullQ.Id); Case anchorCase=new Case(Subject='Synthetic',Origin='Phone');insert anchorCase;caseId=anchorCase.Id;own.CaseLink__c=caseId;fullWork.CaseLink__c=caseId;insert new List<SyntheticWork__c>{own,emptyWork,fullWork};
    ownId=own.Id;emptyId=emptyWork.Id;fullId=fullWork.Id;
    obs.put('owner',visible(ownId));obs.put('creatorEmptyQueue',visible(emptyId));obs.put('creatorFullQueue',visible(fullId));obs.put('creatorFullQueueUser',visibleUser(fullId));obs.put('creatorFullQueueAccess',access(fullId));
   }
   System.runAs(b) {obs.put('otherOwn',visible(ownId));obs.put('otherEmptyQueue',visible(emptyId));obs.put('fullQueueMember',visible(fullId));obs.put('fullQueueMemberUser',visibleUser(fullId));obs.put('otherOwnUser',visibleUser(ownId));obs.put('fullQueueMemberAccess',access(fullId));obs.put('otherOwnAccess',access(ownId)); Set<Id> selected=new Set<Id>{ownId,fullId}; List<AggregateResult> grouped=[SELECT OwnerId,COUNT(Id) total FROM SyntheticWork__c WHERE Id IN :selected GROUP BY OwnerId]; obs.put('groupRows',grouped.size()); obs.put('groupCount',grouped.isEmpty()?0:(Integer)grouped[0].get('total')); List<AggregateResult> groupedUser=[SELECT OwnerId,COUNT(Id) total FROM SyntheticWork__c WHERE Id IN :selected WITH USER_MODE GROUP BY OwnerId]; obs.put('groupRowsUser',groupedUser.size()); obs.put('groupCountUser',groupedUser.isEmpty()?0:(Integer)groupedUser[0].get('total')); SyntheticWork__c child=new SyntheticWork__c(Name='Child',Parent__c=fullId);insert child; SyntheticWork__c projected=[SELECT Parent__r.Name FROM SyntheticWork__c WHERE Id=:child.Id];obs.put('parentProjected',projected.Parent__r==null?'null':projected.Parent__r.Name); List<Case> parents=[SELECT (SELECT Id FROM SyntheticWorks__r) FROM Case WHERE Id=:caseId WITH USER_MODE];obs.put('childRowsUser',parents[0].SyntheticWorks__r.size());}
  } catch(Exception e) {obs.put('setupError',e.getTypeName()+': '+e.getMessage().left(180));}
  System.assert(false,'CAPTURE|SyntheticWork__c|'+JSON.serialize(new Map<String,Object>{'sharing'=>obs}));
 }
}
