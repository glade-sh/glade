List<CollHold__c> rows0=[SELECT Name FROM CollHold__c ORDER BY Name DESC];
List<String> out0=new List<String>(); for(CollHold__c r:rows0){out0.add(r.Name);}
for(Integer c=0;c<out0.size();c+=300){
 List<String> part=new List<String>();
 for(Integer i=c;i<Math.min(c+300,out0.size());i++){part.add(out0[i]);}
 System.debug('C2|QDESC_H_D_'+c+'|'+String.join(part,'~'));
}
List<CollHold__c> rows3=[SELECT Name FROM CollHold__c ORDER BY Name DESC NULLS FIRST];
List<String> out3=new List<String>(); for(CollHold__c r:rows3){out3.add(r.Name);}
for(Integer c=0;c<out3.size();c+=300){
 List<String> part=new List<String>();
 for(Integer i=c;i<Math.min(c+300,out3.size());i++){part.add(out3[i]);}
 System.debug('C2|QDESC_H_NF_'+c+'|'+String.join(part,'~'));
}
List<CollHold__c> rows6=[SELECT Name FROM CollHold__c ORDER BY Name DESC NULLS LAST];
List<String> out6=new List<String>(); for(CollHold__c r:rows6){out6.add(r.Name);}
for(Integer c=0;c<out6.size();c+=300){
 List<String> part=new List<String>();
 for(Integer i=c;i<Math.min(c+300,out6.size());i++){part.add(out6[i]);}
 System.debug('C2|QDESC_H_NL_'+c+'|'+String.join(part,'~'));
}
