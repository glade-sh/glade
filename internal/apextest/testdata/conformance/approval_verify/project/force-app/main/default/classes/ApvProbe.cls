public class ApvProbe {
    static String err(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    static List<String> errs(List<Database.Error> es) {
        if (es == null) { return null; }
        List<String> out = new List<String>();
        for (Database.Error e : es) { out.add(String.valueOf(e.getStatusCode()) + '|' + e.getMessage()); }
        return out;
    }
    static Map<String, Object> lockResult(Approval.LockResult r) { return new Map<String, Object>{'success' => r.isSuccess(), 'errors' => errs(r.getErrors())}; }
    static Map<String, Object> unlockResult(Approval.UnlockResult r) { return new Map<String, Object>{'success' => r.isSuccess(), 'errors' => errs(r.getErrors())}; }
    static Map<String, Object> processResult(Approval.ProcessResult r) {
        return new Map<String, Object>{'success' => r.isSuccess(), 'errors' => errs(r.getErrors()), 'instanceStatus' => r.getInstanceStatus(), 'newWorkitems' => r.getNewWorkitemIds() == null ? -1 : r.getNewWorkitemIds().size()};
    }
    public static Map<String, Object> state(Id rid) {
        Map<String, Object> m = new Map<String, Object>{'locked' => Approval.isLocked(rid)};
        List<Object> instances = new List<Object>();
        for (ProcessInstance pi : [SELECT Status, ProcessDefinition.DeveloperName, (SELECT StepStatus FROM StepsAndWorkitems ORDER BY CreatedDate, Id) FROM ProcessInstance WHERE TargetObjectId = :rid ORDER BY CreatedDate, Id]) {
            List<String> history = new List<String>();
            for (ProcessInstanceHistory h : pi.StepsAndWorkitems) { history.add(h.StepStatus); }
            history.sort();
            instances.add(new Map<String, Object>{'status' => pi.Status, 'definition' => pi.ProcessDefinition.DeveloperName, 'history' => history});
        }
        m.put('instances', instances);
        m.put('workitems', [SELECT COUNT() FROM ProcessInstanceWorkitem WHERE ProcessInstance.TargetObjectId = :rid]);
        List<String> steps = new List<String>();
        for (ProcessInstanceStep s : [SELECT StepStatus FROM ProcessInstanceStep WHERE ProcessInstance.TargetObjectId = :rid]) { steps.add(s.StepStatus); }
        steps.sort();
        m.put('steps', steps);
        return m;
    }
    static Account account(String name) { Account a = new Account(Name = name); insert a; return a; }
    public static Map<String, Object> lockNoProcess() {
        Map<String, Object> o = new Map<String, Object>();
        Account a = account('lock');
        o.put('lock1', lockResult(Approval.lock(a.Id)));
        o.put('afterLock', state(a.Id));
        try { o.put('lock2', lockResult(Approval.lock(a.Id))); } catch (Exception e) { o.put('lock2', err(e)); }
        try { a.Name = 'edited'; update a; o.put('adminUpdate', 'ok'); } catch (Exception e) { o.put('adminUpdate', err(e)); }
        o.put('unlock1', unlockResult(Approval.unlock(a.Id)));
        o.put('afterUnlock', state(a.Id));
        try { o.put('unlock2', unlockResult(Approval.unlock(a.Id))); } catch (Exception e) { o.put('unlock2', err(e)); }
        return o;
    }
    public static Map<String, Object> lockList() {
        Map<String, Object> o = new Map<String, Object>();
        Account a = account('a'); Account b = account('b');
        List<Object> rs = new List<Object>();
        for (Approval.LockResult r : Approval.lock(new List<Account>{a, b})) { rs.add(lockResult(r)); }
        o.put('lock', rs);
        o.put('lockedA', Approval.isLocked(a.Id));
        o.put('lockedMap', Approval.isLocked(new List<Id>{a.Id, b.Id}).values());
        List<Object> us = new List<Object>();
        for (Approval.UnlockResult r : Approval.unlock(new List<Id>{a.Id, b.Id}, false)) { us.add(unlockResult(r)); }
        o.put('unlock', us);
        o.put('lockedB', Approval.isLocked(b));
        return o;
    }
    static Approval.ProcessResult submit(Id rid) {
        Approval.ProcessSubmitRequest req = new Approval.ProcessSubmitRequest();
        req.setObjectId(rid);
        req.setProcessDefinitionNameOrId('Owned_Active');
        req.setComments('submit');
        return Approval.process(req);
    }
    public static Map<String, Object> submitOnly() {
        Map<String, Object> o = new Map<String, Object>();
        Account a = account('s');
        o.put('submit', processResult(submit(a.Id)));
        o.put('afterSubmit', state(a.Id));
        try { o.put('submit2', processResult(submit(a.Id))); } catch (Exception e) { o.put('submit2', err(e)); }
        o.put('unlockPending', unlockResult(Approval.unlock(a.Id)));
        o.put('afterUnlock', state(a.Id));
        return o;
    }
    public static Map<String, Object> decide(String action) {
        Map<String, Object> o = new Map<String, Object>();
        Account a = account('d');
        Approval.ProcessResult s = submit(a.Id);
        o.put('submit', processResult(s));
        Approval.ProcessWorkitemRequest w = new Approval.ProcessWorkitemRequest();
        w.setWorkitemId(s.getNewWorkitemIds()[0]);
        w.setAction(action);
        w.setComments(action);
        try { o.put('decide', processResult(Approval.process(w))); } catch (Exception e) { o.put('decide', err(e)); }
        o.put('afterDecide', state(a.Id));
        try { o.put('decideAgain', processResult(Approval.process(w))); } catch (Exception e) { o.put('decideAgain', err(e)); }
        return o;
    }
    public static Map<String, Object> deploy() {
        Map<String, Object> o = new Map<String, Object>();
        try {
            Id job = Metadata.Operations.enqueueDeployment(new Metadata.DeployContainer(), new ApvCallback());
            o.put('enqueue', job == null ? 'null' : 'id');
        } catch (Exception e) { o.put('enqueue', err(e)); }
        try {
            new ApvCallback().handleResult(new Metadata.DeployResult(), new Metadata.DeployCallbackContext());
            o.put('direct', 'ok');
        } catch (Exception e) { o.put('direct', err(e)); }
        o.put('ran', ApvCallback.ran);
        o.put('status', ApvCallback.status);
        return o;
    }
}
