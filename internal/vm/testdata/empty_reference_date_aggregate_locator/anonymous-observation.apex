class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
class DateAggregateLocatorObservation {
    public Map<String,Object> capture(String shape) {
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
    private Map<String,Object> observe(Set<Id> parents,String aggregate,String criterion) {
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
 public Map<String,Object> negative() {

  Account parent=new Account(Name='Owned negative control',NumberOfEmployees=3);insert parent;
  Set<Id> owned=new Set<Id>{parent.Id};
  String query='SELECT NumberOfEmployees,COUNT(Id) n FROM Account WHERE Id IN :owned GROUP BY NumberOfEmployees';
  Map<String,Object> answer=new Map<String,Object>{'shape'=>'negative-control','boundSize'=>owned.size(),'iterator'=>new Map<String,Object>{'attempted'=>false}};
  Database.QueryLocator handle;
  try { handle=Database.getQueryLocator(query);answer.put('construction',new Map<String,Object>{'attempted'=>true,'created'=>handle!=null,'queryPreserved'=>handle.getQuery()==query}); }
  catch(Exception issue) {answer.put('construction',new Map<String,Object>{'attempted'=>true,'type'=>issue.getTypeName(),'message'=>issue.getMessage()});}
  if(handle!=null) {
   try {Database.QueryLocatorIterator rows=handle.iterator();Map<String,Object> stage=new Map<String,Object>{'attempted'=>true,'created'=>true};answer.put('iterator',stage);
    try {Boolean more=rows.hasNext();stage.put('hasNext',more);if(more)stage.put('first',JSON.serialize(rows.next()));}
    catch(Exception issue){stage.put('iterationType',issue.getTypeName());stage.put('iterationMessage',issue.getMessage());}
   } catch(Exception issue){answer.put('iterator',new Map<String,Object>{'attempted'=>true,'type'=>issue.getTypeName(),'message'=>issue.getMessage()});}
  }
  return answer;
 }
}

DateAggregateLocatorObservation observation=new DateAggregateLocatorObservation();
pq.out('D001',JSON.serialize(observation.capture('maximum-date')));
pq.out('D002',JSON.serialize(observation.capture('maximum-date-or-decimal')));
pq.out('D003',JSON.serialize(observation.capture('count-date')));
pq.out('D004',JSON.serialize(observation.negative()));
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
