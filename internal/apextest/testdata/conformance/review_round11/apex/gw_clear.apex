Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
gd.clear();
String o = '' + gd.size() + '|' + gd.containsKey('account') + '|' + gd.get('account') + '|' + gd.keySet().size() + '|' + (Schema.getGlobalDescribe().size() - n);
System.debug('OBS|gw_clear|' + o);