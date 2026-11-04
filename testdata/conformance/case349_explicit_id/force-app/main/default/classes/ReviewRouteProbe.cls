public class ReviewRouteProbe {
 public static List<Object> run() {
  Savepoint sp=Database.setSavepoint(); List<Object> rows=new List<Object>();
  for (String mode : new List<String>{'implicit','explicit'}) {
   for (String kind : new List<String>{'first','bademail','validemail','blankemail'}) {
    Account seed=new Account(Name='Business'); Database.insert(seed,AccessLevel.SYSTEM_MODE);
    Account a=new Account(Id=seed.Id);
    if (kind=='first') a.FirstName='Synthetic';
    if (kind=='bademail') a.PersonEmail='bad';
    if (kind=='validemail') a.PersonEmail='synthetic@example.invalid';
    if (kind=='blankemail') a.PersonEmail='   ';
    ReviewRouteTrace.kinds.clear();
    Database.UpsertResult r;
    try {
     if (mode=='explicit') r=Database.upsert(a,Account.Fields.Id,false,AccessLevel.SYSTEM_MODE);
     else r=Database.upsert(a,false,AccessLevel.SYSTEM_MODE);
     rows.add(new Map<String,Object>{'route'=>mode,'case'=>kind,'success'=>r.isSuccess(),'status'=>r.isSuccess()?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode()),'before'=>ReviewRouteTrace.kinds.clone()});
    } catch(Exception e) { rows.add(new Map<String,Object>{'route'=>mode,'case'=>kind,'exception'=>e.getTypeName(),'message'=>e.getMessage(),'before'=>ReviewRouteTrace.kinds.clone()}); }
   }
  }
  Database.rollback(sp); return rows;
 }
}
