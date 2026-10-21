// Intentional assertion transports an observation from check-only execution.
@IsTest private class DateAggregateLocatorNegativeProbe {
 @IsTest static void observe() {
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
  System.assert(false,'DATE_AGGREGATE_OBSERVATION|negative-control|'+JSON.serialize(answer));
 }
}
