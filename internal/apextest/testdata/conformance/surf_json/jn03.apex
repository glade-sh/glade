List<String> rows = new List<String>();
Map<String,String> bodies = new Map<String,String>{
 'maxLong'=>'9223372036854775807',
 'overLong'=>'9223372036854775808',
 'underLong'=>'-9223372036854775809',
 'huge'=>'1e400',
 'oneZero'=>'1.0',
 'exp'=>'1E2',
 'negZero'=>'-0',
 'decScale'=>'12.500',
 'maxInt'=>'2147483647',
 'overInt'=>'2147483648'
};
for (String k : bodies.keySet()) {
 try {
  Object v = JSON.deserializeUntyped(bodies.get(k));
  String f = ((v instanceof Integer) ? 'I' : '-') + ((v instanceof Long) ? 'L' : '-') + ((v instanceof Decimal) ? 'D' : '-') + ((v instanceof Double) ? 'X' : '-');
  rows.add(k + '=' + f + ':' + String.valueOf(v));
 } catch (Exception e) { rows.add(k + '=' + e.getTypeName() + '|' + e.getMessage()); }
}
for (String r : rows) { System.debug('OBS|' + r); }
