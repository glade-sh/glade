public with sharing class MeadowDirectProbe {
    public static Map<String,Object> observe() {
        insert new List<UserRole>{new UserRole(Name='Plain One',DeveloperName='PlainOne'),new UserRole(Name='Plain Two',DeveloperName='PlainTwo'),new UserRole(Name='Plain Three',DeveloperName='PlainThree')};
        PermissionSetGroup bundle = new PermissionSetGroup(DeveloperName='PlainBundle',MasterLabel='Plain Bundle');
        insert bundle;
        Group queue = new Group(Name='Plain Queue',DeveloperName='PlainQueue',Type='Queue');
        insert queue;
        QueueSobject supported = new QueueSobject(QueueId=queue.Id,SobjectType='Case');
        insert supported;
        Group regular = new Group(Name='Plain Readers',DeveloperName='PlainReaders',Type='Regular');
        insert regular;
        GroupMember member = new GroupMember(GroupId=regular.Id,UserOrGroupId=UserInfo.getUserId());
        insert member;
        Map<String,Object> result = new Map<String,Object>();
        readRows(result,'admin',bundle.Id,supported.Id,member.Id);
        if (Test.isRunningTest()) {
            Profile profile=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
            User reader=new User(ProfileId=profile.Id,Username='plain.'+profile.Id+'@example.invalid',Email='plain@example.invalid',LastName='Reader',Alias='plain',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
            System.runAs(reader) { readRows(result,'Standard User',bundle.Id,supported.Id,member.Id); }
        }
        return result;
    }
    private static void readRows(Map<String,Object> result,String route,Id bundleId,Id supportedId,Id memberId) {
        List<String> names=new List<String>();
        List<UserRole> roles=[SELECT DeveloperName FROM UserRole WHERE DeveloperName IN ('PlainOne','PlainTwo','PlainThree') ORDER BY DeveloperName];
        for (UserRole role: roles) names.add(role.DeveloperName);
        result.put(route+'.roles.list',names);
        result.put(route+'.roles.count',[SELECT COUNT() FROM UserRole WHERE DeveloperName IN ('PlainOne','PlainTwo','PlainThree')]);
        result.put(route+'.roles.dynamic',Database.query('SELECT Id FROM UserRole WHERE DeveloperName IN (\'PlainOne\',\'PlainTwo\',\'PlainThree\')').size());
        result.put(route+'.roles.grouped',[SELECT DeveloperName n,COUNT(Id) c FROM UserRole WHERE DeveloperName IN ('PlainOne','PlainTwo','PlainThree') GROUP BY DeveloperName].size());
        result.put(route+'.bundle',[SELECT COUNT() FROM PermissionSetGroup WHERE Id=:bundleId WITH SYSTEM_MODE]);
        result.put(route+'.queue',[SELECT COUNT() FROM QueueSobject WHERE Id=:supportedId WITH SYSTEM_MODE]);
        result.put(route+'.member',[SELECT COUNT() FROM GroupMember WHERE Id=:memberId WITH SYSTEM_MODE]);
    }
}
