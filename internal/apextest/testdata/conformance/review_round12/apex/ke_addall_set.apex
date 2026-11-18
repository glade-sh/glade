Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); Integer n = k.size();
Boolean r = k.addAll(new Set<String>());
String o = '' + r + '|' + (k.size() - n);
System.debug('OBS|ke_addall_set|' + o);