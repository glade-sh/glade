List<String> rows = new List<String>();
for (String s : new List<String>{' 1', '+1', '01', '1.0', '', '-0', '1,000', '2147483648', '1e3', '007'}) {
 try { rows.add('i[' + s + ']=' + Integer.valueOf(s)); } catch (Exception e) { rows.add('i[' + s + ']=' + e.getTypeName() + '|' + e.getMessage()); }
}
for (String s : new List<String>{'1.2.3', ' 1', '1e5', '', '-0.0', '1.500', '1,000', 'abc'}) {
 try { rows.add('d[' + s + ']=' + Decimal.valueOf(s)); } catch (Exception e) { rows.add('d[' + s + ']=' + e.getTypeName() + '|' + e.getMessage()); }
}
System.debug('OBS|' + String.join(rows, ';'));
