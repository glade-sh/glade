public class DefaultLoadDmlProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    public static Map<String,Object> observe() {
        Map<String,Object> o = new Map<String,Object>();
        if (!Test.isRunningTest()) { runAll(o); return o; }
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User'];
        User u = new User(Alias = 'alias', Email = 'dldp@example.com', EmailEncodingKey = 'UTF-8', FirstName = 'Jill', LastName = 'Scott',
            LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', TimeZoneSidKey = 'America/Los_Angeles', IsActive = true,
            Username = 'dldp' + String.valueOf(Math.abs(Crypto.getRandomLong())) + '@example.com', ProfileId = p.Id);
        insert u;
        System.runAs(u) { runAll(o); }
        return o;
    }
    static void runAll(Map<String,Object> o) {
        {
            for (String tn : new List<String>{ 'Account' }) {
                SObject d = Schema.getGlobalDescribe().get(tn).newSObject(null, true);
                if (tn == 'Account') d.put('Name', 'dflt');
                if (tn == 'Contact') d.put('LastName', 'dflt');
                if (tn == 'Opportunity') { d.put('Name', 'dflt'); d.put('StageName', 'Prospecting'); d.put('CloseDate', Date.today()); }
                if (tn == 'Lead') { d.put('LastName', 'dflt'); d.put('Company', 'c'); }
                if (tn == 'Case') { d.put('Origin', 'Web'); }
                try { Database.insert(new List<SObject>{ d }, AccessLevel.USER_MODE); o.put(tn, 'ok'); } catch (Exception e) { o.put(tn, err(e)); }
            }
            SObject a = Schema.Account.SObjectType.newSObject(null, true);
            a.put('Name', 'plain');
            try { insert a; o.put('AccountStatementInsert', 'ok'); } catch (Exception e) { o.put('AccountStatementInsert', err(e)); }
        }
    }
}
