Map<String,Integer> m = new Map<String,Integer>{'a' => 1}; Map<String,Integer> d = m.deepClone();
String o = '' + d.get('a');
System.debug('OBS|dc_int|' + o);