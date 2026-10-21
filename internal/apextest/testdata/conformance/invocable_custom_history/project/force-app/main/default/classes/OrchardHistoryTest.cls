@IsTest
public class OrchardHistoryTest {
    public static Integer created = 0;
    static Invocable.Action.Result invoke(Case c,Case prior){
        Invocable.Action a=Invocable.Action.createCustomAction('flow','OrchardHistoryCreate');
        a.setInvocations(new List<Map<String,Object>>{new Map<String,Object>{'record'=>c,'recordPrior'=>prior}});
        Invocable.Action.Result r=a.invoke()[0];System.assert(r.isSuccess(),String.valueOf(r.getErrors()));return r;
    }
    @IsTest static void observe(){
        Account a=new Account(Name='Garden');insert a;
        Case c=new Case(Subject='Garden',Status='New',Origin='Phone',AccountId=a.Id);insert c;
        String lastName=[SELECT LastName FROM User WHERE Id=:UserInfo.getUserId()].LastName;
        Invocable.Action.Result r=invoke(c,new Case(Subject='before'));
        JournalEntry__c t=[SELECT Name,Details__c,RelatedCase__c FROM JournalEntry__c WHERE RelatedCase__c=:c.Id WITH SYSTEM_MODE];
        Map<String,String> rows=new Map<String,String>{'formula'=>String.valueOf(t.Name=='Garden / before / '+lastName),'output'=>String.valueOf(r.getOutputParameters().get('message')==t.Name),'template'=>t.Details__c};
        invoke(new Case(),null);
        c.Subject='different';invoke(c,null);
        rows.put('entries',String.valueOf([SELECT count() FROM JournalEntry__c WITH SYSTEM_MODE]));
        rows.put('created',String.valueOf(created));
        rows.put('related',String.valueOf(t.RelatedCase__c == c.Id));
        System.assert(false,'OBSERVED|observe|'+JSON.serialize(rows));
    }
}
