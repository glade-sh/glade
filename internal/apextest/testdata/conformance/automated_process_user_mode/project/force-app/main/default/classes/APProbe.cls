public class APProbe {
    static Object tryRun(String what) {
        try {
            if (what == 'userModeQuery') { [SELECT Id FROM Probe__c WITH USER_MODE].size(); return 'ok'; }
            if (what == 'dynamicUserModeQuery') { Database.query('SELECT Id FROM Probe__c', AccessLevel.USER_MODE).size(); return 'ok'; }
            if (what == 'userModeAccountQuery') { [SELECT Id, Name FROM Account WITH USER_MODE].size(); return 'ok'; }
            if (what == 'userModeInsert') { Database.insert(new Probe__c(Name = 'u', Val__c = 'x'), AccessLevel.USER_MODE); return 'ok'; }
            if (what == 'userModeInsertAccount') { Database.insert(new Account(Name = 'u'), AccessLevel.USER_MODE); return 'ok'; }
            if (what == 'userModeUpdate') {
                Probe__c p = (Probe__c) Database.query('SELECT Id FROM Probe__c LIMIT 1', AccessLevel.SYSTEM_MODE)[0];
                p.Val__c = 'changed'; Database.update(p, AccessLevel.USER_MODE); return 'ok';
            }
            if (what == 'userModeDelete') {
                Probe__c p = new Probe__c(Name = 'd'); Database.insert(p, AccessLevel.SYSTEM_MODE);
                Database.delete(p, AccessLevel.USER_MODE); return 'ok';
            }
            if (what == 'systemInsert') { Database.insert(new Probe__c(Name = 's', Val__c = 'x'), AccessLevel.SYSTEM_MODE); return 'ok'; }
        } catch (Exception e) {
            return e.getTypeName() + '|' + e.getMessage();
        }
        return null;
    }
    public static Map<String, Object> probe() {
        Map<String, Object> o = new Map<String, Object>();
        o.put('userType', UserInfo.getUserType());
        for (String k : new List<String>{'userModeQuery', 'dynamicUserModeQuery', 'userModeAccountQuery', 'userModeInsert', 'userModeInsertAccount', 'userModeUpdate', 'userModeDelete', 'systemInsert'}) {
            o.put(k, tryRun(k));
        }
        return o;
    }
    public static void record(String ctx) {
        Database.insert(new Obs__c(Name = ctx, Data__c = JSON.serialize(probe())), AccessLevel.SYSTEM_MODE);
    }
    public class Q implements Queueable {
        String ctx;
        public Q(String ctx) { this.ctx = ctx; }
        public void execute(QueueableContext c) { APProbe.record(ctx); }
    }
    @future public static void fut(String ctx) { record(ctx); }
    @InvocableMethod(label='Rec') public static void inv(List<String> ctxs) { record(ctxs[0]); }
    public static Map<String, Object> observe() {
        Database.insert(new Probe__c(Name = 'seed', Val__c = 'v'), AccessLevel.SYSTEM_MODE);
        Test.startTest();
        System.enqueueJob(new Q('queueable_from_test'));
        fut('future_from_test');
        System.assert(EventBus.publish(new Evt__e(Note__c = 'n')).isSuccess());
        Test.stopTest();
        Map<String, Object> o = new Map<String, Object>();
        for (Obs__c r : (List<Obs__c>) Database.query('SELECT Name, Data__c FROM Obs__c ORDER BY Name', AccessLevel.SYSTEM_MODE)) {
            o.put(r.Name, JSON.deserializeUntyped(r.Data__c));
        }
        return o;
    }
}
