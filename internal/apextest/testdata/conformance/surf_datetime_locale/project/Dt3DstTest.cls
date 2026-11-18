@IsTest
private class Dt3DstTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  for (User u : DtSupport.zones()) {
   System.runAs(u) {
    List<String> r = new List<String>();
    Datetime a = Datetime.newInstance(2026, 3, 8, 2, 30, 0);
    Datetime b = Datetime.newInstance(2026, 11, 1, 1, 30, 0);
    Datetime c = Datetime.newInstance(2026, 3, 29, 1, 30, 0);
    Datetime d = Datetime.newInstance(2026, 10, 25, 1, 30, 0);
    r.add('a=' + DtSupport.stamp(a));
    r.add('ah=' + DtSupport.stamp(a.addHours(1)));
    r.add('b=' + DtSupport.stamp(b));
    r.add('bh=' + DtSupport.stamp(b.addHours(1)));
    r.add('c=' + DtSupport.stamp(c));
    r.add('d=' + DtSupport.stamp(d));
    rows.add(DtSupport.tag(u) + '={' + String.join(r, ',') + '}');
   }
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
