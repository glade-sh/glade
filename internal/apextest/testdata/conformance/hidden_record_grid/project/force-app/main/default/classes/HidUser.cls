// Local stand-in for the pre-created actor of the native rows.
@IsTest
public class HidUser {
    public static User get() {
        List<User> us = [SELECT Id, Profile.Name FROM User WHERE Alias = 'hidres' LIMIT 1];
        if (!us.isEmpty()) { return us[0]; }
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        User u = new User(LastName = 'Hidres', Alias = 'hidres', Email = 'hidres@example.invalid', Username = 'hidres.' + String.valueOf(Crypto.getRandomLong()) + '@example.invalid', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', LocaleSidKey = 'en_US', EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US');
        System.runAs(new User(Id = UserInfo.getUserId())) {
            insert u;
            insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'HidAccess' LIMIT 1].Id);
        }
        return [SELECT Id, Profile.Name FROM User WHERE Id = :u.Id];
    }
}
