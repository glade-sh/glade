Contact before = new Contact(LastName='Update sample', Title='Prior title'); insert before;
Flow.Interview f = Flow.Interview.createInterview('TransformUpdate', new Map<String,Object>{'InputText'=>'Update sample','InputId'=>String.valueOf(before.Id)});
f.start(); Contact result = (Contact)f.getVariableValue('OutputRecord');
update result; Contact after = [SELECT Title FROM Contact WHERE Id = :before.Id];
System.debug('OBS|' + JSON.serialize(new Map<String,Object>{'populatedNull'=>result.getPopulatedFieldsAsMap().containsKey('Title'),'titleAfter'=>after.Title}));
