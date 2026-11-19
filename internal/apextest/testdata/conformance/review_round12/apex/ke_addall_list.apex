Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); Integer n = k.size();
Boolean r = k.addAll(new List<String>());
String o = '' + r + '|' + (k.size() - n);
System.debug('OBS|ke_addall_list|' + o);