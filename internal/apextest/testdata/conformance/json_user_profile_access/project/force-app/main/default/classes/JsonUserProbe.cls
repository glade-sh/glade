public class JsonUserProbe {
    static User base(String profileName, Integer n) {
        Profile p = [SELECT Id FROM Profile WHERE Name = :profileName];
        String uniq = String.valueOf(Math.abs(Crypto.getRandomInteger()));
        return new User(Alias = 'v35' + n, Email = 'v35' + n + uniq + '@example.com', EmailEncodingKey = 'UTF-8', LastName = 'V35' + n, LanguageLocaleKey = 'en_US',
            LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', Username = 'v35' + n + uniq + '@example.com');
    }
    static String access(User u) {
        insert u;
        String r;
        System.runAs(u) {
            Schema.DescribeSObjectResult d = V35Obj__c.SObjectType.getDescribe();
            r = (d.isAccessible() ? 'A' : '-') + (d.isCreateable() ? 'C' : '-') + (d.isUpdateable() ? 'U' : '-') + (d.isDeletable() ? 'D' : '-');
        }
        return r;
    }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        if (!Test.isRunningTest()) { m.put('note', 'not a test run'); return m; }
        Integer n = 0;
        for (String profile : new List<String>{ 'Minimum Access - Salesforce', 'Standard User' }) {
            n++;
            m.put(profile + ' constructed', access(base(profile, n)));
            n++;
            m.put(profile + ' json', access((User)JSON.deserialize(JSON.serialize(base(profile, n)), User.class)));
        }
        return m;
    }
}
