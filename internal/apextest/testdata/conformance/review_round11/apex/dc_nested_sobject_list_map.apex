Map<String,Map<Id,SObject>> m = new Map<String,Map<Id,SObject>>{'a' => new Map<Id,SObject>()}; Map<String,Map<Id,SObject>> d = m.deepClone();
String o = '' + d.get('a').size();
System.debug('OBS|dc_nested_sobject_list_map|' + o);