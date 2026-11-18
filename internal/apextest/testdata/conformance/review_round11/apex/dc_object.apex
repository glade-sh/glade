Map<String,Object> m = new Map<String,Object>{'a' => 1}; Map<String,Object> d = m.deepClone();
String o = '' + d.get('a');
System.debug('OBS|dc_object|' + o);