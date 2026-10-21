@IsTest(SeeAllData=false) private class LocatorCriteriaForm2Probe{
@IsTest static void observe(){
Map<String,Object> observed=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {Set<Id> keys=new Set<Id>();observed.put('boundSize',keys.size());observed.put('phase','create');construct(keys,observed);
}catch(Exception issue){observed.put('exceptionType',issue.getTypeName());observed.put('exceptionMessage',issue.getMessage());}
System.assert(false,'CRITERIA_OBSERVATION|date-or-amount|'+JSON.serialize(observed));
}
private static void construct(Set<Id> keys,Map<String,Object> observed){
String text='SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :keys AND (CloseDate < TODAY OR Amount > 100) GROUP BY AccountId';
Database.QueryLocator handle=Database.getQueryLocator(text);observed.put('created',handle!=null);observed.put('phase','complete');
}
}
