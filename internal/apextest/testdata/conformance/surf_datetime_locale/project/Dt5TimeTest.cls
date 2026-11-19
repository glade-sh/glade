@IsTest
private class Dt5TimeTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  User u = DtSupport.zones()[0];
  System.runAs(u) {
   rows.add('addH=' + Time.newInstance(23, 30, 0, 0).addHours(2));
   rows.add('addHneg=' + Time.newInstance(1, 0, 0, 0).addHours(-2));
   rows.add('addM=' + Time.newInstance(23, 30, 0, 0).addMinutes(45));
   rows.add('addS=' + Time.newInstance(0, 0, 1, 0).addSeconds(-2));
   rows.add('addMs=' + Time.newInstance(0, 0, 0, 900).addMilliseconds(200));
   rows.add('fields=' + Time.newInstance(13, 45, 30, 250).hour() + ',' + Time.newInstance(13, 45, 30, 250).minute() + ',' + Time.newInstance(13, 45, 30, 250).second() + ',' + Time.newInstance(13, 45, 30, 250).millisecond());
   rows.add('str=' + String.valueOf(Time.newInstance(13, 45, 30, 250)));
  }
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
