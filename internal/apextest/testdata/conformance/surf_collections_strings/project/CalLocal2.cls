@IsTest private class CalLocal2 {
 static String p = '';
 static void add(String k, Object v) { p += '|' + k + '=' + v; }
 @IsTest static void t() {
  User u = new User(Alias='cal', Email='cal@example.invalid', EmailEncodingKey='UTF-8', LastName='Cal', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id, TimeZoneSidKey='America/Los_Angeles', Username='cal'+String.valueOf(Crypto.getRandomLong())+'@example.invalid'); insert u;
  System.runAs(u) {
   Datetime a = Datetime.newInstance(1500,1,1,12,0,0); Datetime b = Datetime.newInstance(1500,2,28,20,0,0); Datetime c = Datetime.valueOf('1500-01-01 12:00:00');
   Integer i = 0;
   for (Datetime x : new List<Datetime>{a,b,c}) { i++; add('ms'+i, x.getTime()); add('valueOf'+i, String.valueOf(x)); add('concat'+i, '' + x); add('fmt'+i, x.format('yyyy-MM-dd HH:mm:ss')); add('ymd'+i, x.year() + '-' + x.month() + '-' + x.day() + ' doy=' + x.dayOfYear()); add('date'+i, x.date()); add('gmt'+i, x.formatGmt('yyyy-MM-dd HH:mm:ss')); add('dateGmt'+i, x.dateGmt() + ' doy=' + x.dayOfYearGmt()); }
   add('addYears100', a.addYears(100).format('yyyy-MM-dd HH:mm') + '/' + a.addYears(100).getTime());
   add('addMonths', a.addMonths(1200).format('yyyy-MM-dd HH:mm'));
   add('eq', a == c);
   add('long', a.formatLong());
   add('fmtz', a.format('yyyy-MM-dd HH:mm z'));
  }
  System.assert(false, 'CalLocal2' + p);
 }
}
