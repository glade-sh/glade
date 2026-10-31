@IsTest private class FieldHash68Test { @IsTest static void observe() {
Map<String,Object> o=new Map<String,Object>();
Date d0=Date.newInstance(1970,1,1); Date d1=Date.newInstance(2030,1,1); Date d2=Date.newInstance(2030,1,2); Date d3=Date.newInstance(2024,2,29); Date d4=Date.newInstance(1969,12,31);
o.put('F01',new List<Integer>{System.hashCode(d0),System.hashCode(d1),System.hashCode(d2),System.hashCode(d3),System.hashCode(d4),System.hashCode('CloseDate'),System.hashCode('Company'),System.hashCode('StageName')});
o.put('F02',new List<Integer>{System.hashCode(new Lead(Company='Example')),System.hashCode(new Lead(Company='Other')),System.hashCode(new Lead(Company=null)),System.hashCode(new Lead(LastName='Synthetic',Company='Example')),System.hashCode(new List<Lead>{new Lead(LastName='Synthetic',Company='Example')})});
o.put('F03',new List<Integer>{System.hashCode(new Opportunity(StageName='Prospecting')),System.hashCode(new Opportunity(StageName='Closed Won')),System.hashCode(new Opportunity(StageName=null)),System.hashCode(new Opportunity(CloseDate=d0)),System.hashCode(new Opportunity(CloseDate=d1)),System.hashCode(new Opportunity(CloseDate=d2)),System.hashCode(new Opportunity(CloseDate=d3)),System.hashCode(new Opportunity(CloseDate=d4)),System.hashCode(new Opportunity(CloseDate=null))});
o.put('F04',new List<Integer>{System.hashCode(new Opportunity(Name='Synthetic',StageName='Prospecting',CloseDate=d1)),System.hashCode(new Opportunity(Name='Other',StageName='Closed Won',CloseDate=d3)),System.hashCode(new List<Opportunity>{new Opportunity(Name='Synthetic',StageName='Prospecting',CloseDate=d1)}),System.hashCode(new List<List<Opportunity>>{new List<Opportunity>{new Opportunity(Name='Other',StageName='Closed Won',CloseDate=d3)}})});
o.put('F07',new List<Integer>{System.hashCode(new List<Date>{d0}),System.hashCode(new List<Date>{d1}),System.hashCode(new List<Date>{d2}),System.hashCode(new List<Date>{d3}),System.hashCode(new List<Date>{d4}),System.hashCode(new List<Date>{d0,d1}),System.hashCode(new List<List<Date>>{new List<Date>{d1}})});
o.put('F08',new List<Integer>{System.hashCode(new List<Object>{d1,new Account()}),System.hashCode(new List<Object>{new Account(),d2})});
o.put('F05',new List<Boolean>{new Lead(Company='Example').equals(new Lead(Company='Other')),new Opportunity(CloseDate=d1).equals(new Opportunity(CloseDate=d2))});
try{System.hashCode(null);o.put('F06','accepted');}catch(Exception e){o.put('F06',e.getTypeName()+':'+e.getMessage());}
System.assert(false,'OBS|' + JSON.serialize(o));
} }
