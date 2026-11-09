Account a=[SELECT Id FROM Account WHERE Name='B6 First' LIMIT 1];
insert new List<Opportunity>{new Opportunity(Name='B6 Outside Quarter',AccountId=a.Id,StageName='Prospecting',CloseDate=Date.today().addMonths(6),Amount=50),new Opportunity(Name='B6 Zero Probability',AccountId=a.Id,StageName='Prospecting',CloseDate=Date.today(),Probability=0,Amount=25)};
