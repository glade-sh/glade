Integer n = Schema.getGlobalDescribe().size();
Schema.getGlobalDescribe().clear();
String o = '' + (Schema.getGlobalDescribe().size() - n) + '|' + Schema.getGlobalDescribe().get('account');
System.debug('OBS|gw_chained_clear|' + o);