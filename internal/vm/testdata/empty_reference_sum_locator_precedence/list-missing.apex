Map<String,Object> observed=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
List<Id> keys=new List<Id>();
observed.put('boundSize',keys.size());observed.put('phase','create');
String text='SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :keys AND MissingCriterion = \'Synthetic\' GROUP BY AccountId';
Database.QueryLocator handle=Database.getQueryLocator(text);
observed.put('created',handle!=null);
observed.put('phase','complete');
}catch(Exception issue){observed.put('exceptionType',issue.getTypeName());observed.put('exceptionMessage',issue.getMessage());}
System.debug('PRECEDENCE_OBSERVATION|list-missing|'+JSON.serialize(observed));
