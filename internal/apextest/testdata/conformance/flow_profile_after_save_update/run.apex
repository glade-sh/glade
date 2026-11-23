FX__c f = new FX__c(Name='f', Status__c='aaa'); insert f;
f.Status__c = 'bbb'; update f;
Map<String,Integer> m = new Map<String,Integer>();
for (FL__c l : [SELECT Tag__c FROM FL__c]) m.put(l.Tag__c, (m.containsKey(l.Tag__c) ? m.get(l.Tag__c) : 0) + 1);
List<String> keys = new List<String>(m.keySet()); keys.sort();
String s = ''; for (String k : keys) s += k + '=' + m.get(k) + ',';
System.debug('R5|flow|' + s);
