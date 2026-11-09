public class HhProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        delete [SELECT Id FROM V35Hh__c];
        insert new V35Hh__c(SetupOwnerId = UserInfo.getOrganizationId(), Val__c = 'a');
        m.put('adminWith', HhWith.run());
        m.put('adminWithout', HhWithout.run());
        if (!Test.isRunningTest()) { m.put('standard', 'not a test run'); return m; }
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        String uniq = String.valueOf(Math.abs(Crypto.getRandomInteger()));
        User u = new User(Alias = 'v35h', Email = 'v35h' + uniq + '@example.com', EmailEncodingKey = 'UTF-8', LastName = 'V35', LanguageLocaleKey = 'en_US',
            LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', Username = 'v35h' + uniq + '@example.com');
        insert u;
        try {
            System.runAs(u) { m.put('standardWith', HhWith.run()); m.put('standardWithout', HhWithout.run()); }
        } catch (Exception e) { m.put('standard', e.getTypeName() + ':' + e.getMessage()); }
        return m;
    }
}
