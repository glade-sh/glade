@IsTest
private class Dt4UnitTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  for (User u : new List<User>{ DtSupport.zones()[0], DtSupport.zones()[2] }) {
   System.runAs(u) {
    List<String> r = new List<String>();
    r.add('jan31p1=' + Date.newInstance(2026, 1, 31).addMonths(1));
    r.add('mar31m1=' + Date.newInstance(2026, 3, 31).addMonths(-1));
    r.add('feb29p12=' + Date.newInstance(2024, 2, 29).addMonths(12));
    r.add('aug31p1=' + Date.newInstance(2026, 8, 31).addMonths(1));
    r.add('dec31p1=' + Date.newInstance(2026, 12, 31).addMonths(1));
    r.add('dayAcrossDst=' + Date.newInstance(2026, 3, 7).daysBetween(Date.newInstance(2026, 3, 9)));
    r.add('daysNeg=' + Date.newInstance(2026, 3, 9).daysBetween(Date.newInstance(2026, 3, 7)));
    rows.add(DtSupport.tag(u) + '={' + String.join(r, ',') + '}');
   }
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
