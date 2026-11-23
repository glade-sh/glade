Account a=new Account(Name='Probe Parent'); insert a;
Contact hit=new Contact(LastName='Hit',AccountId=a.Id); Contact miss=new Contact(LastName='Miss',AccountId=a.Id); insert new List<Contact>{hit,miss};
insert new Case(Subject='Probe',ContactId=hit.Id);
String result; try { List<SObject> rows=Database.query('SELECT Id, (SELECT Id FROM Contacts WHERE LastName != null AND Id IN (SELECT ContactId FROM Case WHERE ContactId != null) LIMIT 1), (SELECT Id FROM Opportunities LIMIT 1) FROM Account WHERE Name = \'Probe Parent\''); List<SObject> contacts=rows[0].getSObjects('Contacts'); List<SObject> opportunities=rows[0].getSObjects('Opportunities'); result='rows='+rows.size()+';contacts='+(contacts==null?0:contacts.size())+';opportunities='+(opportunities==null?0:opportunities.size()); } catch (Exception e) { result=e.getTypeName()+':'+e.getMessage(); }
System.debug('OBS:'+result);
