public class EmailBoundaryProbe {
 public static String capture(String value) {
  EmailBoundaryTrace.events.clear();
  Account a=new Account(Name='Synthetic email boundary',PersonEmail=value);
  Database.SaveResult r=Database.insert(a,false);
  String status=r.isSuccess()?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode());
  return status+'|'+EmailBoundaryTrace.events.size()+'|'+(EmailBoundaryTrace.events.isEmpty()?'none':EmailBoundaryTrace.events[0]);
 }
}
