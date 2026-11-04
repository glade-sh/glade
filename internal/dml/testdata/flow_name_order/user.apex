List<String> rows = new List<String>();
String suffix=String.valueOf(Datetime.now().getTime());
for (String first : new List<String>{'Nia','Other','Reject','NullProbe','NonNullProbe'}) {
 Savepoint sp=Database.setSavepoint();
 User u=new User(FirstName=first, LastName='Probe', Alias='probe', Email='probe@example.invalid', Username='nameprobe'+first+suffix+'@example.invalid', TimeZoneSidKey='America/Los_Angeles', LocaleSidKey='en_US', EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US', ProfileId=UserInfo.getProfileId());
 Database.SaveResult r=Database.insert(u,false); rows.add(String.valueOf(r.isSuccess()) + '|' + (r.isSuccess() ? '' : r.getErrors()[0].getMessage())); Database.rollback(sp);
}
for (String row : rows) System.debug('NAME_ORDER|user|' + row);
