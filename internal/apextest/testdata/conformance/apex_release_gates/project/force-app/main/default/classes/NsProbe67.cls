public class NsProbe67 {
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('\\s+',' ').replaceAll('[a-zA-Z0-9]{15,18}','ID'); return e.getTypeName()+'|'+m; }
  static String st(Integer q0,Integer a0,Integer r0){ return 'q='+(Limits.getQueries()-q0)+' agg='+(Limits.getAggregateQueries()-a0)+' rows='+(Limits.getQueryRows()-r0); }
  public static String run(){
    List<String> o=new List<String>();
    L1__c a=new L1__c(Name='a'); insert a;
    L2__c b=new L2__c(Name='b',Up__c=a.Id); insert b;
    L3__c c=new L3__c(Name='c',Up__c=b.Id); insert c;
    L4__c d=new L4__c(Name='d',Up__c=c.Id); insert d;
    L5__c e=new L5__c(Name='e',Up__c=d.Id); insert e;
    L6__c f=new L6__c(Name='f',Up__c=e.Id); insert f;
    Integer q0, a0, r0;
    q0=Limits.getQueries(); a0=Limits.getAggregateQueries(); r0=Limits.getQueryRows();
    try{ List<L1__c> r=[SELECT Id,(SELECT Id FROM Down2__r) FROM L1__c]; o.add('NS1 '+st(q0,a0,r0)); }catch(Exception x){ o.add('NS1 '+ex(x)); }
    q0=Limits.getQueries(); a0=Limits.getAggregateQueries(); r0=Limits.getQueryRows();
    try{ List<L1__c> r=[SELECT Id,(SELECT Id,(SELECT Id FROM Down3__r) FROM Down2__r) FROM L1__c]; o.add('NS2 '+st(q0,a0,r0)); }catch(Exception x){ o.add('NS2 '+ex(x)); }
    q0=Limits.getQueries(); a0=Limits.getAggregateQueries(); r0=Limits.getQueryRows();
    try{ List<L1__c> r=[SELECT Id,(SELECT Id,(SELECT Id,(SELECT Id FROM Down4__r) FROM Down3__r) FROM Down2__r) FROM L1__c]; o.add('NS3 '+st(q0,a0,r0)); }catch(Exception x){ o.add('NS3 '+ex(x)); }
    q0=Limits.getQueries(); a0=Limits.getAggregateQueries(); r0=Limits.getQueryRows();
    try{ List<L1__c> r=[SELECT Id,(SELECT Id,(SELECT Id,(SELECT Id,(SELECT Id FROM Down5__r) FROM Down4__r) FROM Down3__r) FROM Down2__r) FROM L1__c]; o.add('NS4 '+st(q0,a0,r0)); }catch(Exception x){ o.add('NS4 '+ex(x)); }
    q0=Limits.getQueries(); a0=Limits.getAggregateQueries(); r0=Limits.getQueryRows();
    try{ List<L1__c> r=Database.query('SELECT Id,(SELECT Id,(SELECT Id,(SELECT Id,(SELECT Id FROM Down5__r) FROM Down4__r) FROM Down3__r) FROM Down2__r) FROM L1__c'); o.add('NS4d '+st(q0,a0,r0)+' depth='+(r[0].getSObjects('Down2__r')[0].getSObjects('Down3__r')[0].getSObjects('Down4__r')[0].getSObjects('Down5__r')==null?'x':'5')); }catch(Exception x){ o.add('NS4d '+ex(x)); }
    q0=Limits.getQueries(); a0=Limits.getAggregateQueries(); r0=Limits.getQueryRows();
    try{ List<L1__c> r=Database.query('SELECT Id,(SELECT Id,(SELECT Id,(SELECT Id,(SELECT Id,(SELECT Id FROM Down6__r) FROM Down5__r) FROM Down4__r) FROM Down3__r) FROM Down2__r) FROM L1__c'); o.add('NS5d '+st(q0,a0,r0)); }catch(Exception x){ o.add('NS5d '+ex(x)); }
    q0=Limits.getQueries(); a0=Limits.getAggregateQueries(); r0=Limits.getQueryRows();
    try{ AggregateResult[] r=[SELECT COUNT(Id) c FROM L1__c]; o.add('NSa '+st(q0,a0,r0)); }catch(Exception x){ o.add('NSa '+ex(x)); }
    q0=Limits.getQueries(); a0=Limits.getAggregateQueries(); r0=Limits.getQueryRows();
    try{ List<L3__c> r=[SELECT Id,Up__r.Id,(SELECT Id FROM Down4__r) FROM L3__c]; o.add('NS8 '+st(q0,a0,r0)); }catch(Exception x){ o.add('NS8 '+ex(x)); }
    return String.join(o,' ## ');
  }
}
