Flow.Interview f = Flow.Interview.createInterview('TransformProbe', new Map<String,Object>{'InputText' => 'Map sample'});
f.start();
Contact c = (Contact)f.getVariableValue('OutputRecord');
System.debug('OBS|' + JSON.serialize(new Map<String,Object>{'lastName'=>c.LastName,'title'=>c.Title,'populated'=>c.getPopulatedFieldsAsMap()}));
