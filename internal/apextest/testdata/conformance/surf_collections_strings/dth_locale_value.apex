Datetime a=Datetime.newInstance(0L), b=Datetime.newInstance(1L);
Date d=Date.newInstance(2026,11,16); Time t=Time.newInstance(12,34,56,789);
List<Datetime> l=new List<Datetime>{a,b}; Set<Datetime> s=new Set<Datetime>{a,b}; Map<Datetime,Integer> m=new Map<Datetime,Integer>{a=>1,b=>2};
throw new DthException(JSON.serialize(new Map<String,Object>{'locale'=>UserInfo.getLocale(),'a_ms'=>a.getTime(),'a_hash'=>a.hashCode(),'b_ms'=>b.getTime(),'b_hash'=>b.hashCode(),'list_hash'=>l.hashCode(),'set_hash'=>s.hashCode(),'map_hash'=>m.hashCode(),'date_hash'=>d.hashCode(),'time_hash'=>t.hashCode()}));
