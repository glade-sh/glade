@IsTest
private class Dt6PatternTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  Datetime a = Datetime.newInstanceGmt(2026, 3, 8, 10, 30, 6);
  List<User> q = DtSupport.quartet();
  List<User> us = new List<User>{ q[0], q[2] };
  for (User u : us) {
   System.runAs(u) {
    List<String> r = new List<String>();
    for (String p : new List<String>{'E', 'EEEE', 'M', 'MMM', 'MMMM', 'a', 'K', 'k', 'S', 'SSS', 'u', 'D', 'X', 'XXX', 'Z', 'z', 'zzzz', 'G'}) {
     try { r.add(p + '=' + a.format(p)); } catch (Exception x) { r.add(p + '=' + x.getTypeName()); }
    }
    rows.add(DtSupport.tag(u) + '={' + String.join(r, ',') + '}');
   }
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
