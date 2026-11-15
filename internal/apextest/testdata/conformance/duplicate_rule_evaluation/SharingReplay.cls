public class SharingReplay {
    public static User prepare() {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        User u = new User(LastName='Sharing Reader', Alias='shread', Email='reader@example.invalid', Username='sharing.reader@example.invalid', ProfileId=p.Id, TimeZoneSidKey='America/Los_Angeles', LocaleSidKey='en_US', EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US');
        insert u;
        Group g = new Group(Name='Sharing Readers', DeveloperName='SharingReaders', Type='Regular'); insert g;
        insert new GroupMember(GroupId=g.Id, UserOrGroupId=u.Id);
        return u;
    }
}
