public class BrProbe {
    static String enc(String s) {
        if (s == null) { return 'null'; }
        List<String> codes = new List<String>();
        for (Integer c : s.getChars()) { if (c < 48 || (c > 57 && c < 65) || (c > 90 && c < 97) || c > 122) { codes.add(String.valueOf(c)); } }
        return 'len=' + s.length() + ' text=[' + s + '] other=[' + String.join(codes, ',') + ']';
    }
    static Account read(Id id) {
        return [SELECT Name, FaBr__c, FaLen__c, FaSub1__c, FaSub2__c, FaSub3__c, FaNest__c, FdDec__c, FbBr__c, FbLen__c, FcBr__c, FcLen__c, WfBr__c, WfLen__c, Dflt__c, Calc__c FROM Account WHERE Id = :id WITH SYSTEM_MODE];
    }
    static Map<String,Object> fields(Account a, List<String> names) {
        Map<String,Object> out = new Map<String,Object>();
        for (String n : names) { out.put(n, enc((String) a.get(n))); }
        return out;
    }
    static String tryInsert(String name) {
        try { insert as system new Account(Name = name); return 'inserted'; }
        catch (DmlException e) { return 'error:' + e.getDmlStatusCode(0) + ':' + e.getDmlMessage(0); }
    }
    public static String sample() {
        Map<String,Object> out = new Map<String,Object>();
        // Record-triggered before-save flow assigning formula resources.
        Account fa = new Account(Name = 'FlowAssign');
        insert as system fa;
        Account fa2 = read(fa.Id);
        out.put('flowBeforeSaveInMemory', fields(fa, new List<String>{'FaBr__c','FaLen__c','FaSub1__c','FaSub2__c','FaSub3__c','FaNest__c'}));
        out.put('flowBeforeSaveStored', fields(fa2, new List<String>{'FaBr__c','FaLen__c','FaSub1__c','FaSub2__c','FaSub3__c','FaNest__c'}));
        // Record-triggered before-save flow whose Decision tests LEN of a BR() formula.
        Account fd = new Account(Name = 'FlowDecide');
        insert as system fd;
        out.put('flowDecideStored', fields(read(fd.Id), new List<String>{'FdDec__c'}));
        // Autolaunched flow read in Apex, written to a record field.
        Flow.Interview i = Flow.Interview.createInterview('BrResources', new Map<String,Object>());
        i.start();
        Map<String,Object> io = new Map<String,Object>();
        for (String n : new List<String>{'outBr','outLen','outSub1','outSub2','outSub3','outNest','outDec'}) { io.put(n, enc((String) i.getVariableValue(n))); }
        out.put('invocableOutputs', io);
        Account fb = new Account(Name = 'FlowInvocable', FbBr__c = (String) i.getVariableValue('outBr'), FbLen__c = (String) i.getVariableValue('outLen'));
        insert as system fb;
        out.put('invocableStored', fields(read(fb.Id), new List<String>{'FbBr__c','FbLen__c'}));
        // After-save flow reading a stored formula field.
        Account fc = new Account(Name = 'AfterSave');
        insert as system fc;
        out.put('afterSaveStored', fields(read(fc.Id), new List<String>{'Calc__c','FcBr__c','FcLen__c'}));
        // Workflow field updates.
        Account wf = new Account(Name = 'WfTrigger');
        insert as system wf;
        out.put('workflowStored', fields(read(wf.Id), new List<String>{'WfBr__c','WfLen__c'}));
        // Default value formula.
        Account dd = new Account(Name = 'DefaultProbe');
        insert as system dd;
        out.put('defaultInsertStored', fields(read(dd.Id), new List<String>{'Dflt__c'}));
        Account nd = (Account) Account.sObjectType.newSObject(null, true);
        out.put('defaultNewSObject', enc(nd.Dflt__c));
        Account nd2 = new Account();
        out.put('defaultPlainNew', enc(nd2.Dflt__c));
        // Validation rules.
        Map<String,Object> vr = new Map<String,Object>();
        for (String n : new List<String>{'VrLen12','VrLen4','VrCont','VrContTag','VrSub','VrMsg'}) { vr.put(n, tryInsert(n)); }
        out.put('validation', vr);
        return JSON.serialize(out);
    }
}
