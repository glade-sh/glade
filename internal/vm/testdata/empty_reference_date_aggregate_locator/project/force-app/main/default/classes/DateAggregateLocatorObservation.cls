public class DateAggregateLocatorObservation {
    public static Map<String,Object> capture(String shape) {
        Map<String,Object> answer=new Map<String,Object>{'shape'=>shape};
        try {
            Account parent=new Account(Name='Owned aggregate parent');insert parent;
            Opportunity child=new Opportunity(Name='Owned aggregate child',AccountId=parent.Id,CloseDate=Date.today().addDays(-2),StageName='Prospecting',Amount=150);insert child;
            answer.put('seededMatchingRows',1);
            String aggregate=shape=='count-date'?'COUNT(CloseDate)':'MAX(CloseDate)';
            String criterion=shape=='count-date'?'Amount < 200':(shape=='maximum-date-or-decimal'?'CloseDate < TODAY OR Amount > 100':'CloseDate < TODAY');
            answer.put('empty',observe(new Set<Id>(),aggregate,criterion));
            answer.put('matchingControl',observe(new Set<Id>{parent.Id},aggregate,criterion));
        } catch(Exception issue) { answer.put('setupError',new Map<String,Object>{'type'=>issue.getTypeName(),'message'=>issue.getMessage()}); }
        return answer;
    }
    private static Map<String,Object> observe(Set<Id> parents,String aggregate,String criterion) {
        parents=parents.clone();
        String query='SELECT AccountId,'+aggregate+' resultValue FROM Opportunity WHERE AccountId IN :parents AND ('+criterion+') GROUP BY AccountId';
        Map<String,Object> answer=new Map<String,Object>{'boundSize'=>parents.size(),'construction'=>new Map<String,Object>{'attempted'=>true},'iterator'=>new Map<String,Object>{'attempted'=>false}};
        Database.QueryLocator handle;
        try { handle=Database.getQueryLocator(query);answer.put('construction',new Map<String,Object>{'attempted'=>true,'created'=>handle!=null,'queryPreserved'=>handle.getQuery()==query}); }
        catch(Exception issue) { answer.put('construction',new Map<String,Object>{'attempted'=>true,'type'=>issue.getTypeName(),'message'=>issue.getMessage()});return answer; }
        try {
            Database.QueryLocatorIterator rows=handle.iterator();
            Map<String,Object> stage=new Map<String,Object>{'attempted'=>true,'created'=>true};answer.put('iterator',stage);
            try { Boolean more=rows.hasNext();stage.put('hasNext',more);if(more){stage.put('first',JSON.serialize(rows.next()));} }
            catch(Exception issue) {stage.put('iterationType',issue.getTypeName());stage.put('iterationMessage',issue.getMessage());}
        } catch(Exception issue) {answer.put('iterator',new Map<String,Object>{'attempted'=>true,'type'=>issue.getTypeName(),'message'=>issue.getMessage()});}
        return answer;
    }
}
