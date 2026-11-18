Set<String> k = Account.SObjectType.getDescribe(SObjectDescribeOptions.DEFERRED).fields.getMap().keySet(); Integer n = k.size();
Boolean r = k.addAll(new List<String>());
String o = '' + r + '|' + (k.size() - n);
System.debug('OBS|ke_def_addall_list|' + o);