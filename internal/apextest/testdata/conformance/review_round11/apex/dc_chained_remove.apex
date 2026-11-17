Integer n = Schema.getGlobalDescribe().size(); Map<String,Schema.SObjectType> d = Schema.getGlobalDescribe().deepClone();
Object r = d.remove('account');
String o = '' + r + '|' + d.get('account') + '|' + d.containsKey('account') + '|' + (n - d.size()) + '|' + Schema.getGlobalDescribe().containsKey('account') + '|' + Schema.getGlobalDescribe().get('account') + '|' + (Schema.getGlobalDescribe().size() - n) + '|' + Schema.getGlobalDescribe().get('account');
System.debug('OBS|dc_chained_remove|' + o);