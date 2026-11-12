public class ResidualCriteria {
 static Id q; static Id queueId(){ if(q==null) q=[SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='ResidualQueue'].Id; return q; }
 static void run(Map<String,Object> m, String k, Lead l){
  try{
   Database.DMLOptions o=new Database.DMLOptions(); o.assignmentRuleHeader.useDefaultRule=true; o.duplicateRuleHeader.allowSave=true;
   Database.SaveResult r=Database.insert(l,o);
   if(!r.isSuccess()){ m.put(k,'ERR:'+r.getErrors()[0].getMessage().left(80)); return; }
   m.put(k,[SELECT OwnerId FROM Lead WHERE Id=:l.Id].OwnerId==queueId()?1:0);
  }catch(Exception e){ m.put(k,'EX:'+e.getMessage().left(80)); }
 }
 public static Map<String,Object> run(){
  Map<String,Object> m=new Map<String,Object>();
  run(m,'T1|Abc',new Lead(LastName='x',Company='kT1',Title='Abc'));
  run(m,'T1|xAb',new Lead(LastName='x',Company='kT1',Title='xAb'));
  run(m,'T1|abc',new Lead(LastName='x',Company='kT1',Title='abc'));
  run(m,'T1|null',new Lead(LastName='x',Company='kT1',Title=null));
  run(m,'T2|xABy',new Lead(LastName='x',Company='kT2',Title='xABy'));
  run(m,'T2|xaby',new Lead(LastName='x',Company='kT2',Title='xaby'));
  run(m,'T2|xy',new Lead(LastName='x',Company='kT2',Title='xy'));
  run(m,'T3|xABy',new Lead(LastName='x',Company='kT3',Title='xABy'));
  run(m,'T3|xy',new Lead(LastName='x',Company='kT3',Title='xy'));
  run(m,'T3|null',new Lead(LastName='x',Company='kT3',Title=null));
  run(m,'T4|alpha',new Lead(LastName='x',Company='kT4',Title='alpha'));
  run(m,'T4|Alpha',new Lead(LastName='x',Company='kT4',Title='Alpha'));
  run(m,'T4|Alphax',new Lead(LastName='x',Company='kT4',Title='Alphax'));
  run(m,'T5|a',new Lead(LastName='x',Company='kT5',Title='a'));
  run(m,'T5|b',new Lead(LastName='x',Company='kT5',Title='b'));
  run(m,'T5|a,b',new Lead(LastName='x',Company='kT5',Title='a,b'));
  run(m,'T5|c',new Lead(LastName='x',Company='kT5',Title='c'));
  run(m,'T6|a',new Lead(LastName='x',Company='kT6',Title='a'));
  run(m,'T6|c',new Lead(LastName='x',Company='kT6',Title='c'));
  run(m,'T7|ax',new Lead(LastName='x',Company='kT7',Title='ax'));
  run(m,'T7|bx',new Lead(LastName='x',Company='kT7',Title='bx'));
  run(m,'T7|cx',new Lead(LastName='x',Company='kT7',Title='cx'));
  run(m,'T8|xa',new Lead(LastName='x',Company='kT8',Title='xa'));
  run(m,'T8|xb',new Lead(LastName='x',Company='kT8',Title='xb'));
  run(m,'T8|xc',new Lead(LastName='x',Company='kT8',Title='xc'));
  run(m,'N1|6',new Lead(LastName='x',Company='kN1',NumberOfEmployees=6));
  run(m,'N1|5',new Lead(LastName='x',Company='kN1',NumberOfEmployees=5));
  run(m,'N1|null',new Lead(LastName='x',Company='kN1',NumberOfEmployees=null));
  run(m,'N2|5',new Lead(LastName='x',Company='kN2',NumberOfEmployees=5));
  run(m,'N2|4',new Lead(LastName='x',Company='kN2',NumberOfEmployees=4));
  run(m,'N2|6',new Lead(LastName='x',Company='kN2',NumberOfEmployees=6));
  run(m,'N3|4',new Lead(LastName='x',Company='kN3',NumberOfEmployees=4));
  run(m,'N3|5',new Lead(LastName='x',Company='kN3',NumberOfEmployees=5));
  run(m,'N3|null',new Lead(LastName='x',Company='kN3',NumberOfEmployees=null));
  run(m,'N4|5',new Lead(LastName='x',Company='kN4',NumberOfEmployees=5));
  run(m,'N4|6',new Lead(LastName='x',Company='kN4',NumberOfEmployees=6));
  run(m,'N4|4',new Lead(LastName='x',Company='kN4',NumberOfEmployees=4));
  run(m,'N5|7',new Lead(LastName='x',Company='kN5',NumberOfEmployees=7));
  run(m,'N5|6',new Lead(LastName='x',Company='kN5',NumberOfEmployees=6));
  run(m,'N6|5',new Lead(LastName='x',Company='kN6',NumberOfEmployees=5));
  run(m,'N6|6',new Lead(LastName='x',Company='kN6',NumberOfEmployees=6));
  run(m,'N6|null',new Lead(LastName='x',Company='kN6',NumberOfEmployees=null));
  run(m,'N7|1000',new Lead(LastName='x',Company='kN7',AnnualRevenue=1000));
  run(m,'N7|999.99',new Lead(LastName='x',Company='kN7',AnnualRevenue=999.99));
  run(m,'P1|Hot',new Lead(LastName='x',Company='kP1',Rating='Hot'));
  run(m,'P1|Cold',new Lead(LastName='x',Company='kP1',Rating='Cold'));
  run(m,'P2|Hot',new Lead(LastName='x',Company='kP2',Rating='Hot'));
  run(m,'P2|Cold',new Lead(LastName='x',Company='kP2',Rating='Cold'));
  run(m,'P3|Cold',new Lead(LastName='x',Company='kP3',Rating='Cold'));
  run(m,'P3|null',new Lead(LastName='x',Company='kP3',Rating=null));
  run(m,'P3|Hot',new Lead(LastName='x',Company='kP3',Rating='Hot'));
  run(m,'P4|Warm',new Lead(LastName='x',Company='kP4',Rating='Warm'));
  run(m,'P4|Hot',new Lead(LastName='x',Company='kP4',Rating='Hot'));
  run(m,'M1|A;B',new Lead(LastName='x',Company='kM1',Multi__c='A;B'));
  run(m,'M1|B;C',new Lead(LastName='x',Company='kM1',Multi__c='B;C'));
  run(m,'M2|A',new Lead(LastName='x',Company='kM2',Multi__c='A'));
  run(m,'M2|B;C',new Lead(LastName='x',Company='kM2',Multi__c='B;C'));
  run(m,'M2|C',new Lead(LastName='x',Company='kM2',Multi__c='C'));
  run(m,'M3|A;B',new Lead(LastName='x',Company='kM3',Multi__c='A;B'));
  run(m,'M3|B;C',new Lead(LastName='x',Company='kM3',Multi__c='B;C'));
  run(m,'M3|null',new Lead(LastName='x',Company='kM3',Multi__c=null));
  run(m,'M4|A;B',new Lead(LastName='x',Company='kM4',Multi__c='A;B'));
  run(m,'M4|B;A',new Lead(LastName='x',Company='kM4',Multi__c='B;A'));
  run(m,'M4|A',new Lead(LastName='x',Company='kM4',Multi__c='A'));
  run(m,'B1|true',new Lead(LastName='x',Company='kB1',Flag__c=true));
  run(m,'B1|false',new Lead(LastName='x',Company='kB1',Flag__c=false));
  run(m,'B2|true',new Lead(LastName='x',Company='kB2',Flag__c=true));
  run(m,'B2|false',new Lead(LastName='x',Company='kB2',Flag__c=false));
  run(m,'B3|false',new Lead(LastName='x',Company='kB3',Flag__c=false));
  run(m,'B3|true',new Lead(LastName='x',Company='kB3',Flag__c=true));
  run(m,'D1|0102',new Lead(LastName='x',Company='kD1',Day__c=Date.newInstance(2026,1,2)));
  run(m,'D1|0101',new Lead(LastName='x',Company='kD1',Day__c=Date.newInstance(2026,1,1)));
  run(m,'D1|null',new Lead(LastName='x',Company='kD1',Day__c=null));
  run(m,'D2|0101',new Lead(LastName='x',Company='kD2',Day__c=Date.newInstance(2026,1,1)));
  run(m,'D2|1231',new Lead(LastName='x',Company='kD2',Day__c=Date.newInstance(2025,12,31)));
  run(m,'D3|0101',new Lead(LastName='x',Company='kD3',Day__c=Date.newInstance(2026,1,1)));
  run(m,'D3|0102',new Lead(LastName='x',Company='kD3',Day__c=Date.newInstance(2026,1,2)));
  run(m,'D4|1231',new Lead(LastName='x',Company='kD4',Day__c=Date.newInstance(2025,12,31)));
  run(m,'D4|0101',new Lead(LastName='x',Company='kD4',Day__c=Date.newInstance(2026,1,1)));
  return m;
 }
}
