public class C4Probe {
    public static void log(String name, Map<String, Object> data) {
        insert new C4Obs__c(Name = name, Data__c = JSON.serialize(data));
    }
    public static void run(String source, String mode) {
        log(source + '_before', new Map<String, Object>{'mode' => mode, 'queries' => Limits.getQueries(), 'dml' => Limits.getDmlStatements(), 'userType' => UserInfo.getUserType()});
        if (mode == 'npe') {
            String s;
            Integer n = s.length();
        } else if (mode == 'limit') {
            for (Integer i = 0; i < 150; i++) {
                List<C4Obs__c> r = [SELECT Id FROM C4Obs__c LIMIT 1];
            }
        } else if (mode == 'dmlex') {
            insert new Account();
        } else if (mode == 'listx') {
            List<Integer> l = new List<Integer>();
            Integer v = l[3];
        }
        log(source + '_after', new Map<String, Object>{'mode' => mode, 'queries' => Limits.getQueries()});
    }
    public static List<String> dump() {
        List<String> out = new List<String>();
        for (C4Obs__c r : [SELECT Name, Data__c FROM C4Obs__c ORDER BY CreatedDate, Id]) {
            out.add(r.Name + '=' + r.Data__c);
        }
        return out;
    }
}
