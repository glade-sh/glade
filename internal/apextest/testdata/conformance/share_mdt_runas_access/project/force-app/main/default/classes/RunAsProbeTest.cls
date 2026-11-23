@IsTest
private class RunAsProbeTest {
    static User mk(String alias, String profileName, String permset) {
        Profile p = [SELECT Id FROM Profile WHERE Name = :profileName];
        User u = new User(Username = alias + UserInfo.getOrganizationId() + '@synthtest.example', Alias = alias, Email = alias + '@synthtest.example',
            EmailEncodingKey = 'UTF-8', LastName = alias, LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles');
        insert u;
        if (permset != null) {
            insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = :permset].Id);
        }
        return u;
    }
    @TestSetup static void setup() {
        mk('mFull', 'Minimum Access - Salesforce', 'SynthFull');
        mk('mCrud', 'Minimum Access - Salesforce', 'SynthCrud');
        mk('mRead', 'Minimum Access - Salesforce', 'SynthRead');
        mk('mNone', 'Minimum Access - Salesforce', 'SynthNone');
        mk('sFull', 'Standard User', 'SynthFull');
        mk('sNone', 'Standard User', 'SynthNone');
        mk('aSys', 'System Administrator', null);
        System.runAs(new User(Id = UserInfo.getUserId())) { insert new Synth_Doc__c(Name = 'd1'); }
    }
    static void go(String alias) {
        User u = [SELECT Id FROM User WHERE Alias = :alias];
        String out;
        Id did = [SELECT Id FROM Synth_Doc__c LIMIT 1].Id;
        System.runAs(u) { out = RunAsProbe.who() + '|' + RunAsProbe.share(did) + '|' + RunAsProbe.mdt() + '|' + RunAsProbe.describeProbe(); }
        System.assert(false, 'OUT:' + out);
    }
    @IsTest static void mFull() { go('mFull'); }
    @IsTest static void mCrud() { go('mCrud'); }
    @IsTest static void mRead() { go('mRead'); }
    @IsTest static void mNone() { go('mNone'); }
    @IsTest static void sFull() { go('sFull'); }
    @IsTest static void sNone() { go('sNone'); }
    @IsTest static void aSys() { go('aSys'); }
}
