List<String> inputs = new List<String>{
 '1234567890123456789012345678901234',
 '12345678901234567890123456789012345',
 '1234567890123456789012345678901234567890',
 '100000000000000000000000000000000' + '5',
 '100000000000000000000000000000001' + '5',
 '100000000000000000000000000000000' + '4',
 '100000000000000000000000000000000' + '6',
 '-100000000000000000000000000000000' + '5',
 '0.' + '1234567890123456789012345678901234567890',
 '12345678901234567890.1234567890123456789012345',
 '1000000000000000000000000000000000',
 '100000000000000000000000000000000'
};
for (String s : inputs) {
  try {
    Decimal d = Decimal.valueOf(s);
    System.debug('CAP|' + s + '|' + d.toPlainString() + '|p=' + d.precision() + '|s=' + d.scale());
  } catch (Exception e) { System.debug('CAP|' + s + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
Datetime dt = Datetime.newInstanceGmt(2026,1,1,0,0,0);
for (String f : new List<String>{'z','zzzz','Z','XXX'}) {
  System.debug('CAP|sgt|' + f + '|' + dt.format('yyyy-MM-dd HH:mm ' + f, 'Asia/Singapore'));
}
System.debug('CAP|sgtjuly|' + Datetime.newInstanceGmt(2026,7,1,0,0,0).format('yyyy-MM-dd HH:mm z zzzz','Asia/Singapore'));
