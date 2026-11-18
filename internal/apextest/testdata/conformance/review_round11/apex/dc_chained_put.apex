Integer n = Schema.getGlobalDescribe().size(); Map<String,Schema.SObjectType> d = Schema.getGlobalDescribe().deepClone();
d.put('zzshare', Account.SObjectType);
String o = '' + d.containsKey('zzshare') + '|' + d.get('zzshare') + '|' + (d.size() - n) + '|' + Schema.getGlobalDescribe().containsKey('zzshare') + '|' + (Schema.getGlobalDescribe().size() - n) + '|' + Schema.getGlobalDescribe().containsKey('zzshare');
System.debug('OBS|dc_chained_put|' + o);