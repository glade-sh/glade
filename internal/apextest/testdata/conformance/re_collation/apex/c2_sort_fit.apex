List<CollFit__c> all=[SELECT Name FROM CollFit__c];
List<String> V=new List<String>(); for(CollFit__c r:all){V.add(r.Name);}
V.sort();
for(Integer c=0;c<V.size();c+=300){
 List<String> part=new List<String>();
 for(Integer i=c;i<Math.min(c+300,V.size());i++){part.add(V[i]);}
 System.debug('C2|SORTF_'+c+'|'+String.join(part,'~'));
}
