List<Long> V=new List<Long>();
for(Integer i=0;i<20;i++){V.add(System.now().getTime());Integer x=0;for(Integer j=0;j<5000;j++){x+=j;}}
List<String> S=new List<String>(); for(Long v:V){S.add(String.valueOf(v));}
System.debug('T4|NOW|'+String.join(S,','));
List<String> ms=new List<String>(); for(Long v:V){ms.add(String.valueOf(Math.mod(v,1000)));}
System.debug('T4|NOWMS|'+String.join(ms,','));
Datetime n1=Datetime.now(); Datetime n2=Datetime.now();
System.debug('T4|DTN|'+n1.getTime()+'|'+n2.getTime()+'|'+n1.format('yyyy-MM-dd HH:mm:ss.SSS'));
