Map<String,Integer> m = new Map<String,Integer>{'a'=>1};
Set<String> ks = m.keySet(); ks.add('b');
System.debug('OBS|ks_plain_map_add|' + ks + '|' + ks.size() + '|' + m.size() + '|' + m.containsKey('b'));