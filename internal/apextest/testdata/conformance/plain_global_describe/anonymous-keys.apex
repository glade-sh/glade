List<String> ks = new List<String>(Schema.getGlobalDescribe().keySet());
ks.sort();
System.debug('COUNT|' + ks.size());
Integer i = 0;
while (i < ks.size()) {
  Integer j = Math.min(i + 400, ks.size());
  List<String> part = new List<String>();
  for (Integer k = i; k < j; k++) part.add(ks[k]);
  System.debug('KEYS|' + String.join(part, ','));
  i = j;
}
