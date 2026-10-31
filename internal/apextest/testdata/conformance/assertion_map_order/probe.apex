Map<String,Object> m = new Map<String,Object>();
m.put('alpha', 1); m.put('beta', 2);
System.debug('CASE_forward=' + JSON.serialize(m));
Map<String,Object> n = new Map<String,Object>();
n.put('beta', 2); n.put('alpha', 1);
System.debug('CASE_reverse=' + JSON.serialize(n));
Map<String,Object> nestedMap = new Map<String,Object>(); nestedMap.put('alpha', 1); nestedMap.put('beta', 2);
Map<String,Object> outerMap = new Map<String,Object>(); outerMap.put('inner',nestedMap);
System.debug('CASE_nested=' + JSON.serialize(outerMap));
Map<String,Object> one = new Map<String,Object>(); one.put('alpha',1);
System.debug('CASE_boundary=' + JSON.serialize(one));
Map<String,Object> invalid = new Map<String,Object>(); invalid.put(null,1);
try { System.debug('CASE_reject=' + JSON.serialize(invalid)); } catch (Exception e) { System.debug('CASE_reject=' + e.getTypeName() + '|' + e.getMessage()); }
