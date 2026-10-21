System.debug('OBSERVED|concrete|' + JSON.serialize(new Map<String,Object>{'value'=>PlanProbe.concrete()}));
System.debug('OBSERVED|inherited|' + JSON.serialize(new Map<String,Object>{'value'=>PlanProbe.inherited()}));
System.debug('OBSERVED|throughInterface|' + JSON.serialize(new Map<String,Object>{'value'=>PlanProbe.throughInterface()}));
