Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
User u=new User(ProfileId=p.Id,Alias='owner',LastName='ProbeSharingOwner',Username='sharing'+Crypto.getRandomLong()+'@example.invalid',Email='probe@example.invalid',TimeZoneSidKey='GMT',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'); insert u;
