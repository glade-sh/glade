Account row = new Account(FirstName='A',LastName='B',PersonEmail='person@example.com');
System.assertEquals('A',row.FirstName);System.assertEquals('B',row.LastName);System.assertEquals('person@example.com',row.PersonEmail);
Account business = new Account(Name='Evidence Business');insert business;
Account stored = [SELECT IsPersonAccount FROM Account WHERE Id=:business.Id];
System.assertEquals(false,stored.IsPersonAccount);