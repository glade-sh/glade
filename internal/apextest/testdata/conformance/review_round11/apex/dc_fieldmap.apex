Map<String,Schema.SObjectField> m = Schema.SObjectType.Account.fields.getMap(); Map<String,Schema.SObjectField> d = m.deepClone();
String o = '' + (d.size() == m.size()) + '|' + d.get('name');
System.debug('OBS|dc_fieldmap|' + o);