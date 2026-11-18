Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Set<String> saved = gd.keySet(); Set<String> saved2 = gd.keySet(); Integer n = gd.size();
saved.remove('account');
String o = '' + (n - gd.size()) + '|' + gd.containsKey('account') + '|' + saved2.contains('account') + '|' + (n - saved2.size()) + '|' + (n - saved.size());
gd.clear(); Boolean r = saved.remove('user');
o += '|' + r + '|' + gd.size() + '|' + saved.size() + '|' + saved2.size() + '|' + (Schema.getGlobalDescribe().size() - n);
System.debug('OBS|gk_saved|' + o);