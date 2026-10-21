Map<String,Object> payload=new Map<String,Object>{'phase'=>'setup','created'=>false,'exceptionType'=>null,'exceptionMessage'=>null};
try {
Set<Id> keys=new Set<Id>();
List<Account> seed=new List<Account>{new Account(Name='Synthetic alpha',NumberOfEmployees=0),new Account(Name='Synthetic beta',NumberOfEmployees=1),new Account(Name='Synthetic gamma',NumberOfEmployees=2),new Account(Name='Synthetic omega',NumberOfEmployees=null)};insert seed;keys=new Map<Id,Account>(seed).keySet();
payload.put('boundSize',keys.size());payload.put('phase','create');
String text='SELECT NumberOfEmployees,COUNT(Id) n FROM Account WHERE Id IN :keys GROUP BY NumberOfEmployees';
Database.QueryLocator handle=Database.getQueryLocator(text);
payload.put('created',handle!=null);
payload.put('phase','complete');
}catch(Exception issue){payload.put('exceptionType',issue.getTypeName());payload.put('exceptionMessage',issue.getMessage());}
System.debug('HANDLE_OBSERVATION|matching-count-control|'+JSON.serialize(payload));
