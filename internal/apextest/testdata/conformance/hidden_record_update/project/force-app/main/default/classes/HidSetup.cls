// Local stand-in for the org data the native rows read with SeeAllData=true:
// the restricted user, its permission set and the two records.
@IsTest
public class HidSetup {
    public static User run() {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        User u = new User(LastName = 'Hidden', Alias = 'hid62', Email = 'glade.hidden@example.invalid',
            Username = 'glade.hidden.' + String.valueOf(Crypto.getRandomLong()) + '@example.invalid',
            ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', LocaleSidKey = 'en_US',
            EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US');
        // Setup objects and ordinary records cannot share a transaction part.
        System.runAs(new User(Id = UserInfo.getUserId())) {
            insert u;
            PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name = 'HidAccess' LIMIT 1];
            insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = ps.Id);
        }
        insert new HidRec__c(Name = 'HiddenTarget', Flag__c = 'a');
        insert new HidRec__c(Name = 'VisibleTarget', Flag__c = 'a', OwnerId = u.Id);
        return u;
    }
}
