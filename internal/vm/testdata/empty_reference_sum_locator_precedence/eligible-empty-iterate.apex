Map<String,Object> observed=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
Set<Id> keys=new Set<Id>();
observed.put('boundSize',keys.size());observed.put('phase','create');
String text='SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :keys AND Amount > 200 GROUP BY AccountId';
Database.QueryLocator handle=Database.getQueryLocator(text);
observed.put('created',handle!=null);
observed.put('phase','iterate');Database.QueryLocatorIterator iterator=handle.iterator();observed.put('hasNext',iterator.hasNext());
observed.put('phase','complete');
}catch(Exception issue){observed.put('exceptionType',issue.getTypeName());observed.put('exceptionMessage',issue.getMessage());}
System.debug('PRECEDENCE_OBSERVATION|eligible-empty-iterate|'+JSON.serialize(observed));
