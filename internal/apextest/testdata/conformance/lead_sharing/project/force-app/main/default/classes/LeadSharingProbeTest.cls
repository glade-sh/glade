@IsTest
private with sharing class LeadSharingProbeTest {
    static User makeUser(Id profileId, String key) {
        return new User(Alias=key, Email='synthetic@example.invalid', EmailEncodingKey='UTF-8', LastName='Synthetic', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=profileId, TimeZoneSidKey='America/Los_Angeles', Username='flowbatch.'+key+'.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');
    }
    @IsTest static void observe() {
        Profile p = [SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
        User creator=makeUser(p.Id,'r29cre'), reader=makeUser(p.Id,'r29read'), member=makeUser(p.Id,'r29mem');
        insert new List<User>{creator,reader,member};
        Group emptyQueue = new Group(Name='FlowBatch Empty', DeveloperName='FlowBatch_Empty', Type='Queue');
        Group fullQueue = new Group(Name='FlowBatch Full', DeveloperName='FlowBatch_Full', Type='Queue');
        insert new List<Group>{emptyQueue,fullQueue};
        insert new List<QueueSobject>{new QueueSobject(QueueId=emptyQueue.Id,SobjectType='Lead'),new QueueSobject(QueueId=fullQueue.Id,SobjectType='Lead')};
        insert new GroupMember(GroupId=fullQueue.Id,UserOrGroupId=member.Id);
        Map<String,Id> ids = new Map<String,Id>();
        System.runAs(creator) {
            List<Lead> leads = new List<Lead>{new Lead(LastName='Empty queue',Company='Synthetic',OwnerId=emptyQueue.Id),new Lead(LastName='Full queue',Company='Synthetic',OwnerId=fullQueue.Id),new Lead(LastName='Creator owned',Company='Synthetic',OwnerId=creator.Id),new Lead(LastName='Reader owned',Company='Synthetic',OwnerId=reader.Id)};
            insert leads;
            for (Lead l:leads) ids.put(l.LastName,l.Id);
        }
        Map<String,Object> out = new Map<String,Object>();
        System.runAs(creator) {
            for (String key:ids.keySet()) { Id id=ids.get(key); out.put('creator:'+key,[SELECT count() FROM Lead WHERE Id=:id WITH USER_MODE]); }
        }
        System.runAs(reader) {
            for (String key:ids.keySet()) { Id id=ids.get(key); out.put('reader:'+key,[SELECT count() FROM Lead WHERE Id=:id WITH USER_MODE]); }
        }
        System.assert(false,'LEADSHARING|' + JSON.serialize(out));
    }
}
