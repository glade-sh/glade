Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size(); Map<String,Schema.SObjectType> d = gd.deepClone();
String o = '' + (d.size() - n) + '|' + d.get('account') + '|' + d.containsKey('account') + '|' + d.containsKey('Account') + '|' + d.keySet().contains('account') + '|' + (d == gd);
System.debug('OBS|dc_typed_self|' + o);