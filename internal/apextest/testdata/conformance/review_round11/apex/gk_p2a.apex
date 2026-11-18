Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
gd.put('zzshare', Account.SObjectType); gd.remove('account');
String o = '' + gd.containsKey('zzshare') + '|' + (gd.size() - n) + '|' + gd.get('account') + '|' + Schema.getGlobalDescribe().containsKey('zzshare') + '|' + (Schema.getGlobalDescribe().size() - n);
System.debug('OBS|gk_p2a|' + o);