public class EvtProbe {
    public static Integer flag = 0;
    public static String mode = '';
    public static Integer ids = -1;
    public static String trigErr = '';
    public static String evt2(String tag) {
        String out = '';
        try {
            BatchApexErrorEvent ev = new BatchApexErrorEvent(JobScope = '[{"records":["001000000000000"],"size":1}]', ExceptionType = 'System.NullPointerException', Message = 'm', StackTrace = 's', AsyncApexJobId = '707000000000000');
            Test.startTest();
            Database.SaveResult sr = EventBus.publish(ev);
            Test.stopTest();
            out += 'pub=' + sr.isSuccess() + ';flag=' + flag + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String evt(String tag) {
        String out = '';
        try {
            BatchApexErrorEvent e1 = new BatchApexErrorEvent(JobScope = '[{"records":["001000000000000"],"size":1}]', ExceptionType = 'System.NullPointerException', Message = 'm', StackTrace = 's', AsyncApexJobId = '707000000000000');
            out += 'built;';
            Database.SaveResult sr = EventBus.publish(e1);
            out += 'pub=' + sr.isSuccess() + ':' + sr.getErrors().size() + (sr.getErrors().isEmpty() ? '' : ':' + sr.getErrors()[0].getStatusCode() + ':' + sr.getErrors()[0].getMessage()) + ';';
        } catch (Exception e) { out += 'ERR1 ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        try {
            BatchApexErrorEvent e2 = new BatchApexErrorEvent(Message = 'm', AsyncApexJobId = '707000000000000');
            Database.SaveResult sr2 = Database.insertImmediate(e2);
            out += 'insImm=' + sr2.isSuccess() + ':' + sr2.getErrors().size() + (sr2.getErrors().isEmpty() ? '' : ':' + sr2.getErrors()[0].getStatusCode() + ':' + sr2.getErrors()[0].getMessage()) + ';';
        } catch (Exception e) { out += 'ERR2 ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        try {
            BatchApexErrorEvent e3 = new BatchApexErrorEvent(Message = 'm', AsyncApexJobId = '707000000000000');
            insert e3;
            out += 'ins;';
        } catch (Exception e) { out += 'ERR3 ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    static String one(String label, BatchApexErrorEvent ev) {
        try {
            Database.SaveResult sr = EventBus.publish(ev);
            return label + '=' + sr.isSuccess() + ':' + (sr.getErrors().isEmpty() ? '' : String.valueOf(sr.getErrors()[0].getStatusCode())) + ';';
        } catch (Exception e) { return label + '=ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
    }
    public static String evt4(String tag) {
        String out = '';
        out += one('none', new BatchApexErrorEvent());
        out += one('jobid', new BatchApexErrorEvent(AsyncApexJobId = '707000000000000'));
        out += one('exceeds', new BatchApexErrorEvent(DoesExceedJobScopeMaxLength = true));
        out += one('extype', new BatchApexErrorEvent(ExceptionType = 'X'));
        out += one('scope', new BatchApexErrorEvent(JobScope = 'abc'));
        out += one('msg', new BatchApexErrorEvent(Message = 'm'));
        out += one('phase', new BatchApexErrorEvent(Phase = 'EXECUTE'));
        out += one('req', new BatchApexErrorEvent(RequestId = 'r'));
        out += one('stack', new BatchApexErrorEvent(StackTrace = 's'));
        try {
            BatchApexErrorEvent asg = new BatchApexErrorEvent();
            asg.Message = 'assigned';
            out += one('assign', asg);
        } catch (Exception e) { out += 'assign=ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        try {
            List<SObject> er = new List<SObject>{ new BatchApexErrorEvent(Message = 'e') };
            List<Database.SaveResult> rs2 = EventBus.publish(er);
            out += 'erased=' + rs2[0].isSuccess() + ';';
        } catch (Exception e) { out += 'erased=ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        try {
            Database.SaveResult sr3 = Database.insertImmediate(new BatchApexErrorEvent(Message = 'e'));
            out += 'insImm=' + sr3.isSuccess() + ';';
        } catch (Exception e) { out += 'insImm=ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String evt5(String tag) {
        String out = '';
        try {
            BatchApexErrorEvent ev = new BatchApexErrorEvent(JobScope = 'abc', Message = 'm');
            Test.startTest();
            Database.SaveResult sr = EventBus.publish(ev);
            Test.getEventBus().deliver();
            Test.stopTest();
            out += 'pub=' + sr.isSuccess() + ';flag=' + flag + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String bad(String tag) {
        String out = '';
        BatchApexErrorEvent ev;
        try {
            ev = new BatchApexErrorEvent(JobScope = '[{"records":["bad_rec_id_0001"],"size":1}]', ExceptionType = 'System.NullPointerException', Message = 'm', StackTrace = 's', AsyncApexJobId = 'bad_job_id_0001');
            out += 'built;';
        } catch (Exception e) { return tag + '|ERRctor ' + e.getTypeName() + ' ' + e.getMessage(); }
        try {
            Test.startTest();
            Database.SaveResult sr = EventBus.publish(ev);
            Test.stopTest();
            out += 'pub=' + sr.isSuccess() + ';flag=' + flag + ';ids=' + ids + ';trigErr=' + trigErr + ';';
        } catch (Exception e) { out += 'ERRpub ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String good(String tag) {
        String out = '';
        try {
            BatchApexErrorEvent ev = new BatchApexErrorEvent(AsyncApexJobId = '707000000000000', Message = 'm');
            Test.startTest();
            Database.SaveResult sr = EventBus.publish(ev);
            Test.stopTest();
            out += 'pub=' + sr.isSuccess() + ';flag=' + flag + ';ids=' + ids + ';trigErr=' + trigErr + ';';
        } catch (Exception e) { out += 'ERRpub ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String badId(String tag) {
        String out = '';
        try { Id x = 'bad_job_id_0001'; out += 'idlit=' + x + ';'; } catch (Exception e) { out += 'ERRidlit ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        try { BatchApexErrorEvent b = new BatchApexErrorEvent(AsyncApexJobId = 'bad_job_id_0001'); out += 'ctor15;'; } catch (Exception e) { out += 'ERRctor15 ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String uncaught(String tag) {
        String out = '';
        mode = 'throw';
        try {
            BatchApexErrorEvent ev = new BatchApexErrorEvent(AsyncApexJobId = 'bad_job_id_0001', Message = 'm');
            Test.startTest();
            Database.SaveResult sr = EventBus.publish(ev);
            Test.stopTest();
            out += 'pub=' + sr.isSuccess() + ';flag=' + flag + ';ids=' + ids + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String dsc(String tag) {
        String out = '';
        try {
            Schema.DescribeSObjectResult d = BatchApexErrorEvent.SObjectType.getDescribe();
            out += 'creat=' + d.isCreateable() + ';acc=' + d.isAccessible() + ';';
        } catch (Exception e) { out += 'ERRd ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
}
