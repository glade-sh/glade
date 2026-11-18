@IsTest
private class Dt8GmtTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  User u = DtSupport.zones()[1];
  System.runAs(u) {
   Datetime d = Datetime.valueOfGmt('2026-03-08 10:30:00');
   rows.add('vge=' + d.getTime() + '|g=' + d.formatGmt('yyyy-MM-dd HH:mm:ss.SSS Z'));
   Datetime p = Datetime.parse('2026-03-08 10:30:00.123');
   rows.add('pm=' + p.getTime() + '|g=' + p.formatGmt('yyyy-MM-dd HH:mm:ss.SSS'));
   Datetime pz = Datetime.parse('2026-03-08T10:30:00.123Z');
   rows.add('pz=' + pz.getTime());
   rows.add('gfmt=' + d.formatGmt('EEE, dd MMM yyyy HH:mm:ss z'));
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
