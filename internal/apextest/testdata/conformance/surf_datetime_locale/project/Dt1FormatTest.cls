@IsTest
private class Dt1FormatTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  Datetime a = Datetime.newInstanceGmt(2026, 3, 8, 10, 30, 0);
  Date d = Date.newInstance(2026, 3, 8);
  for (User u : DtSupport.quartet()) {
   System.runAs(u) {
    rows.add(DtSupport.tag(u) + '=' + a.format('yyyy-MM-dd HH:mm:ss z Z')
     + '|g=' + a.formatGmt('yyyy-MM-dd HH:mm:ss')
     + '|L=' + a.formatLong()
     + '|D=' + d.format());
   }
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
