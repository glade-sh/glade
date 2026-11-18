List<CollFit__c> rows0=[SELECT Name FROM CollFit__c ORDER BY Name DESC];
List<String> out0=new List<String>(); for(CollFit__c r:rows0){out0.add(r.Name);}
for(Integer c=0;c<out0.size();c+=300){
 List<String> part=new List<String>();
 for(Integer i=c;i<Math.min(c+300,out0.size());i++){part.add(out0[i]);}
 System.debug('C2|QDESC_F_D_'+c+'|'+String.join(part,'~'));
}
List<CollFit__c> rows3=[SELECT Name FROM CollFit__c ORDER BY Name DESC NULLS FIRST];
List<String> out3=new List<String>(); for(CollFit__c r:rows3){out3.add(r.Name);}
for(Integer c=0;c<out3.size();c+=300){
 List<String> part=new List<String>();
 for(Integer i=c;i<Math.min(c+300,out3.size());i++){part.add(out3[i]);}
 System.debug('C2|QDESC_F_NF_'+c+'|'+String.join(part,'~'));
}
List<CollFit__c> rows6=[SELECT Name FROM CollFit__c ORDER BY Name DESC NULLS LAST];
List<String> out6=new List<String>(); for(CollFit__c r:rows6){out6.add(r.Name);}
for(Integer c=0;c<out6.size();c+=300){
 List<String> part=new List<String>();
 for(Integer i=c;i<Math.min(c+300,out6.size());i++){part.add(out6[i]);}
 System.debug('C2|QDESC_F_NL_'+c+'|'+String.join(part,'~'));
}
