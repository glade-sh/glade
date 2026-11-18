Map<Id,Account> m = new Map<Id,Account>(); Map<Id,Account> d = m.deepClone();
String o = '' + d.size();
System.debug('OBS|dc_account|' + o);