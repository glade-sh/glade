@IsTest
private class Dt10AmbigTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  List<String> zones = new List<String>{'Australia/Sydney', 'Pacific/Auckland', 'Australia/Adelaide'};
  List<Integer> gapM = new List<Integer>{10, 9, 10};
  List<Integer> gapD = new List<Integer>{4, 27, 4};
  for (Integer i = 0; i < zones.size(); i++) {
   User u = DtSupport.mk('en_US', zones[i], 'am' + i, 'en_US');
   System.runAs(u) {
    List<String> r = new List<String>();
    Datetime x = Datetime.newInstance(2026, 4, 5, 2, 45, 0);
    Datetime y = Datetime.newInstance(2026, 4, 5, 2, 15, 0);
    Datetime v = Datetime.valueOf('2026-04-05 02:45:00');
    Datetime dt = Datetime.newInstance(Date.newInstance(2026, 4, 5), Time.newInstance(2, 45, 0, 0));
    Datetime pre = Datetime.newInstance(2026, 4, 5, 1, 45, 0);
    Datetime g = Datetime.newInstance(2026, gapM[i], gapD[i], 2, 30, 0);
    r.add('x=' + DtSupport.stamp(x));
    r.add('y=' + DtSupport.stamp(y));
    r.add('v=' + DtSupport.stamp(v));
    r.add('dt=' + DtSupport.stamp(dt));
    r.add('pre1=' + DtSupport.stamp(pre.addHours(1)));
    r.add('pre2=' + DtSupport.stamp(pre.addHours(2)));
    r.add('pre3=' + DtSupport.stamp(pre.addHours(3)));
    r.add('xm=' + DtSupport.stamp(x.addMinutes(30)));
    r.add('gap=' + DtSupport.stamp(g));
    r.add('gapv=' + DtSupport.stamp(Datetime.valueOf('2026-' + gapM[i] + '-' + gapD[i] + ' 02:30:00')));
    r.add('gaph=' + DtSupport.stamp(g.addHours(1)));
    rows.add(zones[i] + '={' + String.join(r, ',') + '}');
   }
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
