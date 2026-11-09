public class BeProbe {
    public class ProbeException extends Exception {}
    public static String err(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    static Object tryRun(String what) {
        try {
            if (what == 'userModeQuery') { [SELECT Id FROM Probe__c WITH USER_MODE].size(); return 'ok'; }
            if (what == 'userModeInsert') { Database.insert(new Probe__c(Name = 'u', Val__c = 'x'), AccessLevel.USER_MODE); return 'ok'; }
            if (what == 'userModeUpdate') { Probe__c p = (Probe__c) Database.query('SELECT Id FROM Probe__c WHERE Name = \'seed\'', AccessLevel.SYSTEM_MODE)[0]; p.Val__c = 'y'; Database.update(p, AccessLevel.USER_MODE); return 'ok'; }
            if (what == 'userModeInsertAccount') { Database.insert(new Account(Name = 'u'), AccessLevel.USER_MODE); return 'ok'; }
        } catch (Exception e) {
            return err(e);
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
    static Map<String, Object> flags() {
        return new Map<String, Object>{'isBatch' => System.isBatch(), 'isFuture' => System.isFuture(), 'isQueueable' => System.isQueueable(), 'isScheduled' => System.isScheduled()};
    }
    public static void record(String ctx, Map<String, Object> extra, Boolean withProbe) {
        Map<String, Object> o = flags();
        o.put('userType', UserInfo.getUserType());
        o.put('userId', idKind(UserInfo.getUserId()));
        if (withProbe) {
            for (String k : new List<String>{'userModeQuery', 'userModeInsert', 'userModeUpdate', 'userModeInsertAccount'}) {
                o.put(k, tryRun(k));
            }
        }
        if (extra != null) { o.putAll(extra); }
        Database.insert(new Obs__c(Name = ctx, Data__c = JSON.serialize(o)), AccessLevel.SYSTEM_MODE);
    }
    // The async call an event trigger makes; jobs record as <src>_<kind>.
    public static String call(String src, String kind) {
        try {
            if (kind == 'future') { fut(src + '_future'); }
            else if (kind == 'batch') { Database.executeBatch(new BeChildBatch(src + '_batch'), 1); }
            else if (kind == 'schedBatch') { System.scheduleBatch(new BeChildBatch(src + '_schedBatch'), 'sb_' + src, 1); }
            else if (kind == 'queue') { System.enqueueJob(new Q(src + '_queue')); }
            else if (kind == 'schedule') { System.schedule('sc_' + src, '0 0 0 1 1 ? 2099', new Sched(src + '_schedule')); }
            else { return 'none'; }
            return 'ok';
        } catch (Exception e) {
            return err(e);
        }
    }
    static String kindOf(String note) {
        if (note != null && note.startsWith('call:')) { return note.removeStart('call:'); }
        return 'none';
    }
    public static void onEvent(String src, String note) {
        Map<String, Object> extra = new Map<String, Object>{'call' => call(src, kindOf(note))};
        record(src + '_trigger', extra, true);
        if (note == 'throw') { throw new ProbeException('trigger threw'); }
    }
    public static void onCdc(List<String> types, String note) {
        Integer n = Database.countQuery('SELECT COUNT() FROM Obs__c WHERE Name LIKE \'cdc_trigger%\'', AccessLevel.SYSTEM_MODE);
        Map<String, Object> extra = new Map<String, Object>{'call' => call('cdc', kindOf(note)), 'changeTypes' => types};
        record('cdc_trigger_' + (n + 1), extra, true);
        if (note == 'throw') { throw new ProbeException('trigger threw'); }
    }
    public class Q implements Queueable {
        String ctx;
        public Q(String ctx) { this.ctx = ctx; }
        public void execute(QueueableContext c) { record(ctx, null, true); }
    }
    @future public static void fut(String ctx) { record(ctx, null, true); }
    public class Sched implements Schedulable {
        String ctx;
        public Sched(String ctx) { this.ctx = ctx; }
        public void execute(SchedulableContext c) { record(ctx, null, true); }
    }
    
    public static void publish(String evt, String note) {
        if (evt == 'pe') { System.assert(EventBus.publish(new Evt__e(Note__c = note)).isSuccess()); }
        else if (evt == 'pei') { System.assert(EventBus.publish(new EvtI__e(Note__c = note)).isSuccess()); }
        else { Database.insert(new Cdc__c(Name = note), AccessLevel.SYSTEM_MODE); }
    }
    
    public static List<String> names() {
        List<String> mid = new List<String>();
        for (Obs__c r : (List<Obs__c>) Database.query('SELECT Name FROM Obs__c WHERE Name != \'seed\' ORDER BY Name, Id', AccessLevel.SYSTEM_MODE)) { mid.add(r.Name); }
        return mid;
    }
    static Map<String, Object> finish(Map<String, Object> o) {
        o.put('all', names());
        for (Obs__c r : (List<Obs__c>) Database.query('SELECT Name, Data__c FROM Obs__c WHERE Name != \'seed\' ORDER BY Name, Id', AccessLevel.SYSTEM_MODE)) {
            o.put(r.Name, JSON.deserializeUntyped(r.Data__c));
        }
        return o;
    }
    static void seed() {
        Database.insert(new Probe__c(Name = 'seed', Val__c = 'v'), AccessLevel.SYSTEM_MODE);
        Database.insert(new Obs__c(Name = 'seed'), AccessLevel.SYSTEM_MODE);
    }
    // N13: event trigger during batch start/execute calling async.
    public static Map<String, Object> runBatch(String phase, String evt, String kind, Boolean deliver) {
        seed();
        if (evt == 'cdc') { Test.enableChangeDataCapture(); }
        Test.startTest();
        Database.executeBatch(new BePubBatch(phase, evt, kind, deliver), 1);
        Test.stopTest();
        return finish(new Map<String, Object>());
    }
    // Control and N12: the same trigger call outside a batch, delivered explicitly.
    public static Map<String, Object> runDirect(String evt, String kind) {
        seed();
        Test.startTest();
        publish(evt, 'call:' + kind);
        Test.getEventBus().deliver();
        List<String> mid = names();
        Test.stopTest();
        return finish(new Map<String, Object>{'afterDeliver' => mid});
    }
    // N13: a second executeBatch in one test method.
    public static Map<String, Object> twoBatches(String shape) {
        seed();
        Map<String, Object> o = new Map<String, Object>();
        Boolean before = shape == 'beforeAndInside' || shape == 'beforeOnly';
        if (before) { o.put('first', submit('b1')); }
        if (shape != 'noStart') { Test.startTest(); }
        if (!before) { o.put('first', submit('b1')); }
        if (shape != 'insideThenAfter' && shape != 'beforeOnly') { o.put('second', submit('b2')); }
        if (shape != 'noStart') { Test.stopTest(); }
        if (shape == 'insideThenAfter') { o.put('second', submit('b2')); }
        return finish(o);
    }
    static String submit(String ctx) {
        try { return Database.executeBatch(new BeChildBatch(ctx), 1) == null ? 'null' : 'ok'; } catch (Exception e) { return err(e); }
    }
    static Integer cdcRows() { return Database.countQuery('SELECT COUNT() FROM Cdc__c', AccessLevel.SYSTEM_MODE); }
    // k3c81d5f0e29: a CDC event skipped at stopTest, then deliver().
    public static Map<String, Object> skip(Integer probe) {
        seed();
        Map<String, Object> o = new Map<String, Object>();
        if (probe == 1 || probe == 2) {
            Test.startTest();
            publish('cdc', 'plain');
            Test.stopTest();
            o.put('afterStop', names());
            if (probe == 2) { Test.enableChangeDataCapture(); }
            Test.getEventBus().deliver();
            o.put('afterDeliver', names());
        } else if (probe == 3) {
            publish('cdc', 'plain');
            Test.getEventBus().deliver();
            o.put('afterDeliver1', names());
            Test.enableChangeDataCapture();
            Test.getEventBus().deliver();
            o.put('afterDeliver2', names());
        } else {
            Cdc__c rec = new Cdc__c(Name = 'plain');
            Database.insert(rec, AccessLevel.SYSTEM_MODE);
            Test.enableChangeDataCapture();
            if (probe == 5) { Test.startTest(); }
            rec.Name = 'updated';
            Database.update(rec, AccessLevel.SYSTEM_MODE);
            if (probe == 5) { Test.stopTest(); o.put('afterStop', names()); }
            else { Test.getEventBus().deliver(); o.put('afterDeliver', names()); }
        }
        return finish(o);
    }
    // k3c81d5f0e29 probe 5: a CDC (or platform event) trigger that throws.
    public static Map<String, Object> throwing(String evt, Boolean deliver) {
        seed();
        Map<String, Object> o = new Map<String, Object>();
        if (evt == 'cdc' && !deliver) { Test.enableChangeDataCapture(); }
        Test.startTest();
        publish(evt, 'throw');
        if (deliver) {
            try { Test.getEventBus().deliver(); o.put('deliver', 'ok'); } catch (Exception e) { o.put('deliver', err(e)); }
            o.put('afterDeliver', names());
        }
        try { Test.stopTest(); o.put('stop', 'ok'); } catch (Exception e) { o.put('stop', err(e)); }
        o.put('cdcRows', cdcRows());
        return finish(o);
    }
    // Anonymous route: the same publisher batch outside a test; read with dump().
    public static void anon(String phase, String evt, String kind) {
        Database.delete((List<SObject>) Database.query('SELECT Id FROM Obs__c', AccessLevel.SYSTEM_MODE), AccessLevel.SYSTEM_MODE);
        Database.delete((List<SObject>) Database.query('SELECT Id FROM Probe__c', AccessLevel.SYSTEM_MODE), AccessLevel.SYSTEM_MODE);
        Database.delete((List<SObject>) Database.query('SELECT Id FROM Cdc__c', AccessLevel.SYSTEM_MODE), AccessLevel.SYSTEM_MODE);
        try { for (CronTrigger ct : [SELECT Id FROM CronTrigger WHERE CronJobDetail.Name LIKE 'sb_%' OR CronJobDetail.Name LIKE 'sc_%']) { System.abortJob(ct.Id); } } catch (Exception e) {}
        seed();
        Database.executeBatch(new BePubBatch(phase, evt, kind, false), 1);
    }
    public static String dump() { return JSON.serialize(finish(new Map<String, Object>())); }
}
