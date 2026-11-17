@IsTest private class CalParse {
 static String p = '';
 static void add(String k, Object v) { p += '|' + k + '=' + v; }
 static void tryParse(String s) { try { Datetime d = Datetime.parse(s); add('dt[' + s + ']', d.getTime() + ' ' + d.formatGmt('yyyy-MM-dd HH:mm:ss')); } catch (Exception e) { add('dt[' + s + ']', 'EX:' + e.getTypeName() + ':' + e.getMessage()); } }
 static void tryDate(String s) { try { Date d = Date.parse(s); add('d[' + s + ']', d); } catch (Exception e) { add('d[' + s + ']', 'EX:' + e.getTypeName() + ':' + e.getMessage()); } }
 static void tryValueOf(String s) { try { Datetime d = Datetime.valueOf(s); add('v[' + s + ']', d.getTime()); } catch (Exception e) { add('v[' + s + ']', 'EX:' + e.getTypeName() + ':' + e.getMessage()); } }
 @IsTest static void t() {
  for (String s : new List<String>{'1/1/1500, 12:00 PM','1/1/1500 12:00 PM','1/1/1582, 12:00 PM','10/4/1582, 12:00 PM','10/15/1582, 12:00 PM','1/1/1600, 12:00 PM','1/1/1699, 12:00 PM','1/1/1700, 12:00 PM','1/1/1900, 12:00 PM','1/1/1900 12:00 PM','1/1/2026, 12:00 PM','2/29/1500, 12:00 PM'}) tryParse(s);
  for (String s : new List<String>{'1/1/1500','1/1/1700','1/1/1900','2/29/1500','10/10/1582'}) tryDate(s);
  for (String s : new List<String>{'1500-01-01 12:00:00','1500-02-29 12:00:00','1582-10-10 12:00:00','0001-01-01 00:00:00','1900-01-01 12:00:00'}) tryValueOf(s);
  System.assert(false, 'CalParse' + p);
 }
}
