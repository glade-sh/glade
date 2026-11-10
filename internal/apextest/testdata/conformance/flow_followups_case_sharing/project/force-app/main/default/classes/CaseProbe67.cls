@IsTest private with sharing class CaseProbe67 {
 static User user(String label, Id profileId) { return new User(Alias=label, Email=label+'@example.invalid', EmailEncodingKey='UTF-8', LastName=label, LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='GMT', ProfileId=profileId, Username=label+Math.abs(Crypto.getRandomInteger())+'@example.invalid'); }
 static Integer visible(Id item) { return [SELECT count() FROM Case WHERE Id=:item]; }
 static Integer visibleUser(Id item) { return [SELECT count() FROM Case WHERE Id=:item WITH USER_MODE]; }
 @IsTest static void probe() {
  Map<String,Object> obs=new Map<String,Object>();
  try {
   Id profileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id;
   User a=user('probea',profileId), b=user('probeb',profileId); Group emptyQ, fullQ;
   System.runAs(new User(Id=UserInfo.getUserId())) {
    Database.DMLOptions opt=new Database.DMLOptions(); opt.EmailHeader.triggerUserEmail=false; opt.EmailHeader.triggerOtherEmail=false;
    Database.insert(new List<User>{a,b},opt);
    emptyQ=new Group(Name='Synthetic Empty',DeveloperName='SyntheticEmpty',Type='Queue'); fullQ=new Group(Name='Synthetic Full',DeveloperName='SyntheticFull',Type='Queue'); insert new List<Group>{emptyQ,fullQ};
    insert new List<QueueSobject>{new QueueSobject(QueueId=emptyQ.Id,SobjectType='Case'),new QueueSobject(QueueId=fullQ.Id,SobjectType='Case')};
    insert new GroupMember(GroupId=fullQ.Id,UserOrGroupId=b.Id);
   }
   Id ownId, emptyId, fullId;
   System.runAs(a) {
    Case own=new Case(Subject='Own',Origin='Phone'); Case emptyCase=new Case(Subject='Empty',Origin='Phone',OwnerId=emptyQ.Id); Case fullCase=new Case(Subject='Full',Origin='Phone',OwnerId=fullQ.Id); insert new List<Case>{own,emptyCase,fullCase};
    ownId=own.Id;emptyId=emptyCase.Id;fullId=fullCase.Id;
    obs.put('owner',visible(ownId));obs.put('creatorEmptyQueue',visible(emptyId));obs.put('creatorFullQueue',visible(fullId));obs.put('creatorFullQueueUser',visibleUser(fullId));
   }
   System.runAs(b) {obs.put('otherOwn',visible(ownId));obs.put('otherEmptyQueue',visible(emptyId));obs.put('fullQueueMember',visible(fullId));obs.put('fullQueueMemberUser',visibleUser(fullId));obs.put('otherOwnUser',visibleUser(ownId));}
  } catch(Exception e) {obs.put('setupError',e.getTypeName()+': '+e.getMessage().left(180));}
  System.assert(false,'CAPTURE|Case|'+JSON.serialize(new Map<String,Object>{'sharing'=>obs}));
 }
}
