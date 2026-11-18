Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
Object r = gd.remove('Account');
String o = '' + r + '|' + gd.containsKey('account') + '|' + gd.get('account') + '|' + (n - gd.size()) + '|' + gd.keySet().contains('account');
System.debug('OBS|gk_direct_upper|' + o);