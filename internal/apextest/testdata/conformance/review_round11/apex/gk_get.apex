Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Map<String,Schema.SObjectType> other = Schema.getGlobalDescribe(); Set<String> k = gd.keySet(); Integer n = gd.size();
k.remove('account');
String o = '' + gd.get('account') + '|' + gd.get('Account') + '|' + gd.containsKey('Account') + '|' + (n - gd.size()) + '|' + other.get('account') + '|' + other.containsKey('account') + '|' + (n - other.size()) + '|' + k.contains('Account') + '|' + Schema.getGlobalDescribe().get('account');
System.debug('OBS|gk_get|' + o);