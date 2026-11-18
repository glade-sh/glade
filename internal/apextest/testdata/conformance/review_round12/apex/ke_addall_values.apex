Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); Integer n = k.size();
Map<String,String> e = new Map<String,String>();
Boolean r = k.addAll(e.values());
String o = '' + r + '|' + (k.size() - n);
System.debug('OBS|ke_addall_values|' + o);