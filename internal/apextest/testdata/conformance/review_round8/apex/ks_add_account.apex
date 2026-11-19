Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet();
k.add('aaa_added'); k.add('zzz_added');
System.debug('OBS|ks_add_account|' + k + '|' + k.size());