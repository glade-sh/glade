List<Object> rows=new List<Object>();
for(User u:[SELECT UserType,TimeZoneSidKey FROM User WHERE UserType IN ('AutomatedProcess','Guest') ORDER BY UserType]) rows.add(new List<String>{u.UserType,u.TimeZoneSidKey});
User current=[SELECT TimeZoneSidKey FROM User WHERE Id=:UserInfo.getUserId()];
rows.add(new List<String>{'running',current.TimeZoneSidKey,UserInfo.getTimeZone().getID(),[SELECT TimeZoneSidKey FROM Organization LIMIT 1].TimeZoneSidKey});
rows.add(new List<Object>{'boundary',TimeZone.getTimeZone('GMT').getOffset(Datetime.newInstanceGmt(2024,1,15,0,0,0)),TimeZone.getTimeZone('Asia/Tokyo').getOffset(Datetime.newInstanceGmt(2024,1,15,0,0,0))});
try { User u=new User(Username='synthetic.zone@example.invalid',LastName='Synthetic',Alias='ztest',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',ProfileId=UserInfo.getProfileId(),LocaleSidKey='en_US',LanguageLocaleKey='en_US',TimeZoneSidKey='UTC'); insert u; rows.add('unexpected success'); } catch(DmlException e) { rows.add(new List<String>{'reject',String.valueOf(e.getDmlType(0))}); }
System.debug('TZROW|'+JSON.serialize(rows));
