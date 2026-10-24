public class ApprovalSubmissionProbe {
    public static Map<String,Object> observeGroup(String shape) {
        Map<String,Object> rows = new Map<String,Object>();
        for (String variant : new List<String>{'base_default','base_true','base_false','typed_default','typed_true','typed_false'}) {
            rows.put(shape + '_' + variant, observe(shape, variant));
        }
        return rows;
    }
    private static Map<String,Object> observe(String shape, String variant) {
        List<Account> accounts = new List<Account>();
        for (Integer i=0; i<(shape.startsWith('single_') ? 1 : 4); i++) accounts.add(new Account(Name='Owned Mixed Approval '+i));
        insert accounts;
        List<Id> targets = new List<Id>();
        List<Approval.ProcessRequest> requests = new List<Approval.ProcessRequest>();
        List<Approval.ProcessSubmitRequest> submissions = new List<Approval.ProcessSubmitRequest>();
        List<Approval.ProcessWorkitemRequest> workitems = new List<Approval.ProcessWorkitemRequest>();
        for (Integer i=0; i<(shape.startsWith('single_') ? 1 : 4); i++) {
            targets.add(accounts[i].Id);
            Boolean valid = shape.startsWith('single_') ? shape != 'single_invalid' : (shape == 'mixed_success' || (shape == 'mixed_early' ? Math.mod(i,2)==1 : Math.mod(i,2)==0));
            Approval.ProcessSubmitRequest request = new Approval.ProcessSubmitRequest();
            request.setObjectId(accounts[i].Id);
            request.setProcessDefinitionNameOrId(valid || (shape == 'mixed_workitems' || shape == 'mixed_reject' || shape.startsWith('single_')) ? 'Owned_Active' : 'No_Such_Process');
            request.setSkipEntryCriteria(true);
            if ((shape == 'mixed_workitems' || shape == 'mixed_reject' || shape.startsWith('single_'))) {
                Approval.ProcessResult pending = Approval.process(request);
                Approval.ProcessWorkitemRequest work = new Approval.ProcessWorkitemRequest();
                work.setWorkitemId(pending.getNewWorkitemIds()[0]);
                work.setAction(valid ? (shape == 'mixed_reject' || shape == 'single_reject' ? 'Reject' : 'Approve') : 'Invalid');
                workitems.add(work);
                requests.add(work);
            } else {
                submissions.add(request);
                requests.add(request);
            }
        }
        Map<String,Object> observed = new Map<String,Object>();
        try {
            List<Approval.ProcessResult> results;
            if (shape.startsWith('single_')) {
                Approval.ProcessRequest base = workitems[0];
                Approval.ProcessResult result;
                if (variant == 'base_default') result = Approval.process(base);
                if (variant == 'base_true') result = Approval.process(base,true);
                if (variant == 'base_false') result = Approval.process(base,false);
                if (variant == 'typed_default') result = Approval.process(workitems[0]);
                if (variant == 'typed_true') result = Approval.process(workitems[0],true);
                if (variant == 'typed_false') result = Approval.process(workitems[0],false);
                results = new List<Approval.ProcessResult>{result};
            } else {
            if (variant == 'base_default') results = Approval.process(requests);
            if (variant == 'base_true') results = Approval.process(requests,true);
            if (variant == 'base_false') results = Approval.process(requests,false);
            if ((shape == 'mixed_workitems' || shape == 'mixed_reject' || shape.startsWith('single_'))) {
                if (variant == 'typed_default') results = Approval.process(workitems);
                if (variant == 'typed_true') results = Approval.process(workitems,true);
                if (variant == 'typed_false') results = Approval.process(workitems,false);
            } else {
                if (variant == 'typed_default') results = Approval.process(submissions);
                if (variant == 'typed_true') results = Approval.process(submissions,true);
                if (variant == 'typed_false') results = Approval.process(submissions,false);
            }
            }
            observed.put('kind','results');
            List<Object> projected = new List<Object>();
            for (Approval.ProcessResult result : results) {
                List<Object> errors = new List<Object>();
                if (result.getErrors()!=null) for (Database.Error problem : result.getErrors()) {
                    errors.add(new Map<String,Object>{'status'=>String.valueOf(problem.getStatusCode()),'message'=>problem.getMessage(),'fields'=>problem.getFields()});
                }
                projected.add(new Map<String,Object>{'success'=>result.isSuccess(),'errors'=>errors});
            }
            observed.put('results',projected);
        } catch (Exception e) {
            observed.put('kind','exception');
            observed.put('type',e.getTypeName());
            observed.put('message',e.getMessage());
            if (e instanceof DmlException) {
                DmlException problem = (DmlException)e;
                List<Object> errors = new List<Object>();
                for (Integer i=0; i<problem.getNumDml(); i++) {
                    errors.add(new Map<String,Object>{'index'=>problem.getDmlIndex(i),'status'=>String.valueOf(problem.getDmlStatusCode(i)),'message'=>problem.getDmlMessage(i)});
                }
                observed.put('errors',errors);
            }
        }
        List<ProcessInstance> instances = [SELECT Status FROM ProcessInstance WHERE TargetObjectId IN :targets];
        Integer pending=0;
        Integer approved=0;
        Integer rejected=0;
        for (ProcessInstance instance : instances) {
            if (instance.Status == 'Pending') pending++;
            if (instance.Status == 'Approved') approved++;
            if (instance.Status == 'Rejected') rejected++;
        }
        observed.put('instances',instances.size());
        observed.put('pending',pending);
        observed.put('approved',approved);
        observed.put('rejected',rejected);
        List<Id> instanceIds = new List<Id>();
        for (ProcessInstance instance : instances) instanceIds.add(instance.Id);
        observed.put('workitems',[SELECT COUNT() FROM ProcessInstanceWorkitem WHERE ProcessInstanceId IN :instanceIds]);
        observed.put('steps',[SELECT COUNT() FROM ProcessInstanceStep WHERE ProcessInstanceId IN :instanceIds]);
        return observed;
    }
}
