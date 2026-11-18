Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
Boolean r = gd.keySet().removeAll(new Set<String>{'account'});
String o = '' + r + '|' + (n - gd.size()) + '|' + gd.containsKey('account') + '|' + gd.get('account') + '|' + Schema.getGlobalDescribe().containsKey('account') + '|' + (Schema.getGlobalDescribe().size() - n);
System.debug('OBS|gk_removeall|' + o);