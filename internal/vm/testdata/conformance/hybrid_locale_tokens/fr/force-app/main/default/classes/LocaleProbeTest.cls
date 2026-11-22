@IsTest private class LocaleProbeTest {
 @IsTest static void observe() {
  Datetime d = Datetime.newInstanceGmt(1500,2,28,12,0,0);
  System.assertEquals('ROW', d.format('EEEE|EEE|MMMM|MMM|u','GMT') + '|' + d.addDays(1).format('EEEE|EEE|MMMM|MMM|u','GMT') + '|' + Datetime.newInstanceGmt(1583,1,1,12,0,0).format('EEEE|EEE|MMMM|MMM|u','GMT'));
 }
}
