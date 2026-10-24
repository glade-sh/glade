public class ApprovalSubmissionProbe {
    public static Map<String,Object> observeGroup(String shape) {
        Map<String,Object> rows = new Map<String,Object>();
        for (String variant : new List<String>{'single_default','single_true','single_false','list_default','list_true','list_false'}) {
            rows.put(shape + '_' + variant, observe(shape, variant));
        }
        return rows;
    }
    public static Map<String,Object> observe(String shape, String variant) {
        Id recordId;
        if (shape == 'no_applicable') {
            Contact row = new Contact(LastName='Owned Approval Contact');
            insert row;
            recordId = row.Id;
        } else if (shape != 'null_id') {
            Account row = new Account(Name='Owned Approval Account');
            insert row;
            recordId = row.Id;
        }
        Approval.ProcessSubmitRequest request = new Approval.ProcessSubmitRequest();
        request.setObjectId(recordId);
        if (shape == 'missing') request.setProcessDefinitionNameOrId('No_Such_Process');
        if (shape == 'inactive') request.setProcessDefinitionNameOrId('Owned_Inactive');
        if (shape == 'pending') {
            request.setProcessDefinitionNameOrId('Owned_Active');
            request.setSkipEntryCriteria(true);
            Approval.process(request);
        }
        try {
            Approval.ProcessResult result;
            if (variant == 'single_default') result = Approval.process(request);
            if (variant == 'single_true') result = Approval.process(request, true);
            if (variant == 'single_false') result = Approval.process(request, false);
            List<Approval.ProcessRequest> requests = new List<Approval.ProcessRequest>{request};
            if (variant == 'list_default') result = Approval.process(requests)[0];
            if (variant == 'list_true') result = Approval.process(requests, true)[0];
            if (variant == 'list_false') result = Approval.process(requests, false)[0];
            return projectResult(result, recordId);
        } catch (Exception e) {
            Map<String,Object> observed = new Map<String,Object>{'kind'=>'exception','type'=>e.getTypeName(),'message'=>normalize(e.getMessage(),recordId)};
            if (e instanceof DmlException) {
                DmlException problem = (DmlException)e;
                observed.put('count',problem.getNumDml());
                if (problem.getNumDml() > 0) {
                    observed.put('status',String.valueOf(problem.getDmlStatusCode(0)));
                    observed.put('detail',normalize(problem.getDmlMessage(0),recordId));
                    observed.put('index',problem.getDmlIndex(0));
                }
            }
            return observed;
        }
    }
    public static Map<String,Object> success() {
        Account row = new Account(Name='Owned Successful Approval');
        insert row;
        Approval.ProcessSubmitRequest request = new Approval.ProcessSubmitRequest();
        request.setObjectId(row.Id);
        request.setProcessDefinitionNameOrId('Owned_Active');
        request.setSkipEntryCriteria(true);
        request.setComments('Owned submission');
        Approval.ProcessResult result = Approval.process(request);
        Map<String,Object> observed = projectResult(result,row.Id);
        ProcessInstance instance = [SELECT Status,TargetObjectId,SubmittedById FROM ProcessInstance WHERE Id=:result.getInstanceId()];
        observed.put('instanceStatus',instance.Status);
        observed.put('instanceTargetMatches',instance.TargetObjectId == row.Id);
        observed.put('submitterMatches',instance.SubmittedById == UserInfo.getUserId());
        List<ProcessInstanceStep> steps = [SELECT StepStatus,Comments,ActorId,OriginalActorId FROM ProcessInstanceStep WHERE ProcessInstanceId=:instance.Id];
        List<Object> stepRows = new List<Object>();
        for (ProcessInstanceStep step : steps) {
            stepRows.add(new Map<String,Object>{'status'=>step.StepStatus,'comments'=>step.Comments,'actorMatches'=>step.ActorId==UserInfo.getUserId(),'originalActorMatches'=>step.OriginalActorId==UserInfo.getUserId()});
        }
        observed.put('steps',stepRows);
        observed.put('workitems',[SELECT COUNT() FROM ProcessInstanceWorkitem WHERE ProcessInstanceId=:instance.Id]);
        return new Map<String,Object>{'success'=>observed};
    }
    private static Map<String,Object> projectResult(Approval.ProcessResult result, Id recordId) {
        List<Object> errors = new List<Object>();
        if (result.getErrors() != null) for (Database.Error problem : result.getErrors()) {
            errors.add(new Map<String,Object>{'status'=>String.valueOf(problem.getStatusCode()),'message'=>normalize(problem.getMessage(),recordId),'fields'=>problem.getFields()});
        }
        return new Map<String,Object>{'kind'=>'result','success'=>result.isSuccess(),'entityNull'=>result.getEntityId()==null,'entityMatches'=>recordId!=null && result.getEntityId()==recordId,'instanceNull'=>result.getInstanceId()==null,'status'=>result.getInstanceStatus(),'newWorkitems'=>result.getNewWorkitemIds()==null ? null : result.getNewWorkitemIds().size(),'actorCount'=>result.getActorIds()==null ? null : result.getActorIds().size(),'errors'=>errors};
    }
    private static String normalize(String message,Id recordId) {
        if (recordId == null) return message;
        return message.replace(String.valueOf(recordId),'<record>').replace(String.valueOf(recordId).substring(0,15),'<record>');
    }
}
