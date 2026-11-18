Set<String> k = Account.SObjectType.getDescribe(SObjectDescribeOptions.DEFERRED).fields.getMap().keySet(); Integer n = k.size();
Map<String,String> e = new Map<String,String>();
Boolean r = k.addAll(e.values());
String o = '' + r + '|' + (k.size() - n);
System.debug('OBS|ke_def_addall_values|' + o);