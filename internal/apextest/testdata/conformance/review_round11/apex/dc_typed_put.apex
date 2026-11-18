Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size(); Map<String,Schema.SObjectType> d = gd.deepClone();
d.put('zzshare', Account.SObjectType);
String o = '' + d.containsKey('zzshare') + '|' + d.get('zzshare') + '|' + (d.size() - n) + '|' + gd.containsKey('zzshare') + '|' + (Schema.getGlobalDescribe().size() - n) + '|' + Schema.getGlobalDescribe().containsKey('zzshare');
System.debug('OBS|dc_typed_put|' + o);