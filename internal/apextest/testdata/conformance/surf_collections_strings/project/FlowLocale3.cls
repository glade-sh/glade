@IsTest private class FlowLocale3 {
 static String p = '';
 static String run(String n) { try { Flow.Interview f = Flow.Interview.createInterview(n, new Map<String,Object>()); f.start(); return 'ok:' + String.valueOf(f.getVariableValue('Out')); } catch (Exception e) { return 'ex:' + e.getTypeName() + ':' + e.getMessage(); } }
 static User mk(String loc, String n) { User u = new User(Alias='fl'+n, Email='fl@example.invalid', EmailEncodingKey='UTF-8', LastName='Fl'+n, LanguageLocaleKey='en_US', LocaleSidKey=loc, ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id, TimeZoneSidKey='GMT', Username='fl'+n+String.valueOf(Crypto.getRandomLong())+'@example.invalid'); insert u; return u; }
 @IsTest static void t() {
  Integer i = 0;
  for (String loc : new List<String>{'it_IT','nl_NL','en_CA','en_AU','fr_CA','de_AT'}) {
   User u = mk(loc, String.valueOf(i++));
   System.runAs(u) { for (String n : new List<String>{'Fl_numbig','Fl_numbig_text','Fl_numfbig','Fl_datevar','Fl_datevar_text','Fl_numvar','Fl_negnum'}) p += '|' + loc + ' ' + n + '=' + run(n); }
  }
  System.assert(false, 'FlowLocale' + p);
 }
}
