Flow.Interview f = Flow.Interview.createInterview('TransformEmpty', new Map<String,Object>{'InputText'=>'Sample'}); f.start();
Contact c=(Contact)f.getVariableValue('OutputRecord');
System.debug('OBS|' + JSON.serialize(new Map<String,Object>{'lastName'=>c.LastName,'title'=>c.Title,'populated'=>c.getPopulatedFieldsAsMap().containsKey('Title')}));
