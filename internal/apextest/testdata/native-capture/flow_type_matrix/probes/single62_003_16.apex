Account a=new Account(Name='Flow Type Probe A',NumberOfEmployees=41,AnnualRevenue=41,Type='Prospect');
Account b=new Account(Name='Flow Type Probe B',NumberOfEmployees=42,AnnualRevenue=42,Type='Other');
insert new List<Account>{a,b};
b=[SELECT Id,OwnerId,CreatedDate FROM Account WHERE Id=:b.Id];
Contact x=new Contact(LastName='Flow Type Probe A',Birthdate=Date.newInstance(2001,2,2));
Contact y=new Contact(LastName='Flow Type Probe B',Birthdate=Date.newInstance(2001,2,3));
insert new List<Contact>{x,y};
try { Flow.Interview f=Flow.Interview.createInterview('T62TextString',new Map<String,Object>{'inputValue'=>'42'}); f.start(); SObject o=(SObject)f.getVariableValue('outputRecord'); System.debug('OBS|T62TextString|value|'+String.valueOf(o.get('Name'))); } catch(Exception e) { System.debug('OBS|T62TextString|value|ERROR:'+e.getTypeName()); }
