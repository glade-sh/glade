@IsTest
private class Dt7NewInstanceTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  for (User u : DtSupport.zones()) {
   System.runAs(u) {
    Datetime local = Datetime.newInstance(2026, 3, 8, 10, 30, 0);
    List<String> r = new List<String>();
    r.add('epoch=' + local.getTime());
    r.add('gmt=' + local.formatGmt('yyyy-MM-dd HH:mm:ss'));
    r.add('local=' + local.format('yyyy-MM-dd HH:mm:ss z'));
    r.add('date=' + local.date());
    r.add('addDay=' + local.addDays(1).formatGmt('yyyy-MM-dd HH:mm:ss'));
    r.add('addHour=' + local.addHours(-1).formatGmt('yyyy-MM-dd HH:mm:ss'));
    r.add('addMin=' + local.addMinutes(60).formatGmt('yyyy-MM-dd HH:mm:ss'));
    rows.add(DtSupport.tag(u) + '={' + String.join(r, ',') + '}');
   }
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
