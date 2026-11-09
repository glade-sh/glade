Account a=[SELECT Id FROM Account WHERE Name='B6 Other Owner' LIMIT 1]; insert new Contact(LastName='B6 Visible Parent',AccountId=a.Id);
