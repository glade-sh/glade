@IsTest private class LocaleProbeTest {
 @IsTest static void observe() {
  String result = '';
  for (Integer month = 1; month <= 12; month++) {
   Datetime d = Datetime.newInstanceGmt(1500,month,15,12,0,0);
   result += d.format('EEEE|EEE|MMMM|MMM|u','GMT') + ';';
  }
  for (Integer day = 1; day <= 7; day++) {
   Datetime d = Datetime.newInstanceGmt(1400,1,day,12,0,0);
   result += d.format('EEEE|EEE|MMMM|MMM|u','GMT') + ';';
  }
  System.assertEquals('ROW', result);
 }
}
