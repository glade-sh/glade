Contact original = new Contact(LastName='Update sample', Title='Prior title'); insert original;
Flow.Interview f = Flow.Interview.createInterview('TransformCollection', new Map<String,Object>{'InputText'=>'Update sample','InputId'=>String.valueOf(original.Id)}); f.start();
List<Contact> items = (List<Contact>)f.getVariableValue('OutputRecords');
update items; Contact after = [SELECT Title FROM Contact WHERE Id = :original.Id];
System.debug('OBS|' + JSON.serialize(new Map<String,Object>{'size'=>items.size(),'populatedNull'=>items[0].getPopulatedFieldsAsMap().containsKey('Title'),'titleAfter'=>after.Title}));
