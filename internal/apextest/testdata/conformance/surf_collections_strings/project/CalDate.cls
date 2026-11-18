@IsTest private class CalDate {
 static String p = '';
 static void add(String k, Object v) { p += '|' + k + '=' + v; }
 @IsTest static void t() {
  
  List<Date> ds = new List<Date>{ Date.newInstance(1500,1,1), Date.newInstance(1500,2,29), Date.newInstance(1500,3,1), Date.newInstance(1582,10,4), Date.newInstance(1582,10,10), Date.newInstance(1582,10,15), Date.newInstance(1,6,1), Date.newInstance(1400,2,29) };
  for (Date d : ds) { add('d', String.valueOf(d) + ' doy=' + d.dayOfYear() + ' hash=' + d.hashCode() + ' json=' + JSON.serialize(d) + ' ' + d.year() + '-' + d.month() + '-' + d.day() + ' sow=' + d.toStartOfWeek()); }
  add('add1004', Date.newInstance(1582,10,4).addDays(1));
  add('add1015', Date.newInstance(1582,10,15).addDays(-1));
  add('daysBetween', Date.newInstance(1582,10,4).daysBetween(Date.newInstance(1582,10,15)));
  add('daysBetween1500', Date.newInstance(1500,1,1).daysBetween(Date.newInstance(1600,1,1)));
  add('addYears', Date.newInstance(1500,2,29).addYears(1) + ' ' + Date.newInstance(1500,1,1).addYears(100) + ' ' + Date.newInstance(1500,1,1).addMonths(1200));
  add('isLeap', Date.isLeapYear(1500) + ',' + Date.isLeapYear(1400) + ',' + Date.isLeapYear(1600));
  add('dim', Date.daysInMonth(1500, 2));
  add('dt', Datetime.newInstance(Date.newInstance(1500,2,29), Time.newInstance(0,0,0,0)).getTime());
  add('dtg', Datetime.newInstanceGmt(Date.newInstance(1500,1,1), Time.newInstance(0,0,0,0)).getTime());
  add('valueOfDate', Date.valueOf('1500-02-29'));

  System.assert(false, 'CalDate' + p);
 }
}
