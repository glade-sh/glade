Map<String,List<Account>> m = new Map<String,List<Account>>{'a' => new List<Account>()}; Map<String,List<Account>> d = m.deepClone();
String o = '' + d.get('a').size();
System.debug('OBS|dc_list_value|' + o);