@IsTest(SeeAllData=false) private class GroupedHandle9Probe{@IsTest static void observe(){
Map<String,Object> payload=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
Set<Id> keys=new Set<Id>();
payload.put('boundSize',keys.size());payload.put('phase','create');
String text='SELECT AccountId,COUNT(Id) total FROM Opportunity WHERE AccountId IN :keys GROUP BY AccountId';
Database.QueryLocator handle=Database.getQueryLocator(text);
payload.put('created',handle!=null);
payload.put('phase','iterate');
Database.QueryLocatorIterator iterator=handle.iterator();Boolean more=iterator.hasNext();payload.put('hasNext',more);if(more){iterator.next();payload.put('nextRead',true);}
payload.put('phase','complete');
}catch(Exception issue){payload.put('exceptionType',issue.getTypeName());payload.put('exceptionMessage',issue.getMessage());}
System.assert(false,'HANDLE_OBSERVATION|count-empty|'+JSON.serialize(payload));
}}
