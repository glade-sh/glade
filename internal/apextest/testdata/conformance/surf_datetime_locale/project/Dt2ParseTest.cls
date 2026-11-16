@IsTest
private class Dt2ParseTest {
 private static String e(List<String> rows, String id, String body) {
  try {
   Datetime d = Datetime.parse(body);
   rows.add(id + '=' + d.getTime());
  } catch (Exception x) {
   rows.add(id + '=' + x.getTypeName() + '|' + x.getMessage());
  }
  return '';
 }
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  for (User u : DtSupport.quartet()) {
   System.runAs(u) {
    List<String> r = new List<String>();
    e(r, 'p', '2026-03-08 10:30:00');
    e(r, 'pz', '2026-03-08T10:30:00.000Z');
    e(r, 'po', '2026-03-08T10:30:00+01:00');
    try { r.add('v=' + Datetime.valueOf('2026-03-08 10:30:00').getTime()); } catch (Exception x) { r.add('v=' + x.getTypeName() + '|' + x.getMessage()); }
    try { r.add('vg=' + Datetime.valueOfGmt('2026-03-08 10:30:00').getTime()); } catch (Exception x) { r.add('vg=' + x.getTypeName() + '|' + x.getMessage()); }
    try { r.add('dp=' + Date.parse('3/8/2026').daysBetween(Date.newInstance(2026, 1, 1))); } catch (Exception x) { r.add('dp=' + x.getTypeName() + '|' + x.getMessage()); }
    rows.add(DtSupport.tag(u) + '={' + String.join(r, ',') + '}');
   }
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
