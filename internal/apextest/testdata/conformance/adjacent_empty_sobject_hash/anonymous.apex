Map<String,Object> o = new Map<String,Object>();
o.put('H01', new List<Integer>{System.hashCode(new Opportunity()),System.hashCode(new Lead()),System.hashCode(new Case())});
o.put('H02', new List<Integer>{System.hashCode(new Opportunity(Name='Synthetic')),System.hashCode(new Lead(LastName='Synthetic')),System.hashCode(new Opportunity(Name=null)),System.hashCode(new Lead(LastName=null))});
o.put('H03', new List<Integer>{System.hashCode(new List<Opportunity>{new Opportunity()}),System.hashCode(new List<Lead>{new Lead()})});
try { System.hashCode(null); o.put('H04','accepted'); } catch (Exception e) { o.put('H04', e.getTypeName() + ':' + e.getMessage()); }
System.debug('OBS|' + JSON.serialize(o));
