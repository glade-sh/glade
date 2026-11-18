Map<String,Map<String,Account>> m = new Map<String,Map<String,Account>>{'a' => new Map<String,Account>{'b' => new Account(Name = 'x')}}; Map<String,Map<String,Account>> d = m.deepClone();
d.get('a').get('b').Name = 'y';
String o = '' + d.get('a').get('b').Name + '|' + m.get('a').get('b').Name;
System.debug('OBS|dc_nested_map|' + o);