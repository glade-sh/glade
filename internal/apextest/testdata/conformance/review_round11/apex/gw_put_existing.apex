Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe();
gd.put('account', User.SObjectType);
String o = '' + gd.get('account');
System.debug('OBS|gw_put_existing|' + o);