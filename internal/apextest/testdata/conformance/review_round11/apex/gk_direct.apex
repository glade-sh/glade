Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size(); Set<String> pre = gd.keySet();
gd.remove('account');
String o = '' + gd.get('account') + '|' + gd.containsKey('account') + '|' + (n - gd.size()) + '|' + gd.keySet().contains('account') + '|' + pre.contains('account') + '|' + gd.get('Account') + '|' + Schema.getGlobalDescribe().containsKey('account');
System.debug('OBS|gk_direct|' + o);