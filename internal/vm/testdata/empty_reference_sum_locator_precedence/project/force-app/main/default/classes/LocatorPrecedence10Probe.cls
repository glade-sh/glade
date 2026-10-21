@IsTest(SeeAllData=false) private class LocatorPrecedence10Probe{@IsTest static void observe(){
Map<String,Object> observed=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
Set<Id> keys=new Set<Id>();
observed.put('boundSize',keys.size());observed.put('phase','create');
String text='SELECT Id FROM Opportunity WHERE AccountId IN :keys AND MissingCriterion = \'Synthetic\'';
Database.QueryLocator handle=Database.getQueryLocator(text);
observed.put('created',handle!=null);
observed.put('phase','complete');
}catch(Exception issue){observed.put('exceptionType',issue.getTypeName());observed.put('exceptionMessage',issue.getMessage());}
System.assert(false,'PRECEDENCE_OBSERVATION|ordinary-missing|'+JSON.serialize(observed));
}}
