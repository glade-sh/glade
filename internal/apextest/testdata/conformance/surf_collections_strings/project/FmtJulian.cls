@IsTest private class FmtJulian {
 static String p = '';
 static void add(String k, Object v) { p += '|' + k + '=' + v; }
 @IsTest static void t() {
  User u = new User(Alias='fmj', Email='fmj@example.invalid', EmailEncodingKey='UTF-8', LastName='Fmj', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id, TimeZoneSidKey='GMT', Username='fmj'+String.valueOf(Crypto.getRandomLong())+'@example.invalid'); insert u;
  System.runAs(u) {
   Map<String,Datetime> ds = new Map<String,Datetime>{ 'j1500_0229' => Datetime.newInstanceGmt(1500,2,29,13,45,30), 'j1500_0301' => Datetime.newInstanceGmt(1500,3,1,13,45,30), 'j1500_0101' => Datetime.newInstanceGmt(1500,1,1,13,45,30), 'j1500_1231' => Datetime.newInstanceGmt(1500,12,31,13,45,30), 'j1582_1004' => Datetime.newInstanceGmt(1582,10,4,13,45,30), 'j1582_1015' => Datetime.newInstanceGmt(1582,10,15,13,45,30), 'g1600_0229' => Datetime.newInstanceGmt(1600,2,29,13,45,30), 'g2026_0301' => Datetime.newInstanceGmt(2026,3,1,13,45,30) };
   for (String k : new List<String>{'j1500_0229','j1500_0301','j1500_0101','j1500_1231','j1582_1004','j1582_1015','g1600_0229','g2026_0301'}) {
    Datetime d = ds.get(k);
    for (String f : new List<String>{'EEEE','EEE','E','u','D','DDD','w','ww','W','F','FF','yyyy-MM-dd','G','MMMM d, yyyy','EEEE, MMMM d, yyyy','yy-M-d'}) {
     try { add(k + '[' + f + ']', d.formatGmt(f)); } catch (Exception e) { add(k + '[' + f + ']', 'EX:' + e.getMessage()); }
    }
    add(k + '.format', d.format('EEE yyyy-MM-dd D w'));
    add(k + '.long', d.formatLong());
   }
  }
  System.assert(false, 'FmtJulian' + p);
 }
}
