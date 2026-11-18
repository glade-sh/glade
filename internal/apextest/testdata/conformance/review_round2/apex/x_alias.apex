AlLg.ev = new List<String>();
AlLg.add('one');
Map<String,Object> o = new Map<String,Object>(); o.put('x', AlLg.ev);
AlLg.add('two');
List<String> g = AlLg.get();
Map<String,Object> o2 = new Map<String,Object>(); o2.put('x', g); AlLg.add('three');
List<String> loc = new List<String>(); Map<String,Object> o3 = new Map<String,Object>(); o3.put('x', loc); loc.add('a');
System.debug('OBS|alias|' + JSON.serialize(o) + JSON.serialize(o2) + JSON.serialize(o3));