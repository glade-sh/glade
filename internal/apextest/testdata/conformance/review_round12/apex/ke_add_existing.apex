Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); Integer n = k.size();
Boolean r = k.add('id');
String o = '' + r + '|' + (k.size() - n);
System.debug('OBS|ke_add_existing|' + o);