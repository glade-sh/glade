@IsTest(SeeAllData=false) private class GroupedHandle4Probe{@IsTest static void observe(){
Map<String,Object> payload=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
Set<Id> keys=new Set<Id>();
Account parent=new Account(Name='Synthetic parent');insert parent;
Opportunity child=new Opportunity(Name='Synthetic child',StageName='Prospecting',CloseDate=Date.newInstance(2024,2,3),Amount=500,AccountId=parent.Id);insert child;keys.add(parent.Id);
delete child;delete parent;
payload.put('boundSize',keys.size());payload.put('phase','create');
String text='SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :keys AND Amount > 200 GROUP BY AccountId';
Database.QueryLocator handle=Database.getQueryLocator(text);
payload.put('created',handle!=null);
payload.put('phase','iterate');
Database.QueryLocatorIterator iterator=handle.iterator();Boolean more=iterator.hasNext();payload.put('hasNext',more);if(more){iterator.next();payload.put('nextRead',true);}
payload.put('phase','complete');
}catch(Exception issue){payload.put('exceptionType',issue.getTypeName());payload.put('exceptionMessage',issue.getMessage());}
System.assert(false,'HANDLE_OBSERVATION|sum-deleted-iterate|'+JSON.serialize(payload));
}}
