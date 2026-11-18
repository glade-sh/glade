@IsTest private class FlowDiff {
 static String p = '';
 static String run(String n) { try { Flow.Interview f = Flow.Interview.createInterview(n, new Map<String,Object>()); f.start(); return 'ok:' + String.valueOf(f.getVariableValue('Out')).replace('\n','<NL>').replace('\t','<TAB>'); } catch (Exception e) { return 'ex:' + e.getTypeName() + ':' + e.getMessage(); } }
 static User mk(String loc, String n) { User u = new User(Alias='fd'+n, Email='fd@example.invalid', EmailEncodingKey='UTF-8', LastName='Fd'+n, LanguageLocaleKey='en_US', LocaleSidKey=loc, ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id, TimeZoneSidKey='GMT', Username='fd'+n+String.valueOf(Crypto.getRandomLong())+'@example.invalid'); insert u; return u; }
 @IsTest static void t() {
  Integer i = 0;
  for (String loc : new List<String>{'ru_RU','pt_PT','de','en_US','de_DE'}) {
   User u = mk(loc, String.valueOf(i++));
   System.runAs(u) { for (String n : new List<String>{'Fd_numexact','Fd_numtext','Fd_numnl','Fd_numtab','Fd_numamp','Fd_numq','Fd_dateexact','Fd_datetext','Fd_datenl','Fd_datetab','Fd_dateamp'}) p += '\n' + loc + ' ' + n + '=' + run(n); }
  }
  System.assert(false, 'FlowDiff' + p);
 }
}
