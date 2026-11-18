Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe();
gd.putAll(new Map<String,Schema.SObjectType>{'zzshare' => Account.SObjectType});
String o = '' + gd.containsKey('zzshare');
System.debug('OBS|gw_putall|' + o);