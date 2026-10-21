Map<String,Object> observed=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
Set<Id> keys=new Set<Id>();
observed.put('boundSize',keys.size());observed.put('phase','create');
String text='SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :keys AND MissingCriterion = \'Synthetic\' GROUP BY AccountId';
Database.Cursor handle=Database.getCursor(text);
observed.put('created',handle!=null);
observed.put('phase','complete');
}catch(Exception issue){observed.put('exceptionType',issue.getTypeName());observed.put('exceptionMessage',issue.getMessage());}
System.debug('PRECEDENCE_OBSERVATION|cursor-missing|'+JSON.serialize(observed));
