Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size(); Map<String,Schema.SObjectType> d = gd.deepClone();
d.clear();
String o = '' + d.size() + '|' + d.get('account') + '|' + d.containsKey('account') + '|' + (gd.size() - n) + '|' + gd.get('account') + '|' + (Schema.getGlobalDescribe().size() - n);
System.debug('OBS|dc_typed_clear|' + o);