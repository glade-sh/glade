Integer n = Schema.getGlobalDescribe().size(); Map<String,Schema.SObjectType> d = Schema.getGlobalDescribe().deepClone();
String o = '' + (d.size() - n) + '|' + d.get('account') + '|' + d.containsKey('account') + '|' + d.containsKey('Account') + '|' + d.keySet().contains('account') + '|' + (d == Schema.getGlobalDescribe());
System.debug('OBS|dc_chained_self|' + o);