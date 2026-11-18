Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
gd.put('zzshare', Account.SObjectType);
String o = '' + gd.containsKey('zzshare') + '|' + (gd.size() - n);
System.debug('OBS|gw_put|' + o);