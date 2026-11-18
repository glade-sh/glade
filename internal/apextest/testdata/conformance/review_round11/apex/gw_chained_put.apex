Integer n = Schema.getGlobalDescribe().size();
Schema.getGlobalDescribe().put('zzshare', Account.SObjectType);
String o = '' + Schema.getGlobalDescribe().containsKey('zzshare') + '|' + (Schema.getGlobalDescribe().size() - n);
System.debug('OBS|gw_chained_put|' + o);