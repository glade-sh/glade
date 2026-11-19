@IsTest(SeeAllData=true) public class NullExistingCore67 {
 public static String probeObject(String obj, List<String> fields) {
  Map<String,Object> out=new Map<String,Object>();
  for(String f:fields) {
   Map<String,Object> row=new Map<String,Object>();
   try {
    List<SObject> ns=Database.query('SELECT Id FROM '+obj+' WHERE '+f+' = null LIMIT 1',AccessLevel.SYSTEM_MODE);
    List<SObject> vs=Database.query('SELECT Id FROM '+obj+' WHERE '+f+' != null LIMIT 1',AccessLevel.SYSTEM_MODE);
    if(ns.isEmpty() || vs.isEmpty()) {row.put('skip','no existing null/non-null pair');out.put(f,row);continue;}
    Set<Id> ids=new Set<Id>{ns[0].Id,vs[0].Id};Map<Id,String> labels=new Map<Id,String>{ns[0].Id=>'N',vs[0].Id=>'V'};
    for(String tail:new List<String>{'NULLS FIRST','NULLS LAST','ASC','DESC','DESC NULLS LAST'}) {
     List<String> result=new List<String>();for(SObject x:Database.query('SELECT Id FROM '+obj+' WHERE Id IN :ids ORDER BY '+f+' '+tail,AccessLevel.SYSTEM_MODE)) result.add(labels.get(x.Id));row.put(tail,String.join(result,''));
    }
   } catch(Exception e) {row.put('skip','EXC '+e.getTypeName()+': '+e.getMessage().left(160));}
   out.put(f,row);
  }
  return JSON.serialize(out);
 }
}