Integer n = Schema.getGlobalDescribe().size();
Schema.getGlobalDescribe().remove('account');
String o = '' + Schema.getGlobalDescribe().containsKey('account') + '|' + (n - Schema.getGlobalDescribe().size());
System.debug('OBS|gw_chained_remove|' + o);