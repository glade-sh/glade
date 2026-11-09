Account a=[SELECT Id FROM Account WHERE Name='B6 First' LIMIT 1];
insert new List<Opportunity>{new Opportunity(Name='B6 Opportunity One',AccountId=a.Id,StageName='Prospecting',CloseDate=Date.today(),Amount=1234.5),new Opportunity(Name='B6 Opportunity Two',AccountId=a.Id,StageName='Prospecting',CloseDate=Date.today())};
