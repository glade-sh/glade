Set<String> k = KeHolder.fields(); Integer n = k.size();
k.addAll(new Map<String,String>().values());
String o = '' + (k.size() - n);
System.debug('OBS|ke_returned_addall|' + o);