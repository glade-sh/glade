Map<String,SObject> m = new Map<String,SObject>{'a' => new Account(Name = 'x')}; Map<String,SObject> d = m.deepClone();
String o = '' + d.get('a').get('Name');
System.debug('OBS|dc_sobject|' + o);