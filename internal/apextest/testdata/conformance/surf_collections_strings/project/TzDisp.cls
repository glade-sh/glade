@IsTest private class TzDisp {
 static User mk(String tz, String n) { User u = new User(Alias='tzd'+n, Email='tzd@example.invalid', EmailEncodingKey='UTF-8', LastName='Tzd'+n, LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id, TimeZoneSidKey=tz, Username='tzd'+n+String.valueOf(Crypto.getRandomLong())+'@example.invalid'); insert u; return u; }
 static String row(Datetime d) { return d.getTime() + '=' + d.format('z') + '/' + d.format('zzzz') + '/' + d.format('Z') + '/' + d.format('XXX') + '/' + TimeZone.getTimeZone(UserInfo.getTimeZone().getID()).getOffset(d) + '/' + d.format('yyyy-MM-dd HH:mm:ss') + '/' + UserInfo.getTimeZone().getDisplayName(); }
 @IsTest static void t() {
  String p = 'tz';
  System.runAs(mk('America/Los_Angeles','la')) {
   for (Long ms : new List<Long>{ Datetime.newInstanceGmt(2006,3,20,20,0,0).getTime(), Datetime.newInstanceGmt(2006,4,10,20,0,0).getTime(), Datetime.newInstanceGmt(1999,7,1,20,0,0).getTime(), Datetime.newInstanceGmt(1905,6,15,20,0,0).getTime(), Datetime.newInstanceGmt(2007,3,12,20,0,0).getTime(), Datetime.newInstanceGmt(2026,7,1,20,0,0).getTime() }) p += '|LA ' + row(Datetime.newInstance(ms));
  }
  System.runAs(mk('Australia/Adelaide','ad')) {
   for (Long ms : new List<Long>{ Datetime.newInstanceGmt(2007,10,10,0,0,0).getTime(), Datetime.newInstanceGmt(1970,1,1,0,0,0).getTime(), Datetime.newInstanceGmt(1993,10,4,0,0,0).getTime(), Datetime.newInstanceGmt(2026,1,1,0,0,0).getTime() }) p += '|AD ' + row(Datetime.newInstance(ms));
  }
  System.runAs(mk('Asia/Kolkata','in')) {
   for (Long ms : new List<Long>{ Datetime.newInstanceGmt(1943,1,1,0,0,0).getTime(), Datetime.newInstanceGmt(1900,1,1,0,0,0).getTime(), Datetime.newInstanceGmt(2026,1,1,0,0,0).getTime() }) p += '|IN ' + row(Datetime.newInstance(ms));
  }
  System.assert(false, p);
 }
}
