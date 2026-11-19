Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Map<String,Schema.SObjectType> c = gd.clone(); Integer n = c.size();
c.remove('account');
String o = '' + (n - c.size()) + '|' + c.containsKey('account') + '|' + c.get('account') + '|' + gd.containsKey('account');
System.debug('OBS|gw_clone_remove|' + o);