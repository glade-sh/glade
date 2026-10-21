Map<String,Object> payload=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
Set<Id> keys=new Set<Id>();
payload.put('boundSize',keys.size());payload.put('phase','create');
String text='SELECT SUM(Amount) total FROM Opportunity WHERE AccountId IN :keys';
Database.QueryLocator handle=Database.getQueryLocator(text);
payload.put('created',handle!=null);
payload.put('phase','iterate');
Database.QueryLocatorIterator iterator=handle.iterator();Boolean more=iterator.hasNext();payload.put('hasNext',more);if(more){iterator.next();payload.put('nextRead',true);}
payload.put('phase','complete');
}catch(Exception issue){payload.put('exceptionType',issue.getTypeName());payload.put('exceptionMessage',issue.getMessage());}
System.debug('HANDLE_OBSERVATION|ungrouped-sum-empty|'+JSON.serialize(payload));
