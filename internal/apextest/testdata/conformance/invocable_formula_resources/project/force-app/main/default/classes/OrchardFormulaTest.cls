@IsTest
public class OrchardFormulaTest {
    static Invocable.Action.Result invoke(Case c,Case prior){
        Invocable.Action a=Invocable.Action.createCustomAction('flow','OrchardFormulaResources');
        a.setInvocations(new List<Map<String,Object>>{new Map<String,Object>{'record'=>c,'recordPrior'=>prior}});
        Invocable.Action.Result r=a.invoke()[0];System.assert(r.isSuccess(),String.valueOf(r.getErrors()));return r;
    }
    @IsTest static void observe(){
        Account a=new Account(Name='Garden');insert a;
        Case c=new Case(Subject='Garden',Status='New',Origin='Phone',AccountId=a.Id);insert c;
        String lastName=[SELECT LastName FROM User WHERE Id=:UserInfo.getUserId()].LastName;
        Invocable.Action.Result r=invoke(c,new Case(Subject='before'));
        Task t=[SELECT Subject,Description FROM Task WHERE WhatId=:c.Id];
        Map<String,String> rows=new Map<String,String>{'formula'=>String.valueOf(t.Subject=='Garden / before / '+lastName),'output'=>String.valueOf(r.getOutputParameters().get('message')==t.Subject),'template'=>t.Description};
        invoke(new Case(),null);
        c.Subject='different';invoke(c,null);
        rows.put('tasks',String.valueOf([SELECT count() FROM Task]));
        System.assert(false,'OBSERVED|observe|'+JSON.serialize(rows));
    }
}
