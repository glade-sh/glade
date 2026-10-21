@IsTest
public class OrchardObservationTest {
    public static Integer updates = 0;
    static Case invoke(String name, Case record, Case prior) {
        Invocable.Action action = Invocable.Action.createCustomAction('flow', name);
        action.setInvocations(new List<Map<String,Object>>{new Map<String,Object>{'record'=>record,'recordPrior'=>prior}});
        Invocable.Action.Result result = action.invoke()[0];
        System.assert(result.isSuccess(), String.valueOf(result.getErrors()));
        return (Case)result.getOutputParameters().get('record');
    }
    @IsTest static void lookup() {
        Account account = new Account(Name='Orchard'); insert account;
        Map<String,String> rows = new Map<String,String>();
        rows.put('found', invoke('OrchardLookup',new Case(AccountId=account.Id),null).Subject);
        rows.put('missing', invoke('OrchardLookup',new Case(),null).Subject);
        System.assert(false,'OBSERVED|lookup|'+JSON.serialize(rows));
    }
    @IsTest static void updateRecord() {
        Case record = new Case(Subject='start',Status='New',Origin='Phone'); insert record;
        updates=0;
        invoke('OrchardUpdate',record,null);
        Case saved=[SELECT Description FROM Case WHERE Id=:record.Id];
        System.assert(false,'OBSERVED|updateRecord|'+JSON.serialize(new Map<String,String>{'description'=>saved.Description,'updates'=>String.valueOf(updates)}));
    }
    @IsTest static void createRecord() {
        Case record = new Case(Subject='current',Status='New',Origin='Phone'); insert record;
        invoke('OrchardCreate',record,new Case(Subject='prior'));
        Task task=[SELECT Subject,WhatId FROM Task WHERE WhatId=:record.Id];
        System.assert(false,'OBSERVED|createRecord|'+JSON.serialize(new Map<String,String>{'subject'=>task.Subject,'related'=>String.valueOf(task.WhatId==record.Id)}));
    }
    @IsTest static void emptyString() {
        Case record = new Case(ObservationText__c='');
        System.assert(false,'OBSERVED|emptyString|'+JSON.serialize(new Map<String,String>{'isNull'=>String.valueOf(record.ObservationText__c==null),'length'=>String.valueOf(record.ObservationText__c==null ? -1 : record.ObservationText__c.length())}));
    }
}
