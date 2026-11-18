Integer n = Schema.getGlobalDescribe().size(); Map<String,Schema.SObjectType> d = Schema.getGlobalDescribe().deepClone();
d.clear();
String o = '' + d.size() + '|' + d.get('account') + '|' + d.containsKey('account') + '|' + (Schema.getGlobalDescribe().size() - n) + '|' + Schema.getGlobalDescribe().get('account') + '|' + (Schema.getGlobalDescribe().size() - n);
System.debug('OBS|dc_chained_clear|' + o);