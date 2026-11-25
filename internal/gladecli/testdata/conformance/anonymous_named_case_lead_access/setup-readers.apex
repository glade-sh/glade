Id c = [SELECT Id FROM Contact WHERE LastName='Sharing Probe Contact' LIMIT 1].Id;
Map<String, String> aliasByProfile = new Map<String,String>{'Standard User'=>'rdstd','Customer Community User'=>'rdcc'};
for (Profile p : [SELECT Id, Name FROM Profile WHERE Name IN :aliasByProfile.keySet()]) {
    String a = aliasByProfile.get(p.Name);
    insert new User(Alias=a, Email=a+'@example.com', EmailEncodingKey='UTF-8', LastName='Sharing Probe Reader', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=p.Id, ContactId=a=='rdstd'?null:c, Username=a+'.'+Crypto.getRandomLong()+'@example.com');
}
