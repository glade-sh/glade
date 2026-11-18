Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
Object r = gd.remove('zzz_missing');
String o = '' + r + '|' + (n - gd.size());
System.debug('OBS|gw_remove_missing|' + o);