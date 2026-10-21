Map<String,Object> observed=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
List<Account> seed=new List<Account>{new Account(Name='Synthetic alpha',NumberOfEmployees=0),new Account(Name='Synthetic beta',NumberOfEmployees=1),new Account(Name='Synthetic gamma',NumberOfEmployees=2),new Account(Name='Synthetic omega',NumberOfEmployees=null)};insert seed;Set<Id> keys=new Map<Id,Account>(seed).keySet();
observed.put('boundSize',keys.size());observed.put('phase','create');
String text='SELECT NumberOfEmployees,COUNT(Id) n FROM Account WHERE Id IN :keys GROUP BY NumberOfEmployees';
Database.QueryLocator handle=Database.getQueryLocator(text);
observed.put('created',handle!=null);
observed.put('phase','complete');
}catch(Exception issue){observed.put('exceptionType',issue.getTypeName());observed.put('exceptionMessage',issue.getMessage());}
System.debug('PRECEDENCE_OBSERVATION|matching-count|'+JSON.serialize(observed));
