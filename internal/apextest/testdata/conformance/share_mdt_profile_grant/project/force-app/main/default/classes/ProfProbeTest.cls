@IsTest
private class ProfProbeTest {
    static void go(String profileName) {
        Profile p = [SELECT Id FROM Profile WHERE Name = :profileName];
        User u = new User(Username = 'u' + UserInfo.getOrganizationId() + '@synthtest.example', Alias = 'pu', Email = 'pu@synthtest.example',
            EmailEncodingKey = 'UTF-8', LastName = 'pu', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles');
        insert u;
        String out;
        System.runAs(u) { out = ProfProbe.go(); }
        System.assert(false, 'OUT:' + out);
    }
    @IsTest static void granted() { go('SynthProf'); }
    @IsTest static void ungranted() { go('SynthProfNone'); }
}
