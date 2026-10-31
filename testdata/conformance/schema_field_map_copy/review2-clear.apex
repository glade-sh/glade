Map<String,Schema.SObjectField> d=Schema.SObjectType.Account.fields.getMap(); d.clear();
Map<String,Schema.SObjectField> c=new Map<String,Schema.SObjectField>(d); Map<String,Schema.SObjectField> l=d.clone();
System.debug('ROAD123|C56|'+(d.size()==0));
System.debug('ROAD123|C57|'+(c.get('NAME')==null));
System.debug('ROAD123|C58|'+(l.get('NAME')==null));
System.debug('ROAD123|C59|'+(c.size()==0));
System.debug('ROAD123|C60|'+(l.size()==0));
