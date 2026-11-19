@IsTest private with sharing class LeadProbe67 {
 static User user(String label, Id profileId) { return new User(Alias=label, Email=label+'@example.invalid', EmailEncodingKey='UTF-8', LastName=label, LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='GMT', ProfileId=profileId, Username=label+Math.abs(Crypto.getRandomInteger())+'@example.invalid'); }
 static Integer visible(Id item) { return [SELECT count() FROM Lead WHERE Id=:item]; }
 @IsTest static void probe() {
  Map<String,Object> obs=new Map<String,Object>();
  try {
   Id profileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id;
   User a=user('probea',profileId), b=user('probeb',profileId); Group emptyQ, fullQ;
   System.runAs(new User(Id=UserInfo.getUserId())) {
    Database.DMLOptions opt=new Database.DMLOptions(); opt.EmailHeader.triggerUserEmail=false; opt.EmailHeader.triggerOtherEmail=false;
    Database.insert(new List<User>{a,b},opt);
    emptyQ=new Group(Name='Synthetic Empty',DeveloperName='SyntheticEmpty',Type='Queue'); fullQ=new Group(Name='Synthetic Full',DeveloperName='SyntheticFull',Type='Queue'); insert new List<Group>{emptyQ,fullQ};
    insert new List<QueueSobject>{new QueueSobject(QueueId=emptyQ.Id,SobjectType='Lead'),new QueueSobject(QueueId=fullQ.Id,SobjectType='Lead')};
    insert new GroupMember(GroupId=fullQ.Id,UserOrGroupId=b.Id);
   }
   Id ownId, emptyId, fullId;
   System.runAs(a) {
    Lead own=new Lead(LastName='Own',Company='Synthetic'); Lead emptyLead=new Lead(LastName='Empty',Company='Synthetic',OwnerId=emptyQ.Id); Lead fullLead=new Lead(LastName='Full',Company='Synthetic',OwnerId=fullQ.Id); insert new List<Lead>{own,emptyLead,fullLead};
    ownId=own.Id;emptyId=emptyLead.Id;fullId=fullLead.Id;
    obs.put('owner',visible(ownId));obs.put('creatorEmptyQueue',visible(emptyId));obs.put('creatorFullQueue',visible(fullId));
   }
   System.runAs(b) {obs.put('otherOwn',visible(ownId));obs.put('otherEmptyQueue',visible(emptyId));obs.put('fullQueueMember',visible(fullId));}
  } catch(Exception e) {obs.put('setupError',e.getTypeName()+': '+e.getMessage().left(180));}
  System.assert(false,'CAPTURE|Lead|'+JSON.serialize(new Map<String,Object>{'sharing'=>obs}));
 }
}
