Account a=new Account(Name='Flow Type Probe A',NumberOfEmployees=41,AnnualRevenue=41,Type='Prospect');
Account b=new Account(Name='Flow Type Probe B',NumberOfEmployees=42,AnnualRevenue=42,Type='Other');
insert new List<Account>{a,b};
b=[SELECT Id,OwnerId,CreatedDate FROM Account WHERE Id=:b.Id];
Contact x=new Contact(LastName='Flow Type Probe A',Birthdate=Date.newInstance(2001,2,2));
Contact y=new Contact(LastName='Flow Type Probe B',Birthdate=Date.newInstance(2001,2,3));
insert new List<Contact>{x,y};
try { Flow.Interview f=Flow.Interview.createInterview('F67NotInDateDate',new Map<String,Object>{'values'=>new List<Date>{Date.newInstance(2001,2,3)}}); f.start(); System.debug('OBS|F67NotInDateDate|value|'+String.valueOf(f.getVariableValue('outcome'))); } catch(Exception e) { System.debug('OBS|F67NotInDateDate|value|ERROR:'+e.getTypeName()); }
