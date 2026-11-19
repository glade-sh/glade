Map<String, Schema.FieldSet> m = Schema.SObjectType.Account.fieldSets.getMap();
List<String> keys = new List<String>(m.keySet()); keys.sort();
List<String> labels = new List<String>(); for (Schema.FieldSet fs : m.values()) labels.add(fs.getLabel()); labels.sort();
String a = 'firstpkg__CollisionProbe171'; String b = 'secondpkg__CollisionProbe171';
System.debug('COLLISION_RESULT:' + JSON.serialize(new Map<String,Object>{'keys'=>keys,'size'=>m.size(),'valuesSize'=>m.values().size(),'labels'=>labels,'firstQualified'=>m.get(a)!=null,'secondQualified'=>m.get(b)!=null,'firstMixedCase'=>m.get(a.toUpperCase())!=null,'secondMixedCase'=>m.get(b.toUpperCase())!=null,'localLookup'=>m.get('CollisionProbe171')!=null,'missingLocal'=>m.get('CollisionProbe171Missing')!=null,'missingQualified'=>m.get('ghostns__CollisionProbe171')!=null}));
