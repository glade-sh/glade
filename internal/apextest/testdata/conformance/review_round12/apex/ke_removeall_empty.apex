Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); Integer n = k.size();
Boolean r = k.removeAll(new Set<String>());
String o = '' + r + '|' + (k.size() - n);
System.debug('OBS|ke_removeall_empty|' + o);