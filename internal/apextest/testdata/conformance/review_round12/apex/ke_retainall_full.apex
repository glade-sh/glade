Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); Integer n = k.size();
Boolean r = k.retainAll(new Set<String>(k));
String o = '' + r + '|' + (k.size() - n);
System.debug('OBS|ke_retainall_full|' + o);