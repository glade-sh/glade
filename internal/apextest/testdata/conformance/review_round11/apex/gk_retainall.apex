Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
Boolean r = gd.keySet().retainAll(new Set<String>{'user'});
String o = '' + r + '|' + gd.size() + '|' + gd.containsKey('user') + '|' + gd.get('user') + '|' + gd.containsKey('account') + '|' + gd.get('account') + '|' + gd.keySet() + '|' + (Schema.getGlobalDescribe().size() - n);
System.debug('OBS|gk_retainall|' + o);