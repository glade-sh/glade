public class FinSupport {
 public static String noRes;
 public static String noCtx;
 public static String yesRes;
 public static String yesCtx;
 public static Integer noCount;
 public static Integer yesCount;
 public static String call() {
  HttpRequest req = new HttpRequest();
  req.setEndpoint('https://example.invalid/probe');
  req.setMethod('GET');
  try { HttpResponse res = new Http().send(req); return 'ok:' + res.getStatusCode(); }
  catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
 }
 public class NoFin implements Finalizer {
  public void execute(FinalizerContext ctx) {
   FinSupport.noRes = FinSupport.call();
   FinSupport.noCtx = String.valueOf(ctx.getResult());
   FinSupport.noCount = Limits.getCallouts();
  }
 }
 public class YesFin implements Finalizer {
  public void execute(FinalizerContext ctx) {
   FinSupport.yesRes = FinSupport.call();
   FinSupport.yesCtx = String.valueOf(ctx.getResult());
   FinSupport.yesCount = Limits.getCallouts();
  }
 }
 public class NoParent implements Queueable {
  public void execute(QueueableContext ctx) { System.attachFinalizer(new NoFin()); }
 }
 public class YesParent implements Queueable, Database.AllowsCallouts {
  public void execute(QueueableContext ctx) { System.attachFinalizer(new YesFin()); }
 }
 public class Mock implements HttpCalloutMock {
  public HTTPResponse respond(HTTPRequest req) {
   HttpResponse res = new HttpResponse();
   res.setStatusCode(201);
   res.setBody('{"ok":true}');
   return res;
  }
 }
}
