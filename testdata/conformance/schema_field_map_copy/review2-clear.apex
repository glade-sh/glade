Map<String,Schema.SObjectField> d=Schema.SObjectType.Account.fields.getMap(); d.clear();
Map<String,Schema.SObjectField> c=new Map<String,Schema.SObjectField>(d); Map<String,Schema.SObjectField> l=d.clone();
System.debug('CASE123|C56|'+(d.size()==0));
System.debug('CASE123|C57|'+(c.get('NAME')==null));
System.debug('CASE123|C58|'+(l.get('NAME')==null));
System.debug('CASE123|C59|'+(c.size()==0));
System.debug('CASE123|C60|'+(l.size()==0));
