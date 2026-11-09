public class CdcProbe {
    static Object tryRun(String what) {
        try {
            if (what == 'userModeQuery') { [SELECT Id FROM Probe__c WITH USER_MODE].size(); return 'ok'; }
            if (what == 'userModeInsert') { Database.insert(new Probe__c(Name = 'u', Val__c = 'x'), AccessLevel.USER_MODE); return 'ok'; }
            if (what == 'userModeInsertAccount') { Database.insert(new Account(Name = 'u'), AccessLevel.USER_MODE); return 'ok'; }
            if (what == 'systemInsert') { Database.insert(new Probe__c(Name = 's', Val__c = 'x'), AccessLevel.SYSTEM_MODE); return 'ok'; }
        } catch (Exception e) {
            return e.getTypeName() + '|' + e.getMessage();
        }
        return null;
    }
    static String idKind(Id uid) {
        Id runner = (Id) Database.query('SELECT CreatedById FROM Obs__c WHERE Name = \'seed\'', AccessLevel.SYSTEM_MODE)[0].get('CreatedById');
        List<User> ap = Database.query('SELECT Id FROM User WHERE UserType = \'AutomatedProcess\' LIMIT 1', AccessLevel.SYSTEM_MODE);
        if (!ap.isEmpty() && ap[0].Id == uid) { return 'automatedProcess'; }
        if (uid == runner) { return 'testRunner'; }
        return 'other';
    }
    public static Map<String, Object> probe() {
        Map<String, Object> o = new Map<String, Object>();
        o.put('userType', UserInfo.getUserType());
        o.put('userId', idKind(UserInfo.getUserId()));
        for (String k : new List<String>{'userModeQuery', 'userModeInsert', 'userModeInsertAccount', 'systemInsert'}) {
            o.put(k, tryRun(k));
        }
        return o;
    }
    public static void record(String ctx, Map<String, Object> extra) {
        Map<String, Object> o = probe();
        if (extra != null) { o.putAll(extra); }
        Database.insert(new Obs__c(Name = ctx, Data__c = JSON.serialize(o)), AccessLevel.SYSTEM_MODE);
    }
    public class Q implements Queueable {
        String ctx; Boolean chain;
        public Q(String ctx, Boolean chain) { this.ctx = ctx; this.chain = chain; }
        public void execute(QueueableContext c) {
            Map<String, Object> extra = new Map<String, Object>();
            if (chain) {
                try { System.enqueueJob(new Q(ctx + '_child', false)); extra.put('chainEnqueue', 'ok'); }
                catch (Exception e) { extra.put('chainEnqueue', e.getTypeName() + '|' + e.getMessage()); }
            }
            record(ctx, extra);
        }
    }
    @future public static void fut(String ctx) { record(ctx, null); }
    static List<String> names() {
        List<String> mid = new List<String>();
        for (Obs__c r : (List<Obs__c>) Database.query('SELECT Name FROM Obs__c WHERE Name != \'seed\' ORDER BY Name', AccessLevel.SYSTEM_MODE)) { mid.add(r.Name); }
        return mid;
    }
    static Map<String, Object> finish(List<String> mid) {
        Map<String, Object> o = new Map<String, Object>();
        o.put('afterDeliver', mid);
        for (Obs__c r : (List<Obs__c>) Database.query('SELECT Name, Data__c FROM Obs__c WHERE Name != \'seed\' ORDER BY Name', AccessLevel.SYSTEM_MODE)) {
            o.put(r.Name, JSON.deserializeUntyped(r.Data__c));
        }
        return o;
    }
    static void seed() {
        Database.insert(new Probe__c(Name = 'seed', Val__c = 'v'), AccessLevel.SYSTEM_MODE);
        Database.insert(new Obs__c(Name = 'seed'), AccessLevel.SYSTEM_MODE);
    }
    // CDC trigger, event raised inside startTest/stopTest; enable: Test.enableChangeDataCapture().
    public static Map<String, Object> cdcStop(Boolean enable) {
        seed();
        if (enable) { Test.enableChangeDataCapture(); }
        Test.startTest();
        Database.insert(new Cdc__c(Name = 'c'), AccessLevel.SYSTEM_MODE);
        Test.stopTest();
        return finish(null);
    }
    // CDC event raised before startTest.
    public static Map<String, Object> cdcStopOutside(Boolean enable) {
        seed();
        if (enable) { Test.enableChangeDataCapture(); }
        Database.insert(new Cdc__c(Name = 'c'), AccessLevel.SYSTEM_MODE);
        Test.startTest();
        Test.stopTest();
        return finish(null);
    }
    // CDC trigger, delivered by an explicit deliver() before stopTest.
    public static Map<String, Object> cdcDeliver(Boolean enable) {
        seed();
        if (enable) { Test.enableChangeDataCapture(); }
        Test.startTest();
        Database.insert(new Cdc__c(Name = 'c'), AccessLevel.SYSTEM_MODE);
        Test.getEventBus().deliver();
        List<String> mid = names();
        Test.stopTest();
        return finish(mid);
    }
    // CDC trigger, delivered by an explicit deliver() with no startTest/stopTest.
    public static Map<String, Object> cdcDeliverNoStop() {
        seed();
        Database.insert(new Cdc__c(Name = 'c'), AccessLevel.SYSTEM_MODE);
        Test.getEventBus().deliver();
        return finish(names());
    }
    // Event trigger enqueues a chaining queueable, delivered explicitly.
    public static Map<String, Object> eventChainDeliver() {
        seed();
        Test.startTest();
        System.assert(EventBus.publish(new Evt__e(Note__c = 'n')).isSuccess());
        Test.getEventBus().deliver();
        List<String> mid = names();
        Test.stopTest();
        return finish(mid);
    }
    // Control: a test-started chaining queueable.
    public static Map<String, Object> testChain() {
        seed();
        Test.startTest();
        System.enqueueJob(new Q('queueable_from_test', true));
        Test.stopTest();
        return finish(null);
    }
}
