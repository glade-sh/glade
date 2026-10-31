@IsTest private class ClearedHash62Test { @IsTest static void observe() {
Map<String,Object> o = new Map<String,Object>();
Account a=new Account(Name='Synthetic'); insert a;
Contact c=new Contact(LastName='Synthetic'); insert c;
Lead l=new Lead(LastName='Synthetic',Company='Example'); insert l;
Opportunity p=new Opportunity(Name='Synthetic',StageName='Prospecting',CloseDate=Date.newInstance(2030,1,1)); insert p;
Case k=new Case(); insert k;
Account aq=[SELECT Id,Name FROM Account WHERE Id=:a.Id];
Contact cq=[SELECT Id,LastName FROM Contact WHERE Id=:c.Id];
Lead lq=[SELECT Id,LastName FROM Lead WHERE Id=:l.Id];
Opportunity pq=[SELECT Id,Name FROM Opportunity WHERE Id=:p.Id];
Case kq=[SELECT Id FROM Case WHERE Id=:k.Id];
List<SObject> queried=new List<SObject>{aq,cq,lq,pq,kq};
List<SObject> fresh=new List<SObject>{new Account(Name='Synthetic'),new Contact(LastName='Synthetic'),new Lead(LastName='Synthetic'),new Opportunity(Name='Synthetic'),new Case()};
List<Object> rows=new List<Object>();
for(Integer i=0;i<queried.size();i++){
 SObject q=queried[i]; SObject f=fresh[i]; q.Id=null;
 rows.add(new List<Object>{q.equals(f),System.hashCode(q),System.hashCode(f),System.hashCode(new List<SObject>{q}),System.hashCode(new List<SObject>{f}),System.hashCode(new List<List<SObject>>{new List<SObject>{q}}),System.hashCode(new List<List<SObject>>{new List<SObject>{f}})});
}
o.put('C01',rows);
List<SObject> nullId=new List<SObject>{new Account(Name='Synthetic',Id=null),new Contact(LastName='Synthetic',Id=null),new Lead(LastName='Synthetic',Id=null),new Opportunity(Name='Synthetic',Id=null),new Case(Id=null)};
rows=new List<Object>();
for(Integer i=0;i<queried.size();i++){SObject q=queried[i]; SObject f=nullId[i];rows.add(new List<Object>{q.equals(f),System.hashCode(q),System.hashCode(f),System.hashCode(new List<SObject>{q}),System.hashCode(new List<SObject>{f})});}
o.put('C05',rows);
List<SObject> inserted=new List<SObject>{a,c,l,p,k};
List<SObject> equivalents=new List<SObject>{new Account(Name='Synthetic'),new Contact(LastName='Synthetic'),new Lead(LastName='Synthetic',Company='Example'),new Opportunity(Name='Synthetic',StageName='Prospecting',CloseDate=Date.newInstance(2030,1,1)),new Case()};
rows=new List<Object>();
for(Integer i=0;i<inserted.size();i++){
 SObject q=inserted[i]; SObject f=equivalents[i]; q.Id=null;
 rows.add(new List<Object>{q.equals(f),System.hashCode(q),System.hashCode(f),System.hashCode(new List<SObject>{q}),System.hashCode(new List<SObject>{f})});
}
o.put('C02',rows);
Account different=new Account(Name='Other');
o.put('C03',new List<Boolean>{aq.equals(different),System.hashCode(aq)==System.hashCode(different)});
try {System.hashCode(null);o.put('C04','accepted');} catch(Exception e){o.put('C04',e.getTypeName()+':'+e.getMessage());}
System.assert(false,'OBS|' + JSON.serialize(o));
} }
