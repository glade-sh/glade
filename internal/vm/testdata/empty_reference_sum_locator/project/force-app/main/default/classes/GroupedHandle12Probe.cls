@IsTest(SeeAllData=false) private class GroupedHandle12Probe{@IsTest static void observe(){
Map<String,Object> payload=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
Set<Id> keys=new Set<Id>();
payload.put('boundSize',keys.size());payload.put('phase','create');
String text='SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :keys AND Amount > 200 GROUP BY AccountId';
Database.Cursor handle=Database.getCursor(text);
payload.put('created',handle!=null);
payload.put('phase','iterate');
Integer size=handle.getNumRecords();payload.put('numRecords',size);if(size>0){List<SObject> page=handle.fetch(0,Math.min(10,size));payload.put('fetched',page.size());}else{payload.put('fetched',0);}
payload.put('phase','complete');
}catch(Exception issue){payload.put('exceptionType',issue.getTypeName());payload.put('exceptionMessage',issue.getMessage());}
System.assert(false,'HANDLE_OBSERVATION|cursor-sum-empty|'+JSON.serialize(payload));
}}
