public class E32BA62LegacyObserver {
    public class MissingOutputException extends Exception {}
    public static String stage='disabled';
    public static String graph='E32BA62LegacyName';
    public static Boolean cloneInput=false;
    public static Boolean requireOutput=false;
    public static Boolean attachError=false;
    public static Boolean observeCreated=false;
    public static Boolean rejectCreated=false;
    public static Integer triggerCalls=0;
    public static Integer createdBefore=0;
    public static Integer createdAfter=0;
    public static Set<Id> createdIds=new Set<Id>();
    public static List<Object> frames=new List<Object>();
    public static Account lastOutput;
    public static void reset(){stage='disabled';cloneInput=false;requireOutput=false;attachError=false;observeCreated=false;rejectCreated=false;triggerCalls=0;createdBefore=0;createdAfter=0;createdIds.clear();frames.clear();lastOutput=null;}
    public static String clean(String s){
        if(s==null)return null;
        return s.replaceAll('https?://[^\\s"\'<>]+','[host]').replaceAll('\\b[0-9A-Za-z]{15}(?:[0-9A-Za-z]{3})?\\b','[id]').replaceAll('[0-9a-fA-F]{8}-[0-9a-fA-F-]{27,}','[guid]');
    }
    public static List<Object> snap(Account a,Id origin){
        if(a==null)return null;
        return new List<Object>{a.Id==null?'null':(a.Id==origin?'same':'other'),a.Name,a.Description,a.Industry};
    }
    public static Map<String,Object> invoke(Account original,String flowName,String parameter,Boolean runFault){
        Id origin=original.Id;
        Map<String,Object> frame=new Map<String,Object>{'originalBefore'=>snap(original,origin),'flowFired'=>false};
        frames.add(frame);
        Account inputRecord=cloneInput?(Account)original.clone(true,true,false,false):original;
        frame.put('inputBefore',snap(inputRecord,origin));
        try{
            Invocable.Action action=Invocable.Action.createCustomAction('flow',flowName);
            action.setInvocationParameter(parameter,inputRecord);
            if(runFault)action.setInvocationParameter('runFault',true);
            List<Invocable.Action.Result> results=action.invoke();
            frame.put('resultCount',results.size());
            if(results.size()!=1){frame.put('fixtureError','result-count');return frame;}
            Invocable.Action.Result result=results[0];
            frame.put('isSuccess',result.isSuccess());
            List<Object> errors=new List<Object>();
            Boolean reachedFault=false;
            for(Invocable.Action.Error err:result.getErrors()){
                String message=clean(err.getMessage());
                errors.add(new List<Object>{err.getCode(),message});
                if(message!=null && (message.toLowerCase().contains('required_field_missing') || message.toLowerCase().contains('required fields are missing')) && message.contains('Name'))reachedFault=true;
            }
            frame.put('errors',errors);
            Map<String,Object> outputs=result.getOutputParameters();
            Boolean outputPresent=outputs!=null && outputs.containsKey(parameter);
            frame.put('outputPresent',outputPresent);
            List<String> outputKeys=new List<String>();if(outputs!=null)outputKeys.addAll(outputs.keySet());outputKeys.sort();frame.put('outputKeys',outputKeys);
            Account outputRecord=outputPresent?(Account)outputs.get(parameter):null;lastOutput=outputRecord;
            frame.put('output',snap(outputRecord,origin));
            frame.put('fired',outputs!=null && outputs.get('fired')==true);
            frame.put('flowFired',(outputs!=null && outputs.get('fired')==true) || reachedFault);
            frame.put('originalAfter',snap(original,origin));
            frame.put('inputAfter',snap(inputRecord,origin));
            if(requireOutput && result.isSuccess() && outputRecord==null){frame.put('callerException','MissingOutputException');throw new MissingOutputException('Synthetic handler requires output record');}
            if(attachError && !result.isSuccess()){
                String message=errors.isEmpty()?'Synthetic handler received no error':String.valueOf(((List<Object>)errors[0])[1]);
                original.addError(message);frame.put('callerAddError',true);
            }
            if(stage.startsWith('before') && !stage.endsWith('Delete') && outputRecord!=null){original.Name=outputRecord.Name;original.Description=outputRecord.Description;}
            frame.put('originalAfterCaller',snap(original,origin));
            return frame;
        }catch(Exception e){frame.put('exceptionType',e.getTypeName());frame.put('exceptionMessage',clean(e.getMessage()));throw e;}
    }
    public static void handle(List<Account> records,Boolean isBefore,Boolean isInsert,String currentStage){
        for(Account original:records){
            if(observeCreated && original.Description=='E32 created'){
                if(isBefore)createdBefore++;else{createdAfter++;if(original.Id!=null)createdIds.add(original.Id);}
                if(rejectCreated && isBefore)original.addError('Synthetic Flow-created record rejected');
            }
            if(currentStage==stage && original.Name=='E32 Original' && original.Description=='E32 seed'){triggerCalls++;invoke(original,graph,'record',graph.endsWith('Fault'));}
        }
    }
    public static Map<String,Object> triggerRun(String requestedStage,String flowName,Boolean cloned,Boolean needOutput,Boolean addCallerError,Boolean partialUpdate){
        reset();
        Account subject=new Account(Name='E32 Original',Description='E32 seed',Industry='Technology');
        if(!requestedStage.endsWith('Insert'))insert subject;
        stage=requestedStage;graph=flowName;cloneInput=cloned;requireOutput=needOutput;attachError=addCallerError;
        Id origin=subject.Id;
        Map<String,Object> payload=new Map<String,Object>();
        try{
            if(requestedStage.endsWith('Insert'))insert subject;
            else if(requestedStage.endsWith('Update')){
                if(partialUpdate){Database.SaveResult saved=Database.update(subject,false);payload.put('dmlSuccess',saved.isSuccess());payload.put('saveResultId',saved.getId()==null?'null':(saved.getId()==subject.Id?'same':'other'));List<Object> saveErrors=new List<Object>();for(Database.Error err:saved.getErrors())saveErrors.add(new List<Object>{String.valueOf(err.getStatusCode()),clean(err.getMessage()),err.getFields()});payload.put('saveResultErrors',saveErrors);}
                else update subject;
            }
            else delete subject;
            if(!partialUpdate)payload.put('dmlSuccess',true);
        }catch(Exception e){payload.put('dmlSuccess',false);payload.put('exceptionType',e.getTypeName());payload.put('exceptionMessage',clean(e.getMessage()));}
        stage='disabled';
        origin=subject.Id;
        List<Account> dbRows=new List<Account>();
        if(origin!=null)dbRows=[SELECT Id,Name,Description,Industry FROM Account WHERE Id=:origin];
        payload.put('databaseCount',dbRows.size());payload.put('database',dbRows.isEmpty()?null:snap(dbRows[0],origin));
        payload.put('triggerCalls',triggerCalls);payload.put('frames',frames);
        Boolean fired=!frames.isEmpty() && ((Map<String,Object>)frames[0]).get('flowFired')==true;
        payload.put('flowFired',fired);
        Set<Id> ownedIds=new Set<Id>();if(origin!=null)ownedIds.add(origin);
        Integer outsideCount=[SELECT COUNT() FROM Account WHERE Name='E32 Original' AND Id NOT IN :ownedIds];
        if(outsideCount!=0)payload.put('fixtureError','out-of-scope-owned-records');
        if(!dbRows.isEmpty())delete dbRows;
        return payload;
    }
    public static Map<String,Object> direct(String flowName,String parameter){
        reset();Account original=new Account(Name='E32 Original',Description='E32 seed',Industry='Technology');
        insert original;Id origin=original.Id;
        Map<String,Object> result;
        try{result=invoke(original,flowName,parameter,false);}catch(Exception e){result=frames.isEmpty()?new Map<String,Object>():((Map<String,Object>)frames[0]);result.put('exceptionType',e.getTypeName());result.put('exceptionMessage',clean(e.getMessage()));result.put('flowFired',false);}
        Account databaseBefore=[SELECT Id,Name,Description,Industry FROM Account WHERE Id=:origin];result.put('databaseBeforeWriteBack',snap(databaseBefore,origin));
        result.put('writeBackAttempted',lastOutput!=null);
        if(lastOutput!=null){try{update lastOutput;result.put('writeBackSuccess',true);}catch(Exception e){result.put('writeBackSuccess',false);result.put('writeBackExceptionType',e.getTypeName());result.put('writeBackExceptionMessage',clean(e.getMessage()));}}
        List<Account> owned=[SELECT Id,Name,Description,Industry FROM Account WHERE Id=:origin];
        result.put('databaseAfterWriteBack',owned.isEmpty()?null:snap(owned[0],origin));
        Integer outsideCount=[SELECT COUNT() FROM Account WHERE Name='E32 Original' AND Id!=:origin];if(outsideCount!=0)result.put('fixtureError','out-of-scope-owned-records');
        delete owned;return result;
    }
    public static Map<String,Object> createRun(String flowName,Boolean rejectRecord){
        reset();observeCreated=true;rejectCreated=rejectRecord;
        Invocable.Action action=Invocable.Action.createCustomAction('flow',flowName);
        Map<String,Object> payload=new Map<String,Object>();Boolean nameFault=false;
        try{
            List<Invocable.Action.Result> results=action.invoke();payload.put('resultCount',results.size());
            if(results.size()==1){
                Invocable.Action.Result result=results[0];payload.put('isSuccess',result.isSuccess());
                List<Object> errors=new List<Object>();for(Invocable.Action.Error err:result.getErrors()){String message=clean(err.getMessage());errors.add(new List<Object>{err.getCode(),message});if(message!=null && (message.toLowerCase().contains('required_field_missing') || message.toLowerCase().contains('required fields are missing')) && message.contains('Name'))nameFault=true;}
                payload.put('errors',errors);Map<String,Object> outputs=result.getOutputParameters();List<String> outputKeys=new List<String>();if(outputs!=null)outputKeys.addAll(outputs.keySet());outputKeys.sort();payload.put('outputKeys',outputKeys);payload.put('fired',outputs!=null && outputs.get('fired')==true);
            }
        }catch(Exception e){payload.put('exceptionType',e.getTypeName());payload.put('exceptionMessage',clean(e.getMessage()));}
        observeCreated=false;rejectCreated=false;
        List<Account> created=[SELECT Id,Name,Description,Industry FROM Account WHERE Id IN :createdIds];
        payload.put('createdBefore',createdBefore);payload.put('createdAfter',createdAfter);payload.put('databaseCount',created.size());
        payload.put('flowFired',createdBefore>0 || (flowName.endsWith('CreateMissingName') && nameFault));List<Object> snapshots=new List<Object>();for(Account a:created)snapshots.add(snap(a,a.Id));payload.put('database',snapshots);
        Integer outsideCount=[SELECT COUNT() FROM Account WHERE Name='E32 Flow Created' AND Id NOT IN :createdIds];if(outsideCount!=0)payload.put('fixtureError','out-of-scope-owned-records');
        if(!created.isEmpty())delete created;
        return payload;
    }
}
