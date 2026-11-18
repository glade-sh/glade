Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size(); Map<String,Schema.SObjectType> d = gd.deepClone();
Object r = d.remove('account');
String o = '' + r + '|' + d.get('account') + '|' + d.containsKey('account') + '|' + (n - d.size()) + '|' + gd.containsKey('account') + '|' + gd.get('account') + '|' + (Schema.getGlobalDescribe().size() - n) + '|' + Schema.getGlobalDescribe().get('account');
System.debug('OBS|dc_typed_remove|' + o);