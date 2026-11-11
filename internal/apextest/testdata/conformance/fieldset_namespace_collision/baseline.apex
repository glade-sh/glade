Map<String, Schema.FieldSet> m = Schema.SObjectType.Account.fieldSets.getMap();
System.debug('COLLISION_PROBE:' + JSON.serialize(new Map<String,Object>{'keys' => m.keySet(), 'size' => m.size(), 'valuesSize' => m.values().size()}));
